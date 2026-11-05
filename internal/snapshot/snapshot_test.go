package snapshot

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// --- Doubles ----------------------------------------------------------------

// events records, in order, what the doubles saw.
type events struct {
	mu  sync.Mutex
	log []string
}

func (e *events) add(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.log = append(e.log, s)
}

func (e *events) list() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.log...)
}

// noisyJPEG is a real JPEG large enough to need several parts.
func noisyJPEG(t *testing.T, w, h, quality int) platform.Frame {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	x := uint32(2463534242)
	for i := range img.Pix {
		x ^= x << 13
		x ^= x >> 17
		x ^= x << 5
		img.Pix[i] = byte(x)
	}
	img.Set(0, 0, color.White)
	f, err := platform.EncodeFrame(img, w, quality)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// fakeCapture is the ScreenCapture double: it returns frame, or err.
type fakeCapture struct {
	ev        *events
	frame     platform.Frame
	err       error
	sizes     map[int]int // quality -> JPEG size to pretend (oversized tests)
	qualities []int
	widths    []int
}

func (c *fakeCapture) Capture(_ context.Context, maxWidth, quality int) (platform.Frame, error) {
	c.ev.add("capture")
	c.qualities = append(c.qualities, quality)
	c.widths = append(c.widths, maxWidth)
	if c.err != nil {
		return platform.Frame{}, c.err
	}
	if n, ok := c.sizes[quality]; ok {
		return platform.Frame{JPEG: make([]byte, n), Width: 1920, Height: 1080}, nil
	}
	return c.frame, nil
}

type fakeAnnouncer struct {
	ev  *events
	err error
}

func (a *fakeAnnouncer) Announce(text string, d time.Duration) error {
	if a.err != nil {
		return a.err
	}
	a.ev.add("announce:" + text)
	return nil
}

// sink collects what the agent sends, and checks that every message fits
// in one WebSocket message once encoded.
type sink struct {
	t    *testing.T
	ev   *events
	mu   sync.Mutex
	sent []protocol.Payload
	err  error // returned by send
}

func (s *sink) send(_ context.Context, p protocol.Payload) error {
	if s.err != nil {
		return s.err
	}
	data, err := protocol.Encode(protocol.New(p))
	if err != nil {
		s.t.Errorf("Encode %s: %v", p.MessageType(), err)
	} else if len(data) > protocol.MaxMessageSize {
		s.t.Errorf("%s is %d bytes", p.MessageType(), len(data))
	}
	s.ev.add("send:" + string(p.MessageType()))
	s.mu.Lock()
	s.sent = append(s.sent, p)
	s.mu.Unlock()
	return nil
}

func (s *sink) payloads() []protocol.Payload {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]protocol.Payload(nil), s.sent...)
}

func newTaker(ev *events, c *fakeCapture) *Taker {
	return &Taker{
		Capture: c, Indicator: &fakeAnnouncer{ev: ev}, Settle: -1,
		Identity: func() (string, string) { return "LAB3-PC07", "alumno07" },
		OnTaken:  func(time.Time) { ev.add("taken") },
		Now:      func() time.Time { return time.Date(2026, 11, 5, 10, 42, 6, 0, time.Local) },
	}
}

func request(t *testing.T, s *Store, session string) (protocol.Envelope, Snapshot) {
	t.Helper()
	req, snap, err := s.Request(session, "LAB3-PC07", "alumno07", false)
	if err != nil {
		t.Fatal(err)
	}
	env := protocol.New(req)
	s.Sent(snap.ID, env.ID)
	return env, snap
}

// --- Agent ------------------------------------------------------------------

// The whole path: the agent announces, captures, tells the window and sends
// the parts; the console reassembles a checked image identical to the
// capture.
func TestTakeAndReassemble(t *testing.T) {
	ev := &events{}
	frame := noisyJPEG(t, 1280, 720, 95)
	if len(frame.JPEG) <= protocol.SnapshotPartSize {
		t.Fatalf("test image is %d bytes: it must need several parts", len(frame.JPEG))
	}
	c := &fakeCapture{ev: ev, frame: frame}
	tk := newTaker(ev, c)
	out := &sink{t: t, ev: ev}
	store := &Store{}
	env, snap := request(t, store, "s1")

	if reply, ok := tk.Handle(context.Background(), out.send, env); !ok || reply != nil {
		t.Fatalf("Handle = %v, %v", reply, ok)
	}
	tk.Wait()

	got := ev.list()
	if got[0] != "announce:"+IndicatorText || got[1] != "capture" || got[2] != "taken" {
		t.Fatalf("order = %v; want announce, capture, taken, then the parts", got)
	}
	if c.widths[0] != protocol.MaxSnapshotWidth || c.qualities[0] != protocol.DefaultSnapshotQuality {
		t.Fatalf("captured at %d px / q%d; want native and the requested quality", c.widths[0], c.qualities[0])
	}
	parts := out.payloads()
	want := (len(frame.JPEG) + protocol.SnapshotPartSize - 1) / protocol.SnapshotPartSize
	if len(parts) != want {
		t.Fatalf("%d parts, want %d", len(parts), want)
	}
	first := parts[0].(protocol.ScreenSnapshotResponse)
	if first.RefID != env.ID || first.Hostname != "LAB3-PC07" || first.Username != "alumno07" ||
		first.Width != 1280 || first.Height != 720 || first.Size != len(frame.JPEG) {
		t.Fatalf("part 1 = %+v", first)
	}
	// Deliver out of order: the console must not care.
	for _, i := range append([]int{len(parts) - 1}, rangeTo(len(parts)-1)...) {
		store.Response("s1", parts[i].(protocol.ScreenSnapshotResponse))
	}
	s, _ := store.Get(snap.ID)
	if s.Status != StatusReady {
		t.Fatalf("status = %s (%s)", s.Status, s.Reason)
	}
	img, ok := store.Image(snap.ID)
	if !ok || !bytes.Equal(img, frame.JPEG) {
		t.Fatal("reassembled image differs from the capture")
	}
}

func rangeTo(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func TestOneAtATime(t *testing.T) {
	ev := &events{}
	release := make(chan struct{})
	tk := newTaker(ev, &fakeCapture{ev: ev, frame: noisyJPEG(t, 64, 64, 90)})
	tk.Indicator = blockingAnnouncer{release}
	out := &sink{t: t, ev: ev}
	if _, ok := tk.Handle(context.Background(), out.send, protocol.New(protocol.ScreenSnapshotRequest{SnapshotID: protocol.NewID(), Quality: 90})); !ok {
		t.Fatal("not handled")
	}
	reply, _ := tk.Handle(context.Background(), out.send, protocol.New(protocol.ScreenSnapshotRequest{SnapshotID: protocol.NewID(), Quality: 90}))
	e, ok := reply.(protocol.ErrorPayload)
	if !ok || e.Code != protocol.CodeInternal || reasonFor(e) != ReasonBusy {
		t.Fatalf("second request = %#v; want busy", reply)
	}
	close(release)
	tk.Wait()
	// Free again once the first one is sent.
	if reply, _ := tk.Handle(context.Background(), out.send, protocol.New(protocol.ScreenSnapshotRequest{SnapshotID: protocol.NewID(), Quality: 90})); reply != nil {
		t.Fatalf("third request refused: %#v", reply)
	}
	tk.Wait()
}

type blockingAnnouncer struct{ release chan struct{} }

func (b blockingAnnouncer) Announce(string, time.Duration) error { <-b.release; return nil }

func TestUnsupportedWithoutCaptureOrIndicator(t *testing.T) {
	for _, tk := range []*Taker{{}, {Capture: &fakeCapture{}}, {Indicator: &fakeAnnouncer{}}} {
		if tk.Capabilities() != nil {
			t.Fatal("must not declare screen.snapshot")
		}
		reply, ok := tk.Handle(context.Background(), nil, protocol.New(protocol.ScreenSnapshotRequest{SnapshotID: protocol.NewID(), Quality: 90}))
		if e, isErr := reply.(protocol.ErrorPayload); !ok || !isErr || e.Code != protocol.CodeUnsupportedCapability {
			t.Fatalf("reply = %#v", reply)
		}
	}
	ok := &Taker{Capture: &fakeCapture{}, Indicator: &fakeAnnouncer{}}
	if c := ok.Capabilities(); len(c) != 1 || c[0] != protocol.CapScreenSnapshot {
		t.Fatalf("Capabilities = %v", c)
	}
	if _, handled := ok.Handle(context.Background(), nil, protocol.New(protocol.ScreenThumbStop{})); handled {
		t.Fatal("handled a message that is not a snapshot request")
	}
}

// Rule 44: without the notice, nothing is captured.
func TestNoIndicatorNoCapture(t *testing.T) {
	ev := &events{}
	c := &fakeCapture{ev: ev, frame: noisyJPEG(t, 64, 64, 90)}
	tk := newTaker(ev, c)
	tk.Indicator = &fakeAnnouncer{ev: ev, err: errors.New("no desktop")}
	out := &sink{t: t, ev: ev}
	store := &Store{}
	env, snap := request(t, store, "s1")
	tk.Handle(context.Background(), out.send, env)
	tk.Wait()
	for _, e := range ev.list() {
		if e == "capture" || e == "taken" {
			t.Fatalf("captured without the notice: %v", ev.list())
		}
	}
	sent := out.payloads()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages", len(sent))
	}
	store.Error("s1", sent[0].(protocol.ErrorPayload))
	if s, _ := store.Get(snap.ID); s.Status != StatusFailed || s.Reason != ReasonCapture {
		t.Fatalf("snapshot = %+v", s)
	}
}

func TestCaptureFailureIsReported(t *testing.T) {
	ev := &events{}
	tk := newTaker(ev, &fakeCapture{ev: ev, err: errors.New("locked")})
	out := &sink{t: t, ev: ev}
	tk.Handle(context.Background(), out.send, protocol.New(protocol.ScreenSnapshotRequest{SnapshotID: protocol.NewID(), Quality: 90}))
	tk.Wait()
	sent := out.payloads()
	if len(sent) != 1 {
		t.Fatalf("sent %v", sent)
	}
	if e := sent[0].(protocol.ErrorPayload); e.Code != protocol.CodeInternal {
		t.Fatalf("error = %+v", e)
	}
	for _, e := range ev.list() {
		if e == "taken" {
			t.Fatal("the window was told about a screenshot that was not taken")
		}
	}
}

func TestOversizedLowersQualityThenGivesUp(t *testing.T) {
	ev := &events{}
	big := protocol.MaxSnapshotBytes + 1
	c := &fakeCapture{ev: ev, frame: noisyJPEG(t, 64, 64, 50), sizes: map[int]int{90: big, 80: big, 70: big}}
	tk := newTaker(ev, c)
	out := &sink{t: t, ev: ev}
	tk.Handle(context.Background(), out.send, protocol.New(protocol.ScreenSnapshotRequest{SnapshotID: protocol.NewID(), Quality: 90}))
	tk.Wait()
	if got := c.qualities; len(got) != 4 || got[3] != 60 {
		t.Fatalf("qualities = %v; want 90, 80, 70, 60", got)
	}
	if _, isErr := out.payloads()[0].(protocol.ErrorPayload); isErr {
		t.Fatal("a capture that fits at q60 must be sent")
	}

	ev2 := &events{}
	c2 := &fakeCapture{ev: ev2, sizes: map[int]int{95: big, 85: big, 75: big, 65: big, 55: big, 50: big}}
	tk2 := newTaker(ev2, c2)
	out2 := &sink{t: t, ev: ev2}
	tk2.Handle(context.Background(), out2.send, protocol.New(protocol.ScreenSnapshotRequest{SnapshotID: protocol.NewID(), Quality: 95}))
	tk2.Wait()
	if got := c2.qualities; got[len(got)-1] != protocol.MinSnapshotQuality {
		t.Fatalf("qualities = %v; want down to %d", got, protocol.MinSnapshotQuality)
	}
	if e, isErr := out2.payloads()[0].(protocol.ErrorPayload); !isErr || e.Code != protocol.CodeMessageTooLarge {
		t.Fatalf("reply = %#v", out2.payloads()[0])
	}
}

// Rule 47: the agent never writes the screenshot to disk, and rule 48: its
// log never carries the image. The test points every place the process
// could write to (temp, home, working directory) at empty folders and
// checks they stay empty.
func TestAgentKeepsNothingOnDiskOrInLogs(t *testing.T) {
	dirs := map[string]string{}
	for _, v := range []string{"TMP", "TEMP", "TMPDIR", "HOME", "USERPROFILE", "LOCALAPPDATA", "APPDATA"} {
		d := t.TempDir()
		dirs[v] = d
		t.Setenv(v, d)
	}
	wd := t.TempDir()
	t.Chdir(wd)
	dirs["cwd"] = wd

	var logs bytes.Buffer
	ev := &events{}
	frame := noisyJPEG(t, 1280, 720, 95)
	tk := newTaker(ev, &fakeCapture{ev: ev, frame: frame})
	tk.Logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	out := &sink{t: t, ev: ev}
	tk.Handle(context.Background(), out.send, protocol.New(protocol.ScreenSnapshotRequest{SnapshotID: protocol.NewID(), Quality: 90}))
	tk.Wait()
	// A delivery that fails half-way drops the image too.
	failing := &sink{t: t, ev: ev, err: errors.New("closed")}
	tk.Handle(context.Background(), failing.send, protocol.New(protocol.ScreenSnapshotRequest{SnapshotID: protocol.NewID(), Quality: 90}))
	tk.Wait()

	for name, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Errorf("%s (%s) has %d entries after a screenshot", name, d, len(entries))
		}
	}
	assertNoImage(t, logs.String(), frame.JPEG)
	if !strings.Contains(logs.String(), "screenshot sent") {
		t.Fatal("the log must record that a screenshot was sent")
	}
}

func assertNoImage(t *testing.T, logs string, jpeg []byte) {
	t.Helper()
	b64 := base64.StdEncoding.EncodeToString(jpeg)
	for _, probe := range []string{b64[:32], b64[len(b64)/2 : len(b64)/2+32], "/9j/", string(jpeg[100:132])} {
		if strings.Contains(logs, probe) {
			t.Fatalf("the log carries the image (%q)", probe)
		}
	}
}

// --- Console ----------------------------------------------------------------

func readyStore(t *testing.T, root string) (*Store, Snapshot, []byte) {
	t.Helper()
	ev := &events{}
	frame := noisyJPEG(t, 320, 200, 80)
	tk := newTaker(ev, &fakeCapture{ev: ev, frame: frame})
	out := &sink{t: t, ev: ev}
	store := &Store{}
	env, snap := request(t, store, "s1")
	tk.Handle(context.Background(), out.send, env)
	tk.Wait()
	for _, p := range out.payloads() {
		store.Response("s1", p.(protocol.ScreenSnapshotResponse))
	}
	s, _ := store.Get(snap.ID)
	if s.Status != StatusReady {
		t.Fatalf("status = %s", s.Status)
	}
	return store, s, frame.JPEG
}

func TestPartsFromAnotherStudentOrRequestAreDropped(t *testing.T) {
	ev := &events{}
	tk := newTaker(ev, &fakeCapture{ev: ev, frame: noisyJPEG(t, 64, 64, 90)})
	out := &sink{t: t, ev: ev}
	store := &Store{}
	env, snap := request(t, store, "s1")
	tk.Handle(context.Background(), out.send, env)
	tk.Wait()
	p := out.payloads()[0].(protocol.ScreenSnapshotResponse)

	store.Response("s2", p) // another student
	other := p
	other.SnapshotID = protocol.NewID() // never asked for
	store.Response("s1", other)
	if s, _ := store.Get(snap.ID); s.Status != StatusPending {
		t.Fatalf("status = %s", s.Status)
	}
	if len(store.List()) != 1 {
		t.Fatal("an unknown snapshot created an entry")
	}
	store.Response("s1", p)
	if s, _ := store.Get(snap.ID); s.Status != StatusReady {
		t.Fatalf("status = %s", s.Status)
	}
}

func TestCorruptImageFails(t *testing.T) {
	ev := &events{}
	tk := newTaker(ev, &fakeCapture{ev: ev, frame: noisyJPEG(t, 64, 64, 90)})
	out := &sink{t: t, ev: ev}
	store := &Store{}
	env, snap := request(t, store, "s1")
	tk.Handle(context.Background(), out.send, env)
	tk.Wait()
	p := out.payloads()[0].(protocol.ScreenSnapshotResponse)
	p.Data = append([]byte{}, p.Data...)
	p.Data[len(p.Data)/2] ^= 0xff
	store.Response("s1", p)
	s, _ := store.Get(snap.ID)
	if s.Status != StatusFailed || s.Reason != ReasonCorrupt {
		t.Fatalf("snapshot = %+v", s)
	}
	if _, ok := store.Image(snap.ID); ok {
		t.Fatal("a corrupt image is kept")
	}
}

func TestTimeoutGoneAndUnsent(t *testing.T) {
	var mu sync.Mutex
	var last []Snapshot
	store := &Store{Timeout: 20 * time.Millisecond, OnChange: func(l []Snapshot) { mu.Lock(); last = l; mu.Unlock() }}
	_, a := request(t, store, "s1")
	deadline := time.Now().Add(3 * time.Second)
	for {
		if s, _ := store.Get(a.ID); s.Status == StatusFailed {
			if s.Reason != ReasonTimeout {
				t.Fatalf("reason = %s", s.Reason)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
	mu.Lock()
	if len(last) != 1 || last[0].Status != StatusFailed {
		t.Fatalf("OnChange got %+v", last)
	}
	mu.Unlock()

	store.Timeout = time.Hour
	_, b := request(t, store, "s2")
	_, c := request(t, store, "s3")
	store.Gone(func(id string) bool { return id != "s2" })
	if s, _ := store.Get(b.ID); s.Reason != ReasonGone {
		t.Fatalf("gone: %+v", s)
	}
	if s, _ := store.Get(c.ID); s.Status != StatusPending {
		t.Fatalf("a connected student's screenshot failed: %+v", s)
	}
	store.Unsent(c.ID)
	if s, _ := store.Get(c.ID); s.Reason != ReasonGone {
		t.Fatalf("unsent: %+v", s)
	}
}

func TestErrorsMapToReasons(t *testing.T) {
	store := &Store{}
	for code, want := range map[protocol.ErrorCode]string{
		protocol.CodeUnsupportedCapability: ReasonUnsupported,
		protocol.CodeMessageTooLarge:       ReasonTooLarge,
		protocol.CodeInternal:              ReasonCapture,
	} {
		env, snap := request(t, store, "s1")
		if !store.Error("s1", protocol.NewError(env.ID, code, "x")) {
			t.Fatalf("%s not recognized", code)
		}
		if s, _ := store.Get(snap.ID); s.Reason != want {
			t.Errorf("%s -> %s, want %s", code, s.Reason, want)
		}
	}
	if store.Error("s1", protocol.NewError(protocol.NewID(), protocol.CodeInternal, "x")) {
		t.Fatal("an error for another message was taken as a screenshot's")
	}
}

func TestMaxHeldAndDiscardAndReset(t *testing.T) {
	store := &Store{MaxHeld: 2}
	_, a := request(t, store, "s1")
	request(t, store, "s2")
	if _, _, err := store.Request("s3", "x", "", false); !errors.Is(err, ErrTooMany) {
		t.Fatalf("third request: %v", err)
	}
	if !store.Discard(a.ID) || store.Discard(a.ID) {
		t.Fatal("Discard misbehaves")
	}
	if _, _, err := store.Request("s3", "x", "", false); err != nil {
		t.Fatalf("after discarding: %v", err)
	}
	store.Reset()
	if len(store.List()) != 0 {
		t.Fatal("Reset kept screenshots")
	}
}

// Rule 46: <root>\{alumno}\{fecha}\{hora}.jpg, never overwritten.
func TestSaveOrganizesByStudentAndDate(t *testing.T) {
	docs := t.TempDir()
	root := Root(docs)
	var logs bytes.Buffer
	store, snap, jpeg := readyStore(t, root)
	store.Logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	saved, err := store.Save(snap.ID, root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(docs, "Aula", "capturas", "LAB3-PC07 (alumno07)", "2026-11-05", "10-42-06.jpg")
	if saved.Path != want {
		t.Fatalf("Path = %s\nwant   %s", saved.Path, want)
	}
	data, err := os.ReadFile(want)
	if err != nil || !bytes.Equal(data, jpeg) {
		t.Fatalf("saved file differs (%v)", err)
	}
	// Saving again is the same file; another capture in the same second
	// gets its own name.
	again, _ := store.Save(snap.ID, root)
	if again.Path != want {
		t.Fatalf("second Save = %s", again.Path)
	}
	p2, err := WriteFile(root, snap, jpeg)
	if err != nil || filepath.Base(p2) != "10-42-06-2.jpg" {
		t.Fatalf("collision -> %s, %v", p2, err)
	}
	// The path names the student: it stays out of the log.
	if strings.Contains(logs.String(), "alumno07") || strings.Contains(logs.String(), docs) {
		t.Fatalf("the log carries the path: %s", logs.String())
	}
	assertNoImage(t, logs.String(), jpeg)
}

func TestNothingOnDiskWithoutSave(t *testing.T) {
	docs := t.TempDir()
	store, snap, _ := readyStore(t, Root(docs))
	store.Discard(snap.ID)
	store.Reset()
	entries, _ := os.ReadDir(docs)
	if len(entries) != 0 {
		t.Fatalf("something was written without «Guardar»: %v", entries)
	}
	if _, err := store.Save(snap.ID, Root(docs)); !errors.Is(err, ErrNotReady) {
		t.Fatalf("Save of a discarded screenshot: %v", err)
	}
}

func TestStudentFolder(t *testing.T) {
	for _, c := range []struct {
		host, user string
		sim        bool
		want       string
	}{
		{"LAB3-PC07", "alumno07", false, "LAB3-PC07 (alumno07)"},
		{"LAB3-PC15", "", false, "LAB3-PC15"},
		{"Simulado 1", "", true, "Simulado 1 (simulado)"},
		{`PC:1/2`, `dom\ana`, false, "PC_1_2 (dom_ana)"},
		{"CON", "", false, "_CON"},
		{"nul.local", "", false, "_nul.local"},
		{"", "", false, "equipo"},
		{"pc.", "", false, "pc"},
		{"..", "", false, "equipo"},
		{"a\x00b\tc", "", false, "abc"},
		{strings.Repeat("x", 200), "", false, strings.Repeat("x", maxNameRunes)},
	} {
		if got := StudentFolder(c.host, c.user, c.sim); got != c.want {
			t.Errorf("StudentFolder(%q, %q, %v) = %q, want %q", c.host, c.user, c.sim, got, c.want)
		}
	}
}
