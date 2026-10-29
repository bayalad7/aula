package platform

import (
	"context"
	"net"
)

// The student agent as an OS service (D-016): the service keeps the
// connection with the console and launches a helper in the interactive
// session of the student, which shows everything on screen. Windows today;
// elsewhere every function returns ErrNotSupported.

// ServiceConfig describes the service to install.
type ServiceConfig struct {
	Name        string // e.g. "AulaAgente"
	DisplayName string // shown in services.msc: never hidden or misleading
	Description string
	Exe         string   // absolute path of the executable
	Args        []string // e.g. ["servicio", "ejecutar"]
}

// ServiceState is the state reported by the service manager.
type ServiceState int

// Service states.
const (
	ServiceNotInstalled ServiceState = iota
	ServiceStopped
	ServiceStartPending
	ServiceStopPending
	ServiceRunning
	ServiceOther // paused, continue pending…
)

func (s ServiceState) String() string {
	switch s {
	case ServiceNotInstalled:
		return "no instalado"
	case ServiceStopped:
		return "detenido"
	case ServiceStartPending:
		return "iniciando"
	case ServiceStopPending:
		return "deteniéndose"
	case ServiceRunning:
		return "en ejecución"
	default:
		return "otro"
	}
}

// ServiceStatus is what Status reports.
type ServiceStatus struct {
	State ServiceState
	PID   int  // 0 unless running
	Auto  bool // starts with the computer
	// RestartOnFailure is set when the service manager restarts it after a
	// crash or an exit with an error.
	RestartOnFailure bool
}

// ServiceControl installs and operates one service. Install, Uninstall,
// Start and Stop need an administrator; Status works for any user.
type ServiceControl interface {
	// Install creates the service (automatic start, LocalSystem, restarted
	// on failure after 5, 10 and 30 s) or updates it if it exists.
	Install(ServiceConfig) error
	// Uninstall deletes the service; it is not an error if it does not exist.
	Uninstall() error
	// Start starts it; it is not an error if it is already running.
	Start() error
	// Stop stops it and waits until it is stopped or ctx ends.
	Stop(ctx context.Context) error
	Status() (ServiceStatus, error)
}

// NewServiceControl returns the control of the service called name.
func NewServiceControl(name string) ServiceControl { return newServiceControl(name) }

// RunAsService runs run as the service called name until the service manager
// stops it (ctx is canceled). sessionChanged receives a value whenever a user
// logs on or off or the console session changes. If run returns an error, the
// service exits with a failure and the manager restarts it.
func RunAsService(name string, run func(ctx context.Context, sessionChanged <-chan struct{}) error) error {
	return runAsService(name, run)
}

// IsService reports whether this process was started by the service manager.
func IsService() (bool, error) { return isService() }

// Session is the interactive session of the computer's console.
type Session struct {
	ID   uint32
	User string // account name, without the domain
	SID  string // e.g. "S-1-5-21-…"
}

// Process is a process started by SessionManager.Launch.
type Process interface {
	PID() int
	// Wait blocks until the process exits or ctx ends.
	Wait(ctx context.Context) error
	Kill() error
}

// SessionManager finds the student's session and starts the helper in it.
// It needs the service's privileges (LocalSystem on Windows).
type SessionManager interface {
	// Active returns the console session with a logged-on user; ok is false
	// at the logon screen.
	Active() (s Session, ok bool, err error)
	// Launch starts exe in session s with the user's own token (not
	// elevated) and environment, on the interactive desktop.
	Launch(s Session, exe string, args []string) (Process, error)
}

// NewSessionManager returns the OS session manager, or ErrNotSupported.
func NewSessionManager() (SessionManager, error) { return newSessionManager() }

// ListenLocal opens a local channel (a named pipe on Windows) that only the
// service itself and the user with SID allowSID can open.
func ListenLocal(name, allowSID string) (net.Listener, error) { return listenLocal(name, allowSID) }

// DialLocal connects to a channel opened with ListenLocal.
func DialLocal(ctx context.Context, name string) (net.Conn, error) { return dialLocal(ctx, name) }

// PeerPID returns the process ID of the client at the other end of a
// connection accepted from ListenLocal.
func PeerPID(c net.Conn) (int, error) { return peerPID(c) }
