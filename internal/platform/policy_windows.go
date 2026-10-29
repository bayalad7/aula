//go:build windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// --- Processes ------------------------------------------------------------------------

func newProcessControl() (ProcessControl, error) { return winProcesses{}, nil }

type winProcesses struct{}

func (winProcesses) List(session uint32) ([]ProcessInfo, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("process list: %w", err)
	}
	defer windows.CloseHandle(snap) //nolint:errcheck // snapshot
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var out []ProcessInfo
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		var sid uint32
		if windows.ProcessIdToSessionId(e.ProcessID, &sid) != nil || sid != session {
			continue
		}
		out = append(out, ProcessInfo{PID: int(e.ProcessID), Name: windows.UTF16ToString(e.ExeFile[:]), Path: imagePath(e.ProcessID)})
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, fmt.Errorf("process list: %w", err)
	}
	return out, nil
}

func imagePath(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h) //nolint:errcheck // query only
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

func (winProcesses) Kill(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil // already exited
	}
	if err != nil {
		return fmt.Errorf("kill %d: %w", pid, err)
	}
	defer windows.CloseHandle(h) //nolint:errcheck // terminate only
	if err := windows.TerminateProcess(h, 1); err != nil && !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return fmt.Errorf("kill %d: %w", pid, err)
	}
	return nil
}

// --- DNS ------------------------------------------------------------------------------

// nrptComment tags the NRPT rules Aula creates, so Clear finds them even if
// the journal that named them was lost.
const nrptComment = "aula-filtro-web"

var (
	dnsapi                    = windows.NewLazySystemDLL("dnsapi.dll")
	procDnsFlushResolverCache = dnsapi.NewProc("DnsFlushResolverCache")
	powershellTimeout         = 60 * time.Second
)

func newDNSRedirect() (DNSRedirect, error) { return winDNS{}, nil }

type winDNS struct{}

// powershell runs one of the DnsClient cmdlets, which notify the DNS client
// of the change (editing the registry directly would not).
func powershell(script string) error {
	ctx, cancel := context.WithTimeout(context.Background(), powershellTimeout)
	defer cancel()
	exe := filepath.Join(windowsDir(), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	out, err := exec.CommandContext(ctx, exe, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-Command", "$ErrorActionPreference='Stop'; "+script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func windowsDir() string {
	if d, err := windows.GetWindowsDirectory(); err == nil {
		return d
	}
	return `C:\Windows`
}

func (winDNS) Apply(server string) error {
	if net.ParseIP(server) == nil {
		return fmt.Errorf("nrpt: %q is not an IP address", server)
	}
	if err := powershell(fmt.Sprintf("Add-DnsClientNrptRule -Namespace '.' -NameServers '%s' -Comment '%s'", server, nrptComment)); err != nil {
		return fmt.Errorf("nrpt rule: %w", err)
	}
	return nil
}

func (winDNS) Clear() error {
	script := fmt.Sprintf("Get-DnsClientNrptRule | Where-Object { $_.Comment -eq '%s' } | "+
		"ForEach-Object { Remove-DnsClientNrptRule -Name $_.Name -Force }", nrptComment)
	if err := powershell(script); err != nil {
		return fmt.Errorf("nrpt clear: %w", err)
	}
	return nil
}

func (winDNS) Flush() error {
	if err := procDnsFlushResolverCache.Find(); err != nil {
		return fmt.Errorf("dns flush: %w", err)
	}
	if r, _, err := procDnsFlushResolverCache.Call(); r == 0 {
		return fmt.Errorf("dns flush: %w", err)
	}
	return nil
}

func (winDNS) Upstreams() (DNSConfig, error) {
	size := uint32(15 << 10)
	var buf []byte
	for range 3 {
		buf = make([]byte, size)
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, windows.GAA_FLAG_SKIP_ANYCAST|windows.GAA_FLAG_SKIP_MULTICAST,
			0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])), &size)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			return DNSConfig{}, fmt.Errorf("adapters: %w", err)
		}
	}
	var cfg DNSConfig
	seen := map[string]bool{}
	add := func(list *[]string, s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			*list = append(*list, s)
		}
	}
	for a := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); a != nil; a = a.Next {
		if a.OperStatus != windows.IfOperStatusUp || a.IfType == windows.IF_TYPE_SOFTWARE_LOOPBACK {
			continue
		}
		for d := a.FirstDnsServerAddress; d != nil; d = d.Next {
			ip := d.Address.IP()
			if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || isSiteLocalDNS(ip) {
				continue
			}
			add(&cfg.Servers, ip.String())
		}
		if a.DnsSuffix != nil {
			add(&cfg.Suffixes, strings.ToLower(windows.UTF16PtrToString(a.DnsSuffix)))
		}
	}
	return cfg, nil
}

// isSiteLocalDNS drops fec0:0:0:ffff::1-3, the IPv6 placeholders Windows
// lists on adapters without a real IPv6 DNS server.
func isSiteLocalDNS(ip net.IP) bool {
	return ip.To4() == nil && len(ip) == net.IPv6len && ip[0] == 0xfe && ip[1]&0xc0 == 0xc0
}

// --- Browser DNS over HTTPS -----------------------------------------------------------

// dohPolicies are the machine policies that turn DNS over HTTPS off.
var dohPolicies = []struct {
	key, name, kind string
	str             string
	num             uint32
}{
	{key: `SOFTWARE\Policies\Google\Chrome`, name: "DnsOverHttpsMode", kind: "sz", str: "off"},
	{key: `SOFTWARE\Policies\Microsoft\Edge`, name: "DnsOverHttpsMode", kind: "sz", str: "off"},
	{key: `SOFTWARE\Policies\Mozilla\Firefox\DNSOverHTTPS`, name: "Enabled", kind: "dword", num: 0},
}

func newBrowserDoH() (BrowserDoH, error) { return winDoH{}, nil }

type winDoH struct{}

func (winDoH) Snapshot() ([]SavedSetting, error) {
	var out []SavedSetting
	for _, p := range dohPolicies {
		s := SavedSetting{Key: p.key, Name: p.name, Created: missingKeys(p.key)}
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, p.key, registry.QUERY_VALUE)
		if err == nil {
			if v, _, err := k.GetStringValue(p.name); err == nil {
				s.Existed, s.Kind, s.Str = true, "sz", v
			} else if v, _, err := k.GetIntegerValue(p.name); err == nil {
				s.Existed, s.Kind, s.Num = true, "dword", uint32(v)
			}
			k.Close()
		}
		out = append(out, s)
	}
	return out, nil
}

// missingKeys lists the keys of path that do not exist yet, parents first.
func missingKeys(path string) []string {
	parts := strings.Split(path, `\`)
	var missing []string
	for i := range parts {
		p := strings.Join(parts[:i+1], `\`)
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, p, registry.QUERY_VALUE)
		if err != nil {
			missing = append(missing, p)
			continue
		}
		k.Close()
	}
	return missing
}

func (winDoH) Disable() error {
	for _, p := range dohPolicies {
		k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, p.key, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("browser policy %s: %w", p.key, err)
		}
		if p.kind == "sz" {
			err = k.SetStringValue(p.name, p.str)
		} else {
			err = k.SetDWordValue(p.name, p.num)
		}
		k.Close()
		if err != nil {
			return fmt.Errorf("browser policy %s\\%s: %w", p.key, p.name, err)
		}
	}
	return nil
}

func (winDoH) Restore(saved []SavedSetting) error {
	var errs []error
	for _, s := range saved {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, s.Key, registry.SET_VALUE)
		if err == nil {
			switch {
			case !s.Existed:
				if err := k.DeleteValue(s.Name); err != nil && !errors.Is(err, registry.ErrNotExist) {
					errs = append(errs, err)
				}
			case s.Kind == "sz":
				errs = append(errs, k.SetStringValue(s.Name, s.Str))
			default:
				errs = append(errs, k.SetDWordValue(s.Name, s.Num))
			}
			k.Close()
		}
		// Delete the keys created for it, deepest first, only if left empty.
		for i := len(s.Created) - 1; i >= 0; i-- {
			if err := deleteIfEmpty(s.Created[i]); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("browser policies: %w", err)
	}
	return nil
}

func deleteIfEmpty(path string) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	info, err := k.Stat()
	k.Close()
	if err != nil || info.SubKeyCount > 0 || info.ValueCount > 0 {
		return err // in use by someone else: leave it
	}
	if err := registry.DeleteKey(registry.LOCAL_MACHINE, path); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}
