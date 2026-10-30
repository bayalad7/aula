package main

import (
	"testing"

	"github.com/bayalad7/aula/internal/protocol"
)

// A student that answers a delivery with an error is listed apart, with its
// code, on the delivery and on its row; «Reenviar» retries it.
func TestAgentErrorRejectsDeliveryAndMarksStudent(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapCodeView),
		student("s2", "PC02", protocol.CapCodeView))
	d, err := h.app.Emit(EmitRequest{Title: "Uno", Lang: "python", Content: pySnippet, Mode: "replace"})
	if err != nil {
		t.Fatal(err)
	}
	h.hooks.onAck(h.class.envIDs[0])
	h.hooks.onError("s2", protocol.NewError(h.class.envIDs[1], protocol.CodeInternal, "no se pudo"))

	got := h.rec.last(EventDeliveries).(Deliveries).Deliveries[0]
	if len(got.Pending) != 0 || len(got.Rejected) != 1 || got.Rejected[0].Code != string(protocol.CodeInternal) {
		t.Fatalf("delivery = %+v, want s2 rejected with internal", got)
	}
	var marked *AgentError
	for _, s := range h.rec.last(EventStudents).(Students).Students {
		if s.LastError != nil {
			if s.ID != got.Rejected[0].Session {
				t.Errorf("error shown on %s, want on %s", s.ID, got.Rejected[0].Session)
			}
			marked = s.LastError
		}
	}
	if marked == nil || marked.Code != string(protocol.CodeInternal) || marked.At == "" {
		t.Fatalf("student error = %+v", marked)
	}

	if _, err := h.app.Resend(d.ID); err != nil {
		t.Fatal(err)
	}
	direct := h.class.takeDirect()
	if len(direct) != 1 || direct[0].sessionID != got.Rejected[0].Session {
		t.Fatalf("resent to %+v, want only the student that failed", direct)
	}
	h.hooks.onAck(direct[0].envID)
	got = h.rec.last(EventDeliveries).(Deliveries).Deliveries[0]
	if got.Acked != 2 || len(got.Rejected) != 0 || len(got.Pending) != 0 {
		t.Errorf("after the resend = %+v", got)
	}

	// An error about something else (no delivery) only marks the student.
	h.hooks.onError("s1", protocol.NewError(protocol.NewID(), protocol.CodeUnsupportedCapability, "x"))
	for _, s := range h.rec.last(EventStudents).(Students).Students {
		if s.ID == "s1" && (s.LastError == nil || s.LastError.Code != string(protocol.CodeUnsupportedCapability)) {
			t.Errorf("s1 error = %+v", s.LastError)
		}
	}
}
