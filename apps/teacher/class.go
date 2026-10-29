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
	// Send delivers p to one session and returns the envelope ID.
	Send(ctx context.Context, sessionID string, p protocol.Payload) (string, error)
	Sessions() []session.Session
	Stop()
}

// classHooks are called from the console goroutine.
type classHooks struct {
	onAck    func(refID string)                                  // a code.ack or msg.ack arrived
	onChange func()                                              // the registry changed
	onFrame  func(sessionID string, f protocol.ScreenThumbFrame) // a screen thumbnail arrived
	// onBroadcast receives the agents' screen.broadcast answers, candidates
	// and errors.
	onBroadcast func(sessionID string, env protocol.Envelope)
}

type classConfig struct {
	Channel     string
	ConsoleName string
	Dir         string // certificate and the personal identity
	SiteKeyDir  string // the site key, when this machine has one
	Port        int
}

// Key sources reported in ClassInfo.KeySource.
const (
	keyCentre   = "centro"
	keyPersonal = "personal"
)

// consoleKey returns the key the console signs with: the site key (clave del
// centro) when this machine has one, which is how a centre is deployed;
// otherwise the user's personal identity, created on first use if create is
// set (development, and machines installed before the site key existed).
func consoleKey(siteDir, userDir string, create bool) (*security.Identity, string, error) {
	if siteDir != "" {
		id, err := security.LoadSiteKey(siteDir)
		if err == nil {
			return id, keyCentre, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, "", fmt.Errorf("clave del centro: %w", err)
		}
	}
	id, err := security.LoadIdentity(userDir)
	if errors.Is(err, fs.ErrNotExist) && create {
		id, err = security.CreateIdentity(userDir, false)
	}
	if err != nil {
		return nil, "", fmt.Errorf("clave de la consola: %w", err)
	}
	return id, keyPersonal, nil
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

func (l *liveClass) Send(ctx context.Context, sessionID string, p protocol.Payload) (string, error) {
	return l.console.Send(ctx, sessionID, p)
}

func (l *liveClass) Sessions() []session.Session { return l.reg.List(l.channel) }

func (l *liveClass) Stop() {
	l.cancel()
	l.wg.Wait()
}

// startLiveClass opens a class: it loads the console key (consoleKey) and the
// certificate, listens on cfg.Port and announces the channel over mDNS.
func startLiveClass(ctx context.Context, cfg classConfig, hooks classHooks, log *slog.Logger) (classroom, ClassInfo, error) {
	id, source, err := consoleKey(cfg.SiteKeyDir, cfg.Dir, true)
	if err != nil {
		return nil, ClassInfo{}, err
	}
	if source == keyPersonal {
		log.Warn("no site key on this machine, using a personal key", "site_key_dir", cfg.SiteKeyDir)
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
		OnMessage: func(_ context.Context, conn *transport.Conn, env protocol.Envelope) {
			switch p := env.Payload.(type) {
			case protocol.CodeAck:
				hooks.onAck(p.RefID)
			case protocol.MsgAck:
				hooks.onAck(p.RefID)
			case protocol.ScreenThumbFrame:
				if hooks.onFrame != nil {
					hooks.onFrame(conn.ID(), p)
				}
			case protocol.ScreenBroadcastAnswer, protocol.ScreenBroadcastICE, protocol.ErrorPayload:
				if hooks.onBroadcast != nil {
					hooks.onBroadcast(conn.ID(), env)
				}
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
		KeySource:   source,
		Port:        port,
	}, nil
}
