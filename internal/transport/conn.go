package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/bayalad7/aula/internal/protocol"
)

// Path is the HTTP path of the WebSocket endpoint.
const Path = "/ws"

// Timing defaults from docs/protocol.md §3.4.
const (
	DefaultHeartbeatInterval = 5 * time.Second
	DefaultIdleTimeout       = 15 * time.Second
	DefaultWriteTimeout      = 10 * time.Second
	DefaultDialTimeout       = 5 * time.Second
	defaultSendQueue         = 32
)

// ErrClosed is returned when sending on a closed connection.
var ErrClosed = errors.New("transport: connection closed")

// CloseCode is a WebSocket close status code used by the protocol (§1.2).
type CloseCode int

// Close codes used by Aula.
const (
	CloseNormal          CloseCode = 1000
	CloseGoingAway       CloseCode = 1001
	CloseProtocolError   CloseCode = 1002
	ClosePolicyViolation CloseCode = 1008
	CloseMessageTooBig   CloseCode = 1009
)

// Options tunes a connection. Zero values take the protocol defaults; tests
// shorten them.
type Options struct {
	HeartbeatInterval time.Duration
	IdleTimeout       time.Duration
	WriteTimeout      time.Duration
	DialTimeout       time.Duration
	SendQueue         int
	Logger            *slog.Logger
	// TLS is the server configuration (Server) or the client configuration
	// (Dial, with a wss:// URL). Nil means plain WebSocket, used only by tests
	// of this package.
	TLS *tls.Config
}

func (o Options) withDefaults() Options {
	if o.HeartbeatInterval <= 0 {
		o.HeartbeatInterval = DefaultHeartbeatInterval
	}
	if o.IdleTimeout <= 0 {
		o.IdleTimeout = DefaultIdleTimeout
	}
	if o.WriteTimeout <= 0 {
		o.WriteTimeout = DefaultWriteTimeout
	}
	if o.DialTimeout <= 0 {
		o.DialTimeout = DefaultDialTimeout
	}
	if o.SendQueue <= 0 {
		o.SendQueue = defaultSendQueue
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return o
}

// Conn is one WebSocket connection carrying protocol envelopes.
type Conn struct {
	id     string
	remote string
	ws     *websocket.Conn
	opts   Options
	log    *slog.Logger

	ctx    context.Context // canceled when the connection is finished
	cancel context.CancelFunc
	out    chan []byte

	// deliver hands a decoded message to the owner; it returns false if the
	// owner is gone.
	deliver func(ctx context.Context, env protocol.Envelope) bool
	onClose func(err error)

	lastRecv atomic.Int64   // unix nanoseconds
	wg       sync.WaitGroup // writer, watchdog and heartbeat goroutines
	done     chan struct{}
	err      error // set before done is closed

	// Close requests go through the writer so queued messages (such as an
	// error reply) are sent before the close frame.
	closeOnce   sync.Once
	closeCh     chan struct{} // closed by Close
	closeCode   CloseCode     // set before closeCh is closed
	closeReason string

	mu        sync.Mutex // guards the fields below
	heartbeat bool
	finished  bool
}

// newConn starts the goroutines of a connection. The connection is closed
// with CloseGoingAway when parent is canceled.
func newConn(parent context.Context, ws *websocket.Conn, remote string, opts Options,
	deliver func(context.Context, protocol.Envelope) bool, onClose func(error)) *Conn {
	ws.SetReadLimit(protocol.MaxMessageSize)
	// The internal context outlives parent so the close handshake can finish.
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	c := &Conn{
		id:      protocol.NewID(),
		remote:  remote,
		ws:      ws,
		opts:    opts,
		ctx:     ctx,
		cancel:  cancel,
		out:     make(chan []byte, opts.SendQueue),
		deliver: deliver,
		onClose: onClose,
		done:    make(chan struct{}),
		closeCh: make(chan struct{}),
	}
	c.log = opts.Logger.With("conn", c.id, "remote", remote)
	c.lastRecv.Store(time.Now().UnixNano())

	c.wg.Add(2)
	go c.writeLoop()
	go c.watchdog(parent)
	go c.readLoop()
	return c
}

// ID is a random identifier of this connection, unique per process.
func (c *Conn) ID() string { return c.id }

// RemoteAddr is the peer address (server side) or the dialed URL (client side).
func (c *Conn) RemoteAddr() string { return c.remote }

// Done is closed once the connection is completely finished.
func (c *Conn) Done() <-chan struct{} { return c.done }

// Err reports why the connection finished. Only valid after Done is closed.
func (c *Conn) Err() error { return c.err }

// Send wraps p in a new envelope and queues it. It returns the envelope ID, to
// correlate replies (ref_id). It blocks while the queue is full.
func (c *Conn) Send(ctx context.Context, p protocol.Payload) (string, error) {
	env := protocol.New(p)
	data, err := protocol.Encode(env)
	if err != nil {
		return "", fmt.Errorf("send %s: %w", p.MessageType(), err)
	}
	// Checked first: with room in the queue, select would pick at random.
	select {
	case <-c.ctx.Done():
		return "", ErrClosed
	case <-c.closeCh:
		return "", ErrClosed
	default:
	}
	select {
	case c.out <- data:
		return env.ID, nil
	case <-c.ctx.Done():
		return "", ErrClosed
	case <-c.closeCh:
		return "", ErrClosed
	case <-ctx.Done():
		return "", fmt.Errorf("send %s: %w", p.MessageType(), ctx.Err())
	}
}

// StartHeartbeat starts sending heartbeat every HeartbeatInterval. It is
// called once the session is established (after hello); later calls are no-ops.
func (c *Conn) StartHeartbeat() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.heartbeat || c.finished {
		return
	}
	c.heartbeat = true
	c.wg.Add(1)
	go c.heartbeatLoop()
}

// Close starts the close handshake with the given code. It returns
// immediately; wait on Done for the end. Later calls are no-ops.
func (c *Conn) Close(code CloseCode, reason string) {
	c.closeOnce.Do(func() {
		c.log.Debug("closing connection", "code", int(code), "reason", reason)
		c.closeCode, c.closeReason = code, reason
		close(c.closeCh)
	})
}

// abort drops the connection without handshake (dead peer, write failure).
func (c *Conn) abort() { _ = c.ws.CloseNow() }

func (c *Conn) readLoop() {
	var err error
	for {
		var typ websocket.MessageType
		var data []byte
		typ, data, err = c.ws.Read(c.ctx)
		if err != nil {
			break
		}
		c.lastRecv.Store(time.Now().UnixNano())
		if typ != websocket.MessageText {
			c.reply("", protocol.CodeInvalidMessage, "binary messages are not supported")
			continue
		}
		env, derr := protocol.Decode(data)
		if derr != nil {
			c.handleDecodeError(env, derr)
			continue
		}
		c.log.Debug("message received", "type", env.Type)
		if !c.deliver(c.ctx, env) {
			c.Close(CloseGoingAway, "shutting down")
		}
	}
	c.finish(err)
}

// handleDecodeError answers a bad message as docs/protocol.md §2.1 requires.
func (c *Conn) handleDecodeError(env protocol.Envelope, err error) {
	code := protocol.ErrorCodeFor(err)
	c.log.Debug("invalid message", "type", env.Type, "code", code)
	// Never answer an error with another error.
	if env.Type != protocol.TypeError {
		c.reply(env.ID, code, err.Error())
	}
	if code == protocol.CodeUnsupportedVersion {
		c.Close(ClosePolicyViolation, "unsupported protocol version")
	}
}

func (c *Conn) reply(refID string, code protocol.ErrorCode, msg string) {
	if _, err := c.Send(c.ctx, protocol.NewError(refID, code, msg)); err != nil && !errors.Is(err, ErrClosed) {
		c.log.Warn("cannot send error reply", "err", err)
	}
}

// writeLoop is the only goroutine that writes to the socket.
func (c *Conn) writeLoop() {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			return
		case data := <-c.out:
			if !c.write(data) {
				return
			}
		case <-c.closeCh:
			// Flush what was queued before Close, then do the close handshake;
			// the read loop receives the peer's answer and finishes the Conn.
			for len(c.out) > 0 {
				if !c.write(<-c.out) {
					return
				}
			}
			_ = c.ws.Close(websocket.StatusCode(c.closeCode), c.closeReason)
			return
		}
	}
}

func (c *Conn) write(data []byte) bool {
	wctx, cancel := context.WithTimeout(c.ctx, c.opts.WriteTimeout)
	defer cancel()
	if err := c.ws.Write(wctx, websocket.MessageText, data); err != nil {
		c.log.Debug("write failed", "err", err)
		c.abort()
		return false
	}
	return true
}

func (c *Conn) heartbeatLoop() {
	defer c.wg.Done()
	t := time.NewTicker(c.opts.HeartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-t.C:
			if _, err := c.Send(c.ctx, protocol.Heartbeat{}); err != nil {
				return
			}
		}
	}
}

// watchdog closes the connection when the peer goes silent for IdleTimeout or
// when the owner's context (parent) is canceled.
func (c *Conn) watchdog(parent context.Context) {
	defer c.wg.Done()
	t := time.NewTicker(c.opts.IdleTimeout / 5)
	defer t.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-parent.Done():
			c.Close(CloseGoingAway, "shutting down")
			return
		case <-t.C:
			idle := time.Since(time.Unix(0, c.lastRecv.Load()))
			if idle > c.opts.IdleTimeout {
				c.log.Info("peer silent, dropping connection", "idle", idle.Round(time.Millisecond))
				c.abort()
				return
			}
		}
	}
}

// finish runs once, when the read loop ends.
func (c *Conn) finish(err error) {
	c.mu.Lock()
	c.finished = true // no goroutine may be added to wg from now on
	c.mu.Unlock()
	c.cancel()
	_ = c.ws.CloseNow()
	c.wg.Wait()
	c.err = err
	c.log.Debug("connection finished", "status", int(websocket.CloseStatus(err)))
	if c.onClose != nil {
		c.onClose(err)
	}
	close(c.done)
}
