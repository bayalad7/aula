package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/snapshot"
)

type pictureCapture struct{}

func (pictureCapture) Capture(_ context.Context, maxWidth, quality int) (platform.Frame, error) {
	img := image.NewRGBA(image.Rect(0, 0, 800, 450))
	for i := range img.Pix {
		img.Pix[i] = byte(i * 13)
	}
	return platform.EncodeFrame(img, maxWidth, quality)
}

type noAnnounce struct{}

func (noAnnounce) Announce(string, time.Duration) error { return nil }

// answer plays the agent: it takes the screenshot asked in d and returns
// what it would send back.
func answer(t *testing.T, d directSend, host, user string) []protocol.ScreenSnapshotResponse {
	t.Helper()
	var mu sync.Mutex
	var out []protocol.ScreenSnapshotResponse
	tk := &snapshot.Taker{Capture: pictureCapture{}, Indicator: noAnnounce{}, Settle: -1,
		Identity: func() (string, string) { return host, user }}
	send := func(_ context.Context, p protocol.Payload) error {
		mu.Lock()
		defer mu.Unlock()
		out = append(out, p.(protocol.ScreenSnapshotResponse))
		return nil
	}
	env := protocol.Envelope{V: protocol.Version, ID: d.envID, TS: 1, Type: d.p.MessageType(), Payload: d.p}
	if _, ok := tk.Handle(context.Background(), send, env); !ok {
		t.Fatal("not a screenshot request")
	}
	tk.Wait()
	return out
}

func snapshotHarness(t *testing.T) (*harness, string, *[]string, *bytes.Buffer) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapCodeView, protocol.CapScreenThumb, protocol.CapScreenSnapshot),
		student("s2", "PC02", protocol.CapCodeView), // an older agent, or Linux
		student("s3", "PC03", protocol.CapScreenThumb, protocol.CapRemote, protocol.CapScreenSnapshot))
	h.class.sessions[0].Username = "alumno01"
	docs := t.TempDir()
	opened := &[]string{}
	h.app.documents = func() (string, error) { return docs, nil }
	h.app.openFolder = func(p string) error { *opened = append(*opened, p); return nil }
	logs := &bytes.Buffer{}
	h.app.snaps.Logger = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return h, docs, opened, logs
}

// One student: ask, receive the parts, preview, save under
// capturas\{alumno}\{fecha}\, open the folder.
func TestSnapshotTakePreviewSave(t *testing.T) {
	h, docs, opened, logs := snapshotHarness(t)
	if st := h.app.View().Students; !st.Students[0].CanSnapshot || st.Students[1].CanSnapshot || st.CanSnapshot != 2 {
		t.Fatalf("canSnapshot = %+v", st)
	}
	if _, err := h.app.TakeSnapshot("s2"); errorCode(err) != codeSnapshotUnsupported {
		t.Fatalf("TakeSnapshot(no capability) = %v", err)
	}
	snap, err := h.app.TakeSnapshot("s1")
	if err != nil || snap.Status != snapshot.StatusPending || snap.Hostname != "PC01" {
		t.Fatalf("TakeSnapshot = %+v, %v", snap, err)
	}
	d := h.class.takeDirect()
	req, ok := d[0].p.(protocol.ScreenSnapshotRequest)
	if len(d) != 1 || !ok || d[0].sessionID != "s1" || req.MaxWidth != 0 || req.Quality != protocol.DefaultSnapshotQuality {
		t.Fatalf("sent %+v", d)
	}
	if _, err := h.app.SnapshotImage(snap.ID); errorCode(err) != codeSnapshotGone {
		t.Fatalf("image before it arrived: %v", err)
	}
	// Parts from another student are not this screenshot.
	parts := answer(t, d[0], "PC01", "alumno01")
	h.hooks.onSnapshot("s3", protocol.New(parts[0]))
	if got := h.app.View().Snapshots[0]; got.Status != snapshot.StatusPending {
		t.Fatalf("a part from another student counted: %+v", got)
	}
	for _, p := range parts {
		h.hooks.onSnapshot("s1", protocol.New(p))
	}
	ev, _ := h.rec.last(EventSnapshots).(Snapshots)
	if len(ev.Snapshots) != 1 || ev.Snapshots[0].Status != snapshot.StatusReady || ev.Snapshots[0].Width != 800 {
		t.Fatalf("event = %+v", ev)
	}
	b64, err := h.app.SnapshotImage(snap.ID)
	if err != nil {
		t.Fatal(err)
	}
	jpeg, _ := base64.StdEncoding.DecodeString(b64)
	// Nothing is written before «Guardar» (rule 45).
	if entries, _ := os.ReadDir(docs); len(entries) != 0 {
		t.Fatalf("written before saving: %v", entries)
	}
	if f := h.app.SnapshotFolder(); f.Exists || f.Path != filepath.Join(docs, "Aula", "capturas") {
		t.Fatalf("folder before saving = %+v", f)
	}
	if err := h.app.OpenSnapshotFolder(""); errorCode(err) != codeSnapshotFolder {
		t.Fatalf("open a folder that does not exist yet: %v", err)
	}

	saved, err := h.app.SaveSnapshots([]string{snap.ID})
	if err != nil || len(saved) != 1 {
		t.Fatalf("SaveSnapshots = %+v, %v", saved, err)
	}
	day := saved[0].TakenAt.Local().Format("2006-01-02")
	dir := filepath.Join(docs, "Aula", "capturas", "PC01 (alumno01)", day)
	if filepath.Dir(saved[0].Path) != dir {
		t.Fatalf("saved in %s, want %s", saved[0].Path, dir)
	}
	if data, err := os.ReadFile(saved[0].Path); err != nil || !bytes.Equal(data, jpeg) {
		t.Fatalf("file differs from the preview (%v)", err)
	}
	if f := h.app.SnapshotFolder(); !f.Exists {
		t.Fatal("folder not reported after saving")
	}
	if err := h.app.OpenSnapshotFolder(saved[0].Path); err != nil || len(*opened) != 1 || (*opened)[0] != dir {
		t.Fatalf("OpenSnapshotFolder(file) = %v, opened %v", err, *opened)
	}
	if err := h.app.OpenSnapshotFolder(docs); errorCode(err) != codeSnapshotFolder {
		t.Fatalf("a folder outside capturas was opened: %v", err)
	}
	// Rule 48: no image and no path in the log.
	for _, probe := range []string{b64[:40], "alumno01", docs} {
		if strings.Contains(logs.String(), probe) {
			t.Fatalf("the log carries %q:\n%s", probe, logs.String())
		}
	}
	// Closing the preview frees it; the file stays.
	h.app.DiscardSnapshots([]string{snap.ID})
	if len(h.app.View().Snapshots) != 0 {
		t.Fatal("discarded screenshot still in memory")
	}
	if _, err := os.Stat(saved[0].Path); err != nil {
		t.Fatal("discarding removed the saved file")
	}
}

// The batch asks those who can and names the others; an error from an
// agent fails only its own screenshot; closing the class drops them all.
func TestSnapshotBatchErrorsAndClassEnd(t *testing.T) {
	h, _, _, _ := snapshotHarness(t)
	b, err := h.app.TakeSnapshots([]string{"s1", "s2", "s3"})
	if err != nil || len(b.Snapshots) != 2 || len(b.Skipped) != 1 || b.Skipped[0] != "PC02" {
		t.Fatalf("TakeSnapshots = %+v, %v", b, err)
	}
	if _, err := h.app.TakeSnapshots([]string{"s2"}); errorCode(err) != codeSnapshotUnsupported {
		t.Fatalf("batch with nobody able: %v", err)
	}
	d := h.class.takeDirect()
	h.hooks.onSnapshot("s3", protocol.New(protocol.NewError(d[1].envID, protocol.CodeInternal, "no se pudo capturar la pantalla")))
	for _, p := range answer(t, d[0], "PC01", "") {
		h.hooks.onSnapshot("s1", protocol.New(p))
	}
	got := map[string]snapshot.Snapshot{}
	for _, s := range h.app.View().Snapshots {
		got[s.SessionID] = s
	}
	if got["s1"].Status != snapshot.StatusReady || got["s3"].Status != snapshot.StatusFailed || got["s3"].Reason != snapshot.ReasonCapture {
		t.Fatalf("snapshots = %+v", got)
	}
	// Saving the batch skips the failed one.
	saved, err := h.app.SaveSnapshots([]string{got["s1"].ID, got["s3"].ID})
	if err != nil || len(saved) != 1 || !strings.Contains(saved[0].Path, filepath.Join("capturas", "PC01")) {
		t.Fatalf("SaveSnapshots = %+v, %v", saved, err)
	}
	// A student who leaves fails its pending screenshot.
	s, _ := h.app.TakeSnapshot("s1")
	h.class.mu.Lock()
	h.class.sessions = h.class.sessions[1:]
	h.class.mu.Unlock()
	h.app.onChange()
	if g, _ := h.app.snaps.Get(s.ID); g.Reason != snapshot.ReasonGone {
		t.Fatalf("after leaving: %+v", g)
	}
	h.app.EndClass()
	if len(h.app.snaps.List()) != 0 {
		t.Fatal("screenshots outlived the class")
	}
	if _, err := os.Stat(saved[0].Path); err != nil {
		t.Fatal("closing the class removed a saved file")
	}
}

// A screenshot while the teacher controls that student: the control goes
// on (no remote.stop), and its input still flows afterwards.
func TestSnapshotDuringRemoteControl(t *testing.T) {
	h, _, _, _ := snapshotHarness(t)
	c, err := h.app.TakeControl("s3")
	if err != nil {
		t.Fatal(err)
	}
	d := h.class.takeDirect()
	h.hooks.onRemote("s3", protocol.New(protocol.RemoteAck{RefID: d[0].envID, ControlID: c.ControlID, Status: protocol.RemoteActive}))

	if _, err := h.app.TakeSnapshot("s3"); err != nil {
		t.Fatal(err)
	}
	d = h.class.takeDirect()
	if len(d) != 1 {
		t.Fatalf("sent %d messages", len(d))
	}
	if _, ok := d[0].p.(protocol.ScreenSnapshotRequest); !ok {
		t.Fatalf("sent %T, want only the screenshot request", d[0].p)
	}
	for _, p := range answer(t, d[0], "PC03", "") {
		h.hooks.onSnapshot("s3", protocol.New(p))
	}
	if cur := h.app.desk.Current(); !cur.Live() || cur.Status != protocol.RemoteActive {
		t.Fatalf("the control changed: %+v", cur)
	}
	if err := h.app.SendRemoteInput([]protocol.InputEvent{{T: protocol.InputMove, X: 0.5, Y: 0.5}}); err != nil {
		t.Fatal(err)
	}
	sent := h.class.takeDirect()
	if len(sent) != 1 {
		t.Fatalf("input after the screenshot: %v", sent)
	}
	if _, ok := sent[0].p.(protocol.RemoteInput); !ok {
		t.Fatalf("sent %T", sent[0].p)
	}
	if h.app.View().Snapshots[0].Status != snapshot.StatusReady {
		t.Fatal("the screenshot did not arrive")
	}
}
