package platform

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestErrorKindNeverCarriesThePath(t *testing.T) {
	const secret = "LAB3-PC07 (ana)"
	dir := filepath.Join(t.TempDir(), secret)
	_, statErr := os.Stat(filepath.Join(dir, "tarea.py"))
	own := errors.New("pkg: invalid")
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"path error", fmt.Errorf("save: %w", statErr), fs.ErrNotExist.Error()},
		{"quoted path", fmt.Errorf("work folder: %q leaves the folder: %w", secret+"/x", ErrLinkInPath), ErrLinkInPath.Error()},
		{"caller sentinel first", fmt.Errorf("%w: %q", own, secret), own.Error()},
		{"both sentinels", fmt.Errorf("%w: %w", ErrNoSpace, &fs.PathError{Op: "write", Path: secret, Err: syscall.Errno(112)}), ErrNoSpace.Error()},
		{"errno", &fs.PathError{Op: "open", Path: secret, Err: syscall.Errno(1234)}, "errno 1234"},
		{"other", fmt.Errorf("too many files named %s", secret), "other"},
	} {
		got := ErrorKind(tc.err, own)
		if got != tc.want {
			t.Errorf("%s: ErrorKind = %q, want %q", tc.name, got, tc.want)
		}
		if strings.Contains(got, secret) || strings.Contains(got, "ana") {
			t.Errorf("%s: the kind %q carries the path", tc.name, got)
		}
	}
}
