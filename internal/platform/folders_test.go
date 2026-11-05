package platform

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDocumentsDir(t *testing.T) {
	dir, err := DocumentsDir()
	if err != nil {
		t.Skipf("no documents folder here: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("DocumentsDir = %q, want an absolute path", dir)
	}
}

// OpenFolder is not called on a real folder: it would open a window on the
// desktop of whoever runs the tests (manual case C-51).
func TestOpenFolderRejectsMissingOrFiles(t *testing.T) {
	if runtime.GOOS != "windows" {
		if err := OpenFolder(t.TempDir()); err != ErrNotSupported {
			t.Fatalf("OpenFolder = %v, want ErrNotSupported", err)
		}
		return
	}
	if err := OpenFolder(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("OpenFolder of a missing folder must fail")
	}
}
