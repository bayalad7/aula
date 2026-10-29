package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/transport"
)

// DefaultHelloTimeout bounds the whole handshake, from connection to hello
// (docs/protocol.md §3.1).
const DefaultHelloTimeout = 10 * time.Second

// Console runs the teacher side of the sessions of one channel.
type Console struct {
	Channel  string
	Registry *Registry
	// Identity signs the agents' challenges; CertSHA256 is the hash of the TLS
	// certificate the server presents, which the signature is bound to.
	Identity     *security.Identity
	CertSHA256   [32]byte
	HelloTimeout time.Duration // 0 means DefaultHelloTimeout
	// OnMessage receives every message of an established session except hello
	// and heartbeat (acks, errors…). It runs on the Run goroutine: keep it short.
	OnMessage func(ctx context.Context, conn *transport.Conn, env protocol.Envelope)
	Logger    *slog.Logger
}

// handshake phases of a connection on the console side.
type phase int

const (
	awaitChallenge phase = iota // nothing received yet
	awaitAuthOK                 // auth.proof sent
	awaitHello                  // agent accepted us; it must introduce itself
)

type pendingConn struct {
	since   time.Time
	phase   phase
	proofID string
}

// Run consumes server events until the channel is closed (the server closes it
// after shutting down), so no event is ever left undelivered. It drives the
// handshake of each connection (auth.challenge → auth.proof → auth.ok → hello)
// and then routes messages by type: heartbeat and any other message refresh
// the session, the rest goes to OnMessage.
func (c *Console) Run(ctx context.Context, events <-chan transport.Event) error {
	if !protocol.ValidChannel(c.Channel) {
		return fmt.Errorf("console: invalid channel %q", c.Channel)
	}
	if c.Registry == nil || c.Identity == nil || c.CertSHA256 == [32]byte{} {
		return fmt.Errorf("console: registry, identity and certificate hash are required")
	}
	helloTimeout := c.HelloTimeout
	if helloTimeout <= 0 {
		helloTimeout = DefaultHelloTimeout
	}
	log := c.Logger
	if log == nil {
		log = slog.Default()
	}
	log = log.With("channel", c.Channel)

	pending := map[*transport.Conn]*pendingConn{}
	tick := time.NewTicker(min(helloTimeout, c.Registry.timeout) / 5)
	defer tick.Stop()

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			switch ev.Kind {
			case transport.EventConnect:
				pending[ev.Conn] = &pendingConn{since: time.Now()}
			case transport.EventDisconnect:
				delete(pending, ev.Conn)
				if c.Registry.Disconnect(ev.Conn.ID()) {
					log.Info("agent disconnected", "conn", ev.Conn.ID())
				}
			case transport.EventMessage:
				if p, ok := pending[ev.Conn]; ok {
					if c.handshake(ctx, log, p, ev) {
						delete(pending, ev.Conn)
					}
					continue
				}
				c.route(ctx, log, ev)
			}
		case <-tick.C:
			now := time.Now()
			for conn, p := range pending {
				if now.Sub(p.since) > helloTimeout {
					delete(pending, conn)
					log.Info("handshake timeout", "conn", conn.ID())
					conn.Close(transport.ClosePolicyViolation, "handshake timeout")
				}
			}
			for _, id := range c.Registry.Sweep() {
				log.Info("agent silent, marked disconnected", "conn", id)
			}
		}
	}
}

// handshake processes one message of a connection that is not established
// yet. It returns true when the connection leaves the pending set (session
// opened, or connection being closed).
func (c *Console) handshake(ctx context.Context, log *slog.Logger, p *pendingConn, ev transport.Event) bool {
	env := ev.Envelope
	switch pl := env.Payload.(type) {
	case protocol.ErrorPayload:
		// The agent usually closes right after; an unauthorized error means it
		// does not trust this console's key (wrong key configured).
		log.Warn("agent reported an error during handshake", "conn", ev.Conn.ID(), "code", pl.Code)
		return false
	case protocol.AuthChallenge:
		if p.phase != awaitChallenge {
			break
		}
		sig := c.Identity.SignProof(pl.Nonce, c.Channel, c.CertSHA256)
		id, err := ev.Conn.Send(ctx, protocol.AuthProof{RefID: env.ID, Signature: sig})
		if err != nil {
			log.Debug("cannot send auth.proof", "conn", ev.Conn.ID(), "err", err)
			return false
		}
		p.phase, p.proofID = awaitAuthOK, id
		return false
	case protocol.AuthOK:
		if p.phase != awaitAuthOK || pl.RefID != p.proofID {
			break
		}
		p.phase = awaitHello
		return false
	case protocol.HelloInfo:
		if p.phase != awaitHello {
			break
		}
		s := c.Registry.Connect(ev.Conn.ID(), c.Channel, ev.Conn.RemoteAddr(), pl)
		ev.Conn.StartHeartbeat()
		log.Info("agent joined", "conn", s.ID, "hostname", s.Hostname, "os", s.OS, "version", s.AgentVersion)
		return true
	}

	// Anything else is out of order.
	if p.phase == awaitHello {
		log.Info("first message after auth.ok is not hello", "conn", ev.Conn.ID(), "type", env.Type)
		c.sendError(ctx, log, ev, protocol.CodeInvalidMessage, "expected hello")
		ev.Conn.Close(transport.CloseProtocolError, "expected hello")
	} else {
		log.Info("message before authentication", "conn", ev.Conn.ID(), "type", env.Type)
		c.sendError(ctx, log, ev, protocol.CodeUnauthorized, "authentication required")
		ev.Conn.Close(transport.ClosePolicyViolation, "authentication required")
	}
	return true
}

// route handles a message of an established session.
func (c *Console) route(ctx context.Context, log *slog.Logger, ev transport.Event) {
	c.Registry.Touch(ev.Conn.ID())
	switch ev.Envelope.Type {
	case protocol.TypeHeartbeat:
	case protocol.TypeHello, protocol.TypeAuthChallenge, protocol.TypeAuthOK:
		c.sendError(ctx, log, ev, protocol.CodeInvalidMessage, "session already established")
	default:
		if c.OnMessage != nil {
			c.OnMessage(ctx, ev.Conn, ev.Envelope)
		}
	}
}

func (c *Console) sendError(ctx context.Context, log *slog.Logger, ev transport.Event, code protocol.ErrorCode, msg string) {
	if ev.Envelope.Type == protocol.TypeError {
		return // never answer an error with an error
	}
	if _, err := ev.Conn.Send(ctx, protocol.NewError(ev.Envelope.ID, code, msg)); err != nil {
		log.Debug("cannot send error", "conn", ev.Conn.ID(), "err", err)
	}
}
