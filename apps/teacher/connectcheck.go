package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"sync"

	"github.com/bayalad7/aula/internal/connectcheck"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
)

// connectChecks holds what the connection checker needs from the machine;
// tests replace it.
type connectChecks struct {
	mu sync.Mutex // one check at a time
	// source finds the consoles of a channel (mDNS in the app).
	source func(channel string) session.ConsoleSource
	net    platform.NetworkInspector // nil where unsupported
	// localAddrs are this computer's addresses.
	localAddrs func() []net.IP
	// loopback accepts 127.0.0.1 (tests only).
	loopback bool
}

func newConnectChecks(log *slog.Logger) *connectChecks {
	c := &connectChecks{localAddrs: interfaceAddrs}
	c.source = func(channel string) session.ConsoleSource {
		return &discovery.Browser{Channel: channel, Logger: log}
	}
	if n, err := platform.NewNetworkInspector(); err == nil {
		c.net = n
	}
	return c
}

// interfaceAddrs lists the IP addresses of this computer.
func interfaceAddrs() []net.IP {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []net.IP
	for _, a := range addrs {
		if ip, ok := a.(*net.IPNet); ok {
			out = append(out, ip.IP)
		}
	}
	return out
}

// CheckConnection walks the path a student takes to reach this console
// (prompt 18, Fase 6.3) and says at which step it would fail. Each step is
// also sent with aula:connectcheck as it runs. It never reaches students,
// real or simulated.
func (a *App) CheckConnection() (connectcheck.Report, error) {
	a.mu.Lock()
	class, info, ctx := a.class, a.info, a.ctx
	a.mu.Unlock()
	if class == nil {
		return connectcheck.Report{}, errNoClass
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !a.checks.mu.TryLock() {
		return connectcheck.Report{}, codedf(codeCheckRunning, "ya se está comprobando la conexión: espera el resultado")
	}
	defer a.checks.mu.Unlock()
	pub, err := security.ParsePublicKey(info.PublicKey)
	if err != nil {
		return connectcheck.Report{}, coded(codeConsoleKey, err)
	}
	program, err := os.Executable()
	if err != nil {
		a.log.Warn("connection check: cannot find the executable", "err", err)
	}
	c := &connectcheck.Checker{
		Channel: info.Channel, PublicKey: pub, Fingerprint: info.Fingerprint, Port: info.Port,
		Program: program, LocalAddrs: a.checks.localAddrs(), Source: a.checks.source(info.Channel),
		Net: a.checks.net, Loopback: a.checks.loopback, Logger: a.log,
	}
	r := c.Run(ctx, func(r connectcheck.Report) { a.emit(EventConnectCheck, r) })
	a.log.Info("connection checked", "channel", info.Channel, "cause", string(r.Cause))
	return r, nil
}
