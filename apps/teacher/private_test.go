package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/protocol"
)

// A private message goes to the chosen student only, marked personal; its
// delivery is tracked like any notice, and neither its label nor the log
// carry the text (docs/privacidad.md rule 53).
func TestPrivateNoticeReachesOnlyTheChosenStudent(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapMsgShow),
		student("s2", "PC02", protocol.CapMsgShow),
		student("s3", "PC03", protocol.CapCodeView)) // no msg.show
	logs := &bytes.Buffer{}
	h.app.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	const secret = "Revisa la línea 12: falta un dos puntos"

	d, err := h.app.SendPrivateNotice("s2", "  "+secret+"  ", "info")
	if err != nil {
		t.Fatal(err)
	}
	sent := h.class.takeDirect()
	if len(sent) != 1 || sent[0].sessionID != "s2" {
		t.Fatalf("sent to %+v, want s2 only", sent)
	}
	if h.class.last() != nil {
		t.Fatal("a private message was broadcast")
	}
	show := sent[0].p.(protocol.MsgShow)
	if !show.Personal || show.Text != secret || show.ReplyTo != "" {
		t.Errorf("msg.show = %+v", show)
	}
	if d.Sent != 1 || d.Kind != "notice" || d.Label != "Mensaje privado a PC02" {
		t.Errorf("delivery = %+v", d)
	}
	h.hooks.onAck(sent[0].envID)
	if got := h.rec.last(EventDeliveries).(Deliveries).Deliveries[0]; got.ID != d.ID || got.Acked != 1 {
		t.Errorf("ack not counted: %+v", got)
	}
	if strings.Contains(logs.String(), "línea 12") {
		t.Errorf("the text reached the log:\n%s", logs.String())
	}

	if _, err := h.app.SendPrivateNotice("s3", "hola", "info"); errorCode(err) != codeNoticeUnsupported {
		t.Errorf("to a computer without msg.show = %v", err)
	}
	if _, err := h.app.SendPrivateNotice("nadie", "hola", "info"); errorCode(err) != codeStudentGone {
		t.Errorf("to an unknown session = %v", err)
	}
	if _, err := h.app.SendPrivateNotice("s1", "   ", "info"); errorCode(err) != codeInvalidNotice {
		t.Errorf("empty = %v", err)
	}
	if len(h.class.takeDirect()) != 0 {
		t.Error("a refused private message was sent")
	}
}
