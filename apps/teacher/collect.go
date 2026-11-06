package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/bayalad7/aula/internal/collect"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// --- Collecting the students' work (docs/protocol.md §5.21) ---------------------------
//
// The «Recolecciones» tab of the Archivos module. «Crear carpeta en los
// equipos» opens a collection (today's date and an activity): every student
// who can (file.collect) gets the folder Documentos\Aula\recolecciones\
// <fecha>\<actividad>; who connects later gets it on arrival. «Recoger» asks
// them, four at a time, for what they saved there, and each round is saved in
// the teacher's Documentos\Aula\recolecciones\<fecha>\<actividad>\<alumno>\
// as it arrives (docs/privacidad.md rules 60–65). Per student: what arrived,
// when, «Abrir» and «Guardar como…». Names, paths and contents are never
// logged.

// partSendTimeout bounds sending one ack or one open to a session.
const partSendTimeout = 10 * time.Second

// EventCollections carries a CollectionsView when a collection changes.
const EventCollections = "aula:collections"

// CollectionsView is the tab: the collections of the class and where they
// are saved.
type CollectionsView struct {
	List   []collect.Summary `json:"list"` // newest first
	Folder string            `json:"folder"`
	// FolderError says why the teacher's folder is not available.
	FolderError string         `json:"folderError"`
	Today       string         `json:"today"` // the date a new collection takes
	Limits      collect.Limits `json:"limits"`
}

// CollectionRequest is what «Crear carpeta en los equipos» sends.
type CollectionRequest struct {
	Activity string `json:"activity"`
	Patterns string `json:"patterns"` // "*.py, *.txt"; empty: every file
}

// collectState is the console's side of file.collect.
type collectState struct {
	mu     sync.Mutex
	opened map[string]bool // sessions that got the open collections
	ctx    context.Context
	cancel context.CancelFunc
}

func (a *App) collectRoot() (string, error) {
	docs, err := a.documents()
	if err != nil {
		return "", err
	}
	return collect.Root(docs), nil
}

// collectCtx is the context of the rounds of the open class.
func (a *App) collectCtx() context.Context {
	a.coll.mu.Lock()
	defer a.coll.mu.Unlock()
	if a.coll.ctx == nil {
		a.coll.ctx, a.coll.cancel = context.WithCancel(context.Background())
	}
	return a.coll.ctx
}

// stopCollections forgets the collections of a class that closed; the
// folders stay.
func (a *App) stopCollections() {
	a.coll.mu.Lock()
	cancel := a.coll.cancel
	a.coll.ctx, a.coll.cancel, a.coll.opened = nil, nil, nil
	a.coll.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	a.collector.Forget()
	a.emitCollections()
}

// Collections returns the tab's state.
func (a *App) Collections() CollectionsView {
	v := CollectionsView{List: a.collector.List(), Today: a.now().Format(protocol.FileCollectDateLayout),
		Limits: collect.DefaultLimits}
	if root, err := a.collectRoot(); err != nil {
		v.FolderError = "no se encontró la carpeta Documentos"
	} else {
		v.Folder = root
	}
	for i := range v.List {
		if v.List[i].Students == nil {
			v.List[i].Students = []collect.StudentResult{}
		}
	}
	return v
}

func (a *App) emitCollections() { a.emit(EventCollections, a.Collections()) }

// collectStudents are the connected students, real and simulated, who can
// be collected from.
func (a *App) collectStudents(class classroom) (out []collect.Student, noCap int) {
	add := func(list []session.Session, sim bool) {
		for _, s := range list {
			if s.State != session.StateConnected {
				continue
			}
			if !s.Can(protocol.CapFileCollect) {
				noCap++
				continue
			}
			out = append(out, collect.Student{Session: s.ID, Hostname: s.Hostname, Username: s.Username, Simulated: sim})
		}
	}
	add(class.Sessions(), false)
	add(simSessionsOf(class), true)
	return out, noCap
}

// OpenCollection creates today's activity folder on every student who can
// get it, and the teacher's.
func (a *App) OpenCollection(req CollectionRequest) (collect.Summary, error) {
	class := a.currentClass()
	if class == nil {
		return collect.Summary{}, errNoClass
	}
	activity := strings.TrimSpace(req.Activity)
	if err := collect.CheckActivity(activity); err != nil {
		return collect.Summary{}, codedf(codeCollectActivity, "el nombre de la actividad no sirve como carpeta: %v", err)
	}
	patterns, err := collect.ParsePatterns(req.Patterns)
	if err != nil {
		return collect.Summary{}, codedf(codeCollectActivity, "los tipos de archivo no son válidos (ejemplo: *.py, *.txt)")
	}
	students, _ := a.collectStudents(class)
	date := a.now().Format(protocol.FileCollectDateLayout)
	id, err := a.collector.Open(a.collectCtx(), date, activity, patterns, collect.DefaultLimits, students)
	if err != nil {
		if errors.Is(err, collect.ErrInvalid) {
			return collect.Summary{}, codedf(codeCollectActivity, "%v", err)
		}
		return collect.Summary{}, codedf(codeCollectFolder, "no se pudo crear la carpeta de la actividad en Documentos\\Aula\\recolecciones: %v", err)
	}
	a.coll.mu.Lock()
	if a.coll.opened == nil {
		a.coll.opened = map[string]bool{}
	}
	for _, s := range students {
		a.coll.opened[s.Session] = true
	}
	a.coll.mu.Unlock()
	a.log.Info("collection opened", "collect_id", id, "students", len(students))
	s, _ := a.collector.Get(id)
	a.emitCollections()
	return s, nil
}

// Collect asks for a new round: every connected student who can, or only
// the students of keys (StudentResult.Key: «Recoger otra vez», «Recoger
// los que faltan»).
func (a *App) Collect(id string, keys []string) (collect.Summary, error) {
	class := a.currentClass()
	if class == nil {
		return collect.Summary{}, errNoClass
	}
	sum, ok := a.collector.Get(id)
	if !ok {
		return collect.Summary{}, codedf(codeCollectGone, "esa recolección ya no está abierta")
	}
	if sum.Round >= protocol.MaxFileCollectRound {
		return collect.Summary{}, codedf(codeCollectGone, "esa recolección llegó a su límite de rondas: abre otra")
	}
	students, _ := a.collectStudents(class)
	if len(keys) > 0 {
		want := map[string]bool{}
		for _, k := range keys {
			want[k] = true
		}
		var picked []collect.Student
		for _, s := range students {
			if want[s.Key()] {
				picked = append(picked, s)
			}
		}
		students = picked
	}
	if len(students) == 0 {
		return collect.Summary{}, codedf(codeCollectNoStudents, "ninguno de esos alumnos está conectado con un equipo que permita recoger")
	}
	if err := a.collector.Collect(a.collectCtx(), id, students); err != nil {
		return collect.Summary{}, codedf(codeCollectGone, "%v", err)
	}
	a.log.Info("collection round requested", "collect_id", id, "students", len(students))
	s, _ := a.collector.Get(id)
	return s, nil
}

// OpenCollectionFolder shows in Explorer the teacher's folder of one
// student of a collection, of the whole activity when key is empty, or the
// folder of every collection (Documentos\Aula\recolecciones, created if
// missing) when id is empty too.
func (a *App) OpenCollectionFolder(id, key string) error {
	dir := ""
	if id == "" {
		root, err := a.collectRoot()
		if err == nil {
			err = os.MkdirAll(root, 0o700)
		}
		if err != nil {
			return codedf(codeCollectFolder, "no se pudo preparar la carpeta de recolecciones: %v", err)
		}
		dir = root
	} else if key == "" {
		s, ok := a.collector.Get(id)
		if !ok {
			return codedf(codeCollectGone, "esa recolección ya no está abierta")
		}
		dir = s.Dir
	} else {
		var ok bool
		if dir, ok = a.collector.StudentDir(id, key); !ok {
			return codedf(codeCollectGone, "ese alumno no está en la recolección")
		}
	}
	if err := a.openFolder(dir); err != nil {
		return codedf(codeCollectFolder, "la carpeta todavía no existe o no se pudo abrir (se crea al llegar lo recogido): %v", err)
	}
	return nil
}

// SaveCollectionAs copies what was collected from one student to a folder
// the teacher picks («Guardar como…»); title is the dialog's title. It
// returns the new folder, or "" when the teacher cancels.
func (a *App) SaveCollectionAs(id, key, title string) (string, error) {
	dir, ok := a.collector.StudentDir(id, key)
	if !ok {
		return "", codedf(codeCollectGone, "ese alumno no está en la recolección")
	}
	parent, err := a.chooseFolder(title)
	if err != nil {
		return "", codedf(codeCollectFolder, "no se pudo abrir el diálogo: %v", err)
	}
	if parent == "" {
		return "", nil
	}
	dest, err := collect.CopyTo(dir, parent)
	if errors.Is(err, fs.ErrNotExist) {
		return "", codedf(codeCollectNothing, "todavía no se ha recogido nada de este alumno")
	}
	if err != nil {
		return "", codedf(codeCollectFolder, "no se pudo copiar: %v", err)
	}
	a.log.Info("collection copied", "collect_id", id)
	return dest, nil
}

// onCollectMessage takes file.collect.ack and the parts of collected files,
// from real and simulated students, and answers each file.
func (a *App) onCollectMessage(sessionID string, env protocol.Envelope) {
	switch p := env.Payload.(type) {
	case protocol.FileCollectAck:
		a.collector.Ack(sessionID, p)
	case protocol.FileCollectResponse:
		if reply := a.collector.Response(sessionID, env.ID, p); reply != nil {
			// Not from the reading goroutine: a slow write must not hold
			// the next messages of this student.
			go func() {
				ctx, cancel := context.WithTimeout(a.collectCtx(), partSendTimeout)
				defer cancel()
				if _, err := a.sendToSession(ctx, sessionID, reply); err != nil {
					a.log.Debug("collected file ack not sent", "conn", sessionID, "err", err)
				}
			}()
		}
	case protocol.ErrorPayload:
		a.collector.Error(sessionID, p)
	}
}

// syncCollections sends the open collections to who joined since, and
// abandons the rounds of who left.
func (a *App) syncCollections(class classroom) {
	students, _ := a.collectStudents(class)
	live := map[string]bool{}
	var fresh []collect.Student
	a.coll.mu.Lock()
	for _, s := range students {
		live[s.Session] = true
		if !a.coll.opened[s.Session] {
			fresh = append(fresh, s)
		}
	}
	for id := range a.coll.opened {
		if !live[id] {
			delete(a.coll.opened, id)
			a.collector.Gone(id)
		}
	}
	if len(a.collector.List()) > 0 {
		if a.coll.opened == nil {
			a.coll.opened = map[string]bool{}
		}
		for _, s := range fresh {
			a.coll.opened[s.Session] = true
		}
	} else {
		fresh = nil
	}
	a.coll.mu.Unlock()
	if len(fresh) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(a.collectCtx(), partSendTimeout)
	defer cancel()
	for _, s := range fresh {
		a.collector.Reopen(ctx, s)
	}
}

// openFolderDialog is the system dialog to pick a folder.
func (a *App) openFolderDialog(title string) (string, error) {
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx == nil {
		return "", errors.New("la ventana no está lista")
	}
	return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{Title: title, CanCreateDirectories: true})
}
