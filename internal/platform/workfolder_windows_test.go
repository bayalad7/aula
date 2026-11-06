//go:build windows

package platform

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	got, err := os.ReadFile(filepath.Join(root, "Ejercicio 4", "datos", "datos.csv"))
	if err != nil || string(got) != "id,n\n" {
		t.Fatalf("file = %q, %v", got, err)
	}
	if _, err := w.Create("Ejercicio 4/datos/datos.csv", []byte("otro"), false); !errors.Is(err, ErrFileExists) {
		t.Fatalf("second Create without overwrite = %v, want ErrFileExists", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "Ejercicio 4", "datos", "datos.csv")); string(got) != "id,n\n" {
		t.Fatalf("the student's file changed: %q", got)
	}
	replaced, err = w.Create("Ejercicio 4/datos/datos.csv", []byte("nuevo"), true)
	if err != nil || !replaced {
		t.Fatalf("Create with overwrite = %v, %v", replaced, err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "Ejercicio 4", "datos", "datos.csv")); string(got) != "nuevo" {
		t.Fatalf("not replaced: %q", got)
	}
	if replaced, err := w.Create("vacio.py", nil, true); err != nil || replaced {
		t.Fatalf("empty file = %v, %v", replaced, err)
	}
	// No temporary file is left behind.
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, _ error) error {
		if strings.HasPrefix(d.Name(), ".aula-") {
			t.Errorf("temporary file left: %s", p)
		}
		return nil
	})
}

func TestWorkFolderLargeFile(t *testing.T) {
	w, root := testWorkFolder(t)
	data := bytes.Repeat([]byte{0x5a}, 3<<20)
	if _, err := w.Create("grande.bin", data, false); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(root, "grande.bin")); err != nil || info.Size() != int64(len(data)) {
		t.Fatalf("size = %v, %v", info, err)
	}
}

func TestWorkFolderStaysInside(t *testing.T) {
	w, root := testWorkFolder(t)
	outside := filepath.Join(filepath.Dir(root), "fuera.txt")
	for _, rel := range []string{"../fuera.txt", "a/../../fuera.txt", `..\fuera.txt`, `C:\Windows\x.txt`, "/x.txt", "a:b.txt", ""} {
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

func TestWorkFolderRefusesJunction(t *testing.T) {
	w, root := testWorkFolder(t)
	if _, err := w.Root(); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	link := filepath.Join(root, "atajo")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("cannot create a junction here: %v %s", err, out)
	}
	if _, err := w.Create("atajo/x.txt", []byte("x"), false); !errors.Is(err, ErrLinkInPath) {
		t.Fatalf("Create through a junction = %v, want ErrLinkInPath", err)
	}
	if _, err := os.Stat(filepath.Join(target, "x.txt")); err == nil {
		t.Fatal("the write followed the junction")
	}
}

func TestWorkFolderPathTooLong(t *testing.T) {
	w, _ := testWorkFolder(t)
	long := strings.Repeat("a", 90) + "/" + strings.Repeat("b", 90) + "/" + strings.Repeat("c", 90) + ".txt"
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
}
