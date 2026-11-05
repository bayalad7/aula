package main

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/snapshot"
)

// --- Screenshot of a student (docs/protocol.md §5.17) -----------------------------

// TakeSnapshot asks one student, real or simulated, for a still screenshot.
// It also works while that student is remote-controlled: it does not end
// or pause the control. The image arrives with aula:snapshots and stays in
// memory until the teacher saves or discards it.
func (a *App) TakeSnapshot(sessionID string) (snapshot.Snapshot, error) {
	class := a.currentClass()
	if class == nil {
		return snapshot.Snapshot{}, errNoClass
	}
	s, simulated, ok := snapshotTarget(class, sessionID)
	if !ok {
		return snapshot.Snapshot{}, codedf(codeSnapshotUnsupported, "el equipo de este alumno no permite tomar una captura")
	}
	return a.requestSnapshot(class, s, simulated)
}

// TakeSnapshots asks every selected student whose computer can take one;
// the others are listed in Skipped.
func (a *App) TakeSnapshots(sessionIDs []string) (SnapshotBatch, error) {
	class := a.currentClass()
	if class == nil {
		return SnapshotBatch{}, errNoClass
	}
	out := SnapshotBatch{Snapshots: []snapshot.Snapshot{}, Skipped: []string{}}
	for _, id := range sessionIDs {
		s, simulated, ok := snapshotTarget(class, id)
		if !ok {
			if s.Hostname != "" {
				out.Skipped = append(out.Skipped, s.Hostname)
			}
			continue
		}
		snap, err := a.requestSnapshot(class, s, simulated)
		if err != nil {
			if len(out.Snapshots) == 0 {
				return out, err
			}
			break // too many in memory: keep what was asked
		}
		out.Snapshots = append(out.Snapshots, snap)
	}
	if len(out.Snapshots) == 0 {
		return out, codedf(codeSnapshotUnsupported, "ninguno de los alumnos seleccionados permite tomar una captura")
	}
	return out, nil
}

func (a *App) requestSnapshot(class classroom, s session.Session, simulated bool) (snapshot.Snapshot, error) {
	req, snap, err := a.snaps.Request(s.ID, s.Hostname, s.Username, simulated)
	if errors.Is(err, snapshot.ErrTooMany) {
		return snapshot.Snapshot{}, codedf(codeSnapshotTooMany, "hay %d capturas sin guardar ni descartar", snapshot.DefaultMaxHeld)
	}
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	envID, err := class.Send(ctx, s.ID, req)
	if err != nil {
		a.snaps.Unsent(snap.ID)
		snap, _ = a.snaps.Get(snap.ID)
		return snap, codedf(codeSnapshotGone, "no se pudo llegar al equipo de %s: %v", s.Hostname, err)
	}
	a.snaps.Sent(snap.ID, envID)
	return snap, nil
}

// snapshotTarget finds a connected student, real or simulated, that
// declared screen.snapshot. Without it, s still names the student if found.
func snapshotTarget(class classroom, id string) (s session.Session, simulated, ok bool) {
	if s, found := connectedSession(class.Sessions(), id); found {
		return s, false, s.Can(protocol.CapScreenSnapshot)
	}
	if s, found := connectedSession(simSessionsOf(class), id); found {
		return s, true, s.Can(protocol.CapScreenSnapshot)
	}
	return session.Session{}, false, false
}

// SnapshotImage returns the JPEG of a ready screenshot in Base64, for the
// preview. It never touches the disk.
func (a *App) SnapshotImage(id string) (string, error) {
	data, ok := a.snaps.Image(id)
	if !ok {
		return "", codedf(codeSnapshotGone, "esa captura ya no está en memoria")
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// SaveSnapshots writes the ready screenshots among ids under
// <Documentos>\Aula\capturas\{alumno}\{fecha}\ and returns them with their
// path. It is the only way a screenshot reaches the disk
// (docs/privacidad.md rule 45); the others are left as they are.
func (a *App) SaveSnapshots(ids []string) ([]snapshot.Snapshot, error) {
	root, err := a.snapshotRoot()
	if err != nil {
		return nil, err
	}
	out := []snapshot.Snapshot{}
	for _, id := range ids {
		if s, ok := a.snaps.Get(id); !ok || s.Status != snapshot.StatusReady {
			continue
		}
		s, err := a.snaps.Save(id, root)
		if err != nil {
			return out, codedf(codeSnapshotSaveFailed, "no se pudo guardar la captura en %s: %v", root, errors.Unwrap(err))
		}
		out = append(out, s)
	}
	if len(out) == 0 && len(ids) > 0 {
		return out, codedf(codeSnapshotGone, "esas capturas ya no están en memoria")
	}
	return out, nil
}

// DiscardSnapshots forgets screenshots and frees their images. Files
// already saved stay where they are.
func (a *App) DiscardSnapshots(ids []string) {
	for _, id := range ids {
		a.snaps.Discard(id)
	}
}

// SnapshotFolder says where saved screenshots go, for Ajustes › Acerca de.
func (a *App) SnapshotFolder() SnapshotFolder {
	root, err := a.snapshotRoot()
	if err != nil {
		return SnapshotFolder{Error: err.Error()}
	}
	info, err := os.Stat(root)
	return SnapshotFolder{Path: root, Exists: err == nil && info.IsDir()}
}

// OpenSnapshotFolder shows in Explorer the folder of screenshots, or the
// folder of one saved screenshot (path of the file or of a folder inside
// it). Nothing outside that folder can be opened.
func (a *App) OpenSnapshotFolder(path string) error {
	root, err := a.snapshotRoot()
	if err != nil {
		return err
	}
	dir := root
	if path != "" {
		dir = filepath.Clean(path)
		if info, err := os.Stat(dir); err == nil && !info.IsDir() {
			dir = filepath.Dir(dir)
		}
		if rel, err := filepath.Rel(root, dir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return codedf(codeSnapshotFolder, "solo se abre la carpeta de capturas")
		}
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return codedf(codeSnapshotFolder, "la carpeta %s todavía no existe: se crea al guardar la primera captura", dir)
	}
	if err := a.openFolder(dir); err != nil {
		return codedf(codeSnapshotFolder, "no se pudo abrir la carpeta: %v", err)
	}
	return nil
}

func (a *App) snapshotRoot() (string, error) {
	docs, err := a.documents()
	if err != nil {
		return "", codedf(codeSnapshotFolder, "no se encontró la carpeta Documentos: %v", err)
	}
	return snapshot.Root(docs), nil
}

// onSnapshotMessage takes the parts of a screenshot and the errors that
// may answer a request.
func (a *App) onSnapshotMessage(sessionID string, env protocol.Envelope) {
	switch p := env.Payload.(type) {
	case protocol.ScreenSnapshotResponse:
		a.snaps.Response(sessionID, p)
	case protocol.ErrorPayload:
		a.snaps.Error(sessionID, p)
	}
}

// syncSnapshots fails the pending screenshots of students who left.
func (a *App) syncSnapshots(class classroom) {
	live := map[string]bool{}
	for _, list := range [][]session.Session{class.Sessions(), simSessionsOf(class)} {
		for _, s := range list {
			if s.State == session.StateConnected {
				live[s.ID] = true
			}
		}
	}
	a.snaps.Gone(func(id string) bool { return live[id] })
}
