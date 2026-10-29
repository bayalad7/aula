//go:build windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// Restart delays after a failure; the counter resets after a day without one.
var restartDelays = []time.Duration{5 * time.Second, 10 * time.Second, 30 * time.Second}

const recoveryResetSecs = 24 * 60 * 60

func newServiceControl(name string) ServiceControl { return winService{name: name} }

type winService struct{ name string }

func (w winService) Install(c ServiceConfig) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("service manager: %w", err)
	}
	defer m.Disconnect()
	cfg := mgr.Config{
		StartType:    mgr.StartAutomatic,
		DisplayName:  c.DisplayName,
		Description:  c.Description,
		ErrorControl: mgr.ErrorNormal,
		// ServiceStartName empty: LocalSystem, needed to start the helper in
		// the user's session (WTSQueryUserToken requires SeTcbPrivilege).
	}
	s, err := m.OpenService(c.Name)
	if err == nil {
		// Upgrade: same service, new path or arguments.
		cur, err := s.Config()
		if err != nil {
			s.Close()
			return fmt.Errorf("service config: %w", err)
		}
		cur.StartType, cur.DisplayName, cur.Description, cur.ErrorControl = cfg.StartType, cfg.DisplayName, cfg.Description, cfg.ErrorControl
		cur.BinaryPathName = windows.ComposeCommandLine(append([]string{c.Exe}, c.Args...))
		cur.ServiceStartName = "LocalSystem"
		if err := s.UpdateConfig(cur); err != nil {
			s.Close()
			return fmt.Errorf("update service: %w", err)
		}
	} else {
		if s, err = m.CreateService(c.Name, c.Exe, cfg, c.Args...); err != nil {
			return fmt.Errorf("create service: %w", err)
		}
	}
	defer s.Close()
	actions := make([]mgr.RecoveryAction, len(restartDelays))
	for i, d := range restartDelays {
		actions[i] = mgr.RecoveryAction{Type: mgr.ServiceRestart, Delay: d}
	}
	if err := s.SetRecoveryActions(actions, recoveryResetSecs); err != nil {
		return fmt.Errorf("service recovery: %w", err)
	}
	// Also restart when the service stops itself with an error, not only on
	// crashes.
	if err := s.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		return fmt.Errorf("service recovery: %w", err)
	}
	return nil
}

func (w winService) Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("service manager: %w", err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(w.name)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer s.Close()
	if err := s.Delete(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
		return fmt.Errorf("delete service: %w", err)
	}
	return nil
}

func (w winService) Start() error {
	s, done, err := w.open(windows.SERVICE_START | windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return err
	}
	defer done()
	if err := s.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return fmt.Errorf("start service: %w", err)
	}
	return nil
}

func (w winService) Stop(ctx context.Context) error {
	s, done, err := w.open(windows.SERVICE_STOP | windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return err
	}
	defer done()
	if _, err := s.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return fmt.Errorf("stop service: %w", err)
	}
	for {
		st, err := s.Query()
		if err != nil {
			return fmt.Errorf("query service: %w", err)
		}
		if st.State == svc.Stopped {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("stop service: %w", ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (w winService) Status() (ServiceStatus, error) {
	s, done, err := w.open(windows.SERVICE_QUERY_STATUS | windows.SERVICE_QUERY_CONFIG)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return ServiceStatus{State: ServiceNotInstalled}, nil
	}
	if err != nil {
		return ServiceStatus{}, err
	}
	defer done()
	st, err := s.Query()
	if err != nil {
		return ServiceStatus{}, fmt.Errorf("query service: %w", err)
	}
	out := ServiceStatus{State: stateOf(st.State), PID: int(st.ProcessId)}
	if cfg, err := s.Config(); err == nil {
		out.Auto = cfg.StartType == mgr.StartAutomatic
	}
	if acts, err := s.RecoveryActions(); err == nil && len(acts) > 0 {
		out.RestartOnFailure = acts[0].Type == mgr.ServiceRestart
	}
	return out, nil
}

// open opens the service with only the rights needed, so Status works for a
// standard user (mgr.Connect asks for full access to the manager).
func (w winService) open(access uint32) (*mgr.Service, func(), error) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, nil, fmt.Errorf("service manager: %w", err)
	}
	name, err := windows.UTF16PtrFromString(w.name)
	if err != nil {
		windows.CloseServiceHandle(scm)
		return nil, nil, err
	}
	h, err := windows.OpenService(scm, name, access)
	if err != nil {
		windows.CloseServiceHandle(scm)
		return nil, nil, fmt.Errorf("open service %s: %w", w.name, err)
	}
	s := &mgr.Service{Name: w.name, Handle: h}
	return s, func() { s.Close(); windows.CloseServiceHandle(scm) }, nil
}

func stateOf(s svc.State) ServiceState {
	switch s {
	case svc.Stopped:
		return ServiceStopped
	case svc.StartPending:
		return ServiceStartPending
	case svc.StopPending:
		return ServiceStopPending
	case svc.Running:
		return ServiceRunning
	default:
		return ServiceOther
	}
}

func isService() (bool, error) { return svc.IsWindowsService() }

func runAsService(name string, run func(context.Context, <-chan struct{}) error) error {
	return svc.Run(name, &handler{run: run})
}

type handler struct {
	run func(context.Context, <-chan struct{}) error
}

// Execute reports the service running, forwards session changes and stops
// run when the manager asks. If run fails, the service exits with an error
// code so the manager applies the recovery actions.
func (h *handler) Execute(_ []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	const accepts = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptSessionChange
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changed := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() { done <- h.run(ctx, changed) }()
	status <- svc.Status{State: svc.Running, Accepts: accepts}
	for {
		select {
		case err := <-done:
			if err != nil {
				return true, 1 // service-specific error: triggers the restart
			}
			return false, 0
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			case svc.SessionChange:
				select {
				case changed <- struct{}{}:
				default: // one pending notice is enough: the receiver re-reads the session
				}
			}
		}
	}
}
