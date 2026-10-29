package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/broadcast"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

const (
	bcID      = "7d1f2e3a-4b5c-4d6e-a7f8-9a0b1c2d3e4f"
	offerSDP  = "v=0\r\no=- 1 2 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 102\r\nc=IN IP4 0.0.0.0\r\na=mid:0\r\na=sendonly\r\na=rtpmap:102 H264/90000\r\n"
	answerSDP = "v=0\r\no=- 3 2 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 102\r\nc=IN IP4 0.0.0.0\r\na=mid:0\r\na=recvonly\r\na=rtpmap:102 H264/90000\r\n"
)

// mediaApp is a test app whose frontend reported WebRTC with H.264.
func mediaApp(t *testing.T, webrtc, h264 bool) (*App, *recorder, *[]bool) {
	a, rec := newTestApp(nil)
	var mu sync.Mutex
	var maximized []bool
	a.maximize = func(on bool) { mu.Lock(); maximized = append(maximized, on); mu.Unlock() }
	a.mediaWait = time.Second
	go a.ReportMedia(webrtc, h264)
	a.awaitMedia(t.Context())
	return a, rec, &maximized
}

func TestBroadcastCapabilityFollowsTheFrontend(t *testing.T) {
	a, _, _ := mediaApp(t, true, true)
	if !protocol.Has(a.hello.Capabilities, protocol.CapScreenBroadcast) {
		t.Fatalf("capabilities %v, want screen.broadcast", a.hello.Capabilities)
	}
	b, _, _ := mediaApp(t, true, false) // WebView2 without H.264 (Windows N)
	if protocol.Has(b.hello.Capabilities, protocol.CapScreenBroadcast) {
		t.Fatal("screen.broadcast declared without an H.264 decoder")
	}
	// No report in time: not declared, and a late report changes nothing.
	c, _ := newTestApp(nil)
	c.mediaWait = 10 * time.Millisecond
	c.awaitMedia(t.Context())
	c.ReportMedia(true, true)
	if protocol.Has(c.hello.Capabilities, protocol.CapScreenBroadcast) || c.viewer.Supported {
		t.Fatal("screen.broadcast declared without a report from the frontend")
	}
}

func TestBroadcastViewNoticeAndSignaling(t *testing.T) {
	a, rec, maximized := mediaApp(t, true, true)
	a.onState(session.AgentConnected, discovery.Console{Name: "Prof. Ayala"})
	var mu sync.Mutex
	var sent []protocol.Payload
	send := func(_ context.Context, p protocol.Payload) error {
		mu.Lock()
		defer mu.Unlock()
		sent = append(sent, p)
		return nil
	}
	handle := func(p protocol.Payload) protocol.Envelope {
		e := protocol.New(p)
		reply, ok := a.viewer.Handle(t.Context(), send, e)
		if !ok || reply != nil {
			t.Fatalf("%s: handled %v, reply %v", p.MessageType(), ok, reply)
		}
		return e
	}

	handle(protocol.ScreenBroadcastStart{BroadcastID: bcID, Width: 1920, Height: 1080, FPS: 15})
	bv, ok := rec.last(EventBroadcast).(BroadcastView)
	if !ok || !bv.Active || bv.Notice != "▶ Estás viendo la pantalla de Prof. Ayala (canal 1)" {
		t.Fatalf("broadcast view %+v", rec.last(EventBroadcast))
	}
	if rec.last(EventSignal) != nil {
		t.Fatal("a signal reached the frontend before the offer")
	}
	st := rec.last(EventStatus).(Status)
	if !st.Watching || st.Banner != "Aula — viendo la pantalla de Prof. Ayala (canal 1)" {
		t.Fatalf("status %+v", st)
	}
	if rec.raised == 0 || len(*maximized) != 1 || !(*maximized)[0] {
		t.Fatalf("window not raised and maximized: raised %d, maximized %v", rec.raised, *maximized)
	}
	// The full state includes the broadcast (and does not deadlock with it).
	if v := a.View(); !v.Broadcast.Active || !v.Status.Watching {
		t.Fatalf("View %+v", v.Broadcast)
	}

	offer := handle(protocol.ScreenBroadcastOffer{BroadcastID: bcID, Attempt: 1, SDP: offerSDP})
	sig, ok := rec.last(EventSignal).(broadcast.Signal)
	if !ok || sig.Kind != broadcast.SignalOffer || sig.SDP != offerSDP {
		t.Fatalf("signal %+v", rec.last(EventSignal))
	}
	if err := a.BroadcastAnswer(bcID, 1, answerSDP); err != nil {
		t.Fatal(err)
	}
	if err := a.BroadcastCandidate(bcID, 1, "candidate:1 1 udp 1 192.168.1.30 50000 typ host", "0", 0); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	ans, _ := sent[0].(protocol.ScreenBroadcastAnswer)
	_, isICE := sent[1].(protocol.ScreenBroadcastICE)
	mu.Unlock()
	if ans.RefID != offer.ID || !isICE {
		t.Fatalf("sent %+v", sent)
	}

	// The teacher's screen being received is not this screen being viewed:
	// the red "being viewed" banner stays off.
	if st := rec.last(EventStatus).(Status); st.Viewed {
		t.Fatal("receiving a broadcast marked this screen as viewed")
	}

	// The connection drops: the view goes away by itself.
	a.onState(session.AgentDisconnected, discovery.Console{Name: "Prof. Ayala"})
	if bv := rec.last(EventBroadcast).(BroadcastView); bv.Active {
		t.Fatal("broadcast view still shown after the connection closed")
	}
	if st := rec.last(EventStatus).(Status); st.Watching || strings.Contains(st.Banner, "viendo") {
		t.Fatalf("status after disconnect %+v", st)
	}
	if m := *maximized; m[len(m)-1] {
		t.Fatal("window still maximized after the broadcast")
	}
}
