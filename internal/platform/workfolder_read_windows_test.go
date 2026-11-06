//go:build windows

package platform

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWorkFolderReaderListAndRead(t *testing.T) {
	w, root := testWorkFolder(t)
	act := "recolecciones/2026-11-06/Ejercicio 4"
	if err := w.MkdirAll(act); err != nil {
		t.Fatal(err)
	}
	big := bytes.Repeat([]byte{7}, 2<<20)
	for rel, data := range map[string][]byte{
		act + "/main.py":          []byte("print(1)\n"),
		act + "/src/util.py":      []byte("x = 1\n"),
		act + "/a/b/c/d/hondo.py": []byte("deep"), // level 5: not listed with depth 4
		act + "/grande.bin":       big,
		"fuera.py":                []byte("not in the activity"),
	} {
		if _, err := w.Create(rel, data, false); err != nil {
			t.Fatal(err)
		}
	}
	// A write of file.push in progress is never listed.
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(act), ".aula-1.tmp"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := w.List(act, 4)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range files {
		got = append(got, f.Path)
	}
	want := []string{"grande.bin", "main.py", "src/util.py"}
	if len(got) != len(want) {
		t.Fatalf("List = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("List = %v, want %v", got, want)
		}
	}
	if files[0].Size != int64(len(big)) || files[0].ModTime.IsZero() {
		t.Errorf("grande.bin info = %+v", files[0])
	}
	data, err := w.Read(act+"/src/util.py", 100)
	if err != nil || string(data) != "x = 1\n" {
		t.Fatalf("Read = %q, %v", data, err)
	}
	if _, err := w.Read(act+"/grande.bin", 1<<20); !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("Read over the limit = %v, want ErrFileTooLarge", err)
	}
	if _, err := w.Read(act+"/no.py", 100); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Read missing = %v, want ErrNotExist", err)
	}
	if _, err := w.List("recolecciones/2026-11-06/Otra", 4); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("List missing = %v, want ErrNotExist", err)
	}
	for _, rel := range []string{"../x", act + "/../../../../fuera.py", `C:\Windows\win.ini`, ""} {
		if _, err := w.Read(rel, 100); !errors.Is(err, ErrLinkInPath) {
			t.Errorf("Read(%q) = %v, want ErrLinkInPath", rel, err)
		}
	}
}

func TestWorkFolderReaderEditorKeepsFileOpen(t *testing.T) {
	w, root := testWorkFolder(t)
	if _, err := w.Create("r/a.py", []byte("abc"), false); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(root, "r", "a.py"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if data, err := w.Read("r/a.py", 10); err != nil || string(data) != "abc" {
		t.Fatalf("Read while open = %q, %v", data, err)
	}
}

func TestWorkFolderReaderSkipsJunction(t *testing.T) {
	w, root := testWorkFolder(t)
	if err := w.MkdirAll("r"); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "secreto.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "r", "atajo")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("cannot create a junction here: %v %s", err, out)
	}
	files, err := w.List("r", 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "atajo" || !files[0].Link {
		t.Fatalf("List = %+v, want only the junction marked Link", files)
	}
	if _, err := w.Read("r/atajo/secreto.txt", 10); !errors.Is(err, ErrLinkInPath) {
		t.Fatalf("Read through a junction = %v, want ErrLinkInPath", err)
	}
	if _, err := w.List("r/atajo", 4); !errors.Is(err, ErrLinkInPath) {
		t.Fatalf("List through a junction = %v, want ErrLinkInPath", err)
	}
	if err := w.MkdirAll("r/atajo/nueva"); !errors.Is(err, ErrLinkInPath) {
		t.Fatalf("MkdirAll through a junction = %v, want ErrLinkInPath", err)
	}
}
