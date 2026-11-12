package main

import (
	"context"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/broadcast"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

const (
	bcID      = "7d1f2e3a-4b5c-4d6e-a7f8-9a0b1c2d3e4f"
	offerSDP  = "v=0\r\no=- 1 2 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 102\r\nc=IN IP4 0.0.0.0\r\na=mid:0\r\na=sendonly\r\na=rtpmap:102 H264/90000\r\n"
	answerSDP = "v=0\r\no=- 3 2 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 102\r\nc=IN IP4 0.0.0.0\r\na=mid:0\r\na=recvonly\r\na=rtpmap:102 H264/90000\r\n"
)

// holds records the App's hold hook: how the window is held or let go.
type holds struct {
	mu  sync.Mutex
	got []string // "on:<mode>" or "off"
}

func (h *holds) hold(on bool, mode protocol.BroadcastMode) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if on {
		h.got = append(h.got, "on:"+string(mode))
	} else {
		h.got = append(h.got, "off")
	}
}

func (h *holds) list() []string { h.mu.Lock(); defer h.mu.Unlock(); return slices.Clone(h.got) }

func (h *holds) last() string {
	l := h.list()
	if len(l) == 0 {
		return ""
	}
	return l[len(l)-1]
}

// mediaApp is a test app whose frontend reported WebRTC with H.264.
func mediaApp(t *testing.T, webrtc, h264 bool) (*App, *recorder, *holds) {
	a, rec := newTestApp(nil)
	h := &holds{}
	a.hold = h.hold
	a.mediaWait = time.Second
	go a.ReportMedia(webrtc, h264)
	a.awaitMedia(t.Context())
	return a, rec, h
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
	a, rec, held := mediaApp(t, true, true)
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
	if !ok || !bv.Active || bv.Teacher != "Prof. Ayala" || bv.Channel != "1" {
		t.Fatalf("broadcast view %+v", rec.last(EventBroadcast))
	}
	if rec.last(EventSignal) != nil {
		t.Fatal("a signal reached the frontend before the offer")
	}
	st := rec.last(EventStatus).(Status)
	if !st.Watching || st.Teacher != "Prof. Ayala" {
		t.Fatalf("status %+v", st)
	}
	// A console without the mode (before v0.15): v0.4's maximized window.
	if got := held.list(); !slices.Equal(got, []string{"on:"}) {
		t.Fatalf("window holds %v, want v0.4's maximized window", got)
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
	if st := rec.last(EventStatus).(Status); st.Watching {
		t.Fatalf("status after disconnect %+v", st)
	}
	if held.last() != "off" {
		t.Fatalf("window still held after the broadcast: %v", held.list())
	}
}

// freezer is a TCP proxy to the console that can stop passing bytes while
// keeping the connections open: a console that stops answering.
type freezer struct {
	addr   string
	mu     sync.Mutex
	frozen bool
	thaw   chan struct{}
}

func startFreezer(t *testing.T, target string) *freezer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &freezer{addr: ln.Addr().String(), thaw: make(chan struct{})}
	var conns []net.Conn
	var cmu sync.Mutex
	t.Cleanup(func() {
		_ = ln.Close()
		f.mu.Lock()
		if f.frozen {
			close(f.thaw)
			f.frozen = false
		}
		f.mu.Unlock()
		cmu.Lock()
		for _, c := range conns {
			_ = c.Close()
		}
		cmu.Unlock()
	})
	pipe := func(dst, src net.Conn) {
		buf := make([]byte, 32<<10)
		for {
			n, err := src.Read(buf)
			f.mu.Lock()
			frozen, thaw := f.frozen, f.thaw
			f.mu.Unlock()
			if frozen {
				<-thaw // hold the bytes: nothing reaches the other side
			}
			if n > 0 {
				if _, werr := dst.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}
	go func() {
		for {
			in, err := ln.Accept()
			if err != nil {
				return
			}
			out, err := net.Dial("tcp", target)
			if err != nil {
				_ = in.Close()
				continue
			}
			cmu.Lock()
			conns = append(conns, in, out)
			cmu.Unlock()
			go pipe(out, in)
			go pipe(in, out)
		}
	}()
	return f
}

func (f *freezer) freeze() { f.mu.Lock(); f.frozen = true; f.mu.Unlock() }

// TestFullscreenIsReleasedWhenTheConsoleStopsAnswering is the acceptance
// test of rule 12: a full screen broadcast holds the window, and when the
// console stops answering (the connection stays open but nothing arrives)
// the agent's idle timeout of §3.4 ends the session and lets the window go
// (window_test.go checks it goes back as it was). The same happens when the
// connection closes (TestBroadcastViewNoticeAndSignaling).
func TestFullscreenIsReleasedWhenTheConsoleStopsAnswering(t *testing.T) {
	// Both ends beat every 50 ms: only a frozen console makes the agent's
	// 500 ms idle timeout fire.
	fast := transport.Options{HeartbeatInterval: 50 * time.Millisecond, IdleTimeout: 500 * time.Millisecond}
	c := startLiveConsoleOpts(t, nil, fast)
	px := startFreezer(t, c.addr)
	a, _ := newTestApp(nil)
	held := &holds{}
	a.hold = held.hold
	a.mediaWait = 5 * time.Second
	go a.ReportMedia(true, true) // start declares screen.broadcast once it arrives
	cfg := Config{Channel: "1", ConsolePublicKey: c.key, Direct: px.addr}
	pub, err := cfg.Validate()
	if err != nil {
		t.Fatal(err)
	}
	a.cfg, a.pub = cfg, pub
	a.connOpts = fast
	a.start(t.Context())
	defer a.stop()

	s := c.waitAgent(t, "agent that receives the broadcast", func(s session.Session) bool { return s.Can(protocol.CapScreenBroadcast) })
	start := protocol.ScreenBroadcastStart{BroadcastID: bcID, Width: 1920, Height: 1080, FPS: 15, Mode: protocol.BroadcastFullscreen}
	if _, err := c.console.Send(t.Context(), s.ID, start); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, 5*time.Second, "window held in full screen", func() bool { return held.last() == "on:fullscreen" })
	time.Sleep(time.Second) // a live console keeps it held past the idle timeout
	if held.last() != "on:fullscreen" {
		t.Fatalf("released while the console answers: %v", held.list())
	}
	if bv := a.View().Broadcast; bv.Mode != "fullscreen" {
		t.Fatalf("view mode %q", bv.Mode)
	}

	px.freeze()
	waitUntil(t, 5*time.Second, "window released after the idle timeout", func() bool { return held.last() == "off" })
	if a.View().Broadcast.Active {
		t.Fatal("broadcast view still on after the console stopped answering")
	}
}

func waitUntil(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
