package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"strconv"
	"sync"

	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

// DefaultPort is the console's WebSocket/TLS port.
const DefaultPort = 47830

// classroom is an open class: what the App needs from the network stack.
// liveClass implements it; tests use a double.
type classroom interface {
	Broadcast(ctx context.Context, p protocol.Payload) (sent map[string]string, failed map[string]error)
	Sessions() []session.Session
	Stop()
}

// classHooks are called from the console goroutine.
type classHooks struct {
	onAck    func(refID string) // a code.ack or msg.ack arrived
	onChange func()             // the registry changed
}

type classConfig struct {
	Channel     string
	ConsoleName string
	Dir         string // identity and certificate
	Port        int
}

type startClassFunc func(ctx context.Context, cfg classConfig, hooks classHooks, log *slog.Logger) (classroom, ClassInfo, error)

// liveClass is the real class: TLS server, console, registry and mDNS.
type liveClass struct {
	console *session.Console
	reg     *session.Registry
	channel string
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func (l *liveClass) Broadcast(ctx context.Context, p protocol.Payload) (map[string]string, map[string]error) {
	return l.console.Broadcast(ctx, p)
}

func (l *liveClass) Sessions() []session.Session { return l.reg.List(l.channel) }

func (l *liveClass) Stop() {
	l.cancel()
	l.wg.Wait()
}

// startLiveClass opens a class: it loads (or creates on first use) the
// console identity and certificate, listens on cfg.Port and announces the
// channel over mDNS.
func startLiveClass(ctx context.Context, cfg classConfig, hooks classHooks, log *slog.Logger) (classroom, ClassInfo, error) {
	id, err := security.LoadIdentity(cfg.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		id, err = security.CreateIdentity(cfg.Dir, false)
	}
	if err != nil {
		return nil, ClassInfo{}, fmt.Errorf("clave de la consola: %w", err)
	}
	cert, err := security.LoadOrCreateCertificate(cfg.Dir)
	if err != nil {
		return nil, ClassInfo{}, fmt.Errorf("certificado: %w", err)
	}
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Port))
	if err != nil {
		return nil, ClassInfo{}, fmt.Errorf("el puerto %d está ocupado (¿hay otra consola abierta?): %w", cfg.Port, err)
	}

	port := ln.Addr().(*net.TCPAddr).Port // the real one when cfg.Port is 0 (tests)
	reg := session.NewRegistry(0)
	srv := transport.NewServer(transport.Options{TLS: security.ServerTLS(cert), Logger: log})
	console := &session.Console{
		Channel: cfg.Channel, Registry: reg, Identity: id, CertSHA256: cert.SHA256, Logger: log,
		OnChange: hooks.onChange,
		OnMessage: func(_ context.Context, _ *transport.Conn, env protocol.Envelope) {
			switch p := env.Payload.(type) {
			case protocol.CodeAck:
				hooks.onAck(p.RefID)
			case protocol.MsgAck:
				hooks.onAck(p.RefID)
			}
		},
	}
	announcer := &discovery.Announcer{Logger: log, Announcement: discovery.Announcement{
		Channel: cfg.Channel, ConsoleName: cfg.ConsoleName, WSPort: port, CertFingerprint: cert.Fingerprint(),
	}}

	ctx, cancel := context.WithCancel(ctx)
	l := &liveClass{console: console, reg: reg, channel: cfg.Channel, cancel: cancel}
	l.wg.Add(3)
	go func() {
		defer l.wg.Done()
		if err := srv.Serve(ctx, ln); err != nil {
			log.Error("server stopped", "err", err)
		}
	}()
	go func() {
		defer l.wg.Done()
		if err := console.Run(ctx, srv.Events()); err != nil {
			log.Error("console stopped", "err", err)
		}
	}()
	go func() {
		defer l.wg.Done()
		// Without mDNS, agents can still connect with a direct address.
		if err := announcer.Run(ctx); err != nil {
			log.Warn("mdns announce unavailable", "err", err)
		}
	}()
	return l, ClassInfo{
		Channel:     cfg.Channel,
		ConsoleName: cfg.ConsoleName,
		PublicKey:   security.EncodePublicKey(id.Public()),
		Fingerprint: security.KeyFingerprint(id.Public()),
		Port:        port,
	}, nil
}
