package poll

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// fakeClock is a clock the test moves by hand.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *fakeClock { return &fakeClock{t: time.Unix(1_792_771_200, 0)} }

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) Add(d time.Duration) {
	f.mu.Lock()
	f.t = f.t.Add(d)
	f.mu.Unlock()
}

const (
	refA = "7d1f2e3a-4b5c-4d6e-a7f8-9a0b1c2d3e4f"
	refB = "0c9a7e52-6a4b-4f7e-8d33-2b1f4e6a9c10"
	pid1 = "e4b1c2d3-5f6a-4b7c-8d9e-0f1a2b3c4d5e"
)

func student(id, host string, caps ...protocol.Capability) session.Session {
	if caps == nil {
		caps = []protocol.Capability{protocol.CapPoll}
	}
	return session.Session{ID: id, Hostname: host, Username: "alumno", State: session.StateConnected, Capabilities: caps}
}

func who(host string) Student { return Student{Hostname: host, Username: "alumno"} }

func choice(n int) *int { return &n }

func voteFor(pollID string, n int) protocol.PollVote {
	return protocol.PollVote{PollID: pollID, Choice: choice(n)}
}

var question = Request{Kind: protocol.PollKindChoice, Question: "¿Qué imprime print(3 // 2)?",
	Options: []string{"1.5", "1", " ", "2"}}

func types(cmds []Command) string {
	var b strings.Builder
	for _, c := range cmds {
		b.WriteString(c.SessionID + ":" + string(c.Payload.MessageType()) + " ")
	}
	return strings.TrimSpace(b.String())
}

// --- Console side ------------------------------------------------------------

func TestOpenValidates(t *testing.T) {
	c := NewClass(nil)
	for _, req := range []Request{
		{Kind: protocol.PollKindChoice, Question: "¿?", Options: []string{"solo", "  "}},
		{Kind: protocol.PollKindYesNo, Question: "   "},
		{Kind: "rating", Question: "¿?"},
		{Kind: protocol.PollKindOpen, Question: strings.Repeat("ñ", protocol.MaxPollQuestion+1)},
	} {
		if _, err := c.Open(req); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: %v, want ErrInvalid", req, err)
		}
	}
	// Options of a yes/no poll are ignored, empty choice options dropped.
	if _, err := c.Open(Request{Kind: protocol.PollKindYesNo, Question: "¿Entendiste?", Options: []string{"x"}}); err != nil {
		t.Fatal(err)
	}
	id, err := c.Open(question)
	if err != nil {
		t.Fatal(err)
	}
	cmds := c.Plan([]session.Session{student("s1", "PC01")}, nil)
	open, ok := cmds[len(cmds)-1].Payload.(protocol.PollOpen)
	if !ok || open.PollID != id || len(open.Options) != 3 || open.Validate() != nil {
		t.Fatalf("plan = %s, %+v", types(cmds), open)
	}
}

func TestOneVotePerStudentLastCounts(t *testing.T) {
	clk := newClock()
	c := NewClass(clk.Now)
	id, _ := c.Open(question)
	c.Plan([]session.Session{student("s1", "PC01"), student("s2", "PC02"), student("s3", "PC03")}, nil)
	for _, v := range []struct {
		sess, host string
		n          int
	}{{"s1", "PC01", 0}, {"s2", "PC02", 1}, {"s3", "PC03", 1}} {
		if st, err := c.Vote(v.sess, who(v.host), voteFor(id, v.n)); err != nil || st != protocol.PollVoteAccepted {
			t.Fatalf("vote %+v: %s, %v", v, st, err)
		}
	}
	clk.Add(2 * time.Second)
	// PC01 changes its mind: still one vote.
	if st, _ := c.Vote("s1", who("PC01"), voteFor(id, 1)); st != protocol.PollVoteAccepted {
		t.Fatalf("change = %s", st)
	}
	v := c.Snapshot(nil, nil).Current
	if v == nil || v.Answered != 3 || v.Options[0].Count != 0 || v.Options[1].Count != 3 || v.Options[2].Count != 0 {
		t.Fatalf("view = %+v", v)
	}
	// The same student on another session (reconnected) is still one vote.
	clk.Add(2 * time.Second)
	c.Vote("s1-again", who("PC01"), voteFor(id, 2))
	v = c.Snapshot(nil, nil).Current
	if v.Answered != 3 || v.Options[1].Count != 2 || v.Options[2].Count != 1 {
		t.Fatalf("after reconnection = %+v", v.Options)
	}
}

func TestVoteChecks(t *testing.T) {
	clk := newClock()
	c := NewClass(clk.Now)
	if st, _ := c.Vote("s1", who("PC01"), voteFor(pid1, 0)); st != protocol.PollVoteUnknown {
		t.Fatalf("no poll: %s", st)
	}
	id, _ := c.Open(Request{Kind: protocol.PollKindYesNo, Question: "¿Sí?", Hosts: []string{"pc01"}})
	clk.Add(time.Second)
	if st, _ := c.Vote("s2", who("PC02"), voteFor(id, 0)); st != protocol.PollVoteUnknown {
		t.Fatalf("not a target: %s", st)
	}
	clk.Add(time.Second)
	if _, err := c.Vote("s1", who("PC01"), voteFor(id, 2)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("out of the yes/no options: %v", err)
	}
	clk.Add(time.Second)
	if _, err := c.Vote("s1", who("PC01"), protocol.PollVote{PollID: id, Text: "sí"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("text to a yes/no poll: %v", err)
	}
	clk.Add(time.Second)
	if st, err := c.Vote("s1", who("PC01"), voteFor(id, 0)); err != nil || st != protocol.PollVoteAccepted {
		t.Fatalf("valid: %s %v", st, err)
	}
	if _, err := c.Close(); err != nil {
		t.Fatal(err)
	}
	clk.Add(time.Second)
	if st, _ := c.Vote("s1", who("PC01"), voteFor(id, 1)); st != protocol.PollVoteClosed {
		t.Fatalf("after close: %s", st)
	}
	closed := c.Snapshot(nil, nil).Closed[0]
	if closed.Options[0].Count != 1 || closed.Options[1].Count != 0 || closed.ClosedAt == nil {
		t.Fatalf("a late vote changed the totals: %+v", closed)
	}
	if _, err := c.Close(); !errors.Is(err, ErrNotOpen) {
		t.Fatalf("close twice: %v", err)
	}
}

func TestVotePace(t *testing.T) {
	clk := newClock()
	c := NewClass(clk.Now)
	id, _ := c.Open(question)
	for i := range VoteBurst {
		if _, err := c.Vote("s1", who("PC01"), voteFor(id, i%3)); err != nil {
			t.Fatalf("burst %d: %v", i, err)
		}
	}
	if _, err := c.Vote("s1", who("PC01"), voteFor(id, 0)); !errors.Is(err, ErrTooFast) {
		t.Fatalf("past the burst: %v", err)
	}
	clk.Add(VoteEvery)
	if _, err := c.Vote("s1", who("PC01"), voteFor(id, 0)); err != nil {
		t.Fatalf("after a second: %v", err)
	}
}

func TestPlanOpenCloseResultsAndReconnection(t *testing.T) {
	clk := newClock()
	c := NewClass(clk.Now)
	if cmds := c.Plan([]session.Session{student("s1", "PC01")}, nil); len(cmds) != 0 {
		t.Fatalf("no poll: %s", types(cmds))
	}
	id, _ := c.Open(question)
	sessions := []session.Session{student("s1", "PC01"), student("s2", "PC02"), student("old", "PC09", protocol.CapTimer)}
	if got := types(c.Plan(sessions, nil)); got != "s1:poll.open s2:poll.open" {
		t.Fatalf("open = %q", got)
	}
	if got := types(c.Plan(sessions, nil)); got != "" {
		t.Fatalf("up to date = %q", got)
	}
	c.Ack("s1", protocol.PollAck{RefID: refA, PollID: id, Status: protocol.PollAckShown})
	c.Vote("s1", who("PC01"), voteFor(id, 1))
	c.Vote("s2", who("PC02"), voteFor(id, 0))

	// PC02 reconnects with the poll open: it gets it again.
	sessions[1] = student("s2b", "PC02")
	if got := types(c.Plan(sessions, nil)); got != "s2b:poll.open" {
		t.Fatalf("reconnection = %q", got)
	}
	v := c.Snapshot(sessions, nil).Current
	if v.Eligible != 2 || len(v.Unsupported) != 1 || v.Unsupported[0] != "PC09" || v.Received != 1 {
		t.Fatalf("open view = %+v", v)
	}

	c.Close()
	if got := types(c.Plan(sessions, nil)); got != "s1:poll.close s2b:poll.close" {
		t.Fatalf("close = %q", got)
	}
	c.Ack("s1", protocol.PollAck{RefID: refA, PollID: id, Status: protocol.PollAckClosed})
	if err := c.Share(id); err != nil {
		t.Fatal(err)
	}
	cmds := c.Plan(sessions, nil)
	if got := types(cmds); got != "s1:poll.results s2b:poll.results" {
		t.Fatalf("share = %q", got)
	}
	res := cmds[0].Payload.(protocol.PollResults)
	if res.Voters != 2 || res.Counts[0] != 1 || res.Counts[1] != 1 || res.Recipients != 2 || res.Validate() != nil {
		t.Fatalf("results = %+v", res)
	}
	c.Ack("s1", protocol.PollAck{RefID: refA, PollID: id, Status: protocol.PollAckResults})

	// PC01 reconnects after the close: nothing again (it acked both); PC02
	// did not ack: it gets the close and the results again.
	sessions[0], sessions[1] = student("s1b", "PC01"), student("s2c", "PC02")
	if got := types(c.Plan(sessions, nil)); got != "s2c:poll.close s2c:poll.results" {
		t.Fatalf("reconnection after close = %q", got)
	}
	// A newcomer that never had the poll gets nothing.
	if got := types(c.Plan(append(sessions, student("s3", "PC03")), nil)); got != "" {
		t.Fatalf("newcomer after close = %q", got)
	}
}

func TestNewPollClosesThePrevious(t *testing.T) {
	c := NewClass(nil)
	sessions := []session.Session{student("s1", "PC01")}
	first, _ := c.Open(question)
	c.Plan(sessions, nil)
	second, _ := c.Open(Request{Kind: protocol.PollKindOpen, Question: "¿Qué te costó más?"})
	cmds := c.Plan(sessions, nil)
	if got := types(cmds); got != "s1:poll.close s1:poll.open" {
		t.Fatalf("replace = %q", got)
	}
	if cmds[0].Payload.(protocol.PollClose).PollID != first || cmds[1].Payload.(protocol.PollOpen).PollID != second {
		t.Fatal("wrong polls")
	}
	view := c.Snapshot(nil, nil)
	if view.Current == nil || view.Current.PollID != second || len(view.Closed) != 1 || view.Closed[0].PollID != first {
		t.Fatalf("view = %+v", view)
	}
	if err := c.Share(second); !errors.Is(err, ErrStillOpen) {
		t.Fatalf("share open: %v", err)
	}
	c.Close()
	if err := c.Share(second); !errors.Is(err, ErrNotShareable) {
		t.Fatalf("share open answers: %v", err)
	}
	if err := c.Share(pid1); !errors.Is(err, ErrGone) {
		t.Fatalf("share unknown: %v", err)
	}
}

func TestSelectionAndFailed(t *testing.T) {
	c := NewClass(nil)
	c.Open(Request{Kind: protocol.PollKindYesNo, Question: "¿Sí?", Hosts: []string{"PC02"}})
	sessions := []session.Session{student("s1", "PC01"), student("s2", "pc02")}
	cmds := c.Plan(sessions, nil)
	if got := types(cmds); got != "s2:poll.open" {
		t.Fatalf("selection = %q", got)
	}
	c.Failed(cmds[0])
	if got := types(c.Plan(sessions, nil)); got != "s2:poll.open" {
		t.Fatalf("after a failed send = %q", got)
	}
}

func TestAnonymityAndSimulated(t *testing.T) {
	clk := newClock()
	c := NewClass(clk.Now)
	id, _ := c.Open(Request{Kind: protocol.PollKindOpen, Question: "¿Qué te costó más?"})
	real := []session.Session{student("s1", "PC01"), student("s2", "PC02")}
	sims := []session.Session{student("x1", "SIM-01")}
	c.Plan(real, sims)
	c.Vote("s1", who("PC01"), protocol.PollVote{PollID: id, Text: "  el for "})
	c.Vote("x1", Student{Hostname: "SIM-01", Username: "alumno", Simulated: true}, protocol.PollVote{PollID: id, Text: "nada"})
	v := c.Snapshot(real, sims).Current
	if v.Answered != 1 || v.AnsweredSimulated != 1 || len(v.Missing) != 0 {
		t.Fatalf("view = %+v", v)
	}
	for _, a := range v.Answers {
		if a.Hostname != "" || a.Username != "" || !a.At.IsZero() {
			t.Fatalf("an anonymous answer says who or when: %+v", a)
		}
	}
	if v.Answers[0].Text != "el for" || v.Answers[1].Text != "nada" || !v.Answers[1].Simulated {
		t.Fatalf("answers = %+v", v.Answers)
	}

	// A named choice poll lists who chose what and who is missing; the
	// shared totals leave the simulated students out.
	id, _ = c.Open(Request{Kind: protocol.PollKindChoice, Question: "¿?", Options: []string{"a", "b"}, Named: true})
	c.Plan(real, sims)
	clk.Add(time.Second)
	c.Vote("s1", who("PC01"), voteFor(id, 1))
	c.Vote("x1", Student{Hostname: "SIM-01", Username: "alumno", Simulated: true}, voteFor(id, 1))
	v = c.Snapshot(real, sims).Current
	if len(v.Answers) != 2 || v.Answers[0].Hostname != "PC01" || v.Answers[0].Choice != 1 {
		t.Fatalf("named answers = %+v", v.Answers)
	}
	if len(v.Missing) != 1 || v.Missing[0].Hostname != "PC02" {
		t.Fatalf("missing = %+v", v.Missing)
	}
	if v.Options[1].Count != 1 || v.Options[1].Simulated != 1 {
		t.Fatalf("options = %+v", v.Options)
	}
	c.Close()
	c.Share(id)
	for _, cmd := range c.Plan(real, sims) {
		if r, ok := cmd.Payload.(protocol.PollResults); ok && (r.Voters != 1 || r.Recipients != 2) {
			t.Fatalf("shared totals count simulated students: %+v", r)
		}
	}
	// An anonymous choice poll lists no answers at all.
	id, _ = c.Open(question)
	c.Plan(real, sims)
	clk.Add(time.Second)
	c.Vote("s1", who("PC01"), voteFor(id, 0))
	if v := c.Snapshot(real, sims).Current; len(v.Answers) != 0 || len(v.Missing) != 0 {
		t.Fatalf("an anonymous poll tells who: %+v", v)
	}
}

func TestKeepsAtMostMaxPolls(t *testing.T) {
	c := NewClass(nil)
	for range MaxPolls + 3 {
		if _, err := c.Open(question); err != nil {
			t.Fatal(err)
		}
	}
	v := c.Snapshot(nil, nil)
	if v.Current == nil || len(v.Closed) != MaxPolls-1 {
		t.Fatalf("kept %d closed", len(v.Closed))
	}
	c.Reset()
	if v := c.Snapshot(nil, nil); v.Current != nil || len(v.Closed) != 0 || c.IsOpen() {
		t.Fatal("reset kept polls")
	}
}

// --- Agent side --------------------------------------------------------------

var openMsg = protocol.PollOpen{PollID: pid1, Kind: protocol.PollKindChoice, Question: "¿?", Options: []string{"a", "b", "c"}}

func TestBallotVoteAndAck(t *testing.T) {
	clk := newClock()
	b := NewBallot(clk.Now)
	ack, resend, isNew := b.Open(refA, openMsg)
	if ack.Status != protocol.PollAckShown || ack.RefID != refA || resend != nil || !isNew {
		t.Fatalf("open = %+v %v %v", ack, resend, isNew)
	}
	if _, err := b.Vote(pid1, 3, ""); !errors.Is(err, ErrBadVote) {
		t.Fatalf("out of range: %v", err)
	}
	v, err := b.Vote(pid1, 2, "")
	if err != nil || *v.Choice != 2 {
		t.Fatalf("vote = %+v %v", v, err)
	}
	b.Sending(refB, v)
	if c := b.View().Cards[0]; c.Vote != VoteSending || c.Choice != 2 {
		t.Fatalf("sending = %+v", c)
	}
	if _, err := b.Vote(pid1, 1, ""); !errors.Is(err, ErrTooSoon) {
		t.Fatalf("too soon: %v", err)
	}
	if b.VoteAck(protocol.PollVoteAck{RefID: refA, PollID: pid1, Status: protocol.PollVoteAccepted}) {
		t.Fatal("an ack for another envelope applied")
	}
	if !b.VoteAck(protocol.PollVoteAck{RefID: refB, PollID: pid1, Status: protocol.PollVoteAccepted}) {
		t.Fatal("ack not applied")
	}
	if c := b.View().Cards[0]; c.Vote != VoteAccepted {
		t.Fatalf("accepted = %+v", c)
	}
	// Open again (a resend): same card, nothing new.
	if _, _, isNew := b.Open(refA, openMsg); isNew || len(b.View().Cards) != 1 {
		t.Fatal("a resend made a new card")
	}
}

func TestBallotResendAfterReconnection(t *testing.T) {
	b := NewBallot(nil)
	b.Open(refA, openMsg)
	v, _ := b.Vote(pid1, 0, "")
	b.Unsent(v)
	b.Reconnected()
	_, resend, _ := b.Open(refB, openMsg)
	if resend == nil || *resend.Choice != 0 {
		t.Fatalf("resend = %+v", resend)
	}
	b.Sending(refB, *resend)
	if !b.VoteAck(protocol.PollVoteAck{RefID: refB, PollID: pid1, Status: protocol.PollVoteClosed}) {
		t.Fatal("ack")
	}
	if c := b.View().Cards[0]; c.Vote != VoteLate || c.Open {
		t.Fatalf("late = %+v", c)
	}
}

func TestBallotOrphansAfterReconnection(t *testing.T) {
	b := NewBallot(nil)
	other := protocol.PollOpen{PollID: refA, Kind: protocol.PollKindYesNo, Question: "¿Sí?"}
	b.Open(refA, openMsg)
	b.Open(refB, other)
	v, _ := b.Vote(refA, 0, "")
	b.Unsent(v)
	b.Reconnected()
	b.Open(refB, openMsg) // the console sends only the first one again
	if !b.Orphans() {
		t.Fatal("nothing closed")
	}
	for _, c := range b.View().Cards {
		switch c.PollID {
		case pid1:
			if !c.Open {
				t.Error("the poll sent again closed")
			}
		case refA:
			if c.Open || c.Vote != VoteLate {
				t.Errorf("the orphan = %+v", c)
			}
		}
	}
	if b.Orphans() {
		t.Error("orphans twice")
	}
}

func TestBallotOpenAnswerCloseAndResults(t *testing.T) {
	clk := newClock()
	b := NewBallot(clk.Now)
	b.Open(refA, protocol.PollOpen{PollID: pid1, Kind: protocol.PollKindOpen, Question: "¿?"})
	if _, err := b.Vote(pid1, 0, "   "); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := b.Vote(pid1, 0, strings.Repeat("ñ", protocol.MaxPollText+1)); !errors.Is(err, ErrTooLong) {
		t.Fatalf("too long: %v", err)
	}
	v, err := b.Vote(pid1, 0, " el for ")
	if err != nil || v.Text != "el for" || v.Choice != nil {
		t.Fatalf("text vote = %+v %v", v, err)
	}
	b.Unsent(v)
	ack, ok := b.Close(refB, protocol.PollClose{PollID: pid1})
	if !ok || ack.Status != protocol.PollAckClosed {
		t.Fatalf("close = %+v", ack)
	}
	if c := b.View().Cards[0]; c.Open || c.Vote != VoteLate {
		t.Fatalf("closed with an unsent answer = %+v", c)
	}
	clk.Add(time.Second)
	if _, err := b.Vote(pid1, 0, "otra"); !errors.Is(err, ErrClosed) {
		t.Fatalf("vote after close: %v", err)
	}
	if _, ok := b.Results(refB, protocol.PollResults{PollID: pid1, Counts: []int{1, 0}, Voters: 1, Recipients: 1}); ok {
		t.Fatal("results applied to an open-answer poll")
	}
	if _, ok := b.Close(refB, protocol.PollClose{PollID: refA}); ok {
		t.Fatal("closed an unknown poll")
	}

	b.Open(refA, protocol.PollOpen{PollID: refA, Kind: protocol.PollKindYesNo, Question: "¿Sí?"})
	if _, ok := b.Results(refB, protocol.PollResults{PollID: refA, Counts: []int{1, 0, 0}, Voters: 1, Recipients: 1}); ok {
		t.Fatal("results with the wrong number of options")
	}
	ack, ok = b.Results(refB, protocol.PollResults{PollID: refA, Counts: []int{3, 1}, Voters: 4, Recipients: 5})
	c := b.View().Cards[0]
	if !ok || ack.Status != protocol.PollAckResults || c.Results == nil || c.Results.Voters != 4 || c.Open {
		t.Fatalf("results = %+v %+v", ack, c)
	}
}

func TestBallotRefusedAndKeepsMaxCards(t *testing.T) {
	clk := newClock()
	b := NewBallot(clk.Now)
	b.Open(refA, openMsg)
	v, _ := b.Vote(pid1, 1, "")
	b.Sending(refB, v)
	if !b.Refused(protocol.ErrorPayload{RefID: refB, Code: protocol.CodeUnsupportedCapability}) {
		t.Fatal("not refused")
	}
	if c := b.View().Cards[0]; c.Vote != VoteUnsupported {
		t.Fatalf("refused = %+v", c)
	}
	clk.Add(time.Second)
	if _, err := b.Vote(pid1, 0, ""); !errors.Is(err, ErrNoAnswer) {
		t.Fatalf("an older console: %v", err)
	}
	for range MaxCards + 2 {
		b.Open(refA, protocol.PollOpen{PollID: protocol.NewID(), Kind: protocol.PollKindYesNo, Question: "¿?"})
	}
	if n := len(b.View().Cards); n != MaxCards {
		t.Fatalf("kept %d cards", n)
	}
	b.Reset()
	if len(b.View().Cards) != 0 {
		t.Fatal("reset kept cards")
	}
}

func TestForgetSimulated(t *testing.T) {
	c := NewClass(nil)
	id, _ := c.Open(question)
	sims := []session.Session{student("x1", "SIM-01")}
	c.Plan([]session.Session{student("s1", "PC01")}, sims)
	c.Vote("x1", Student{Hostname: "SIM-01", Username: "alumno", Simulated: true}, voteFor(id, 0))
	c.Vote("s1", who("PC01"), voteFor(id, 1))
	c.ForgetSimulated()
	v := c.Snapshot(nil, nil).Current
	if v.AnsweredSimulated != 0 || v.Answered != 1 || v.Options[0].Simulated != 0 {
		t.Fatalf("simulated answers kept: %+v", v)
	}
}
