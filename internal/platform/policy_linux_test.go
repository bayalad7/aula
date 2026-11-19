//go:build linux

package platform

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppCgroup(t *testing.T) {
	for cg, want := range map[string]bool{
		"0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-gnome-firefox_firefox-4711.scope\n": true,
		"0::/user.slice/user-1000.slice/session-2.scope\n":                                                  true,
		"0::/user.slice/user-1000.slice/session-7.scope\n":                                                  false, // another session
		"0::/user.slice/user-1000.slice/user@1000.service/session.slice/org.gnome.Shell@x11.service\n":      false,
		"0::/system.slice/aula-agente.service\n":                                                            false,
		"12:pids:/user.slice\n":                                                                             false,
	} {
		if got := appCgroup(cg, 2); got != want {
			t.Errorf("appCgroup(%q) = %v, want %v", cg, got, want)
		}
	}
}

func TestProcessName(t *testing.T) {
	for _, tc := range []struct{ exe, comm, want string }{
		{"/opt/google/chrome/chrome", "chrome", "chrome"},
		{"/snap/firefox/5000/usr/lib/firefox/firefox", "Isolated Web Co", "firefox"},
		{"/usr/bin/python3.12", "thonny", "thonny"}, // a Python app goes by its script
		{"/usr/bin/bash", "juego.sh", "juego.sh"},
		{"", "kworker", "kworker"},
	} {
		if got := processName(tc.exe, tc.comm); got != tc.want {
			t.Errorf("processName(%q, %q) = %q, want %q", tc.exe, tc.comm, got, tc.want)
		}
	}
	for _, core := range []string{"gnome-shell", "Xorg", "nautilus", "gsd-power", "ibus-daemon", "aula-cliente"} {
		if !desktopCore(core) {
			t.Errorf("%s is not protected", core)
		}
	}
	for _, app := range []string{"firefox", "gnome-calculator", "code", "thonny", "steam"} {
		if desktopCore(app) {
			t.Errorf("%s is protected but is an app", app)
		}
	}
	if !systemPath("/usr/libexec/gnome-shell-calendar-server") || systemPath("/usr/bin/firefox") {
		t.Error("systemPath")
	}
}

// fakeProc writes a /proc with one process per entry.
func fakeProc(t *testing.T, procs map[string][4]string) {
	t.Helper()
	root := t.TempDir()
	for pid, p := range procs { // uid, cgroup, exe, comm
		dir := filepath.Join(root, pid)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(dir, "status"), []byte("Name:\tx\nUid:\t"+p[0]+"\t"+p[0]+"\t"+p[0]+"\t"+p[0]+"\n"), 0o644)
		_ = os.WriteFile(filepath.Join(dir, "cgroup"), []byte(p[1]+"\n"), 0o644)
		_ = os.WriteFile(filepath.Join(dir, "comm"), []byte(p[3]+"\n"), 0o644)
		if p[2] != "" {
			if err := os.Symlink(p[2], filepath.Join(dir, "exe")); err != nil {
				t.Fatal(err)
			}
		}
	}
	_ = os.MkdirAll(filepath.Join(root, "self"), 0o755) // not a PID
	old := procRoot
	procRoot = root
	t.Cleanup(func() { procRoot = old })
}

func TestListStudentApps(t *testing.T) {
	app := "0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-x.scope"
	fakeProc(t, map[string][4]string{
		"100": {"1000", app, "/opt/google/chrome/chrome", "chrome"},
		"101": {"1000", app, "/usr/bin/python3", "thonny"},
		"102": {"1000", "0::/user.slice/user-1000.slice/user@1000.service/session.slice/org.gnome.Shell@x11.service", "/usr/bin/gnome-shell", "gnome-shell"},
		"103": {"1001", app, "/usr/bin/steam", "steam"},              // another user
		"104": {"1000", app, "/usr/libexec/gsd-xsettings", "gsd-xs"}, // a system helper
		"105": {"1000", app, "/usr/bin/nautilus", "nautilus"},        // the file manager
		"106": {"0", app, "/usr/bin/sudo", "sudo"},                   // root
	})
	p := &linuxProcesses{uids: map[uint32]int{2: 1000}}
	list, err := p.List(2)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, x := range list {
		got[x.Name] = true
	}
	if len(list) != 2 || !got["chrome"] || !got["thonny"] {
		t.Fatalf("List = %+v, want only chrome and thonny", list)
	}
	if err := p.Kill(999999); err != nil {
		t.Errorf("Kill of a process that already exited = %v", err)
	}
	if err := p.Kill(1); err == nil {
		t.Error("Kill(1) accepted")
	}
}

func TestParseResolvConf(t *testing.T) {
	cfg := parseResolvConf(strings.NewReader("# systemd-resolved\nnameserver 127.0.0.53\nnameserver 192.168.1.1\n" +
		"nameserver fe80::1%enp3s0\nnameserver 127.0.0.1\nsearch escuela.edu.mx .\n"))
	if strings.Join(cfg.Servers, ",") != "192.168.1.1,fe80::1%enp3s0" || strings.Join(cfg.Suffixes, ",") != "escuela.edu.mx" {
		t.Fatalf("parseResolvConf = %+v", cfg)
	}
}

func TestNftScripts(t *testing.T) {
	var scripts []string
	old := runNft
	runNft = func(s string) error { scripts = append(scripts, s); return nil }
	t.Cleanup(func() { runNft = old })
	d := linuxDNS{}
	if err := d.Apply("::1"); err == nil {
		t.Error("an IPv6 server was accepted")
	}
	if err := d.Apply("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if err := d.Clear(); err != nil {
		t.Fatal(err)
	}
	apply, clear := scripts[0], scripts[1]
	for _, want := range []string{"delete table inet aula", "dnat ip to 127.0.0.1:53", "meta skuid 0 return", "tcp dport 853 reject", "meta nfproto ipv6 udp dport 53 reject"} {
		if !strings.Contains(apply, want) {
			t.Errorf("Apply lacks %q:\n%s", want, apply)
		}
	}
	if clear != "table inet aula\ndelete table inet aula\n" {
		t.Errorf("Clear = %q", clear)
	}
}

func TestFirefoxNoDoH(t *testing.T) {
	out, err := firefoxNoDoH([]byte(`{"policies": {"DisableTelemetry": true, "DNSOverHTTPS": {"Enabled": true}}}`))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Policies map[string]any `json:"policies"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	doh, _ := doc.Policies["DNSOverHTTPS"].(map[string]any)
	if doc.Policies["DisableTelemetry"] != true || doh["Enabled"] != false || doh["Locked"] != true {
		t.Fatalf("merged = %s", out)
	}
	if _, err := firefoxNoDoH([]byte("{no es json")); err == nil {
		t.Error("an invalid policies.json was overwritten")
	}
}

// Snapshot, Disable and Restore give back every file exactly, delete what
// did not exist and the folders created for it.
func TestBrowserDoHRoundTrip(t *testing.T) {
	root := t.TempDir()
	chrome := filepath.Join(root, "etc/opt/chrome/policies/managed/aula-doh.json")
	firefox := filepath.Join(root, "etc/firefox/policies/policies.json")
	if err := os.MkdirAll(filepath.Dir(firefox), 0o755); err != nil {
		t.Fatal(err)
	}
	original := "{\n  \"policies\": {\"DisableTelemetry\": true}\n}\n"
	if err := os.WriteFile(firefox, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	old := browserPolicies
	browserPolicies = []browserPolicy{{path: chrome}, {path: firefox, firefox: true}}
	t.Cleanup(func() { browserPolicies = old })

	d := linuxDoH{}
	saved, err := d.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Disable(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(chrome); !strings.Contains(string(b), `"DnsOverHttpsMode": "off"`) {
		t.Fatalf("chrome policy = %s", b)
	}
	if b, _ := os.ReadFile(firefox); !strings.Contains(string(b), "DNSOverHTTPS") || !strings.Contains(string(b), "DisableTelemetry") {
		t.Fatalf("firefox policy = %s", b)
	}
	if err := d.Restore(saved); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(firefox); string(b) != original {
		t.Fatalf("firefox not restored exactly: %q", b)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/opt")); err == nil {
		t.Fatal("the folders created for Chrome were left behind")
	}
}

// With root and nft (the container with systemd, the CI's service job):
// the real table goes in and out.
func TestNftTableForReal(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root")
	}
	if _, err := exec.LookPath("nft"); err != nil {
		t.Skip("no nft")
	}
	d := linuxDNS{}
	if err := d.Apply("127.0.0.1"); err != nil {
		t.Skipf("nft cannot load tables here: %v", err)
	}
	t.Cleanup(func() { _ = d.Clear() })
	if err := d.Apply("127.0.0.1"); err != nil { // again: replaced, not doubled
		t.Fatal(err)
	}
	out, err := exec.Command("nft", "list", "table", "inet", "aula").CombinedOutput()
	if err != nil || strings.Count(string(out), "dnat ip to 127.0.0.1:53") != 2 {
		t.Fatalf("table:\n%s %v", out, err)
	}
	if err := d.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := d.Clear(); err != nil {
		t.Fatalf("Clear without a table = %v", err)
	}
	if out, err := exec.Command("nft", "list", "table", "inet", "aula").CombinedOutput(); err == nil {
		t.Fatalf("the table is still there:\n%s", out)
	}
}
