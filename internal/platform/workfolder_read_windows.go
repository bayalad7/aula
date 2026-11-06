//go:build windows

package platform

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Attributes of a file that is only a placeholder of a cloud file.
const offlineAttrs = windows.FILE_ATTRIBUTE_OFFLINE | windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS |
	windows.FILE_ATTRIBUTE_RECALL_ON_OPEN

func newWorkFolderReader() (WorkFolderReader, error) {
	w, err := newWorkFolder()
	if err != nil {
		return nil, err
	}
	return w.(*workFolder), nil
}

// fileAttributeTagInfo is FILE_ATTRIBUTE_TAG_INFO.
type fileAttributeTagInfo struct {
	FileAttributes uint32
	ReparseTag     uint32
}

// isLinkTag reports whether a reparse tag redirects to another place: a
// symbolic link or a junction (mount point). Other reparse points (cloud
// files, deduplication) are the file itself.
func isLinkTag(attrs, tag uint32) bool {
	return attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 &&
		(tag == windows.IO_REPARSE_TAG_SYMLINK || tag == windows.IO_REPARSE_TAG_MOUNT_POINT)
}

func handleTag(h windows.Handle) (fileAttributeTagInfo, error) {
	var info fileAttributeTagInfo
	err := windows.GetFileInformationByHandleEx(h, windows.FileAttributeTagInfo,
		(*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	return info, err
}

// pathTag reads the attributes and reparse tag of p without following it.
func pathTag(p string) (fileAttributeTagInfo, error) {
	name, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return fileAttributeTagInfo{}, err
	}
	h, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return fileAttributeTagInfo{}, err
	}
	defer windows.CloseHandle(h)
	return handleTag(h)
}

// insideDir checks that every folder from root down to dir exists and is a
// real folder, not a link.
func insideDir(root, dir string) error {
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
		info, err := pathTag(cur)
		if err != nil {
			return fmt.Errorf("work folder: %w", mapWriteError(err))
		}
		if isLinkTag(info.FileAttributes, info.ReparseTag) {
			return fmt.Errorf("work folder: %s: %w", seg, ErrLinkInPath)
		}
		if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
			return fmt.Errorf("work folder: a file is in the way: %w", fs.ErrNotExist)
		}
	}
	return nil
}

func (w *workFolder) MkdirAll(rel string) error {
	root, err := w.Root()
	if err != nil {
		return err
	}
	dir, err := w.resolve(root, rel)
	if err != nil {
		return err
	}
	if len(dir) > maxWindowsPath {
		return fmt.Errorf("work folder: %d characters: %w", len(dir), ErrPathTooLong)
	}
	return makeDirs(root, dir)
}

func (w *workFolder) List(rel string, depth int) ([]WorkFile, error) {
	root, err := w.Root()
	if err != nil {
		return nil, err
	}
	dir, err := w.resolve(root, rel)
	if err != nil {
		return nil, err
	}
	if err := insideDir(root, dir); err != nil {
		return nil, err
	}
	var out []WorkFile
	if err := listDir(dir, "", depth, &out); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func listDir(dir, prefix string, depth int, out *[]WorkFile) error {
	if depth < 1 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("work folder: %w", mapWriteError(err))
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".aula-") && strings.HasSuffix(name, ".tmp") {
			continue // a write of file.push in progress
		}
		full := filepath.Join(dir, name)
		rel := path.Join(prefix, name)
		info, err := e.Info()
		if err != nil {
			continue // removed while listing
		}
		var attrs uint32
		if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
			attrs = d.FileAttributes
		}
		if attrs&windows.FILE_ATTRIBUTE_HIDDEN != 0 && attrs&windows.FILE_ATTRIBUTE_SYSTEM != 0 {
			continue // desktop.ini and the like
		}
		link := false
		if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			t, err := pathTag(full)
			link = err != nil || isLinkTag(t.FileAttributes, t.ReparseTag)
		}
		if link {
			*out = append(*out, WorkFile{Path: rel, ModTime: info.ModTime(), Link: true})
			continue
		}
		if info.IsDir() {
			if err := listDir(full, rel, depth-1, out); err != nil {
				return err
			}
			continue
		}
		*out = append(*out, WorkFile{Path: rel, Size: info.Size(), ModTime: info.ModTime(),
			Offline: attrs&offlineAttrs != 0})
	}
	return nil
}

func (w *workFolder) Read(rel string, limit int64) ([]byte, error) {
	root, err := w.Root()
	if err != nil {
		return nil, err
	}
	if rel == "" {
		return nil, fmt.Errorf("work folder: empty name: %w", ErrLinkInPath)
	}
	full, err := w.resolve(root, rel)
	if err != nil {
		return nil, err
	}
	if err := insideDir(root, filepath.Dir(full)); err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(full)
	if err != nil {
		return nil, fmt.Errorf("work folder: %w", err)
	}
	// Read sharing with writers: an editor that keeps the file open must
	// not stop the collection; one that locks it gives ErrDenied.
	h, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_SEQUENTIAL_SCAN, 0)
	if err != nil {
		return nil, fmt.Errorf("work folder: %w", mapWriteError(err))
	}
	f := os.NewFile(uintptr(h), full)
	defer f.Close()
	tag, err := handleTag(h)
	if err != nil {
		return nil, fmt.Errorf("work folder: %w", err)
	}
	switch {
	case isLinkTag(tag.FileAttributes, tag.ReparseTag):
		return nil, fmt.Errorf("work folder: %w", ErrLinkInPath)
	case tag.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0:
		return nil, fmt.Errorf("work folder: a folder: %w", fs.ErrNotExist)
	case tag.FileAttributes&offlineAttrs != 0:
		return nil, fmt.Errorf("work folder: %w", ErrOffline)
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
