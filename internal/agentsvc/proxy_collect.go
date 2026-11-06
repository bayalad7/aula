package agentsvc

import (
	"context"

	"github.com/bayalad7/aula/internal/platform"
)

// Calls of file.collect (docs/protocol.md §5.21): the helper reads the
// activity folder as the student, never as SYSTEM (docs/privacidad.md rule
// 60). The paths are checked by internal/collect in the service and again by
// the helper.
const (
	CallFolderCreate = "folder.create" // file.collect: create a folder of the work folder (rel)
	CallFolderList   = "folder.list"   // file.collect: FolderListArgs → FolderListResult
	CallFileRead     = "file.read"     // file.collect: FileReadArgs → FileReadResult
)

// FolderListArgs are the arguments of CallFolderList.
type FolderListArgs struct {
	Rel   string `json:"rel"`
	Depth int    `json:"depth"`
}

// FolderListResult is the reply of CallFolderList; Error as in
// FileWriteResult.
type FolderListResult struct {
	Files []platform.WorkFile `json:"files"`
	Error string              `json:"error,omitempty"`
}

// FileReadArgs are the arguments of CallFileRead.
type FileReadArgs struct {
	Rel string `json:"rel"`
	Max int64  `json:"max"`
}

// FileReadResult is the reply of CallFileRead; Error as in FileWriteResult.
type FileReadResult struct {
	Data  []byte `json:"data"`
	Error string `json:"error,omitempty"`
}

// FolderCreateResult is the reply of CallFolderCreate.
type FolderCreateResult struct {
	Error string `json:"error,omitempty"`
}

// WorkFolderReader reads the activity folder of a collection through the
// helper, in the student's session and with the student's identity. It
// implements platform.WorkFolderReader.
type WorkFolderReader struct{ Helper *Current }

func (w WorkFolderReader) call(name string, args, out any) error {
	l := w.Helper.Get()
	if l == nil {
		return ErrNoHelper
	}
	ctx, cancel := context.WithTimeout(context.Background(), fileCallTimeout)
	defer cancel()
	return l.Call(ctx, name, args, out)
}

// MkdirAll implements platform.WorkFolderReader.
func (w WorkFolderReader) MkdirAll(rel string) error {
	var r FolderCreateResult
	if err := w.call(CallFolderCreate, rel, &r); err != nil {
		return err
	}
	return fileWriteErr(r.Error)
}

// List implements platform.WorkFolderReader.
func (w WorkFolderReader) List(rel string, depth int) ([]platform.WorkFile, error) {
	var r FolderListResult
	if err := w.call(CallFolderList, FolderListArgs{Rel: rel, Depth: depth}, &r); err != nil {
		return nil, err
	}
	return r.Files, fileWriteErr(r.Error)
}

// Read implements platform.WorkFolderReader.
func (w WorkFolderReader) Read(rel string, max int64) ([]byte, error) {
	var r FileReadResult
	if err := w.call(CallFileRead, FileReadArgs{Rel: rel, Max: max}, &r); err != nil {
		return nil, err
	}
	if err := fileWriteErr(r.Error); err != nil {
		return nil, err
	}
	return r.Data, nil
}
