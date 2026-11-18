//go:build linux

package platform

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// The work folder on Linux (docs/privacidad.md rule 98): every level below
// the root is opened with openat and O_NOFOLLOW from the level above, so a
// symbolic link on the way is refused even if it appears while Aula walks.
// The root itself may be a link an administrator set up, as on Windows.

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

func newWorkFolderReader() (WorkFolderReader, error) {
	w, err := newWorkFolder()
	if err != nil {
		return nil, err
	}
	return w.(*workFolder), nil
}

func (w *workFolder) Root() (string, error) {
	root, err := w.root()
	if err != nil {
		return "", fmt.Errorf("work folder: %w", err)
	}
	if err := os.MkdirAll(root, 0o777); err != nil {
		return "", fmt.Errorf("work folder: %w", mapWriteError(err))
	}
	return root, nil
}

// splitRel checks a "/"-separated path relative to the work folder and
// returns its segments. internal/filepush and internal/collect already
// checked it; this refuses anything that could leave the folder anyway.
func splitRel(rel string) ([]string, error) {
	if rel == "" {
		return nil, nil
	}
	if strings.HasPrefix(rel, "/") || strings.ContainsAny(rel, "\\:\x00") {
		return nil, fmt.Errorf("work folder: %q: %w", rel, ErrLinkInPath)
	}
	segs := strings.Split(rel, "/")
	for _, s := range segs {
		if s == "" || s == "." || s == ".." {
			return nil, fmt.Errorf("work folder: %q leaves the folder: %w", rel, ErrLinkInPath)
		}
	}
	return segs, nil
}

// openRoot opens the root folder (following it if it is a link).
func openRoot(root string) (int, error) {
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, fmt.Errorf("work folder: %w", mapWriteError(err))
	}
	return fd, nil
}

// walkDirs opens the folders segs below the folder dirfd, one level at a
// time, never following a link; with create it makes the missing ones. The
// caller closes the returned descriptor; dirfd stays open.
func walkDirs(dirfd int, segs []string, create bool) (int, error) {
	cur, err := unix.Dup(dirfd)
	if err != nil {
		return -1, fmt.Errorf("work folder: %w", err)
	}
	for _, s := range segs {
		if create {
			if err := unix.Mkdirat(cur, s, 0o777); err != nil && !errors.Is(err, unix.EEXIST) {
				unix.Close(cur)
				return -1, fmt.Errorf("work folder: %w", mapWriteError(err))
			}
		}
		next, err := unix.Openat(cur, s, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			err = whyNotDir(cur, s, err)
			unix.Close(cur)
			return -1, err
		}
		unix.Close(cur)
		cur = next
	}
	return cur, nil
}

// whyNotDir explains why name in dirfd could not be opened as a folder: a
// link, a file in the way or the OS error.
func whyNotDir(dirfd int, name string, err error) error {
	var st unix.Stat_t
	if serr := unix.Fstatat(dirfd, name, &st, unix.AT_SYMLINK_NOFOLLOW); serr == nil {
		switch st.Mode & unix.S_IFMT {
		case unix.S_IFLNK:
			return fmt.Errorf("work folder: %s: %w", name, ErrLinkInPath)
		case unix.S_IFDIR:
		default:
			return fmt.Errorf("work folder: a file is in the way: %w: %w", ErrFileExists, fs.ErrNotExist)
		}
	}
	return fmt.Errorf("work folder: %w", mapWriteError(err))
}

func (w *workFolder) Create(rel string, data []byte, overwrite bool) (bool, error) {
	root, err := w.Root()
	if err != nil {
		return false, err
	}
	segs, err := splitRel(rel)
	if err != nil {
		return false, err
	}
	if len(segs) == 0 {
		return false, fmt.Errorf("work folder: empty name: %w", ErrLinkInPath)
	}
	if n := len(filepath.Join(root, rel)); n >= unix.PathMax {
		return false, fmt.Errorf("work folder: %d characters: %w", n, ErrPathTooLong)
	}
	rootfd, err := openRoot(root)
	if err != nil {
		return false, err
	}
	defer unix.Close(rootfd)
	dirfd, err := walkDirs(rootfd, segs[:len(segs)-1], true)
	if err != nil {
		return false, err
	}
	defer unix.Close(dirfd)
	name := segs[len(segs)-1]

	replaced := false
	var st unix.Stat_t
	if err := unix.Fstatat(dirfd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err == nil {
		if !overwrite {
			return false, fmt.Errorf("work folder: %w", ErrFileExists)
		}
		if st.Mode&unix.S_IFMT == unix.S_IFDIR {
			return false, fmt.Errorf("work folder: a folder has that name: %w", ErrFileExists)
		}
		replaced = true
	}
	tmp, err := writeTempAt(dirfd, data)
	if err != nil {
		return false, err
	}
	if overwrite {
		// rename replaces the name itself: a link there is replaced, never
		// followed.
		err = unix.Renameat(dirfd, tmp, dirfd, name)
	} else {
		err = renameNoReplace(dirfd, tmp, name)
	}
	if err != nil {
		_ = unix.Unlinkat(dirfd, tmp, 0)
		return false, fmt.Errorf("work folder: %w", mapWriteError(err))
	}
	_ = unix.Fsync(dirfd)
	return replaced, nil
}

// renameNoReplace moves tmp to name only if name does not exist. Without
// RENAME_NOREPLACE (some file systems), a hard link does the same.
func renameNoReplace(dirfd int, tmp, name string) error {
	err := unix.Renameat2(dirfd, tmp, dirfd, name, unix.RENAME_NOREPLACE)
	if !errors.Is(err, unix.EINVAL) && !errors.Is(err, unix.ENOSYS) {
		return err
	}
	if err := unix.Linkat(dirfd, tmp, dirfd, name, 0); err != nil {
		return err
	}
	return unix.Unlinkat(dirfd, tmp, 0)
}

// writeTempAt writes data to a new hidden file in dirfd and returns its name.
// The mode follows the student's umask, like any file they save.
func writeTempAt(dirfd int, data []byte) (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("work folder: %w", err)
	}
	tmp := ".aula-" + hex.EncodeToString(b[:]) + ".tmp"
	fd, err := unix.Openat(dirfd, tmp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o666)
	if err != nil {
		return "", fmt.Errorf("work folder: %w", mapWriteError(err))
	}
	f := os.NewFile(uintptr(fd), tmp)
	_, werr := f.Write(data)
	if werr == nil {
		werr = f.Sync()
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = unix.Unlinkat(dirfd, tmp, 0)
		return "", fmt.Errorf("work folder: %w", mapWriteError(werr))
	}
	return tmp, nil
}

func (w *workFolder) Open(rel string) error {
	root, err := w.Root()
	if err != nil {
		return err
	}
	segs, err := splitRel(rel)
	if err != nil {
		return err
	}
	return openFolder(filepath.Join(append([]string{root}, segs...)...))
}

func (w *workFolder) MkdirAll(rel string) error {
	root, err := w.Root()
	if err != nil {
		return err
	}
	segs, err := splitRel(rel)
	if err != nil {
		return err
	}
	if n := len(filepath.Join(root, rel)); n >= unix.PathMax {
		return fmt.Errorf("work folder: %d characters: %w", n, ErrPathTooLong)
	}
	rootfd, err := openRoot(root)
	if err != nil {
		return err
	}
	defer unix.Close(rootfd)
	fd, err := walkDirs(rootfd, segs, true)
	if err != nil {
		return err
	}
	return unix.Close(fd)
}

func (w *workFolder) List(rel string, depth int) ([]WorkFile, error) {
	root, err := w.Root()
	if err != nil {
		return nil, err
	}
	segs, err := splitRel(rel)
	if err != nil {
		return nil, err
	}
	rootfd, err := openRoot(root)
	if err != nil {
		return nil, err
	}
	defer unix.Close(rootfd)
	fd, err := walkDirs(rootfd, segs, false)
	if err != nil {
		return nil, err
	}
	var out []WorkFile
	err = listAt(fd, "", depth, &out) // closes fd
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// listAt appends the files below the folder fd, up to depth levels, and
// closes fd. Links are listed and never entered; sockets, pipes and devices
// are left out.
func listAt(fd int, prefix string, depth int, out *[]WorkFile) error {
	dir := os.NewFile(uintptr(fd), prefix)
	defer dir.Close()
	if depth < 1 {
		return nil
	}
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return fmt.Errorf("work folder: %w", mapWriteError(err))
	}
	for _, name := range names {
		if strings.HasPrefix(name, ".aula-") && strings.HasSuffix(name, ".tmp") {
			continue // a write of file.push in progress
		}
		var st unix.Stat_t
		if err := unix.Fstatat(fd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			continue // removed while listing
		}
		rel := path.Join(prefix, name)
		mod := time.Unix(st.Mtim.Unix())
		switch st.Mode & unix.S_IFMT {
		case unix.S_IFLNK:
			*out = append(*out, WorkFile{Path: rel, ModTime: mod, Link: true})
		case unix.S_IFDIR:
			sub, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			if err != nil {
				if errors.Is(err, unix.ELOOP) {
					*out = append(*out, WorkFile{Path: rel, ModTime: mod, Link: true})
				}
				continue // replaced or unreadable while listing
			}
			if err := listAt(sub, rel, depth-1, out); err != nil {
				return err
			}
		case unix.S_IFREG:
			*out = append(*out, WorkFile{Path: rel, Size: st.Size, ModTime: mod})
		}
	}
	return nil
}

func (w *workFolder) Read(rel string, limit int64) ([]byte, error) {
	root, err := w.Root()
	if err != nil {
		return nil, err
	}
	segs, err := splitRel(rel)
	if err != nil {
		return nil, err
	}
	if len(segs) == 0 {
		return nil, fmt.Errorf("work folder: empty name: %w", ErrLinkInPath)
	}
	rootfd, err := openRoot(root)
	if err != nil {
		return nil, err
	}
	defer unix.Close(rootfd)
	dirfd, err := walkDirs(rootfd, segs[:len(segs)-1], false)
	if err != nil {
		return nil, err
	}
	defer unix.Close(dirfd)
	// O_NONBLOCK: a named pipe must not hang the collection.
	fd, err := unix.Openat(dirfd, segs[len(segs)-1], unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("work folder: %w", mapWriteError(err))
	}
	f := os.NewFile(uintptr(fd), rel)
	defer f.Close()
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return nil, fmt.Errorf("work folder: %w", err)
	}
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFREG:
	case unix.S_IFDIR:
		return nil, fmt.Errorf("work folder: a folder: %w", fs.ErrNotExist)
	default:
		return nil, fmt.Errorf("work folder: not a regular file: %w", ErrDenied)
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("work folder: %w", mapWriteError(err))
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("work folder: more than %d bytes: %w", limit, ErrFileTooLarge)
	}
	return data, nil
}

// mapWriteError turns the OS errors a write or read can meet into the errors
// of WorkFolder, keeping the original wrapped.
func mapWriteError(err error) error {
	var errno unix.Errno
	if !errors.As(err, &errno) {
		return err
	}
	switch errno {
	case unix.EEXIST:
		return fmt.Errorf("%w: %w", ErrFileExists, err)
	case unix.ENOSPC, unix.EDQUOT:
		return fmt.Errorf("%w: %w", ErrNoSpace, err)
	case unix.EACCES, unix.EPERM, unix.EROFS, unix.ETXTBSY, unix.EBUSY:
		return fmt.Errorf("%w: %w", ErrDenied, err)
	case unix.ENAMETOOLONG:
		return fmt.Errorf("%w: %w", ErrPathTooLong, err)
	case unix.ELOOP:
		return fmt.Errorf("%w: %w", ErrLinkInPath, err)
	}
	return err
}
