package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/screen"
	"github.com/bayalad7/aula/internal/session"
)

func TestPauseTargetsAcksMessageAndResume(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapCodeView, protocol.CapScreenFreeze),
		student("s2", "PC02", protocol.CapCodeView, protocol.CapScreenFreeze),
		student("s3", "MAC03", protocol.CapCodeView)) // no pause window (not Windows)
	if st := h.app.View().Students; st.CanPause != 2 || !st.Students[0].CanPause || st.Students[2].CanPause {
		t.Fatalf("counts %+v", st)
	}
	res, err := h.app.PauseScreens(PauseRequest{Message: "Miren al pizarrón"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Sent != 2 || !slices.Equal(res.Unsupported, []string{"MAC03"}) {
		t.Fatalf("result %+v", res)
	}
	ds := h.class.takeDirect()
	if got := sentTypes(ds); !slices.Equal(got, []string{"s1:screen.freeze", "s2:screen.freeze"}) {
		t.Fatalf("sent %v", got)
	}
	if f := ds[0].p.(protocol.ScreenFreeze); f.Message != "Miren al pizarrón" {
		t.Fatalf("freeze %+v", f)
	}
	if s := h.rec.last(EventPauses).(screen.PauseSummary); s.Paused != 2 || s.Message != "Miren al pizarrón" {
		t.Fatalf("summary %+v", s)
	}
	if st := h.rec.last(EventStudents).(Students); st.Students[0].Pause != string(screen.PauseSent) {
		t.Fatalf("row %+v", st.Students[0])
	}

	// PC01 acks; PC02 could not show it.
	byID := map[string]directSend{}
	for _, d := range ds {
		byID[d.sessionID] = d
	}
	h.hooks.onPause("s1", protocol.Envelope{Payload: protocol.ScreenFreezeAck{RefID: byID["s1"].envID, Frozen: true}})
	h.hooks.onPause("s2", protocol.Envelope{Payload: protocol.NewError(byID["s2"].envID, protocol.CodeInternal, "no se pudo mostrar la pantalla en pausa")})
	st := h.rec.last(EventStudents).(Students)
	if st.Students[0].Pause != string(screen.PausePaused) || st.Students[1].Pause != string(screen.PauseFailed) {
		t.Fatalf("rows %q %q", st.Students[0].Pause, st.Students[1].Pause)
	}

	// Changing the message only goes to who is paused.
	if r, err := h.app.ChangePauseMessage("Ya casi"); err != nil || r.Sent != 1 {
		t.Fatalf("change %+v, %v", r, err)
	}
	ds = h.class.takeDirect()
	if got := sentTypes(ds); !slices.Equal(got, []string{"s1:screen.freeze"}) || ds[0].p.(protocol.ScreenFreeze).Message != "Ya casi" {
		t.Fatalf("change sent %v", got)
	}
	h.hooks.onPause("s1", protocol.Envelope{Payload: protocol.ScreenFreezeAck{RefID: ds[0].envID, Frozen: true}})

	// Resume everyone: those with a pause (or a failed one) get screen.unfreeze.
	if r := h.app.ResumeScreens(nil); r.Sent != 2 {
		t.Fatalf("resume %+v", r)
	}
	ds = h.class.takeDirect()
	if got := sentTypes(ds); !slices.Equal(got, []string{"s1:screen.unfreeze", "s2:screen.unfreeze"}) {
		t.Fatalf("resume sent %v", got)
	}
	for _, d := range ds {
		h.hooks.onPause(d.sessionID, protocol.Envelope{Payload: protocol.ScreenFreezeAck{RefID: d.envID, Frozen: false}})
	}
	if s := h.app.Pauses(); s.Paused != 0 || s.Failed != 0 {
		t.Fatalf("after resume %+v", s)
	}
	if r := h.app.ResumeScreens(nil); r.Sent != 0 {
		t.Fatalf("a second resume sent %+v", r)
	}
}

func TestPauseRejectsLongMessagesAndNobody(t *testing.T) {
	h := newHarness(t, student("s1", "MAC01", protocol.CapCodeView))
	_, err := h.app.PauseScreens(PauseRequest{Message: strings.Repeat("x", protocol.MaxFreezeMessage+1)})
	if err == nil || !strings.Contains(err.Error(), "aula:"+codeInvalidPauseMessage) {
		t.Fatalf("long message: %v", err)
	}
	if _, err := h.app.PauseScreens(PauseRequest{}); err == nil || !strings.Contains(err.Error(), "aula:"+codeNoPauseTargets) {
		t.Fatalf("nobody can: %v", err)
	}
	if _, err := h.app.ChangePauseMessage(strings.Repeat("x", protocol.MaxFreezeMessage+1)); err == nil {
		t.Fatal("a long message was accepted")
	}
	if d := h.class.takeDirect(); len(d) != 0 {
		t.Fatalf("sent %v after invalid requests", sentTypes(d))
	}
}

// A student who disconnects is not paused any more: its agent removed the
// pause (docs/privacidad.md rule 10).
func TestPausedStudentWhoLeavesIsForgotten(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapScreenFreeze),
		student("s2", "PC02", protocol.CapScreenFreeze))
	if _, err := h.app.PauseScreens(PauseRequest{Hosts: []string{"pc01"}}); err != nil {
		t.Fatal(err)
	}
	if got := sentTypes(h.class.takeDirect()); !slices.Equal(got, []string{"s1:screen.freeze"}) {
		t.Fatalf("sent %v", got)
	}
	h.class.mu.Lock()
	h.class.sessions[0].State = session.StateDisconnected
	h.class.mu.Unlock()
	h.hooks.onChange()
	if s := h.rec.last(EventPauses).(screen.PauseSummary); s.Paused != 0 {
		t.Fatalf("summary %+v", s)
	}
}

// Taking control of a paused student resumes them first: the teacher wants
// the real screen (docs/protocol.md §5.9).
func TestTakeControlResumesAPausedStudent(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapScreenFreeze, protocol.CapRemote, protocol.CapScreenThumb))
	if _, err := h.app.PauseScreens(PauseRequest{}); err != nil {
		t.Fatal(err)
	}
	h.class.takeDirect()
	if _, err := h.app.TakeControl("s1"); err != nil {
		t.Fatal(err)
	}
	var got []protocol.Type
	for _, d := range h.class.takeDirect() {
		got = append(got, d.p.MessageType())
	}
	if len(got) < 2 || got[0] != protocol.TypeScreenUnfreeze || got[1] != protocol.TypeRemoteStart {
		t.Fatalf("sent %v: unfreeze must go before remote.start", got)
	}
	if err := h.app.ReleaseControl(); err != nil {
		t.Fatal(err)
	}
}

// Pausing and resuming 20 students takes well under a second (prompt 10.21).
func TestPauseTwentyStudentsQuickly(t *testing.T) {
	var list []session.Session
	for i := range 20 {
		list = append(list, student(fmt.Sprintf("s%02d", i), fmt.Sprintf("PC%02d", i), protocol.CapScreenFreeze))
	}
	h := newHarness(t, list...)
	start := time.Now()
	if r, err := h.app.PauseScreens(PauseRequest{Message: "Ojos al frente"}); err != nil || r.Sent != 20 {
		t.Fatalf("%+v, %v", r, err)
	}
	for _, d := range h.class.takeDirect() {
		h.hooks.onPause(d.sessionID, protocol.Envelope{Payload: protocol.ScreenFreezeAck{RefID: d.envID, Frozen: true}})
	}
	if r := h.app.ResumeScreens(nil); r.Sent != 20 {
		t.Fatalf("resume %+v", r)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("pausing and resuming 20 took %v", d)
	}
}
