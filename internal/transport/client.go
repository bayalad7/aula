package transport

import (
	"context"
	"fmt"

	"github.com/coder/websocket"

	"github.com/bayalad7/aula/internal/protocol"
)

// Client is the agent side of a connection.
type Client struct {
	*Conn
	in chan protocol.Envelope
}

// Dial connects to a console at url ("ws://host:port/ws"). The handshake is
// bounded by Options.DialTimeout; the connection lives until ctx is canceled
// or Close is called.
func Dial(ctx context.Context, url string, opts Options) (*Client, error) {
	opts = opts.withDefaults()
	dctx, cancel := context.WithTimeout(ctx, opts.DialTimeout)
	defer cancel()
	ws, _, err := websocket.Dial(dctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", url, err)
	}
	cl := &Client{in: make(chan protocol.Envelope, 64)}
	deliver := func(dctx context.Context, env protocol.Envelope) bool {
		select {
		case cl.in <- env:
			return true
		case <-dctx.Done():
			return false
		}
	}
	cl.Conn = newConn(ctx, ws, url, opts, deliver, func(error) { close(cl.in) })
	return cl, nil
}

// Incoming delivers the messages received from the console, decoded and
// validated. It is closed when the connection finishes. The caller must keep
// reading it; while it is full, reading from the socket pauses.
func (c *Client) Incoming() <-chan protocol.Envelope { return c.in }
