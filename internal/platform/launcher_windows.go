//go:build windows

package platform

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	errNoAssociation  = syscall.Errno(1155) // ERROR_NO_ASSOCIATION
	coinitApartment   = 0x2                 // COINIT_APARTMENTTHREADED
	coinitNoOLE1DDE   = 0x4                 // COINIT_DISABLE_OLE1DDE
	finalPathMaxChars = 32768
)

func newLauncher() (Launcher, error) { return winLauncher{}, nil }

type winLauncher struct{}

func knownFolder(id *windows.KNOWNFOLDERID) string {
	dir, err := windows.KnownFolderPath(id, windows.KF_FLAG_DEFAULT)
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Clean(dir)
}

func (winLauncher) Folders() (AppFolders, error) {
	pf := knownFolder(windows.FOLDERID_ProgramFiles)
	pf86 := knownFolder(windows.FOLDERID_ProgramFilesX86)
	sysRoot := knownFolder(windows.FOLDERID_Windows)
	local := knownFolder(windows.FOLDERID_LocalAppData)
	if pf == "" || sysRoot == "" {
		return AppFolders{}, errors.New("app folders: Program Files or Windows folder unknown")
	}
	f := AppFolders{
		Vars: map[string]string{"ProgramFiles": pf, "ProgramFiles(x86)": pf86, "SystemRoot": sysRoot, "LOCALAPPDATA": local},
	}
	userPrograms := knownFolder(windows.FOLDERID_UserProgramFiles)
	for _, r := range []string{pf, pf86, sysRoot, userPrograms} {
		if r != "" {
			f.Roots = append(f.Roots, r)
		}
	}
	tmp := filepath.Clean(os.TempDir())
	for _, d := range []string{
		knownFolder(windows.FOLDERID_Documents), knownFolder(windows.FOLDERID_Desktop),
		knownFolder(windows.FOLDERID_Downloads), tmp, filepath.Join(sysRoot, "Temp"),
	} {
		if d != "" && d != "." {
			f.Forbidden = append(f.Forbidden, d)
		}
	}
	return f, nil
}

// Resolve opens the file and asks Windows for its final path, so links and
// junctions on the way are already followed.
func (winLauncher) Resolve(path string) (string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", fmt.Errorf("resolve: %w", err)
	}
	h, err := windows.CreateFile(p, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0) // also opens a folder, to refuse it
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return "", fmt.Errorf("resolve: %w", fs.ErrNotExist)
		}
		return "", fmt.Errorf("resolve: %w", err)
	}
	defer windows.CloseHandle(h)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return "", fmt.Errorf("resolve: %w", err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return "", fmt.Errorf("resolve: a folder: %w", fs.ErrNotExist)
	}
	buf := make([]uint16, finalPathMaxChars)
	n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 0) // FILE_NAME_NORMALIZED | VOLUME_NAME_DOS
	if err != nil {
		return "", fmt.Errorf("resolve: %w", err)
	}
	final := windows.UTF16ToString(buf[:n])
	if strings.HasPrefix(final, `\\?\UNC\`) {
		return "", errors.New("resolve: the file is on a network share")
	}
	return strings.TrimPrefix(final, `\\?\`), nil
}

// OpenURL runs ShellExecuteW "open" on a thread with COM initialized, as the
// shell asks for.
func (winLauncher) OpenURL(url string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(url)
	if err != nil {
		return fmt.Errorf("open url: %w", err)
	}
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := windows.CoInitializeEx(0, coinitApartment|coinitNoOLE1DDE); err == nil {
			defer windows.CoUninitialize()
		}
		done <- windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
	}()
	if err := <-done; err != nil {
		if errors.Is(err, errNoAssociation) {
			return ErrNoDefaultBrowser
		}
		return fmt.Errorf("open url: %w", err)
	}
	return nil
}

// StartApp creates the process with the path as its only argument (argv[0])
// and its folder as the working directory; Aula does not wait for it.
func (winLauncher) StartApp(path string) error {
	cmd := exec.Command(path) // the command line is only the program's own name
	cmd.Dir = filepath.Dir(path)
	if err := cmd.Start(); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("start app: %w", fs.ErrNotExist)
		}
		return fmt.Errorf("start app: %w", err)
	}
	return cmd.Process.Release()
}
