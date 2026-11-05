package main

import (
	"testing"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/remote"
	"github.com/bayalad7/aula/internal/screen"
	"github.com/bayalad7/aula/internal/session"
)

// Take, follow the agent's acks, send input, release: one computer at a time.
func TestRemoteControlTakeInputRelease(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapCodeView, protocol.CapScreenThumb, protocol.CapRemote),
		student("s2", "PC02", protocol.CapCodeView, protocol.CapScreenThumb, protocol.CapRemote),
		student("s3", "PC03", protocol.CapCodeView)) // no remote (Linux, older agent)

	if st := h.app.View().Students.Students; !st[0].CanRemote || st[2].CanRemote {
		t.Fatalf("canRemote = %v, %v", st[0].CanRemote, st[2].CanRemote)
	}
	if _, err := h.app.TakeControl("s3"); errorCode(err) != codeRemoteUnsupported {
		t.Fatalf("TakeControl(no remote) = %v", err)
	}
	if _, err := h.app.TakeControl("nadie"); errorCode(err) != codeRemoteUnsupported {
		t.Fatalf("TakeControl(unknown) = %v", err)
	}
	c, err := h.app.TakeControl("s1")
	if err != nil || c.Status != remote.StatusStarting || c.Hostname != "PC01" {
		t.Fatalf("TakeControl = %+v, %v", c, err)
	}
	d := h.class.takeDirect()
	start, ok := d[0].p.(protocol.RemoteStart)
	if len(d) != 1 || !ok || d[0].sessionID != "s1" || start.MaxWidth != remote.ViewParams.MaxWidth {
		t.Fatalf("sent %v", d)
	}
	if _, err := h.app.TakeControl("s2"); errorCode(err) != codeAlreadyControlling {
		t.Fatalf("second TakeControl = %v", err)
	}
	// No input before the agent shows its band (active).
	if err := h.app.SendRemoteInput([]protocol.InputEvent{{T: protocol.InputKey, Code: "KeyA", Down: true}}); err != nil {
		t.Fatal(err)
	}
	if d := h.class.takeDirect(); len(d) != 0 {
		t.Fatalf("input sent before active: %v", d)
	}
	h.hooks.onRemote("s1", protocol.New(protocol.RemoteAck{RefID: d[0].envID, ControlID: start.ControlID, Status: protocol.RemoteActive}))
	if c := h.rec.last(EventRemote).(remote.Control); c.Status != protocol.RemoteActive {
		t.Fatalf("event %+v", c)
	}
	// Frames of the control go to their own event, not to the thumbnails.
	h.hooks.onFrame("s1", protocol.ScreenThumbFrame{RefID: d[0].envID, Seq: 1, Width: 1280, Height: 720, JPEG: []byte{1}})
	if th, ok := h.rec.last(EventRemoteFrame).(screen.Thumb); !ok || th.SessionID != "s1" || th.Width != 1280 {
		t.Fatalf("remote frame %+v", h.rec.last(EventRemoteFrame))
	}
	if h.rec.last(EventFrame) != nil {
		t.Fatal("a control frame went to the thumbnails")
	}
	// Input: keys outside the protocol's list are skipped.
	if err := h.app.SendRemoteInput([]protocol.InputEvent{
		{T: protocol.InputMove, X: 0.1, Y: 0.1}, {T: protocol.InputMove, X: 0.2, Y: 0.2},
		{T: protocol.InputKey, Code: "MetaLeft", Down: true}, {T: protocol.InputKey, Code: "KeyA", Down: true}}); err != nil {
		t.Fatal(err)
	}
	d2 := h.class.takeDirect()
	in, ok := d2[0].p.(protocol.RemoteInput)
	if len(d2) != 1 || !ok || len(in.Events) != 2 || in.Events[0].X != 0.2 || in.Events[1].Code != "KeyA" || in.ControlID != start.ControlID {
		t.Fatalf("input sent %+v", d2)
	}
	// Closing the view slows the video; the control goes on.
	if err := h.app.ShowControl(false); err != nil {
		t.Fatal(err)
	}
	if d := h.class.takeDirect(); len(d) != 1 || d[0].p.(protocol.RemoteStart).IntervalMS != remote.HiddenParams.IntervalMS {
		t.Fatalf("hidden view sent %v", d)
	}
	h.app.ShowControl(true)
	h.class.takeDirect()
	// Release: a remote.stop; the control ends with the agent's ack.
	if err := h.app.ReleaseControl(); err != nil {
		t.Fatal(err)
	}
	d3 := h.class.takeDirect()
	if stop, ok := d3[0].p.(protocol.RemoteStop); len(d3) != 1 || !ok || stop.ControlID != start.ControlID {
		t.Fatalf("release sent %v", d3)
	}
	h.hooks.onRemote("s1", protocol.New(protocol.RemoteAck{RefID: d3[0].envID, ControlID: start.ControlID,
		Status: protocol.RemoteEnded, Reason: protocol.RemoteByTeacher}))
	if c := h.rec.last(EventRemote).(remote.Control); c.Live() || c.Reason != protocol.RemoteByTeacher {
		t.Fatalf("after release %+v", c)
	}
	if _, err := h.app.TakeControl("s2"); err != nil {
		t.Fatalf("take another after the release: %v", err)
	}
}

// A student who disconnects ends the control; an error answering the start
// fails it; closing the class releases it.
func TestRemoteControlEndsWithTheStudent(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapScreenThumb, protocol.CapRemote))
	h.app.TakeControl("s1")
	d := h.class.takeDirect()
	h.hooks.onRemote("s1", protocol.New(protocol.NewError(d[0].envID, protocol.CodeInternal, "sin aviso")))
	if c := h.rec.last(EventRemote).(remote.Control); c.Status != remote.StatusFailed {
		t.Fatalf("after error %+v", c)
	}

	h.app.TakeControl("s1")
	h.class.takeDirect()
	h.class.mu.Lock()
	h.class.sessions = nil
	h.class.mu.Unlock()
	h.hooks.onChange()
	if c := h.rec.last(EventRemote).(remote.Control); c.Status != remote.StatusGone {
		t.Fatalf("after disconnect %+v", c)
	}

	h.class.mu.Lock()
	h.class.sessions = []session.Session{student("s1", "PC01", protocol.CapScreenThumb, protocol.CapRemote)}
	h.class.mu.Unlock()
	h.app.TakeControl("s1")
	h.class.takeDirect()
	h.app.EndClass()
	if _, ok := h.class.takeDirect()[0].p.(protocol.RemoteStop); !ok {
		t.Fatal("closing the class did not release the control")
	}
	if h.app.desk.Current().Live() {
		t.Fatal("the control survived the class")
	}
}
