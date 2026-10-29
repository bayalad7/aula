package main

import (
	"context"
	"image"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/screen"
	"github.com/bayalad7/aula/internal/session"
)

type stubCapture struct{}

func (stubCapture) Capture(_ context.Context, maxWidth, quality int) (platform.Frame, error) {
	return platform.EncodeFrame(image.NewRGBA(image.Rect(0, 0, 640, 360)), maxWidth, quality)
}

type stubIndicator struct {
	mu    sync.Mutex
	shown bool
}

func (i *stubIndicator) Show(string) error { i.mu.Lock(); i.shown = true; i.mu.Unlock(); return nil }
func (i *stubIndicator) Hide() error       { i.mu.Lock(); i.shown = false; i.mu.Unlock(); return nil }
func (i *stubIndicator) on() bool          { i.mu.Lock(); defer i.mu.Unlock(); return i.shown }

func TestScreenViewedBannerAndStopOnDisconnect(t *testing.T) {
	a, rec := newTestApp(nil)
	ind := &stubIndicator{}
	a.enableScreen(stubCapture{}, ind)
	if !protocol.Has(a.hello.Capabilities, protocol.CapScreenThumb) {
		t.Fatalf("hello capabilities = %v, want screen.thumb", a.hello.Capabilities)
	}
	a.onState(session.AgentConnected, discovery.Console{Name: "Prof. Ayala"})

	frames := make(chan protocol.Payload, 10)
	send := func(_ context.Context, p protocol.Payload) error { frames <- p; return nil }
	a.screen.Handle(t.Context(), send, protocol.New(screen.ThumbParams))
	select {
	case p := <-frames:
		if _, ok := p.(protocol.ScreenThumbFrame); !ok {
			t.Fatalf("got %T, want a frame", p)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no frame")
	}
	st := rec.last(EventStatus).(Status)
	if !ind.on() || !st.Viewed || !strings.Contains(st.Banner, "tu pantalla está siendo vista por Prof. Ayala") {
		t.Fatalf("indicator %v, status %+v", ind.on(), st)
	}

	a.onState(session.AgentDisconnected, discovery.Console{Name: "Prof. Ayala"})
	if ind.on() {
		t.Fatal("indicator still shown after the connection closed")
	}
	if st := rec.last(EventStatus).(Status); st.Viewed || st.State != StateDisconnected {
		t.Fatalf("status after disconnect = %+v", st)
	}
}

func TestNoScreenCapabilityWithoutPlatform(t *testing.T) {
	a, _ := newTestApp(nil)
	if protocol.Has(a.hello.Capabilities, protocol.CapScreenThumb) {
		t.Fatal("screen.thumb declared without capture")
	}
}
