package poll

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/protocol"
)

// MaxQuizMinutes is the longest time a quiz may run with the timer.
const MaxQuizMinutes = 120

// Errors of the quizzes.
var (
	ErrNotQuiz = errors.New("the poll is not a quiz")
)

// Quiz is a quiz as the teacher writes and saves it: their material, with
// the correct options (docs/privacidad.md rule 89). It never travels as is:
// the students get its questions without Correct (rule 88).
type Quiz struct {
	Title     string         `json:"title"`
	Questions []QuizQuestion `json:"questions"`
	Minutes   int            `json:"minutes"` // 0: no time; else the timer closes it
	Shuffle   bool           `json:"shuffle"` // questions and options in another order for each student
}

// QuizQuestion is one question of a Quiz. Correct are indexes of Options
// (yesno: 0 yes, 1 no); more than one makes it a «choose all that apply».
type QuizQuestion struct {
	Text    string            `json:"text"`
	Kind    protocol.PollKind `json:"kind"` // choice or yesno
	Options []string          `json:"options"`
	Correct []int             `json:"correct"`
	Points  int               `json:"points"`
}

// Normalize trims the texts, sorts Correct and drops what a yes/no question
// does not have.
func (q Quiz) Normalize() Quiz {
	out := Quiz{Title: strings.TrimSpace(q.Title), Minutes: q.Minutes, Shuffle: q.Shuffle,
		Questions: make([]QuizQuestion, 0, len(q.Questions))}
	for _, qq := range q.Questions {
		n := QuizQuestion{Text: strings.TrimSpace(qq.Text), Kind: qq.Kind, Points: qq.Points,
			Options: []string{}, Correct: slices.Compact(slices.Sorted(slices.Values(qq.Correct)))}
		if n.Correct == nil {
			n.Correct = []int{}
		}
		if qq.Kind == protocol.PollKindChoice {
			for _, o := range qq.Options {
				n.Options = append(n.Options, strings.TrimSpace(o))
			}
		}
		out.Questions = append(out.Questions, n)
	}
	return out
}

// Validate checks the quiz against the limits of §5.23.1: what the console
// sends must be a valid poll.open, and every question has its correct
// options.
func (q Quiz) Validate() error {
	if n := utf8.RuneCountInString(q.Title); strings.TrimSpace(q.Title) == "" || n > protocol.MaxQuizTitle {
		return fmt.Errorf("%w: the title must be 1-%d characters", ErrInvalid, protocol.MaxQuizTitle)
	}
	if q.Minutes < 0 || q.Minutes > MaxQuizMinutes {
		return fmt.Errorf("%w: the time must be 0-%d minutes", ErrInvalid, MaxQuizMinutes)
	}
	if len(q.Questions) < 1 || len(q.Questions) > protocol.MaxQuizQuestions {
		return fmt.Errorf("%w: a quiz has 1-%d questions", ErrInvalid, protocol.MaxQuizQuestions)
	}
	for i, qq := range q.Questions {
		if err := qq.wire(i, nil).Validate(); err != nil {
			return fmt.Errorf("%w: question %d: %w", ErrInvalid, i+1, err)
		}
		n := qq.optionCount()
		if len(qq.Correct) < 1 || (qq.Kind == protocol.PollKindYesNo && len(qq.Correct) != 1) {
			return fmt.Errorf("%w: question %d has no correct option", ErrInvalid, i+1)
		}
		for j, c := range qq.Correct {
			if c < 0 || c >= n || (j > 0 && qq.Correct[j-1] >= c) {
				return fmt.Errorf("%w: question %d: correct options are 0-%d, once each", ErrInvalid, i+1, n-1)
			}
		}
	}
	return nil
}

// Max is the points of the whole quiz.
func (q Quiz) Max() int {
	sum := 0
	for _, qq := range q.Questions {
		sum += qq.Points
	}
	return sum
}

func (qq QuizQuestion) optionCount() int {
	if qq.Kind == protocol.PollKindYesNo {
		return 2
	}
	return len(qq.Options)
}

// multi says the student may tick several options.
func (qq QuizQuestion) multi() bool { return len(qq.Correct) > 1 }

// wire is the question as the student sees it: never Correct. order is the
// order of its options (nil: as written).
func (qq QuizQuestion) wire(id int, order []int) protocol.PollQuizQuestion {
	w := protocol.PollQuizQuestion{ID: id, Kind: qq.Kind, Text: qq.Text, Points: qq.Points, Multi: qq.multi()}
	if qq.Kind == protocol.PollKindChoice {
		if order == nil {
			order = identity(len(qq.Options))
		}
		for _, i := range order {
			w.Options = append(w.Options, protocol.PollQuizOption{ID: i, Text: qq.Options[i]})
		}
	}
	return w
}

// earned grades one answer, all or nothing: the points only when the
// student chose exactly the correct options.
func (qq QuizQuestion) earned(choices []int) int {
	got := slices.Sorted(slices.Values(choices))
	if slices.Equal(slices.Compact(got), qq.Correct) {
		return qq.Points
	}
	return 0
}

// wire returns the questions for one student: in another order for each
// one when the quiz is shuffled, always the same for the same student (also
// when sent again). seed is empty for the written order.
func (q Quiz) wire(seed string) []protocol.PollQuizQuestion {
	var r *rand.Rand
	if q.Shuffle && seed != "" {
		h := fnv.New64a()
		h.Write([]byte(seed))
		s := h.Sum64()
		r = rand.New(rand.NewPCG(s, s>>1|1))
	}
	order := identity(len(q.Questions))
	if r != nil {
		r.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	}
	out := make([]protocol.PollQuizQuestion, 0, len(order))
	for _, i := range order {
		var opts []int
		if r != nil && q.Questions[i].Kind == protocol.PollKindChoice {
			opts = identity(len(q.Questions[i].Options))
			r.Shuffle(len(opts), func(a, b int) { opts[a], opts[b] = opts[b], opts[a] })
		}
		out = append(out, q.Questions[i].wire(i, opts))
	}
	return out
}

func identity(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// grade returns the points of each question (in the quiz's order) and the
// total for answers.
func (q Quiz) grade(answers []protocol.PollQuizAnswer) ([]int, int) {
	by := map[int][]int{}
	for _, a := range answers {
		by[a.Q] = a.Choices
	}
	earned := make([]int, len(q.Questions))
	total := 0
	for i, qq := range q.Questions {
		if c, ok := by[i]; ok {
			earned[i] = qq.earned(c)
		}
		total += earned[i]
	}
	return earned, total
}

// result is one student's own result (rule 90).
func (q Quiz) result(pollID string, answers []protocol.PollQuizAnswer) protocol.PollQuizResult {
	by := map[int][]int{}
	for _, a := range answers {
		by[a.Q] = a.Choices
	}
	earned, total := q.grade(answers)
	out := protocol.PollQuizResult{PollID: pollID, Score: total, Max: q.Max()}
	for i, qq := range q.Questions {
		chosen := slices.Sorted(slices.Values(by[i]))
		if chosen == nil {
			chosen = []int{}
		}
		out.Questions = append(out.Questions, protocol.PollQuizResultQuestion{Q: i, Choices: chosen,
			Correct: slices.Clone(qq.Correct), Earned: earned[i]})
	}
	return out
}

// seedOf is the shuffle seed of a student in a poll.
func seedOf(pollID string, who Student) string {
	return pollID + "\x00" + strings.ToLower(who.Hostname) + "\x00" + who.Username
}

// normalizeAnswers sorts the answers by question and their choices, and
// drops the empty ones (a question the student left blank).
func normalizeAnswers(in []protocol.PollQuizAnswer) []protocol.PollQuizAnswer {
	out := []protocol.PollQuizAnswer{}
	for _, a := range in {
		if len(a.Choices) == 0 {
			continue
		}
		out = append(out, protocol.PollQuizAnswer{Q: a.Q, Choices: slices.Sorted(slices.Values(a.Choices))})
	}
	slices.SortFunc(out, func(a, b protocol.PollQuizAnswer) int { return a.Q - b.Q })
	return out
}

// QuizQuestionView is one question of a quiz for the console: its correct
// options and how the real students did, the simulated ones apart.
type QuizQuestionView struct {
	Text              string            `json:"text"`
	Kind              protocol.PollKind `json:"kind"`
	Options           []string          `json:"options"`
	Correct           []int             `json:"correct"`
	Points            int               `json:"points"`
	Right             int               `json:"right"`    // real students who got it right
	Answered          int               `json:"answered"` // real students who answered it
	RightSimulated    int               `json:"rightSimulated"`
	AnsweredSimulated int               `json:"answeredSimulated"`
}

// QuizScore is one student's score, for «Participación» only (rule 90).
type QuizScore struct {
	Student
	Score     int   `json:"score"`
	Answered  int   `json:"answered"` // questions answered
	Submitted bool  `json:"submitted"`
	Earned    []int `json:"earned"` // by question, in the quiz's order; -1 not answered
}

// QuizView is the quiz part of a PollView.
type QuizView struct {
	Title              string             `json:"title"`
	Max                int                `json:"max"`
	Minutes            int                `json:"minutes"`
	Shuffle            bool               `json:"shuffle"`
	Questions          []QuizQuestionView `json:"questions"`
	Scores             []QuizScore        `json:"scores"`
	Submitted          int                `json:"submitted"`
	SubmittedSimulated int                `json:"submittedSimulated"`
	Exportable         bool               `json:"exportable"` // closed, with real students
}

// quizView builds the console view of a quiz poll.
func (p *poll) quizView() *QuizView {
	q := p.quiz
	v := &QuizView{Title: q.Title, Max: q.Max(), Minutes: q.Minutes, Shuffle: q.Shuffle,
		Questions: []QuizQuestionView{}, Scores: []QuizScore{}}
	for _, qq := range q.Questions {
		opts := slices.Clone(qq.Options)
		if opts == nil {
			opts = []string{}
		}
		v.Questions = append(v.Questions, QuizQuestionView{Text: qq.Text, Kind: qq.Kind, Options: opts,
			Correct: slices.Clone(qq.Correct), Points: qq.Points})
	}
	for _, who := range p.quizStudents(true) {
		got := p.votes[who]
		earned, total := q.grade(got.answers)
		answered := map[int]bool{}
		for _, a := range got.answers {
			answered[a.Q] = true
		}
		s := QuizScore{Student: who, Score: total, Answered: len(got.answers), Submitted: got.submitted,
			Earned: make([]int, len(q.Questions))}
		for i := range q.Questions {
			s.Earned[i] = -1
			if !answered[i] {
				continue
			}
			s.Earned[i] = earned[i]
			qv := &v.Questions[i]
			if who.Simulated {
				qv.AnsweredSimulated++
				if earned[i] > 0 {
					qv.RightSimulated++
				}
			} else {
				qv.Answered++
				if earned[i] > 0 {
					qv.Right++
				}
			}
		}
		switch {
		case got.submitted && who.Simulated:
			v.SubmittedSimulated++
		case got.submitted:
			v.Submitted++
		}
		v.Scores = append(v.Scores, s)
		if !who.Simulated && !p.open {
			v.Exportable = true
		}
	}
	return v
}

// quizStudents are who got the quiz or answered it, sorted; the simulated
// ones only when sims.
func (p *poll) quizStudents(sims bool) []Student {
	seen := map[Student]bool{}
	var out []Student
	add := func(who Student) {
		if !seen[who] && (sims || !who.Simulated) {
			seen[who] = true
			out = append(out, who)
		}
	}
	for who := range p.delivered {
		add(who)
	}
	for who := range p.votes {
		add(who)
	}
	slices.SortFunc(out, cmpStudent)
	return out
}

// Grades returns the CSV of a closed quiz: one row per real student who got
// it, never the simulated ones (rule 91). It is UTF-8 with a BOM and «;» so
// a spreadsheet in Spanish opens it as columns. Title is the quiz's title.
func (c *Class) Grades(pollID string) (data []byte, title string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.find(pollID)
	switch {
	case p == nil:
		return nil, "", ErrGone
	case p.quiz == nil:
		return nil, "", ErrNotQuiz
	case p.open:
		return nil, "", ErrStillOpen
	}
	q := p.quiz
	var buf bytes.Buffer
	buf.WriteString("\uFEFF")
	w := csv.NewWriter(&buf)
	w.Comma, w.UseCRLF = ';', true
	head := []string{"Equipo", "Usuario", "Puntaje", "Máximo", "Entregó"}
	for i, qq := range q.Questions {
		head = append(head, fmt.Sprintf("P%d (%d)", i+1, qq.Points))
	}
	if err := w.Write(head); err != nil {
		return nil, "", fmt.Errorf("grades: %w", err)
	}
	for _, who := range p.quizStudents(false) {
		got := p.votes[who]
		earned, total := q.grade(got.answers)
		entrego := "no"
		if got.submitted {
			entrego = "sí"
		}
		row := []string{csvSafe(who.Hostname), csvSafe(who.Username), strconv.Itoa(total), strconv.Itoa(q.Max()), entrego}
		for _, e := range earned {
			row = append(row, strconv.Itoa(e))
		}
		if err := w.Write(row); err != nil {
			return nil, "", fmt.Errorf("grades: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", fmt.Errorf("grades: %w", err)
	}
	return buf.Bytes(), q.Title, nil
}

// csvSafe keeps a spreadsheet from reading a value as a formula.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}
