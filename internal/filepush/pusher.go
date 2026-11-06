package filepush

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// Defaults of Pusher (docs/protocol.md §5.20).
const (
	DefaultParallel = 4
	DefaultAckWait  = 60 * time.Second
	partSendWait    = 30 * time.Second
)

// Reasons of Outcome besides the protocol's FilePushReason and error codes.
const (
	ReasonTimeout = "timeout" // no ack in AckWait
	ReasonGone    = "gone"    // the parts could not be sent: the session left
)

// SendFunc sends p to one session and returns the envelope ID.
type SendFunc func(ctx context.Context, sessionID string, p protocol.Payload) (string, error)

// Outcome is how a push ended for one session.
type Outcome struct {
	PushID  string
	Session string
	Saved   int      // files acked saved
	Files   int      // files of the push
	Paths   []string // where they were saved, relative to the work folder
	// Reason is empty when every file was saved; otherwise the first
	// failure: a FilePushReason, a protocol error code, ReasonTimeout or
	// ReasonGone.
	Reason string
}

// OK reports whether the session has every file.
func (o Outcome) OK() bool { return o.Reason == "" }

// Pusher sends pushes and waits for their acks. Each session gets the files
// one after the other (it waits for each ack before the next file), and at
// most Parallel sessions are served at once, so a class does not flood the
// network. It is safe for concurrent use.
type Pusher struct {
	Send SendFunc
	// OnOutcome runs once per session and push, from the pusher's goroutines.
	OnOutcome func(Outcome)
	Parallel  int           // 0: DefaultParallel
	AckWait   time.Duration // 0: DefaultAckWait
	Logger    *slog.Logger

	once  sync.Once
	slots chan struct{}
	mu    sync.Mutex
	waits map[waitKey]chan reply // file awaiting its answer
	refs  map[string]waitKey     // envelope ID of a part → its file, for errors
	wg    sync.WaitGroup
}

type waitKey struct{ session, fileID string }

// reply is what came back for one file: an ack or an error.
type reply struct {
	ack    protocol.FilePushAck
	reason string // set for errors
}

// Deliver sends push to sessions in the background, until ctx ends (the
// class closes).
func (p *Pusher) Deliver(ctx context.Context, push *Push, sessions []string) {
	p.init()
	for _, s := range sessions {
		p.wg.Add(1)
		go func(session string) {
			defer p.wg.Done()
			select {
			case p.slots <- struct{}{}:
			case <-ctx.Done():
				p.done(Outcome{PushID: push.ID, Session: session, Files: len(push.Files), Reason: ReasonGone})
				return
			}
			o := p.deliverTo(ctx, push, session)
			<-p.slots
			p.done(o)
		}(s)
	}
}

// Wait blocks until every delivery in progress ended.
func (p *Pusher) Wait() { p.wg.Wait() }

func (p *Pusher) deliverTo(ctx context.Context, push *Push, session string) Outcome {
	o := Outcome{PushID: push.ID, Session: session, Files: len(push.Files)}
	log := p.log().With("push_id", push.ID, "conn", session)
	for i, f := range push.Files {
		key := waitKey{session, f.ID}
		ch := make(chan reply, 1)
		p.mu.Lock()
		p.waits[key] = ch
		p.mu.Unlock()
		var envs []string
		sent := true
		for _, part := range push.Parts(i) {
			sctx, cancel := context.WithTimeout(ctx, partSendWait)
			id, err := p.Send(sctx, session, part)
			cancel()
			if err != nil {
				log.Info("file push not sent", "file_id", f.ID, "part", part.Part, "err", err)
				sent = false
				break
			}
			envs = append(envs, id)
			p.mu.Lock()
			p.refs[id] = key
			p.mu.Unlock()
		}
		var r reply
		if !sent {
			r.reason = ReasonGone
		} else {
			// An answer may already be waiting; if not, wait for it.
			select {
			case r = <-ch:
			case <-time.After(p.ackWait()):
				r.reason = ReasonTimeout
			case <-ctx.Done():
				r.reason = ReasonGone
			}
		}
		p.mu.Lock()
		delete(p.waits, key)
		for _, id := range envs {
			delete(p.refs, id)
		}
		p.mu.Unlock()
		switch {
		case r.reason != "":
		case r.ack.Status == protocol.FilePushSaved:
			o.Saved++
			o.Paths = append(o.Paths, r.ack.Path)
			continue
		default:
			r.reason = string(r.ack.Reason)
		}
		if o.Reason == "" {
			o.Reason = r.reason
		}
		log.Info("file push failed", "file_id", f.ID, "reason", r.reason)
		if r.reason == ReasonGone || r.reason == string(protocol.FilePushUnsupported) ||
			r.reason == string(protocol.CodeUnsupportedCapability) {
			break // the rest would fail the same way
		}
	}
	log.Info("file push delivered", "saved", o.Saved, "files", o.Files, "reason", o.Reason)
	return o
}

// Ack takes a file.push.ack of session. It reports whether a delivery was
// waiting for it.
func (p *Pusher) Ack(session string, ack protocol.FilePushAck) bool {
	return p.answer(waitKey{session, ack.FileID}, reply{ack: ack})
}

// Error takes an error of session; it fails the file whose part it answers.
// It reports whether it answered one.
func (p *Pusher) Error(session string, e protocol.ErrorPayload) bool {
	p.mu.Lock()
	key, ok := p.refs[e.RefID]
	p.mu.Unlock()
	if !ok || key.session != session {
		return false
	}
	return p.answer(key, reply{reason: string(e.Code)})
}

func (p *Pusher) answer(key waitKey, r reply) bool {
	p.init()
	p.mu.Lock()
	ch, ok := p.waits[key]
	p.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- r:
	default: // already answered
	}
	return true
}

func (p *Pusher) done(o Outcome) {
	if p.OnOutcome != nil {
		p.OnOutcome(o)
	}
}

func (p *Pusher) init() {
	p.once.Do(func() {
		n := p.Parallel
		if n <= 0 {
			n = DefaultParallel
		}
		p.slots = make(chan struct{}, n)
		p.mu.Lock()
		p.waits, p.refs = map[waitKey]chan reply{}, map[string]waitKey{}
		p.mu.Unlock()
	})
}

func (p *Pusher) ackWait() time.Duration {
	if p.AckWait > 0 {
		return p.AckWait
	}
	return DefaultAckWait
}

func (p *Pusher) log() *slog.Logger {
	if p.Logger != nil {
		return p.Logger
	}
	return slog.New(slog.DiscardHandler)
}
