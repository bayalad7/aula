package poll

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// MaxPolls is how many polls a class keeps in memory; past it, the oldest
// closed one is dropped.
const MaxPolls = 20

// Pace of poll.vote per agent: a burst of 3, then one per second.
const (
	VoteBurst = 3
	VoteEvery = time.Second
)

// Errors of the console side.
var (
	ErrInvalid      = errors.New("invalid poll")
	ErrNotOpen      = errors.New("no poll is open")
	ErrGone         = errors.New("the poll is no longer in the class")
	ErrStillOpen    = errors.New("the poll is still open")
	ErrNotShareable = errors.New("open answers are not shared")
	ErrTooFast      = errors.New("too fast")
)

// Student is who answers, as the rest of the console tells them apart:
// computer and user, and whether it is a simulated student.
type Student struct {
	Hostname  string `json:"hostname"`
	Username  string `json:"username"`
	Simulated bool   `json:"simulated"`
}

// Command is a message to send to one session.
type Command struct {
	SessionID string
	Payload   protocol.Payload
}

// Request is what the «Encuesta» panel asks for.
type Request struct {
	Kind     protocol.PollKind `json:"kind"`
	Question string            `json:"question"`
	Options  []string          `json:"options"`
	Named    bool              `json:"named"`
	Hosts    []string          `json:"hosts"` // the selection; empty: the whole channel
}

// Option is one bar: how many real and simulated students chose it. Label is
// empty in a yes/no poll: the window writes «Sí» and «No».
type Option struct {
	Label     string `json:"label"`
	Count     int    `json:"count"`
	Simulated int    `json:"simulated"`
}

// Answer is one answer the teacher may see. In an anonymous poll only the
// text of open answers is listed, without the student (rule 72).
type Answer struct {
	Student
	Choice int       `json:"choice"` // -1 for a text answer
	Text   string    `json:"text"`
	At     time.Time `json:"at"`
}

// PollView is one poll for the console: the franja and «Participación».
type PollView struct {
	PollID    string            `json:"pollId"`
	Kind      protocol.PollKind `json:"kind"`
	Question  string            `json:"question"`
	Named     bool              `json:"named"`
	Open      bool              `json:"open"`
	Shared    bool              `json:"shared"`
	Shareable bool              `json:"shareable"` // closed, not an open question, not shared yet
	All       bool              `json:"all"`
	Hosts     []string          `json:"hosts"`
	OpenedAt  time.Time         `json:"openedAt"`
	ClosedAt  *time.Time        `json:"closedAt"`
	Options   []Option          `json:"options"`
	// Answered counts the real students' answers; AnsweredSimulated, the
	// simulated ones, always apart (rule 32).
	Answered          int `json:"answered"`
	AnsweredSimulated int `json:"answeredSimulated"`
	Received          int `json:"received"` // students whose window shows it
	// Eligible and Unsupported are the connected targets that can and cannot
	// show it; only while it is open.
	Eligible    int       `json:"eligible"`
	Unsupported []string  `json:"unsupported"`
	Answers     []Answer  `json:"answers"`
	Missing     []Student `json:"missing"` // named and open: who has not answered
	// Quiz is set for a quiz (§5.23.1): its questions with the correct
	// options, the scores and how each question went.
	Quiz *QuizView `json:"quiz"`
}

// View is every poll of the class.
type View struct {
	Current *PollView  `json:"current"` // the open one, if any
	Closed  []PollView `json:"closed"`  // newest first
}

type vote struct {
	choice    int
	text      string
	answers   []protocol.PollQuizAnswer // a quiz: every answer so far
	submitted bool                      // a quiz: «Entregar»
	at        time.Time
}

type poll struct {
	id       string
	kind     protocol.PollKind
	question string
	options  []string
	named    bool
	hosts    map[string]bool // nil: the whole channel
	open     bool
	shared   bool
	openedAt time.Time
	closedAt time.Time
	quiz     *Quiz // a quiz (§5.23.1); nil for a poll of one question

	votes     map[Student]vote
	delivered map[Student]bool // poll.open was sent to them
	shown     map[Student]bool // their window acked it
}

// studentState is what one student's agent has of the polls.
type studentState struct {
	sessionID string
	pollID    string // the last poll sent to them
	// What was sent on sessionID; a new session sends it again.
	openSent, closeSent, resultsSent bool
	closeAcked, resultsAcked         bool
}

// Class is the console side of the polls: one per class. It is safe for
// concurrent use.
type Class struct {
	mu  sync.Mutex
	now func() time.Time

	polls     []*poll // oldest first; only the last one may be open
	students  map[Student]*studentState
	bySession map[string]Student
	pace      map[string]*bucket // by session ID
}

// NewClass returns a class without polls. now is time.Now when nil.
func NewClass(now func() time.Time) *Class {
	if now == nil {
		now = time.Now
	}
	c := &Class{now: now}
	c.reset()
	return c
}

func (c *Class) reset() {
	c.polls = nil
	c.students = map[Student]*studentState{}
	c.bySession = map[string]Student{}
	c.pace = map[string]*bucket{}
}

// Open opens a new poll for the hosts (hostnames, without case) or the whole
// channel when there are none, and closes the one in force. Plan then says
// what to send.
func (c *Class) Open(req Request) (string, error) {
	msg := protocol.PollOpen{PollID: protocol.NewID(), Kind: req.Kind,
		Question: strings.TrimSpace(req.Question), Named: req.Named}
	if req.Kind == protocol.PollKindChoice {
		for _, o := range req.Options {
			if o = strings.TrimSpace(o); o != "" {
				msg.Options = append(msg.Options, o)
			}
		}
	}
	if err := msg.Validate(); err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if cur := c.current(); cur != nil {
		cur.open, cur.closedAt = false, now
	}
	p := &poll{id: msg.PollID, kind: msg.Kind, question: msg.Question, options: msg.Options,
		named: msg.Named, open: true, openedAt: now,
		votes: map[Student]vote{}, delivered: map[Student]bool{}, shown: map[Student]bool{}}
	p.setHosts(req.Hosts)
	c.add(p)
	return p.id, nil
}

// OpenQuiz opens a quiz (§5.23.1) for the hosts, or the whole channel when
// there are none, and closes the poll in force. Only the agents that
// declared poll.quiz get it.
func (c *Class) OpenQuiz(q Quiz, hosts []string) (string, error) {
	q = q.Normalize()
	if err := q.Validate(); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if cur := c.current(); cur != nil {
		cur.open, cur.closedAt = false, now
	}
	p := &poll{id: protocol.NewID(), kind: protocol.PollKindQuiz, question: q.Title, named: true,
		open: true, openedAt: now, quiz: &q,
		votes: map[Student]vote{}, delivered: map[Student]bool{}, shown: map[Student]bool{}}
	p.setHosts(hosts)
	c.add(p)
	return p.id, nil
}

// add appends p and drops the oldest past MaxPolls; c.mu is held.
func (c *Class) add(p *poll) {
	c.polls = append(c.polls, p)
	if len(c.polls) > MaxPolls {
		c.polls = slices.Delete(c.polls, 0, len(c.polls)-MaxPolls)
	}
}

func (p *poll) setHosts(hosts []string) {
	for _, h := range hosts {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			if p.hosts == nil {
				p.hosts = map[string]bool{}
			}
			p.hosts[h] = true
		}
	}
}

// Close closes the poll in force: no more answers count.
func (c *Class) Close() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cur := c.current()
	if cur == nil {
		return "", ErrNotOpen
	}
	cur.open, cur.closedAt = false, c.now()
	return cur.id, nil
}

// Share marks a closed poll to send its totals to the students who got it.
func (c *Class) Share(pollID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.find(pollID)
	switch {
	case p == nil:
		return ErrGone
	case p.open:
		return ErrStillOpen
	case p.kind == protocol.PollKindOpen:
		return ErrNotShareable
	}
	// A quiz: each student gets only their own result (rule 90).
	p.shared = true
	return nil
}

// Reset forgets everything (the class closed or changed channel); nothing
// is sent.
func (c *Class) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reset()
}

// Vote records the answer of a student on session sessionID. It returns the
// status for poll.vote.ack, and ErrTooFast (answer error invalid_message, no
// ack) or ErrInvalid (a vote that does not fit the poll).
func (c *Class) Vote(sessionID string, who Student, v protocol.PollVote) (protocol.PollVoteStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	b := c.pace[sessionID]
	if b == nil {
		b = newBucket(VoteBurst, VoteEvery, now)
		c.pace[sessionID] = b
	}
	if !b.take(now) {
		return "", ErrTooFast
	}
	p := c.find(v.PollID)
	if p == nil || !p.targets(who.Hostname) {
		return protocol.PollVoteUnknown, nil
	}
	if !p.open {
		return protocol.PollVoteClosed, nil
	}
	if err := v.ValidFor(p.kind, p.options); err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if p.quiz != nil {
		if err := protocol.ValidQuizAnswers(v.Answers, p.quiz.wire("")); err != nil {
			return "", fmt.Errorf("%w: %w", ErrInvalid, err)
		}
		if p.votes[who].submitted {
			return protocol.PollVoteClosed, nil // handed in: it does not change
		}
		p.votes[who] = vote{choice: -1, answers: normalizeAnswers(v.Answers), submitted: v.Submitted, at: now}
		p.delivered[who] = true
		return protocol.PollVoteAccepted, nil
	}
	got := vote{choice: -1, text: strings.TrimSpace(v.Text), at: now}
	if v.Choice != nil {
		got.choice, got.text = *v.Choice, ""
	}
	p.votes[who] = got
	p.delivered[who] = true
	return protocol.PollVoteAccepted, nil
}

// Plan returns what must be sent so every connected session that can show
// polls, real or simulated, has the last poll meant for it: poll.open while it
// is open; poll.close and, if shared, poll.results once it closed (also after
// a reconnection). What it returns counts as sent: call Failed for a command
// that could not be sent.
func (c *Class) Plan(real, sims []session.Session) []Command {
	c.mu.Lock()
	defer c.mu.Unlock()
	live := map[string]bool{}
	var cmds []Command
	add := func(list []session.Session, simulated bool) {
		for _, s := range list {
			if s.State != session.StateConnected || !s.Can(protocol.CapPoll) {
				continue
			}
			live[s.ID] = true
			who := Student{Hostname: s.Hostname, Username: s.Username, Simulated: simulated}
			c.bySession[s.ID] = who
			cmds = append(cmds, c.planFor(s.ID, who, s.Can(protocol.CapPollQuiz))...)
		}
	}
	add(real, false)
	add(sims, true)
	for id := range c.bySession {
		if !live[id] {
			delete(c.bySession, id)
			delete(c.pace, id)
		}
	}
	return cmds
}

// planFor brings one student up to date; c.mu is held.
func (c *Class) planFor(sessionID string, who Student, canQuiz bool) []Command {
	st := c.students[who]
	if st == nil {
		st = &studentState{}
		c.students[who] = st
	}
	if st.sessionID != sessionID {
		st.sessionID = sessionID
		st.openSent, st.closeSent, st.resultsSent = false, false, false
	}
	last := c.latestFor(who.Hostname, canQuiz)
	if last == nil {
		return nil
	}
	var cmds []Command
	if st.pollID != last.id {
		// Close the card of the previous one if the agent may still show it
		// open (a dropped poll may still be on its window).
		if old := c.find(st.pollID); st.pollID != "" && !st.closeAcked && (old == nil || old.delivered[who]) {
			cmds = append(cmds, Command{sessionID, protocol.PollClose{PollID: st.pollID}})
		}
		*st = studentState{sessionID: sessionID, pollID: last.id}
	}
	switch {
	case last.open:
		if !st.openSent {
			cmds = append(cmds, Command{sessionID, last.openMsgFor(who)})
			st.openSent = true
			last.delivered[who] = true
		}
	case last.delivered[who]:
		if !st.closeAcked && !st.closeSent {
			cmds = append(cmds, Command{sessionID, protocol.PollClose{PollID: last.id}})
			st.closeSent = true
		}
		if last.shared && !st.resultsAcked && !st.resultsSent {
			if last.quiz != nil {
				// Only their own result (rule 90).
				cmds = append(cmds, Command{sessionID, last.quiz.result(last.id, last.votes[who].answers)})
			} else {
				cmds = append(cmds, Command{sessionID, last.results()})
			}
			st.resultsSent = true
		}
	}
	return cmds
}

// Failed undoes what Plan recorded for a command that could not be sent, so
// the next Plan tries again.
func (c *Class) Failed(cmd Command) {
	c.mu.Lock()
	defer c.mu.Unlock()
	who, ok := c.bySession[cmd.SessionID]
	st := c.students[who]
	if !ok || st == nil || st.sessionID != cmd.SessionID {
		return
	}
	switch p := cmd.Payload.(type) {
	case protocol.PollOpen:
		if p.PollID == st.pollID {
			st.openSent = false
		}
	case protocol.PollClose:
		if p.PollID == st.pollID {
			st.closeSent = false
		}
	case protocol.PollResults:
		if p.PollID == st.pollID {
			st.resultsSent = false
		}
	case protocol.PollQuizResult:
		if p.PollID == st.pollID {
			st.resultsSent = false
		}
	}
}

// Ack records what an agent said about a poll. It reports whether the view
// changed.
func (c *Class) Ack(sessionID string, a protocol.PollAck) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	who, ok := c.bySession[sessionID]
	if !ok {
		return false
	}
	st := c.students[who]
	switch a.Status {
	case protocol.PollAckShown:
		if p := c.find(a.PollID); p != nil && !p.shown[who] {
			p.shown[who] = true
			return true
		}
	case protocol.PollAckClosed:
		if st != nil && st.pollID == a.PollID {
			st.closeAcked = true
		}
	case protocol.PollAckResults:
		if st != nil && st.pollID == a.PollID {
			st.resultsAcked = true
		}
	}
	return false
}

// IsOpen reports whether a poll is open.
func (c *Class) IsOpen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current() != nil
}

// Snapshot returns the polls for the console. real and sims are the class's
// sessions, to tell who can still answer the open one.
func (c *Class) Snapshot(real, sims []session.Session) View {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := View{Closed: []PollView{}}
	for i := len(c.polls) - 1; i >= 0; i-- {
		p := c.polls[i]
		v := p.view()
		if p.open {
			p.addEligible(&v, real, false)
			p.addEligible(&v, sims, true)
			slices.Sort(v.Unsupported)
			out.Current = &v
			continue
		}
		out.Closed = append(out.Closed, v)
	}
	return out
}

// current is the open poll; c.mu is held.
func (c *Class) current() *poll {
	if n := len(c.polls); n > 0 && c.polls[n-1].open {
		return c.polls[n-1]
	}
	return nil
}

// find returns the poll with id; c.mu is held.
func (c *Class) find(id string) *poll {
	for _, p := range c.polls {
		if p.id == id {
			return p
		}
	}
	return nil
}

// latestFor is the newest poll meant for hostname that its agent can show
// (a quiz only with poll.quiz); c.mu is held.
func (c *Class) latestFor(hostname string, canQuiz bool) *poll {
	for i := len(c.polls) - 1; i >= 0; i-- {
		if c.polls[i].targets(hostname) && (canQuiz || c.polls[i].quiz == nil) {
			return c.polls[i]
		}
	}
	return nil
}

func (p *poll) targets(hostname string) bool {
	return p.hosts == nil || p.hosts[strings.ToLower(hostname)]
}

// openMsgFor is the poll.open for one student. A quiz carries its questions
// in that student's order, never the correct options (rule 88), and what
// that student already had acknowledged.
func (p *poll) openMsgFor(who Student) protocol.PollOpen {
	m := protocol.PollOpen{PollID: p.id, Kind: p.kind, Question: p.question,
		Options: p.options, Named: p.named}
	if p.quiz != nil {
		m.Questions = p.quiz.wire(seedOf(p.id, who))
		if got, ok := p.votes[who]; ok {
			m.Answers = slices.Clone(got.answers)
			m.Submitted = got.submitted
		}
	}
	return m
}

// results are the totals the students may see: real students only (rule 32).
func (p *poll) results() protocol.PollResults {
	counts := make([]int, p.kind.Options(p.options))
	voters := 0
	for who, v := range p.votes {
		if who.Simulated || v.choice < 0 || v.choice >= len(counts) {
			continue
		}
		counts[v.choice]++
		voters++
	}
	recipients := 0
	for who := range p.delivered {
		if !who.Simulated {
			recipients++
		}
	}
	return protocol.PollResults{PollID: p.id, Counts: counts, Voters: voters, Recipients: max(recipients, voters)}
}

func (p *poll) view() PollView {
	v := PollView{PollID: p.id, Kind: p.kind, Question: p.question, Named: p.named, Open: p.open,
		Shared: p.shared, All: p.hosts == nil, Hosts: []string{}, OpenedAt: p.openedAt,
		Options: []Option{}, Unsupported: []string{}, Answers: []Answer{}, Missing: []Student{}}
	v.Shareable = !p.open && !p.shared && p.kind != protocol.PollKindOpen
	if p.quiz != nil {
		v.Quiz = p.quizView()
		for who := range p.votes {
			if who.Simulated {
				v.AnsweredSimulated++
			} else {
				v.Answered++
			}
		}
		v.Received = len(p.shown)
		return v
	}
	if !p.open {
		at := p.closedAt
		v.ClosedAt = &at
	}
	for h := range p.hosts {
		v.Hosts = append(v.Hosts, h)
	}
	slices.Sort(v.Hosts)
	for i := range p.kind.Options(p.options) {
		o := Option{}
		if p.kind == protocol.PollKindChoice {
			o.Label = p.options[i]
		}
		v.Options = append(v.Options, o)
	}
	for who, got := range p.votes {
		if who.Simulated {
			v.AnsweredSimulated++
		} else {
			v.Answered++
		}
		if got.choice >= 0 && got.choice < len(v.Options) {
			if who.Simulated {
				v.Options[got.choice].Simulated++
			} else {
				v.Options[got.choice].Count++
			}
		}
		switch {
		case p.named:
			v.Answers = append(v.Answers, Answer{Student: who, Choice: got.choice, Text: got.text, At: got.at})
		case got.choice < 0:
			// Anonymous: the text without who wrote it, nor when (rule 72).
			v.Answers = append(v.Answers, Answer{Student: Student{Simulated: who.Simulated}, Choice: -1, Text: got.text})
		}
	}
	if p.named {
		slices.SortFunc(v.Answers, func(a, b Answer) int {
			return cmpStudent(a.Student, b.Student)
		})
	} else {
		slices.SortFunc(v.Answers, func(a, b Answer) int { return strings.Compare(a.Text, b.Text) })
	}
	v.Received = len(p.shown)
	return v
}

// addEligible counts the connected targets of the open poll in list.
func (p *poll) addEligible(v *PollView, list []session.Session, simulated bool) {
	for _, s := range list {
		if s.State != session.StateConnected || !p.targets(s.Hostname) {
			continue
		}
		if !s.Can(protocol.CapPoll) || (p.quiz != nil && !s.Can(protocol.CapPollQuiz)) {
			v.Unsupported = append(v.Unsupported, s.Hostname)
			continue
		}
		v.Eligible++
		who := Student{Hostname: s.Hostname, Username: s.Username, Simulated: simulated}
		if _, ok := p.votes[who]; !ok && p.named {
			v.Missing = append(v.Missing, who)
		}
	}
	slices.SortFunc(v.Missing, cmpStudent)
}

func cmpStudent(a, b Student) int {
	if c := strings.Compare(strings.ToLower(a.Hostname), strings.ToLower(b.Hostname)); c != 0 {
		return c
	}
	return strings.Compare(a.Username, b.Username)
}

// ForgetSimulated drops what the simulated students answered and received:
// it goes with them when the aula simulada closes (docs/privacidad.md rule 35).
func (c *Class) ForgetSimulated() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.polls {
		for _, m := range []map[Student]bool{p.delivered, p.shown} {
			for who := range m {
				if who.Simulated {
					delete(m, who)
				}
			}
		}
		for who := range p.votes {
			if who.Simulated {
				delete(p.votes, who)
			}
		}
	}
	for who := range c.students {
		if who.Simulated {
			delete(c.students, who)
		}
	}
}
