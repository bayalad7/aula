package main

import (
	"context"
	"errors"
	"image"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

type fakeMonitors struct{}

func (fakeMonitors) Displays() ([]platform.Display, error) {
	return []platform.Display{
		{Index: 0, Bounds: image.Rect(0, 0, 640, 360), Primary: true},
		{Index: 1, Bounds: image.Rect(640, 0, 1280, 360)},
	}, nil
}

func (fakeMonitors) Grab(context.Context, int) (*image.RGBA, error) {
	return image.NewRGBA(image.Rect(0, 0, 640, 360)), nil
}

type fakeH264 struct{}

func (fakeH264) Encode(*image.RGBA, bool) ([]byte, error) {
	return []byte{0, 0, 0, 1, 0x65, 1, 2, 3}, nil
}
func (fakeH264) Close() error { return nil }

// excluder records the console's capture affinity.
type excluder struct {
	mu    sync.Mutex
	calls []bool
	err   error
}

func (e *excluder) set(on bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.err != nil {
		return e.err
	}
	e.calls = append(e.calls, on)
	return nil
}

func (e *excluder) last() (bool, int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.calls) == 0 {
		return false, 0
	}
	return e.calls[len(e.calls)-1], len(e.calls)
}

func broadcastHarness(t *testing.T) (*harness, *excluder) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapCodeView, protocol.CapScreenBroadcast),
		student("s2", "PC02", protocol.CapCodeView)) // an agent that cannot receive it
	ex := &excluder{}
	h.app.exclude = ex.set
	h.app.bc.Capture = fakeMonitors{}
	h.app.bc.NewEncoder = func(platform.VideoEncoderConfig) (platform.VideoEncoder, error) { return fakeH264{}, nil }
	h.app.bc.Media.Loopback = true
	h.app.broadcastUDPPort = 0
	t.Cleanup(func() { h.app.StopBroadcast() })
	return h, ex
}

// waitDirect collects messages sent to single sessions until sessionID got
// want of them.
func waitDirect(t *testing.T, h *harness, sessionID string, want int) []directSend {
	t.Helper()
	var got []directSend
	deadline := time.Now().Add(3 * time.Second)
	for len(types(got, sessionID)) < want {
		got = append(got, h.class.takeDirect()...)
		if time.Now().After(deadline) {
			t.Fatalf("%s got %v, want %d messages", sessionID, types(got, sessionID), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return got
}

func types(ds []directSend, sessionID string) []protocol.Type {
	var out []protocol.Type
	for _, d := range ds {
		if d.sessionID == sessionID {
			out = append(out, d.p.MessageType())
		}
	}
	return out
}

func TestBroadcastStartsHidesTheConsoleAndFollowsTheClass(t *testing.T) {
	h, ex := broadcastHarness(t)
	ds := h.app.Displays()
	if len(ds) != 2 || ds[0].Label != "Monitor 1 (principal) · 640×360" || !ds[0].Primary {
		t.Fatalf("displays %+v", ds)
	}
	if v := h.app.View(); v.Students.CanBroadcast != 1 || v.Broadcast.Active || !v.Broadcast.Available {
		t.Fatalf("view before %+v / %+v", v.Students, v.Broadcast)
	}

	st, err := h.app.StartBroadcast(0)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Active || st.Display != 0 {
		t.Fatalf("state %+v", st)
	}
	if on, n := ex.last(); !on || n != 1 {
		t.Fatal("console not excluded from the capture before broadcasting")
	}
	got := waitDirect(t, h, "s1", 2)
	if ts := types(got, "s1"); len(ts) < 2 || ts[0] != protocol.TypeScreenBroadcastStart || ts[1] != protocol.TypeScreenBroadcastOffer {
		t.Fatalf("s1 got %v", ts)
	}
	if ts := types(got, "s2"); len(ts) != 0 {
		t.Fatalf("an agent without screen.broadcast got %v", ts)
	}
	if _, err := h.app.StartBroadcast(1); err == nil {
		t.Fatal("second broadcast accepted")
	}

	// A student who joins while broadcasting receives it too.
	h.class.mu.Lock()
	h.class.sessions = append(h.class.sessions, student("s3", "PC03", protocol.CapScreenBroadcast))
	h.class.mu.Unlock()
	h.hooks.onChange()
	if ts := types(waitDirect(t, h, "s3", 2), "s3"); len(ts) < 2 || ts[0] != protocol.TypeScreenBroadcastStart {
		t.Fatalf("s3 got %v", ts)
	}
	waitState := func(cond func(BroadcastState) bool) BroadcastState {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			s, _ := h.rec.last(EventBroadcast).(BroadcastState)
			if cond(s) {
				return s
			}
			if time.Now().After(deadline) {
				t.Fatalf("last broadcast state %+v", s)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitState(func(s BroadcastState) bool { return s.Active && s.Eligible == 2 && s.Receivers == 0 })

	// Closing the class stops the broadcast first: every receiver gets stop.
	h.app.EndClass()
	stops := map[string]bool{}
	for _, d := range h.class.takeDirect() {
		if p, ok := d.p.(protocol.ScreenBroadcastStop); ok && p.BroadcastID != "" {
			stops[d.sessionID] = true
		}
	}
	if !stops["s1"] || !stops["s3"] || stops["s2"] {
		t.Fatalf("stops %v, want s1 and s3", stops)
	}
	if on, _ := ex.last(); on {
		t.Fatal("console still excluded from captures after the broadcast")
	}
	waitState(func(s BroadcastState) bool { return !s.Active && s.Display == -1 })
	if _, err := h.app.StartBroadcast(0); !errors.Is(err, errNoClass) {
		t.Fatalf("StartBroadcast without class = %v", err)
	}
}

func TestBroadcastRefusedWhenTheConsoleCannotBeHidden(t *testing.T) {
	h, ex := broadcastHarness(t)
	ex.err = errors.New("access denied")
	if _, err := h.app.StartBroadcast(0); err == nil {
		t.Fatal("broadcast started with the console visible in the capture")
	}
	time.Sleep(50 * time.Millisecond)
	if d := h.class.takeDirect(); len(d) != 0 {
		t.Fatalf("sent %v", d)
	}
	if h.app.BroadcastState().Active {
		t.Fatal("broadcast active")
	}
}

func TestBroadcastWithoutEncoderExplainsWhy(t *testing.T) {
	h, ex := broadcastHarness(t)
	h.app.bc.NewEncoder = func(platform.VideoEncoderConfig) (platform.VideoEncoder, error) {
		return nil, platform.ErrNotSupported
	}
	_, err := h.app.StartBroadcast(0)
	if err == nil || !strings.Contains(err.Error(), "H.264") || !strings.Contains(err.Error(), "Media Feature Pack") {
		t.Fatalf("error %v", err)
	}
	if on, _ := ex.last(); on {
		t.Fatal("console left excluded after a failed start")
	}
}
