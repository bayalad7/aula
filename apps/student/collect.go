package main

import (
	"slices"
	"strings"
	"time"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/collect"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// --- Collecting the student's work (docs/protocol.md §5.21) ----------------------
//
// A card per collection: «Recolección: Ejercicio 4 · Guarda tu ejercicio en
// Documentos\Aula\recolecciones\…», with «Abrir carpeta» and «Nuevo», and
// what was collected round by round. Before reading, a notice «El profesor
// solicitó recoger tu ejercicio en …»; after, «Tu profesor recogió…». It
// only reads that folder and never changes it; it uses notify, never raise
// (docs/privacidad.md rules 60–63). Go sends states; the window words them.

// EventCollections carries Collections whenever a collection opens, is
// asked for or ends a round.
const EventCollections = "aula:collections"

// Notice kinds of a collection; Ref is the collection ID.
const (
	NoticeCollectRequested = "collect-requested" // before reading (rule 61)
	NoticeCollectDone      = "collect-done"      // after the round (rule 62)
)

// Collections is what the window shows, oldest first.
type Collections struct {
	List []collect.Collection `json:"list"`
	// Folder is how the window names the work folder (Documents\Aula);
	// each collection's Folder is relative to it, "/"-separated.
	Folder string `json:"folder"`
}

// enableCollect lets the console collect from the activity folders of this
// user and declares file.collect. Call it before start.
func (a *App) enableCollect(r platform.WorkFolderReader) {
	a.collect.Reader = r
	a.hello.Capabilities = append(slices.Clone(a.hello.Capabilities), a.collect.Capabilities()...)
}

// onCollect shows a change of a collection. Never raise: the student keeps
// the keyboard (prompt 19).
func (a *App) onCollect(c collect.Collection, ev collect.Event) {
	a.emit(EventCollections, a.collectionsView())
	switch ev {
	case collect.EventRequested:
		a.onNotice(codedist.Notice{ID: protocol.NewID(), Kind: NoticeCollectRequested, Ref: c.ID,
			Level: protocol.LevelInfo, At: time.Now()})
	case collect.EventCollected:
		a.onNotice(codedist.Notice{ID: protocol.NewID(), Kind: NoticeCollectDone, Ref: c.ID,
			Level: protocol.LevelInfo, At: time.Now()})
	default:
		a.notify()
	}
}

func (a *App) collectionsView() Collections {
	list := a.collect.Collections()
	if list == nil {
		list = []collect.Collection{}
	}
	return Collections{List: list, Folder: platform.WorkFolderName}
}

// OpenCollectionFolder shows in Explorer the activity folder of a
// collection. The window passes only its ID: the folder comes from what the
// agent created, inside the work folder, never from the page.
func (a *App) OpenCollectionFolder(id string) error {
	if a.remote != nil {
		return a.forward("OpenCollectionFolder", nil, id)
	}
	rel, ok := a.collect.Folder(id)
	if !ok || a.work == nil || !strings.HasPrefix(rel, collect.BaseFolder+"/") {
		return errNoFolder
	}
	if err := a.work.Open(rel); err != nil {
		a.log.Warn("cannot open the collection folder", "collect_id", id, "err", err)
		return errNoFolder
	}
	return nil
}
