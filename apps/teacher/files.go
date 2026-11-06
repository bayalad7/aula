package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/filepush"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// --- Files and exercise templates (docs/protocol.md §5.20) ---------------------------
//
// The «Archivos» tab of the Código module: the teacher picks files or a
// template and sends them to the whole channel or the marked students. The
// acks go to «Últimos envíos» like the code («quién lo tiene, quién falta»);
// the sends go to the history, to send again; the templates are folders in
// <Documentos>\Aula\plantillas. The students' computers save them only in
// their work folder (docs/privacidad.md rules 55-58). Names, paths and
// contents are never logged.

// EventFiles carries a FilesView when the templates or the sends change.
const EventFiles = "aula:files"

// maxFileSends caps the sends remembered in the history.
const maxFileSends = 20

// FileChoice is a file the teacher picked, as the panel lists it.
type FileChoice struct {
	Path string `json:"path"`
	Rel  string `json:"rel"` // its name in the push
	Size int64  `json:"size"`
}

// FilesRequest is what the «Archivos» tab sends: chosen files, or a template
// (Template set: its files are read from its folder at send time).
type FilesRequest struct {
	Files     []filepush.Source `json:"files"`
	Template  string            `json:"template"`
	Dest      string            `json:"dest"`
	Overwrite bool              `json:"overwrite"`
	Targets   []string          `json:"targets"` // session IDs; empty: everyone
}

// FileSend is one send in the history, to send again.
type FileSend struct {
	ID        string            `json:"id"`
	Label     string            `json:"label"`
	Template  string            `json:"template"`
	Names     []string          `json:"names"`
	Size      int               `json:"size"`
	Dest      string            `json:"dest"`
	Overwrite bool              `json:"overwrite"`
	Times     int               `json:"times"`
	LastAt    string            `json:"lastAt"` // "15:04"
	Sources   []filepush.Source `json:"-"`
}

// FilesView is the templates and the sends of the class.
type FilesView struct {
	Templates []filepush.Template `json:"templates"`
	// TemplatesError says why the templates could not be listed; empty
	// when they could.
	TemplatesError string     `json:"templatesError"`
	Sends          []FileSend `json:"sends"` // newest first
}

// fileState is the console's side of file.push.
type fileState struct {
	mu     sync.Mutex
	sends  []FileSend        // oldest first
	pushes map[string]string // push ID → delivery ID
	byDel  map[string]string // delivery ID → send ID (for «Reenviar a los que faltan»)
	cancel context.CancelFunc
	ctx    context.Context
	seq    int
}

// filesCtx is the context of the deliveries of the open class: closing the
// class stops them.
func (a *App) filesCtx() context.Context {
	a.files.mu.Lock()
	defer a.files.mu.Unlock()
	if a.files.ctx == nil {
		a.files.ctx, a.files.cancel = context.WithCancel(context.Background())
	}
	return a.files.ctx
}

// stopFiles stops what is being sent (EndClass).
func (a *App) stopFiles() {
	a.files.mu.Lock()
	cancel := a.files.cancel
	a.files.ctx, a.files.cancel = nil, nil
	a.files.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) templates() filepush.Templates {
	docs, err := a.documents()
	if err != nil {
		return filepush.Templates{}
	}
	return filepush.Templates{Dir: filepath.Join(docs, platform.WorkFolderName, filepush.TemplatesFolder)}
}

// Files returns the templates and the sends of the class.
func (a *App) Files() FilesView {
	v := FilesView{Templates: []filepush.Template{}, Sends: []FileSend{}}
	t := a.templates()
	if t.Dir == "" {
		v.TemplatesError = "no se encontró la carpeta Documentos"
	} else if list, err := t.List(); err != nil {
		v.TemplatesError = err.Error()
	} else {
		v.Templates = list
	}
	a.files.mu.Lock()
	for i := len(a.files.sends) - 1; i >= 0; i-- {
		v.Sends = append(v.Sends, a.files.sends[i])
	}
	a.files.mu.Unlock()
	return v
}

func (a *App) emitFiles() { a.emit(EventFiles, a.Files()) }

// ChooseFiles opens the system dialog to pick files; title is the dialog's
// title, from textos.es-MX.js. Cancelling returns no files and no error.
func (a *App) ChooseFiles(title string) ([]FileChoice, error) {
	paths, err := a.chooseFiles(title)
	if err != nil {
		return nil, coded(codePushFileGone, err)
	}
	out := []FileChoice{}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		out = append(out, FileChoice{Path: p, Rel: filepath.Base(p), Size: info.Size()})
	}
	return out, nil
}

// SendFiles sends the chosen files, or a template, to every student who can
// receive them, or only to req.Targets.
func (a *App) SendFiles(req FilesRequest) (codedist.Delivery, error) {
	class := a.currentClass()
	if class == nil {
		return codedist.Delivery{}, errNoClass
	}
	sources, label := req.Files, ""
	if req.Template != "" {
		var err error
		if sources, err = a.templates().Sources(req.Template); err != nil {
			return codedist.Delivery{}, templateError(err)
		}
		label = req.Template
	}
	push, err := filepush.NewPush(label, req.Dest, sources, req.Overwrite)
	if err != nil {
		return codedist.Delivery{}, pushError(err)
	}
	send := a.rememberSend(push, req.Template)
	return a.deliverPush(class, push, send.ID, req.Targets)
}

// ResendFiles sends again a send of the history, to everyone, reading its
// files again (they may have changed since).
func (a *App) ResendFiles(sendID string) (codedist.Delivery, error) {
	class := a.currentClass()
	if class == nil {
		return codedist.Delivery{}, errNoClass
	}
	s, ok := a.fileSend(sendID)
	if !ok {
		return codedist.Delivery{}, codedf(codeDeliveryGone, "ese envío ya no está en el historial")
	}
	push, err := a.pushAgain(s)
	if err != nil {
		return codedist.Delivery{}, err
	}
	a.rememberSend(push, s.Template)
	return a.deliverPush(class, push, sendID, nil)
}

// pushAgain reads a send's files again.
func (a *App) pushAgain(s FileSend) (*filepush.Push, error) {
	sources := s.Sources
	if s.Template != "" {
		var err error
		if sources, err = a.templates().Sources(s.Template); err != nil {
			return nil, templateError(err)
		}
	}
	push, err := filepush.NewPush(s.Label, s.Dest, sources, s.Overwrite)
	if err != nil {
		return nil, pushError(err)
	}
	return push, nil
}

// deliverPush records the delivery and starts sending. Only who declared
// file.push gets it; the rest count as «no puede recibirlo».
func (a *App) deliverPush(class classroom, push *filepush.Push, sendID string, targets []string) (codedist.Delivery, error) {
	real, sims := class.Sessions(), simSessionsOf(class)
	sent, simSent := map[string]string{}, map[string]string{}
	noCap, gone := 0, 0
	pick := func(list []session.Session, into map[string]string) {
		for _, s := range list {
			if s.State != session.StateConnected || (len(targets) > 0 && !slices.Contains(targets, s.ID)) {
				continue
			}
			if !s.Can(protocol.CapFilePush) {
				noCap++
				continue
			}
			into[s.ID] = fileRef(push.ID, s.ID)
		}
	}
	pick(real, sent)
	pick(sims, simSent)
	for _, id := range targets {
		if !slices.ContainsFunc(append(slices.Clone(real), sims...), func(s session.Session) bool {
			return s.ID == id && s.State == session.StateConnected
		}) {
			gone++
		}
	}
	if len(sent)+len(simSent) == 0 {
		return codedist.Delivery{}, codedf(codePushUnsupported, "ninguno de los alumnos elegidos puede recibir archivos")
	}
	a.sendMu.Lock()
	a.mu.Lock()
	d := a.deliveries.Start("file", fileLabel(push), sent, gone, noCap, a.now())
	if len(simSent) > 0 {
		d, _ = a.deliveries.AddSimulated(d.ID, simSent)
	}
	a.mu.Unlock()
	a.files.mu.Lock()
	if a.files.pushes == nil {
		a.files.pushes, a.files.byDel = map[string]string{}, map[string]string{}
	}
	a.files.pushes[push.ID] = d.ID
	a.files.byDel[d.ID] = sendID
	a.files.mu.Unlock()
	a.sendMu.Unlock()

	sessions := make([]string, 0, len(sent)+len(simSent))
	for id := range sent {
		sessions = append(sessions, id)
	}
	for id := range simSent {
		sessions = append(sessions, id)
	}
	slices.Sort(sessions)
	a.log.Info("file push", "push_id", push.ID, "files", len(push.Files), "bytes", push.Size(),
		"sent", len(sent), "simulated_sent", len(simSent), "no_capability", noCap)
	a.pusher.Deliver(a.filesCtx(), push, sessions)
	a.emitDeliveries()
	a.emitFiles()
	return d, nil
}

// resendFiles is «Reenviar a los que faltan» of a file delivery: the same
// send, read again, to who has not got it.
func (a *App) resendFiles(class classroom, d codedist.Delivery) (codedist.Delivery, error) {
	a.files.mu.Lock()
	sendID := a.files.byDel[d.ID]
	a.files.mu.Unlock()
	s, ok := a.fileSend(sendID)
	if !ok {
		return codedist.Delivery{}, codedf(codeDeliveryGone, "ese envío ya no está entre los recientes")
	}
	push, err := a.pushAgain(s)
	if err != nil {
		return codedist.Delivery{}, err
	}
	again := slices.Clone(d.Pending)
	for _, r := range d.Rejected {
		again = append(again, r.Session)
	}
	connected := map[string]bool{}
	for _, s := range append(class.Sessions(), simSessionsOf(class)...) {
		if s.State == session.StateConnected && s.Can(protocol.CapFilePush) {
			connected[s.ID] = true
		}
	}
	sent := map[string]string{}
	var sessions []string
	for _, id := range again {
		if connected[id] {
			sent[id] = fileRef(push.ID, id)
			sessions = append(sessions, id)
		}
	}
	a.sendMu.Lock()
	a.mu.Lock()
	d, _ = a.deliveries.Resent(d.ID, sent)
	a.mu.Unlock()
	a.files.mu.Lock()
	a.files.pushes[push.ID] = d.ID
	a.files.mu.Unlock()
	a.sendMu.Unlock()
	a.log.Info("file push resend", "push_id", push.ID, "delivery", d.ID, "sent", len(sent))
	a.pusher.Deliver(a.filesCtx(), push, sessions)
	a.emitDeliveries()
	return d, nil
}

// onFileOutcome counts how a push ended for one session.
func (a *App) onFileOutcome(o filepush.Outcome) {
	a.files.mu.Lock()
	_, ok := a.files.pushes[o.PushID]
	a.files.mu.Unlock()
	if !ok {
		return
	}
	ref := fileRef(o.PushID, o.Session)
	a.sendMu.Lock()
	a.mu.Lock()
	var changed bool
	if o.OK() {
		_, changed = a.deliveries.Ack(ref)
	} else {
		_, changed = a.deliveries.Reject(ref, o.Reason)
	}
	a.mu.Unlock()
	a.sendMu.Unlock()
	if changed {
		a.emitDeliveries()
	}
}

// onFileMessage takes file.push.ack and the errors, from real and simulated
// students.
func (a *App) onFileMessage(sessionID string, env protocol.Envelope) {
	switch p := env.Payload.(type) {
	case protocol.FilePushAck:
		a.pusher.Ack(sessionID, p)
	case protocol.ErrorPayload:
		a.pusher.Error(sessionID, p)
	}
}

// SaveTemplate makes a template from the chosen files, under name.
func (a *App) SaveTemplate(name string, files []filepush.Source) (FilesView, error) {
	t := a.templates()
	if t.Dir == "" {
		return FilesView{}, codedf(codeTemplateInvalid, "no se encontró la carpeta Documentos")
	}
	if err := t.Save(strings.TrimSpace(name), files); err != nil {
		return FilesView{}, templateError(err)
	}
	a.log.Info("template saved", "files", len(files))
	v := a.Files()
	a.emit(EventFiles, v)
	return v, nil
}

// OpenTemplates shows a template's folder in Explorer, or the folder of all
// templates when name is empty (created if missing).
func (a *App) OpenTemplates(name string) error {
	t := a.templates()
	if t.Dir == "" {
		return codedf(codeTemplateInvalid, "no se encontró la carpeta Documentos")
	}
	dir := t.Dir
	if name != "" {
		var err error
		if dir, err = t.Path(name); err != nil {
			return templateError(err)
		}
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		return codedf(codeTemplateInvalid, "no se pudo crear la carpeta de plantillas: %w", err)
	}
	if err := a.openFolder(dir); err != nil {
		return codedf(codeTemplateInvalid, "no se pudo abrir la carpeta: %w", err)
	}
	return nil
}

// rememberSend puts a send at the top of the history: the same files to the
// same place count as the same send.
func (a *App) rememberSend(push *filepush.Push, template string) FileSend {
	a.files.mu.Lock()
	defer a.files.mu.Unlock()
	names := push.Names()
	same := func(s FileSend) bool {
		return s.Template == template && s.Dest == push.Dest && s.Overwrite == push.Overwrite && slices.Equal(s.Names, names) &&
			(template != "" || slices.Equal(s.Sources, push.Sources))
	}
	s := FileSend{Label: push.Label, Template: template, Names: names, Size: push.Size(), Dest: push.Dest,
		Overwrite: push.Overwrite, Sources: push.Sources}
	if i := slices.IndexFunc(a.files.sends, same); i >= 0 {
		s.ID, s.Times = a.files.sends[i].ID, a.files.sends[i].Times
		a.files.sends = slices.Delete(a.files.sends, i, i+1)
	} else {
		a.files.seq++
		s.ID = "f" + strconv.Itoa(a.files.seq)
	}
	s.Times++
	s.LastAt = a.now().Format("15:04")
	a.files.sends = append(a.files.sends, s)
	if len(a.files.sends) > maxFileSends {
		a.files.sends = slices.Clone(a.files.sends[1:])
	}
	return s
}

func (a *App) fileSend(id string) (FileSend, bool) {
	a.files.mu.Lock()
	defer a.files.mu.Unlock()
	i := slices.IndexFunc(a.files.sends, func(s FileSend) bool { return s.ID == id })
	if i < 0 {
		return FileSend{}, false
	}
	return a.files.sends[i], true
}

// fileRef is the reference of a push to one session in Deliveries: a file
// delivery is counted per session, once every file is acked.
func fileRef(pushID, sessionID string) string { return "file:" + pushID + ":" + sessionID }

// fileLabel names a push in «Últimos envíos».
func fileLabel(p *filepush.Push) string {
	switch {
	case p.Label != "":
		return fmt.Sprintf("Archivos: %s (%d)", p.Label, len(p.Files))
	case len(p.Files) == 1:
		return "Archivo: " + p.Files[0].Name
	default:
		return fmt.Sprintf("Archivos: %s y %d más", p.Files[0].Name, len(p.Files)-1)
	}
}

// pushError gives NewPush's errors their code and a Spanish detail.
func pushError(err error) error {
	file := ""
	var fe *filepush.FileError
	if errors.As(err, &fe) {
		file = fe.File + ": "
	}
	switch {
	case errors.Is(err, filepush.ErrNoFiles):
		return codedf(codePushNoFiles, "no hay archivos para enviar")
	case errors.Is(err, filepush.ErrTooManyFiles), errors.Is(err, filepush.ErrFileTooLarge), errors.Is(err, filepush.ErrPushTooLarge):
		return codedf(codePushTooLarge, "%shasta %d archivos, de %d MB cada uno y %d MB en total", file,
			protocol.MaxFilePushFiles, protocol.MaxFilePushBytes>>20, protocol.MaxFilePushTotal>>20)
	case errors.Is(err, filepush.ErrInvalidPath), errors.Is(err, filepush.ErrDuplicate):
		return codedf(codePushPathInvalid, "%snombre o carpeta no válidos", file)
	case errors.Is(err, filepush.ErrUnreadable):
		return codedf(codePushFileGone, "%sno se pudo leer", file)
	}
	return coded(codePushFileGone, err)
}

// templateError gives Templates' errors their code.
func templateError(err error) error {
	switch {
	case errors.Is(err, filepush.ErrTemplateExists):
		return codedf(codeTemplateExists, "ya hay una plantilla con ese nombre")
	case errors.Is(err, filepush.ErrTemplateMissing):
		return codedf(codeTemplateGone, "esa plantilla ya no está en la carpeta de plantillas")
	case errors.Is(err, filepush.ErrInvalidPath), errors.Is(err, filepush.ErrNoFiles):
		return codedf(codeTemplateInvalid, "el nombre de la plantilla o sus archivos no son válidos")
	}
	return pushError(err)
}

// sendToSession is how the pusher reaches one session of the open class,
// real or simulated.
func (a *App) sendToSession(ctx context.Context, sessionID string, p protocol.Payload) (string, error) {
	class := a.currentClass()
	if class == nil {
		return "", errNoClass
	}
	return class.Send(ctx, sessionID, p)
}

// openFilesDialog is the system dialog to pick files.
func (a *App) openFilesDialog(title string) ([]string, error) {
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx == nil {
		return nil, errors.New("la ventana no está lista")
	}
	return runtime.OpenMultipleFilesDialog(ctx, runtime.OpenDialogOptions{Title: title})
}
