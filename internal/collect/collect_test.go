package collect

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// memReader is a work folder in memory: path → content, plus marked links
// and cloud-only files.
type memReader struct {
	mu      sync.Mutex
	dirs    map[string]bool
	files   map[string][]byte
	links   map[string]bool
	offline map[string]bool
	reads   []string
	denied  map[string]bool
}

func newMem() *memReader {
	return &memReader{dirs: map[string]bool{}, files: map[string][]byte{}, links: map[string]bool{},
		offline: map[string]bool{}, denied: map[string]bool{}}
}

func (m *memReader) MkdirAll(rel string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dirs[rel] = true
	return nil
}

func (m *memReader) List(rel string, depth int) ([]platform.WorkFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.dirs[rel] {
		return nil, fs.ErrNotExist
	}
	var out []platform.WorkFile
	for p, d := range m.files {
		if !strings.HasPrefix(p, rel+"/") {
			continue
		}
		in := strings.TrimPrefix(p, rel+"/")
		if strings.Count(in, "/")+1 > depth {
			continue
		}
		out = append(out, platform.WorkFile{Path: in, Size: int64(len(d)), ModTime: time.UnixMilli(1793955720000),
			Offline: m.offline[p]})
	}
	for p := range m.links {
		out = append(out, platform.WorkFile{Path: strings.TrimPrefix(p, rel+"/"), Link: true})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (m *memReader) Read(rel string, max int64) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reads = append(m.reads, rel)
	if m.denied[rel] {
		return nil, platform.ErrDenied
	}
	d, ok := m.files[rel]
	if !ok {
		return nil, fs.ErrNotExist
	}
	if int64(len(d)) > max {
		return nil, platform.ErrFileTooLarge
	}
	return append([]byte(nil), d...), nil
}

// lab wires a Collector to one Responder per session through the wire
// format (Encode/Decode), the way the WebSocket would.
type lab struct {
	t       *testing.T
	col     *Collector
	agents  map[string]*Responder
	events  map[string][]Event
	mu      sync.Mutex
	drop    func(session string, p protocol.Payload) bool // a lost message
	tamper  func(p *protocol.FileCollectResponse)
	ctx     context.Context
	changes int
}

func wire(t *testing.T, p protocol.Payload) protocol.Envelope {
	t.Helper()
	b, err := protocol.Encode(protocol.New(p))
	if err != nil {
		t.Errorf("encode %s: %v", p.MessageType(), err)
		return protocol.Envelope{}
	}
	if len(b) > protocol.MaxMessageSize {
		t.Errorf("%s is %d bytes", p.MessageType(), len(b))
	}
	env, err := protocol.Decode(b)
	if err != nil {
		t.Errorf("decode %s: %v", p.MessageType(), err)
	}
	return env
}

func newLab(t *testing.T, sessions ...string) *lab {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	l := &lab{t: t, agents: map[string]*Responder{}, events: map[string][]Event{}, ctx: ctx}
	root := filepath.Join(t.TempDir(), "recolecciones")
	l.col = &Collector{Root: func() (string, error) { return root, nil }, ReplyWait: 10 * time.Second, FileWait: 10 * time.Second,
		OnChange: func(string) { l.mu.Lock(); l.changes++; l.mu.Unlock() }}
	l.col.Send = func(ctx context.Context, session string, p protocol.Payload) (string, error) {
		if l.drop != nil && l.drop(session, p) {
			return protocol.NewID(), nil
		}
		env := wire(t, p)
		r := l.agents[session]
		if r == nil {
			return "", errors.New("gone")
		}
		reply, handled := r.Handle(l.ctx, l.agentSend(session), env)
		if !handled {
			t.Errorf("agent did not handle %s", p.MessageType())
		}
		if reply != nil {
			_ = l.agentSend(session)(ctx, reply)
		}
		return env.ID, nil
	}
	for _, s := range sessions {
		s := s
		l.agents[s] = &Responder{Reader: newMem(), AckWait: 10 * time.Second, OnChange: func(_ Collection, ev Event) {
			l.mu.Lock()
			l.events[s] = append(l.events[s], ev)
			l.mu.Unlock()
		}}
	}
	return l
}

func (l *lab) agentSend(session string) Sender {
	return func(ctx context.Context, p protocol.Payload) error {
		if l.drop != nil && l.drop(session, p) {
			return nil
		}
		if resp, ok := p.(protocol.FileCollectResponse); ok && l.tamper != nil {
			l.tamper(&resp)
			p = resp
		}
		env := wire(l.t, p)
		switch m := env.Payload.(type) {
		case protocol.FileCollectAck:
			l.col.Ack(session, m)
		case protocol.FileCollectResponse:
			if reply := l.col.Response(session, env.ID, m); reply != nil {
				r := l.agents[session]
				r.Handle(ctx, l.agentSend(session), wire(l.t, reply))
			}
		case protocol.ErrorPayload:
			l.col.Error(session, m)
		default:
			l.t.Errorf("unexpected %s from the agent", p.MessageType())
		}
		return nil
	}
}

func (l *lab) mem(session string) *memReader { return l.agents[session].Reader.(*memReader) }

func (l *lab) students() []Student {
	var out []Student
	for s := range l.agents {
		out = append(out, Student{Session: s, Hostname: "LAB-" + s, Username: "alumno-" + s})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Session < out[j].Session })
	return out
}

func (l *lab) wait() {
	l.col.Wait()
	for _, r := range l.agents {
		r.Wait()
	}
}

const act = "recolecciones/2026-11-06/Ejercicio 4"

func randomBytes(t *testing.T, n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

// TestCollectTwoStudentsWithALargeFile is the heart of the function: two
// students, one with a file bigger than a message, both arrive whole and
// organized per student.
func TestCollectTwoStudentsWithALargeFile(t *testing.T) {
	l := newLab(t, "a", "b")
	id, err := l.col.Open(l.ctx, "2026-11-06", "Ejercicio 4", []string{"*.py", "*.bin"}, DefaultLimits, l.students())
	if err != nil {
		t.Fatal(err)
	}
	for s, r := range l.agents {
		if !r.Reader.(*memReader).dirs[act] {
			t.Fatalf("%s: the activity folder was not created", s)
		}
	}
	sum, _ := l.col.Get(id)
	for _, st := range sum.Students {
		if st.Status != StatusReady {
			t.Fatalf("%s after open = %s %s", st.Session, st.Status, st.Reason)
		}
	}
	big := randomBytes(t, 3<<20+12345) // 7 parts
	l.mem("a").files[act+"/20231234-Ana.py"] = []byte("print('hola')\n")
	l.mem("a").files[act+"/datos/grande.bin"] = big
	l.mem("a").files[act+"/notas.txt"] = []byte("no coincide con el patrón")
	l.mem("a").files["fuera.py"] = []byte("fuera de la carpeta")
	l.mem("b").files[act+"/20231235-Luis.py"] = []byte("x = 1\n")

	if err := l.col.Collect(l.ctx, id, l.students()); err != nil {
		t.Fatal(err)
	}
	l.wait()
	sum, _ = l.col.Get(id)
	if sum.Round != 1 || sum.Collecting {
		t.Fatalf("summary = round %d collecting %v", sum.Round, sum.Collecting)
	}
	for _, st := range sum.Students {
		if st.Status != StatusCollected {
			t.Fatalf("%s = %s %s", st.Session, st.Status, st.Reason)
		}
	}
	dirA := StudentDir(sum.Dir, Student{Hostname: "LAB-a", Username: "alumno-a"})
	if filepath.Base(dirA) != "LAB-a (alumno-a)" {
		t.Fatalf("student folder = %s", dirA)
	}
	got, err := os.ReadFile(filepath.Join(dirA, "datos", "grande.bin"))
	if err != nil || !bytes.Equal(got, big) {
		t.Fatalf("large file not whole: %d bytes, %v", len(got), err)
	}
	if b, _ := os.ReadFile(filepath.Join(dirA, "20231234-Ana.py")); string(b) != "print('hola')\n" {
		t.Fatalf("small file = %q", b)
	}
	if _, err := os.Stat(filepath.Join(dirA, "notas.txt")); err == nil {
		t.Fatal("a file outside the patterns was collected")
	}
	for _, r := range l.mem("a").reads {
		if !strings.HasPrefix(r, act+"/") {
			t.Fatalf("read outside the activity folder: %s", r)
		}
	}
	m, err := ReadManifest(dirA)
	if err != nil || m.Round != 1 || len(m.Files) != 2 || m.Hostname != "LAB-a" {
		t.Fatalf("manifest = %+v, %v", m, err)
	}
	if b, _ := os.ReadFile(filepath.Join(StudentDir(sum.Dir, Student{Hostname: "LAB-b", Username: "alumno-b"}), "20231235-Luis.py")); string(b) != "x = 1\n" {
		t.Fatalf("second student = %q", b)
	}
	// No staging left behind.
	entries, _ := os.ReadDir(sum.Dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".aula-") {
			t.Fatalf("staging left: %s", e.Name())
		}
	}
	// The student saw it before and after, and the window keeps what and when.
	l.mu.Lock()
	evs := l.events["a"]
	l.mu.Unlock()
	if len(evs) != 3 || evs[0] != EventOpened || evs[1] != EventRequested || evs[2] != EventCollected {
		t.Fatalf("student events = %v", evs)
	}
	cs := l.agents["a"].Collections()
	if len(cs) != 1 || cs[0].State != StateCollected || len(cs[0].Rounds) != 1 {
		t.Fatalf("window = %+v", cs)
	}
	rd := cs[0].Rounds[0]
	if len(rd.Files) != 2 || !rd.Files[0].Saved || !rd.Files[1].Saved || rd.At.IsZero() || rd.DoneAt.IsZero() {
		t.Fatalf("round = %+v", rd)
	}
}

func TestCollectSecondRoundReplacesTheTeachersCopy(t *testing.T) {
	l := newLab(t, "a")
	id, err := l.col.Open(l.ctx, "2026-11-06", "Ejercicio 4", nil, DefaultLimits, l.students())
	if err != nil {
		t.Fatal(err)
	}
	l.mem("a").files[act+"/v1.py"] = []byte("1")
	_ = l.col.Collect(l.ctx, id, l.students())
	l.wait()
	delete(l.mem("a").files, act+"/v1.py")
	l.mem("a").files[act+"/v2.py"] = []byte("2")
	_ = l.col.Collect(l.ctx, id, l.students())
	l.wait()
	sum, _ := l.col.Get(id)
	dir := sum.Students[0].Dir
	if _, err := os.Stat(filepath.Join(dir, "v1.py")); err == nil {
		t.Fatal("the previous round is still there")
	}
	if m, _ := ReadManifest(dir); m.Round != 2 || len(m.Files) != 1 || m.Files[0].Path != "v2.py" {
		t.Fatalf("manifest = %+v", m)
	}
	if _, ok := l.mem("a").files[act+"/v2.py"]; !ok {
		t.Fatal("the student's file must stay")
	}
}

func TestCollectSkipsAndLimits(t *testing.T) {
	l := newLab(t, "a")
	id, _ := l.col.Open(l.ctx, "2026-11-06", "Ejercicio 4", nil,
		Limits{MaxFileSize: 100, MaxTotal: 150, MaxFiles: 2}, l.students())
	m := l.mem("a")
	m.files[act+"/a.py"] = bytes.Repeat([]byte("a"), 80)
	m.files[act+"/b.py"] = bytes.Repeat([]byte("b"), 80) // passes max_total with a.py
	m.files[act+"/c.py"] = []byte("c")
	m.files[act+"/d.py"] = []byte("d") // a third file: too many
	m.files[act+"/e.mp4"] = bytes.Repeat([]byte("e"), 200)
	m.files[act+"/nube.py"] = []byte("n")
	m.offline[act+"/nube.py"] = true
	m.files[act+"/bz.py"] = []byte("z")
	m.denied[act+"/bz.py"] = true
	m.links[act+"/atajo"] = true
	_ = l.col.Collect(l.ctx, id, l.students())
	l.wait()
	sum, _ := l.col.Get(id)
	st := sum.Students[0]
	if st.Status != StatusCollected || len(st.Files) != 2 || st.Files[0].Path != "a.py" || st.Files[1].Path != "c.py" {
		t.Fatalf("collected = %s %+v", st.Status, st.Files)
	}
	reasons := map[string]string{}
	for _, s := range st.Skipped {
		reasons[s.Path] = s.Reason
	}
	want := map[string]string{"b.py": "too_large", "d.py": "too_many", "e.mp4": "too_large", "nube.py": "offline",
		"bz.py": "denied", "atajo": "link"}
	for p, r := range want {
		if reasons[p] != r {
			t.Errorf("skipped %s = %q, want %q (all: %v)", p, reasons[p], r, reasons)
		}
	}
	for _, r := range m.reads {
		if strings.HasSuffix(r, "nube.py") || strings.HasSuffix(r, "e.mp4") || strings.HasSuffix(r, "atajo") {
			t.Errorf("read what must be skipped: %s", r)
		}
	}
}

func TestCollectEmptyFolder(t *testing.T) {
	l := newLab(t, "a")
	id, _ := l.col.Open(l.ctx, "2026-11-06", "Ejercicio 4", nil, DefaultLimits, l.students())
	_ = l.col.Collect(l.ctx, id, l.students())
	l.wait()
	sum, _ := l.col.Get(id)
	if sum.Students[0].Status != StatusEmpty {
		t.Fatalf("status = %s", sum.Students[0].Status)
	}
	cs := l.agents["a"].Collections()
	if len(cs[0].Rounds) != 1 || len(cs[0].Rounds[0].Files) != 0 {
		t.Fatalf("window = %+v", cs[0].Rounds)
	}
}

func TestCollectCorruptFileIsRefused(t *testing.T) {
	l := newLab(t, "a")
	id, _ := l.col.Open(l.ctx, "2026-11-06", "Ejercicio 4", nil, DefaultLimits, l.students())
	l.mem("a").files[act+"/a.py"] = []byte("abc")
	l.tamper = func(p *protocol.FileCollectResponse) {
		if p.Part == 1 {
			p.Data = []byte("abd")
		}
	}
	_ = l.col.Collect(l.ctx, id, l.students())
	l.wait()
	sum, _ := l.col.Get(id)
	if st := sum.Students[0]; st.Status != StatusFailed || st.Reason != string(protocol.CollectCorrupt) {
		t.Fatalf("status = %s %s", st.Status, st.Reason)
	}
	rd := l.agents["a"].Collections()[0].Rounds[0]
	if len(rd.Files) != 1 || rd.Files[0].Saved || rd.Files[0].Reason != protocol.CollectCorrupt {
		t.Fatalf("window = %+v", rd.Files)
	}
	if _, err := os.Stat(sum.Students[0].Dir); err == nil {
		t.Fatal("a corrupt round must not be saved")
	}
}

func TestCollectNoAnswerTimesOut(t *testing.T) {
	l := newLab(t, "a")
	id, _ := l.col.Open(l.ctx, "2026-11-06", "Ejercicio 4", nil, DefaultLimits, l.students())
	l.col.ReplyWait = 50 * time.Millisecond
	l.drop = func(_ string, p protocol.Payload) bool { _, ok := p.(protocol.FileCollectAck); return ok }
	_ = l.col.Collect(l.ctx, id, l.students())
	l.wait()
	sum, _ := l.col.Get(id)
	if st := sum.Students[0]; st.Status != StatusFailed || st.Reason != ReasonTimeout {
		t.Fatalf("status = %s %s", st.Status, st.Reason)
	}
}

func TestCollectStudentLeavesMidway(t *testing.T) {
	l := newLab(t, "a")
	id, _ := l.col.Open(l.ctx, "2026-11-06", "Ejercicio 4", nil, DefaultLimits, l.students())
	l.mem("a").files[act+"/a.bin"] = randomBytes(t, 2<<20)
	l.drop = func(_ string, p protocol.Payload) bool {
		if r, ok := p.(protocol.FileCollectResponse); ok && r.Part == 2 {
			go l.col.Gone("a")
			return true
		}
		return false
	}
	_ = l.col.Collect(l.ctx, id, l.students())
	l.col.Wait()
	l.agents["a"].Reset()
	l.agents["a"].Wait()
	sum, _ := l.col.Get(id)
	if st := sum.Students[0]; st.Status != StatusFailed || st.Reason != ReasonGone {
		t.Fatalf("status = %s %s", st.Status, st.Reason)
	}
	entries, _ := os.ReadDir(sum.Dir)
	if len(entries) != 0 {
		t.Fatalf("half a round left on the teacher's disk: %v", entries)
	}
}

func TestResponderBusyAndUnavailable(t *testing.T) {
	r := &Responder{Reader: newMem()}
	block := make(chan struct{})
	sent := make(chan protocol.Payload, 10)
	send := func(ctx context.Context, p protocol.Payload) error {
		if _, ok := p.(protocol.FileCollectAck); ok {
			<-block
		}
		sent <- p
		return nil
	}
	req := protocol.FileCollectRequest{CollectID: protocol.NewID(), Date: "2026-11-06", Activity: "Ej", Round: 1,
		MaxFileSize: 10, MaxTotal: 10, MaxFiles: 1}
	if reply, _ := r.Handle(context.Background(), send, protocol.New(req)); reply != nil {
		t.Fatalf("first request answered at once: %+v", reply)
	}
	reply, _ := r.Handle(context.Background(), send, protocol.New(req))
	if a, ok := reply.(protocol.FileCollectAck); !ok || a.Status != protocol.FileCollectFailed || a.Reason != protocol.CollectBusy {
		t.Fatalf("second request = %+v", reply)
	}
	close(block)
	r.Wait()
	bad := req
	bad.Activity = "../fuera"
	reply, _ = r.Handle(context.Background(), send, protocol.New(bad))
	if a, ok := reply.(protocol.FileCollectAck); !ok || a.Reason != protocol.CollectInvalidPath {
		t.Fatalf("bad activity = %+v", reply)
	}
	none := &Responder{}
	if len(none.Capabilities()) != 0 {
		t.Fatal("no reader, no capability")
	}
	reply, _ = none.Handle(context.Background(), send, protocol.New(req))
	if e, ok := reply.(protocol.ErrorPayload); !ok || e.Code != protocol.CodeUnsupportedCapability {
		t.Fatalf("unavailable = %+v", reply)
	}
}

func TestPaths(t *testing.T) {
	if f, err := Folder("2026-11-06", "Ejercicio 4"); err != nil || f != act {
		t.Fatalf("Folder = %q, %v", f, err)
	}
	for _, a := range []string{"", "..", "a/b", `a\b`, "con", "x:y", "fin.", strings.Repeat("a", 81)} {
		if _, err := Folder("2026-11-06", a); !errors.Is(err, ErrInvalid) {
			t.Errorf("activity %q accepted", a)
		}
	}
	for _, d := range []string{"", "2026-02-30", "06-11-2026", "2026-11-6"} {
		if _, err := Folder(d, "a"); !errors.Is(err, ErrInvalid) {
			t.Errorf("date %q accepted", d)
		}
	}
	for _, p := range []string{"a.py", "src/a.py", "a/b/c/d.py"} {
		if err := CheckPath(p); err != nil {
			t.Errorf("CheckPath(%q) = %v", p, err)
		}
	}
	for _, p := range []string{"", "/a.py", "a/../b.py", "a//b.py", "a/b/c/d/e.py", `a\b.py`, "nul.txt"} {
		if err := CheckPath(p); err == nil {
			t.Errorf("CheckPath(%q) accepted", p)
		}
	}
	pats, err := ParsePatterns("*.py, *.PY;  main.*")
	if err != nil || len(pats) != 2 {
		t.Fatalf("ParsePatterns = %v, %v", pats, err)
	}
	if !Match(pats, "Main.java") || !Match(pats, "x.Py") || Match(pats, "x.txt") || !Match(nil, "x") {
		t.Fatal("Match")
	}
	if _, err := ParsePatterns("src/*.py"); err == nil {
		t.Fatal("a folder in a pattern must be refused")
	}
	if _, err := ParsePatterns("[a"); err == nil {
		t.Fatal("a bad pattern must be refused")
	}
}

// TestCollectManyStudentsInBatches: more students than Parallel, each with
// several files; every answer lands in its own folder.
func TestCollectManyStudentsInBatches(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e", "f", "g"}
	l := newLab(t, names...)
	id, err := l.col.Open(l.ctx, "2026-11-06", "Ejercicio 4", nil, DefaultLimits, l.students())
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		l.mem(n).files[act+"/main.py"] = []byte("# " + n)
		l.mem(n).files[act+"/lib/datos.csv"] = randomBytes(t, 700<<10) // 2 parts
	}
	if err := l.col.Collect(l.ctx, id, l.students()); err != nil {
		t.Fatal(err)
	}
	l.wait()
	sum, _ := l.col.Get(id)
	if len(sum.Students) != len(names) {
		t.Fatalf("%d students", len(sum.Students))
	}
	for _, st := range sum.Students {
		if st.Status != StatusCollected || len(st.Files) != 2 {
			t.Fatalf("%s = %s %s %d files", st.Session, st.Status, st.Reason, len(st.Files))
		}
		b, err := os.ReadFile(filepath.Join(st.Dir, "main.py"))
		if err != nil || string(b) != "# "+st.Session {
			t.Fatalf("%s main.py = %q, %v (answers mixed up)", st.Session, b, err)
		}
		csv, _ := os.ReadFile(filepath.Join(st.Dir, "lib", "datos.csv"))
		if !bytes.Equal(csv, l.mem(st.Session).files[act+"/lib/datos.csv"]) {
			t.Fatalf("%s datos.csv not whole", st.Session)
		}
	}
}

func TestCheckWorkPath(t *testing.T) {
	for _, ok := range []struct {
		rel  string
		file bool
	}{{act, false}, {act + "/a.py", true}, {act + "/src/x/y/a.py", true}} {
		if err := CheckWorkPath(ok.rel, ok.file); err != nil {
			t.Errorf("CheckWorkPath(%q) = %v", ok.rel, err)
		}
	}
	for _, bad := range []struct {
		rel  string
		file bool
	}{{"", false}, {"Ejercicio 4", false}, {"plantillas/2026-11-06/x", false}, {act + "/src", false},
		{act, true}, {act + "/../../../x.py", true}, {"recolecciones/2026-11-06/../x.py", true},
		{"recolecciones/x/Ej/a.py", true}, {act + "/a/b/c/d/e.py", true}} {
		if err := CheckWorkPath(bad.rel, bad.file); err == nil {
			t.Errorf("CheckWorkPath(%q, %v) accepted", bad.rel, bad.file)
		}
	}
}

func TestCopyTo(t *testing.T) {
	src := filepath.Join(t.TempDir(), "LAB-a (ana)")
	if err := os.MkdirAll(filepath.Join(src, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(src, "src", "a.py"), []byte("a"), 0o600)
	parent := t.TempDir()
	d1, err := CopyTo(src, parent)
	if err != nil || filepath.Base(d1) != "LAB-a (ana)" {
		t.Fatalf("CopyTo = %q, %v", d1, err)
	}
	d2, err := CopyTo(src, parent)
	if err != nil || filepath.Base(d2) != "LAB-a (ana) (2)" {
		t.Fatalf("second CopyTo = %q, %v", d2, err)
	}
	if b, _ := os.ReadFile(filepath.Join(d2, "src", "a.py")); string(b) != "a" {
		t.Fatalf("copy = %q", b)
	}
	if _, err := CopyTo(filepath.Join(parent, "no"), parent); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing = %v", err)
	}
}

func TestResponderForget(t *testing.T) {
	r := &Responder{Reader: newMem()}
	open := protocol.FileCollectOpen{CollectID: protocol.NewID(), Date: "2026-11-06", Activity: "Ej"}
	if reply, _ := r.Handle(context.Background(), func(context.Context, protocol.Payload) error { return nil }, protocol.New(open)); reply == nil {
		t.Fatal("open not answered")
	}
	if n := r.Forget(); n != 1 || len(r.Collections()) != 0 {
		t.Fatalf("Forget = %d, left %v", n, r.Collections())
	}
}
