//go:build !windows

package platform

import (
	"context"
	"net"
)

// The agent service is only implemented on Windows for now; Linux (systemd)
// and macOS (launchd) will follow.

func newServiceControl(string) ServiceControl { return unsupportedService{} }

type unsupportedService struct{}

func (unsupportedService) Install(ServiceConfig) error    { return ErrNotSupported }
func (unsupportedService) Uninstall() error               { return ErrNotSupported }
func (unsupportedService) Start() error                   { return ErrNotSupported }
func (unsupportedService) Stop(context.Context) error     { return ErrNotSupported }
func (unsupportedService) Status() (ServiceStatus, error) { return ServiceStatus{}, ErrNotSupported }

func runAsService(string, func(context.Context, <-chan struct{}) error) error { return ErrNotSupported }

func isService() (bool, error) { return false, nil }

func newSessionManager() (SessionManager, error) { return nil, ErrNotSupported }

func listenLocal(string, string) (net.Listener, error)    { return nil, ErrNotSupported }
func dialLocal(context.Context, string) (net.Conn, error) { return nil, ErrNotSupported }
func peerPID(net.Conn) (int, error)                       { return 0, ErrNotSupported }
