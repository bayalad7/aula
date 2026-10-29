package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/bayalad7/aula/internal/protocol"
)

// EventKind tells what happened on a server connection.
type EventKind int

// Event kinds, in the order they happen on a connection.
const (
	EventConnect EventKind = iota + 1
	EventMessage
	EventDisconnect
)

func (k EventKind) String() string {
	switch k {
	case EventConnect:
		return "connect"
	case EventMessage:
		return "message"
	case EventDisconnect:
		return "disconnect"
	default:
		return fmt.Sprintf("EventKind(%d)", int(k))
	}
}

// Event is delivered by Server.Events. For one connection, events arrive in
// order: one EventConnect, any number of EventMessage, one EventDisconnect.
type Event struct {
	Kind     EventKind
	Conn     *Conn
	Envelope protocol.Envelope // EventMessage only; already decoded and validated
	Err      error             // EventDisconnect only: why the connection ended
}

// Server accepts agent connections. Create it with NewServer, consume Events,
// and run Serve.
type Server struct {
	opts   Options
	events chan Event

	mu    sync.Mutex
	ctx   context.Context
	conns map[*Conn]struct{}
	wg    sync.WaitGroup // one per live connection
}

// NewServer returns a server; nothing listens until Serve is called.
func NewServer(opts Options) *Server {
	return &Server{
		opts:   opts.withDefaults(),
		events: make(chan Event, 64),
		conns:  map[*Conn]struct{}{},
	}
}

// Events returns the stream of connection events. It is closed when Serve
// returns and every connection has finished. The consumer must keep reading:
// a slow consumer slows down the agents (backpressure), it never loses events.
func (s *Server) Events() <-chan Event { return s.events }

// Serve accepts connections on ln until ctx is canceled. Then it stops
// accepting, closes every connection with CloseGoingAway, waits for them and
// closes Events. It returns nil after a clean shutdown.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	if s.opts.TLS != nil {
		ln = tls.NewListener(ln, s.opts.TLS)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(Path, s.handle)
	hs := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	errc := make(chan error, 1)
	go func() { errc <- hs.Serve(ln) }()
	s.opts.Logger.Info("websocket server listening", "addr", ln.Addr().String(), "tls", s.opts.TLS != nil)

	var err error
	select {
	case <-ctx.Done():
	case err = <-errc:
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = hs.Shutdown(shutdownCtx) // stops accepting; hijacked WebSockets are ours to close
	s.mu.Lock()
	for c := range s.conns {
		c.Close(CloseGoingAway, "server shutting down")
	}
	s.mu.Unlock()
	s.wg.Wait()
	close(s.events)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	ctx := s.ctx
	s.mu.Unlock()
	if ctx == nil || ctx.Err() != nil {
		http.Error(w, "shutting down", http.StatusServiceUnavailable)
		return
	}
	// Default origin checks stay on: agents send no Origin header, and a web
	// page in a student's browser must not be able to open this socket.
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		s.opts.Logger.Debug("websocket upgrade rejected", "remote", r.RemoteAddr, "err", err)
		return
	}

	s.wg.Add(1)
	var c *Conn
	ready := make(chan struct{})
	deliver := func(dctx context.Context, env protocol.Envelope) bool {
		<-ready
		return s.emit(dctx, Event{Kind: EventMessage, Conn: c, Envelope: env})
	}
	onClose := func(err error) {
		<-ready
		s.mu.Lock()
		delete(s.conns, c)
		s.mu.Unlock()
		// Deliver the disconnect even during shutdown: Events is only closed
		// after every connection is done.
		s.events <- Event{Kind: EventDisconnect, Conn: c, Err: err}
		s.wg.Done()
	}
	c = newConn(ctx, ws, r.RemoteAddr, s.opts, deliver, onClose)
	s.mu.Lock()
	s.conns[c] = struct{}{}
	s.mu.Unlock()
	s.events <- Event{Kind: EventConnect, Conn: c}
	close(ready)
	c.log.Info("agent connected")

	<-c.Done()
}

func (s *Server) emit(ctx context.Context, e Event) bool {
	select {
	case s.events <- e:
		return true
	case <-ctx.Done():
		return false
	}
}
