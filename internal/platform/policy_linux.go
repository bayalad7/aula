//go:build linux

package platform

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// The class restrictions on Linux (docs/privacidad.md rule 105, D-025): the
// student's app processes, an nftables table that takes DNS to Aula's
// resolver, and the browsers' policies against DNS over HTTPS. Only the
// service (root) uses them.

// procRoot is /proc; tests point it at a fake tree.
var procRoot = "/proc"

func newProcessControl() (ProcessControl, error) {
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("processes: %w (the service runs as root)", ErrNotSupported)
	}
	return &linuxProcesses{uids: map[uint32]int{}}, nil
}

type linuxProcesses struct {
	mu   sync.Mutex
	uids map[uint32]int // logind session → its user's UID
}

// List returns the app processes of the session's user. Only apps are
// candidates (rule 105): in GNOME the ones under app.slice, elsewhere those
// of the session's scope; never the desktop, its services, what lives under
// /usr/lib or /usr/libexec, nor a desktop core program.
func (p *linuxProcesses) List(session uint32) ([]ProcessInfo, error) {
	uid, err := p.uidOf(session)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, fmt.Errorf("processes: %w", err)
	}
	var out []ProcessInfo
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if info, ok := readProc(pid, uid, session); ok {
			out = append(out, info)
		}
	}
	return out, nil
}

func (p *linuxProcesses) uidOf(session uint32) (int, error) {
	p.mu.Lock()
	uid, ok := p.uids[session]
	p.mu.Unlock()
	if ok {
		return uid, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	info, good, err := readSession(ctx, strconv.FormatUint(uint64(session), 10))
	if err != nil {
		return 0, err
	}
	if !good {
		return 0, fmt.Errorf("processes: session %d is not a user session", session)
	}
	p.mu.Lock()
	p.uids[session] = info.UID
	p.mu.Unlock()
	return info.UID, nil
}

// readProc reads one process; ok only for an app of the user in the session.
func readProc(pid, uid int, session uint32) (ProcessInfo, bool) {
	dir := filepath.Join(procRoot, strconv.Itoa(pid))
	if procUID(dir) != uid {
		return ProcessInfo{}, false
	}
	cg, err := os.ReadFile(filepath.Join(dir, "cgroup"))
	if err != nil || !appCgroup(string(cg), session) {
		return ProcessInfo{}, false
	}
	exe, _ := os.Readlink(filepath.Join(dir, "exe"))
	exe = strings.TrimSuffix(exe, " (deleted)")
	comm := strings.TrimSpace(readSmall(filepath.Join(dir, "comm")))
	name := processName(exe, comm)
	if name == "" || desktopCore(name) || systemPath(exe) {
		return ProcessInfo{}, false
	}
	return ProcessInfo{PID: pid, Name: name, Path: exe}, true
}

// procUID is the real UID of the process (the "Uid:" line of status), -1 if
// it cannot be read.
func procUID(dir string) int {
	f, err := os.Open(filepath.Join(dir, "status"))
	if err != nil {
		return -1
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if v, ok := strings.CutPrefix(s.Text(), "Uid:"); ok {
			f := strings.Fields(v)
			if len(f) > 0 {
				if n, err := strconv.Atoi(f[0]); err == nil {
					return n
				}
			}
		}
	}
	return -1
}

// appCgroup: the process is an app the student opened. GNOME starts every
// app in its own scope under app.slice; other desktops leave them in the
// session's scope. The desktop's own services live in session.slice.
func appCgroup(cgroup string, session uint32) bool {
	for _, line := range strings.Split(cgroup, "\n") {
		_, path, ok := strings.Cut(line, "::")
		if !ok {
			continue // cgroup v1 lines: not used by modern desktops
		}
		if strings.Contains(path, "/session.slice/") {
			return false
		}
		if strings.Contains(path, "/app.slice/") || strings.Contains(path, fmt.Sprintf("/session-%d.scope", session)) {
			return true
		}
	}
	return false
}

// interpreters run a script whose name is the program's: a Python app such
// as Thonny shows as its script (comm), not as "python3".
var interpreters = []string{"python", "perl", "ruby", "node", "java", "bash", "sh", "dash", "lua"}

// processName is the name the policy matches: the executable's, or the
// script's (comm) for an interpreter.
func processName(exe, comm string) string {
	base := filepath.Base(exe)
	if exe == "" {
		return comm
	}
	for _, i := range interpreters {
		if base == i || strings.HasPrefix(base, i+"3") || strings.HasPrefix(base, i+"2") {
			if comm != "" {
				return comm
			}
		}
	}
	return base
}

// coreNames are the desktop's own programs: never closed, even if listed
// or missing from an allow list.
var coreNames = map[string]bool{
	"gnome-shell": true, "gnome-session-binary": true, "gnome-session": true, "gnome-session-c": true,
	"Xorg": true, "Xwayland": true, "gdm-x-session": true, "gdm-wayland-session": true,
	"nautilus": true, "nemo": true, "thunar": true, "pcmanfm": true, "dolphin": true,
	"dbus-daemon": true, "dbus-broker": true, "dbus-broker-launch": true, "systemd": true,
	"pipewire": true, "pipewire-pulse": true, "wireplumber": true, "pulseaudio": true,
	"xfce4-session": true, "xfce4-panel": true, "xfwm4": true, "xfdesktop": true, "xfsettingsd": true,
	"plasmashell": true, "kwin_x11": true, "kwin_wayland": true, "ksmserver": true,
	"cinnamon": true, "cinnamon-session": true, "mate-session": true, "mate-panel": true, "marco": true,
	"lxsession": true, "lxpanel": true, "openbox": true, "aula-cliente": true,
}

var corePrefixes = []string{"gsd-", "ibus", "gvfs", "xdg-desktop-portal", "xdg-document-portal",
	"xdg-permission-store", "at-spi", "evolution-", "tracker-", "goa-", "gnome-keyring"}

func desktopCore(name string) bool {
	if coreNames[name] {
		return true
	}
	for _, p := range corePrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// systemPath: helpers installed under /usr/lib, /usr/libexec or /lib (the
// equivalent of %SystemRoot% on Windows).
func systemPath(exe string) bool {
	for _, p := range []string{"/usr/lib/", "/usr/libexec/", "/lib/", "/lib64/", "/usr/lib64/"} {
		if strings.HasPrefix(exe, p) {
			return true
		}
	}
	return false
}

func readSmall(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	b, _ := io.ReadAll(io.LimitReader(f, 4096))
	return string(b)
}

// Kill asks the app to end (SIGTERM), like closing its window.
func (p *linuxProcesses) Kill(pid int) error {
	if pid <= 1 {
		return fmt.Errorf("kill: pid %d", pid)
	}
	err := syscall.Kill(pid, syscall.SIGTERM)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("kill: %w", err)
	}
	return nil
}

// --- DNS: an nftables table -------------------------------------------------

// nftTable is Aula's table; Clear deletes it whole. It is never saved to
// disk, so a reboot always removes it (rule 105).
const nftTable = "aula"

// resolvConfs list the real upstream servers: systemd-resolved's first.
var resolvConfs = []string{"/run/systemd/resolve/resolv.conf", "/etc/resolv.conf"}

func newDNSRedirect() (DNSRedirect, error) {
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("dns: %w (the service runs as root)", ErrNotSupported)
	}
	if _, err := exec.LookPath("nft"); err != nil {
		return nil, fmt.Errorf("dns: no nft: %w", ErrNotSupported)
	}
	return linuxDNS{}, nil
}

type linuxDNS struct{}

// nftRules takes the DNS of every program but root's (Aula's resolver and
// root services) to server:53, refuses DNS over IPv6 (the resolver listens on
// IPv4) and the system's DNS over TLS (853).
func nftRules(server string) string {
	return fmt.Sprintf(`table inet %[1]s {
	chain redirigir {
		type nat hook output priority -100; policy accept;
		meta skuid 0 return
		meta nfproto ipv4 udp dport 53 dnat ip to %[2]s:53
		meta nfproto ipv4 tcp dport 53 dnat ip to %[2]s:53
	}
	chain cortar {
		type filter hook output priority 0; policy accept;
		meta skuid 0 return
		meta nfproto ipv6 udp dport 53 reject
		meta nfproto ipv6 tcp dport 53 reject
		tcp dport 853 reject
		udp dport 853 reject
	}
}
`, nftTable, server)
}

// runNft feeds a script to nft; replaced in tests.
var runNft = func(script string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nft", "-f", "-")
	cmd.Stdin = strings.NewReader(script)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nft: %w: %s", err, strings.TrimSpace(errOut.String()))
	}
	return nil
}

func (linuxDNS) Apply(server string) error {
	ip := net.ParseIP(server)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("dns: %q is not an IPv4 address", server)
	}
	// Delete-then-add in one transaction: a table left by a crash is
	// replaced, never doubled.
	script := "table inet " + nftTable + "\ndelete table inet " + nftTable + "\n" + nftRules(ip.String())
	return runNft(script)
}

func (linuxDNS) Clear() error {
	// "table" creates it if missing, so the delete never fails for that.
	return runNft("table inet " + nftTable + "\ndelete table inet " + nftTable + "\n")
}

func (linuxDNS) Flush() error {
	if _, err := exec.LookPath("resolvectl"); err != nil {
		return nil // no systemd-resolved: no system cache to empty
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := runCommand(ctx, "resolvectl", "flush-caches")
	return err
}

func (linuxDNS) Upstreams() (DNSConfig, error) {
	for _, p := range resolvConfs {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		cfg := parseResolvConf(f)
		f.Close()
		if len(cfg.Servers) > 0 {
			return cfg, nil
		}
	}
	return DNSConfig{}, fmt.Errorf("dns: no upstream servers")
}

// parseResolvConf reads nameserver and search lines, without loopback
// servers (systemd-resolved's stub, or Aula's own resolver).
func parseResolvConf(r io.Reader) DNSConfig {
	var cfg DNSConfig
	s := bufio.NewScanner(r)
	for s.Scan() {
		f := strings.Fields(s.Text())
		if len(f) < 2 || strings.HasPrefix(f[0], "#") {
			continue
		}
		switch f[0] {
		case "nameserver":
			host, _, _ := strings.Cut(f[1], "%") // fe80::1%eth0
			if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() {
				cfg.Servers = append(cfg.Servers, f[1])
			}
		case "search", "domain":
			for _, d := range f[1:] {
				if d != "." {
					cfg.Suffixes = append(cfg.Suffixes, d)
				}
			}
		}
	}
	return cfg
}

// --- Browsers: DNS over HTTPS off --------------------------------------------

// browserPolicy is one policy file. Chromium-like browsers read every JSON
// in their managed folder, so Aula writes its own file; Firefox has one
// policies.json, which is merged and later given back byte for byte.
type browserPolicy struct {
	path    string
	firefox bool
}

// browserPolicies are replaced in tests.
var browserPolicies = []browserPolicy{
	{path: "/etc/opt/chrome/policies/managed/aula-doh.json"},
	{path: "/etc/chromium/policies/managed/aula-doh.json"},
	{path: "/etc/chromium-browser/policies/managed/aula-doh.json"},
	{path: "/etc/opt/edge/policies/managed/aula-doh.json"},
	{path: "/etc/firefox/policies/policies.json", firefox: true},
}

const chromiumNoDoH = `{"DnsOverHttpsMode": "off"}` + "\n"

func newBrowserDoH() (BrowserDoH, error) {
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("browser policies: %w (the service runs as root)", ErrNotSupported)
	}
	return linuxDoH{}, nil
}

type linuxDoH struct{}

// Snapshot keeps each file's exact content (Kind "file") or that it did not
// exist, and the folders that would have to be created for it.
func (linuxDoH) Snapshot() ([]SavedSetting, error) {
	var out []SavedSetting
	for _, b := range browserPolicies {
		s := SavedSetting{Key: b.path, Kind: "file", Created: missingDirs(filepath.Dir(b.path))}
		data, err := os.ReadFile(b.path)
		switch {
		case err == nil:
			s.Existed, s.Str = true, string(data)
		case !errors.Is(err, os.ErrNotExist):
			return nil, fmt.Errorf("browser policies: %w", err)
		}
		out = append(out, s)
	}
	return out, nil
}

// missingDirs are the folders of dir that do not exist, parents first.
func missingDirs(dir string) []string {
	var out []string
	for d := dir; d != "/" && d != "."; d = filepath.Dir(d) {
		if _, err := os.Stat(d); err == nil {
			break
		}
		out = append([]string{d}, out...)
	}
	return out
}

func (linuxDoH) Disable() error {
	for _, b := range browserPolicies {
		data := []byte(chromiumNoDoH)
		if b.firefox {
			old, err := os.ReadFile(b.path)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("browser policies: %w", err)
			}
			if data, err = firefoxNoDoH(old); err != nil {
				return fmt.Errorf("browser policies: %s: %w", b.path, err)
			}
		}
		if err := os.MkdirAll(filepath.Dir(b.path), 0o755); err != nil {
			return fmt.Errorf("browser policies: %w", err)
		}
		if err := writeFileAtomic(b.path, data, 0o644); err != nil {
			return fmt.Errorf("browser policies: %w", err)
		}
	}
	return nil
}

// firefoxNoDoH sets policies.DNSOverHTTPS (off and locked) in the content of
// a policies.json, keeping every other policy.
func firefoxNoDoH(old []byte) ([]byte, error) {
	doc := map[string]any{}
	if len(bytes.TrimSpace(old)) > 0 {
		if err := json.Unmarshal(old, &doc); err != nil {
			return nil, fmt.Errorf("not valid JSON: %w", err)
		}
	}
	pol, _ := doc["policies"].(map[string]any)
	if pol == nil {
		pol = map[string]any{}
	}
	pol["DNSOverHTTPS"] = map[string]any{"Enabled": false, "Locked": true}
	doc["policies"] = pol
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func (linuxDoH) Restore(saved []SavedSetting) error {
	var errs []error
	for _, s := range saved {
		if s.Kind != "file" || !filepath.IsAbs(s.Key) {
			continue // not a Linux entry
		}
		if s.Existed {
			if err := writeFileAtomic(s.Key, []byte(s.Str), 0o644); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if err := os.Remove(s.Key); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
		for i := len(s.Created) - 1; i >= 0; i-- {
			_ = os.Remove(s.Created[i]) // only if empty: another file keeps it
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("browser policies: %w", err)
	}
	return nil
}

// writeFileAtomic replaces path through a temporary file in its folder.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".aula-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, werr := tmp.Write(data)
	if werr == nil {
		werr = tmp.Chmod(mode)
	}
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return werr
	}
	return os.Rename(tmp.Name(), path)
}
