package broadcast

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bayalad7/aula/internal/protocol"
)

// fakeIndicator records the native badge.
type fakeIndicator struct {
	mu    sync.Mutex
	text  string
	shows int
	fail  error
}

func (f *fakeIndicator) Show(text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return f.fail
	}
	f.text = text
	f.shows++
	return nil
}

func (f *fakeIndicator) Hide() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.text = ""
	return nil
}

func (f *fakeIndicator) shown() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.text
}

// fakePublisher is the media side: it records what the presenter asks.
type fakePublisher struct {
	mu      sync.Mutex
	err     error
	opened  []PublishOptions
	pubs    []*fakePublication
	ev      PublishEvents
	badgeAt string // the badge's text when Open ran
	badge   *fakeIndicator
}

func (f *fakePublisher) Open(opt PublishOptions, ev PublishEvents) (Publication, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.badgeAt = f.badge.shown()
	if f.err != nil {
		return nil, f.err
	}
	f.opened = append(f.opened, opt)
	f.ev = ev
	p := &fakePublication{}
	f.pubs = append(f.pubs, p)
	return p, nil
}

type fakePublication struct {
	mu        sync.Mutex
	answers   []int
	cands     []Candidate
	closed    bool
	answerErr error
}

func (p *fakePublication) Size() (int, int) { return 1280, 720 }

func (p *fakePublication) Answer(attempt int, offer string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.answerErr != nil {
		return "", p.answerErr
	}
	p.answers = append(p.answers, attempt)
	return sdpWith("sendonly"), nil
}

func (p *fakePublication) AddCandidate(_ int, c Candidate) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cands = append(p.cands, c)
	return nil
}

func (p *fakePublication) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}

func (p *fakePublication) isClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

func sdpWith(direction string) string {
	return "v=0\r\no=- 1 2 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 102\r\nc=IN IP4 0.0.0.0\r\na=mid:0\r\na=" + direction + "\r\na=rtpmap:102 H264/90000\r\n"
}

type presenterRig struct {
	p     *Presenter
	pub   *fakePublisher
	badge *fakeIndicator
	mu    sync.Mutex
	sent  []protocol.Payload
	views []PresentView
}

func newPresenterRig(t *testing.T) *presenterRig {
	t.Helper()
	r := &presenterRig{badge: &fakeIndicator{}}
	r.pub = &fakePublisher{badge: r.badge}
	r.p = &Presenter{Publisher: r.pub, Indicator: r.badge,
		OnView: func(v PresentView) { r.mu.Lock(); r.views = append(r.views, v); r.mu.Unlock() }}
	t.Cleanup(r.p.Stop)
	return r
}

func (r *presenterRig) send(_ context.Context, m protocol.Payload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, m)
	return nil
}

// deliver hands a console message to the presenter, the way the agent does:
// its reply goes out on the same connection.
func (r *presenterRig) deliver(t *testing.T, m protocol.Payload) protocol.Envelope {
	t.Helper()
	env := roundTrip(t, m)
	reply, ok := r.p.Handle(context.Background(), r.send, env)
	if !ok {
		t.Fatalf("%s not handled", m.MessageType())
	}
	if reply != nil {
		_ = r.send(context.Background(), reply)
	}
	r.p.Wait()
	return env
}

func (r *presenterRig) acks() []protocol.StudentPresentAck {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []protocol.StudentPresentAck
	for _, m := range r.sent {
		if a, ok := m.(protocol.StudentPresentAck); ok {
			out = append(out, a)
		}
	}
	return out
}

func (r *presenterRig) lastAck(t *testing.T) protocol.StudentPresentAck {
	t.Helper()
	a := r.acks()
	if len(a) == 0 {
		t.Fatal("no ack sent")
	}
	return a[len(a)-1]
}

func (r *presenterRig) lastView() PresentView {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.views) == 0 {
		return PresentView{}
	}
	return r.views[len(r.views)-1]
}

var presentStart = protocol.StudentPresentStart{PresentID: "c0a1b2c3-d4e5-4f60-8a71-b2c3d4e5f607", MaxWidth: 1920, FPS: 15, MaxKbps: 2500}

// The teacher decides (docs/privacidad.md rule 75): no card and no answer
// from the student. The band and the native badge are on screen before the
// capture opens; then the console's offer is answered send-only.
func TestPresenterShowsTheBandBeforeCapturing(t *testing.T) {
	r := newPresenterRig(t)
	if got := r.p.Capabilities(); len(got) != 1 || got[0] != protocol.CapScreenPresent {
		t.Fatalf("capabilities %v", got)
	}
	start := r.deliver(t, presentStart)
	if r.pub.badgeAt != PresentIndicatorText {
		t.Fatalf("badge when the capture opened: %q", r.pub.badgeAt)
	}
	if a := r.lastAck(t); a.Status != protocol.PresentAccepted || a.Width != 1280 || a.Height != 720 || a.RefID != start.ID {
		t.Fatalf("ack %+v", a)
	}
	if len(r.acks()) != 1 {
		t.Fatalf("acks %+v: the student is never asked", r.acks())
	}
	if r.pub.opened[0] != (PublishOptions{MaxWidth: 1920, FPS: 15, MaxKbps: 2500}) {
		t.Fatalf("options %+v", r.pub.opened[0])
	}
	if v := r.lastView(); v.Phase != PresentLive || v.PresentID != presentStart.PresentID {
		t.Fatalf("view %+v", v)
	}
	// A resend of the same start keeps it and answers again.
	r.deliver(t, presentStart)
	if len(r.pub.opened) != 1 || r.lastAck(t).Status != protocol.PresentAccepted {
		t.Fatalf("a resend opened the capture again: %d", len(r.pub.opened))
	}

	// The console's offer is answered send-only; an offer that asks the
	// student to receive (or send audio) is refused.
	offer := r.deliver(t, protocol.StudentPresentOffer{PresentID: presentStart.PresentID, Attempt: 1, SDP: sdpWith("recvonly")})
	var answer protocol.StudentPresentAnswer
	for _, m := range r.sent {
		if a, ok := m.(protocol.StudentPresentAnswer); ok {
			answer = a
		}
	}
	if answer.RefID != offer.ID || answer.Attempt != 1 {
		t.Fatalf("answer %+v", answer)
	}
	reply, _ := r.p.Handle(context.Background(), r.send, roundTrip(t, protocol.StudentPresentOffer{PresentID: presentStart.PresentID, Attempt: 2, SDP: sdpWith("sendonly")}))
	if e, ok := reply.(protocol.ErrorPayload); !ok || e.Code != protocol.CodeInvalidMessage {
		t.Fatalf("send-only offer accepted: %+v", reply)
	}
	// Local candidates go out with their attempt.
	r.pub.ev.OnCandidate(1, Candidate{Candidate: "candidate:1 1 udp 1 10.0.0.2 5000 typ host", SDPMid: "0"})
	r.p.Wait()
	found := false
	for _, m := range r.sent {
		if c, ok := m.(protocol.StudentPresentICE); ok && c.Attempt == 1 {
			found = true
		}
	}
	if !found {
		t.Fatal("local candidate not sent")
	}

	// stop: the capture and the badge end, and the ack says so.
	stop := r.deliver(t, protocol.StudentPresentStop{PresentID: presentStart.PresentID})
	if a := r.lastAck(t); a.Status != protocol.PresentEnded || a.RefID != stop.ID {
		t.Fatalf("ack %+v", a)
	}
	if !r.pub.pubs[0].isClosed() || r.badge.shown() != "" || r.lastView().Phase != PresentOff {
		t.Fatalf("after stop: closed %v, badge %q, view %+v", r.pub.pubs[0].isClosed(), r.badge.shown(), r.lastView())
	}
	// A stop again is ignored; an offer after it is refused.
	n := len(r.acks())
	r.deliver(t, protocol.StudentPresentStop{PresentID: presentStart.PresentID})
	if len(r.acks()) != n {
		t.Fatal("second stop answered")
	}
}

func TestPresenterOneAtATime(t *testing.T) {
	r := newPresenterRig(t)
	r.deliver(t, presentStart)
	other := presentStart
	other.PresentID = "f0a1b2c3-d4e5-4f60-8a71-b2c3d4e5f607"
	start2 := r.deliver(t, other)
	acks := r.acks()
	ended, accepted := acks[len(acks)-2], acks[len(acks)-1]
	if ended.Status != protocol.PresentEnded || ended.PresentID != presentStart.PresentID || ended.RefID != start2.ID {
		t.Fatalf("the old one did not end first: %+v", ended)
	}
	if accepted.Status != protocol.PresentAccepted || accepted.PresentID != other.PresentID {
		t.Fatalf("the new one did not start: %+v", accepted)
	}
	if !r.pub.pubs[0].isClosed() || r.pub.pubs[1].isClosed() || r.badge.shown() != PresentIndicatorText {
		t.Fatal("the old publication goes on, or the new one has no badge")
	}
	// The connection closes: everything ends quietly.
	n := len(r.acks())
	r.p.Stop()
	r.p.Wait()
	if len(r.acks()) != n || r.lastView().Phase != PresentOff || r.badge.shown() != "" || !r.pub.pubs[1].isClosed() {
		t.Fatal("Stop sent an ack or left the band, the badge or the capture")
	}
}

func TestPresenterFailures(t *testing.T) {
	r := newPresenterRig(t)
	r.pub.err = ErrEncoderUnavailable
	r.deliver(t, presentStart)
	if a := r.lastAck(t); a.Status != protocol.PresentFailed || a.Reason != protocol.PresentEncoderUnavailable {
		t.Fatalf("ack %+v", a)
	}
	if r.badge.shown() != "" || r.lastView().Phase != PresentOff {
		t.Fatal("the band or the badge stayed after a failure")
	}

	// Without the badge on screen nothing is captured.
	r.pub.err = nil
	r.badge.fail = errors.New("no window")
	other := presentStart
	other.PresentID = "a1a1b2c3-d4e5-4f60-8a71-b2c3d4e5f607"
	r.deliver(t, other)
	if a := r.lastAck(t); a.Status != protocol.PresentFailed || a.Reason != protocol.PresentCaptureUnavailable {
		t.Fatalf("ack %+v", a)
	}
	if len(r.pub.opened) != 0 {
		t.Fatal("captured without the badge")
	}

	// The encoder stops while publishing: the presentation fails and ends.
	r.badge.fail = nil
	third := presentStart
	third.PresentID = "b1a1b2c3-d4e5-4f60-8a71-b2c3d4e5f607"
	r.deliver(t, third)
	r.pub.ev.OnFail(ErrEncoderUnavailable)
	r.p.Wait()
	if a := r.lastAck(t); a.Status != protocol.PresentFailed || a.Reason != protocol.PresentEncoderUnavailable || a.PresentID != third.PresentID {
		t.Fatalf("ack %+v", a)
	}
	if r.badge.shown() != "" || !r.pub.pubs[0].isClosed() {
		t.Fatal("the badge or the capture stayed after the encoder failed")
	}

	// Without a publisher the agent does not declare it and refuses.
	none := &Presenter{Indicator: r.badge, OnView: func(PresentView) {}}
	if none.Capabilities() != nil {
		t.Fatal("declared without a publisher")
	}
	reply, ok := none.Handle(context.Background(), r.send, roundTrip(t, presentStart))
	if e, isErr := reply.(protocol.ErrorPayload); !ok || !isErr || e.Code != protocol.CodeUnsupportedCapability {
		t.Fatalf("reply %+v", reply)
	}
}

func TestIndicatorMux(t *testing.T) {
	ind := &fakeIndicator{}
	m := &IndicatorMux{Indicator: ind}
	stream, present := m.Stream(), m.Presentation()
	_ = stream.Show("vista")
	_ = present.Show(PresentIndicatorText)
	if ind.shown() != PresentIndicatorText {
		t.Fatalf("shown %q", ind.shown())
	}
	_ = stream.Hide() // the thumbnails end: the presentation's stays
	if ind.shown() != PresentIndicatorText {
		t.Fatalf("shown %q", ind.shown())
	}
	_ = stream.Show("vista")
	_ = present.Hide() // the presentation ends: back to the stream's
	if ind.shown() != "vista" {
		t.Fatalf("shown %q", ind.shown())
	}
	_ = stream.Hide()
	if ind.shown() != "" {
		t.Fatalf("shown %q", ind.shown())
	}
	ind.fail = errors.New("x")
	if err := present.Show("y"); err == nil {
		t.Fatal("a failed Show reported success")
	}
}
