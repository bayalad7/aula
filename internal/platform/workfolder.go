package platform

import "errors"

// WorkFolderName is the student's work folder inside Documents: the only
// place Aula writes the files the teacher sends (docs/privacidad.md rule 55).
const WorkFolderName = "Aula"

// Errors of WorkFolder.Create. They are wrapped: test with errors.Is.
var (
	// ErrFileExists: overwrite was false and the name is taken.
	ErrFileExists = errors.New("platform: file already exists")
	// ErrLinkInPath: a folder on the way is a symbolic link or a junction.
	ErrLinkInPath = errors.New("platform: path crosses a link")
	// ErrNoSpace: the disk is full.
	ErrNoSpace = errors.New("platform: disk full")
	// ErrDenied: the OS refused the write (permissions, file in use).
	ErrDenied = errors.New("platform: access denied")
	// ErrPathTooLong: the absolute path passes the OS limit.
	ErrPathTooLong = errors.New("platform: path too long")
)

// WorkFolder is the student's work folder, Documents\Aula of the session
// user (docs/protocol.md §5.20). It must run as that user: as a service, only
// the helper holds the real one (internal/agentsvc), so every file belongs to
// the student and never to SYSTEM.
//
// Paths are relative to the folder, "/"-separated and already checked by
// internal/filepush; WorkFolder still refuses to leave the folder and never
// follows links inside it.
type WorkFolder interface {
	// Root returns the folder's absolute path, creating it if missing.
	Root() (string, error)
	// Create writes data at rel, creating the folders on the way, through a
	// temporary file so a partial file never shows. With overwrite false it
	// never replaces anything (ErrFileExists); with overwrite true it reports
	// whether a file was replaced.
	Create(rel string, data []byte, overwrite bool) (replaced bool, err error)
	// Open shows the folder rel ("" = the work folder) in the file manager.
	Open(rel string) error
}

// NewWorkFolder returns the work folder of the current user: Windows today;
// elsewhere it fails with ErrNotSupported and the agent does not declare
// file.push.
func NewWorkFolder() (WorkFolder, error) { return newWorkFolder() }
