package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/collect"
)

// The whole path with the aula simulada (docs/protocol.md §5.21): open a
// collection, every simulated student gets the folder, «Recoger» brings back
// each one's last code, saved per student in the teacher's Documentos\Aula\
// recolecciones\<fecha>\<actividad>\, apart as «(simulado)». Nothing of it
// reaches the log.
func TestCollectFromSimulatedStudents(t *testing.T) {
	logs := &lockedBuffer{}
	rec := &recorder{}
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	app.port = 0
	app.emit = rec.emit
	docs := t.TempDir()
	app.documents = func() (string, error) { return docs, nil }
	var opened []string
	app.openFolder = func(p string) error { opened = append(opened, p); return nil }
	if _, err := app.StartClass("7", "Prueba"); err != nil {
		t.Fatal(err)
	}
	defer app.EndClass()
	if _, err := app.SimulationStart(0); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "2 simulated students", func() bool { return app.View().Students.Simulated == 2 })
	if st := app.View().Students; st.CanCollect != 2 {
		t.Fatalf("canCollect = %d", st.CanCollect)
	}
	d, err := app.Emit(EmitRequest{Title: "Factorial", Lang: "python", Content: pySnippet, Mode: "replace"})
	if err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the code on their screens", func() bool { return deliveryOf(app, d.ID).SimAcked == 2 })

	sum, err := app.OpenCollection(CollectionRequest{Activity: "Ejercicio secreto", Patterns: "*.py"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Patterns) != 1 || sum.Patterns[0] != "*.py" {
		t.Fatalf("patterns = %v", sum.Patterns)
	}
	waitUntil(t, "both folders ready", func() bool {
		s, _ := app.collector.Get(sum.ID)
		n := 0
		for _, st := range s.Students {
			if st.Status == collect.StatusReady {
				n++
			}
		}
		return n == 2
	})
	if _, err := app.Collect(sum.ID, nil); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "both collected", func() bool {
		s, _ := app.collector.Get(sum.ID)
		n := 0
		for _, st := range s.Students {
			if st.Status == collect.StatusCollected {
				n++
			}
		}
		return n == 2 && !s.Collecting
	})
	s, _ := app.collector.Get(sum.ID)
	act := filepath.Join(docs, "Aula", "recolecciones", s.Date, "Ejercicio secreto")
	if s.Dir != act {
		t.Fatalf("activity folder = %s, want %s", s.Dir, act)
	}
	for _, st := range s.Students {
		if !strings.HasSuffix(st.Dir, "(simulado)") || filepath.Dir(st.Dir) != act {
			t.Errorf("student folder = %s", st.Dir)
		}
		if len(st.Files) != 1 || !strings.HasSuffix(st.Files[0].Path, ".py") {
			t.Fatalf("files = %+v", st.Files)
		}
		b, err := os.ReadFile(filepath.Join(st.Dir, filepath.FromSlash(st.Files[0].Path)))
		if err != nil || string(b) != pySnippet {
			t.Fatalf("collected = %q, %v", b, err)
		}
		if m, err := collect.ReadManifest(st.Dir); err != nil || m.Round != 1 || !m.Simulated {
			t.Fatalf("manifest = %+v, %v", m, err)
		}
	}
	if v, ok := rec.last(EventCollections).(CollectionsView); !ok || len(v.List) != 1 {
		t.Fatalf("no aula:collections event: %+v", rec.last(EventCollections))
	}

	// «Abrir» and «Guardar como…» per student; nothing opens on the desktop.
	key := s.Students[0].Key
	if err := app.OpenCollectionFolder(sum.ID, key); err != nil || opened[len(opened)-1] != s.Students[0].Dir {
		t.Fatalf("open = %v, %v", err, opened)
	}
	if err := app.OpenCollectionFolder("", ""); err != nil || opened[len(opened)-1] != filepath.Join(docs, "Aula", "recolecciones") {
		t.Fatalf("open the root = %v, %v", err, opened)
	}
	dest := t.TempDir()
	app.chooseFolder = func(string) (string, error) { return dest, nil }
	if got, err := app.SaveCollectionAs(sum.ID, key, "Guardar"); err != nil || filepath.Dir(got) != dest {
		t.Fatalf("save as = %q, %v", got, err)
	}
	app.chooseFolder = func(string) (string, error) { return "", nil }
	if got, err := app.SaveCollectionAs(sum.ID, key, "Guardar"); err != nil || got != "" {
		t.Fatalf("cancelled save as = %q, %v", got, err)
	}

	for _, secret := range []string{"secreto", ".py", "def f"} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("the log has %q", secret)
		}
	}

	// Closing the class forgets the collections; the folders stay.
	app.EndClass()
	if len(app.Collections().List) != 0 {
		t.Error("the collections outlived the class")
	}
	if _, err := os.Stat(act); err != nil {
		t.Errorf("the folder went with the class: %v", err)
	}
}

func TestCollectRefuses(t *testing.T) {
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	if _, err := app.OpenCollection(CollectionRequest{Activity: "Ej"}); errorCode(err) != codeNoClass {
		t.Fatalf("without a class = %v", err)
	}
	h := newHarness(t, student("s1", "PC01"))
	h.app.documents = func() (string, error) { return t.TempDir(), nil }
	for _, bad := range []CollectionRequest{{Activity: ""}, {Activity: "../fuera"}, {Activity: "con"}, {Activity: "a:b"},
		{Activity: "Ej", Patterns: "src/*.py"}} {
		if _, err := h.app.OpenCollection(bad); errorCode(err) != codeCollectActivity {
			t.Errorf("OpenCollection(%+v) = %v", bad, err)
		}
	}
	sum, err := h.app.OpenCollection(CollectionRequest{Activity: "Ej"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Students) != 0 {
		t.Fatalf("an agent without file.collect got the folder: %+v", sum.Students)
	}
	if _, err := h.app.Collect(sum.ID, nil); errorCode(err) != codeCollectNoStudents {
		t.Fatalf("Collect without who can = %v", err)
	}
	if _, err := h.app.Collect("nope", nil); errorCode(err) != codeCollectGone {
		t.Fatalf("Collect unknown = %v", err)
	}
	if _, err := h.app.SaveCollectionAs(sum.ID, "nadie", "x"); errorCode(err) != codeCollectGone {
		t.Fatalf("SaveCollectionAs unknown = %v", err)
	}
	h.app.documents = func() (string, error) { return "", os.ErrNotExist }
	if _, err := h.app.OpenCollection(CollectionRequest{Activity: "Otra"}); errorCode(err) != codeCollectFolder {
		t.Fatalf("without Documents = %v", err)
	}
}
