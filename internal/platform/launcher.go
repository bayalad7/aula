package platform

import "errors"

// ErrNoDefaultBrowser: the session has no program for http/https links.
var ErrNoDefaultBrowser = errors.New("platform: no default browser")

// AppFolders are the folders that decide which executables open.app may
// start (docs/protocol.md §5.22): the roots where installed programs live and
// the user folders that never qualify. Every path is absolute and clean; a
// folder the OS does not have is empty. internal/openapp applies the rules.
type AppFolders struct {
	// Variables open.app may use, by name ("ProgramFiles", "ProgramFiles(x86)",
	// "LOCALAPPDATA", "SystemRoot").
	Vars map[string]string
	// Roots the executable must be inside.
	Roots []string
	// Folders it must never be inside, even under a root: Documents (and the
	// work folder in it), Desktop, Downloads and the temporary folder.
	Forbidden []string
}

// Launcher opens a page or starts an app in the session that runs it
// (docs/protocol.md §5.22). It must run as the student: as a service, only
// the helper holds the real one (internal/agentsvc). It never validates:
// internal/openapp checks the URL and the path first.
type Launcher interface {
	// Folders returns the roots and forbidden folders of this session's user.
	Folders() (AppFolders, error)
	// Resolve returns the final path of an existing regular file, following
	// links, or an error wrapping fs.ErrNotExist.
	Resolve(path string) (string, error)
	// OpenURL opens an http/https URL in the default browser.
	OpenURL(url string) error
	// StartApp starts the executable at path with no arguments, in its own
	// folder.
	StartApp(path string) error
}

// NewLauncher returns the launcher of the current session: Windows today;
// elsewhere ErrNotSupported, and the agent does not declare open.*.
func NewLauncher() (Launcher, error) { return newLauncher() }
