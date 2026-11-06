package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"strings"
	"sync"
	"testing"

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/collect"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// fakeReader is the double of the work folder for a collection: it never
// touches the disk and records every path asked for.
type fakeReader struct {
	mu    sync.Mutex
	dirs  map[string]bool
	files map[string]string
	asked []string
}

func (f *fakeReader) MkdirAll(rel string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, rel)
	if f.dirs == nil {
		f.dirs = map[string]bool{}
	}
	f.dirs[rel] = true
	return nil
}

func (f *fakeReader) List(rel string, _ int) ([]platform.WorkFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, rel)
	if !f.dirs[rel] {
		return nil, fs.ErrNotExist
	}
	var out []platform.WorkFile
	for p, c := range f.files {
		if strings.HasPrefix(p, rel+"/") {
			out = append(out, platform.WorkFile{Path: strings.TrimPrefix(p, rel+"/"), Size: int64(len(c))})
		}
	}
	return out, nil
}

func (f *fakeReader) Read(rel string, _ int64) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, rel)
	c, ok := f.files[rel]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(c), nil
}

// A collection (docs/privacidad.md rules 60–63): the agent declares
// file.collect, creates the folder, tells the student before reading and
// after, reads only the activity folder, and never raises the window.
func TestCollectNotifiesNeverRaises(t *testing.T) {
	a, rec := newTestApp(nil)
	act := "recolecciones/2026-11-06/Ejercicio 4"
	rd := &fakeReader{files: map[string]string{act + "/20231234-Ana.py": "print(1)\n", "fuera.py": "no"}}
	a.enableCollect(rd)
	wf := &fakeWorkFolder{}
	a.work = wf
	if !protocol.Has(a.hello.Capabilities, protocol.CapFileCollect) {
		t.Fatalf("hello capabilities = %v, want file.collect", a.hello.Capabilities)
	}
	id := protocol.NewID()
	sent := make(chan protocol.Payload, 10)
	send := func(_ context.Context, p protocol.Payload) error {
		sent <- p
		if r, ok := p.(protocol.FileCollectResponse); ok {
			// The console saves it and acks.
			go a.collect.Handle(t.Context(), func(context.Context, protocol.Payload) error { return nil }, protocol.New(
				protocol.FileCollectResponseAck{RefID: protocol.NewID(), CollectID: id, Round: r.Round, FileID: r.FileID, Status: protocol.FilePushSaved}))
		}
		return nil
	}
	open := protocol.FileCollectOpen{CollectID: id, Date: "2026-11-06", Activity: "Ejercicio 4", Patterns: []string{"*.py"}}
	reply, ok := a.collect.Handle(t.Context(), send, protocol.New(open))
	if ack, isAck := reply.(protocol.FileCollectAck); !ok || !isAck || ack.Status != protocol.FileCollectReady || ack.Path != act {
		t.Fatalf("open = %+v, %v", reply, ok)
	}
	if v, ok := rec.last(EventCollections).(Collections); !ok || len(v.List) != 1 || v.List[0].Folder != act || v.Folder != "Aula" {
		t.Fatalf("collections event = %#v", rec.last(EventCollections))
	}
	req := protocol.FileCollectRequest{CollectID: id, Date: "2026-11-06", Activity: "Ejercicio 4", Round: 1,
		Patterns: []string{"*.py"}, MaxFileSize: 1 << 20, MaxTotal: 1 << 20, MaxFiles: 5}
	if reply, _ := a.collect.Handle(t.Context(), send, protocol.New(req)); reply != nil {
		t.Fatalf("request answered at once: %+v", reply)
	}
	a.collect.Wait()
	var notices []string
	for _, n := range a.View().Notices {
		if n.Ref == id {
			notices = append(notices, n.Kind)
		}
	}
	if len(notices) != 2 || notices[0] != NoticeCollectRequested || notices[1] != NoticeCollectDone {
		t.Fatalf("notices = %v, want requested then done", notices)
	}
	if rec.raised != 0 || rec.notified < 3 {
		t.Fatalf("raised %d, notified %d; a collection never raises", rec.raised, rec.notified)
	}
	for _, p := range rd.asked {
		if p != act && !strings.HasPrefix(p, act+"/") {
			t.Errorf("asked outside the activity folder: %q", p)
		}
	}
	v := a.View().Collections
	if len(v.List) != 1 || v.List[0].State != collect.StateCollected || len(v.List[0].Rounds) != 1 ||
		len(v.List[0].Rounds[0].Files) != 1 || !v.List[0].Rounds[0].Files[0].Saved {
		t.Fatalf("view = %+v", v)
	}

	// «Abrir carpeta» opens only that activity folder, by its ID.
	if err := a.OpenCollectionFolder(id); err != nil || len(wf.opened) != 1 || wf.opened[0] != act {
		t.Fatalf("open folder = %v, %v", err, wf.opened)
	}
	if err := a.OpenCollectionFolder("otra"); err == nil {
		t.Fatal("an unknown collection must not open a folder")
	}
}

// In service mode the helper reads as the student and checks the path again:
// only an activity folder or a file inside one.
func TestHelperCollectChecksThePath(t *testing.T) {
	rd := &fakeReader{files: map[string]string{"recolecciones/2026-11-06/Ej/a.py": "x"}}
	h := &helper{read: rd, log: quiet}
	call := func(name string, args any) any {
		t.Helper()
		b, _ := json.Marshal(args)
		r, err := h.collectCall(name, b)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return r
	}
	if r := call(agentsvc.CallFolderCreate, "recolecciones/2026-11-06/Ej").(agentsvc.FolderCreateResult); r.Error != "" {
		t.Fatalf("create = %+v", r)
	}
	if r := call(agentsvc.CallFileRead, agentsvc.FileReadArgs{Rel: "recolecciones/2026-11-06/Ej/a.py", Max: 10}).(agentsvc.FileReadResult); r.Error != "" || string(r.Data) != "x" {
		t.Fatalf("read = %+v", r)
	}
	for _, bad := range []string{"Ejercicio 4", "plantillas/x/y", "recolecciones/2026-11-06/../../Desktop", "recolecciones/x/Ej"} {
		if r := call(agentsvc.CallFolderCreate, bad).(agentsvc.FolderCreateResult); r.Error == "" {
			t.Errorf("create %q accepted", bad)
		}
		if r := call(agentsvc.CallFolderList, agentsvc.FolderListArgs{Rel: bad, Depth: 4}).(agentsvc.FolderListResult); r.Error == "" {
			t.Errorf("list %q accepted", bad)
		}
	}
	for _, bad := range []string{"secreto.py", "recolecciones/2026-11-06/Ej/../../../secreto.py", "recolecciones/2026-11-06/Ej"} {
		if r := call(agentsvc.CallFileRead, agentsvc.FileReadArgs{Rel: bad, Max: 10}).(agentsvc.FileReadResult); r.Error == "" {
			t.Errorf("read %q accepted", bad)
		}
	}
	for _, p := range rd.asked {
		if !strings.HasPrefix(p, "recolecciones/2026-11-06/Ej") {
			t.Errorf("the work folder was asked for %q", p)
		}
	}
	none := &helper{log: quiet}
	b, _ := json.Marshal("recolecciones/2026-11-06/Ej")
	if r, _ := none.collectCall(agentsvc.CallFolderCreate, b); r.(agentsvc.FolderCreateResult).Error == "" {
		t.Error("without a work folder the helper must answer unsupported")
	}
}
