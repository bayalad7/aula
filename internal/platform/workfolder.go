package platform

import (
	"errors"
	"time"
)

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

// Errors of WorkFolderReader. They are wrapped: test with errors.Is.
var (
	// ErrFileTooLarge: the file is larger than the limit asked for.
	ErrFileTooLarge = errors.New("platform: file too large")
	// ErrOffline: the file lives only in the cloud (OneDrive files on
	// demand); reading it would download it, so it is not read.
	ErrOffline = errors.New("platform: file only in the cloud")
)

// WorkFile is one file of a work folder listing.
type WorkFile struct {
	Path    string    // relative to the listed folder, "/"-separated
	Size    int64     // bytes
	ModTime time.Time // last modification
	// Link: a symbolic link or junction (file or folder); it is listed but
	// never followed nor read.
	Link bool
	// Offline: only a placeholder is on the disk (see ErrOffline).
	Offline bool
}

// WorkFolderReader reads inside the student's work folder for file.collect
// (docs/protocol.md §5.21, docs/privacidad.md rules 60–63). Like WorkFolder
// it runs as the session user (as a service, through the helper), takes
// "/"-separated paths relative to the work folder already checked by
// internal/collect, never leaves the folder and never follows a link. It
// never deletes, moves nor changes a file.
type WorkFolderReader interface {
	// MkdirAll creates the folder rel and its parents, refusing a link on
	// the way (ErrLinkInPath).
	MkdirAll(rel string) error
	// List returns the files below the folder rel, up to depth levels (a
	// file directly in rel is level 1), sorted by path. Links are listed
	// with Link and not entered; Aula's own temporary files and the
	// system's hidden files are left out. A missing folder wraps
	// fs.ErrNotExist.
	List(rel string, depth int) ([]WorkFile, error)
	// Read returns the content of the file rel. A file larger than max
	// wraps ErrFileTooLarge, a link ErrLinkInPath, a cloud-only file
	// ErrOffline and a file another program locks ErrDenied.
	Read(rel string, max int64) ([]byte, error)
}

// NewWorkFolderReader returns the reader of the current user's work folder:
// Windows today; elsewhere it fails with ErrNotSupported and the agent does
// not declare file.collect.
func NewWorkFolderReader() (WorkFolderReader, error) { return newWorkFolderReader() }
