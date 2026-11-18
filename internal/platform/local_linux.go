//go:build linux

package platform

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"golang.org/x/sys/unix"
)

// The local channel between the service and its helper on Linux (D-024): a
// Unix socket in /run/aula, owned by the student's UID with mode 0600, so
// only that user (and root) can connect. The service then checks the peer's
// PID with SO_PEERCRED (agentsvc.Supervisor).

// localDir holds the sockets; replaced in tests.
var localDir = "/run/aula"

var localName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,80}$`)

func localPath(name string) (string, error) {
	if !localName.MatchString(name) {
		return "", fmt.Errorf("local channel: bad name %q", name)
	}
	return filepath.Join(localDir, name+".sock"), nil
}

// listenLocal opens the socket for the user whose UID is allowSID ("" means
// root only).
func listenLocal(name, allowSID string) (net.Listener, error) {
	path, err := localPath(name)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return nil, fmt.Errorf("local channel: %w", err)
	}
	_ = os.Remove(path) // a socket left by a crash
	// The socket is born in a private folder (0700), gets its owner and mode
	// there and only then moves into place: nobody can connect in between,
	// and the process umask is never touched.
	tmp, err := os.MkdirTemp(localDir, ".nuevo-")
	if err != nil {
		return nil, fmt.Errorf("local channel: %w", err)
	}
	defer os.RemoveAll(tmp)
	born := filepath.Join(tmp, "s")
	ln, err := net.Listen("unix", born)
	if err != nil {
		return nil, fmt.Errorf("local channel: %w", err)
	}
	ul := ln.(*net.UnixListener)
	ul.SetUnlinkOnClose(false) // it is unlinked at its final path, below
	fail := func(err error) (net.Listener, error) {
		ln.Close()
		return nil, fmt.Errorf("local channel: %w", err)
	}
	if allowSID != "" {
		uid, err := strconv.Atoi(allowSID)
		if err != nil {
			return fail(fmt.Errorf("uid %q: %w", allowSID, err))
		}
		if err := os.Chown(born, uid, -1); err != nil {
			return fail(err)
		}
	}
	if err := os.Chmod(born, 0o600); err != nil {
		return fail(err)
	}
	if err := os.Rename(born, path); err != nil {
		return fail(err)
	}
	return &localListener{UnixListener: ul, path: path}, nil
}

// localListener removes its socket file when closed.
type localListener struct {
	*net.UnixListener
	path string
}

func (l *localListener) Close() error {
	err := l.UnixListener.Close()
	_ = os.Remove(l.path)
	return err
}

func dialLocal(ctx context.Context, name string) (net.Conn, error) {
	path, err := localPath(name)
	if err != nil {
		return nil, err
	}
	var d net.Dialer
	c, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("local channel: %w", err)
	}
	return c, nil
}

// peerPID reads the PID of the process at the other end (SO_PEERCRED): the
// kernel fills it in, the peer cannot lie about it.
func peerPID(c net.Conn) (int, error) {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return 0, fmt.Errorf("local channel: not a Unix socket")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return 0, fmt.Errorf("local channel: %w", err)
	}
	var cred *unix.Ucred
	var cerr error
	if err := raw.Control(func(fd uintptr) {
		cred, cerr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return 0, fmt.Errorf("local channel: %w", err)
	}
	if cerr != nil {
		return 0, fmt.Errorf("local channel: %w", cerr)
	}
	return int(cred.Pid), nil
}
