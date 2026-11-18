//go:build linux

package platform

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// The tests point the work folder at a temporary one: they never write to
// the user's Documents.
func testWorkFolder(t *testing.T) (*workFolder, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), WorkFolderName)
	return &workFolder{root: func() (string, error) { return root, nil }}, root
}

func TestWorkFolderCreate(t *testing.T) {
	w, root := testWorkFolder(t)
	replaced, err := w.Create("Ejercicio 4/datos/datos.csv", []byte("id,n\n"), false)
	if err != nil || replaced {
		t.Fatalf("Create = %v, %v", replaced, err)
	}
	file := filepath.Join(root, "Ejercicio 4", "datos", "datos.csv")
	if got, err := os.ReadFile(file); err != nil || string(got) != "id,n\n" {
		t.Fatalf("file = %q, %v", got, err)
	}
	if _, err := w.Create("Ejercicio 4/datos/datos.csv", []byte("otro"), false); !errors.Is(err, ErrFileExists) {
		t.Fatalf("second Create without overwrite = %v, want ErrFileExists", err)
	}
	if got, _ := os.ReadFile(file); string(got) != "id,n\n" {
		t.Fatalf("the student's file changed: %q", got)
	}
	replaced, err = w.Create("Ejercicio 4/datos/datos.csv", []byte("nuevo"), true)
	if err != nil || !replaced {
		t.Fatalf("Create with overwrite = %v, %v", replaced, err)
	}
	if got, _ := os.ReadFile(file); string(got) != "nuevo" {
		t.Fatalf("not replaced: %q", got)
	}
	if replaced, err := w.Create("vacio.py", nil, true); err != nil || replaced {
		t.Fatalf("empty file = %v, %v", replaced, err)
	}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, _ error) error {
		if strings.HasPrefix(d.Name(), ".aula-") {
			t.Errorf("temporary file left: %s", p)
		}
		return nil
	})
}

func TestWorkFolderFollowsUmask(t *testing.T) {
	old := syscall.Umask(0o022)
	defer syscall.Umask(old)
	w, root := testWorkFolder(t)
	if _, err := w.Create("a/b.txt", []byte("x"), false); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]os.FileMode{filepath.Join(root, "a"): 0o755, filepath.Join(root, "a", "b.txt"): 0o644} {
		if info, err := os.Stat(p); err != nil || info.Mode().Perm() != want {
			t.Errorf("%s: mode %v, %v; want %o like any file the student saves", p, info.Mode().Perm(), err, want)
		}
	}
}

func TestWorkFolderStaysInside(t *testing.T) {
	w, root := testWorkFolder(t)
	outside := filepath.Join(filepath.Dir(root), "fuera.txt")
	for _, rel := range []string{"../fuera.txt", "a/../../fuera.txt", `..\fuera.txt`, "/x.txt", "a:b.txt", "a//b", "./a", "a/\x00", ""} {
		if _, err := w.Create(rel, []byte("x"), true); !errors.Is(err, ErrLinkInPath) {
			t.Errorf("Create(%q) = %v, want ErrLinkInPath", rel, err)
		}
	}
	if _, err := os.Stat(outside); err == nil {
		t.Fatal("a file was written outside the work folder")
	}
	if err := w.Open("../"); !errors.Is(err, ErrLinkInPath) {
		t.Errorf("Open outside = %v, want ErrLinkInPath", err)
	}
}

// A symbolic link below the root is never followed: not to write, not to
// create folders, not to list, not to read (docs/privacidad.md rule 98).
func TestWorkFolderRefusesSymlinks(t *testing.T) {
	w, root := testWorkFolder(t)
	if _, err := w.Root(); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "secreto.txt"), []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "atajo")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(target, "secreto.txt"), filepath.Join(root, "enlace.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Create("atajo/x.txt", []byte("x"), false); !errors.Is(err, ErrLinkInPath) {
		t.Fatalf("Create through a link = %v, want ErrLinkInPath", err)
	}
	if _, err := os.Stat(filepath.Join(target, "x.txt")); err == nil {
		t.Fatal("the write followed the link")
	}
	if err := w.MkdirAll("atajo/sub"); !errors.Is(err, ErrLinkInPath) {
		t.Fatalf("MkdirAll through a link = %v, want ErrLinkInPath", err)
	}
	if _, err := w.List("atajo", 4); !errors.Is(err, ErrLinkInPath) {
		t.Fatalf("List through a link = %v, want ErrLinkInPath", err)
	}
	if _, err := w.Read("enlace.txt", 100); !errors.Is(err, ErrLinkInPath) {
		t.Fatalf("Read of a link = %v, want ErrLinkInPath", err)
	}
	if _, err := w.Read("atajo/secreto.txt", 100); !errors.Is(err, ErrLinkInPath) {
		t.Fatalf("Read through a link = %v, want ErrLinkInPath", err)
	}
	// Overwriting the name of a link replaces the link, never its target.
	if replaced, err := w.Create("enlace.txt", []byte("nuevo"), true); err != nil || !replaced {
		t.Fatalf("Create over a link = %v, %v", replaced, err)
	}
	if got, _ := os.ReadFile(filepath.Join(target, "secreto.txt")); string(got) != "no" {
		t.Fatalf("the link's target changed: %q", got)
	}
	files, err := w.List("", 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "atajo" && !f.Link {
			t.Errorf("the link is not marked: %+v", f)
		}
		if strings.HasPrefix(f.Path, "atajo/") {
			t.Errorf("List entered a link: %s", f.Path)
		}
	}
}

func TestWorkFolderPathTooLong(t *testing.T) {
	w, _ := testWorkFolder(t)
	long := strings.Repeat("a", 300) + ".txt" // NAME_MAX is 255
	if _, err := w.Create(long, []byte("x"), false); !errors.Is(err, ErrPathTooLong) {
		t.Fatalf("Create = %v, want ErrPathTooLong", err)
	}
}

func TestWorkFolderFileInTheWay(t *testing.T) {
	w, _ := testWorkFolder(t)
	if _, err := w.Create("datos", []byte("x"), false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Create("datos/x.txt", []byte("x"), false); err == nil {
		t.Fatal("a file named like a folder on the way must fail")
	}
	if _, err := w.Create("carpeta/x.txt", []byte("x"), false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Create("carpeta", []byte("x"), true); !errors.Is(err, ErrFileExists) {
		t.Fatalf("overwrite a folder = %v, want ErrFileExists", err)
	}
}

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
	dir := filepath.Join(root, filepath.FromSlash(act))
	if err := os.WriteFile(filepath.Join(dir, ".aula-1.tmp"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "tubo"), 0o600); err != nil {
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
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("List = %v, want %v", got, want)
	}
	if files[1].Size != 9 || files[1].ModTime.IsZero() {
		t.Errorf("main.py = %+v", files[1])
	}
	if data, err := w.Read(act+"/main.py", 100); err != nil || string(data) != "print(1)\n" {
		t.Fatalf("Read = %q, %v", data, err)
	}
	if _, err := w.Read(act+"/grande.bin", 1<<20); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("Read big = %v, want ErrFileTooLarge", err)
	}
	if _, err := w.Read(act+"/tubo", 100); !errors.Is(err, ErrDenied) {
		t.Fatalf("Read a pipe = %v, want ErrDenied (and never hang)", err)
	}
	if _, err := w.Read(act+"/src", 100); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Read a folder = %v, want fs.ErrNotExist", err)
	}
	if _, err := w.List("recolecciones/2026-11-06/Otra", 4); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("List missing = %v, want fs.ErrNotExist", err)
	}
}

func TestWorkFolderReadDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every file")
	}
	w, root := testWorkFolder(t)
	if _, err := w.Create("cerrado.txt", []byte("x"), false); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "cerrado.txt"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Read("cerrado.txt", 100); !errors.Is(err, ErrDenied) {
		t.Fatalf("Read = %v, want ErrDenied", err)
	}
}
