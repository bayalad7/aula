package agentsvc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
)

// maxFrame bounds one line of the channel: a screen frame is at most 640 KiB
// of JPEG, ~860 KiB in Base64.
const maxFrame = 4 << 20

// Frame kinds.
const (
	kindCall  = "call"
	kindReply = "reply"
	kindEvent = "event"
)

// frame is one line of the channel between the service and the helper.
type frame struct {
	ID    uint64          `json:"id,omitempty"` // call and its reply
	Kind  string          `json:"kind"`
	Name  string          `json:"name,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// ErrClosed is returned by calls on a link that has ended.
var ErrClosed = errors.New("local channel closed")

// CallFunc answers a call from the other end.
type CallFunc func(ctx context.Context, name string, data json.RawMessage) (any, error)

// EventFunc receives an event from the other end. It runs on the reading
// goroutine, in order: keep it short.
type EventFunc func(name string, data json.RawMessage)

// Link is the local channel between the service and the helper: JSON lines
// carrying calls (with a reply), replies and events, in both directions. It is
// not the console protocol: it never leaves the computer.
type Link struct {
	conn    net.Conn
	onCall  CallFunc
	onEvent EventFunc

	wmu sync.Mutex
	w   *bufio.Writer

	mu      sync.Mutex
	next    uint64
	pending map[uint64]chan frame
	done    chan struct{}
	err     error
}

// NewLink wraps conn. Either handler may be nil. Call Run to start reading.
func NewLink(conn net.Conn, onCall CallFunc, onEvent EventFunc) *Link {
	return &Link{conn: conn, onCall: onCall, onEvent: onEvent, w: bufio.NewWriter(conn),
		pending: map[uint64]chan frame{}, done: make(chan struct{})}
}

// Run reads until the connection ends or ctx is canceled, then closes the
// link and fails the calls in flight. Calls from the other end are answered
// on their own goroutines.
func (l *Link) Run(ctx context.Context) error {
	stop := context.AfterFunc(ctx, func() { l.conn.Close() })
	defer stop()
	sc := bufio.NewScanner(l.conn)
	sc.Buffer(make([]byte, 64<<10), maxFrame)
	var wg sync.WaitGroup
	defer wg.Wait()
	for sc.Scan() {
		var f frame
		if err := json.Unmarshal(sc.Bytes(), &f); err != nil {
			l.close(fmt.Errorf("local channel: bad frame: %w", err))
			return l.err
		}
		switch f.Kind {
		case kindReply:
			l.mu.Lock()
			ch := l.pending[f.ID]
			delete(l.pending, f.ID)
			l.mu.Unlock()
			if ch != nil {
				ch <- f
			}
		case kindEvent:
			if l.onEvent != nil {
				l.onEvent(f.Name, f.Data)
			}
		case kindCall:
			wg.Add(1)
			go func() {
				defer wg.Done()
				l.answer(ctx, f)
			}()
		}
	}
	err := sc.Err()
	if err == nil {
		err = ErrClosed
	}
	l.close(err)
	return nil
}

func (l *Link) answer(ctx context.Context, f frame) {
	reply := frame{ID: f.ID, Kind: kindReply}
	var res any
	err := fmt.Errorf("%s: no handler", f.Name)
	if l.onCall != nil {
		res, err = l.onCall(ctx, f.Name, f.Data)
	}
	if err != nil {
		reply.Error = err.Error()
	} else if res != nil {
		if reply.Data, err = json.Marshal(res); err != nil {
			reply.Error = err.Error()
		}
	}
	_ = l.write(reply) // the caller sees ErrClosed if the link is gone
}

// Call sends a call and waits for its reply, decoding it into out (if not
// nil). A failure at the other end comes back as an error with its text.
func (l *Link) Call(ctx context.Context, name string, args, out any) error {
	data, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	ch := make(chan frame, 1)
	l.mu.Lock()
	if l.err != nil {
		l.mu.Unlock()
		return ErrClosed
	}
	l.next++
	id := l.next
	l.pending[id] = ch
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		delete(l.pending, id)
		l.mu.Unlock()
	}()
	if err := l.write(frame{ID: id, Kind: kindCall, Name: name, Data: data}); err != nil {
		return err
	}
	select {
	case f := <-ch:
		if f.Error != "" {
			return errors.New(f.Error)
		}
		if out != nil && len(f.Data) > 0 {
			if err := json.Unmarshal(f.Data, out); err != nil {
				return fmt.Errorf("%s reply: %w", name, err)
			}
		}
		return nil
	case <-l.done:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Notify sends an event; there is no reply.
func (l *Link) Notify(name string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return l.write(frame{Kind: kindEvent, Name: name, Data: raw})
}

func (l *Link) write(f frame) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if len(b) >= maxFrame {
		return fmt.Errorf("%s: %d bytes exceeds the local channel limit", f.Name, len(b))
	}
	l.wmu.Lock()
	defer l.wmu.Unlock()
	select {
	case <-l.done:
		return ErrClosed
	default:
	}
	if _, err := l.w.Write(append(b, '\n')); err != nil {
		l.close(err)
		return ErrClosed
	}
	if err := l.w.Flush(); err != nil {
		l.close(err)
		return ErrClosed
	}
	return nil
}

// Close ends the link.
func (l *Link) Close() error {
	l.close(ErrClosed)
	return nil
}

// Done is closed when the link has ended.
func (l *Link) Done() <-chan struct{} { return l.done }

func (l *Link) close(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return
	}
	l.err = err
	close(l.done)
	l.conn.Close()
}
