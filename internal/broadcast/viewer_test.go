package broadcast

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/bayalad7/aula/internal/protocol"
)

const (
	testID    = "7d1f2e3a-4b5c-4d6e-a7f8-9a0b1c2d3e4f"
	otherID   = "0c9a7e52-6a4b-4f7e-8d33-2b1f4e6a9c10"
	offerSDP  = "v=0\r\no=- 1 2 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 102\r\nc=IN IP4 0.0.0.0\r\na=mid:0\r\na=sendonly\r\na=rtpmap:102 H264/90000\r\n"
	answerSDP = "v=0\r\no=- 3 2 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 102\r\nc=IN IP4 0.0.0.0\r\na=mid:0\r\na=recvonly\r\na=rtpmap:102 H264/90000\r\n"
)

func TestCheckSDP(t *testing.T) {
	if err := checkSDP(offerSDP, "sendonly"); err != nil {
		t.Fatalf("valid offer: %v", err)
	}
	if err := checkSDP(answerSDP, "recvonly"); err != nil {
		t.Fatalf("valid answer: %v", err)
	}
	audio := offerSDP + "m=audio 9 UDP/TLS/RTP/SAVPF 111\r\na=sendonly\r\n"
	data := strings.Replace(offerSDP, "m=video 9 UDP/TLS/RTP/SAVPF 102", "m=application 9 UDP/DTLS/SCTP webrtc-datachannel", 1)
	sendrecv := strings.Replace(offerSDP, "a=sendonly", "a=sendrecv", 1)
	noDir := strings.Replace(offerSDP, "a=sendonly\r\n", "", 1)
	for name, bad := range map[string]string{
		"audio": audio, "data channel": data, "sendrecv": sendrecv, "default direction": noDir, "garbage": "hola",
		"answer as offer": answerSDP,
	} {
		if err := checkSDP(bad, "sendonly"); !errors.Is(err, errBadSDP) {
			t.Errorf("%s accepted: %v", name, err)
		}
	}
}

// viewerHarness records what the viewer shows and sends.
type viewerHarness struct {
	mu      sync.Mutex
	views   []View
	signals []Signal
	sent    []protocol.Payload
	order   []string // "view" and "signal", to check the notice comes first
}

func newViewer(h *viewerHarness) *Viewer {
	return &Viewer{
		Supported: true,
		OnView: func(v View) error {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.views = append(h.views, v)
			h.order = append(h.order, "view")
			return nil
		},
		OnSignal: func(s Signal) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.signals = append(h.signals, s)
			h.order = append(h.order, "signal")
		},
	}
}

func (h *viewerHarness) send(_ context.Context, p protocol.Payload) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sent = append(h.sent, p)
	return nil
}

func env(p protocol.Payload) protocol.Envelope { return protocol.New(p) }

func TestViewerShowsTheNoticeBeforeTheOffer(t *testing.T) {
	h := &viewerHarness{}
	v := newViewer(h)
	ctx := t.Context()
	if got := v.Capabilities(); len(got) != 1 || got[0] != protocol.CapScreenBroadcast {
		t.Fatalf("capabilities %v", got)
	}
	start := protocol.ScreenBroadcastStart{BroadcastID: testID, Width: 1920, Height: 1080, FPS: 15}
	if reply, ok := v.Handle(ctx, h.send, env(start)); !ok || reply != nil {
		t.Fatalf("start: %v %v", reply, ok)
	}
	offer := env(protocol.ScreenBroadcastOffer{BroadcastID: testID, Attempt: 1, SDP: offerSDP})
	if reply, _ := v.Handle(ctx, h.send, offer); reply != nil {
		t.Fatalf("offer: %v", reply)
	}
	ice := protocol.ScreenBroadcastICE{BroadcastID: testID, Attempt: 1, Candidate: "candidate:1 1 udp 1 192.168.1.20 47831 typ host", SDPMid: "0"}
	v.Handle(ctx, h.send, env(ice))
	if strings.Join(h.order, ",") != "view,signal,signal" {
		t.Fatalf("order %v: the view must be shown before the offer", h.order)
	}
	if !h.views[0].Active || h.views[0].Width != 1920 || v.Current().BroadcastID != testID {
		t.Fatalf("view %+v", h.views[0])
	}
	if h.signals[0].Kind != SignalOffer || h.signals[1].Kind != SignalICE || h.signals[1].Candidate != ice.Candidate {
		t.Fatalf("signals %+v", h.signals)
	}

	// The frontend answers and trickles: the answer references the offer.
	if err := v.Answer(testID, 1, answerSDP); err != nil {
		t.Fatal(err)
	}
	if err := v.LocalCandidate(testID, 1, "candidate:2 1 udp 1 192.168.1.30 50000 typ host", "0", 0); err != nil {
		t.Fatal(err)
	}
	ans, ok := h.sent[0].(protocol.ScreenBroadcastAnswer)
	if !ok || ans.RefID != offer.ID || ans.Attempt != 1 {
		t.Fatalf("answer %+v", h.sent[0])
	}
	if _, ok := h.sent[1].(protocol.ScreenBroadcastICE); !ok {
		t.Fatalf("candidate %+v", h.sent[1])
	}
	for _, p := range h.sent {
		if err := p.Validate(); err != nil {
			t.Fatalf("%s: %v", p.MessageType(), err)
		}
	}
	// An answer that would send media is refused before leaving the agent.
	if err := v.Answer(testID, 1, strings.Replace(answerSDP, "recvonly", "sendrecv", 1)); err == nil {
		t.Fatal("sendrecv answer accepted")
	}

	// stop of another broadcast is ignored; the right one hides the view.
	v.Handle(ctx, h.send, env(protocol.ScreenBroadcastStop{BroadcastID: otherID}))
	if v.Current().BroadcastID != testID {
		t.Fatal("stop of another broadcast ended this one")
	}
	v.Handle(ctx, h.send, env(protocol.ScreenBroadcastStop{BroadcastID: testID}))
	if last := h.views[len(h.views)-1]; last.Active || v.Current().Active {
		t.Fatalf("view still active after stop: %+v", last)
	}
	if err := v.Answer(testID, 1, answerSDP); !errors.Is(err, ErrNoBroadcast) {
		t.Fatalf("answer after stop: %v", err)
	}
}

func TestViewerRejectsWhatIsNotAReceiveOnlyVideo(t *testing.T) {
	h := &viewerHarness{}
	v := newViewer(h)
	ctx := t.Context()
	// An offer without start.
	reply, _ := v.Handle(ctx, h.send, env(protocol.ScreenBroadcastOffer{BroadcastID: testID, Attempt: 1, SDP: offerSDP}))
	if e, ok := reply.(protocol.ErrorPayload); !ok || e.Code != protocol.CodeInvalidMessage {
		t.Fatalf("offer without start: %v", reply)
	}
	v.Handle(ctx, h.send, env(protocol.ScreenBroadcastStart{BroadcastID: testID, Width: 1280, Height: 720, FPS: 15}))
	withAudio := offerSDP + "m=audio 9 UDP/TLS/RTP/SAVPF 111\r\na=sendonly\r\n"
	reply, _ = v.Handle(ctx, h.send, env(protocol.ScreenBroadcastOffer{BroadcastID: testID, Attempt: 1, SDP: withAudio}))
	if e, ok := reply.(protocol.ErrorPayload); !ok || e.Code != protocol.CodeInvalidMessage {
		t.Fatalf("offer with audio: %v", reply)
	}
	if len(h.signals) != 0 {
		t.Fatalf("a refused offer reached the frontend: %+v", h.signals)
	}
}

func TestViewerAttempts(t *testing.T) {
	h := &viewerHarness{}
	v := newViewer(h)
	ctx := t.Context()
	v.Handle(ctx, h.send, env(protocol.ScreenBroadcastStart{BroadcastID: testID, Width: 1280, Height: 720, FPS: 15}))
	v.Handle(ctx, h.send, env(protocol.ScreenBroadcastOffer{BroadcastID: testID, Attempt: 2, SDP: offerSDP}))
	// An older offer and the candidates of attempt 1 are dropped.
	v.Handle(ctx, h.send, env(protocol.ScreenBroadcastOffer{BroadcastID: testID, Attempt: 1, SDP: offerSDP}))
	v.Handle(ctx, h.send, env(protocol.ScreenBroadcastICE{BroadcastID: testID, Attempt: 1, Candidate: "candidate:1 1 udp 1 10.0.0.1 1 typ host"}))
	if len(h.signals) != 1 || h.signals[0].Attempt != 2 {
		t.Fatalf("signals %+v", h.signals)
	}
	if err := v.Answer(testID, 1, answerSDP); !errors.Is(err, ErrNoBroadcast) {
		t.Fatalf("answer to an old attempt: %v", err)
	}
	// Candidates beyond the limit are dropped.
	for range protocol.MaxICECandidates + 5 {
		v.Handle(ctx, h.send, env(protocol.ScreenBroadcastICE{BroadcastID: testID, Attempt: 2, Candidate: "candidate:1 1 udp 1 10.0.0.1 1 typ host"}))
		_ = v.LocalCandidate(testID, 2, "candidate:2 1 udp 1 10.0.0.2 2 typ host", "0", 0)
	}
	if n := len(h.signals) - 1; n != protocol.MaxICECandidates {
		t.Fatalf("%d remote candidates relayed", n)
	}
	if n := len(h.sent); n != protocol.MaxICECandidates {
		t.Fatalf("%d local candidates sent", n)
	}
	// The frontend failed: the console hears about the offer in force.
	if err := v.Failed(testID, 2); err != nil {
		t.Fatal(err)
	}
	if e, ok := h.sent[len(h.sent)-1].(protocol.ErrorPayload); !ok || e.Code != protocol.CodeInternal {
		t.Fatalf("failure reported as %+v", h.sent[len(h.sent)-1])
	}
	// The connection closes: the view goes away.
	v.Stop()
	if v.Current().Active || h.views[len(h.views)-1].Active {
		t.Fatal("view still active after the connection closed")
	}
	v.Stop() // idempotent
}

func TestViewerWithoutSupportRefuses(t *testing.T) {
	v := &Viewer{}
	if v.Capabilities() != nil {
		t.Fatal("declares screen.broadcast without support")
	}
	reply, ok := v.Handle(t.Context(), nil, env(protocol.ScreenBroadcastStart{BroadcastID: testID, Width: 1, Height: 1, FPS: 1}))
	if e, isErr := reply.(protocol.ErrorPayload); !ok || !isErr || e.Code != protocol.CodeUnsupportedCapability {
		t.Fatalf("reply %v", reply)
	}
	if _, ok := v.Handle(t.Context(), nil, env(protocol.MsgAck{RefID: testID})); ok {
		t.Fatal("handled a message that is not a broadcast")
	}
}

func TestViewerViewErrorRefusesTheBroadcast(t *testing.T) {
	v := &Viewer{Supported: true, OnView: func(View) error { return errors.New("sin ventana") }, OnSignal: func(Signal) {}}
	reply, _ := v.Handle(t.Context(), nil, env(protocol.ScreenBroadcastStart{BroadcastID: testID, Width: 1, Height: 1, FPS: 1}))
	if e, ok := reply.(protocol.ErrorPayload); !ok || e.Code != protocol.CodeInternal {
		t.Fatalf("reply %v", reply)
	}
	if v.Current().Active {
		t.Fatal("broadcast active without a view")
	}
}
