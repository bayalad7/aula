package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/filepush"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// fakeWorkFolder is the double of Documents\Aula: it never touches the disk
// nor opens a window on the desktop.
type fakeWorkFolder struct {
	mu     sync.Mutex
	files  map[string]string
	opened []string
}

func (f *fakeWorkFolder) Root() (string, error) { return `C:\Users\ana\Documents\Aula`, nil }

func (f *fakeWorkFolder) Create(rel string, data []byte, overwrite bool) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.files == nil {
		f.files = map[string]string{}
	}
	_, exists := f.files[rel]
	if exists && !overwrite {
		return false, platform.ErrFileExists
	}
	f.files[rel] = string(data)
	return exists, nil
}

func (f *fakeWorkFolder) Open(rel string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, rel)
	return nil
}

func filePart(pushID, name, dest, content string) protocol.FilePush {
	s := sha256.Sum256([]byte(content))
	return protocol.FilePush{PushID: pushID, FileID: protocol.NewID(), Part: 1, Parts: 1, Data: []byte(content),
		Name: name, Dest: dest, Size: len(content), SHA256: hex.EncodeToString(s[:]), Label: "Ejercicio 4",
		Index: 1, Files: 1}
}

// A file from the teacher (docs/privacidad.md rules 55-58): it lands in the
// work folder, the ack says where, and the window gets a card. It does not
// act on the computer: notify, never raise.
func TestReceivedFileNotifiesNeverRaises(t *testing.T) {
	a, rec := newTestApp(nil)
	wf := &fakeWorkFolder{files: map[string]string{"Ejercicio 4/datos.csv": "del alumno"}}
	a.enableFiles(wf)
	if !protocol.Has(a.hello.Capabilities, protocol.CapFilePush) {
		t.Fatalf("hello capabilities = %v, want file.push", a.hello.Capabilities)
	}
	a.onState(session.AgentConnected, discovery.Console{Name: "Prof. Ayala"})
	before := rec.notified

	acks := make(chan protocol.Payload, 1)
	send := func(_ context.Context, p protocol.Payload) error { acks <- p; return nil }
	push := protocol.NewID()
	if reply, ok := a.files.Handle(t.Context(), filepush.Sender(send), protocol.New(filePart(push, "datos.csv", "Ejercicio 4", "1,2\n"))); !ok || reply != nil {
		t.Fatalf("Handle = %v, %v", reply, ok)
	}
	a.files.Wait()
	ack := (<-acks).(protocol.FilePushAck)
	if ack.Status != protocol.FilePushSaved || ack.Path != "Ejercicio 4/datos (2).csv" || !ack.Renamed {
		t.Fatalf("ack = %+v", ack)
	}
	if wf.files["Ejercicio 4/datos.csv"] != "del alumno" {
		t.Fatal("the student's file was replaced")
	}
	files, _ := rec.last(EventFiles).(Files)
	if len(files.Received) != 1 || files.Folder != "Aula" {
		t.Fatalf("files = %+v", files)
	}
	r := files.Received[0]
	if r.PushID != push || r.Folder != "Ejercicio 4" || len(r.Items) != 1 || !r.Items[0].Renamed || r.Items[0].Name != "datos.csv" {
		t.Fatalf("card = %+v", r)
	}
	if rec.raised != 0 || rec.notified != before+1 {
		t.Fatalf("raised %d, notified %d; want 0 and %d", rec.raised, rec.notified, before+1)
	}
	if v := a.View(); len(v.Files.Received) != 1 {
		t.Fatalf("View().Files = %+v", v.Files)
	}

	// «Abrir carpeta» opens where the push went, and only that.
	if err := a.OpenReceivedFolder(push); err != nil {
		t.Fatal(err)
	}
	if len(wf.opened) != 1 || wf.opened[0] != "Ejercicio 4" {
		t.Fatalf("opened %q", wf.opened)
	}
	for _, id := range []string{"", protocol.NewID(), `..\..\Windows`} {
		if err := a.OpenReceivedFolder(id); !errors.Is(err, errNoFolder) {
			t.Errorf("OpenReceivedFolder(%q) = %v", id, err)
		}
	}
}

// A refused file also reaches the window, so the student knows something
// did not arrive, and nothing is written.
func TestRefusedFileTellsTheStudent(t *testing.T) {
	a, rec := newTestApp(nil)
	wf := &fakeWorkFolder{}
	a.enableFiles(wf)
	p := filePart(protocol.NewID(), "win.ini", "../../Windows", "x")
	reply, _ := a.files.Handle(t.Context(), func(context.Context, protocol.Payload) error { return nil }, protocol.New(p))
	if ack, ok := reply.(protocol.FilePushAck); !ok || ack.Reason != protocol.FilePushInvalidPath {
		t.Fatalf("reply = %#v", reply)
	}
	if len(wf.files) != 0 {
		t.Fatalf("written: %v", wf.files)
	}
	files, _ := rec.last(EventFiles).(Files)
	if len(files.Received) != 1 || files.Received[0].Items[0].Reason != protocol.FilePushInvalidPath {
		t.Fatalf("files = %+v", files)
	}
	if err := a.OpenReceivedFolder(p.PushID); !errors.Is(err, errNoFolder) {
		t.Fatalf("a push with nothing saved has no folder: %v", err)
	}
}

func TestNoFileCapabilityWithoutWorkFolder(t *testing.T) {
	a, _ := newTestApp(nil)
	if protocol.Has(a.hello.Capabilities, protocol.CapFilePush) {
		t.Fatal("file.push declared without a work folder")
	}
	reply, _ := a.files.Handle(t.Context(), nil, protocol.New(filePart(protocol.NewID(), "a.txt", "", "a")))
	if e, ok := reply.(protocol.ErrorPayload); !ok || e.Code != protocol.CodeUnsupportedCapability {
		t.Fatalf("reply = %#v", reply)
	}
	if v := a.View(); v.Files.Received == nil {
		t.Fatal("the window expects a list, not null")
	}
}

// As a service, file.push is declared only with a helper that has the work
// folder, and the service never writes itself: it goes through the helper,
// so the file belongs to the student (docs/privacidad.md rule 55).
func TestServiceSavesFilesOnlyThroughItsHelper(t *testing.T) {
	sa := newServiceAgent(Config{Channel: "1"}, nil, nil, quiet)
	for _, tc := range []struct {
		ready *agentsvc.Ready
		want  bool
	}{
		{nil, false},
		{&agentsvc.Ready{Capture: true, Clipboard: true}, false},
		{&agentsvc.Ready{Files: true}, true},
	} {
		sa.mu.Lock()
		sa.ready = tc.ready
		sa.mu.Unlock()
		if got := protocol.Has(sa.hello().Capabilities, protocol.CapFilePush); got != tc.want {
			t.Errorf("ready %+v: file.push declared %v, want %v", tc.ready, got, tc.want)
		}
	}
	if _, ok := sa.app.files.Writer.(agentsvc.WorkFolder); !ok {
		t.Fatalf("the service writes with %T, want the helper's proxy", sa.app.files.Writer)
	}
	// Without a helper the write fails and nothing else is tried.
	if _, err := sa.app.files.Writer.Create("a.txt", []byte("a"), false); !errors.Is(err, agentsvc.ErrNoHelper) {
		t.Fatalf("write without helper = %v", err)
	}
	b, _ := agentsvc.NewBinding("OpenReceivedFolder", protocol.NewID())
	raw, _ := jsonOf(b)
	if _, err := sa.binding(t.Context(), agentsvc.CallBinding, raw); !errors.Is(err, errNoFolder) {
		t.Fatalf("OpenReceivedFolder through the service = %v", err)
	}
}

// The helper writes only checked paths, whatever the service asks.
func TestHelperRefusesUncheckedPaths(t *testing.T) {
	wf := &fakeWorkFolder{}
	h := &helper{work: wf, log: quiet}
	for _, rel := range []string{"../x.txt", "a/../x.txt", "/x.txt", `a\x.txt`, "con", "a//x"} {
		args, _ := jsonOf(agentsvc.FileWriteArgs{Rel: rel, Data: []byte("x")})
		out, err := h.call(t.Context(), agentsvc.CallFileWrite, args)
		r, _ := out.(agentsvc.FileWriteResult)
		if err != nil || r.Error == "" {
			t.Errorf("rel %q: %+v, %v; want a refusal", rel, out, err)
		}
	}
	if len(wf.files) != 0 {
		t.Fatalf("written: %v", wf.files)
	}
	args, _ := jsonOf(agentsvc.FileWriteArgs{Rel: "Ejercicio 4/a.py", Data: []byte("print(1)")})
	out, err := h.call(t.Context(), agentsvc.CallFileWrite, args)
	if r, _ := out.(agentsvc.FileWriteResult); err != nil || r.Error != "" || wf.files["Ejercicio 4/a.py"] != "print(1)" {
		t.Fatalf("write = %+v, %v", out, err)
	}
	for _, rel := range []string{"..", "a/../..", `C:\`} {
		args, _ := jsonOf(rel)
		if _, err := h.call(t.Context(), agentsvc.CallFolderOpen, args); err == nil {
			t.Errorf("open %q accepted", rel)
		}
	}
	if strings.Join(wf.opened, "|") != "" {
		t.Fatalf("opened %q", wf.opened)
	}
}

func jsonOf(v any) (json.RawMessage, error) { return json.Marshal(v) }
