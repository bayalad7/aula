package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/roomplan"
	"github.com/bayalad7/aula/internal/session"
)

// --- The room: labels of the computers and the plan (prompt 10.15, §5.26) ---------
//
// The «Salón» module of the menu edits the plan of the open channel: each
// computer's label and its seat. The Alumnos view draws it («Salón»), every
// view shows the label instead of the hostname, and agents that declare
// computer.label receive their own label. The plan lives in
// Documentos\Aula\salones\canal-<n>.json; simulated students take seats only
// in memory. It keeps computers, never students (docs/privacidad.md 31 ter).

// roomState is the plan of the open channel and what each session was told.
type roomState struct {
	room *roomplan.Room
	// seen are the session IDs whose hello the plan recorded; told, the
	// label each session last received ("" sent or nothing to send). Both
	// guarded by App.mu and forgotten with the class.
	seen map[string]bool
	told map[string]string
}

// RoomRename is a computer that came back with another hostname and kept its
// seat and label (§5.26.2).
type RoomRename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// RoomView is what aula:room carries: the plan, and the computers renamed
// just now (only on the emission where it happened, for a notice).
type RoomView struct {
	roomplan.View
	Renamed []RoomRename `json:"renamed"`
}

// RoomImport is what an import would bring, for the confirmation: the file
// chosen and a summary. Path "" means the teacher cancelled.
type RoomImport struct {
	Path      string `json:"path"`
	Channel   string `json:"channel"`
	Rows      int    `json:"rows"`
	Cols      int    `json:"cols"`
	Computers int    `json:"computers"`
	Labeled   int    `json:"labeled"`
	Seated    int    `json:"seated"`
	// OtherChannel: the file is of another channel; importing it takes this one.
	OtherChannel bool `json:"otherChannel"`
}

// roomDir is Documentos\Aula\salones, "" when Documents is unknown (the plan
// then lives in memory).
func (a *App) roomDir() string {
	docs, err := a.documents()
	if err != nil || docs == "" {
		return ""
	}
	return filepath.Join(docs, platform.WorkFolderName, roomplan.Folder)
}

// openRoom loads the plan of channel when a class opens, and adds the
// computers the console remembers (prompt 10.19) to «Sin acomodar».
func (a *App) openRoom(channel string) {
	room, err := roomplan.Open(a.roomDir(), channel)
	if err != nil {
		a.log.Warn("room plan not read", "channel", channel, "err", err)
	}
	for _, m := range a.wake.machines.List(channel) {
		if _, err := room.Know(m.Hostname, m.MACs); err != nil {
			a.log.Warn("known computer not added to the room", "err", err)
		}
	}
	a.mu.Lock()
	a.roomSt = roomState{room: room, seen: map[string]bool{}, told: map[string]string{}}
	a.mu.Unlock()
	a.emitRoom(nil)
}

// currentRoom is the plan of the open channel; nil before the first class.
func (a *App) currentRoom() *roomplan.Room {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.roomSt.room
}

func (a *App) roomView(renamed []RoomRename) RoomView {
	a.mu.Lock()
	v := a.roomSt.roomViewLocked()
	a.mu.Unlock()
	if renamed != nil {
		v.Renamed = renamed
	}
	return v
}

// roomViewLocked is the plan with no renames; the caller holds App.mu (the
// Room has its own lock).
func (st roomState) roomViewLocked() RoomView {
	v := RoomView{Renamed: []RoomRename{}}
	if st.room != nil {
		v.View = st.room.View()
	} else {
		v.View = roomplan.View{Rows: roomplan.DefaultRows, Cols: roomplan.DefaultCols,
			Places: []roomplan.Place{}, Computers: []roomplan.ComputerView{}}
	}
	return v
}

func (a *App) emitRoom(renamed []RoomRename) { a.emit(EventRoom, a.roomView(renamed)) }

// Room returns the plan of the open channel.
func (a *App) Room() RoomView { return a.roomView(nil) }

// syncRoom records the hello of the real sessions that just connected
// (adding new computers to «Sin acomodar» or recognising a renamed one),
// seats the simulated students and sends each session its label.
func (a *App) syncRoom(class classroom) {
	room := a.currentRoom()
	if room == nil {
		return
	}
	sessions := class.Sessions()
	connected := map[string]bool{}
	for _, s := range sessions {
		if s.State == session.StateConnected {
			connected[strings.ToLower(s.Hostname)] = true
		}
	}
	changed := false
	var renamed []RoomRename
	for _, s := range sessions {
		if s.State != session.StateConnected {
			continue
		}
		a.mu.Lock()
		done := a.roomSt.seen[s.ID]
		a.roomSt.seen[s.ID] = true
		a.mu.Unlock()
		if done {
			continue
		}
		arr, err := room.Seen(s.Hostname, s.MACs, func(h string) bool { return connected[strings.ToLower(h)] })
		if err != nil {
			a.log.Warn("room plan not saved", "err", err)
			continue
		}
		if arr.RenamedFrom != "" {
			a.log.Info("computer renamed; it kept its seat", "from", arr.RenamedFrom, "to", s.Hostname)
			renamed = append(renamed, RoomRename{From: arr.RenamedFrom, To: s.Hostname})
		}
		changed = changed || arr.Changed
	}
	var names []string
	sims := simSessionsOf(class)
	for _, s := range sims {
		if s.State == session.StateConnected {
			names = append(names, s.Hostname)
		}
	}
	if room.SyncSims(names) {
		changed = true
	}
	a.tellLabels(class, sessions, false)
	a.tellLabels(class, sims, true)
	if changed {
		a.emitRoom(renamed)
	}
}

// tellLabels sends computer.label to the connected sessions that declared it
// and whose label differs from the one they were last told.
func (a *App) tellLabels(class classroom, sessions []session.Session, simulated bool) {
	room := a.currentRoom()
	if room == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, s := range sessions {
		if s.State != session.StateConnected || !s.Can(protocol.CapComputerLabel) {
			continue
		}
		label := room.Label(s.Hostname, simulated)
		a.mu.Lock()
		last, known := a.roomSt.told[s.ID]
		a.mu.Unlock()
		if last == label && (known || label == "") {
			continue // a fresh session with no label needs nothing (§5.26.1)
		}
		if _, err := class.Send(ctx, s.ID, protocol.ComputerLabel{Label: label}); err != nil {
			a.log.Debug("computer label not sent", "session", s.ID, "err", err)
			continue
		}
		a.mu.Lock()
		a.roomSt.told[s.ID] = label
		a.mu.Unlock()
	}
}

// retell sends the labels again after the plan changed.
func (a *App) retell() {
	class := a.currentClass()
	if class == nil {
		return
	}
	a.tellLabels(class, class.Sessions(), false)
	a.tellLabels(class, simSessionsOf(class), true)
}

// roomError turns an error of the plan into a coded one.
func roomError(err error) error {
	switch {
	case errors.Is(err, roomplan.ErrUnknown):
		return codedf(codeRoomComputerGone, "ese equipo ya no está en el salón: %w", err)
	case errors.Is(err, roomplan.ErrSpecialPlace):
		return codedf(codeRoomSpecialPlace, "ese lugar es el escritorio del profesor o un pasillo: quítalo primero")
	case errors.Is(err, roomplan.ErrOtherChannel):
		return codedf(codeRoomOtherChannel, "el archivo es del salón de otro canal: %w", err)
	case errors.Is(err, roomplan.ErrNewer):
		return codedf(codeRoomInvalid, "el archivo lo hizo una versión más nueva de Aula: %w", err)
	case errors.Is(err, roomplan.ErrInvalid):
		return codedf(codeRoomInvalid, "el salón no es válido: %w", err)
	}
	return codedf(codeRoomNotSaved, "no se pudo guardar el salón: %w", err)
}

// roomOp runs a change on the plan and returns the new one.
func (a *App) roomOp(f func(r *roomplan.Room) error) (RoomView, error) {
	room := a.currentRoom()
	if room == nil {
		return RoomView{}, errNoClass
	}
	if err := f(room); err != nil {
		return a.roomView(nil), roomError(err)
	}
	a.emitRoom(nil)
	return a.roomView(nil), nil
}

// ResizeRoom changes the rows and columns (1 to 12). Who is left outside
// goes to «Sin acomodar».
func (a *App) ResizeRoom(rows, cols int) (RoomView, error) {
	return a.roomOp(func(r *roomplan.Room) error { return r.Resize(rows, cols) })
}

// SetRoomPlace marks a cell as the teacher's desk ("teacher"), an aisle
// ("aisle") or empty ("").
func (a *App) SetRoomPlace(row, col int, kind string) (RoomView, error) {
	return a.roomOp(func(r *roomplan.Room) error {
		return r.SetPlace(roomplan.Cell{Row: row, Col: col}, roomplan.Kind(kind))
	})
}

// MoveComputer seats a computer at row, col (from 1); row 0 sends it to
// «Sin acomodar». A computer already there swaps with it.
func (a *App) MoveComputer(hostname string, simulated bool, row, col int) (RoomView, error) {
	var to *roomplan.Cell
	if row > 0 {
		to = &roomplan.Cell{Row: row, Col: col}
	}
	return a.roomOp(func(r *roomplan.Room) error { return r.Move(hostname, simulated, to) })
}

// ForgetRoomComputer takes a real computer out of the plan (one that left
// the room). If it connects again, it comes back to «Sin acomodar».
func (a *App) ForgetRoomComputer(hostname string) (RoomView, error) {
	v, err := a.roomOp(func(r *roomplan.Room) error { return r.Forget(hostname) })
	if err == nil {
		a.log.Info("computer taken out of the room")
	}
	return v, err
}

// SetComputerLabel changes the label of a computer (up to 40 characters; ""
// removes it) and tells its agent. Never logged: the hostname is enough.
func (a *App) SetComputerLabel(hostname string, simulated bool, label string) (RoomView, error) {
	label = strings.TrimSpace(label)
	if err := protocol.ValidComputerLabel(label); err != nil {
		return a.roomView(nil), codedf(codeLabelInvalid, "la etiqueta pasa de %d caracteres o tiene saltos de línea", protocol.MaxComputerLabel)
	}
	changed := false
	v, err := a.roomOp(func(r *roomplan.Room) error {
		var err error
		changed, err = r.SetLabel(hostname, simulated, label)
		return err
	})
	if err == nil && changed {
		a.retell()
	}
	return v, err
}

// ExportRoom writes the plan to a file the teacher chooses, to import it in
// the other consoles of the centre. It returns the path ("" if cancelled).
func (a *App) ExportRoom() (string, error) {
	room := a.currentRoom()
	if room == nil {
		return "", errNoClass
	}
	data, err := room.Export()
	if err != nil {
		return "", roomError(err)
	}
	path, err := a.chooseSave("Exportar el salón", "salon-"+roomplan.FileName(room.View().Channel))
	if err != nil {
		return "", codedf(codeRoomNotSaved, "no se pudo abrir el diálogo: %w", err)
	}
	if path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", codedf(codeRoomNotSaved, "no se pudo escribir el archivo: %w", err)
	}
	a.log.Info("room plan exported")
	return path, nil
}

// PreviewRoomImport lets the teacher choose a plan file and says what it
// brings, so the window can ask before replacing the room.
func (a *App) PreviewRoomImport() (RoomImport, error) {
	room := a.currentRoom()
	if room == nil {
		return RoomImport{}, errNoClass
	}
	path, err := a.chooseOpen("Importar un salón")
	if err != nil {
		return RoomImport{}, codedf(codeRoomInvalid, "no se pudo abrir el diálogo: %w", err)
	}
	if path == "" {
		return RoomImport{}, nil
	}
	p, err := readRoomFile(path)
	if err != nil {
		return RoomImport{}, roomError(err)
	}
	out := RoomImport{Path: path, Channel: p.Channel, Rows: p.Rows, Cols: p.Cols, Computers: len(p.Computers),
		OtherChannel: p.Channel != room.View().Channel}
	for _, c := range p.Computers {
		if c.Label != "" {
			out.Labeled++
		}
		if c.Seat != nil {
			out.Seated++
		}
	}
	return out, nil
}

func readRoomFile(path string) (roomplan.Plan, error) {
	info, err := os.Stat(path)
	if err != nil {
		return roomplan.Plan{}, err
	}
	if info.Size() > roomplan.MaxFileSize {
		return roomplan.Plan{}, roomplan.ErrInvalid
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return roomplan.Plan{}, err
	}
	return roomplan.Parse(data)
}

// ImportRoom replaces the plan with the file at path; otherChannel accepts a
// plan made for another channel. Each agent gets its new label.
func (a *App) ImportRoom(path string, otherChannel bool) (RoomView, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return a.roomView(nil), codedf(codeRoomInvalid, "no se pudo leer el archivo: %w", err)
	}
	v, err := a.roomOp(func(r *roomplan.Room) error { return r.Import(data, otherChannel) })
	if err == nil {
		a.log.Info("room plan imported", "computers", len(v.Computers))
		a.retell()
	}
	return v, err
}

// OpenRoomFolder shows Documentos\Aula\salones in Explorer.
func (a *App) OpenRoomFolder() error {
	dir := a.roomDir()
	if dir == "" {
		return codedf(codeRoomNotSaved, "no se encontró la carpeta Documentos")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return codedf(codeRoomNotSaved, "no se pudo crear la carpeta del salón: %w", err)
	}
	if err := a.openFolder(dir); err != nil {
		return codedf(codeRoomNotSaved, "no se pudo abrir la carpeta: %w", err)
	}
	return nil
}

// saveFileDialog and openFileDialog are the system dialogs (replaced in
// tests: never a window on the desktop).
func (a *App) saveFileDialog(title, name string) (string, error) {
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx == nil {
		return "", errors.New("la ventana no está lista")
	}
	return runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{Title: title, DefaultFilename: name,
		Filters: []runtime.FileFilter{{DisplayName: "Salón de Aula (*.json)", Pattern: "*.json"}}})
}

func (a *App) openFileDialog(title string) (string, error) {
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx == nil {
		return "", errors.New("la ventana no está lista")
	}
	return runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{Title: title,
		Filters: []runtime.FileFilter{{DisplayName: "Salón de Aula (*.json)", Pattern: "*.json"}}})
}
