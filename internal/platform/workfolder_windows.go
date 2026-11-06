//go:build windows

package platform

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// maxWindowsPath is MAX_PATH less the terminating NUL: Explorer and most
// programs cannot open a longer path.
const maxWindowsPath = 259

type workFolder struct {
	// root returns the work folder; tests point it at a temporary folder.
	root func() (string, error)
}

func newWorkFolder() (WorkFolder, error) {
	return &workFolder{root: func() (string, error) {
		docs, err := documentsDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(docs, WorkFolderName), nil
	}}, nil
}

func (w *workFolder) Root() (string, error) {
	root, err := w.root()
	if err != nil {
		return "", fmt.Errorf("work folder: %w", err)
	}
	if err := ensureDir(root); err != nil {
		return "", fmt.Errorf("work folder: %w", err)
	}
	return root, nil
}

// resolve joins rel to the root and checks it stays inside.
func (w *workFolder) resolve(root, rel string) (string, error) {
	if strings.ContainsAny(rel, `\:`) || filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return "", fmt.Errorf("work folder: %q: %w", rel, ErrLinkInPath)
	}
	full := filepath.Join(root, filepath.FromSlash(rel))
	inside, err := filepath.Rel(root, full)
	if err != nil || inside == ".." || strings.HasPrefix(inside, `..\`) {
		return "", fmt.Errorf("work folder: %q leaves the folder: %w", rel, ErrLinkInPath)
	}
	return full, nil
}

func (w *workFolder) Create(rel string, data []byte, overwrite bool) (bool, error) {
	root, err := w.Root()
	if err != nil {
		return false, err
	}
	if rel == "" {
		return false, fmt.Errorf("work folder: empty name: %w", ErrLinkInPath)
	}
	target, err := w.resolve(root, rel)
	if err != nil {
		return false, err
	}
	if len(target) > maxWindowsPath {
		return false, fmt.Errorf("work folder: %d characters: %w", len(target), ErrPathTooLong)
	}
	// The root itself may be a junction an administrator set up; what Aula
	// refuses is a link below it, which could lead the write elsewhere.
	if err := checkNoLink(root); err != nil {
		return false, err
	}
	dir := filepath.Dir(target)
	if err := makeDirs(root, dir); err != nil {
		return false, err
	}
	tmp, err := writeTemp(dir, data)
	if err != nil {
		return false, err
	}
	replaced := false
	flags := uint32(windows.MOVEFILE_WRITE_THROUGH)
	if overwrite {
		if info, err := os.Lstat(target); err == nil {
			if info.IsDir() {
				_ = os.Remove(tmp)
				return false, fmt.Errorf("work folder: a folder has that name: %w", ErrFileExists)
			}
			replaced = true
		}
		flags |= windows.MOVEFILE_REPLACE_EXISTING
	}
	if err := moveFile(tmp, target, flags); err != nil {
		_ = os.Remove(tmp)
		return false, fmt.Errorf("work folder: %w", mapWriteError(err))
	}
	return replaced, nil
}

func (w *workFolder) Open(rel string) error {
	root, err := w.Root()
	if err != nil {
		return err
	}
	dir := root
	if rel != "" {
		if dir, err = w.resolve(root, rel); err != nil {
			return err
		}
	}
	return openFolder(dir)
}

func ensureDir(dir string) error {
	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		return mapWriteError(err)
	}
	return nil
}

// makeDirs creates dir (inside root) one level at a time, refusing any level
// that is a link or a junction.
func makeDirs(root, dir string) error {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return fmt.Errorf("work folder: %w", err)
	}
	if rel == "." {
		return nil
	}
	cur := root
	for _, seg := range strings.Split(rel, `\`) {
		cur = filepath.Join(cur, seg)
		if err := os.Mkdir(cur, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("work folder: %w", mapWriteError(err))
		}
		if err := checkNoLink(cur); err != nil {
			return err
		}
	}
	return nil
}

// checkNoLink fails if path is a reparse point (symbolic link, junction,
// mount point) or not a folder.
func checkNoLink(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("work folder: %w", err)
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return fmt.Errorf("work folder: %w", mapWriteError(err))
	}
	if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("work folder: %s: %w", filepath.Base(path), ErrLinkInPath)
	}
	if attrs&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return fmt.Errorf("work folder: a file is in the way: %w", ErrFileExists)
	}
	return nil
}

// writeTemp writes data to a new hidden file in dir and returns its path.
func writeTemp(dir string, data []byte) (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("work folder: %w", err)
	}
	tmp := filepath.Join(dir, ".aula-"+hex.EncodeToString(b[:])+".tmp")
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("work folder: %w", mapWriteError(err))
	}
	if p, err := windows.UTF16PtrFromString(tmp); err == nil {
		_ = windows.SetFileAttributes(p, windows.FILE_ATTRIBUTE_HIDDEN)
	}
	_, werr := f.Write(data)
	if werr == nil {
		werr = f.Sync()
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("work folder: %w", mapWriteError(werr))
	}
	// The final file is a normal one.
	if p, err := windows.UTF16PtrFromString(tmp); err == nil {
		_ = windows.SetFileAttributes(p, windows.FILE_ATTRIBUTE_NORMAL)
	}
	return tmp, nil
}

func moveFile(from, to string, flags uint32) error {
	f, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	t, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(f, t, flags)
}

// mapWriteError turns the OS errors a write can meet into the errors of
// WorkFolder, keeping the original wrapped.
func mapWriteError(err error) error {
	var errno windows.Errno
	if !errors.As(err, &errno) {
		return err
	}
	switch errno {
	case windows.ERROR_ALREADY_EXISTS, windows.ERROR_FILE_EXISTS:
		return fmt.Errorf("%w: %w", ErrFileExists, err)
	case windows.ERROR_DISK_FULL, windows.ERROR_HANDLE_DISK_FULL:
		return fmt.Errorf("%w: %w", ErrNoSpace, err)
	case windows.ERROR_ACCESS_DENIED, windows.ERROR_SHARING_VIOLATION, windows.ERROR_LOCK_VIOLATION,
		windows.ERROR_WRITE_PROTECT:
		return fmt.Errorf("%w: %w", ErrDenied, err)
	case windows.ERROR_FILENAME_EXCED_RANGE:
		return fmt.Errorf("%w: %w", ErrPathTooLong, err)
	}
	return err
}
