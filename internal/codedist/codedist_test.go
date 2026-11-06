package codedist

import (
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

func share(id, title string, mode protocol.CodeMode) protocol.CodeShare {
	return protocol.CodeShare{SnippetID: id, Title: title, Lang: "python", Content: "print(" + title + ")\n", Mode: mode}
}

func titles(list []Snippet) []string {
	var out []string
	for _, s := range list {
		out = append(out, s.Title)
	}
	return out
}

func TestBoardReplaceAppendClear(t *testing.T) {
	b := NewBoard(3)
	now := time.Now()
	b.Apply(share("1", "a", protocol.ModeAppend), now)
	b.Apply(share("2", "b", protocol.ModeAppend), now)
	if got := titles(b.Snippets()); len(got) != 2 || got[1] != "b" {
		t.Fatalf("after append: %v", got)
	}
	b.Apply(share("3", "c", protocol.ModeReplace), now)
	if got := titles(b.Snippets()); len(got) != 1 || got[0] != "c" {
		t.Fatalf("after replace: %v", got)
	}
	// Re-sending the same snippet updates it in place.
	b.Apply(share("4", "d", protocol.ModeAppend), now)
	b.Apply(share("3", "c2", protocol.ModeAppend), now)
	if got := titles(b.Snippets()); len(got) != 2 || got[0] != "c2" || got[1] != "d" {
		t.Fatalf("after re-send: %v", got)
	}
	// The cap drops the oldest.
	b.Apply(share("5", "e", protocol.ModeAppend), now)
	b.Apply(share("6", "f", protocol.ModeAppend), now)
	if got := titles(b.Snippets()); len(got) != 3 || got[0] != "d" || got[2] != "f" {
		t.Fatalf("after cap: %v", got)
	}
	if s, ok := b.Get("5"); !ok || s.Title != "e" {
		t.Fatal("Get failed")
	}
	b.Clear()
	if len(b.Snippets()) != 0 {
		t.Fatal("Clear left snippets")
	}
}

func TestBoardSnapshotIsCopy(t *testing.T) {
	b := NewBoard(0)
	list := b.Apply(share("1", "a", protocol.ModeReplace), time.Now())
	list[0].Content = "tampered"
	if s, _ := b.Get("1"); s.Content == "tampered" {
		t.Fatal("snapshot shares memory with the board")
	}
}

func TestReceiverAcks(t *testing.T) {
	var boards [][]Snippet
	var notices []Notice
	r := &Receiver{
		Board:    NewBoard(0),
		OnBoard:  func(s []Snippet) { boards = append(boards, s) },
		OnNotice: func(n Notice) { notices = append(notices, n) },
	}
	s := share(protocol.NewID(), "a", protocol.ModeReplace)
	env := protocol.New(s)
	if ack, ok := r.Handle(env).(protocol.CodeAck); !ok || ack.RefID != env.ID || ack.SnippetID != s.SnippetID {
		t.Fatalf("code.share reply = %#v", r.Handle(env))
	}
	clr := protocol.New(protocol.CodeClear{})
	if ack, ok := r.Handle(clr).(protocol.CodeAck); !ok || ack.RefID != clr.ID || ack.SnippetID != "" {
		t.Fatal("code.clear not acked")
	}
	msg := protocol.New(protocol.MsgShow{Text: "Guarden su trabajo", Level: protocol.LevelWarning})
	if ack, ok := r.Handle(msg).(protocol.MsgAck); !ok || ack.RefID != msg.ID {
		t.Fatal("msg.show not acked")
	}
	if len(boards) != 2 || len(boards[0]) != 1 || len(boards[1]) != 0 || len(notices) != 1 || notices[0].Level != protocol.LevelWarning {
		t.Fatalf("callbacks: boards=%d notices=%+v", len(boards), notices)
	}
	r.Handle(protocol.New(protocol.MsgShow{Text: "Revisa la línea 12", Level: protocol.LevelInfo, Personal: true, ReplyTo: protocol.NewID()}))
	if n := notices[len(notices)-1]; !n.Personal || n.ReplyTo == "" {
		t.Fatalf("personal notice = %+v", n)
	}
	if r.Handle(protocol.New(protocol.Heartbeat{})) != nil {
		t.Fatal("heartbeat must not be answered")
	}
	if e, ok := r.Handle(protocol.New(protocol.MsgAck{RefID: protocol.NewID()})).(protocol.ErrorPayload); !ok || e.Code != protocol.CodeInvalidMessage {
		t.Fatal("unexpected message not rejected")
	}
}

func TestDeliveriesCountSimulatedApart(t *testing.T) {
	var d Deliveries
	del := d.Start("code", "Ejercicio 1", map[string]string{"pc1": "r1", "pc2": "r2"}, 0, 0, time.Now())
	if _, ok := d.AddSimulated(del.ID, map[string]string{"sim1": "s1"}); !ok {
		t.Fatal("AddSimulated did not find the delivery")
	}
	if _, ok := d.AddSimulated("nope", map[string]string{"sim2": "s2"}); ok {
		t.Error("AddSimulated found an unknown delivery")
	}
	d.Ack("r1")
	got, _ := d.Ack("s1")
	if got.Sent != 2 || got.Acked != 1 || got.SimSent != 1 || got.SimAcked != 1 {
		t.Errorf("delivery = %+v, want sent 2, acked 1, simSent 1, simAcked 1", got)
	}
	if _, ok := d.Ack("s1"); ok {
		t.Error("a simulated ack counted twice")
	}
	if len(got.Pending) != 1 || got.Pending[0] != "pc2" {
		t.Errorf("pending = %v, want [pc2]", got.Pending)
	}
}

// Resending to who is missing replaces their envelope: the late ack of the
// first send no longer counts, the new one does, once.
func TestDeliveriesResent(t *testing.T) {
	var d Deliveries
	del := d.Start("code", "Ejercicio 2", map[string]string{"pc1": "r1", "pc2": "r2"}, 0, 0, time.Now())
	d.Ack("r1")
	got, ok := d.Resent(del.ID, map[string]string{"pc2": "r2b", "pc1": "ignored"})
	if !ok || len(got.Pending) != 1 || got.Pending[0] != "pc2" {
		t.Fatalf("Resent = %+v, %v", got, ok)
	}
	if _, late := d.Ack("r2"); late {
		t.Error("the ack of the replaced envelope still counted")
	}
	got, _ = d.Ack("r2b")
	if got.Acked != 2 || got.Sent != 2 || len(got.Pending) != 0 {
		t.Errorf("after resend = %+v, want 2/2 and nobody pending", got)
	}
	if _, ok := d.Ack("ignored"); ok {
		t.Error("an envelope for a session that was not pending was tracked")
	}
	if g, ok := d.Get(del.ID); !ok || g.Acked != 2 {
		t.Errorf("Get = %+v, %v", g, ok)
	}
}

// An error instead of the ack takes the session out of Pending and into
// Rejected with its code; a resend makes it pending again.
func TestDeliveriesReject(t *testing.T) {
	var d Deliveries
	del := d.Start("code", "Ejercicio 3", map[string]string{"pc1": "r1", "pc2": "r2"}, 0, 0, time.Now())
	d.AddSimulated(del.ID, map[string]string{"sim1": "s1"})
	got, ok := d.Reject("r2", "unsupported_capability")
	if !ok || len(got.Rejected) != 1 || got.Rejected[0] != (Rejection{Session: "pc2", Code: "unsupported_capability"}) {
		t.Fatalf("Reject = %+v, %v", got, ok)
	}
	if len(got.Pending) != 2 || got.Pending[0] != "pc1" || got.Pending[1] != "sim1" {
		t.Errorf("pending = %v, want [pc1 sim1]", got.Pending)
	}
	if _, again := d.Reject("r2", "internal"); again {
		t.Error("a settled envelope was rejected twice")
	}
	if _, late := d.Ack("r2"); late {
		t.Error("the ack of a rejected envelope counted")
	}
	d.Reject("s1", "internal")
	got, _ = d.Resent(del.ID, map[string]string{"pc2": "r2b", "sim1": "s1b"})
	if len(got.Rejected) != 0 || len(got.Pending) != 3 {
		t.Fatalf("after resend = %+v, want nobody rejected and 3 pending", got)
	}
	d.Ack("r2b")
	got, _ = d.Ack("s1b")
	if got.Acked != 1 || got.SimAcked != 1 {
		t.Errorf("acks after resend = %+v, want 1 real and 1 simulated", got)
	}
}
