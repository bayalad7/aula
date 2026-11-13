package protocol

import (
	"strings"
	"unicode/utf8"
)

// Limits of the poll.* messages (docs/protocol.md §5.23, §7).
const (
	MaxPollQuestion = 200
	MaxPollOption   = 80
	MinPollOptions  = 2
	MaxPollOptions  = 6
	MaxPollText     = 140
	MaxPollVoters   = 10000

	// Quizzes (§5.23.1).
	MaxQuizTitle     = 80
	MaxQuizQuestions = 30
	MaxQuizPoints    = 100
	MaxQuizScore     = MaxQuizQuestions * MaxQuizPoints
)

// PollKind is the kind of question of a poll.
type PollKind string

// Poll kinds.
const (
	PollKindChoice PollKind = "choice" // multiple choice, 2–6 options
	PollKindYesNo  PollKind = "yesno"  // yes (0) or no (1); the window writes the words
	PollKindOpen   PollKind = "open"   // short free text
	// PollKindQuiz is a quiz: several choice or yesno questions graded by
	// the console (§5.23.1). Only for agents that declared poll.quiz.
	PollKindQuiz PollKind = "quiz"
)

// Options returns how many options a vote may choose from: len(options) for
// choice, 2 for yesno and 0 for open.
func (k PollKind) Options(options []string) int {
	switch k {
	case PollKindChoice:
		return len(options)
	case PollKindYesNo:
		return 2
	}
	return 0
}

// PollVoteStatus is the state a poll.vote.ack reports.
type PollVoteStatus string

// Vote statuses.
const (
	PollVoteAccepted PollVoteStatus = "accepted" // it counts, replacing the previous one
	PollVoteClosed   PollVoteStatus = "closed"   // the poll was already closed
	PollVoteUnknown  PollVoteStatus = "unknown"  // unknown poll, or not for this student
)

// PollAckStatus is the state a poll.ack reports.
type PollAckStatus string

// Poll ack statuses.
const (
	PollAckShown   PollAckStatus = "shown"   // answers poll.open
	PollAckClosed  PollAckStatus = "closed"  // answers poll.close
	PollAckResults PollAckStatus = "results" // answers poll.results
)

// PollOpen shows a question to the student (P → A). Named says whether the
// teacher sees each student's answer; the card says so before answering.
//
// A quiz (Kind PollKindQuiz) carries its title in Question and its questions
// in Questions, never the correct answers; on a resend, Answers and
// Submitted are what that student already had acknowledged.
type PollOpen struct {
	PollID    string             `json:"poll_id"`
	Kind      PollKind           `json:"kind"`
	Question  string             `json:"question"`
	Options   []string           `json:"options,omitempty"`
	Named     bool               `json:"named"`
	Questions []PollQuizQuestion `json:"questions,omitempty"`
	Answers   []PollQuizAnswer   `json:"answers,omitempty"`
	Submitted bool               `json:"submitted,omitempty"`
}

// PollQuizOption is one option of a quiz question: ID is its number in the
// saved quiz (0–5), whatever order the student sees.
type PollQuizOption struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

// PollQuizQuestion is one question of a quiz as the student sees it: never
// which options are correct. Multi says it has more than one correct option
// (the student ticks boxes), not which nor how many.
type PollQuizQuestion struct {
	ID      int              `json:"id"`
	Kind    PollKind         `json:"kind"`
	Text    string           `json:"text"`
	Points  int              `json:"points"`
	Multi   bool             `json:"multi,omitempty"`
	Options []PollQuizOption `json:"options,omitempty"`
}

// PollQuizAnswer is the answer to one quiz question: Q is the question ID
// and Choices the option IDs (yesno: 0 yes, 1 no).
type PollQuizAnswer struct {
	Q       int   `json:"q"`
	Choices []int `json:"choices"`
}

// Validate checks a question on its own.
func (q PollQuizQuestion) Validate() error {
	if q.ID < 0 || q.ID >= MaxQuizQuestions {
		return invalid(TypePollOpen, "a question id must be 0-%d", MaxQuizQuestions-1)
	}
	if n := utf8.RuneCountInString(q.Text); strings.TrimSpace(q.Text) == "" || n > MaxPollQuestion {
		return invalid(TypePollOpen, "a question must be 1-%d characters", MaxPollQuestion)
	}
	if q.Points < 1 || q.Points > MaxQuizPoints {
		return invalid(TypePollOpen, "a question is worth 1-%d points", MaxQuizPoints)
	}
	switch q.Kind {
	case PollKindChoice:
		if len(q.Options) < MinPollOptions || len(q.Options) > MaxPollOptions {
			return invalid(TypePollOpen, "a choice question has %d-%d options", MinPollOptions, MaxPollOptions)
		}
		seen := map[int]bool{}
		for _, o := range q.Options {
			if o.ID < 0 || o.ID >= len(q.Options) || seen[o.ID] {
				return invalid(TypePollOpen, "option ids are 0-%d, once each", len(q.Options)-1)
			}
			seen[o.ID] = true
			if strings.TrimSpace(o.Text) == "" || utf8.RuneCountInString(o.Text) > MaxPollOption {
				return invalid(TypePollOpen, "each option must be 1-%d characters", MaxPollOption)
			}
		}
	case PollKindYesNo:
		if len(q.Options) != 0 || q.Multi {
			return invalid(TypePollOpen, "a yes/no question has no options and one answer")
		}
	default:
		return invalid(TypePollOpen, "a question kind must be choice or yesno")
	}
	return nil
}

// OptionCount is how many options a question has (yesno: 2).
func (q PollQuizQuestion) OptionCount() int {
	if q.Kind == PollKindYesNo {
		return 2
	}
	return len(q.Options)
}

// validAnswersShape checks answers without the questions: at most one per
// question, 1-6 distinct options each.
func validAnswersShape(t Type, answers []PollQuizAnswer) error {
	if len(answers) > MaxQuizQuestions {
		return invalid(t, "at most %d answers", MaxQuizQuestions)
	}
	seen := map[int]bool{}
	for _, a := range answers {
		if a.Q < 0 || a.Q >= MaxQuizQuestions || seen[a.Q] {
			return invalid(t, "one answer per question id 0-%d", MaxQuizQuestions-1)
		}
		seen[a.Q] = true
		if len(a.Choices) < 1 || len(a.Choices) > MaxPollOptions {
			return invalid(t, "an answer chooses 1-%d options", MaxPollOptions)
		}
		picked := map[int]bool{}
		for _, c := range a.Choices {
			if c < 0 || c >= MaxPollOptions || picked[c] {
				return invalid(t, "chosen options are 0-%d, once each", MaxPollOptions-1)
			}
			picked[c] = true
		}
	}
	return nil
}

// ValidQuizAnswers checks answers against the questions of a quiz: known
// questions, known options, and a single option where the question is not
// Multi.
func ValidQuizAnswers(answers []PollQuizAnswer, questions []PollQuizQuestion) error {
	if err := validAnswersShape(TypePollVote, answers); err != nil {
		return err
	}
	byID := map[int]PollQuizQuestion{}
	for _, q := range questions {
		byID[q.ID] = q
	}
	for _, a := range answers {
		q, ok := byID[a.Q]
		if !ok {
			return invalid(TypePollVote, "the quiz has no question %d", a.Q)
		}
		if !q.Multi && len(a.Choices) != 1 {
			return invalid(TypePollVote, "question %d takes one option", a.Q)
		}
		for _, c := range a.Choices {
			if c >= q.OptionCount() {
				return invalid(TypePollVote, "question %d has no option %d", a.Q, c)
			}
		}
	}
	return nil
}

// MessageType implements Payload.
func (PollOpen) MessageType() Type { return TypePollOpen }

// Validate implements Payload.
func (p PollOpen) Validate() error {
	if !isUUIDv4(p.PollID) {
		return invalid(TypePollOpen, "poll_id is not a UUID v4")
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(p.Question)); n < 1 || utf8.RuneCountInString(p.Question) > MaxPollQuestion {
		return invalid(TypePollOpen, "question must be 1-%d characters", MaxPollQuestion)
	}
	switch p.Kind {
	case PollKindChoice:
		if len(p.Options) < MinPollOptions || len(p.Options) > MaxPollOptions {
			return invalid(TypePollOpen, "a choice poll has %d-%d options", MinPollOptions, MaxPollOptions)
		}
		for _, o := range p.Options {
			if strings.TrimSpace(o) == "" || utf8.RuneCountInString(o) > MaxPollOption {
				return invalid(TypePollOpen, "each option must be 1-%d characters", MaxPollOption)
			}
		}
	case PollKindYesNo, PollKindOpen:
		if len(p.Options) != 0 {
			return invalid(TypePollOpen, "only a choice poll has options")
		}
	case PollKindQuiz:
		return p.validateQuiz()
	default:
		return invalid(TypePollOpen, "kind must be choice, yesno, open or quiz")
	}
	if len(p.Questions) != 0 || len(p.Answers) != 0 || p.Submitted {
		return invalid(TypePollOpen, "only a quiz has questions and answers")
	}
	return nil
}

func (p PollOpen) validateQuiz() error {
	if utf8.RuneCountInString(p.Question) > MaxQuizTitle {
		return invalid(TypePollOpen, "a quiz title must be 1-%d characters", MaxQuizTitle)
	}
	if len(p.Options) != 0 || !p.Named {
		return invalid(TypePollOpen, "a quiz is named and has its options in each question")
	}
	if len(p.Questions) < 1 || len(p.Questions) > MaxQuizQuestions {
		return invalid(TypePollOpen, "a quiz has 1-%d questions", MaxQuizQuestions)
	}
	seen := map[int]bool{}
	for _, q := range p.Questions {
		if err := q.Validate(); err != nil {
			return err
		}
		if seen[q.ID] {
			return invalid(TypePollOpen, "question ids are unique")
		}
		seen[q.ID] = true
	}
	if ValidQuizAnswers(p.Answers, p.Questions) != nil {
		return invalid(TypePollOpen, "the answers do not fit the quiz")
	}
	return nil
}

// PollVote is the student's answer (A → P): Choice for choice and yesno polls,
// Text for open ones, Answers and Submitted for a quiz (§5.23.1). Exactly one
// of them.
type PollVote struct {
	PollID    string           `json:"poll_id"`
	Choice    *int             `json:"choice,omitempty"`
	Text      string           `json:"text,omitempty"`
	Answers   []PollQuizAnswer `json:"answers,omitempty"`
	Submitted bool             `json:"submitted,omitempty"`
}

// IsQuiz reports whether the vote answers a quiz.
func (p PollVote) IsQuiz() bool { return len(p.Answers) > 0 || p.Submitted }

// MessageType implements Payload.
func (PollVote) MessageType() Type { return TypePollVote }

// Validate implements Payload.
func (p PollVote) Validate() error {
	if !isUUIDv4(p.PollID) {
		return invalid(TypePollVote, "poll_id is not a UUID v4")
	}
	kinds := 0
	for _, has := range []bool{p.Choice != nil, p.Text != "", p.IsQuiz()} {
		if has {
			kinds++
		}
	}
	if kinds != 1 {
		return invalid(TypePollVote, "a vote carries either choice, text or answers")
	}
	if p.IsQuiz() {
		return validAnswersShape(TypePollVote, p.Answers)
	}
	if p.Choice != nil && (*p.Choice < 0 || *p.Choice >= MaxPollOptions) {
		return invalid(TypePollVote, "choice must be 0-%d", MaxPollOptions-1)
	}
	if p.Choice == nil {
		if n := utf8.RuneCountInString(p.Text); strings.TrimSpace(p.Text) == "" || n > MaxPollText {
			return invalid(TypePollVote, "text must be 1-%d characters", MaxPollText)
		}
	}
	return nil
}

// ValidFor checks the vote against the poll it answers: a choice within the
// options of a choice or yesno poll, or a text for an open one.
func (p PollVote) ValidFor(kind PollKind, options []string) error {
	if (kind == PollKindQuiz) != p.IsQuiz() {
		return invalid(TypePollVote, "only a quiz takes answers")
	}
	if kind == PollKindQuiz {
		return nil // checked against its questions with ValidQuizAnswers
	}
	if kind == PollKindOpen {
		if p.Choice != nil {
			return invalid(TypePollVote, "an open poll takes text")
		}
		return nil
	}
	if p.Choice == nil {
		return invalid(TypePollVote, "this poll takes a choice")
	}
	if *p.Choice >= kind.Options(options) {
		return invalid(TypePollVote, "choice is not one of the options")
	}
	return nil
}

// PollVoteAck tells the agent whether its answer counts (P → A).
type PollVoteAck struct {
	RefID  string         `json:"ref_id"`
	PollID string         `json:"poll_id"`
	Status PollVoteStatus `json:"status"`
}

// MessageType implements Payload.
func (PollVoteAck) MessageType() Type { return TypePollVoteAck }

// Validate implements Payload.
func (p PollVoteAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypePollVoteAck, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.PollID) {
		return invalid(TypePollVoteAck, "poll_id is not a UUID v4")
	}
	switch p.Status {
	case PollVoteAccepted, PollVoteClosed, PollVoteUnknown:
	default:
		return invalid(TypePollVoteAck, "status must be accepted, closed or unknown")
	}
	return nil
}

// PollClose stops accepting answers (P → A).
type PollClose struct {
	PollID string `json:"poll_id"`
}

// MessageType implements Payload.
func (PollClose) MessageType() Type { return TypePollClose }

// Validate implements Payload.
func (p PollClose) Validate() error {
	if !isUUIDv4(p.PollID) {
		return invalid(TypePollClose, "poll_id is not a UUID v4")
	}
	return nil
}

// PollResults shares the totals of a closed poll (P → A). Never who chose
// what.
type PollResults struct {
	PollID     string `json:"poll_id"`
	Counts     []int  `json:"counts"`
	Voters     int    `json:"voters"`
	Recipients int    `json:"recipients"`
}

// MessageType implements Payload.
func (PollResults) MessageType() Type { return TypePollResults }

// Validate implements Payload.
func (p PollResults) Validate() error {
	if !isUUIDv4(p.PollID) {
		return invalid(TypePollResults, "poll_id is not a UUID v4")
	}
	if len(p.Counts) < MinPollOptions || len(p.Counts) > MaxPollOptions {
		return invalid(TypePollResults, "counts must have %d-%d entries", MinPollOptions, MaxPollOptions)
	}
	sum := 0
	for _, c := range p.Counts {
		if c < 0 || c > MaxPollVoters {
			return invalid(TypePollResults, "each count must be 0-%d", MaxPollVoters)
		}
		sum += c
	}
	if p.Voters < 0 || p.Voters > MaxPollVoters || sum != p.Voters {
		return invalid(TypePollResults, "voters must be the sum of counts, 0-%d", MaxPollVoters)
	}
	if p.Recipients < p.Voters || p.Recipients > MaxPollVoters {
		return invalid(TypePollResults, "recipients must be voters-%d", MaxPollVoters)
	}
	return nil
}

// PollAck reports what the window did with a poll message (A → P).
type PollAck struct {
	RefID  string        `json:"ref_id"`
	PollID string        `json:"poll_id"`
	Status PollAckStatus `json:"status"`
}

// MessageType implements Payload.
func (PollAck) MessageType() Type { return TypePollAck }

// Validate implements Payload.
func (p PollAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypePollAck, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.PollID) {
		return invalid(TypePollAck, "poll_id is not a UUID v4")
	}
	switch p.Status {
	case PollAckShown, PollAckClosed, PollAckResults:
	default:
		return invalid(TypePollAck, "status must be shown, closed or results")
	}
	return nil
}

// PollQuizResult is one student's own result of a closed quiz (P → A): never
// anyone else's, nor the class totals (§5.23.1).
type PollQuizResult struct {
	PollID    string                   `json:"poll_id"`
	Score     int                      `json:"score"`
	Max       int                      `json:"max"`
	Questions []PollQuizResultQuestion `json:"questions"`
}

// PollQuizResultQuestion is the result of one question: what the student
// chose (empty if not answered), the correct options and the points earned.
type PollQuizResultQuestion struct {
	Q       int   `json:"q"`
	Choices []int `json:"choices"`
	Correct []int `json:"correct"`
	Earned  int   `json:"earned"`
}

// MessageType implements Payload.
func (PollQuizResult) MessageType() Type { return TypePollQuizResult }

// Validate implements Payload.
func (p PollQuizResult) Validate() error {
	t := TypePollQuizResult
	if !isUUIDv4(p.PollID) {
		return invalid(t, "poll_id is not a UUID v4")
	}
	if p.Max < 0 || p.Max > MaxQuizScore || p.Score < 0 || p.Score > p.Max {
		return invalid(t, "score must be 0-max, max 0-%d", MaxQuizScore)
	}
	if len(p.Questions) < 1 || len(p.Questions) > MaxQuizQuestions {
		return invalid(t, "a result has 1-%d questions", MaxQuizQuestions)
	}
	seen := map[int]bool{}
	sum := 0
	for _, q := range p.Questions {
		if q.Q < 0 || q.Q >= MaxQuizQuestions || seen[q.Q] {
			return invalid(t, "one entry per question id 0-%d", MaxQuizQuestions-1)
		}
		seen[q.Q] = true
		if len(q.Choices) > MaxPollOptions || len(q.Correct) < 1 || len(q.Correct) > MaxPollOptions {
			return invalid(t, "choices are 0-%d and correct 1-%d options", MaxPollOptions, MaxPollOptions)
		}
		for _, list := range [][]int{q.Choices, q.Correct} {
			for _, c := range list {
				if c < 0 || c >= MaxPollOptions {
					return invalid(t, "options are 0-%d", MaxPollOptions-1)
				}
			}
		}
		if q.Earned < 0 || q.Earned > MaxQuizPoints {
			return invalid(t, "earned must be 0-%d", MaxQuizPoints)
		}
		sum += q.Earned
	}
	if sum != p.Score {
		return invalid(t, "score must be the sum of earned")
	}
	return nil
}
