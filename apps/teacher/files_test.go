package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/filepush"
	"github.com/bayalad7/aula/internal/protocol"
)

// fileHarness is a class of three: two can receive files, one cannot. The
// teacher's Documents is a temporary folder and Explorer is never opened.
func fileHarness(t *testing.T) (*harness, *bytes.Buffer, string) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapCodeView, protocol.CapFilePush),
		student("s2", "PC02", protocol.CapCodeView, protocol.CapFilePush),
		student("s3", "PC03", protocol.CapCodeView)) // an older agent
	logs := &bytes.Buffer{}
	h.app.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h.app.pusher.Logger = h.app.log
	h.app.pusher.AckWait = 2 * time.Second
	docs := t.TempDir()
	h.app.documents = func() (string, error) { return docs, nil }
	h.app.openFolder = func(p string) error { h.rec.emit("opened", p); return nil }
	return h, logs, docs
}

// agents answers the file.push the console sends, like the students would:
// saved, unless fail names a session and its reason. It stops with the test.
func agents(t *testing.T, h *harness, fail map[string]protocol.FilePushReason) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for ctx.Err() == nil {
			for _, d := range h.class.takeDirect() {
				p, ok := d.p.(protocol.FilePush)
				if !ok || p.Part != p.Parts {
					continue
				}
				ack := protocol.FilePushAck{RefID: d.envID, PushID: p.PushID, FileID: p.FileID, Status: protocol.FilePushSaved, Path: "x"}
				if r, bad := fail[d.sessionID]; bad {
					ack = protocol.FilePushAck{RefID: d.envID, PushID: p.PushID, FileID: p.FileID, Status: protocol.FilePushFailed, Reason: r}
				}
				h.hooks.onFile(d.sessionID, protocol.New(ack))
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	t.Cleanup(func() { cancel(); wg.Wait() })
}

func writeFiles(t *testing.T, files map[string]string) []filepush.Source {
	dir := t.TempDir()
	var out []filepush.Source
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		out = append(out, filepush.Source{Path: p, Rel: name})
	}
	slices.SortFunc(out, func(a, b filepush.Source) int { return strings.Compare(a.Rel, b.Rel) })
	return out
}

func waitDelivery(t *testing.T, h *harness, id string, done func(codedist.Delivery) bool) codedist.Delivery {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		d, _ := h.app.deliveries.Get(id)
		if done(d) {
			return d
		}
		if time.Now().After(deadline) {
			t.Fatalf("delivery never settled: %+v", d)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// «Enviar» reaches who can receive files, counts a student once every file is
// saved, lists why another one failed, and never logs names or contents.
func TestSendFilesCountsWhoHasThem(t *testing.T) {
	h, logs, _ := fileHarness(t)
	agents(t, h, map[string]protocol.FilePushReason{"s2": protocol.FilePushNoSpace})
	src := writeFiles(t, map[string]string{"secreto.csv": "contenido-secreto", "main.py": "print(1)\n"})
	d, err := h.app.SendFiles(FilesRequest{Files: src, Dest: "Carpeta secreta"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != "file" || d.Sent != 2 || d.NoCapability != 1 || d.Label != "Archivos: main.py y 1 más" {
		t.Fatalf("delivery = %+v", d)
	}
	d = waitDelivery(t, h, d.ID, func(d codedist.Delivery) bool { return d.Acked == 1 && len(d.Rejected) == 1 })
	if len(d.Pending) != 0 || d.Rejected[0].Session != "s2" || d.Rejected[0].Code != "no_space" {
		t.Fatalf("delivery = %+v", d)
	}
	if v := h.app.Files(); len(v.Sends) != 1 || v.Sends[0].Dest != "Carpeta secreta" || len(v.Sends[0].Names) != 2 {
		t.Fatalf("history = %+v", v.Sends)
	}
	for _, s := range []string{"secreto", "main.py", "contenido", "Carpeta"} {
		if strings.Contains(logs.String(), s) {
			t.Errorf("the log has %q:\n%s", s, logs.String())
		}
	}

	// «Reenviar a los que faltan» sends only to s2, reading the files again.
	if _, err := h.app.Resend(d.ID); err != nil {
		t.Fatal(err)
	}
	h.app.pusher.Wait()
	d, _ = h.app.deliveries.Get(d.ID)
	if d.Acked != 1 || len(d.Rejected) != 1 {
		t.Fatalf("after the resend: %+v", d)
	}
}

func TestSendFilesRefusesBeforeSending(t *testing.T) {
	h, _, _ := fileHarness(t)
	src := writeFiles(t, map[string]string{"a.txt": "a"})
	for _, c := range []struct {
		req  FilesRequest
		code string
	}{
		{FilesRequest{}, codePushNoFiles},
		{FilesRequest{Files: src, Dest: "../../Windows"}, codePushPathInvalid},
		{FilesRequest{Files: src, Dest: `C:\Windows`}, codePushPathInvalid},
		{FilesRequest{Files: []filepush.Source{{Path: src[0].Path, Rel: "con.txt"}}}, codePushPathInvalid},
		{FilesRequest{Files: []filepush.Source{{Path: filepath.Join(t.TempDir(), "no"), Rel: "no.txt"}}}, codePushFileGone},
		{FilesRequest{Files: src, Targets: []string{"s3"}}, codePushUnsupported},
		{FilesRequest{Template: "No existe"}, codeTemplateGone},
	} {
		if _, err := h.app.SendFiles(c.req); errorCode(err) != c.code {
			t.Errorf("%+v: %v, want %s", c.req, err, c.code)
		}
	}
	h.app.pusher.Wait()
	if sent := h.class.takeDirect(); len(sent) != 0 {
		t.Fatalf("sent %d messages", len(sent))
	}
}

// A template is a folder under Documentos\Aula\plantillas: saved from the
// chosen files, listed, sent and opened in Explorer.
func TestTemplatesSaveListSendOpen(t *testing.T) {
	h, _, docs := fileHarness(t)
	agents(t, h, nil)
	src := writeFiles(t, map[string]string{"main.py": "print(1)\n", "datos.csv": "1,2\n"})
	v, err := h.app.SaveTemplate(" Ejercicio 4 ", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Templates) != 1 || v.Templates[0].Name != "Ejercicio 4" || len(v.Templates[0].Files) != 2 {
		t.Fatalf("templates = %+v", v.Templates)
	}
	if _, err := os.Stat(filepath.Join(docs, "Aula", "plantillas", "Ejercicio 4", "main.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := h.app.SaveTemplate("Ejercicio 4", src); errorCode(err) != codeTemplateExists {
		t.Errorf("second save = %v", err)
	}
	if _, err := h.app.SaveTemplate("../fuera", src); errorCode(err) != codeTemplateInvalid {
		t.Errorf("bad name = %v", err)
	}
	d, err := h.app.SendFiles(FilesRequest{Template: "Ejercicio 4", Dest: "Ejercicio 4", Targets: []string{"s1"}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Sent != 1 || d.Label != "Archivos: Ejercicio 4 (2)" {
		t.Fatalf("delivery = %+v", d)
	}
	waitDelivery(t, h, d.ID, func(d codedist.Delivery) bool { return d.Acked == 1 })
	if s := h.app.Files().Sends; len(s) != 1 || s[0].Template != "Ejercicio 4" {
		t.Fatalf("history = %+v", s)
	}
	if _, err := h.app.ResendFiles(h.app.Files().Sends[0].ID); err != nil {
		t.Fatal(err)
	}
	h.app.pusher.Wait()
	if s := h.app.Files().Sends; len(s) != 1 || s[0].Times != 2 {
		t.Fatalf("a resend counts on the same row: %+v", s)
	}
	if err := h.app.OpenTemplates("Ejercicio 4"); err != nil {
		t.Fatal(err)
	}
	if p, _ := h.rec.last("opened").(string); p != filepath.Join(docs, "Aula", "plantillas", "Ejercicio 4") {
		t.Fatalf("opened %q", p)
	}
	if err := h.app.OpenTemplates("../.."); errorCode(err) != codeTemplateGone && errorCode(err) != codeTemplateInvalid {
		t.Fatalf("open outside = %v", err)
	}

	// Delete (nota de interfaz 16): only the template's folder; what was sent stays in the history.
	outside := filepath.Join(docs, "Aula", "no-tocar.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"../..", "..", "", "a/b", `..\Aula`} {
		if _, err := h.app.DeleteTemplate(bad); errorCode(err) != codeTemplateInvalid && errorCode(err) != codeTemplateGone {
			t.Errorf("delete %q = %v", bad, err)
		}
	}
	if _, err := h.app.DeleteTemplate("No existe"); errorCode(err) != codeTemplateGone {
		t.Errorf("delete missing = %v", err)
	}
	v, err = h.app.DeleteTemplate("Ejercicio 4")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Templates) != 0 {
		t.Fatalf("templates after delete = %+v", v.Templates)
	}
	if _, err := os.Stat(filepath.Join(docs, "Aula", "plantillas", "Ejercicio 4")); !os.IsNotExist(err) {
		t.Fatalf("template folder still there: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("a file outside the templates was touched: %v", err)
	}
	if _, err := os.Stat(filepath.Join(docs, "Aula", "plantillas")); err != nil {
		t.Fatalf("the templates folder itself was removed: %v", err)
	}
	if s := h.app.Files().Sends; len(s) != 1 {
		t.Fatalf("history after delete = %+v", s)
	}
}

// Closing the class stops what is being sent: the students that did not
// answer count as gone, not pending forever.
func TestEndClassStopsFiles(t *testing.T) {
	h, _, _ := fileHarness(t)
	h.app.pusher.AckWait = time.Minute
	src := writeFiles(t, map[string]string{"a.txt": "a"})
	if _, err := h.app.SendFiles(FilesRequest{Files: src}); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { h.app.EndClass(); h.app.pusher.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the deliveries did not stop with the class")
	}
}

// Every reason a student may not get a file has its manual section.
func TestFileReasonsLinkToTheManual(t *testing.T) {
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	m, _, err := app.loadManual()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range append(slices.Clone(protocol.FilePushReasons), filepush.ReasonTimeout, filepush.ReasonGone) {
		href, ok := fileReasonHelp[protocol.ErrorCode(r)]
		if !ok || !m.Resolves(href, "", nil) {
			t.Errorf("reason %q links to %q, not a section of the manual", r, href)
		}
	}
}
