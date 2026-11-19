//go:build linux

package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXDGLauncher(t *testing.T) {
	l := xdgLauncher{}
	if _, err := l.Folders(); !errors.Is(err, ErrNotSupported) {
		t.Errorf("Folders = %v, want ErrNotSupported (no app rules on Linux yet)", err)
	}
	if err := l.StartApp("/usr/bin/true"); !errors.Is(err, ErrNotSupported) {
		t.Errorf("StartApp = %v, want ErrNotSupported", err)
	}
	// Never handed to xdg-open: it is not a web page.
	for _, u := range []string{"file:///etc/passwd", "-h", "javascript:x"} {
		if err := l.OpenURL(u); err == nil {
			t.Errorf("OpenURL(%q) accepted", u)
		}
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(f, filepath.Join(dir, "enlace")); err != nil {
		t.Fatal(err)
	}
	if got, err := l.Resolve(filepath.Join(dir, "enlace")); err != nil || got != f {
		t.Errorf("Resolve = %q, %v; want %q", got, err, f)
	}
	if _, err := l.Resolve(dir); err == nil {
		t.Error("a folder resolved as a file")
	}
}

func TestParseUFWStatus(t *testing.T) {
	out := `Status: active

To                         Action      From
--                         ------      ----
22/tcp                     ALLOW       Anywhere
47830/tcp                  ALLOW       Anywhere                   # Aula Consola
5353/udp                   ALLOW       Anywhere
47830/tcp (v6)             ALLOW       Anywhere (v6)
`
	fw, rules := parseUFWStatus(out, 47830)
	if !fw.Enabled || fw.InboundAllowed {
		t.Errorf("profile = %+v", fw)
	}
	if len(rules) != 2 || !rules[0].Allow {
		t.Errorf("rules = %+v", rules)
	}
	fw, rules = parseUFWStatus("Status: active\n\n8080/tcp DENY Anywhere\n", 47830)
	if !fw.Enabled || len(rules) != 0 {
		t.Errorf("no rule for the port: %+v %+v", fw, rules)
	}
	fw, _ = parseUFWStatus("Status: inactive\n", 47830)
	if fw.Enabled {
		t.Error("inactive read as active")
	}
	fw, rules = parseUFWStatus("Status: active\n47830 DENY Anywhere\n", 47830)
	if len(rules) != 1 || rules[0].Allow {
		t.Errorf("deny rule: %+v", rules)
	}
}

func TestUFWConf(t *testing.T) {
	for in, want := range map[string]bool{
		"# comment\nENABLED=yes\n": true, "ENABLED=no\n": false, `ENABLED="yes"`: true, "": false,
	} {
		if got := ufwConfEnabled(strings.NewReader(in)); got != want {
			t.Errorf("ufwConfEnabled(%q) = %v", in, got)
		}
	}
}

// With ufw on and its rules unreadable (a teacher is not root), the checker
// is told it cannot judge, never that all is well.
func TestInspectUFWNeedsRoot(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "ufw.conf")
	if err := os.WriteFile(conf, []byte("ENABLED=yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := ufwConf
	ufwConf = conf
	t.Cleanup(func() { ufwConf = old })
	fakeCommands(t, map[string]string{}) // "ufw status" fails, as for a user
	if _, err := (linuxNetwork{}).Inspect(context.Background(), "", 47830); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("Inspect = %v, want ErrNotSupported", err)
	}
	// Neither ufw nor firewalld: no firewall in the way.
	ufwConf = filepath.Join(t.TempDir(), "none")
	st, err := (linuxNetwork{}).Inspect(context.Background(), "", 47830)
	if err != nil || len(st.Firewall) != 1 || st.Firewall[0].Enabled {
		t.Fatalf("no firewall = %+v, %v", st, err)
	}
}
