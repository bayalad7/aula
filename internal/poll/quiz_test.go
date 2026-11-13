package poll

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// sampleQuiz has one question of each shape: one correct option, several
// correct options, and yes/no. Its correct option texts contain «CORRECTA»
// so a test can look for them in what travels.
func sampleQuiz() Quiz {
	return Quiz{Title: " Repaso de bucles ", Questions: []QuizQuestion{
		{Text: "¿Qué imprime print(3 // 2)?", Kind: protocol.PollKindChoice,
			Options: []string{"1.5", "1 CORRECTA", "2"}, Correct: []int{1}, Points: 2},
		{Text: "¿Cuáles son bucles?", Kind: protocol.PollKindChoice,
			Options: []string{"for CORRECTA", "if", "while CORRECTA", "def"}, Correct: []int{2, 0}, Points: 3},
		{Text: "¿range(3) incluye el 3?", Kind: protocol.PollKindYesNo, Correct: []int{1}, Points: 1},
	}}
}

func quizStudent(id, host string) session.Session {
	return student(id, host, protocol.CapPoll, protocol.CapPollQuiz)
}

func answers(pairs ...[]int) []protocol.PollQuizAnswer {
	out := []protocol.PollQuizAnswer{}
	for _, p := range pairs {
		out = append(out, protocol.PollQuizAnswer{Q: p[0], Choices: p[1:]})
	}
	return out
}

func quizVote(pollID string, submit bool, a []protocol.PollQuizAnswer) protocol.PollVote {
	return protocol.PollVote{PollID: pollID, Answers: a, Submitted: submit}
}

func TestQuizValidate(t *testing.T) {
	if err := sampleQuiz().Normalize().Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Quiz){
		"no title":         func(q *Quiz) { q.Title = "  " },
		"long title":       func(q *Quiz) { q.Title = strings.Repeat("ñ", protocol.MaxQuizTitle+1) },
		"no questions":     func(q *Quiz) { q.Questions = nil },
		"no correct":       func(q *Quiz) { q.Questions[0].Correct = nil },
		"correct too far":  func(q *Quiz) { q.Questions[0].Correct = []int{3} },
		"two yes/no":       func(q *Quiz) { q.Questions[2].Correct = []int{0, 1} },
		"zero points":      func(q *Quiz) { q.Questions[0].Points = 0 },
		"empty option":     func(q *Quiz) { q.Questions[0].Options[2] = " " },
		"open question":    func(q *Quiz) { q.Questions[0].Kind = protocol.PollKindOpen },
		"too much time":    func(q *Quiz) { q.Minutes = MaxQuizMinutes + 1 },
		"too many options": func(q *Quiz) { q.Questions[0].Options = []string{"a", "b", "c", "d", "e", "f", "g"} },
	} {
		q := sampleQuiz()
		change(&q)
		if err := q.Normalize().Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v, want ErrInvalid", name, err)
		}
	}
}

// Grading is all or nothing: exactly the correct options.
func TestQuizGrade(t *testing.T) {
	q := sampleQuiz().Normalize()
	if q.Max() != 6 {
		t.Fatalf("max = %d", q.Max())
	}
	for _, c := range []struct {
		answers []protocol.PollQuizAnswer
		earned  []int
		total   int
	}{
		{nil, []int{0, 0, 0}, 0},
		{answers([]int{0, 1}, []int{1, 0, 2}, []int{2, 1}), []int{2, 3, 1}, 6},
		{answers([]int{1, 2, 0}), []int{0, 3, 0}, 3}, // order does not matter
		{answers([]int{1, 0}), []int{0, 0, 0}, 0},    // half is nothing
		{answers([]int{1, 0, 2, 3}), []int{0, 0, 0}, 0},
		{answers([]int{0, 0}, []int{2, 0}), []int{0, 0, 0}, 0},
	} {
		earned, total := q.grade(c.answers)
		if total != c.total || jsonOf(earned) != jsonOf(c.earned) {
			t.Errorf("%+v: earned %v total %d, want %v %d", c.answers, earned, total, c.earned, c.total)
		}
	}
}

func jsonOf(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// The correct options never travel to an agent before the close: not in
// any poll.open (first or resent, shuffled or not) nor in the vote acks.
func TestQuizCorrectNeverSentBeforeClose(t *testing.T) {
	for _, shuffle := range []bool{false, true} {
		clock := newClock()
		c := NewClass(clock.Now)
		q := sampleQuiz()
		q.Shuffle = shuffle
		id, err := c.OpenQuiz(q, nil)
		if err != nil {
			t.Fatal(err)
		}
		s1, s2 := quizStudent("s1", "PC01"), quizStudent("s2", "PC02")
		cmds := c.Plan([]session.Session{s1, s2}, nil)
		if _, err := c.Vote("s1", who("PC01"), quizVote(id, false, answers([]int{0, 1}))); err != nil {
			t.Fatal(err)
		}
		s1.ID = "s1b" // reconnects: the quiz is sent again with its answers
		cmds = append(cmds, c.Plan([]session.Session{s1, s2}, nil)...)
		if len(cmds) != 3 {
			t.Fatalf("sent %s", types(cmds))
		}
		for _, cmd := range cmds {
			data, err := json.Marshal(cmd.Payload)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "correct") {
				t.Errorf("shuffle %v: %s carries the correct options: %s", shuffle, cmd.Payload.MessageType(), data)
			}
			open := cmd.Payload.(protocol.PollOpen)
			if err := open.Validate(); err != nil {
				t.Errorf("poll.open is not valid: %v", err)
			}
			// «CORRECTA» is an option text: it travels as one option among
			// others, never marked. The correct ones are never marked
			// otherwise (no field for it).
			for _, qq := range open.Questions {
				if qq.ID == 1 && !qq.Multi {
					t.Error("a question with two correct options must say «choose all that apply»")
				}
				if qq.ID == 0 && qq.Multi {
					t.Error("a question with one correct option is not multi")
				}
			}
		}
		if got := cmds[2].Payload.(protocol.PollOpen); len(got.Answers) != 1 || got.Answers[0].Choices[0] != 1 {
			t.Errorf("the resend lost the answers: %+v", got.Answers)
		}
	}
}

// Shuffling gives each student their own order, the same every time.
func TestQuizShufflePerStudent(t *testing.T) {
	q := sampleQuiz().Normalize()
	q.Shuffle = true
	for i := range 4 {
		q.Questions = append(q.Questions, QuizQuestion{Text: "¿Extra?", Kind: protocol.PollKindChoice,
			Options: []string{"a", "b", "c", "d", "e", "f"}, Correct: []int{i}, Points: 1})
	}
	order := func(seed string) string {
		var b strings.Builder
		for _, qq := range q.wire(seed) {
			b.WriteString(jsonOf(qq.ID))
			for _, o := range qq.Options {
				b.WriteString(jsonOf(o.ID))
			}
			b.WriteByte('|')
		}
		return b.String()
	}
	a, b := order(seedOf(pid1, who("PC01"))), order(seedOf(pid1, who("PC02")))
	if a != order(seedOf(pid1, who("pc01"))) {
		t.Error("the same student must always get the same order")
	}
	if a == b {
		t.Error("two students got the same order")
	}
	q.Shuffle = false
	if order(seedOf(pid1, who("PC01"))) != order("") {
		t.Error("without shuffle every student gets the written order")
	}
}

// Answers change until «Entregar»; after it (or the close) nothing changes.
// A reconnection brings the student back where they were.
func TestQuizAnswerSubmitAndReconnect(t *testing.T) {
	clock := newClock()
	c := NewClass(clock.Now)
	id, err := c.OpenQuiz(sampleQuiz(), []string{"PC01", "PC02"})
	if err != nil {
		t.Fatal(err)
	}
	s1 := quizStudent("s1", "PC01")
	c.Plan([]session.Session{s1}, nil)
	vote := func(v protocol.PollVote) protocol.PollVoteStatus {
		t.Helper()
		clock.Add(2 * time.Second)
		st, err := c.Vote("s1", who("PC01"), v)
		if err != nil {
			t.Fatal(err)
		}
		return st
	}
	if st := vote(quizVote(id, false, answers([]int{0, 2}))); st != protocol.PollVoteAccepted {
		t.Fatalf("first answer = %s", st)
	}
	vote(quizVote(id, false, answers([]int{0, 1}, []int{2, 1})))
	v := c.Snapshot([]session.Session{s1}, nil).Current
	if v.Quiz == nil || len(v.Quiz.Scores) != 1 || v.Quiz.Scores[0].Score != 3 || v.Quiz.Scores[0].Submitted {
		t.Fatalf("score after changing = %+v", v.Quiz)
	}
	if st := vote(quizVote(id, true, answers([]int{0, 1}, []int{2, 1}, []int{1, 0, 2}))); st != protocol.PollVoteAccepted {
		t.Fatalf("hand in = %s", st)
	}
	if st := vote(quizVote(id, false, answers([]int{0, 0}))); st != protocol.PollVoteClosed {
		t.Fatalf("an answer after handing in = %s", st)
	}
	v = c.Snapshot([]session.Session{s1}, nil).Current
	if sc := v.Quiz.Scores[0]; sc.Score != 6 || !sc.Submitted || v.Quiz.Submitted != 1 || sc.Earned[1] != 3 {
		t.Fatalf("score after handing in = %+v", v.Quiz)
	}
	// A wrong answer shape is invalid, not a vote.
	clock.Add(2 * time.Second)
	if _, err := c.Vote("s1", who("PC01"), quizVote(id, false, answers([]int{0, 0, 1}))); !errors.Is(err, ErrInvalid) {
		t.Errorf("two options where one goes = %v", err)
	}
	clock.Add(2 * time.Second)
	if _, err := c.Vote("s1", who("PC01"), voteFor(id, 0)); !errors.Is(err, ErrInvalid) {
		t.Errorf("a plain choice to a quiz = %v", err)
	}

	s1.ID = "s1b"
	cmds := c.Plan([]session.Session{s1}, nil)
	if len(cmds) != 1 {
		t.Fatalf("reconnect sent %s", types(cmds))
	}
	open := cmds[0].Payload.(protocol.PollOpen)
	if !open.Submitted || len(open.Answers) != 3 {
		t.Fatalf("resend = %+v", open)
	}
}

// An agent without poll.quiz never gets the quiz and is listed as unable;
// its previous poll card is closed.
func TestQuizNeedsCapability(t *testing.T) {
	c := NewClass(nil)
	old := student("s3", "PC03") // poll only
	if _, err := c.Open(question); err != nil {
		t.Fatal(err)
	}
	if cmds := c.Plan([]session.Session{old}, nil); types(cmds) != "s3:poll.open" {
		t.Fatalf("poll sent %s", types(cmds))
	}
	if _, err := c.OpenQuiz(sampleQuiz(), nil); err != nil {
		t.Fatal(err)
	}
	newer := quizStudent("s1", "PC01")
	cmds := c.Plan([]session.Session{old, newer}, nil)
	if types(cmds) != "s3:poll.close s1:poll.open" {
		t.Fatalf("quiz sent %s", types(cmds))
	}
	v := c.Snapshot([]session.Session{old, newer}, nil).Current
	if v.Eligible != 1 || len(v.Unsupported) != 1 || v.Unsupported[0] != "PC03" {
		t.Fatalf("eligible = %d, unsupported = %v", v.Eligible, v.Unsupported)
	}
}

// «Compartir resultados» sends each student only their own result, also
// after a reconnection; the simulated ones get theirs, apart.
func TestQuizResultOnlyToItsOwner(t *testing.T) {
	clock := newClock()
	c := NewClass(clock.Now)
	id, _ := c.OpenQuiz(sampleQuiz(), nil)
	s1, s2 := quizStudent("s1", "PC01"), quizStudent("s2", "PC02")
	sim := quizStudent("sim1", "SIM-01")
	c.Plan([]session.Session{s1, s2}, []session.Session{sim})
	mustVote := func(sessionID string, w Student, a []protocol.PollQuizAnswer) {
		t.Helper()
		if _, err := c.Vote(sessionID, w, quizVote(id, true, a)); err != nil {
			t.Fatal(err)
		}
	}
	mustVote("s1", who("PC01"), answers([]int{0, 1}, []int{2, 1}))
	mustVote("s2", who("PC02"), answers([]int{1, 0, 2}))
	mustVote("sim1", Student{Hostname: "SIM-01", Username: "alumno", Simulated: true}, answers([]int{0, 0}))
	if err := c.Share(id); !errors.Is(err, ErrStillOpen) {
		t.Fatalf("share while open = %v", err)
	}
	if _, err := c.Close(); err != nil {
		t.Fatal(err)
	}
	c.Plan([]session.Session{s1, s2}, []session.Session{sim})
	if err := c.Share(id); err != nil {
		t.Fatal(err)
	}
	cmds := c.Plan([]session.Session{s1, s2}, []session.Session{sim})
	if types(cmds) != "s1:poll.quiz.result s2:poll.quiz.result sim1:poll.quiz.result" {
		t.Fatalf("share sent %s", types(cmds))
	}
	want := map[string]int{"s1": 3, "s2": 3, "sim1": 0}
	for _, cmd := range cmds {
		r := cmd.Payload.(protocol.PollQuizResult)
		if err := r.Validate(); err != nil {
			t.Fatal(err)
		}
		if r.Score != want[cmd.SessionID] || r.Max != 6 || len(r.Questions) != 3 {
			t.Errorf("%s got %+v", cmd.SessionID, r)
		}
		data := jsonOf(r)
		for _, other := range []string{"PC01", "PC02", "SIM-01", "alumno"} {
			if strings.Contains(data, other) {
				t.Errorf("a personal result names %s: %s", other, data)
			}
		}
	}
	// Not acked: a reconnection sends it again, still only theirs.
	s2.ID = "s2b"
	cmds = c.Plan([]session.Session{s1, s2}, nil)
	if types(cmds) != "s2b:poll.close s2b:poll.quiz.result" {
		t.Fatalf("reconnect sent %s", types(cmds))
	}
	if r := cmds[1].Payload.(protocol.PollQuizResult); r.Questions[1].Earned != 3 || r.Score != 3 {
		t.Fatalf("resent result = %+v", r)
	}
}

// The CSV has one row per real student who got the quiz (answered or not),
// never the simulated ones, with «;», a BOM and CRLF.
func TestQuizGradesCSV(t *testing.T) {
	clock := newClock()
	c := NewClass(clock.Now)
	id, _ := c.OpenQuiz(sampleQuiz(), nil)
	s1, s2, s3 := quizStudent("s1", "PC02"), quizStudent("s2", "PC01"), quizStudent("s3", "PC03")
	s3.Username = "=HYPERLINK(1)"
	sim := quizStudent("sim1", "SIM-01")
	c.Plan([]session.Session{s1, s2, s3}, []session.Session{sim})
	if _, _, err := c.Grades(id); !errors.Is(err, ErrStillOpen) {
		t.Fatalf("grades while open = %v", err)
	}
	c.Vote("s1", who("PC02"), quizVote(id, true, answers([]int{0, 1}, []int{1, 0, 2}, []int{2, 1})))
	c.Vote("s2", who("PC01"), quizVote(id, false, answers([]int{0, 2})))
	c.Vote("sim1", Student{Hostname: "SIM-01", Username: "alumno", Simulated: true}, quizVote(id, true, answers([]int{0, 1})))
	c.Close()
	data, title, err := c.Grades(id)
	if err != nil {
		t.Fatal(err)
	}
	want := "\uFEFFEquipo;Usuario;Puntaje;Máximo;Entregó;P1 (2);P2 (3);P3 (1)\r\n" +
		"PC01;alumno;0;6;no;0;0;0\r\n" +
		"PC02;alumno;6;6;sí;2;3;1\r\n" +
		"PC03;'=HYPERLINK(1);0;6;no;0;0;0\r\n"
	if string(data) != want || title != "Repaso de bucles" {
		t.Errorf("CSV =\n%q\nwant\n%q (title %q)", data, want, title)
	}
	if _, _, err := c.Grades(protocol.NewID()); !errors.Is(err, ErrGone) {
		t.Errorf("unknown = %v", err)
	}
	pid, _ := c.Open(question)
	c.Close()
	if _, _, err := c.Grades(pid); !errors.Is(err, ErrNotQuiz) {
		t.Errorf("a poll = %v", err)
	}
	v := c.Snapshot(nil, nil)
	if q := v.Closed[1].Quiz; q == nil || !q.Exportable || q.Questions[0].Right != 1 || q.Questions[0].Answered != 2 || q.Questions[0].RightSimulated != 1 || q.Questions[0].AnsweredSimulated != 1 {
		t.Errorf("quiz view = %+v", q)
	}
}

// --- Saved quizzes ---------------------------------------------------------------

func TestLibrary(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cuestionarios")
	l := Library{Dir: dir}
	if list, err := l.List(); err != nil || len(list) != 0 {
		t.Fatalf("a missing folder = %v, %v", list, err)
	}
	id, err := l.Save("", sampleQuiz())
	if err != nil || id != "repaso-de-bucles" {
		t.Fatalf("Save = %q, %v", id, err)
	}
	id2, err := l.Save("", sampleQuiz())
	if err != nil || id2 != "repaso-de-bucles-2" {
		t.Fatalf("a second one = %q, %v", id2, err)
	}
	q := sampleQuiz()
	q.Title = "Ñandú: ¿qué es?"
	if _, err := l.Save(id2, q); err != nil {
		t.Fatal(err)
	}
	got, err := l.Load(id2)
	if err != nil || got.Title != "Ñandú: ¿qué es?" || got.Questions[1].Correct[0] != 0 {
		t.Fatalf("Load = %+v, %v", got, err)
	}
	if _, err := l.Save("", Quiz{Title: "x"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("an invalid quiz = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roto.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	list, err := l.List()
	if err != nil || len(list) != 3 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if list[0].ID != id2 || list[1].ID != id || list[2].Problem != "unreadable" || list[2].Quiz.Title != "roto" {
		t.Errorf("List order and problems = %+v", list)
	}
	for _, bad := range []string{"../fuera", "", "A", `x\y`} {
		if _, err := l.Load(bad); !errors.Is(err, ErrQuizMissing) {
			t.Errorf("Load(%q) = %v", bad, err)
		}
	}
	if err := l.Delete(id); err != nil {
		t.Fatal(err)
	}
	if err := l.Delete(id); !errors.Is(err, ErrQuizMissing) {
		t.Errorf("second delete = %v", err)
	}
	if slug("¡¡!!") != "cuestionario" || GradesName("Examen 1: Bucles") != "examen-1-bucles.csv" {
		t.Errorf("slug = %q, %q", slug("¡¡!!"), GradesName("Examen 1: Bucles"))
	}
}

// --- Agent side --------------------------------------------------------------------

func TestBallotQuiz(t *testing.T) {
	clock := newClock()
	b := NewBallot(clock.Now)
	c := NewClass(clock.Now)
	id, _ := c.OpenQuiz(sampleQuiz(), nil)
	open := c.Plan([]session.Session{quizStudent("s1", "PC01")}, nil)[0].Payload.(protocol.PollOpen)
	if _, _, isNew := b.Open(refA, open); !isNew {
		t.Fatal("a new quiz is a new card")
	}
	if err := b.AnswerQuiz(id, answers([]int{0, 1}), false); err != nil {
		t.Fatal(err)
	}
	v, wait := b.NextQuizVote(id)
	if v == nil || wait != 0 || v.Submitted {
		t.Fatalf("first = %+v, %v", v, wait)
	}
	b.Sending(refB, *v)
	// Two quick changes: only the last goes, after the gap.
	b.AnswerQuiz(id, answers([]int{0, 2}), false)
	b.AnswerQuiz(id, answers([]int{0, 1}, []int{1, 2, 0}), false)
	if v, wait := b.NextQuizVote(id); v != nil || wait != VoteGap {
		t.Fatalf("too soon = %+v, %v", v, wait)
	}
	if b.VoteAck(protocol.PollVoteAck{RefID: refB, PollID: id, Status: protocol.PollVoteAccepted}); b.View().Cards[0].Vote != VoteSending {
		t.Errorf("with answers waiting the card is still sending: %+v", b.View().Cards[0])
	}
	clock.Add(VoteGap)
	v, _ = b.NextQuizVote(id)
	if v == nil || len(v.Answers) != 2 || v.Answers[1].Choices[0] != 0 {
		t.Fatalf("the last = %+v", v)
	}
	b.Sending(refA, *v)
	b.VoteAck(protocol.PollVoteAck{RefID: refA, PollID: id, Status: protocol.PollVoteAccepted})
	if err := b.AnswerQuiz(id, answers([]int{0, 1}), true); err != nil {
		t.Fatal(err)
	}
	if err := b.AnswerQuiz(id, nil, false); !errors.Is(err, ErrHandedIn) {
		t.Errorf("after «Entregar» = %v", err)
	}
	if _, err := b.Vote(id, 0, ""); !errors.Is(err, ErrBadVote) {
		t.Errorf("a plain vote on a quiz = %v", err)
	}
	clock.Add(VoteGap)
	v, _ = b.NextQuizVote(id)
	b.Sending(refB, *v)
	b.VoteAck(protocol.PollVoteAck{RefID: refB, PollID: id, Status: protocol.PollVoteAccepted})
	card := b.View().Cards[0]
	if !card.Submitted || card.Vote != VoteAccepted || len(card.Questions) != 3 {
		t.Fatalf("handed in = %+v", card)
	}
	if _, ok := b.Close(refA, protocol.PollClose{PollID: id}); !ok {
		t.Fatal("close")
	}
	r := protocol.PollQuizResult{PollID: id, Score: 2, Max: 6, Questions: []protocol.PollQuizResultQuestion{{Q: 0, Choices: []int{1}, Correct: []int{1}, Earned: 2}}}
	if ack, ok := b.QuizResult(refB, r); !ok || ack.Status != protocol.PollAckResults {
		t.Fatalf("result ack = %+v, %v", ack, ok)
	}
	if got := b.View().Cards[0].Result; got == nil || got.Score != 2 {
		t.Fatalf("result = %+v", got)
	}
}

// Answers that never left are lost at the close; a resend after the app
// restarted brings back what the console had.
func TestBallotQuizCloseAndRestart(t *testing.T) {
	clock := newClock()
	b := NewBallot(clock.Now)
	c := NewClass(clock.Now)
	id, _ := c.OpenQuiz(sampleQuiz(), nil)
	open := c.Plan([]session.Session{quizStudent("s1", "PC01")}, nil)[0].Payload.(protocol.PollOpen)
	b.Open(refA, open)
	b.AnswerQuiz(id, answers([]int{0, 1}), false)
	b.Close(refB, protocol.PollClose{PollID: id})
	if card := b.View().Cards[0]; card.Vote != VoteLate || card.Open {
		t.Fatalf("a draft at the close = %+v", card)
	}
	if err := b.AnswerQuiz(id, nil, true); !errors.Is(err, ErrClosed) {
		t.Errorf("answer a closed quiz = %v", err)
	}

	fresh := NewBallot(clock.Now)
	open.Answers, open.Submitted = answers([]int{2, 0}), true
	fresh.Open(refA, open)
	if card := fresh.View().Cards[0]; !card.Submitted || card.Vote != VoteAccepted || len(card.Answers) != 1 {
		t.Fatalf("restarted card = %+v", card)
	}
}
