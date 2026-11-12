package agentsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/broadcast"
)

// Calls and events of student.present.* (docs/protocol.md §5.24). The
// helper captures, encodes and holds the send-only WebRTC connection, in the
// student's session and with the student's rights, never as SYSTEM
// (docs/privacidad.md rule 17); the service only passes the signaling.
const (
	CallPresentOpen      = "present.open"      // broadcast.PublishOptions → PresentOpenResult
	CallPresentAnswer    = "present.answer"    // PresentAnswerArgs → the answer SDP (string)
	CallPresentCandidate = "present.candidate" // PresentCandidateArgs
	CallPresentClose     = "present.close"     // the publication in force
	// Helper → service.
	EventPresentCandidate = "present.candidate" // PresentCandidateArgs: a local candidate
	EventPresentFail      = "present.fail"      // the publication stopped (string)
)

// Errors of CallPresentOpen, as PresentOpenResult.Error.
const (
	PresentErrCapture = "capture"
	PresentErrEncoder = "encoder"
)

// PresentOpenResult is the reply of CallPresentOpen.
type PresentOpenResult struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Error  string `json:"error,omitempty"` // PresentErrCapture or PresentErrEncoder
}

// PresentAnswerArgs are the arguments of CallPresentAnswer.
type PresentAnswerArgs struct {
	Attempt int    `json:"attempt"`
	Offer   string `json:"offer"`
}

// PresentCandidateArgs carry one ICE candidate of an attempt.
type PresentCandidateArgs struct {
	Attempt       int    `json:"attempt"`
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdpMid"`
	SDPMLineIndex int    `json:"sdpMLineIndex"`
}

// PresentOpenError turns an Open error into PresentOpenResult.Error.
func PresentOpenError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, broadcast.ErrEncoderUnavailable):
		return PresentErrEncoder
	default:
		return PresentErrCapture
	}
}

// presentCallTimeout bounds opening a publication: one capture and the
// encoder's start.
const presentCallTimeout = 20 * time.Second

// Publisher publishes through the helper. It implements broadcast.Publisher;
// the service routes the helper's present.* events to Event.
type Publisher struct {
	Helper *Current

	mu sync.Mutex
	ev *broadcast.PublishEvents // of the publication in force
}

// Open implements broadcast.Publisher.
func (p *Publisher) Open(opt broadcast.PublishOptions, ev broadcast.PublishEvents) (broadcast.Publication, error) {
	l := p.Helper.Get()
	if l == nil {
		return nil, fmt.Errorf("present: %w: %w", broadcast.ErrCaptureUnavailable, ErrNoHelper)
	}
	ctx, cancel := context.WithTimeout(context.Background(), presentCallTimeout)
	defer cancel()
	p.mu.Lock()
	p.ev = &ev // before the call: the helper may trickle at once
	p.mu.Unlock()
	var r PresentOpenResult
	err := l.Call(ctx, CallPresentOpen, opt, &r)
	switch {
	case err != nil:
		err = fmt.Errorf("present: %w: %w", broadcast.ErrCaptureUnavailable, err)
	case r.Error == PresentErrEncoder:
		err = fmt.Errorf("present: %w", broadcast.ErrEncoderUnavailable)
	case r.Error != "":
		err = fmt.Errorf("present: %w", broadcast.ErrCaptureUnavailable)
	}
	if err != nil {
		p.forget(&ev)
		return nil, err
	}
	return &proxyPublication{p: p, ev: &ev, w: r.Width, h: r.Height}, nil
}

// Event takes a present.* event of the helper; it reports whether it was one.
func (p *Publisher) Event(name string, data json.RawMessage) bool {
	switch name {
	case EventPresentCandidate, EventPresentFail:
	default:
		return false
	}
	p.mu.Lock()
	ev := p.ev
	p.mu.Unlock()
	if ev == nil {
		return true
	}
	if name == EventPresentCandidate {
		var c PresentCandidateArgs
		if json.Unmarshal(data, &c) == nil && ev.OnCandidate != nil {
			ev.OnCandidate(c.Attempt, broadcast.Candidate{Candidate: c.Candidate, SDPMid: c.SDPMid, SDPMLineIndex: c.SDPMLineIndex})
		}
		return true
	}
	var why string
	_ = json.Unmarshal(data, &why)
	if ev.OnFail != nil {
		ev.OnFail(fmt.Errorf("present: %w: %s", broadcast.ErrEncoderUnavailable, why))
	}
	return true
}

// HelperGone ends the publication in force: without a helper nothing is
// captured any more.
func (p *Publisher) HelperGone() {
	p.mu.Lock()
	ev := p.ev
	p.ev = nil
	p.mu.Unlock()
	if ev != nil && ev.OnFail != nil {
		ev.OnFail(fmt.Errorf("present: %w", ErrNoHelper))
	}
}

func (p *Publisher) forget(ev *broadcast.PublishEvents) {
	p.mu.Lock()
	if p.ev == ev {
		p.ev = nil
	}
	p.mu.Unlock()
}

type proxyPublication struct {
	p    *Publisher
	ev   *broadcast.PublishEvents
	w, h int
}

func (q *proxyPublication) Size() (int, int) { return q.w, q.h }

func (q *proxyPublication) Answer(attempt int, offer string) (string, error) {
	var answer string
	err := q.p.Helper.call(context.Background(), CallPresentAnswer, PresentAnswerArgs{Attempt: attempt, Offer: offer}, &answer)
	return answer, err
}

func (q *proxyPublication) AddCandidate(attempt int, c broadcast.Candidate) error {
	return q.p.Helper.call(context.Background(), CallPresentCandidate,
		PresentCandidateArgs{Attempt: attempt, Candidate: c.Candidate, SDPMid: c.SDPMid, SDPMLineIndex: c.SDPMLineIndex}, nil)
}

func (q *proxyPublication) Close() error {
	q.p.forget(q.ev)
	err := q.p.Helper.call(context.Background(), CallPresentClose, nil, nil)
	if errors.Is(err, ErrNoHelper) || errors.Is(err, ErrClosed) {
		return nil // nothing on screen any more
	}
	return err
}
