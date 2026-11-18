//go:build linux

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// The student's graphical session on Linux, from logind (D-024). Session.ID
// is logind's numeric session ID and Session.SID the user's UID: the local
// channel is opened for that UID only.

func newSessionManager() (SessionManager, error) {
	if _, err := exec.LookPath("loginctl"); err != nil {
		return nil, fmt.Errorf("sessions: no loginctl: %w", ErrNotSupported)
	}
	return logindSessions{}, nil
}

type logindSessions struct{}

func (logindSessions) Active() (Session, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return activeSession(ctx)
}

// sessionInfo is what Aula reads of a logind session.
type sessionInfo struct {
	ID      uint32
	UID     int
	User    string
	Type    string // x11, wayland, tty…
	Display string // the X display of an X11 session, e.g. ":0"
}

// activeSession is the active, local, graphical user session of seat0:
// what the student sees. At the login screen (a greeter), on a text console
// or over SSH there is none.
func activeSession(ctx context.Context) (Session, bool, error) {
	out, err := runCommand(ctx, "loginctl", "show-seat", "seat0", "-p", "ActiveSession")
	if err != nil {
		// No seat0 (a server or a container): nobody to show anything to.
		return Session{}, false, nil
	}
	id := keyValues(out)["ActiveSession"]
	if id == "" {
		return Session{}, false, nil
	}
	info, ok, err := readSession(ctx, id)
	if err != nil || !ok {
		return Session{}, false, err
	}
	return Session{ID: info.ID, User: info.User, SID: strconv.Itoa(info.UID)}, true, nil
}

// readSession reads one session; ok is false unless it is a local graphical
// session of a user.
func readSession(ctx context.Context, id string) (sessionInfo, bool, error) {
	out, err := runCommand(ctx, "loginctl", "show-session", id,
		"-p", "Id", "-p", "Name", "-p", "User", "-p", "Type", "-p", "Class", "-p", "Remote", "-p", "Display", "-p", "State")
	if err != nil {
		return sessionInfo{}, false, fmt.Errorf("sessions: %w", err)
	}
	return parseSession(out)
}

func parseSession(out string) (sessionInfo, bool, error) {
	p := keyValues(out)
	if p["Class"] != "user" || p["Remote"] == "yes" || p["State"] == "closing" {
		return sessionInfo{}, false, nil
	}
	if p["Type"] != "x11" && p["Type"] != "wayland" {
		return sessionInfo{}, false, nil // a text console: no window to show
	}
	n, err := strconv.ParseUint(p["Id"], 10, 32)
	if err != nil {
		return sessionInfo{}, false, nil // logind's non-numeric IDs are greeters
	}
	uid, err := strconv.Atoi(p["User"])
	if err != nil || uid == 0 || p["Name"] == "" {
		return sessionInfo{}, false, nil // never root's session
	}
	return sessionInfo{ID: uint32(n), UID: uid, User: p["Name"], Type: p["Type"], Display: p["Display"]}, true, nil
}

// helperEnvKeys are the variables of the user's session the helper gets:
// what a graphical app needs, nothing else.
var helperEnvKeys = map[string]bool{
	"DISPLAY": true, "XAUTHORITY": true, "WAYLAND_DISPLAY": true, "XDG_RUNTIME_DIR": true,
	"DBUS_SESSION_BUS_ADDRESS": true, "XDG_CURRENT_DESKTOP": true, "XDG_SESSION_DESKTOP": true,
	"DESKTOP_SESSION": true, "XDG_DATA_DIRS": true, "XDG_CONFIG_DIRS": true, "LANG": true,
	"LANGUAGE": true, "GTK_MODULES": true, "QT_IM_MODULE": true, "GTK_IM_MODULE": true,
	"XMODIFIERS": true,
}

func (logindSessions) Launch(s Session, exe string, args []string) (Process, error) {
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("launch helper: %w", errNotRoot)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	info, ok, err := readSession(ctx, strconv.FormatUint(uint64(s.ID), 10))
	if err != nil {
		return nil, err
	}
	if !ok || strconv.Itoa(info.UID) != s.SID {
		return nil, fmt.Errorf("launch helper: session %d is gone", s.ID)
	}
	u, err := user.LookupId(s.SID)
	if err != nil {
		return nil, fmt.Errorf("launch helper: %w", err)
	}
	// The user's systemd manager has the session's environment: GNOME and
	// the other desktops import DISPLAY, XAUTHORITY and the rest into it.
	userEnv, _ := runCommand(ctx, "systemctl", "--user", "-M", u.Username+"@", "show-environment")
	env := helperEnv(info, u, userEnv)
	if info.Type == "x11" && !hasEnv(env, "DISPLAY") {
		return nil, fmt.Errorf("launch helper: session %d has no X display", s.ID)
	}
	cred, err := credentialOf(u)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = env
	cmd.Dir = u.HomeDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: cred, Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch helper: %w", err)
	}
	p := &linuxProcess{cmd: cmd, done: make(chan struct{})}
	go func() {
		p.err = cmd.Wait()
		close(p.done)
	}()
	return p, nil
}

var errNotRoot = errors.New("the service must run as root")

// helperEnv builds the helper's environment from the user's manager
// ("KEY=value" lines), keeping only helperEnvKeys, plus what identifies the
// user and the session.
func helperEnv(info sessionInfo, u *user.User, userEnv string) []string {
	vals := map[string]string{}
	for _, line := range strings.Split(userEnv, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || !helperEnvKeys[k] || strings.HasPrefix(v, "$'") {
			continue // systemd quotes odd values as $'…': not needed by a GUI
		}
		vals[k] = v
	}
	runtime := "/run/user/" + strconv.Itoa(info.UID)
	if vals["XDG_RUNTIME_DIR"] == "" {
		vals["XDG_RUNTIME_DIR"] = runtime
	}
	if vals["DBUS_SESSION_BUS_ADDRESS"] == "" {
		vals["DBUS_SESSION_BUS_ADDRESS"] = "unix:path=" + runtime + "/bus"
	}
	if vals["DISPLAY"] == "" && info.Display != "" {
		vals["DISPLAY"] = info.Display
	}
	vals["XDG_SESSION_TYPE"] = info.Type
	vals["XDG_SESSION_ID"] = strconv.FormatUint(uint64(info.ID), 10)
	vals["HOME"], vals["USER"], vals["LOGNAME"] = u.HomeDir, u.Username, u.Username
	vals["PATH"] = "/usr/local/bin:/usr/bin:/bin"
	env := make([]string, 0, len(vals))
	for k, v := range vals {
		env = append(env, k+"="+v)
	}
	return env
}

func hasEnv(env []string, key string) bool {
	for _, kv := range env {
		if strings.HasPrefix(kv, key+"=") && len(kv) > len(key)+1 {
			return true
		}
	}
	return false
}

// credentialOf runs a process as u, with its primary and extra groups: the
// helper has the student's rights and none more.
func credentialOf(u *user.User) (*syscall.Credential, error) {
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("launch helper: uid %q: %w", u.Uid, err)
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("launch helper: gid %q: %w", u.Gid, err)
	}
	cred := &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	if ids, err := u.GroupIds(); err == nil {
		for _, g := range ids {
			if n, err := strconv.ParseUint(g, 10, 32); err == nil {
				cred.Groups = append(cred.Groups, uint32(n))
			}
		}
	}
	return cred, nil
}

type linuxProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
	once sync.Once
}

func (p *linuxProcess) PID() int { return p.cmd.Process.Pid }

func (p *linuxProcess) Wait(ctx context.Context) error {
	select {
	case <-p.done:
		return p.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Kill asks the helper to end (SIGTERM) and kills it after 3 s.
func (p *linuxProcess) Kill() error {
	var err error
	p.once.Do(func() {
		err = p.cmd.Process.Signal(syscall.SIGTERM)
		go func() {
			select {
			case <-p.done:
			case <-time.After(3 * time.Second):
				_ = p.cmd.Process.Kill()
			}
		}()
	})
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

// currentSession is this process's logind session: XDG_SESSION_ID, which the
// desktop (and the service, for its helper) sets.
func currentSession() (uint32, error) {
	n, err := strconv.ParseUint(os.Getenv("XDG_SESSION_ID"), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("current session: no numeric XDG_SESSION_ID: %w", ErrNotSupported)
	}
	return uint32(n), nil
}

// documentsDirOf is the Documents folder of the user of a session, read from
// their user-dirs.dirs as xdg-user-dir would.
func documentsDirOf(session uint32) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	info, ok, err := readSession(ctx, strconv.FormatUint(uint64(session), 10))
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("documents of session %d: not a user session", session)
	}
	u, err := user.LookupId(strconv.Itoa(info.UID))
	if err != nil {
		return "", fmt.Errorf("documents of session %d: %w", session, err)
	}
	if f, err := os.Open(filepath.Join(u.HomeDir, ".config", "user-dirs.dirs")); err == nil {
		defer f.Close()
		if dir, ok := xdgUserDir(f, "DOCUMENTS", u.HomeDir); ok {
			return dir, nil
		}
	}
	return filepath.Join(u.HomeDir, "Documents"), nil
}
