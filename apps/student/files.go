package main

import (
	"errors"
	"slices"

	"github.com/bayalad7/aula/internal/filepush"
	"github.com/bayalad7/aula/internal/platform"
)

// --- Files the teacher sends (docs/protocol.md §5.20) ----------------------------
//
// A card in the list per push: «Archivo recibido: datos.csv · guardado en
// Documentos\Aula\…», with «Abrir carpeta» and «Nuevo». It writes only in the
// work folder and never opens nor runs a file; it uses notify, never raise
// (docs/privacidad.md rules 55-58). Go sends states; the window words them.

// EventFiles carries Files whenever a file arrives, saved or not.
const EventFiles = "aula:files"

// Files is what arrived in this session of the agent, newest first.
type Files struct {
	Received []filepush.Received `json:"received"`
	// Folder is how the window names the work folder: always Documents\Aula
	// of the session's user (the path itself never reaches the window).
	Folder string `json:"folder"`
}

// enableFiles lets the agent save the teacher's files in the work folder of
// this user and declares file.push. Call it before start.
func (a *App) enableFiles(w platform.WorkFolder) {
	a.files.Writer, a.work = w, w
	a.hello.Capabilities = append(slices.Clone(a.hello.Capabilities), a.files.Capabilities()...)
}

// onFile shows a file that arrived, or could not be saved.
func (a *App) onFile(it filepush.Item) {
	a.emit(EventFiles, a.filesView(a.inbox.Add(it)))
	a.notify() // never raise: the student keeps the keyboard (prompt 19)
}

func (a *App) filesView(list []filepush.Received) Files {
	if list == nil {
		list = []filepush.Received{}
	}
	return Files{Received: list, Folder: platform.WorkFolderName}
}

// errNoFolder is what «Abrir carpeta» says when the push has no saved file
// or the work folder is gone.
var errNoFolder = errors.New("esa carpeta ya no está disponible")

// OpenReceivedFolder shows in Explorer where the files of a push were saved.
// The window passes only the push ID: the folder comes from what the agent
// saved, inside the work folder, never from the page.
func (a *App) OpenReceivedFolder(pushID string) error {
	if a.remote != nil {
		return a.forward("OpenReceivedFolder", nil, pushID)
	}
	rel, ok := a.inbox.Folder(pushID)
	if !ok || a.work == nil {
		return errNoFolder
	}
	if err := a.work.Open(rel); err != nil {
		a.log.Warn("cannot open the received folder", "push_id", pushID, "err", err)
		return errNoFolder
	}
	return nil
}
