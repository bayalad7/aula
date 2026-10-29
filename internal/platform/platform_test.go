package platform

import (
	"errors"
	"fmt"
	"os"
	"testing"
)

func TestErrNotSupportedWrapping(t *testing.T) {
	err := fmt.Errorf("capture screen: %w", ErrNotSupported)
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("errors.Is(%v, ErrNotSupported) = false", err)
	}
}

func TestOpenLogFileRotates(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir()) // os.UserCacheDir on Windows
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	f, path, err := OpenLogFile("prueba", "app.log")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(make([]byte, maxLogSize+1)); err != nil {
		t.Fatal(err)
	}
	f.Close()
	f2, path2, err := OpenLogFile("prueba", "app.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
	if path2 != path {
		t.Fatalf("path changed: %s vs %s", path, path2)
	}
	if info, _ := f2.Stat(); info.Size() != 0 {
		t.Fatal("log not rotated")
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("previous log not kept: %v", err)
	}
}
