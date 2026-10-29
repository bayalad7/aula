//go:build windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// noSession is what WTSGetActiveConsoleSessionId returns while the console is
// being attached or detached.
const noSession = 0xFFFFFFFF

func newSessionManager() (SessionManager, error) { return winSessions{}, nil }

type winSessions struct{}

// Active returns the console session if a user is logged on. The token of a
// session without a user (logon screen) cannot be obtained: that is "no
// session". Runs only as LocalSystem.
func (winSessions) Active() (Session, bool, error) {
	id := windows.WTSGetActiveConsoleSessionId()
	if id == noSession {
		return Session{}, false, nil
	}
	var tok windows.Token
	if err := windows.WTSQueryUserToken(id, &tok); err != nil {
		if errors.Is(err, windows.ERROR_NO_TOKEN) {
			return Session{}, false, nil
		}
		return Session{}, false, fmt.Errorf("session %d token: %w", id, err)
	}
	defer tok.Close()
	tu, err := tok.GetTokenUser()
	if err != nil {
		return Session{}, false, fmt.Errorf("session %d user: %w", id, err)
	}
	user, _, _, err := tu.User.Sid.LookupAccount("")
	if err != nil {
		user = ""
	}
	return Session{ID: id, User: user, SID: tu.User.Sid.String()}, true, nil
}

// Launch starts exe in session s with the user's token, which is the
// standard one (filtered by UAC), never elevated, on the interactive desktop.
func (winSessions) Launch(s Session, exe string, args []string) (Process, error) {
	var tok windows.Token
	if err := windows.WTSQueryUserToken(s.ID, &tok); err != nil {
		return nil, fmt.Errorf("session %d token: %w", s.ID, err)
	}
	defer tok.Close()
	var env *uint16
	if err := windows.CreateEnvironmentBlock(&env, tok, false); err != nil {
		return nil, fmt.Errorf("user environment: %w", err)
	}
	defer windows.DestroyEnvironmentBlock(env) //nolint:errcheck // best effort
	cmd, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{exe}, args...)))
	if err != nil {
		return nil, err
	}
	app, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return nil, err
	}
	dir, err := windows.UTF16PtrFromString(filepath.Dir(exe))
	if err != nil {
		return nil, err
	}
	desktop, _ := windows.UTF16PtrFromString(`winsta0\default`)
	si := windows.StartupInfo{Desktop: desktop}
	si.Cb = uint32(unsafe.Sizeof(si))
	var pi windows.ProcessInformation
	const flags = windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_NEW_PROCESS_GROUP
	if err := windows.CreateProcessAsUser(tok, app, cmd, nil, nil, false, flags, env, dir, &si, &pi); err != nil {
		return nil, fmt.Errorf("start %s in session %d: %w", filepath.Base(exe), s.ID, err)
	}
	windows.CloseHandle(pi.Thread) //nolint:errcheck // not needed
	p := &winProcess{h: pi.Process, pid: int(pi.ProcessId), done: make(chan struct{})}
	go func() {
		windows.WaitForSingleObject(p.h, windows.INFINITE) //nolint:errcheck // returns when it exits
		close(p.done)
	}()
	return p, nil
}

type winProcess struct {
	h    windows.Handle
	pid  int
	done chan struct{}
	once sync.Once
}

func (p *winProcess) PID() int { return p.pid }

func (p *winProcess) Wait(ctx context.Context) error {
	select {
	case <-p.done:
		p.once.Do(func() { windows.CloseHandle(p.h) }) //nolint:errcheck // exited
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *winProcess) Kill() error {
	select {
	case <-p.done:
		return nil
	default:
	}
	if err := windows.TerminateProcess(p.h, 1); err != nil {
		return fmt.Errorf("kill %d: %w", p.pid, err)
	}
	return nil
}

func pipePath(name string) string { return `\\.\pipe\` + name }

// listenLocal creates the named pipe. Its DACL is protected (P: no inherited
// entries) and allows only LocalSystem (the service) and the session's user.
func listenLocal(name, allowSID string) (net.Listener, error) {
	if !strings.HasPrefix(allowSID, "S-1-") {
		return nil, fmt.Errorf("local channel: invalid SID %q", allowSID)
	}
	sddl := "D:P(A;;GA;;;SY)(A;;GRGW;;;" + allowSID + ")"
	l, err := winio.ListenPipe(pipePath(name), &winio.PipeConfig{SecurityDescriptor: sddl})
	if err != nil {
		return nil, fmt.Errorf("local channel %s: %w", name, err)
	}
	return l, nil
}

func dialLocal(ctx context.Context, name string) (net.Conn, error) {
	c, err := winio.DialPipeContext(ctx, pipePath(name))
	if err != nil {
		return nil, fmt.Errorf("local channel %s: %w", name, err)
	}
	return c, nil
}

func peerPID(c net.Conn) (int, error) {
	f, ok := c.(interface{ Fd() uintptr })
	if !ok {
		return 0, fmt.Errorf("peer pid: %T is not a named pipe", c)
	}
	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(windows.Handle(f.Fd()), &pid); err != nil {
		return 0, fmt.Errorf("peer pid: %w", err)
	}
	return int(pid), nil
}
