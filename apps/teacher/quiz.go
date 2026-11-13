package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/poll"
	"github.com/bayalad7/aula/internal/protocol"
)

// --- Quizzes (docs/protocol.md §5.23.1; privacidad, reglas 87-91) -------------------
//
// The «Cuestionarios» view of the menu: the saved quizzes (the teacher's
// material, in Documentos\Aula\cuestionarios), their editor and «Lanzar».
// A launched quiz is the open poll of the class: its results live in the
// «Encuestas» section of «Participación» and its franja is the poll's. The
// title, the questions, the answers and the scores are never logged.

// QuizzesView is what the «Cuestionarios» view lists.
type QuizzesView struct {
	Saved []poll.Saved `json:"saved"`
	// Folder is where they are saved, to say so under the list.
	Folder string `json:"folder"`
	// LastSaved is the id SaveQuiz just wrote, to select it.
	LastSaved string `json:"lastSaved"`
}

// quizLibrary is the folder of the saved quizzes, empty when Documents is
// unknown.
func (a *App) quizLibrary() poll.Library {
	docs, err := a.documents()
	if err != nil || docs == "" {
		return poll.Library{}
	}
	return poll.Library{Dir: filepath.Join(docs, platform.WorkFolderName, poll.QuizzesFolder)}
}

// Quizzes lists the saved quizzes.
func (a *App) Quizzes() (QuizzesView, error) {
	lib := a.quizLibrary()
	v := QuizzesView{Saved: []poll.Saved{}, Folder: lib.Dir}
	if lib.Dir == "" {
		return v, nil
	}
	list, err := lib.List()
	if err != nil {
		return v, codedf(codeQuizNotSaved, "no se pudo leer la carpeta de cuestionarios: %w", err)
	}
	v.Saved = list
	return v, nil
}

// SaveQuiz writes a quiz: over id, or as a new one when id is empty.
func (a *App) SaveQuiz(id string, q poll.Quiz) (QuizzesView, error) {
	lib := a.quizLibrary()
	if lib.Dir == "" {
		return QuizzesView{}, codedf(codeQuizNotSaved, "no se encontró la carpeta Documentos")
	}
	saved, err := lib.Save(id, q)
	switch {
	case errors.Is(err, poll.ErrInvalid):
		return QuizzesView{}, invalidQuiz(err)
	case err != nil:
		return QuizzesView{}, codedf(codeQuizNotSaved, "no se pudo guardar el cuestionario: %w", err)
	}
	// Not the id: it comes from the title (rule 91).
	a.log.Info("quiz saved", "questions", len(q.Questions), "new", id == "")
	v, err := a.Quizzes()
	v.LastSaved = saved
	return v, err
}

// DeleteQuiz removes a saved quiz from its folder.
func (a *App) DeleteQuiz(id string) (QuizzesView, error) {
	switch err := a.quizLibrary().Delete(id); {
	case errors.Is(err, poll.ErrQuizMissing):
		return QuizzesView{}, codedf(codeQuizGone, "ese cuestionario ya no está en la carpeta de cuestionarios")
	case err != nil:
		return QuizzesView{}, codedf(codeQuizNotSaved, "no se pudo eliminar el cuestionario: %w", err)
	}
	a.log.Info("quiz deleted")
	return a.Quizzes()
}

// OpenQuiz launches a saved quiz to the class or the marked students: it
// closes the poll in force and, with a time, starts the timer that closes it
// at zero.
func (a *App) OpenQuiz(id string, hosts []string) (poll.View, error) {
	class := a.currentClass()
	if class == nil {
		return poll.View{}, errNoClass
	}
	q, err := a.quizLibrary().Load(id)
	switch {
	case errors.Is(err, poll.ErrQuizMissing):
		return poll.View{}, codedf(codeQuizGone, "ese cuestionario ya no está en la carpeta de cuestionarios")
	case err != nil:
		return poll.View{}, invalidQuiz(err)
	}
	if !targetsCan(class, hosts, protocol.CapPollQuiz) {
		return poll.View{}, codedf(codeNoQuizTargets, "ninguno de los alumnos elegidos puede mostrar cuestionarios")
	}
	a.stopQuizTimer() // the one of a previous quiz, if it still runs
	pollID, err := a.polls.OpenQuiz(q, hosts)
	if err != nil {
		return poll.View{}, invalidQuiz(err)
	}
	a.log.Info("quiz opened", "poll", pollID, "questions", len(q.Questions), "minutes", q.Minutes, "shuffle", q.Shuffle, "hosts", len(hosts))
	if q.Minutes > 0 && targetsCan(class, hosts, protocol.CapTimer) {
		timerID, err := a.timer.Start(timerLabel(q.Title), "", q.Minutes*60, hosts)
		if err != nil {
			a.log.Warn("quiz timer not started", "poll", pollID, "err", err)
		} else {
			a.mu.Lock()
			a.quizPoll, a.quizTimerID = pollID, timerID
			a.mu.Unlock()
			a.timerChanged(class)
		}
	}
	return a.pollsChanged(class), nil
}

// closeQuizAtZero closes the quiz whose timer reached zero (called when the
// timer of the class ends).
func (a *App) closeQuizAtZero() {
	a.mu.Lock()
	pollID, timerID := a.quizPoll, a.quizTimerID
	a.mu.Unlock()
	if pollID == "" {
		return
	}
	if left, ok := a.timer.Remaining(); !ok || left > 0 || a.timer.Snapshot(nil, nil).TimerID != timerID {
		return // extended, stopped or replaced: the teacher decides
	}
	cur := a.polls.Snapshot(nil, nil).Current
	if cur == nil || cur.PollID != pollID {
		return
	}
	if _, err := a.ClosePoll(); err == nil {
		a.log.Info("quiz closed by its timer", "poll", pollID)
	}
}

// stopQuizTimer stops the timer of the quiz in force, if it is still the
// one running, and forgets it.
func (a *App) stopQuizTimer() {
	a.mu.Lock()
	timerID := a.quizTimerID
	a.quizPoll, a.quizTimerID = "", ""
	a.mu.Unlock()
	if timerID == "" || a.timer.Snapshot(nil, nil).TimerID != timerID {
		return
	}
	if class := a.currentClass(); class != nil && a.timer.Stop() == nil {
		a.timerChanged(class)
	}
}

// ExportQuizGrades writes the grades of a closed quiz to
// Documentos\Aula\calificaciones\<fecha>\<título>.csv (rule 89) and returns
// the path. Only real students; never on its own.
func (a *App) ExportQuizGrades(pollID string) (string, error) {
	data, title, err := a.polls.Grades(pollID)
	switch {
	case errors.Is(err, poll.ErrGone), errors.Is(err, poll.ErrNotQuiz):
		return "", codedf(codePollGone, "ese cuestionario ya no está en la clase")
	case errors.Is(err, poll.ErrStillOpen):
		return "", codedf(codeGradesNotExported, "cierra el cuestionario antes de exportar sus calificaciones")
	case err != nil:
		return "", codedf(codeGradesNotExported, "no se pudieron preparar las calificaciones: %w", err)
	}
	docs, err := a.documents()
	if err != nil || docs == "" {
		return "", codedf(codeGradesNotExported, "no se encontró la carpeta Documentos")
	}
	dir := filepath.Join(docs, platform.WorkFolderName, poll.GradesFolder, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", codedf(codeGradesNotExported, "no se pudo crear la carpeta de calificaciones: %w", err)
	}
	path, err := writeNew(dir, poll.GradesName(title), data)
	if err != nil {
		return "", codedf(codeGradesNotExported, "no se pudo guardar el archivo de calificaciones: %w", err)
	}
	a.log.Info("quiz grades exported", "poll", pollID)
	return path, nil
}

// OpenQuizFolder shows the folder of the saved quizzes ("cuestionarios") or
// of the exported grades ("calificaciones") in Explorer, created if missing.
func (a *App) OpenQuizFolder(which string) error {
	docs, err := a.documents()
	if err != nil || docs == "" || (which != poll.QuizzesFolder && which != poll.GradesFolder) {
		return codedf(codeQuizNotSaved, "no se encontró la carpeta Documentos")
	}
	dir := filepath.Join(docs, platform.WorkFolderName, which)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return codedf(codeQuizNotSaved, "no se pudo crear la carpeta: %w", err)
	}
	if err := a.openFolder(dir); err != nil {
		return codedf(codeQuizNotSaved, "no se pudo abrir la carpeta: %w", err)
	}
	return nil
}

// writeNew writes data to name in dir without replacing a file: «name (2)»
// and so on when it exists.
func writeNew(dir, name string, data []byte) (string, error) {
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]
	for n := 1; n < 1000; n++ {
		try := name
		if n > 1 {
			try = fmt.Sprintf("%s (%d)%s", base, n, ext)
		}
		path := filepath.Join(dir, try)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, werr := f.Write(data)
		if err := errors.Join(werr, f.Close()); err != nil {
			_ = os.Remove(path)
			return "", err
		}
		return path, nil
	}
	return "", errors.New("too many files with that name")
}

// timerLabel fits the quiz title in the timer's name (it is shorter).
func timerLabel(title string) string {
	r := []rune(title)
	if len(r) <= protocol.MaxTimerLabel {
		return title
	}
	return strings.TrimSpace(string(r[:protocol.MaxTimerLabel-1])) + "…"
}

// invalidQuiz is the coded error of a quiz that does not fit the limits.
func invalidQuiz(err error) error {
	return codedf(codeInvalidQuiz, "cuestionario no válido (título de hasta %d caracteres; de 1 a %d preguntas de hasta %d, con su respuesta correcta, de 2 a %d opciones y de 1 a %d puntos; hasta %d minutos): %w",
		protocol.MaxQuizTitle, protocol.MaxQuizQuestions, protocol.MaxPollQuestion, protocol.MaxPollOptions,
		protocol.MaxQuizPoints, poll.MaxQuizMinutes, err)
}

// targetsCan reports whether a connected student among hosts (all when
// empty), real or simulated, declared c.
func targetsCan(class classroom, hosts []string, c protocol.Capability) bool {
	return anyCan(targetsOf(append(class.Sessions(), simSessionsOf(class)...), hosts), c)
}
