//go:build linux

package platform

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestUnitName(t *testing.T) {
	for in, want := range map[string]string{
		"AulaAgente":       "aula-agente.service",
		"AulaAgentePrueba": "aula-agente-prueba.service",
		"aula x":           "aula-x.service",
	} {
		if got := unitName(in); got != want {
			t.Errorf("unitName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnitFile(t *testing.T) {
	u := unitFile(ServiceConfig{Name: "AulaAgente", DisplayName: "Aula — agente del aula",
		Description: "Conecta este equipo.\nVarias líneas.", Exe: "/usr/bin/aula-cliente",
		Args: []string{"servicio", "ejecutar", "-nombre", "Aula 100%"}})
	for _, want := range []string{
		"Description=Aula — agente del aula\n",
		`ExecStart="/usr/bin/aula-cliente" "servicio" "ejecutar" "-nombre" "Aula 100%%"` + "\n",
		"Restart=on-failure\n", "RestartSec=5\n", "WantedBy=multi-user.target\n",
		"# Conecta este equipo. Varias líneas.\n",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("unit lacks %q:\n%s", want, u)
		}
	}
	if q := systemdQuote(`a"b\c`); q != `"a\"b\\c"` {
		t.Errorf("systemdQuote = %s", q)
	}
}

func TestParseUnitStatus(t *testing.T) {
	for _, tc := range []struct {
		out  string
		want ServiceStatus
	}{
		{"LoadState=not-found\nActiveState=inactive\n", ServiceStatus{State: ServiceNotInstalled}},
		{"LoadState=loaded\nActiveState=active\nMainPID=812\nUnitFileState=enabled\nRestart=on-failure\n",
			ServiceStatus{State: ServiceRunning, PID: 812, Auto: true, RestartOnFailure: true}},
		{"LoadState=loaded\nActiveState=failed\nMainPID=0\nUnitFileState=disabled\nRestart=no\n",
			ServiceStatus{State: ServiceStopped}},
		{"LoadState=loaded\nActiveState=activating\n", ServiceStatus{State: ServiceStartPending}},
	} {
		if got := parseUnitStatus(tc.out); got != tc.want {
			t.Errorf("parseUnitStatus(%q) = %+v, want %+v", tc.out, got, tc.want)
		}
	}
}

// fakeCommands replaces runCommand: answers maps "name args" to output.
func fakeCommands(t *testing.T, answers map[string]string) *[]string {
	t.Helper()
	var calls []string
	old := runCommand
	runCommand = func(_ context.Context, name string, args ...string) (string, error) {
		line := name + " " + strings.Join(args, " ")
		calls = append(calls, line)
		for k, v := range answers {
			if strings.HasPrefix(line, k) {
				return v, nil
			}
		}
		return "", errors.New("unexpected: " + line)
	}
	t.Cleanup(func() { runCommand = old })
	return &calls
}

func TestServiceNeedsRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root")
	}
	s := newServiceControl("AulaAgentePrueba")
	for name, err := range map[string]error{
		"Install": s.Install(ServiceConfig{Exe: "/usr/bin/x"}), "Uninstall": s.Uninstall(),
		"Start": s.Start(), "Stop": s.Stop(context.Background()),
	} {
		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("%s as a user = %v, want fs.ErrPermission", name, err)
		}
	}
}

func TestServiceInstallWritesTheUnit(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root (the CI job and the container run it)")
	}
	old := unitDir
	unitDir = t.TempDir()
	t.Cleanup(func() { unitDir = old })
	calls := fakeCommands(t, map[string]string{"systemctl ": ""})
	s := newServiceControl("AulaAgentePrueba")
	if err := s.Install(ServiceConfig{DisplayName: "Prueba", Exe: "/usr/bin/aula-cliente", Args: []string{"servicio"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(unitDir, "aula-agente-prueba.service")); err != nil {
		t.Fatal(err)
	}
	if err := s.Uninstall(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(unitDir, "aula-agente-prueba.service")); err == nil {
		t.Fatal("the unit is still there")
	}
	got := strings.Join(*calls, "\n")
	for _, want := range []string{"systemctl daemon-reload", "systemctl enable aula-agente-prueba.service", "systemctl disable aula-agente-prueba.service"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestParseSession(t *testing.T) {
	ok := "Id=2\nName=ana\nUser=1000\nType=x11\nClass=user\nRemote=no\nDisplay=:0\nState=active\n"
	info, good, err := parseSession(ok)
	if err != nil || !good || info.ID != 2 || info.UID != 1000 || info.User != "ana" || info.Display != ":0" {
		t.Fatalf("parseSession = %+v, %v, %v", info, good, err)
	}
	for name, out := range map[string]string{
		"greeter":  strings.Replace(ok, "Class=user", "Class=greeter", 1),
		"remote":   strings.Replace(ok, "Remote=no", "Remote=yes", 1),
		"tty":      strings.Replace(ok, "Type=x11", "Type=tty", 1),
		"root":     strings.Replace(ok, "User=1000", "User=0", 1),
		"c-id":     strings.Replace(ok, "Id=2", "Id=c1", 1),
		"closing":  strings.Replace(ok, "State=active", "State=closing", 1),
		"no class": "Id=2\n",
	} {
		if _, good, _ := parseSession(out); good {
			t.Errorf("%s: accepted as the student's session", name)
		}
	}
}

func TestActiveSession(t *testing.T) {
	fakeCommands(t, map[string]string{
		"loginctl show-seat seat0": "ActiveSession=3\n",
		"loginctl show-session 3 ": "Id=3\nName=luis\nUser=1001\nType=wayland\nClass=user\nRemote=no\nState=active\n",
	})
	s, ok, err := activeSession(context.Background())
	if err != nil || !ok || s.ID != 3 || s.User != "luis" || s.SID != "1001" {
		t.Fatalf("activeSession = %+v, %v, %v", s, ok, err)
	}
	fakeCommands(t, map[string]string{"loginctl show-seat seat0": "ActiveSession=\n"})
	if _, ok, err := activeSession(context.Background()); ok || err != nil {
		t.Fatalf("at the login screen = %v, %v", ok, err)
	}
}

func TestHelperEnv(t *testing.T) {
	u := &user.User{Uid: "1000", Gid: "1000", Username: "ana", HomeDir: "/home/ana"}
	userEnv := "DISPLAY=:0\nXAUTHORITY=/run/user/1000/gdm/Xauthority\nSSH_AUTH_SOCK=/run/x\n" +
		"LANG=es_MX.UTF-8\nGTK_MODULES=$'gail\\nx'\nPATH=/home/ana/bin:/usr/bin\n"
	env := helperEnv(sessionInfo{ID: 2, UID: 1000, Type: "x11", Display: ":1"}, u, userEnv)
	got := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		got[k] = v
	}
	want := map[string]string{
		"DISPLAY": ":0", "XAUTHORITY": "/run/user/1000/gdm/Xauthority", "LANG": "es_MX.UTF-8",
		"XDG_RUNTIME_DIR": "/run/user/1000", "DBUS_SESSION_BUS_ADDRESS": "unix:path=/run/user/1000/bus",
		"XDG_SESSION_TYPE": "x11", "XDG_SESSION_ID": "2", "HOME": "/home/ana", "USER": "ana",
		"PATH": "/usr/local/bin:/usr/bin:/bin",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	for _, k := range []string{"SSH_AUTH_SOCK", "GTK_MODULES"} {
		if _, ok := got[k]; ok {
			t.Errorf("%s reached the helper", k)
		}
	}
	// Without DISPLAY in the user's manager, logind's.
	env = helperEnv(sessionInfo{ID: 2, UID: 1000, Type: "x11", Display: ":1"}, u, "")
	if !hasEnv(env, "DISPLAY") || !strings.Contains(strings.Join(env, " "), "DISPLAY=:1") {
		t.Errorf("env without the manager's = %v", env)
	}
}

func TestLocalChannel(t *testing.T) {
	old := localDir
	localDir = t.TempDir()
	t.Cleanup(func() { localDir = old })
	if _, err := listenLocal("../fuera", ""); err == nil {
		t.Fatal("a name with a path was accepted")
	}
	me := strconv.Itoa(os.Getuid())
	ln, err := listenLocal("aula-agente-2-0011", me)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(localDir, "aula-agente-2-0011.sock")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("socket mode %v, %v; want 0600", info.Mode().Perm(), err)
	}
	if entries, _ := os.ReadDir(localDir); len(entries) != 1 {
		t.Errorf("leftovers in the folder: %v", entries)
	}
	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err == nil {
			accepted <- c
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := dialLocal(ctx, "aula-agente-2-0011")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	srv := <-accepted
	defer srv.Close()
	if pid, err := peerPID(srv); err != nil || pid != os.Getpid() {
		t.Fatalf("peerPID = %d, %v; want %d", pid, err, os.Getpid())
	}
	ln.Close()
	if _, err := os.Stat(path); err == nil {
		t.Error("the socket file stays after Close")
	}
}

func TestPowerArgsAndLogoff(t *testing.T) {
	if got := strings.Join(powerArgs("poweroff", false), " "); got != "poweroff --no-ask-password --no-wall" {
		t.Errorf("powerArgs = %q", got)
	}
	if got := strings.Join(powerArgs("reboot", true), " "); !strings.HasSuffix(got, " -i") {
		t.Errorf("forced powerArgs = %q", got)
	}
	t.Setenv("XDG_SESSION_ID", "4")
	t.Setenv("XDG_CURRENT_DESKTOP", "") // no desktop: loginctl, as before
	calls := fakeCommands(t, map[string]string{"loginctl terminate-session 4": ""})
	p := systemdPower{}
	if err := p.Logoff(7, false); !errors.Is(err, ErrNotSupported) {
		t.Errorf("Logoff of another session without force = %v", err)
	}
	if err := p.Logoff(4, false); err != nil {
		t.Errorf("Logoff of its own session = %v", err)
	}
	if len(*calls) != 1 {
		t.Errorf("calls = %v", *calls)
	}
}

func TestInterfacesLinux(t *testing.T) {
	nics, err := Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nics {
		if len(n.MAC) != 6 || len(n.Addrs) == 0 {
			t.Errorf("listed %+v", n)
		}
	}
}

// Launch starts the helper as the session's user, with its groups and only
// the session's environment. Needs root and a user "ana" (the container of
// docs/desarrollo/empaquetar-linux.md, and the CI's service job).
func TestLaunchAsTheStudent(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root")
	}
	u, err := user.Lookup("ana")
	if err != nil {
		t.Skip("no user ana")
	}
	fakeCommands(t, map[string]string{
		"loginctl show-session 5 ":                  "Id=5\nName=ana\nUser=" + u.Uid + "\nType=x11\nClass=user\nRemote=no\nDisplay=:7\nState=active\n",
		"systemctl --user -M ana@ show-environment": "LANG=es_MX.UTF-8\nSECRET=x\n",
	})
	// t.TempDir hangs from a 0700 folder of root's: ana could not reach it.
	dir, err := os.MkdirTemp("/tmp", "aula-launch-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "salida")
	p, err := logindSessions{}.Launch(Session{ID: 5, User: "ana", SID: u.Uid}, "/bin/sh",
		[]string{"-c", `echo "$(id -u) $DISPLAY $LANG $XDG_SESSION_ID ${SECRET:-none} $HOME" > ` + out})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(out)
	want := u.Uid + " :7 es_MX.UTF-8 5 none " + u.HomeDir + "\n"
	if string(got) != want {
		t.Fatalf("helper saw %q, want %q", got, want)
	}
	// A session that is no longer the user's is refused.
	if _, err := (logindSessions{}).Launch(Session{ID: 5, User: "ana", SID: "4242"}, "/bin/true", nil); err == nil {
		t.Fatal("launched into a session of another user")
	}
}

func TestSessionLogout(t *testing.T) {
	for desktop, want := range map[string]string{
		"ubuntu:GNOME":        "gnome-session-quit --logout --no-prompt",
		"GNOME-Classic:GNOME": "gnome-session-quit --logout --no-prompt",
		"X-Cinnamon":          "cinnamon-session-quit --logout --no-prompt",
		"MATE":                "mate-session-save --logout",
		"KDE":                 "",
		"XFCE":                "",
		"":                    "",
	} {
		name, args := sessionLogout(desktop)
		if got := strings.TrimSpace(name + " " + strings.Join(args, " ")); got != want {
			t.Errorf("sessionLogout(%q) = %q, want %q", desktop, got, want)
		}
	}
}

// Without force, GNOME's session manager logs out (programs may ask to
// save); loginctl is not used. With force, loginctl ends the session.
func TestLogoffAsksTheSessionManager(t *testing.T) {
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "llamado")
	script := "#!/bin/sh\necho \"$@\" > " + marker + "\n"
	if err := os.WriteFile(filepath.Join(bin, "gnome-session-quit"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CURRENT_DESKTOP", "ubuntu:GNOME")
	t.Setenv("XDG_SESSION_ID", "4")
	calls := fakeCommands(t, map[string]string{"loginctl terminate-session 4": ""})
	p := systemdPower{}
	if err := p.Logoff(4, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(marker); strings.TrimSpace(string(got)) != "--logout --no-prompt" {
		t.Fatalf("gnome-session-quit got %q", got)
	}
	if len(*calls) != 0 {
		t.Fatalf("loginctl was used without force: %v", *calls)
	}
	if err := p.Logoff(4, true); err != nil || len(*calls) != 1 {
		t.Fatalf("with force: %v, calls %v", err, *calls)
	}
}
