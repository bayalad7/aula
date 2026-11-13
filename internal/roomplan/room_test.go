package roomplan

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const mac7 = "3c:52:82:1a:9f:07"

func cell(r, c int) *Cell { return &Cell{Row: r, Col: c} }

func openTemp(t *testing.T) (*Room, string) {
	t.Helper()
	dir := t.TempDir()
	r, err := Open(dir, "1")
	if err != nil {
		t.Fatal(err)
	}
	return r, dir
}

func mustSeen(t *testing.T, r *Room, host string, macs ...string) Arrival {
	t.Helper()
	a, err := r.Seen(host, macs, nil)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestValidate(t *testing.T) {
	good := func() Plan {
		p := New("1")
		p.Places = []Place{{Cell: Cell{1, 1}, Kind: KindTeacher}}
		p.Computers = []Computer{{Hostname: "LAB3-PC07", MACs: []string{mac7}, Label: "Fila 2 · PC07", Seat: cell(2, 3)}}
		return p
	}
	if err := good().Validate(); err != nil {
		t.Fatalf("good plan: %v", err)
	}
	for name, tc := range map[string]struct {
		mod  func(p *Plan)
		want error
	}{
		"newer version":      {func(p *Plan) { p.Version = 2 }, ErrNewer},
		"no version":         {func(p *Plan) { p.Version = 0 }, ErrInvalid},
		"bad channel":        {func(p *Plan) { p.Channel = "16001" }, ErrInvalid},
		"too many rows":      {func(p *Plan) { p.Rows = 13 }, ErrInvalid},
		"no columns":         {func(p *Plan) { p.Cols = 0 }, ErrInvalid},
		"repeated place":     {func(p *Plan) { p.Places = append(p.Places, Place{Cell: Cell{1, 1}, Kind: KindAisle}) }, ErrInvalid},
		"place outside":      {func(p *Plan) { p.Places = append(p.Places, Place{Cell: Cell{6, 1}, Kind: KindAisle}) }, ErrInvalid},
		"unknown kind":       {func(p *Plan) { p.Places[0].Kind = "door" }, ErrInvalid},
		"repeated hostname":  {func(p *Plan) { p.Computers = append(p.Computers, Computer{Hostname: "lab3-pc07"}) }, ErrInvalid},
		"two in a seat":      {func(p *Plan) { p.Computers = append(p.Computers, Computer{Hostname: "PC08", Seat: cell(2, 3)}) }, ErrInvalid},
		"on a special place": {func(p *Plan) { p.Computers[0].Seat = cell(1, 1) }, ErrInvalid},
		"seat outside":       {func(p *Plan) { p.Computers[0].Seat = cell(2, 9) }, ErrInvalid},
		"long label":         {func(p *Plan) { p.Computers[0].Label = strings.Repeat("a", 41) }, ErrInvalid},
		"control in label":   {func(p *Plan) { p.Computers[0].Label = "Fila\n2" }, ErrInvalid},
		"bad MAC":            {func(p *Plan) { p.Computers[0].MACs = []string{"3c:52"} }, ErrInvalid},
		"empty hostname":     {func(p *Plan) { p.Computers[0].Hostname = " " }, ErrInvalid},
	} {
		p := good()
		tc.mod(&p)
		if err := p.Validate(); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", name, err, tc.want)
		}
	}
}

func TestSaveAndReopen(t *testing.T) {
	r, dir := openTemp(t)
	mustSeen(t, r, "LAB3-PC07", mac7)
	mustSeen(t, r, "LAB3-PC08")
	if err := r.Resize(6, 10); err != nil {
		t.Fatal(err)
	}
	if err := r.SetPlace(Cell{1, 1}, KindTeacher); err != nil {
		t.Fatal(err)
	}
	if err := r.Move("lab3-pc07", false, cell(2, 3)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetLabel("LAB3-PC07", false, "  Fila 2 · PC07 "); err != nil {
		t.Fatal(err)
	}
	again, err := Open(dir, "1")
	if err != nil {
		t.Fatal(err)
	}
	v := again.View()
	if v.Rows != 6 || v.Cols != 10 || len(v.Places) != 1 || len(v.Computers) != 2 {
		t.Fatalf("reopened: %+v", v)
	}
	if c := v.Computers[0]; c.Hostname != "LAB3-PC07" || c.Label != "Fila 2 · PC07" || *c.Seat != (Cell{2, 3}) {
		t.Errorf("seated first, label trimmed: %+v", c)
	}
	if c := v.Computers[1]; c.Seat != nil {
		t.Errorf("new computer goes to «Sin acomodar»: %+v", c)
	}
	if got := again.Label("lab3-PC07", false); got != "Fila 2 · PC07" {
		t.Errorf("label without case: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "canal-1.json")); err != nil {
		t.Errorf("file: %v", err)
	}
}

func TestMoveRules(t *testing.T) {
	r, _ := openTemp(t)
	mustSeen(t, r, "A")
	mustSeen(t, r, "B")
	_ = r.Move("A", false, cell(1, 1))
	_ = r.Move("B", false, cell(1, 2))
	if err := r.Move("A", false, cell(1, 2)); err != nil {
		t.Fatal(err)
	}
	if v := r.View(); v.Computers[0].Hostname != "B" || *v.Computers[1].Seat != (Cell{1, 2}) {
		t.Errorf("swap: %+v", v.Computers)
	}
	if err := r.SetPlace(Cell{1, 2}, KindAisle); err != nil {
		t.Fatal(err)
	}
	if r.View().Computers[1].Seat != nil {
		t.Error("a place sends the computer under it to «Sin acomodar»")
	}
	if err := r.Move("B", false, cell(1, 2)); !errors.Is(err, ErrSpecialPlace) {
		t.Errorf("onto a special place: %v", err)
	}
	if err := r.Move("Z", false, cell(2, 2)); !errors.Is(err, ErrUnknown) {
		t.Errorf("unknown computer: %v", err)
	}
	if _, err := r.SetLabel("Z", false, "x"); !errors.Is(err, ErrUnknown) {
		t.Errorf("label of an unknown computer: %v", err)
	}
	if err := r.Move("B", false, cell(9, 9)); !errors.Is(err, ErrInvalid) {
		t.Errorf("outside: %v", err)
	}
	_ = r.Move("B", false, cell(5, 8))
	if err := r.Resize(4, 8); err != nil {
		t.Fatal(err)
	}
	for _, c := range r.View().Computers {
		if c.Hostname == "B" && c.Seat != nil {
			t.Error("a smaller room sends who is left out to «Sin acomodar»")
		}
	}
	if err := r.Resize(13, 1); !errors.Is(err, ErrInvalid) {
		t.Errorf("13 rows: %v", err)
	}
	if err := r.Forget("B"); err != nil || len(r.View().Computers) != 1 {
		t.Errorf("forget: %v %+v", err, r.View().Computers)
	}
}

func TestRenamedByMAC(t *testing.T) {
	r, _ := openTemp(t)
	mustSeen(t, r, "LAB3-PC07", mac7)
	_ = r.Move("LAB3-PC07", false, cell(2, 3))
	_, _ = r.SetLabel("LAB3-PC07", false, "Fila 2 · PC07")

	// Connected: never taken over by another hostname with the same MAC.
	a, _ := r.Seen("CLON", []string{mac7}, func(string) bool { return true })
	if !a.Added || a.RenamedFrom != "" {
		t.Errorf("connected computer must keep its name: %+v", a)
	}
	_ = r.Forget("CLON")

	a, err := r.Seen("LAB3-07", []string{strings.ToUpper(mac7)}, func(string) bool { return false })
	if err != nil || a.RenamedFrom != "LAB3-PC07" || a.Added {
		t.Fatalf("renamed: %+v %v", a, err)
	}
	v := r.View()
	if len(v.Computers) != 1 || v.Computers[0].Hostname != "LAB3-07" || v.Computers[0].Label != "Fila 2 · PC07" || v.Computers[0].Seat == nil {
		t.Errorf("kept seat and label: %+v", v.Computers)
	}

	// The same MAC on two computers: nobody is renamed.
	if a, _ := r.Seen("OTRO", []string{mac7}, func(string) bool { return true }); !a.Added {
		t.Fatalf("second computer with the MAC: %+v", a)
	}
	a, _ = r.Seen("TERCERO", []string{mac7}, nil)
	if a.RenamedFrom != "" || !a.Added {
		t.Errorf("ambiguous MAC: %+v", a)
	}

	// A hello without MACs keeps the known ones; a known one is no change.
	if a := mustSeen(t, r, "LAB3-07"); a.Changed {
		t.Errorf("same hello again: %+v", a)
	}
	if got := r.Plan().Computers[0].MACs; len(got) != 1 || got[0] != mac7 {
		t.Errorf("MACs kept, lowercase: %v", got)
	}
}

func TestKnowNeverRenames(t *testing.T) {
	r, _ := openTemp(t)
	mustSeen(t, r, "LAB3-07", mac7)
	if added, _ := r.Know("LAB3-PC07", []string{mac7}); added {
		t.Error("a remembered computer with a MAC of the plan is not added")
	}
	if added, _ := r.Know("LAB3-PC09", nil); !added {
		t.Error("a remembered computer goes to «Sin acomodar»")
	}
	if added, _ := r.Know("lab3-pc09", nil); added {
		t.Error("twice")
	}
}

func TestSimulatedNeverSaved(t *testing.T) {
	r, dir := openTemp(t)
	mustSeen(t, r, "REAL")
	_ = r.Move("REAL", false, cell(1, 1))
	_ = r.SetPlace(Cell{1, 2}, KindTeacher)
	if !r.SyncSims([]string{"Alumno 1", "Alumno 2"}) {
		t.Fatal("sims added")
	}
	v := r.View()
	var seats []Cell
	for _, c := range v.Computers {
		if c.Simulated {
			seats = append(seats, *c.Seat)
		}
	}
	if len(seats) != 2 || seats[0] != (Cell{1, 3}) || seats[1] != (Cell{1, 4}) {
		t.Errorf("sims take the first free seats: %v", seats)
	}
	if _, err := r.SetLabel("Alumno 1", true, "Simulado A"); err != nil {
		t.Fatal(err)
	}
	if err := r.Move("REAL", false, cell(1, 3)); err != nil {
		t.Fatal(err)
	}
	if got := r.View(); got.Computers[0].Hostname != "REAL" {
		t.Errorf("a real computer takes a sim's seat: %+v", got.Computers)
	}
	if err := r.Move("Alumno 2", true, cell(1, 3)); !errors.Is(err, ErrInvalid) {
		t.Errorf("a sim never takes a real seat: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "canal-1.json"))
	if strings.Contains(string(data), "Alumno") || strings.Contains(string(data), "Simulado") {
		t.Errorf("simulated students reached the file:\n%s", data)
	}
	r.SyncSims(nil)
	for _, c := range r.View().Computers {
		if c.Simulated {
			t.Errorf("sims forgotten: %+v", c)
		}
	}
}

func TestExportImport(t *testing.T) {
	src, _ := openTemp(t)
	mustSeen(t, src, "LAB3-PC07", mac7)
	_ = src.Move("LAB3-PC07", false, cell(2, 3))
	_, _ = src.SetLabel("LAB3-PC07", false, "Fila 2 · PC07")
	_ = src.SetPlace(Cell{1, 1}, KindTeacher)
	src.SyncSims([]string{"Alumno 1"})
	data, err := src.Export()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Alumno 1") {
		t.Error("export never carries simulated students")
	}

	dst, dir := openTemp(t)
	mustSeen(t, dst, "VIEJO")
	if err := dst.Import(data, false); err != nil {
		t.Fatal(err)
	}
	v := dst.View()
	if len(v.Computers) != 1 || v.Computers[0].Label != "Fila 2 · PC07" || len(v.Places) != 1 {
		t.Errorf("imported: %+v", v)
	}
	again, _ := Open(dir, "1")
	if again.Label("LAB3-PC07", false) != "Fila 2 · PC07" {
		t.Error("import is saved")
	}

	other, _ := Open(t.TempDir(), "2")
	if err := other.Import(data, false); !errors.Is(err, ErrOtherChannel) {
		t.Errorf("another channel: %v", err)
	}
	if err := other.Import(data, true); err != nil || other.View().Channel != "2" {
		t.Errorf("another channel, accepted: %v %s", err, other.View().Channel)
	}
	if err := other.Import([]byte(`{"version":1,"channel":"2","rows":1,"cols":1,"computers":[{"hostname":"A","seat":{"row":1,"col":1}},{"hostname":"B","seat":{"row":1,"col":1}}]}`), false); !errors.Is(err, ErrInvalid) {
		t.Errorf("repeated seat: %v", err)
	}
	if other.Label("LAB3-PC07", false) != "Fila 2 · PC07" {
		t.Error("a refused import leaves the plan as it was")
	}
	if err := other.Import([]byte(`no es json`), false); !errors.Is(err, ErrInvalid) {
		t.Errorf("not JSON: %v", err)
	}
}

func TestDamagedAndNewerFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName("1"))
	_ = os.WriteFile(path, []byte("{roto"), 0o644)
	r, err := Open(dir, "1")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("damaged: %v", err)
	}
	v := r.View()
	if v.Problem != ProblemDamaged || v.Backup == "" {
		t.Fatalf("damaged view: %+v", v)
	}
	if data, _ := os.ReadFile(v.Backup); string(data) != "{roto" {
		t.Errorf("backup keeps the damaged file: %q", data)
	}
	mustSeen(t, r, "A") // the room works and saves a new file

	_ = os.WriteFile(path, []byte(`{"version":9,"channel":"1","rows":1,"cols":1}`), 0o644)
	r, err = Open(dir, "1")
	if !errors.Is(err, ErrNewer) || r.View().Problem != ProblemNewer {
		t.Fatalf("newer: %v %+v", err, r.View())
	}
	mustSeen(t, r, "B")
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), `"version":9`) {
		t.Errorf("a newer file is never written over: %s", data)
	}

	_ = os.WriteFile(path, []byte(`{"version":1,"channel":"7","rows":1,"cols":1}`), 0o644)
	if _, err := Open(dir, "1"); !errors.Is(err, ErrInvalid) {
		t.Errorf("a file of another channel under this name: %v", err)
	}
}

func TestMemoryOnly(t *testing.T) {
	r, err := Open("", "1")
	if err != nil {
		t.Fatal(err)
	}
	mustSeen(t, r, "A")
	if v := r.View(); v.File != "" || len(v.Computers) != 1 {
		t.Errorf("memory: %+v", v)
	}
}
