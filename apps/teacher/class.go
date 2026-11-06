package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"strconv"
	"sync"

	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/simclass"
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
	// onPolicyAck receives app.block.ack and web.filter.ack.
	onPolicyAck func(sessionID string, p protocol.Payload)
	// onPower receives power.ack and the errors (which may answer an order).
	onPower func(sessionID string, env protocol.Envelope)
	// onRemote receives remote.ack and the errors (which may answer a
	// remote.start).
	onRemote func(sessionID string, env protocol.Envelope)
	// onSnapshot receives screen.snapshot.response parts and the errors
	// (which may answer a screen.snapshot.request). Simulated students too.
	onSnapshot func(sessionID string, env protocol.Envelope)
	// onClassroom receives hand.raise, hand.lower and msg.student, from real
	// and simulated students (docs/protocol.md §5.18).
	onClassroom func(sessionID string, env protocol.Envelope)
	// onTimer receives timer.ack, from real and simulated students (§5.19).
	onTimer func(sessionID string, p protocol.TimerAck)
	// onFile receives file.push.ack, file.collect.ack, the parts of
	// file.collect.response and the errors (which may answer a file.push or
	// a file.collect.request), from real and simulated students (§5.20–5.21).
	onFile func(sessionID string, env protocol.Envelope)
	// onError receives every error message of a student, real or simulated.
	onError func(sessionID string, p protocol.ErrorPayload)
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
			return nil, "", codedf(codeSiteKey, "clave del centro: %w", err)
		}
	}
	id, err := security.LoadIdentity(userDir)
	if errors.Is(err, fs.ErrNotExist) && create {
		id, err = security.CreateIdentity(userDir, false)
	}
	if err != nil {
		return nil, "", codedf(codeConsoleKey, "clave de la consola: %w", err)
	}
	return id, keyPersonal, nil
}

type startClassFunc func(ctx context.Context, cfg classConfig, hooks classHooks, log *slog.Logger) (classroom, ClassInfo, error)

// simulationHost is a class that can also host simulated students (the
// aula simulada). They connect to their own loopback listener and registry,
// so they are told apart by where they arrived, never by their name, and
// Sessions and Broadcast keep meaning real students only.
type simulationHost interface {
	// StartSimulation opens the listener for simulated students, if needed,
	// and returns where they must connect.
	StartSimulation() (simclass.Console, error)
	// StopSimulation closes it and forgets the simulated sessions.
	StopSimulation()
	SimSessions() []session.Session
	SimBroadcast(ctx context.Context, p protocol.Payload) (sent map[string]string, failed map[string]error)
}

// liveClass is the real class: TLS server, console, registry and mDNS.
type liveClass struct {
	console *session.Console
	reg     *session.Registry
	channel string
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// What a simulated side needs to look like this console.
	id          *security.Identity
	cert        security.Certificate
	consoleName string
	hooks       classHooks
	log         *slog.Logger

	simMu sync.Mutex
	sim   *simSide
}

// simSide is the console for simulated students: same identity and
// certificate, its own registry, on 127.0.0.1 and never announced.
type simSide struct {
	console *session.Console
	reg     *session.Registry
	addr    *net.TCPAddr
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func (l *liveClass) Broadcast(ctx context.Context, p protocol.Payload) (map[string]string, map[string]error) {
	return l.console.Broadcast(ctx, p)
}

// Send delivers p to a real session or, if the ID is a simulated one, to it
// (session IDs are UUIDs: they never collide).
func (l *liveClass) Send(ctx context.Context, sessionID string, p protocol.Payload) (string, error) {
	if _, ok := l.reg.Get(sessionID); !ok {
		if sim := l.simulated(); sim != nil {
			if _, ok := sim.reg.Get(sessionID); ok {
				return sim.console.Send(ctx, sessionID, p)
			}
		}
	}
	return l.console.Send(ctx, sessionID, p)
}

func (l *liveClass) Sessions() []session.Session { return l.reg.List(l.channel) }

func (l *liveClass) Stop() {
	l.StopSimulation()
	l.cancel()
	l.wg.Wait()
}

func (l *liveClass) simulated() *simSide {
	l.simMu.Lock()
	defer l.simMu.Unlock()
	return l.sim
}

// StartSimulation implements simulationHost.
func (l *liveClass) StartSimulation() (simclass.Console, error) {
	l.simMu.Lock()
	defer l.simMu.Unlock()
	if l.sim == nil {
		ln, err := net.Listen("tcp", "127.0.0.1:0") // loopback only: no real student can reach it
		if err != nil {
			return simclass.Console{}, codedf(codeSimFailed, "no se pudo abrir el aula simulada: %w", err)
		}
		log := l.log.With("simulated", true)
		reg := session.NewRegistry(0)
		srv := transport.NewServer(transport.Options{TLS: security.ServerTLS(l.cert), Logger: log})
		console := &session.Console{
			Channel: l.channel, Registry: reg, Identity: l.id, CertSHA256: l.cert.SHA256, Logger: log,
			OnChange:  l.hooks.onChange,
			OnMessage: simulatedMessages(l.hooks),
		}
		ctx, cancel := context.WithCancel(context.Background())
		sim := &simSide{console: console, reg: reg, addr: ln.Addr().(*net.TCPAddr), cancel: cancel}
		sim.wg.Add(2)
		go func() {
			defer sim.wg.Done()
			if err := srv.Serve(ctx, ln); err != nil {
				log.Error("simulated server stopped", "err", err)
			}
		}()
		go func() {
			defer sim.wg.Done()
			if err := console.Run(ctx, srv.Events()); err != nil {
				log.Error("simulated console stopped", "err", err)
			}
		}()
		l.sim = sim
		log.Info("simulated class opened", "addr", sim.addr.String())
	}
	return simclass.Console{
		Addr: l.sim.addr, Channel: l.channel, Name: l.consoleName,
		PublicKey: l.id.Public(), CertFingerprint: l.cert.Fingerprint(),
	}, nil
}

// StopSimulation implements simulationHost.
func (l *liveClass) StopSimulation() {
	l.simMu.Lock()
	sim := l.sim
	l.sim = nil
	l.simMu.Unlock()
	if sim != nil {
		sim.cancel()
		sim.wg.Wait()
		l.log.Info("simulated class closed", "simulated", true)
	}
}

// SimSessions implements simulationHost.
func (l *liveClass) SimSessions() []session.Session {
	if sim := l.simulated(); sim != nil {
		return sim.reg.List(l.channel)
	}
	return nil
}

// SimBroadcast implements simulationHost.
func (l *liveClass) SimBroadcast(ctx context.Context, p protocol.Payload) (map[string]string, map[string]error) {
	if sim := l.simulated(); sim != nil {
		return sim.console.Broadcast(ctx, p)
	}
	return map[string]string{}, map[string]error{}
}

// simulatedMessages routes what simulated students send: acks, screen
// thumbnails, screenshots, and raised hands and messages. They never get power, restrictions or the broadcast, so
// any other answer is ignored.
func simulatedMessages(h classHooks) func(context.Context, *transport.Conn, protocol.Envelope) {
	return func(_ context.Context, conn *transport.Conn, env protocol.Envelope) {
		switch p := env.Payload.(type) {
		case protocol.CodeAck:
			h.onAck(p.RefID)
		case protocol.MsgAck:
			h.onAck(p.RefID)
		case protocol.ScreenThumbFrame:
			if h.onFrame != nil {
				h.onFrame(conn.ID(), p)
			}
		case protocol.ScreenSnapshotResponse:
			if h.onSnapshot != nil {
				h.onSnapshot(conn.ID(), env)
			}
		case protocol.HandRaise, protocol.HandLower, protocol.MsgStudent:
			if h.onClassroom != nil {
				h.onClassroom(conn.ID(), env)
			}
		case protocol.TimerAck:
			if h.onTimer != nil {
				h.onTimer(conn.ID(), p)
			}
		case protocol.FilePushAck, protocol.FileCollectAck, protocol.FileCollectResponse:
			if h.onFile != nil {
				h.onFile(conn.ID(), env)
			}
		case protocol.ErrorPayload:
			if h.onSnapshot != nil {
				h.onSnapshot(conn.ID(), env)
			}
			if h.onFile != nil {
				h.onFile(conn.ID(), env)
			}
			if h.onError != nil {
				h.onError(conn.ID(), p)
			}
		}
	}
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
		return nil, ClassInfo{}, codedf(codeCertificate, "certificado: %w", err)
	}
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Port))
	if err != nil {
		return nil, ClassInfo{}, codedf(codePortInUse, "el puerto %d está ocupado (¿hay otra consola abierta?): %w", cfg.Port, err)
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
			case protocol.AppBlockAck, protocol.WebFilterAck:
				if hooks.onPolicyAck != nil {
					hooks.onPolicyAck(conn.ID(), env.Payload)
				}
			case protocol.PowerAck:
				if hooks.onPower != nil {
					hooks.onPower(conn.ID(), env)
				}
			case protocol.RemoteAck:
				if hooks.onRemote != nil {
					hooks.onRemote(conn.ID(), env)
				}
			case protocol.ScreenSnapshotResponse:
				if hooks.onSnapshot != nil {
					hooks.onSnapshot(conn.ID(), env)
				}
			case protocol.HandRaise, protocol.HandLower, protocol.MsgStudent:
				if hooks.onClassroom != nil {
					hooks.onClassroom(conn.ID(), env)
				}
			case protocol.TimerAck:
				if hooks.onTimer != nil {
					hooks.onTimer(conn.ID(), p)
				}
			case protocol.FilePushAck, protocol.FileCollectAck, protocol.FileCollectResponse:
				if hooks.onFile != nil {
					hooks.onFile(conn.ID(), env)
				}
			case protocol.ScreenBroadcastAnswer, protocol.ScreenBroadcastICE, protocol.ErrorPayload:
				if hooks.onBroadcast != nil {
					hooks.onBroadcast(conn.ID(), env)
				}
				if p, isErr := env.Payload.(protocol.ErrorPayload); isErr {
					if hooks.onPower != nil {
						hooks.onPower(conn.ID(), env)
					}
					if hooks.onRemote != nil {
						hooks.onRemote(conn.ID(), env)
					}
					if hooks.onSnapshot != nil {
						hooks.onSnapshot(conn.ID(), env)
					}
					if hooks.onFile != nil {
						hooks.onFile(conn.ID(), env)
					}
					if hooks.onError != nil {
						hooks.onError(conn.ID(), p)
					}
				}
			}
		},
	}
	announcer := &discovery.Announcer{Logger: log, Announcement: discovery.Announcement{
		Channel: cfg.Channel, ConsoleName: cfg.ConsoleName, WSPort: port, CertFingerprint: cert.Fingerprint(),
	}}

	ctx, cancel := context.WithCancel(ctx)
	l := &liveClass{console: console, reg: reg, channel: cfg.Channel, cancel: cancel,
		id: id, cert: cert, consoleName: cfg.ConsoleName, hooks: hooks, log: log}
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
	info := ClassInfo{
		Channel:     cfg.Channel,
		ConsoleName: cfg.ConsoleName,
		PublicKey:   security.EncodePublicKey(id.Public()),
		Fingerprint: security.KeyFingerprint(id.Public()),
		KeySource:   source,
		Port:        port,
	}
	if t := cert.NotAfter(); !t.IsZero() {
		info.CertExpires = t.Format("2006-01-02")
	}
	return l, info, nil
}
