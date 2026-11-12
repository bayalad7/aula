package filepush

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// memDisk is the double of the student's work folder: it never touches the
// disk. Paths are compared without case, like Windows.
type memDisk struct {
	mu    sync.Mutex
	files map[string][]byte
	fail  error // returned by every Create when set
	calls int
}

func newMemDisk() *memDisk { return &memDisk{files: map[string][]byte{}} }

func (m *memDisk) Create(rel string, data []byte, overwrite bool) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.fail != nil {
		return false, m.fail
	}
	if strings.Contains(rel, "..") || strings.HasPrefix(rel, "/") {
		return false, platform.ErrLinkInPath
	}
	key := strings.ToLower(rel)
	_, exists := m.files[key]
	if exists && !overwrite {
		return false, platform.ErrFileExists
	}
	m.files[key] = slices.Clone(data)
	return exists, nil
}

func (m *memDisk) get(rel string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.files[strings.ToLower(rel)]
	return d, ok
}

// wire collects what an end sends.
type wire struct {
	mu   sync.Mutex
	sent []protocol.Payload
	got  chan protocol.Payload
}

func newWire() *wire { return &wire{got: make(chan protocol.Payload, 64)} }

func (w *wire) send(_ context.Context, p protocol.Payload) error {
	w.mu.Lock()
	w.sent = append(w.sent, p)
	w.mu.Unlock()
	w.got <- p
	return nil
}

func (w *wire) next(t *testing.T) protocol.Payload {
	t.Helper()
	select {
	case p := <-w.got:
		return p
	case <-time.After(5 * time.Second):
		t.Fatal("nothing was sent")
		return nil
	}
}

// parts builds the file.push of one file, as the console does.
func parts(t *testing.T, name, dest string, data []byte, overwrite bool) []protocol.FilePush {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "src")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := NewPush("Ejercicio 4", dest, []Source{{Path: path, Rel: name}}, overwrite)
	if err != nil {
		t.Fatal(err)
	}
	return p.Parts(0)
}

func feed(t *testing.T, r *Receiver, w *wire, ps []protocol.FilePush) (last protocol.Payload) {
	t.Helper()
	for _, p := range ps {
		env := protocol.New(p)
		if reply, handled := r.Handle(context.Background(), w.send, env); !handled {
			t.Fatal("file.push not handled")
		} else if reply != nil {
			last = reply
		}
	}
	return last
}

func TestReceiverSavesLargeFileInParts(t *testing.T) {
	disk, w := newMemDisk(), newWire()
	var items []Item
	var mu sync.Mutex
	r := &Receiver{Writer: disk, OnFile: func(i Item) { mu.Lock(); items = append(items, i); mu.Unlock() }}
	data := bytes.Repeat([]byte("0123456789abcdef"), (3<<20)/16+7) // > 3 MiB: 7 parts
	ps := parts(t, "datos.csv", "Ejercicio 4", data, false)
	if len(ps) != 7 {
		t.Fatalf("parts = %d, want 7", len(ps))
	}
	// Out of order and with a repeated part: the reassembly does not care.
	ps = []protocol.FilePush{ps[0], ps[3], ps[3], ps[1], ps[2], ps[4], ps[5], ps[6]}
	if reply := feed(t, r, w, ps); reply != nil {
		t.Fatalf("immediate reply %#v", reply)
	}
	ack := w.next(t).(protocol.FilePushAck)
	if ack.Status != protocol.FilePushSaved || ack.Path != "Ejercicio 4/datos.csv" || ack.Renamed || ack.Replaced {
		t.Fatalf("ack = %+v", ack)
	}
	if got, _ := disk.get("Ejercicio 4/datos.csv"); !bytes.Equal(got, data) {
		t.Fatal("the saved file is not the one sent")
	}
	r.Wait()
	mu.Lock()
	defer mu.Unlock()
	if len(items) != 1 || items[0].Path != ack.Path || items[0].Name != "datos.csv" || !items[0].Saved() {
		t.Fatalf("window items = %+v", items)
	}
}

func TestReceiverRenamesInsteadOfReplacing(t *testing.T) {
	disk, w := newMemDisk(), newWire()
	disk.files["ejercicio 4/datos.csv"] = []byte("del alumno")
	disk.files["ejercicio 4/datos (2).csv"] = []byte("también del alumno")
	r := &Receiver{Writer: disk}
	feed(t, r, w, parts(t, "datos.csv", "Ejercicio 4", []byte("nuevo"), false))
	ack := w.next(t).(protocol.FilePushAck)
	if ack.Path != "Ejercicio 4/datos (3).csv" || !ack.Renamed {
		t.Fatalf("ack = %+v", ack)
	}
	if got, _ := disk.get("Ejercicio 4/datos.csv"); string(got) != "del alumno" {
		t.Fatal("the student's file was replaced")
	}
	// With overwrite, the same name is replaced and the ack says so.
	feed(t, r, w, parts(t, "datos.csv", "Ejercicio 4", []byte("nuevo"), true))
	ack = w.next(t).(protocol.FilePushAck)
	if ack.Path != "Ejercicio 4/datos.csv" || !ack.Replaced || ack.Renamed {
		t.Fatalf("overwrite ack = %+v", ack)
	}
}

func TestReceiverRefusesPathTraversal(t *testing.T) {
	disk, w := newMemDisk(), newWire()
	var items []Item
	r := &Receiver{Writer: disk, OnFile: func(i Item) { items = append(items, i) }}
	good := parts(t, "x.txt", "", []byte("x"), false)[0]
	for _, c := range []struct{ name, dest string }{
		{"../../Windows/win.ini", ""}, {"x.txt", "../.."}, {"x.txt", "a/../../b"}, {`..\x.txt`, ""},
		{"x.txt", `C:\Windows`}, {"x.txt", "/etc"}, {"x.txt", `\\srv\share`}, {"datos.csv:zone", ""},
		{"nul", ""}, {"x.txt", "a//b"}, {"tarea\u202etxt.exe", ""},
	} {
		p := good
		p.FileID, p.Name, p.Dest = protocol.NewID(), c.name, c.dest
		reply := feed(t, r, w, []protocol.FilePush{p})
		ack, ok := reply.(protocol.FilePushAck)
		if !ok || ack.Status != protocol.FilePushFailed || ack.Reason != protocol.FilePushInvalidPath {
			t.Errorf("name %q dest %q: reply %#v, want invalid_path", c.name, c.dest, reply)
		}
	}
	if disk.calls != 0 {
		t.Fatalf("the disk was touched %d times", disk.calls)
	}
	if len(items) != 11 || items[0].Saved() {
		t.Fatalf("the window must hear of every refused file: %d", len(items))
	}
}

func TestReceiverCorruptAndIncomplete(t *testing.T) {
	disk, w := newMemDisk(), newWire()
	r := &Receiver{Writer: disk, PartsWait: 50 * time.Millisecond}
	ps := parts(t, "a.bin", "", bytes.Repeat([]byte{1}, protocol.FilePushPartSize+10), false)
	bad := ps[1]
	bad.Data = slices.Clone(bad.Data)
	bad.Data[0] ^= 0xff
	feed(t, r, w, []protocol.FilePush{ps[0], bad})
	if ack := w.next(t).(protocol.FilePushAck); ack.Reason != protocol.FilePushCorrupt {
		t.Fatalf("corrupt: %+v", ack)
	}
	ps = parts(t, "b.bin", "", bytes.Repeat([]byte{2}, protocol.FilePushPartSize+10), false)
	feed(t, r, w, ps[:1])
	if ack := w.next(t).(protocol.FilePushAck); ack.Reason != protocol.FilePushIncomplete {
		t.Fatalf("incomplete: %+v", ack)
	}
	// The late part of the expired file is ignored.
	if reply := feed(t, r, w, ps[1:]); reply != nil {
		t.Fatalf("late part answered: %#v", reply)
	}
	if disk.calls != 0 {
		t.Fatal("nothing must be written")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.bytes != 0 || len(r.open) != 0 {
		t.Fatalf("memory not released: %d bytes, %d open", r.bytes, len(r.open))
	}
}

func TestReceiverBusyAndTooLarge(t *testing.T) {
	disk, w := newMemDisk(), newWire()
	r := &Receiver{Writer: disk}
	big := bytes.Repeat([]byte{3}, protocol.FilePushPartSize+1)
	for range MaxOpenFiles {
		feed(t, r, w, parts(t, "a.bin", "", big, false)[:1])
	}
	reply := feed(t, r, w, parts(t, "b.bin", "", big, false)[:1])
	if ack, _ := reply.(protocol.FilePushAck); ack.Reason != protocol.FilePushBusy {
		t.Fatalf("fifth file: %#v, want busy", reply)
	}
	r.Reset()
	head := parts(t, "c.bin", "", big, false)[0]
	head.Size, head.Parts = 12<<20, 24 // three of them pass the memory for files
	for range 2 {
		head.FileID = protocol.NewID()
		if reply := feed(t, r, w, []protocol.FilePush{head}); reply != nil {
			t.Fatalf("admitted file answered: %#v", reply)
		}
	}
	head.FileID = protocol.NewID()
	reply = feed(t, r, w, []protocol.FilePush{head})
	if ack, _ := reply.(protocol.FilePushAck); ack.Reason != protocol.FilePushTooLarge {
		t.Fatalf("over the memory: %#v, want too_large", reply)
	}
}

func TestReceiverResendAnswersWithoutWriting(t *testing.T) {
	disk, w := newMemDisk(), newWire()
	r := &Receiver{Writer: disk}
	ps := parts(t, "a.txt", "", []byte("hola"), false)
	feed(t, r, w, ps)
	first := w.next(t).(protocol.FilePushAck)
	r.Wait()
	reply := feed(t, r, w, ps)
	again, ok := reply.(protocol.FilePushAck)
	if !ok || again.Path != first.Path || again.Status != first.Status {
		t.Fatalf("resend: %#v", reply)
	}
	if disk.calls != 1 {
		t.Fatalf("written %d times, want 1", disk.calls)
	}
}

func TestReceiverEmptyFileAndWriteErrors(t *testing.T) {
	disk, w := newMemDisk(), newWire()
	r := &Receiver{Writer: disk}
	feed(t, r, w, parts(t, "__init__.py", "pkg", nil, false))
	if ack := w.next(t).(protocol.FilePushAck); ack.Status != protocol.FilePushSaved {
		t.Fatalf("empty file: %+v", ack)
	}
	for err, want := range map[error]protocol.FilePushReason{
		platform.ErrNoSpace:      protocol.FilePushNoSpace,
		platform.ErrDenied:       protocol.FilePushDenied,
		platform.ErrPathTooLong:  protocol.FilePushPathTooLong,
		platform.ErrLinkInPath:   protocol.FilePushInvalidPath,
		platform.ErrNotSupported: protocol.FilePushUnsupported,
		errors.New("otro"):       protocol.FilePushInternalFail,
	} {
		disk.fail = err
		feed(t, r, w, parts(t, "x.txt", "", []byte("x"), true))
		if ack := w.next(t).(protocol.FilePushAck); ack.Reason != want {
			t.Errorf("%v: reason %q, want %q", err, ack.Reason, want)
		}
	}
	disk.fail = nil
	// Every alternative taken.
	for n := 1; n <= MaxAlternative; n++ {
		disk.files[strings.ToLower(Alternative("y.txt", n))] = nil
	}
	feed(t, r, w, parts(t, "y.txt", "", []byte("y"), false))
	if ack := w.next(t).(protocol.FilePushAck); ack.Reason != protocol.FilePushExists {
		t.Fatalf("every name taken: %+v", ack)
	}
}

func TestReceiverWithoutWriter(t *testing.T) {
	r := &Receiver{}
	if r.Capabilities() != nil {
		t.Fatal("without a work folder the agent must not declare file.push")
	}
	reply, handled := r.Handle(context.Background(), newWire().send, protocol.New(parts(t, "a", "", []byte("a"), false)[0]))
	if e, ok := reply.(protocol.ErrorPayload); !handled || !ok || e.Code != protocol.CodeUnsupportedCapability {
		t.Fatalf("reply %#v", reply)
	}
	if _, handled := r.Handle(context.Background(), nil, protocol.New(protocol.Heartbeat{})); handled {
		t.Fatal("other messages are not its business")
	}
}

func TestReceiverLogsNoNames(t *testing.T) {
	var log bytes.Buffer
	disk, w := newMemDisk(), newWire()
	r := &Receiver{Writer: disk, Logger: slogTo(&log)}
	feed(t, r, w, parts(t, "secreto-nombre.csv", "carpeta-secreta", []byte("contenido-secreto"), false))
	w.next(t)
	r.Wait()
	for _, s := range []string{"secreto-nombre", "carpeta-secreta", "contenido-secreto", "Ejercicio 4"} {
		if strings.Contains(log.String(), s) {
			t.Errorf("the log has %q: %s", s, log.String())
		}
	}
}

func TestInboxGroupsAndFindsTheFolder(t *testing.T) {
	var in Inbox
	at := time.Now()
	in.Add(Item{PushID: "p1", FileID: "f2", Index: 2, Files: 2, Name: "b.csv", Path: "Ej 4/datos/b.csv", At: at})
	list := in.Add(Item{PushID: "p1", FileID: "f1", Index: 1, Files: 2, Name: "a.py", Path: "Ej 4/a.py", At: at})
	if len(list) != 1 || len(list[0].Items) != 2 || list[0].Items[0].Name != "a.py" || list[0].Folder != "Ej 4" {
		t.Fatalf("list = %+v", list)
	}
	list = in.Add(Item{PushID: "p2", FileID: "f3", Index: 1, Files: 1, Name: "x", Reason: protocol.FilePushNoSpace, At: at})
	if list[0].PushID != "p2" || list[0].Folder != "" {
		t.Fatalf("newest first: %+v", list)
	}
	if f, ok := in.Folder("p1"); !ok || f != "Ej 4" {
		t.Fatalf("Folder(p1) = %q, %v", f, ok)
	}
	if _, ok := in.Folder("p2"); ok {
		t.Fatal("a push with nothing saved has no folder to open")
	}
	for i := range MaxInbox + 5 {
		in.Add(Item{PushID: protocol.NewID(), FileID: "f", Index: 1, Files: 1, Path: "x", At: at.Add(time.Duration(i))})
	}
	if n := len(in.List()); n != MaxInbox {
		t.Fatalf("inbox keeps %d, want %d", n, MaxInbox)
	}
}

func TestNewPushChecksLimitsAndNames(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, size int) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, bytes.Repeat([]byte{'x'}, size), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	small := write("small", 10)
	if _, err := NewPush("", "", nil, false); !errors.Is(err, ErrNoFiles) {
		t.Errorf("no files: %v", err)
	}
	many := make([]Source, protocol.MaxFilePushFiles+1)
	for i := range many {
		many[i] = Source{Path: small, Rel: "f" + strings.Repeat("x", i) + ".txt"}
	}
	if _, err := NewPush("", "", many, false); !errors.Is(err, ErrTooManyFiles) {
		t.Errorf("too many: %v", err)
	}
	if _, err := NewPush("", "", []Source{{Path: write("big", protocol.MaxFilePushBytes+1), Rel: "big.bin"}}, false); !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("file too large: %v", err)
	}
	half := write("half", protocol.MaxFilePushBytes)
	three := []Source{{Path: half, Rel: "a"}, {Path: half, Rel: "b"}, {Path: small, Rel: "c"}}
	if _, err := NewPush("", "", three, false); !errors.Is(err, ErrPushTooLarge) {
		t.Errorf("push too large: %v", err)
	}
	for _, s := range []Source{{Path: small, Rel: "../x.txt"}, {Path: small, Rel: "con.txt"}, {Path: small, Rel: "a/b/c/d/e/f.txt"}} {
		var fe *FileError
		if _, err := NewPush("", "", []Source{s}, false); !errors.Is(err, ErrInvalidPath) || !errors.As(err, &fe) {
			t.Errorf("%q: %v, want a FileError with ErrInvalidPath", s.Rel, err)
		}
	}
	if _, err := NewPush("", "../fuera", []Source{{Path: small, Rel: "x"}}, false); !errors.Is(err, ErrInvalidPath) {
		t.Errorf("dest outside: %v", err)
	}
	if _, err := NewPush("", "", []Source{{Path: small, Rel: "A.txt"}, {Path: small, Rel: "a.TXT"}}, false); !errors.Is(err, ErrDuplicate) {
		t.Errorf("duplicate: %v", err)
	}
	if _, err := NewPush("", "", []Source{{Path: filepath.Join(dir, "missing"), Rel: "m"}}, false); !errors.Is(err, ErrUnreadable) {
		t.Errorf("missing: %v", err)
	}
	p, err := NewPush(strings.Repeat("l", 100), ` Ejercicio 4\ `, []Source{{Path: small, Rel: "datos/a.csv"}, {Path: small, Rel: "main.py"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if p.Dest != "Ejercicio 4" || p.Files[0].Dest != "Ejercicio 4/datos" || p.Files[1].Dest != "Ejercicio 4" {
		t.Fatalf("dests = %q %q %q", p.Dest, p.Files[0].Dest, p.Files[1].Dest)
	}
	head := p.Parts(1)[0]
	if err := head.Validate(); err != nil || head.Index != 2 || head.Files != 2 || !head.Overwrite {
		t.Fatalf("part 1 = %+v, %v", head, err)
	}
}

// pipe connects a Pusher to Receivers, one per session, as the network
// would; drop sessions never answer.
type pipe struct {
	t     *testing.T
	recv  map[string]*Receiver
	disks map[string]*memDisk
	push  *Pusher
	drop  map[string]bool
}

func (p *pipe) send(ctx context.Context, session string, payload protocol.Payload) (string, error) {
	env := protocol.New(payload)
	if p.drop[session] {
		return env.ID, nil
	}
	r := p.recv[session]
	if r == nil {
		return "", errors.New("gone")
	}
	answer := func(_ context.Context, a protocol.Payload) error {
		switch a := a.(type) {
		case protocol.FilePushAck:
			go p.push.Ack(session, a)
		case protocol.ErrorPayload:
			go p.push.Error(session, a)
		}
		return nil
	}
	// The receiver lives as long as the connection, not as the send.
	if reply, _ := r.Handle(context.Background(), answer, env); reply != nil {
		_ = answer(ctx, reply)
	}
	return env.ID, nil
}

func TestPusherDeliversToEverySession(t *testing.T) {
	dir := t.TempDir()
	src := func(name string, data []byte) Source {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return Source{Path: path, Rel: name}
	}
	push, err := NewPush("Ejercicio 4", "Ejercicio 4", []Source{
		src("main.py", []byte("print(1)\n")),
		src("datos.csv", bytes.Repeat([]byte("1,2\n"), 400_000)), // 1.6 MB: 4 parts
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	p := &pipe{t: t, recv: map[string]*Receiver{}, disks: map[string]*memDisk{}, drop: map[string]bool{"lento": true}}
	for _, s := range []string{"pc1", "pc2", "pc3", "pc4", "pc5"} {
		p.disks[s] = newMemDisk()
		p.recv[s] = &Receiver{Writer: p.disks[s]}
	}
	p.disks["pc5"].fail = platform.ErrNoSpace
	var mu sync.Mutex
	outcomes := map[string]Outcome{}
	p.push = &Pusher{Send: p.send, AckWait: 300 * time.Millisecond, Parallel: 2,
		OnOutcome: func(o Outcome) { mu.Lock(); outcomes[o.Session] = o; mu.Unlock() }}
	p.push.Deliver(context.Background(), push, []string{"pc1", "pc2", "pc3", "pc4", "pc5", "lento", "ido"})
	p.push.Wait()
	for _, s := range []string{"pc1", "pc2", "pc3", "pc4"} {
		if o := outcomes[s]; !o.OK() || o.Saved != 2 || !slices.Equal(o.Paths, []string{"Ejercicio 4/main.py", "Ejercicio 4/datos.csv"}) {
			t.Errorf("%s: %+v", s, o)
		}
		if got, _ := p.disks[s].get("Ejercicio 4/datos.csv"); !bytes.Equal(got, push.Files[1].Data) {
			t.Errorf("%s: datos.csv differs", s)
		}
	}
	if o := outcomes["pc5"]; o.Reason != string(protocol.FilePushNoSpace) || o.Saved != 0 {
		t.Errorf("pc5: %+v", o)
	}
	if o := outcomes["lento"]; o.Reason != ReasonTimeout {
		t.Errorf("lento: %+v", o)
	}
	if o := outcomes["ido"]; o.Reason != ReasonGone || o.Saved != 0 {
		t.Errorf("ido: %+v", o)
	}
}

func TestPusherStopsWithTheClass(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	dir := t.TempDir()
	path := filepath.Join(dir, "a")
	_ = os.WriteFile(path, []byte("a"), 0o600)
	push, _ := NewPush("", "", []Source{{Path: path, Rel: "a.txt"}}, false)
	done := make(chan Outcome, 1)
	p := &Pusher{Send: func(context.Context, string, protocol.Payload) (string, error) { return protocol.NewID(), nil },
		OnOutcome: func(o Outcome) { done <- o }}
	p.Deliver(ctx, push, []string{"pc1"})
	cancel()
	select {
	case o := <-done:
		if o.Reason != ReasonGone {
			t.Fatalf("outcome %+v", o)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the delivery did not stop with the class")
	}
}

func TestTemplates(t *testing.T) {
	src := t.TempDir()
	write := func(rel string, data string) Source {
		p := filepath.Join(src, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o700)
		if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		return Source{Path: p, Rel: rel}
	}
	tpls := Templates{Dir: filepath.Join(t.TempDir(), TemplatesFolder)}
	if l, err := tpls.List(); err != nil || len(l) != 0 {
		t.Fatalf("no folder yet: %v, %v", l, err)
	}
	files := []Source{write("main.py", "print(1)"), write("datos/a.csv", "1,2")}
	if err := tpls.Save("Ejercicio 4", files); err != nil {
		t.Fatal(err)
	}
	if err := tpls.Save("Ejercicio 4", files); !errors.Is(err, ErrTemplateExists) {
		t.Fatalf("second save: %v", err)
	}
	for _, bad := range []string{"../fuera", "con", "", "a/b"} {
		if err := tpls.Save(bad, files); !errors.Is(err, ErrInvalidPath) && !errors.Is(err, ErrNoFiles) {
			t.Errorf("Save(%q) = %v", bad, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(tpls.Dir), "fuera")); err == nil {
		t.Fatal("a template was written outside its folder")
	}
	// One the teacher made in Explorer, too big to send.
	_ = os.MkdirAll(filepath.Join(tpls.Dir, "Grande"), 0o700)
	for i := range protocol.MaxFilePushFiles + 1 {
		_ = os.WriteFile(filepath.Join(tpls.Dir, "Grande", "f"+strings.Repeat("x", i)), []byte("x"), 0o600)
	}
	_ = os.MkdirAll(filepath.Join(tpls.Dir, ".nueva-123"), 0o700)
	list, err := tpls.List()
	if err != nil || len(list) != 2 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if list[0].Name != "Ejercicio 4" || len(list[0].Files) != 2 || list[0].Problem != "" || list[0].Size != 11 {
		t.Fatalf("template = %+v", list[0])
	}
	if list[1].Problem != ProblemTooMany {
		t.Fatalf("Grande = %+v", list[1])
	}
	srcs, err := tpls.Sources("Ejercicio 4")
	if err != nil {
		t.Fatal(err)
	}
	push, err := NewPush("Ejercicio 4", "Ejercicio 4", srcs, false)
	if err != nil || len(push.Files) != 2 {
		t.Fatalf("push from template: %v", err)
	}
	if _, err := tpls.Sources("No existe"); !errors.Is(err, ErrTemplateMissing) {
		t.Fatalf("missing template: %v", err)
	}
	if _, err := tpls.Path("../.."); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("Path outside: %v", err)
	}
}

func slogTo(w *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// TestTemplateDeleteStaysInside: Delete removes only a template folder right
// under Dir; a link in its place is not followed and its target is kept.
func TestTemplateDeleteStaysInside(t *testing.T) {
	dir := t.TempDir()
	tpl := Templates{Dir: filepath.Join(dir, "plantillas")}
	src := filepath.Join(dir, "main.py")
	if err := os.WriteFile(src, []byte("print(1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := tpl.Save("Ejercicio 4", []Source{{Path: src, Rel: "main.py"}}); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "fuera")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(tpl.Dir, "Enlace")); err == nil {
		if err := tpl.Delete("Enlace"); !errors.Is(err, ErrTemplateMissing) {
			t.Fatalf("delete a link = %v", err)
		}
		if _, err := os.Stat(outside); err != nil {
			t.Fatal("the link's target was removed")
		}
	} else {
		t.Logf("no symlink here (%v); only the folder case runs", err)
	}
	for _, bad := range []string{"..", "../fuera", ""} {
		if err := tpl.Delete(bad); err == nil {
			t.Fatalf("delete %q accepted", bad)
		}
	}
	if err := tpl.Delete("Ejercicio 4"); err != nil {
		t.Fatal(err)
	}
	if l, _ := tpl.List(); len(l) != 0 {
		t.Fatalf("templates after delete: %+v", l)
	}
	if err := tpl.Delete("Ejercicio 4"); !errors.Is(err, ErrTemplateMissing) {
		t.Fatalf("second delete = %v", err)
	}
}
