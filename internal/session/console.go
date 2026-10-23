package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/transport"
)

// DefaultHelloTimeout bounds the handshake (docs/protocol.md §3.1).
const DefaultHelloTimeout = 10 * time.Second

// Console runs the teacher side of the sessions of one channel.
type Console struct {
	Channel      string
	Registry     *Registry
	HelloTimeout time.Duration // 0 means DefaultHelloTimeout
	// OnMessage receives every message of an established session except hello
	// and heartbeat (acks, errors…). It runs on the Run goroutine: keep it short.
	OnMessage func(ctx context.Context, conn *transport.Conn, env protocol.Envelope)
	Logger    *slog.Logger
}

// Run consumes server events until the channel is closed (the server closes it
// after shutting down), so no event is ever left undelivered. It routes each
// message by type: hello opens the session, heartbeat and any other message
// refresh it, the rest goes to OnMessage.
func (c *Console) Run(ctx context.Context, events <-chan transport.Event) error {
	if !protocol.ValidChannel(c.Channel) {
		return fmt.Errorf("console: invalid channel %q", c.Channel)
	}
	if c.Registry == nil {
		return fmt.Errorf("console: nil registry")
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

	pending := map[*transport.Conn]time.Time{} // connected, hello not received yet
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
				pending[ev.Conn] = time.Now()
			case transport.EventDisconnect:
				delete(pending, ev.Conn)
				if c.Registry.Disconnect(ev.Conn.ID()) {
					log.Info("agent disconnected", "conn", ev.Conn.ID())
				}
			case transport.EventMessage:
				if _, waiting := pending[ev.Conn]; waiting {
					delete(pending, ev.Conn)
					c.open(ctx, log, ev)
					continue
				}
				c.route(ctx, log, ev)
			}
		case <-tick.C:
			now := time.Now()
			for conn, since := range pending {
				if now.Sub(since) > helloTimeout {
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

// open handles the first message of a connection, which must be hello.
func (c *Console) open(ctx context.Context, log *slog.Logger, ev transport.Event) {
	hello, ok := ev.Envelope.Payload.(protocol.HelloInfo)
	if !ok {
		log.Info("first message is not hello", "conn", ev.Conn.ID(), "type", ev.Envelope.Type)
		c.sendError(ctx, log, ev, "first message must be hello")
		ev.Conn.Close(transport.CloseProtocolError, "expected hello")
		return
	}
	s := c.Registry.Connect(ev.Conn.ID(), c.Channel, ev.Conn.RemoteAddr(), hello)
	ev.Conn.StartHeartbeat()
	log.Info("agent joined", "conn", s.ID, "hostname", s.Hostname, "os", s.OS, "version", s.AgentVersion)
}

// route handles a message of an established session.
func (c *Console) route(ctx context.Context, log *slog.Logger, ev transport.Event) {
	c.Registry.Touch(ev.Conn.ID())
	switch ev.Envelope.Type {
	case protocol.TypeHeartbeat:
	case protocol.TypeHello:
		c.sendError(ctx, log, ev, "hello already received")
	default:
		if c.OnMessage != nil {
			c.OnMessage(ctx, ev.Conn, ev.Envelope)
		}
	}
}

func (c *Console) sendError(ctx context.Context, log *slog.Logger, ev transport.Event, msg string) {
	if ev.Envelope.Type == protocol.TypeError {
		return // never answer an error with an error
	}
	p := protocol.NewError(ev.Envelope.ID, protocol.CodeInvalidMessage, msg)
	if _, err := ev.Conn.Send(ctx, p); err != nil {
		log.Debug("cannot send error", "conn", ev.Conn.ID(), "err", err)
	}
}
