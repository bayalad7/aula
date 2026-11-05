package main

import (
	"context"
	"sync"
	"testing"

	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/snapshot"
)

// A screenshot (docs/privacidad.md rule 44): the native indicator first,
// then the image goes out and the window gets a «Captura de pantalla»
// notice that counts as unseen. It does not act on the computer: notify,
// never raise.
func TestSnapshotNoticeNotifiesNeverRaises(t *testing.T) {
	a, rec := newTestApp(nil)
	ind := &stubIndicator{}
	a.enableScreen(stubCapture{}, ind)
	if !protocol.Has(a.hello.Capabilities, protocol.CapScreenSnapshot) {
		t.Fatalf("hello capabilities = %v, want screen.snapshot", a.hello.Capabilities)
	}
	a.snap.Settle = -1
	a.onState(session.AgentConnected, discovery.Console{Name: "Prof. Ayala"})
	before := rec.notified

	var mu sync.Mutex
	var sent []protocol.Payload
	send := func(_ context.Context, p protocol.Payload) error {
		mu.Lock()
		sent = append(sent, p)
		mu.Unlock()
		return nil
	}
	reply, ok := a.snap.Handle(t.Context(), snapshot.Sender(send), protocol.New(protocol.ScreenSnapshotRequest{SnapshotID: protocol.NewID(), Quality: 90}))
	if !ok || reply != nil {
		t.Fatalf("Handle = %v, %v", reply, ok)
	}
	a.snap.Wait()

	if !ind.on() {
		t.Fatal("the indicator was not shown")
	}
	mu.Lock()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages", len(sent))
	}
	first, isPart := sent[0].(protocol.ScreenSnapshotResponse)
	mu.Unlock()
	if !isPart || first.Hostname != a.hello.Hostname {
		t.Fatalf("sent %#v", sent[0])
	}
	n, _ := rec.last(EventNotices).(Notices)
	if len(n.Notices) != 1 || n.Notices[0].Kind != NoticeSnapshot || n.Notices[0].Level != protocol.LevelWarning || n.Notices[0].Text != "" {
		t.Fatalf("notices = %+v", n)
	}
	if rec.raised != 0 || rec.notified != before+1 {
		t.Fatalf("raised %d, notified %d; want 0 and %d", rec.raised, rec.notified, before+1)
	}
	// The student dismisses it like any other notice.
	a.DismissNotice(n.Notices[0].ID)
	if n, _ := rec.last(EventNotices).(Notices); len(n.Notices) != 0 {
		t.Fatalf("notice not dismissed: %+v", n)
	}
}

func TestNoSnapshotCapabilityWithoutPlatform(t *testing.T) {
	a, _ := newTestApp(nil)
	if protocol.Has(a.hello.Capabilities, protocol.CapScreenSnapshot) {
		t.Fatal("screen.snapshot declared without capture")
	}
	reply, ok := a.snap.Handle(t.Context(), nil, protocol.New(protocol.ScreenSnapshotRequest{SnapshotID: protocol.NewID(), Quality: 90}))
	if e, isErr := reply.(protocol.ErrorPayload); !ok || !isErr || e.Code != protocol.CodeUnsupportedCapability {
		t.Fatalf("reply = %#v", reply)
	}
}
