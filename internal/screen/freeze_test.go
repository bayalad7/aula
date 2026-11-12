package screen

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// fakePause is the double of platform.PauseOverlay: it never opens a window.
type fakePause struct {
	ev      *events
	showErr error
	last    platform.PauseText
}

func (p *fakePause) Show(t platform.PauseText) error {
	if p.showErr != nil {
		return p.showErr
	}
	p.last = t
	p.ev.add("show:" + t.Message)
	return nil
}

func (p *fakePause) Yield(on bool) error {
	if on {
		p.ev.add("yield")
	} else {
		p.ev.add("top")
	}
	return nil
}

func (p *fakePause) Hide() error {
	p.ev.add("hide")
	return nil
}

func newFreezer(ev *events) (*Freezer, *fakePause, *[]bool) {
	fp := &fakePause{ev: ev}
	var paused []bool
	f := &Freezer{Overlay: fp, Who: func() (string, string) { return "Prof. Ayala", "1" },
		OnPaused: func(on bool) { paused = append(paused, on) }}
	return f, fp, &paused
}

func freezeEnv(msg string) protocol.Envelope {
	return protocol.New(protocol.ScreenFreeze{Message: msg})
}

func TestFreezeAcksAndOnlyChangesTheMessage(t *testing.T) {
	ev := &events{}
	f, fp, paused := newFreezer(ev)
	env := freezeEnv("Miren al pizarrón")
	reply, ok := f.Handle(context.Background(), env)
	if !ok || reply != (protocol.ScreenFreezeAck{RefID: env.ID, Frozen: true}) {
		t.Fatalf("freeze: %v %#v", ok, reply)
	}
	if fp.last.Title != PauseTitle || !strings.Contains(fp.last.Detail, "Prof. Ayala · canal 1") || fp.last.Note != PauseNote {
		t.Fatalf("text: %#v", fp.last)
	}
	env2 := freezeEnv("Ahora sí")
	reply, _ = f.Handle(context.Background(), env2)
	if reply != (protocol.ScreenFreezeAck{RefID: env2.ID, Frozen: true}) {
		t.Fatalf("second freeze: %#v", reply)
	}
	// The second freeze replaces the text without hiding the windows (no flicker).
	if got, want := ev.list(), []string{"top", "show:Miren al pizarrón", "show:Ahora sí"}; !slices.Equal(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}
	if !slices.Equal(*paused, []bool{true}) || !f.Frozen() {
		t.Fatalf("paused %v frozen %v", *paused, f.Frozen())
	}
}

func TestUnfreezeIsIdempotent(t *testing.T) {
	ev := &events{}
	f, _, paused := newFreezer(ev)
	f.Handle(context.Background(), freezeEnv(""))
	for range 2 {
		env := protocol.New(protocol.ScreenUnfreeze{})
		reply, ok := f.Handle(context.Background(), env)
		if !ok || reply != (protocol.ScreenFreezeAck{RefID: env.ID, Frozen: false}) {
			t.Fatalf("unfreeze: %v %#v", ok, reply)
		}
	}
	if got := ev.list(); slices.Index(got, "hide") != len(got)-1 || strings.Count(strings.Join(got, ","), "hide") != 1 {
		t.Fatalf("events %v: one hide expected", got)
	}
	if !slices.Equal(*paused, []bool{true, false}) || f.Frozen() {
		t.Fatalf("paused %v frozen %v", *paused, f.Frozen())
	}
}

func TestStopRemovesThePauseOnDisconnect(t *testing.T) {
	ev := &events{}
	f, _, paused := newFreezer(ev)
	f.Stop() // nothing up: nothing to do
	f.Handle(context.Background(), freezeEnv("x"))
	f.Stop() // the connection closed (rule 10)
	if got := ev.list(); got[len(got)-1] != "hide" || f.Frozen() {
		t.Fatalf("events %v frozen %v", got, f.Frozen())
	}
	if !slices.Equal(*paused, []bool{true, false}) {
		t.Fatalf("paused %v", *paused)
	}
}

func TestFreezeWithoutOverlayIsUnsupported(t *testing.T) {
	f := &Freezer{}
	if f.Capabilities() != nil {
		t.Fatal("no overlay, no capability")
	}
	for _, p := range []protocol.Payload{protocol.ScreenFreeze{}, protocol.ScreenUnfreeze{}} {
		env := protocol.New(p)
		reply, ok := f.Handle(context.Background(), env)
		e, isErr := reply.(protocol.ErrorPayload)
		if !ok || !isErr || e.Code != protocol.CodeUnsupportedCapability || e.RefID != env.ID {
			t.Fatalf("%s: %v %#v", p.MessageType(), ok, reply)
		}
	}
	if caps := (&Freezer{Overlay: &fakePause{ev: &events{}}}).Capabilities(); !slices.Equal(caps, []protocol.Capability{protocol.CapScreenFreeze}) {
		t.Fatalf("caps %v", caps)
	}
}

func TestFreezeThatCannotShowReportsInternal(t *testing.T) {
	ev := &events{}
	f, fp, paused := newFreezer(ev)
	fp.showErr = errors.New("no desktop")
	env := freezeEnv("")
	reply, _ := f.Handle(context.Background(), env)
	if e, ok := reply.(protocol.ErrorPayload); !ok || e.Code != protocol.CodeInternal || e.RefID != env.ID {
		t.Fatalf("reply %#v", reply)
	}
	if f.Frozen() || len(*paused) != 0 {
		t.Fatal("a pause that was not shown is not a pause")
	}
}

func TestYieldFollowsWhatTheStudentMustSee(t *testing.T) {
	ev := &events{}
	f, _, _ := newFreezer(ev)
	f.Yield(true) // a broadcast is on before the pause
	f.Handle(context.Background(), freezeEnv("a"))
	f.Yield(true) // no change
	f.Yield(false)
	f.Stop()
	f.Yield(true) // no pause: only remembered
	if got, want := ev.list(), []string{"yield", "show:a", "top", "hide"}; !slices.Equal(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}
}

func TestPauseTextWithoutTeacher(t *testing.T) {
	pt := PauseTextFor("", "", "")
	if !strings.HasPrefix(pt.Detail, "Tu profesor.") || pt.Message != "" {
		t.Fatalf("%#v", pt)
	}
}

func TestPausesFollowAcksErrorsAndDisconnections(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	p := &Pauses{Now: func() time.Time { return now }}
	p.Sent("a", "r1", true)
	p.Sent("b", "r2", true)
	p.Sent("c", "r3", true)
	p.SetMessage("Ojos al frente")
	if s := p.Summary(); s.Paused != 3 || s.Unacked != 0 || s.Message != "Ojos al frente" {
		t.Fatalf("summary %+v", s)
	}
	if !p.Ack("a", protocol.ScreenFreezeAck{RefID: "r1", Frozen: true}) {
		t.Fatal("ack a")
	}
	if p.Ack("b", protocol.ScreenFreezeAck{RefID: "old", Frozen: true}) {
		t.Fatal("an ack of another message changes nothing")
	}
	if !p.Error("c", "r3") {
		t.Fatal("error c")
	}
	now = now.Add(AckWait)
	if s := p.Summary(); s.Paused != 2 || s.Unacked != 1 || s.Failed != 1 {
		t.Fatalf("summary %+v", s)
	}
	if e, _ := p.Get("a"); e.Status != PausePaused || !p.Active("a") || p.Active("c") {
		t.Fatalf("a %+v", e)
	}
	// Resuming a.
	p.Sent("a", "r4", false)
	if p.Active("a") {
		t.Fatal("a is resuming")
	}
	p.Ack("a", protocol.ScreenFreezeAck{RefID: "r4", Frozen: false})
	if _, ok := p.Get("a"); ok {
		t.Fatal("a resumed: forgotten")
	}
	// b disconnects: its agent removed the pause by itself.
	if !p.Prune(map[string]bool{"c": true}) {
		t.Fatal("prune")
	}
	if _, ok := p.Get("b"); ok {
		t.Fatal("b left")
	}
	p.Reset()
	if s := p.Summary(); s != (PauseSummary{}) {
		t.Fatalf("after reset %+v", s)
	}
}
