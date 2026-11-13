package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/classtimer"
	"github.com/bayalad7/aula/internal/poll"
	"github.com/bayalad7/aula/internal/protocol"
)

// quizHarness has two agents that show quizzes, one with polls only (an
// older Aula) and a Documents folder of its own.
func quizHarness(t *testing.T) (*harness, string, *bytes.Buffer) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapMsgShow, protocol.CapPoll, protocol.CapPollQuiz, protocol.CapTimer),
		student("s2", "PC02", protocol.CapMsgShow, protocol.CapPoll, protocol.CapPollQuiz, protocol.CapTimer),
		student("s3", "PC03", protocol.CapMsgShow, protocol.CapPoll))
	docs := t.TempDir()
	h.app.documents = func() (string, error) { return docs, nil }
	h.app.openFolder = func(p string) error { h.rec.emit("opened", p); return nil }
	logs := &bytes.Buffer{}
	h.app.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return h, docs, logs
}

func secretQuiz() poll.Quiz {
	return poll.Quiz{Title: "Examen secreto", Questions: []poll.QuizQuestion{
		{Text: "¿Pregunta secreta?", Kind: protocol.PollKindChoice, Options: []string{"opción secreta", "otra"}, Correct: []int{0}, Points: 2},
		{Text: "¿Sí secreto?", Kind: protocol.PollKindYesNo, Correct: []int{1}, Points: 1},
	}}
}

// Saved quizzes live in Documentos\Aula\cuestionarios: save, list, edit and
// delete; an invalid one says what is missing.
func TestQuizLibraryBindings(t *testing.T) {
	h, docs, logs := quizHarness(t)
	v, err := h.app.SaveQuiz("", secretQuiz())
	if err != nil || v.LastSaved != "examen-secreto" || len(v.Saved) != 1 {
		t.Fatalf("SaveQuiz = %+v, %v", v, err)
	}
	if want := filepath.Join(docs, "Aula", poll.QuizzesFolder); v.Folder != want {
		t.Errorf("folder = %q, want %q", v.Folder, want)
	}
	q := secretQuiz()
	q.Minutes = 5
	if v, err = h.app.SaveQuiz("examen-secreto", q); err != nil || v.Saved[0].Quiz.Minutes != 5 || len(v.Saved) != 1 {
		t.Fatalf("edit = %+v, %v", v, err)
	}
	bad := secretQuiz()
	bad.Questions[0].Correct = nil
	if _, err := h.app.SaveQuiz("", bad); errorCode(err) != codeInvalidQuiz {
		t.Errorf("no correct option = %v", err)
	}
	if err := h.app.OpenQuizFolder(poll.QuizzesFolder); err != nil {
		t.Fatal(err)
	}
	if err := h.app.OpenQuizFolder(".."); errorCode(err) != codeQuizNotSaved {
		t.Errorf("another folder = %v", err)
	}
	if v, err = h.app.DeleteQuiz("examen-secreto"); err != nil || len(v.Saved) != 0 {
		t.Fatalf("DeleteQuiz = %+v, %v", v, err)
	}
	if _, err := h.app.DeleteQuiz("examen-secreto"); errorCode(err) != codeQuizGone {
		t.Errorf("second delete = %v", err)
	}
	if _, err := h.app.OpenQuiz("no-existe", nil); errorCode(err) != codeQuizGone {
		t.Errorf("launch a missing one = %v", err)
	}
	if strings.Contains(logs.String(), "secret") {
		t.Errorf("the quiz text is in the log:\n%s", logs.String())
	}
}

// «Lanzar» sends the quiz only to who declared poll.quiz, never with the
// correct options; answers, close, the personal results and the CSV follow.
// The quiz text, the answers and the scores never reach the log.
func TestQuizLaunchAnswerShareExport(t *testing.T) {
	h, docs, logs := quizHarness(t)
	h.app.SaveQuiz("", secretQuiz())
	v, err := h.app.OpenQuiz("examen-secreto", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p := v.Current; p == nil || p.Quiz == nil || p.Eligible != 2 || len(p.Unsupported) != 1 || p.Unsupported[0] != "PC03" {
		t.Fatalf("view = %+v", v.Current)
	}
	sent := h.class.takeDirect()
	if len(sent) != 2 || sent[0].sessionID == "s3" || sent[1].sessionID == "s3" {
		t.Fatalf("sent %+v", sent)
	}
	open := sent[0].p.(protocol.PollOpen)
	data, _ := json.Marshal(open)
	if open.Kind != protocol.PollKindQuiz || strings.Contains(string(data), "correct") {
		t.Fatalf("poll.open = %s", data)
	}
	answer := func(sessionID string, submit bool, a ...protocol.PollQuizAnswer) protocol.PollVoteAck {
		t.Helper()
		h.hooks.onPoll(sessionID, protocol.Envelope{ID: protocol.NewID(), Payload: protocol.PollVote{PollID: open.PollID, Answers: a, Submitted: submit}})
		return ackOfVote(t, h, sessionID)
	}
	if a := answer("s1", true, protocol.PollQuizAnswer{Q: 0, Choices: []int{0}}, protocol.PollQuizAnswer{Q: 1, Choices: []int{1}}); a.Status != protocol.PollVoteAccepted {
		t.Fatalf("hand in = %+v", a)
	}
	answer("s2", false, protocol.PollQuizAnswer{Q: 0, Choices: []int{1}})
	cur := h.app.Polls().Current
	if cur.Quiz.Submitted != 1 || cur.Quiz.Scores[0].Score != 3 || cur.Quiz.Scores[1].Score != 0 {
		t.Fatalf("scores = %+v", cur.Quiz)
	}
	if _, err := h.app.ExportQuizGrades(open.PollID); errorCode(err) != codeGradesNotExported {
		t.Errorf("export while open = %v", err)
	}
	if _, err := h.app.ClosePoll(); err != nil {
		t.Fatal(err)
	}
	h.class.takeDirect()
	if _, err := h.app.SharePollResults(open.PollID); err != nil {
		t.Fatal(err)
	}
	sent = h.class.takeDirect()
	if len(sent) != 2 {
		t.Fatalf("share sent %+v", sent)
	}
	for _, s := range sent {
		r, ok := s.p.(protocol.PollQuizResult)
		want := map[string]int{"s1": 3, "s2": 0}[s.sessionID]
		if !ok || r.Score != want || r.Max != 3 {
			t.Errorf("%s got %+v", s.sessionID, s.p)
		}
	}
	path, err := h.app.ExportQuizGrades(open.PollID)
	if err != nil {
		t.Fatal(err)
	}
	if dir := filepath.Join(docs, "Aula", poll.GradesFolder, time.Now().Format("2006-01-02")); filepath.Dir(path) != dir || filepath.Base(path) != "examen-secreto.csv" {
		t.Errorf("path = %s", path)
	}
	csv, _ := os.ReadFile(path)
	if !strings.Contains(string(csv), "PC01;;3;3;sí;2;1\r\n") || !strings.Contains(string(csv), "PC02;;0;3;no;0;0\r\n") {
		t.Errorf("CSV =\n%s", csv)
	}
	// A second export does not replace the first.
	if again, err := h.app.ExportQuizGrades(open.PollID); err != nil || filepath.Base(again) != "examen-secreto (2).csv" {
		t.Errorf("second export = %q, %v", again, err)
	}
	for _, secret := range []string{"secret", "PC01;"} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("%q is in the log:\n%s", secret, logs.String())
		}
	}
}

func TestQuizNoTargets(t *testing.T) {
	h, _, _ := quizHarness(t)
	h.app.SaveQuiz("", secretQuiz())
	if _, err := h.app.OpenQuiz("examen-secreto", []string{"PC03"}); errorCode(err) != codeNoQuizTargets {
		t.Errorf("only an older agent = %v", err)
	}
}

// With a time, the quiz starts the timer for the same students and closes
// when it reaches zero; closing it before stops its timer.
func TestQuizClosesWithItsTimer(t *testing.T) {
	h, _, _ := quizHarness(t)
	var mu sync.Mutex
	now := time.Unix(1_792_771_200, 0)
	clock := func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	h.app.timer = classtimer.NewClass(clock)
	q := secretQuiz()
	q.Minutes = 2
	h.app.SaveQuiz("", q)
	if _, err := h.app.OpenQuiz("examen-secreto", []string{"PC01"}); err != nil {
		t.Fatal(err)
	}
	ts := h.app.Timer()
	if !ts.Active || ts.DurationS != 120 || ts.All || ts.Label != "Examen secreto" {
		t.Fatalf("timer = %+v", ts)
	}
	h.app.closeQuizAtZero() // not yet
	if h.app.Polls().Current == nil {
		t.Fatal("closed before zero")
	}
	mu.Lock()
	now = now.Add(2 * time.Minute)
	mu.Unlock()
	h.app.closeQuizAtZero()
	if h.app.Polls().Current != nil {
		t.Fatal("still open at zero")
	}

	// Closed by hand: its timer stops.
	if _, err := h.app.OpenQuiz("examen-secreto", nil); err != nil {
		t.Fatal(err)
	}
	if !h.app.Timer().Active {
		t.Fatal("no timer for the second quiz")
	}
	if _, err := h.app.ClosePoll(); err != nil {
		t.Fatal(err)
	}
	if h.app.Timer().Active {
		t.Error("closing the quiz did not stop its timer")
	}
	if timerLabel(strings.Repeat("ñ", 60)) != strings.Repeat("ñ", protocol.MaxTimerLabel-1)+"…" {
		t.Error("a long title does not fit the timer")
	}
}
