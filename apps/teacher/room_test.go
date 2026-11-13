package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/roomplan"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/simclass"
)

// No test writes in the teacher's real Documents: the room plan saves on
// every hello. A test that needs the folder sets App.documents.
func init() {
	documentsDir = func() (string, error) { return "", os.ErrNotExist }
}

const pc7MAC = "3c:52:82:1a:9f:07"

func hasCode(err error, code string) bool {
	return err != nil && strings.HasPrefix(err.Error(), "aula:"+code+":")
}

// labelsSent returns the computer.label messages sent, by session.
func labelsSent(h *harness) map[string]string {
	out := map[string]string{}
	for _, d := range h.class.takeDirect() {
		if p, ok := d.p.(protocol.ComputerLabel); ok {
			out[d.sessionID] = p.Label
		}
	}
	return out
}

func withDocs(t *testing.T, h *harness) string {
	t.Helper()
	docs := t.TempDir()
	h.app.documents = func() (string, error) { return docs, nil }
	if _, err := h.app.StartClass("1", "Prof. Ayala"); err != nil {
		t.Fatal(err)
	}
	return docs
}

// The label replaces nothing on the agent but its own line, and reaches only
// the agents that declared computer.label: after hello and on every change.
func TestComputerLabelReachesTheAgent(t *testing.T) {
	h := newHarness(t,
		student("s1", "LAB3-PC07", protocol.CapComputerLabel),
		student("s2", "LAB3-PC08"))
	h.hooks.onChange()
	if got := labelsSent(h); len(got) != 0 {
		t.Errorf("no label yet, nothing to send: %v", got)
	}
	v := h.app.Room()
	if len(v.Computers) != 2 || v.Computers[0].Seat != nil {
		t.Fatalf("new computers go to «Sin acomodar»: %+v", v.Computers)
	}

	if _, err := h.app.SetComputerLabel("lab3-pc07", false, " Fila 2 · PC07 "); err != nil {
		t.Fatal(err)
	}
	if got := labelsSent(h); len(got) != 1 || got["s1"] != "Fila 2 · PC07" {
		t.Errorf("label to the agent that declared it: %v", got)
	}
	if ev, ok := h.rec.last(EventRoom).(RoomView); !ok || ev.Computers[0].Label != "Fila 2 · PC07" {
		t.Errorf("aula:room after the change: %+v", h.rec.last(EventRoom))
	}
	if _, err := h.app.SetComputerLabel("LAB3-PC08", false, "Junto a la puerta"); err != nil {
		t.Fatal(err)
	}
	if got := labelsSent(h); len(got) != 0 {
		t.Errorf("never to an agent without computer.label: %v", got)
	}

	// A new session of the same computer gets it after its hello.
	h.class.mu.Lock()
	h.class.sessions = []session.Session{student("s3", "LAB3-PC07", protocol.CapComputerLabel)}
	h.class.mu.Unlock()
	h.hooks.onChange()
	if got := labelsSent(h); got["s3"] != "Fila 2 · PC07" {
		t.Errorf("label after hello: %v", got)
	}
	h.hooks.onChange()
	if got := labelsSent(h); len(got) != 0 {
		t.Errorf("only once per session: %v", got)
	}

	// Removing it tells the agent with "".
	if _, err := h.app.SetComputerLabel("LAB3-PC07", false, ""); err != nil {
		t.Fatal(err)
	}
	if got := labelsSent(h); got["s3"] != "" || len(got) != 1 {
		t.Errorf("label removed: %v", got)
	}
}

func TestRoomErrorsAreCoded(t *testing.T) {
	h := newHarness(t, student("s1", "LAB3-PC07"))
	h.hooks.onChange()
	if _, err := h.app.SetComputerLabel("LAB3-PC07", false, strings.Repeat("a", 41)); !hasCode(err, codeLabelInvalid) {
		t.Errorf("41 characters: %v", err)
	}
	if _, err := h.app.SetComputerLabel("LAB3-PC07", false, "Fila\n2"); !hasCode(err, codeLabelInvalid) {
		t.Errorf("line break: %v", err)
	}
	if _, err := h.app.SetComputerLabel("NADIE", false, "x"); !hasCode(err, codeRoomComputerGone) {
		t.Errorf("unknown computer: %v", err)
	}
	if _, err := h.app.SetRoomPlace(1, 1, "teacher"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.app.MoveComputer("LAB3-PC07", false, 1, 1); !hasCode(err, codeRoomSpecialPlace) {
		t.Errorf("onto the teacher's desk: %v", err)
	}
	if _, err := h.app.ResizeRoom(13, 4); !hasCode(err, codeRoomInvalid) {
		t.Errorf("13 rows: %v", err)
	}
	if _, err := h.app.SetRoomPlace(1, 2, "door"); !hasCode(err, codeRoomInvalid) {
		t.Errorf("unknown place: %v", err)
	}
	if v, err := h.app.MoveComputer("LAB3-PC07", false, 2, 3); err != nil || *v.Computers[0].Seat != (roomplan.Cell{Row: 2, Col: 3}) {
		t.Errorf("moved: %v %+v", err, v.Computers)
	}
	if v, err := h.app.ForgetRoomComputer("LAB3-PC07"); err != nil || len(v.Computers) != 0 {
		t.Errorf("forgotten: %v %+v", err, v.Computers)
	}
}

// The plan is saved per channel in Documentos\Aula\salones and survives a
// restart of the console.
func TestRoomIsSavedPerChannel(t *testing.T) {
	h := newHarness(t, student("s1", "LAB3-PC07"))
	docs := withDocs(t, h)
	h.hooks.onChange()
	if _, err := h.app.MoveComputer("LAB3-PC07", false, 2, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := h.app.SetComputerLabel("LAB3-PC07", false, "Fila 2 · PC07"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(docs, platform.WorkFolderName, roomplan.Folder, "canal-1.json")
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("plan file: %v", err)
	}
	if _, err := h.app.StartClass("2", "Prof. Ayala"); err != nil {
		t.Fatal(err)
	}
	if v := h.app.Room(); v.Channel != "2" || len(v.Computers) != 0 {
		t.Errorf("another channel, another room: %+v", v)
	}
	if _, err := h.app.StartClass("1", "Prof. Ayala"); err != nil {
		t.Fatal(err)
	}
	v := h.app.Room()
	if len(v.Computers) != 1 || v.Computers[0].Label != "Fila 2 · PC07" || v.Computers[0].Seat == nil {
		t.Errorf("room kept: %+v", v.Computers)
	}
	if v.File != file {
		t.Errorf("file %q, want %q", v.File, file)
	}
	data, _ := os.ReadFile(file)
	for _, never := range []string{"username", "alumno"} {
		if strings.Contains(strings.ToLower(string(data)), never) {
			t.Errorf("the plan keeps computers, never students (%q):\n%s", never, data)
		}
	}
}

// A computer that comes back with another hostname and the same MAC keeps its
// seat and label, and the teacher is told.
func TestRenamedComputerKeepsItsSeat(t *testing.T) {
	pc := student("s1", "LAB3-PC07", protocol.CapComputerLabel)
	pc.MACs = []string{pc7MAC}
	h := newHarness(t, pc)
	h.hooks.onChange()
	_, _ = h.app.MoveComputer("LAB3-PC07", false, 2, 3)
	_, _ = h.app.SetComputerLabel("LAB3-PC07", false, "Fila 2 · PC07")
	_ = labelsSent(h)

	again := student("s2", "LAB3-07", protocol.CapComputerLabel)
	again.MACs = []string{pc7MAC}
	h.class.mu.Lock()
	h.class.sessions = []session.Session{again}
	h.class.mu.Unlock()
	h.hooks.onChange()
	ev, _ := h.rec.last(EventRoom).(RoomView)
	if len(ev.Renamed) != 1 || ev.Renamed[0] != (RoomRename{From: "LAB3-PC07", To: "LAB3-07"}) {
		t.Errorf("rename notice: %+v", ev.Renamed)
	}
	if len(ev.Computers) != 1 || ev.Computers[0].Hostname != "LAB3-07" || ev.Computers[0].Label != "Fila 2 · PC07" {
		t.Errorf("kept seat and label: %+v", ev.Computers)
	}
	if got := labelsSent(h); got["s2"] != "Fila 2 · PC07" {
		t.Errorf("the renamed computer gets its label: %v", got)
	}
	if ev := h.app.Room(); len(ev.Renamed) != 0 {
		t.Error("the notice is only for the emission where it happened")
	}
}

func TestExportAndImportTheRoom(t *testing.T) {
	h := newHarness(t, student("s1", "LAB3-PC07", protocol.CapComputerLabel))
	withDocs(t, h)
	h.hooks.onChange()
	_, _ = h.app.SetRoomPlace(1, 1, "teacher")
	_, _ = h.app.MoveComputer("LAB3-PC07", false, 2, 3)
	_, _ = h.app.SetComputerLabel("LAB3-PC07", false, "Fila 2 · PC07")
	_ = labelsSent(h)

	out := filepath.Join(t.TempDir(), "salon.json")
	var askedName string
	h.app.chooseSave = func(_, name string) (string, error) { askedName = name; return out, nil }
	if path, err := h.app.ExportRoom(); err != nil || path != out {
		t.Fatalf("export: %q %v", path, err)
	}
	if askedName != "salon-canal-1.json" {
		t.Errorf("suggested name %q", askedName)
	}
	h.app.chooseSave = func(string, string) (string, error) { return "", nil }
	if path, err := h.app.ExportRoom(); err != nil || path != "" {
		t.Errorf("cancelled export: %q %v", path, err)
	}

	// Another console, on channel 2: the file is of channel 1.
	if _, err := h.app.StartClass("2", "Prof. Ayala"); err != nil {
		t.Fatal(err)
	}
	h.hooks.onChange()
	h.app.chooseOpen = func(string) (string, error) { return out, nil }
	pre, err := h.app.PreviewRoomImport()
	if err != nil || !pre.OtherChannel || pre.Channel != "1" || pre.Computers != 1 || pre.Labeled != 1 || pre.Seated != 1 {
		t.Fatalf("preview: %+v %v", pre, err)
	}
	if _, err := h.app.ImportRoom(out, false); !hasCode(err, codeRoomOtherChannel) {
		t.Errorf("another channel without asking: %v", err)
	}
	v, err := h.app.ImportRoom(out, true)
	if err != nil || v.Channel != "2" || v.Computers[0].Label != "Fila 2 · PC07" || len(v.Places) != 1 {
		t.Fatalf("import: %+v %v", v, err)
	}
	if got := labelsSent(h); got["s1"] != "Fila 2 · PC07" {
		t.Errorf("import tells the agents: %v", got)
	}

	h.app.chooseOpen = func(string) (string, error) { return "", nil }
	if pre, err := h.app.PreviewRoomImport(); err != nil || pre.Path != "" {
		t.Errorf("cancelled import: %+v %v", pre, err)
	}
	bad := filepath.Join(t.TempDir(), "roto.json")
	_ = os.WriteFile(bad, []byte(`{"version":1,"channel":"2","rows":20,"cols":1}`), 0o644)
	h.app.chooseOpen = func(string) (string, error) { return bad, nil }
	if _, err := h.app.PreviewRoomImport(); !hasCode(err, codeRoomInvalid) {
		t.Errorf("invalid file: %v", err)
	}
	if _, err := h.app.ImportRoom(bad, true); !hasCode(err, codeRoomInvalid) {
		t.Errorf("invalid import: %v", err)
	}
	if h.app.Room().Computers[0].Label != "Fila 2 · PC07" {
		t.Error("a refused import leaves the room as it was")
	}
}

// simClass is a class that also hosts simulated students.
type simClass struct {
	*fakeClass
	sims []session.Session
}

func (c *simClass) StartSimulation() (simclass.Console, error) {
	return simclass.Console{}, errors.New("not in this test")
}
func (c *simClass) StopSimulation()                {}
func (c *simClass) SimSessions() []session.Session { return c.sims }
func (c *simClass) SimBroadcast(context.Context, protocol.Payload) (map[string]string, map[string]error) {
	return map[string]string{}, map[string]error{}
}

// Simulated students take the free seats in memory, get their label, and
// never reach the plan file.
func TestSimulatedStudentsTakeSeatsInMemory(t *testing.T) {
	h := newHarness(t, student("s1", "LAB3-PC07"))
	docs := t.TempDir()
	h.app.documents = func() (string, error) { return docs, nil }
	sc := &simClass{fakeClass: h.class}
	for _, n := range []string{"Simulado 1", "Simulado 2"} {
		sc.sims = append(sc.sims, student("sim-"+n, n, protocol.CapComputerLabel))
	}
	h.app.start = func(_ context.Context, cfg classConfig, hooks classHooks, _ *slog.Logger) (classroom, ClassInfo, error) {
		h.hooks = hooks
		return sc, ClassInfo{Channel: cfg.Channel}, nil
	}
	if _, err := h.app.StartClass("1", "Prof. Ayala"); err != nil {
		t.Fatal(err)
	}
	h.hooks.onChange()
	v := h.app.Room()
	var seated int
	for _, c := range v.Computers {
		if c.Simulated && c.Seat != nil {
			seated++
		}
	}
	if seated != 2 {
		t.Fatalf("sims seated: %+v", v.Computers)
	}
	if _, err := h.app.SetComputerLabel("Simulado 1", true, "Prueba 1"); err != nil {
		t.Fatal(err)
	}
	if got := labelsSent(h); got["sim-Simulado 1"] != "Prueba 1" {
		t.Errorf("label to the simulated student: %v", got)
	}
	_, _ = h.app.MoveComputer("LAB3-PC07", false, 3, 3) // saves the file
	data, _ := os.ReadFile(filepath.Join(docs, platform.WorkFolderName, roomplan.Folder, "canal-1.json"))
	if len(data) == 0 || strings.Contains(string(data), "Simulado") || strings.Contains(string(data), "Prueba") {
		t.Errorf("simulated students never reach the file:\n%s", data)
	}
}
