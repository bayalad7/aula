//go:build linux

package platform

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// The health of the computer on Linux (docs/privacidad.md rule 103): read
// from /proc and /sys, without special rights and without running anything.
// The disk's S.M.A.R.T. state needs root and a tool, so it is not read.

// healthRoot prefixes /proc and /sys; tests point it at a fake tree.
var healthRoot = ""

func hpath(p string) string { return filepath.Join(healthRoot, p) }

func newSystemHealth() SystemHealth { return linuxHealth{} }

type linuxHealth struct{}

func (linuxHealth) SystemDrive() (string, error) { return "/", nil }

func (linuxHealth) DiskSpace(path string) (DiskSpace, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return DiskSpace{}, fmt.Errorf("disk space: %w", err)
	}
	bs := int64(st.Bsize)
	return DiskSpace{Free: int64(st.Bavail) * bs, Total: int64(st.Blocks) * bs}, nil
}

func (linuxHealth) DiskFailing() (bool, error) {
	return false, fmt.Errorf("disk S.M.A.R.T.: %w", ErrNotSupported)
}

func (linuxHealth) Memory() (Memory, error) {
	f, err := os.Open(hpath("/proc/meminfo"))
	if err != nil {
		return Memory{}, fmt.Errorf("memory: %w", err)
	}
	defer f.Close()
	var m Memory
	s := bufio.NewScanner(f)
	for s.Scan() {
		k, v, ok := strings.Cut(s.Text(), ":")
		if !ok {
			continue
		}
		f := strings.Fields(v)
		if len(f) == 0 {
			continue
		}
		n, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			continue
		}
		switch k {
		case "MemTotal":
			m.Total = n * 1024
		case "MemAvailable":
			m.Available = n * 1024
		}
	}
	if m.Total == 0 {
		return Memory{}, fmt.Errorf("memory: no MemTotal: %w", ErrNotSupported)
	}
	return m, nil
}

func (linuxHealth) CPUTimes() (CPUTimes, error) {
	data, err := os.ReadFile(hpath("/proc/stat"))
	if err != nil {
		return CPUTimes{}, fmt.Errorf("cpu: %w", err)
	}
	return parseProcStat(string(data))
}

// parseProcStat reads the "cpu" line: user nice system idle iowait irq
// softirq steal. Waiting for the disk counts as idle.
func parseProcStat(data string) (CPUTimes, error) {
	for _, line := range strings.Split(data, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || f[0] != "cpu" {
			continue
		}
		var t CPUTimes
		for i, v := range f[1:min(len(f), 9)] {
			n, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return CPUTimes{}, fmt.Errorf("cpu: %w", err)
			}
			t.Total += n
			if i == 3 || i == 4 { // idle, iowait
				t.Idle += n
			}
		}
		return t, nil
	}
	return CPUTimes{}, fmt.Errorf("cpu: no cpu line: %w", ErrNotSupported)
}

// CPULimitPct is the frequency the first processor may reach now against its
// maximum (cpufreq): a power profile or heat lower it.
func (linuxHealth) CPULimitPct() (int, error) {
	dir := hpath("/sys/devices/system/cpu/cpu0/cpufreq")
	limit, err1 := readInt(filepath.Join(dir, "scaling_max_freq"))
	top, err2 := readInt(filepath.Join(dir, "cpuinfo_max_freq"))
	if err1 != nil || err2 != nil || top <= 0 {
		return 0, fmt.Errorf("cpu limit: %w", ErrNotSupported)
	}
	return int(min(max(limit*100/top, 1), 100)), nil
}

func (linuxHealth) Uptime() (time.Duration, error) {
	data, err := os.ReadFile(hpath("/proc/uptime"))
	if err != nil {
		return 0, fmt.Errorf("uptime: %w", err)
	}
	f := strings.Fields(string(data))
	if len(f) == 0 {
		return 0, fmt.Errorf("uptime: empty")
	}
	s, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0, fmt.Errorf("uptime: %w", err)
	}
	return time.Duration(s * float64(time.Second)), nil
}

// RebootPending: Debian and Ubuntu leave /run/reboot-required after an
// update that needs one. Other families have no such mark.
func (linuxHealth) RebootPending() (bool, error) {
	if _, err := os.Stat(hpath("/run/reboot-required")); err == nil {
		return true, nil
	}
	if _, err := os.Stat(hpath("/var/lib/dpkg")); err == nil {
		return false, nil
	}
	return false, fmt.Errorf("reboot pending: %w", ErrNotSupported)
}

func (linuxHealth) Link(local net.IP) (Link, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return Link{}, fmt.Errorf("link: %w", err)
	}
	name := ""
	for _, ifc := range ifs {
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.Equal(local) {
				name = ifc.Name
			}
		}
	}
	if name == "" {
		return Link{}, fmt.Errorf("link: no card has the address: %w", ErrNotSupported)
	}
	var names []string
	for _, ifc := range ifs {
		names = append(names, ifc.Name)
	}
	return linkOf(name, names), nil
}

// linkOf describes the card name; others are every card, for CableIdle.
func linkOf(name string, others []string) Link {
	l := Link{Kind: netKind(name), Mbps: -1, SignalPct: -1}
	if n, err := readInt(hpath("/sys/class/net/" + name + "/speed")); err == nil && n > 0 {
		l.Mbps = int(n)
	}
	if l.Kind == LinkWiFi {
		l.SignalPct = wifiSignal(name)
		for _, o := range others {
			if o != name && netKind(o) == LinkEthernet && physical(o) {
				if c, err := readInt(hpath("/sys/class/net/" + o + "/carrier")); err == nil && c == 1 {
					l.CableIdle = true
				}
			}
		}
	}
	return l
}

func netKind(name string) LinkKind {
	base := hpath("/sys/class/net/" + name)
	if exists(base+"/wireless") || exists(base+"/phy80211") {
		return LinkWiFi
	}
	if t, err := readInt(base + "/type"); err == nil && t == 1 && physical(name) { // ARPHRD_ETHER
		return LinkEthernet
	}
	return LinkOther
}

// physical: a card with hardware behind it (not a bridge, veth or VPN).
func physical(name string) bool { return exists(hpath("/sys/class/net/" + name + "/device")) }

// wifiSignal reads the link quality of /proc/net/wireless (out of 70).
func wifiSignal(name string) int {
	data, err := os.ReadFile(hpath("/proc/net/wireless"))
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(k) != name {
			continue
		}
		f := strings.Fields(v)
		if len(f) < 2 {
			return -1
		}
		q, err := strconv.ParseFloat(strings.TrimSuffix(f[1], "."), 64)
		if err != nil {
			return -1
		}
		return int(min(max(q*100/70, 0), 100))
	}
	return -1
}

func (linuxHealth) Battery() (Battery, error) {
	dir := hpath("/sys/class/power_supply")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Battery{}, nil // no power_supply class: no battery
	}
	b := Battery{WearPct: -1}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		switch readString(p + "/type") {
		case "Mains", "USB":
			if n, err := readInt(p + "/online"); err == nil && n == 1 {
				b.OnAC = true
			}
		case "Battery":
			if readString(p+"/scope") == "Device" {
				continue // a mouse's or a keyboard's battery
			}
			b.Present = true
			if n, err := readInt(p + "/capacity"); err == nil {
				b.ChargePct = int(n)
			}
			full, design := readInt64Pair(p, "energy")
			if design <= 0 {
				full, design = readInt64Pair(p, "charge")
			}
			if design > 0 && full > 0 {
				b.WearPct = int(max(0, 100-full*100/design))
			}
		}
	}
	if !b.Present {
		b.OnAC = false
	}
	return b, nil
}

func readInt64Pair(dir, kind string) (full, design int64) {
	full, _ = readInt(dir + "/" + kind + "_full")
	design, _ = readInt(dir + "/" + kind + "_full_design")
	return full, design
}

// Devices for the self-test: sound output, a network card, a display
// adapter (and whether it has its own driver), a keyboard and a mouse.
func (linuxHealth) Devices() (Devices, error) {
	d := Devices{AudioOut: DeviceMissing, NIC: DeviceMissing, GPU: DeviceMissing,
		Keyboard: DeviceMissing, Mouse: DeviceMissing}
	if data, err := os.ReadFile(hpath("/proc/asound/cards")); err != nil {
		d.AudioOut = DeviceUnknown
	} else if !strings.Contains(string(data), "no soundcards") && strings.TrimSpace(string(data)) != "" {
		d.AudioOut = DeviceOK
	}
	if entries, err := os.ReadDir(hpath("/sys/class/net")); err != nil {
		d.NIC = DeviceUnknown
	} else {
		for _, e := range entries {
			if physical(e.Name()) {
				d.NIC = DeviceOK
			}
		}
	}
	d.GPU = gpuState()
	if data, err := os.ReadFile(hpath("/proc/bus/input/devices")); err != nil {
		d.Keyboard, d.Mouse = DeviceUnknown, DeviceUnknown
	} else {
		d.Keyboard, d.Mouse = inputDevices(string(data))
	}
	return d, nil
}

// gpuState: a DRM card whose driver is a generic framebuffer has no driver
// of its own (like Windows' "Microsoft Basic Display Adapter").
func gpuState() DeviceState {
	cards, _ := filepath.Glob(hpath("/sys/class/drm/card[0-9]"))
	if len(cards) == 0 {
		return DeviceMissing
	}
	for _, c := range cards {
		drv, err := os.Readlink(c + "/device/driver")
		if err != nil {
			continue
		}
		switch filepath.Base(drv) {
		case "simpledrm", "simple-framebuffer", "efifb", "vesafb", "bochs-drm":
		default:
			return DeviceOK
		}
	}
	return DeviceBasicDriver
}

// inputDevices reads /proc/bus/input/devices: a keyboard has the "kbd"
// handler and letter keys (EV with key and repeat: 120013), a mouse the
// "mouse" handler.
func inputDevices(data string) (kbd, mouse DeviceState) {
	kbd, mouse = DeviceMissing, DeviceMissing
	for _, block := range strings.Split(data, "\n\n") {
		handlers, ev := "", ""
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "H: Handlers="):
				handlers = line
			case strings.HasPrefix(line, "B: EV="):
				ev = strings.TrimPrefix(line, "B: EV=")
			}
		}
		if strings.Contains(handlers, "kbd") && ev == "120013" {
			kbd = DeviceOK
		}
		if strings.Contains(handlers, "mouse") {
			mouse = DeviceOK
		}
	}
	return kbd, mouse
}

func readInt(path string) (int64, error) {
	return strconv.ParseInt(readString(path), 10, 64)
}

func readString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func isNoSpace(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT)
}
func isReadOnly(err error) bool { return errors.Is(err, syscall.EROFS) }
