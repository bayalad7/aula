package agentsvc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/broadcast"
)

func TestPublisherProxy(t *testing.T) {
	cur := &Current{}
	pub := &Publisher{Helper: cur}
	if _, err := pub.Open(broadcast.PublishOptions{}, broadcast.PublishEvents{}); !errors.Is(err, broadcast.ErrCaptureUnavailable) {
		t.Fatalf("open without helper = %v", err)
	}

	// The service's side gets the helper's events; the helper answers the
	// calls the way helper.go does.
	var helper *Link
	var mu sync.Mutex
	var calls []string
	openErr := ""
	ca, cb := net.Pipe()
	service := NewLink(ca, nil, func(name string, data json.RawMessage) { pub.Event(name, data) })
	helper = NewLink(cb, func(_ context.Context, name string, data json.RawMessage) (any, error) {
		mu.Lock()
		calls = append(calls, name)
		mu.Unlock()
		switch name {
		case CallPresentOpen:
			var o broadcast.PublishOptions
			if err := json.Unmarshal(data, &o); err != nil || o.FPS != 15 {
				return nil, errors.New("bad options")
			}
			if openErr != "" {
				return PresentOpenResult{Error: openErr}, nil
			}
			return PresentOpenResult{Width: 1280, Height: 720}, nil
		case CallPresentAnswer:
			var a PresentAnswerArgs
			if err := json.Unmarshal(data, &a); err != nil {
				return nil, err
			}
			go helper.Notify(EventPresentCandidate, PresentCandidateArgs{Attempt: a.Attempt, Candidate: "candidate:1 1 udp 1 10.0.0.2 5000 typ host", SDPMid: "0"})
			return "v=0 answer", nil
		case CallPresentCandidate, CallPresentClose:
			return nil, nil
		}
		return nil, errors.New("unexpected " + name)
	}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	go service.Run(ctx)
	go helper.Run(ctx)
	t.Cleanup(func() { cancel(); service.Close(); helper.Close() })
	cur.Set(service)

	got := make(chan broadcast.Candidate, 4)
	failed := make(chan error, 1)
	p, err := pub.Open(broadcast.PublishOptions{MaxWidth: 1920, FPS: 15, MaxKbps: 2500}, broadcast.PublishEvents{
		OnCandidate: func(attempt int, c broadcast.Candidate) {
			if attempt == 1 {
				got <- c
			}
		},
		OnFail: func(err error) { failed <- err },
	})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 1280 || h != 720 {
		t.Fatalf("size %dx%d", w, h)
	}
	if a, err := p.Answer(1, "v=0 offer"); err != nil || a != "v=0 answer" {
		t.Fatalf("answer %q, %v", a, err)
	}
	select {
	case c := <-got:
		if c.SDPMid != "0" {
			t.Fatalf("candidate %+v", c)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the helper's candidate did not arrive")
	}
	if err := p.AddCandidate(1, broadcast.Candidate{Candidate: "candidate:2 1 udp 1 10.0.0.1 47831 typ host"}); err != nil {
		t.Fatal(err)
	}
	// The helper's publication stops: the presenter hears of it.
	_ = helper.Notify(EventPresentFail, "encoder")
	select {
	case err := <-failed:
		if !errors.Is(err, broadcast.ErrEncoderUnavailable) {
			t.Fatalf("fail = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the failure did not arrive")
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	// After Close, late events go nowhere.
	if !pub.Event(EventPresentFail, json.RawMessage(`"late"`)) {
		t.Fatal("present event not recognized")
	}

	openErr = PresentErrEncoder
	if _, err := pub.Open(broadcast.PublishOptions{FPS: 15}, broadcast.PublishEvents{}); !errors.Is(err, broadcast.ErrEncoderUnavailable) {
		t.Fatalf("open with no encoder = %v", err)
	}
	if PresentOpenError(broadcast.ErrEncoderUnavailable) != PresentErrEncoder || PresentOpenError(errors.New("x")) != PresentErrCapture || PresentOpenError(nil) != "" {
		t.Fatal("PresentOpenError")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) < 4 {
		t.Fatalf("calls %v", calls)
	}
}
