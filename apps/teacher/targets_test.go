package main

import (
	"testing"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// Code and notices can go to the selected students only.
func TestEmitToTargets(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapCodeView, protocol.CapMsgShow),
		student("s2", "PC02", protocol.CapCodeView, protocol.CapMsgShow),
		student("s3", "PC03", protocol.CapMsgShow))
	d, err := h.app.Emit(EmitRequest{Title: "Uno", Lang: "python", Content: pySnippet, Mode: "replace", Targets: []string{"s2", "gone"}})
	if err != nil {
		t.Fatal(err)
	}
	direct := h.class.takeDirect()
	if len(direct) != 1 || direct[0].sessionID != "s2" {
		t.Fatalf("sent to %+v, want only s2", direct)
	}
	if d.Sent != 1 || d.Failed != 1 || len(d.Pending) != 1 || d.Pending[0] != "s2" {
		t.Errorf("delivery = %+v", d)
	}
	if h.class.last() != nil {
		t.Error("a targeted emit also broadcast to everyone")
	}
	n, err := h.app.SendNoticeTo("Hola", "info", []string{"s1", "s3"})
	if err != nil || n.Sent != 2 {
		t.Errorf("SendNoticeTo = %+v, %v", n, err)
	}
}

// «Reenviar a los que faltan» goes only to who did not acknowledge, and the
// late ack of the first send no longer counts.
func TestResendToMissing(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapCodeView),
		student("s2", "PC02", protocol.CapCodeView))
	d, err := h.app.Emit(EmitRequest{Title: "Uno", Lang: "python", Content: pySnippet, Mode: "replace"})
	if err != nil {
		t.Fatal(err)
	}
	first := map[string]string{}
	for i, id := range h.class.envIDs {
		first[[]string{"s1", "s2"}[i]] = id
	}
	h.hooks.onAck(h.class.envIDs[0])
	missing := h.rec.last(EventDeliveries).(Deliveries).Deliveries[0].Pending
	if len(missing) != 1 {
		t.Fatalf("pending = %v, want one", missing)
	}
	r, err := h.app.Resend(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	direct := h.class.takeDirect()
	if len(direct) != 1 || direct[0].sessionID != missing[0] {
		t.Fatalf("resent to %+v, want only %s", direct, missing[0])
	}
	if _, isShare := direct[0].p.(protocol.CodeShare); !isShare {
		t.Errorf("resent %T, want the same code.share", direct[0].p)
	}
	h.hooks.onAck(h.class.envIDs[1]) // late ack of the first send
	if got := h.rec.last(EventDeliveries).(Deliveries).Deliveries[0]; got.Acked != 1 {
		t.Errorf("a replaced envelope counted: %+v", got)
	}
	h.hooks.onAck(direct[0].envID)
	got := h.rec.last(EventDeliveries).(Deliveries).Deliveries[0]
	if got.ID != r.ID || got.Acked != 2 || len(got.Pending) != 0 {
		t.Errorf("after the resend = %+v", got)
	}
	if _, err := h.app.Resend("nope"); err == nil {
		t.Error("Resend of an unknown delivery should fail")
	}
}

// Changing channel or name keeps the teacher's history (C2).
func TestHistorySurvivesChannelChange(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapCodeView))
	if _, err := h.app.Emit(EmitRequest{Title: "Uno", Lang: "python", Content: pySnippet, Mode: "replace"}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.app.StartClass("2", "Prof. Ayala"); err != nil {
		t.Fatal(err)
	}
	v := h.app.View()
	if len(v.History) != 1 || v.History[0].Title != "Uno" || len(v.Deliveries) != 1 {
		t.Errorf("after changing channel: history %+v, deliveries %d", v.History, len(v.Deliveries))
	}
	h.class.mu.Lock()
	h.class.sessions = []session.Session{student("s9", "PC09", protocol.CapCodeView)}
	h.class.mu.Unlock()
	if _, err := h.app.Emit(EmitRequest{Title: "Dos", Lang: "python", Content: pySnippet, Mode: "replace"}); err != nil {
		t.Fatal(err)
	}
	if v := h.app.View(); v.History[len(v.History)-1].Number != 2 {
		t.Errorf("numbering restarted: %+v", v.History)
	}
}

func TestInstallerInfo(t *testing.T) {
	h := newHarness(t)
	info := h.app.InstallerInfo()
	if info.Channel != "1" || info.ConsoleName != "Prof. Ayala" || info.Version != version || info.Hostname == "" || info.Addresses == nil {
		t.Errorf("InstallerInfo = %+v", info)
	}
}
