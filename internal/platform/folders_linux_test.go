//go:build linux

package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXDGUserDir(t *testing.T) {
	const home = "/home/ana"
	for _, tc := range []struct {
		file, want string
		ok         bool
	}{
		{`XDG_DOCUMENTS_DIR="$HOME/Documentos"`, "/home/ana/Documentos", true},
		{"# comment\nXDG_DESKTOP_DIR=\"$HOME/Escritorio\"\n  XDG_DOCUMENTS_DIR=\"$HOME/Mis documentos\"\n", "/home/ana/Mis documentos", true},
		{`XDG_DOCUMENTS_DIR="/srv/docs/ana"`, "/srv/docs/ana", true},
		{"XDG_DOCUMENTS_DIR=\"$HOME/A\"\nXDG_DOCUMENTS_DIR=\"$HOME/B\"", "/home/ana/B", true}, // the last one wins
		{`XDG_DOCUMENTS_DIR="$HOME"`, "", false},                                              // disabled
		{`XDG_DOCUMENTS_DIR="$HOME/"`, "", false},
		{`XDG_DOCUMENTS_DIR="relativa"`, "", false},
		{`XDG_DOCUMENTS_DIR=$HOME/sin-comillas`, "", false},
		{`XDG_DOWNLOAD_DIR="$HOME/Descargas"`, "", false},
		{"", "", false},
	} {
		got, ok := xdgUserDir(strings.NewReader(tc.file), "DOCUMENTS", home)
		if got != tc.want || ok != tc.ok {
			t.Errorf("xdgUserDir(%q) = %q, %v; want %q, %v", tc.file, got, ok, tc.want, tc.ok)
		}
	}
}

func TestDocumentsDirFromXDG(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	if got, err := documentsDir(); err != nil || got != filepath.Join(home, "Documents") {
		t.Fatalf("without user-dirs.dirs = %q, %v", got, err)
	}
	if err := os.MkdirAll(filepath.Join(home, "cfg"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "cfg", "user-dirs.dirs"), []byte(`XDG_DOCUMENTS_DIR="$HOME/Documentos"`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := documentsDir(); err != nil || got != filepath.Join(home, "Documentos") {
		t.Fatalf("with user-dirs.dirs = %q, %v", got, err)
	}
}

func TestOpenFolderChecks(t *testing.T) {
	if err := openFolder("relativa"); err == nil {
		t.Error("a relative folder must fail")
	}
	f := filepath.Join(t.TempDir(), "x.txt")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := openFolder(f); err == nil {
		t.Error("a file must fail")
	}
}
