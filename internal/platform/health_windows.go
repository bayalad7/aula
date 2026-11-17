//go:build windows

package platform

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// The health of the computer on Windows (docs/protocol.md §5.27). Every
// call only reads; none needs WMI. As SYSTEM (the service) everything can be
// read; in window mode without administrator rights the S.M.A.R.T. state
// may not, and then it does not travel.

var (
	modKernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx   = modKernel32.NewProc("GlobalMemoryStatusEx")
	procGetSystemTimes         = modKernel32.NewProc("GetSystemTimes")
	procGetTickCount64         = modKernel32.NewProc("GetTickCount64")
	procGetSystemPowerStatus   = modKernel32.NewProc("GetSystemPowerStatus")
	procCallNtPowerInformation = windows.NewLazySystemDLL("powrprof.dll").NewProc("CallNtPowerInformation")
	modWlanapi                 = windows.NewLazySystemDLL("wlanapi.dll")
	procWlanOpenHandle         = modWlanapi.NewProc("WlanOpenHandle")
	procWlanCloseHandle        = modWlanapi.NewProc("WlanCloseHandle")
	procWlanEnumInterfaces     = modWlanapi.NewProc("WlanEnumInterfaces")
	procWlanQueryInterface     = modWlanapi.NewProc("WlanQueryInterface")
	procWlanFreeMemory         = modWlanapi.NewProc("WlanFreeMemory")
	errHealthUnreadable        = errors.New("platform: health datum not available")
	guidDevclassDisplay        = windows.GUID{Data1: 0x4d36e968, Data2: 0xe325, Data3: 0x11ce, Data4: [8]byte{0xbf, 0xc1, 0x08, 0x00, 0x2b, 0xe1, 0x03, 0x18}}
	guidDevclassKeyboard       = windows.GUID{Data1: 0x4d36e96b, Data2: 0xe325, Data3: 0x11ce, Data4: [8]byte{0xbf, 0xc1, 0x08, 0x00, 0x2b, 0xe1, 0x03, 0x18}}
	guidDevclassMouse          = windows.GUID{Data1: 0x4d36e96f, Data2: 0xe325, Data3: 0x11ce, Data4: [8]byte{0xbf, 0xc1, 0x08, 0x00, 0x2b, 0xe1, 0x03, 0x18}}
	guidDeviceInterfaceBattery = windows.GUID{Data1: 0x72631e54, Data2: 0x78a4, Data3: 0x11d0, Data4: [8]byte{0xbc, 0xf7, 0x00, 0xaa, 0x00, 0xb7, 0xb3, 0x2a}}
	rebootPendingKeys          = []string{`SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired`, `SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending`}
	audioRenderKey             = `SOFTWARE\Microsoft\Windows\CurrentVersion\MMDevices\Audio\Render`
)

// DeviceIoControl codes (winioctl.h, ntddstor.h, batclass.h).
const (
	ioctlVolumeGetDiskExtents  = 0x00560000
	ioctlStoragePredictFailure = 0x002d1100
	ioctlStorageQueryProperty  = 0x002d1400
	ioctlBatteryQueryTag       = 0x00294040
	ioctlBatteryQueryInfo      = 0x00294044

	processorInformation = 11 // POWER_INFORMATION_LEVEL
	ifHardwareInterface  = 0x01
	mediaConnected       = 1
	deviceStateActive    = 1
	wlanCurrentConn      = 7 // wlan_intf_opcode_current_connection
	acOnline             = 1
	batteryNone          = 128
	batteryUnknownFlag   = 255
	batteryUnknownPct    = 255
)

func newSystemHealth() SystemHealth { return winHealth{} }

type winHealth struct{}

func (winHealth) SystemDrive() (string, error) {
	dir, err := windows.GetWindowsDirectory()
	if err != nil {
		return "", fmt.Errorf("windows directory: %w", err)
	}
	return filepath.VolumeName(dir) + `\`, nil
}

func (winHealth) DiskSpace(path string) (DiskSpace, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return DiskSpace{}, fmt.Errorf("disk space %q: %w", path, err)
	}
	var avail, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(p, &avail, &total, &free); err != nil {
		return DiskSpace{}, fmt.Errorf("disk space: %w", err)
	}
	return DiskSpace{Free: int64(avail), Total: int64(total)}, nil
}

// systemDiskNumber finds the physical disk of the system volume.
func systemDiskNumber() (uint32, error) {
	root, err := winHealth{}.SystemDrive()
	if err != nil {
		return 0, err
	}
	vol, err := windows.UTF16PtrFromString(`\\.\` + strings.TrimSuffix(root, `\`))
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateFile(vol, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return 0, fmt.Errorf("open system volume: %w", err)
	}
	defer windows.CloseHandle(h)
	// VOLUME_DISK_EXTENTS: NumberOfDiskExtents, padding, then DISK_EXTENT
	// {DiskNumber uint32, padding, StartingOffset, ExtentLength int64}.
	var buf [64]byte
	var n uint32
	if err := windows.DeviceIoControl(h, ioctlVolumeGetDiskExtents, nil, 0, &buf[0], uint32(len(buf)), &n, nil); err != nil {
		return 0, fmt.Errorf("system volume extents: %w", err)
	}
	if binary.LittleEndian.Uint32(buf[0:]) < 1 {
		return 0, errHealthUnreadable
	}
	return binary.LittleEndian.Uint32(buf[8:]), nil
}

func (winHealth) DiskFailing() (bool, error) {
	num, err := systemDiskNumber()
	if err != nil {
		return false, err
	}
	name, _ := windows.UTF16PtrFromString(fmt.Sprintf(`\\.\PhysicalDrive%d`, num))
	// Read access (administrator or SYSTEM) for the SATA prediction; none
	// is needed for the NVMe health log.
	if h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0); err == nil {
		defer windows.CloseHandle(h)
		var out [516]byte // STORAGE_PREDICT_FAILURE: PredictFailure + 512 vendor bytes
		var n uint32
		if err := windows.DeviceIoControl(h, ioctlStoragePredictFailure, nil, 0, &out[0], uint32(len(out)), &n, nil); err == nil && n >= 4 {
			return binary.LittleEndian.Uint32(out[0:]) != 0, nil
		}
	}
	h, err := windows.CreateFile(name, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return false, fmt.Errorf("open system disk: %w", err)
	}
	defer windows.CloseHandle(h)
	return nvmeCriticalWarning(h)
}

// nvmeCriticalWarning reads the NVMe SMART / health log page and reports a
// failing disk when its critical warning says the spare is low, the
// reliability is degraded or the media is read-only (not the temperature
// bit: a hot disk is not a dying one).
func nvmeCriticalWarning(h windows.Handle) (bool, error) {
	const (
		propertyDeviceProtocolSpecific = 50
		protocolTypeNvme               = 3
		nvmeDataTypeLogPage            = 2
		nvmeLogPageHealthInfo          = 2
		protocolDataSize               = 40 // STORAGE_PROTOCOL_SPECIFIC_DATA
		logSize                        = 512
		queryHeader                    = 8 // PropertyId, QueryType
		descriptorHeader               = 8 // Version, Size
	)
	in := make([]byte, queryHeader+protocolDataSize+logSize)
	le := binary.LittleEndian
	le.PutUint32(in[0:], propertyDeviceProtocolSpecific)
	le.PutUint32(in[8:], protocolTypeNvme)
	le.PutUint32(in[12:], nvmeDataTypeLogPage)
	le.PutUint32(in[16:], nvmeLogPageHealthInfo)
	le.PutUint32(in[24:], protocolDataSize) // ProtocolDataOffset
	le.PutUint32(in[28:], logSize)          // ProtocolDataLength
	out := make([]byte, len(in))
	var n uint32
	if err := windows.DeviceIoControl(h, ioctlStorageQueryProperty, &in[0], uint32(len(in)), &out[0], uint32(len(out)), &n, nil); err != nil {
		return false, fmt.Errorf("disk health: %w", err)
	}
	off := descriptorHeader + int(le.Uint32(out[descriptorHeader+16:]))
	if n < uint32(off+1) || off >= len(out) {
		return false, errHealthUnreadable
	}
	return out[off]&0x0d != 0, nil
}

func (winHealth) Memory() (Memory, error) {
	// MEMORYSTATUSEX: dwLength, dwMemoryLoad, then ullTotalPhys, ullAvailPhys…
	var m struct {
		Length, Load               uint32
		TotalPhys, AvailPhys       uint64
		TotalPage, AvailPage       uint64
		TotalVirtual, AvailVirtual uint64
		AvailExtendedVirtual       uint64
	}
	m.Length = uint32(unsafe.Sizeof(m))
	if r, _, err := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m))); r == 0 {
		return Memory{}, fmt.Errorf("memory status: %w", err)
	}
	return Memory{Total: int64(m.TotalPhys), Available: int64(m.AvailPhys)}, nil
}

func (winHealth) CPUTimes() (CPUTimes, error) {
	var idle, kernel, user windows.Filetime
	if r, _, err := procGetSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); r == 0 {
		return CPUTimes{}, fmt.Errorf("system times: %w", err)
	}
	ft := func(f windows.Filetime) uint64 { return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime) }
	// The kernel time includes the idle time.
	return CPUTimes{Idle: ft(idle), Total: ft(kernel) + ft(user)}, nil
}

func (winHealth) CPULimitPct() (int, error) {
	// PROCESSOR_POWER_INFORMATION per logical processor: Number, MaxMhz,
	// CurrentMhz, MhzLimit, MaxIdleState, CurrentIdleState.
	const size = 24
	count := int(windows.GetActiveProcessorCount(windows.ALL_PROCESSOR_GROUPS))
	if count < 1 {
		count = 1
	}
	buf := make([]byte, count*size)
	if r, _, _ := procCallNtPowerInformation.Call(processorInformation, 0, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); r != 0 {
		return 0, fmt.Errorf("processor power information: NTSTATUS 0x%x", r)
	}
	var maxMhz, limit uint64
	for i := 0; i < count; i++ {
		maxMhz += uint64(binary.LittleEndian.Uint32(buf[i*size+4:]))
		limit += uint64(binary.LittleEndian.Uint32(buf[i*size+12:]))
	}
	if maxMhz == 0 {
		return 0, errHealthUnreadable
	}
	return clampPct(int(limit*100/maxMhz), 1), nil
}

func clampPct(p, lo int) int { return max(lo, min(100, p)) }

func (winHealth) Uptime() (time.Duration, error) {
	r, _, _ := procGetTickCount64.Call()
	return time.Duration(r) * time.Millisecond, nil
}

func (winHealth) RebootPending() (bool, error) {
	for _, path := range rebootPendingKeys {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
		if err == nil {
			k.Close()
			return true, nil
		}
		if !errors.Is(err, registry.ErrNotExist) {
			return false, fmt.Errorf("reboot pending key: %w", err)
		}
	}
	return false, nil
}

// adapter is what Link and Devices need of a network card.
type adapter struct {
	guid      string // "{…}" as in AdapterName
	ifType    uint32
	up        bool
	hardware  bool
	connected bool
	mbps      int
	addrs     []net.IP
}

func adapters() ([]adapter, error) {
	size := uint32(15 << 10)
	var buf []byte
	for range 3 {
		buf = make([]byte, size)
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, windows.GAA_FLAG_SKIP_ANYCAST|windows.GAA_FLAG_SKIP_MULTICAST|windows.GAA_FLAG_SKIP_DNS_SERVER, 0,
			(*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])), &size)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			return nil, fmt.Errorf("adapters: %w", err)
		}
		buf = nil
	}
	if buf == nil {
		return nil, errHealthUnreadable
	}
	var out []adapter
	for a := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); a != nil; a = a.Next {
		ad := adapter{guid: windows.BytePtrToString(a.AdapterName), ifType: a.IfType, up: a.OperStatus == windows.IfOperStatusUp}
		speed := max(a.TransmitLinkSpeed, a.ReceiveLinkSpeed)
		ad.mbps = -1
		if speed > 0 && speed != ^uint64(0) {
			ad.mbps = int(speed / 1_000_000)
		}
		row := windows.MibIfRow2{InterfaceLuid: a.Luid}
		if windows.GetIfEntry2Ex(windows.MibIfEntryNormalWithoutStatistics, &row) == nil {
			ad.hardware = row.InterfaceAndOperStatusFlags&ifHardwareInterface != 0
			ad.connected = row.MediaConnectState == mediaConnected
		}
		for u := a.FirstUnicastAddress; u != nil; u = u.Next {
			if ip := u.Address.IP(); ip != nil {
				ad.addrs = append(ad.addrs, ip)
			}
		}
		out = append(out, ad)
	}
	return out, nil
}

func (winHealth) Link(local net.IP) (Link, error) {
	ads, err := adapters()
	if err != nil {
		return Link{}, err
	}
	for _, a := range ads {
		if !hasIP(a.addrs, local) {
			continue
		}
		l := Link{Kind: LinkOther, Mbps: a.mbps, SignalPct: -1}
		switch a.ifType {
		case windows.IF_TYPE_ETHERNET_CSMACD:
			l.Kind = LinkEthernet
		case windows.IF_TYPE_IEEE80211:
			l.Kind = LinkWiFi
			l.SignalPct = wifiSignal(a.guid)
			for _, b := range ads {
				if b.ifType == windows.IF_TYPE_ETHERNET_CSMACD && b.hardware && b.connected {
					l.CableIdle = true
				}
			}
		}
		return l, nil
	}
	return Link{}, errHealthUnreadable
}

func hasIP(list []net.IP, ip net.IP) bool {
	for _, x := range list {
		if x.Equal(ip) {
			return true
		}
	}
	return false
}

// wifiSignal reads the signal quality (0–100) of the Wi-Fi card with the
// given GUID, or -1. It reads nothing else of the connection (no network
// name).
func wifiSignal(guid string) int {
	var version uint32
	var h windows.Handle
	if procWlanOpenHandle.Find() != nil {
		return -1
	}
	if r, _, _ := procWlanOpenHandle.Call(2, 0, uintptr(unsafe.Pointer(&version)), uintptr(unsafe.Pointer(&h))); r != 0 {
		return -1
	}
	defer procWlanCloseHandle.Call(uintptr(h), 0)
	var list *byte
	if r, _, _ := procWlanEnumInterfaces.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&list))); r != 0 || list == nil {
		return -1
	}
	defer procWlanFreeMemory.Call(uintptr(unsafe.Pointer(list)))
	// WLAN_INTERFACE_INFO_LIST: dwNumberOfItems, dwIndex, then 532-byte
	// WLAN_INTERFACE_INFO entries starting with the interface GUID.
	const itemSize = 532
	count := *(*uint32)(unsafe.Pointer(list))
	for i := range uintptr(count) {
		g := (*windows.GUID)(unsafe.Add(unsafe.Pointer(list), 8+i*itemSize))
		if !strings.EqualFold(g.String(), guid) {
			continue
		}
		var size uint32
		var data *byte
		if r, _, _ := procWlanQueryInterface.Call(uintptr(h), uintptr(unsafe.Pointer(g)), wlanCurrentConn, 0,
			uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&data)), 0); r != 0 || data == nil {
			return -1
		}
		defer procWlanFreeMemory.Call(uintptr(unsafe.Pointer(data)))
		// WLAN_CONNECTION_ATTRIBUTES: wlanSignalQuality at offset 576.
		const signalOffset = 576
		if size < signalOffset+4 {
			return -1
		}
		return clampPct(int(*(*uint32)(unsafe.Add(unsafe.Pointer(data), signalOffset))), 0)
	}
	return -1
}

func (winHealth) Battery() (Battery, error) {
	var s struct {
		ACLineStatus, BatteryFlag, BatteryLifePercent, SystemStatusFlag byte
		BatteryLifeTime, BatteryFullLifeTime                            uint32
	}
	if r, _, err := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&s))); r == 0 {
		return Battery{}, fmt.Errorf("power status: %w", err)
	}
	if s.BatteryFlag&batteryNone != 0 || s.BatteryFlag == batteryUnknownFlag || s.BatteryLifePercent == batteryUnknownPct {
		return Battery{}, nil
	}
	return Battery{Present: true, ChargePct: clampPct(int(s.BatteryLifePercent), 0), OnAC: s.ACLineStatus == acOnline, WearPct: batteryWear()}, nil
}

// batteryWear is the capacity the first battery lost against its design,
// in %, or -1.
func batteryWear() int {
	paths, err := windows.CM_Get_Device_Interface_List("", &guidDeviceInterfaceBattery, windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT)
	if err != nil || len(paths) == 0 {
		return -1
	}
	name, err := windows.UTF16PtrFromString(paths[0])
	if err != nil {
		return -1
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return -1
	}
	defer windows.CloseHandle(h)
	var wait, tag, n uint32
	if windows.DeviceIoControl(h, ioctlBatteryQueryTag, (*byte)(unsafe.Pointer(&wait)), 4, (*byte)(unsafe.Pointer(&tag)), 4, &n, nil) != nil || tag == 0 {
		return -1
	}
	// BATTERY_QUERY_INFORMATION {BatteryTag, InformationLevel=0, AtRate}.
	var q [12]byte
	binary.LittleEndian.PutUint32(q[0:], tag)
	// BATTERY_INFORMATION (36 bytes): Capabilities, Technology + 3
	// reserved, Chemistry[4], DesignedCapacity (12), FullChargedCapacity (16)…
	var info [36]byte
	if windows.DeviceIoControl(h, ioctlBatteryQueryInfo, &q[0], uint32(len(q)), &info[0], uint32(len(info)), &n, nil) != nil {
		return -1
	}
	design := binary.LittleEndian.Uint32(info[12:])
	full := binary.LittleEndian.Uint32(info[16:])
	if design == 0 || full == 0 {
		return -1
	}
	if full >= design {
		return 0
	}
	return clampPct(int((design-full)*100/design), 0)
}

func (winHealth) Devices() (Devices, error) {
	d := Devices{
		AudioOut: audioOutput(),
		GPU:      displayState(),
		Keyboard: classPresent(&guidDevclassKeyboard),
		Mouse:    classPresent(&guidDevclassMouse),
		NIC:      DeviceUnknown,
	}
	if ads, err := adapters(); err == nil {
		d.NIC = DeviceMissing
		for _, a := range ads {
			if a.hardware && a.up && (a.ifType == windows.IF_TYPE_ETHERNET_CSMACD || a.ifType == windows.IF_TYPE_IEEE80211) {
				d.NIC = DeviceOK
			}
		}
	}
	return d, nil
}

// audioOutput looks for an active playback endpoint in the registry.
func audioOutput() DeviceState {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, audioRenderKey, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return DeviceUnknown
	}
	defer k.Close()
	names, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return DeviceUnknown
	}
	for _, n := range names {
		sk, err := registry.OpenKey(k, n, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		state, _, err := sk.GetIntegerValue("DeviceState")
		sk.Close()
		if err == nil && state == deviceStateActive {
			return DeviceOK
		}
	}
	return DeviceMissing
}

// eachDevice calls f with every present device of a setup class.
func eachDevice(class *windows.GUID, f func(windows.DevInfo, *windows.DevInfoData)) bool {
	set, err := windows.SetupDiGetClassDevsEx(class, "", 0, windows.DIGCF_PRESENT, 0, "")
	if err != nil {
		return false
	}
	defer set.Close()
	for i := 0; ; i++ {
		data, err := set.EnumDeviceInfo(i)
		if err != nil {
			break
		}
		f(set, data)
	}
	return true
}

func classPresent(class *windows.GUID) DeviceState {
	n := 0
	if !eachDevice(class, func(windows.DevInfo, *windows.DevInfoData) { n++ }) {
		return DeviceUnknown
	}
	if n == 0 {
		return DeviceMissing
	}
	return DeviceOK
}

// displayState is OK when some display adapter runs its own driver, and
// BasicDriver when all of them run Microsoft's basic display driver.
func displayState() DeviceState {
	n, real := 0, 0
	if !eachDevice(&guidDevclassDisplay, func(set windows.DevInfo, data *windows.DevInfoData) {
		n++
		v, err := set.DeviceRegistryProperty(data, windows.SPDRP_SERVICE)
		if s, ok := v.(string); err == nil && ok && !strings.EqualFold(s, "BasicDisplay") {
			real++
		}
	}) {
		return DeviceUnknown
	}
	switch {
	case n == 0:
		return DeviceMissing
	case real == 0:
		return DeviceBasicDriver
	}
	return DeviceOK
}

func documentsDirOf(session uint32) (string, error) {
	var tok windows.Token
	if err := windows.WTSQueryUserToken(session, &tok); err != nil {
		return "", fmt.Errorf("token of session %d: %w", session, err)
	}
	defer tok.Close()
	dir, err := tok.KnownFolderPath(windows.FOLDERID_Documents, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", fmt.Errorf("documents of session %d: %w", session, err)
	}
	return dir, nil
}

func isNoSpace(err error) bool {
	return errors.Is(err, windows.ERROR_DISK_FULL) || errors.Is(err, windows.ERROR_HANDLE_DISK_FULL)
}

func isReadOnly(err error) bool {
	return errors.Is(err, windows.ERROR_WRITE_PROTECT) || errors.Is(err, windows.ERROR_ACCESS_DENIED)
}
