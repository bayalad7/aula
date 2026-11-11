package poll

import (
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/protocol"
)

// MaxCards is how many poll cards the window keeps; past it, the oldest is
// dropped.
const MaxCards = 5

// VoteGap is the least time between two answers of the student.
const VoteGap = time.Second

// ReconnectGrace is how long the agent waits, after a reconnection, for the
// console to send an open poll again; past it the card closes (the poll was
// closed meanwhile, or the class is another one).
const ReconnectGrace = 3 * time.Second

// Errors of the agent side. The window words them (textos.es-MX.js).
var (
	ErrUnknown  = errors.New("unknown poll")
	ErrClosed   = errors.New("the poll is closed")
	ErrEmpty    = errors.New("empty answer")
	ErrTooLong  = errors.New("answer too long")
	ErrBadVote  = errors.New("the answer does not fit the poll")
	ErrTooSoon  = errors.New("answered too soon")
	ErrNoAnswer = errors.New("this console does not take answers")
)

// VoteState is what happened to the student's answer: states, never
// sentences (the window words them).
type VoteState string

// Answer states.
const (
	VoteNone        VoteState = "none"        // not answered yet
	VoteSending     VoteState = "sending"     // waiting for its ack (sent again on reconnection)
	VoteAccepted    VoteState = "accepted"    // it counts
	VoteLate        VoteState = "late"        // it arrived after the poll closed: it does not count
	VoteUnknown     VoteState = "unknown"     // the console did not know the poll
	VoteUnsupported VoteState = "unsupported" // an older console: it does not take answers
	VoteRefused     VoteState = "refused"     // the console refused it (too fast or invalid)
)

// Results are the shared totals of a closed poll.
type Results struct {
	Counts     []int `json:"counts"`
	Voters     int   `json:"voters"`
	Recipients int   `json:"recipients"`
}

// Card is one poll in the student's window.
type Card struct {
	PollID   string            `json:"pollId"`
	Kind     protocol.PollKind `json:"kind"`
	Question string            `json:"question"`
	Options  []string          `json:"options"` // empty in yes/no: the window writes «Sí» and «No»
	Named    bool              `json:"named"`
	Open     bool              `json:"open"`
	// Choice and Text are the student's last answer sent (Choice -1: none or
	// a text answer).
	Choice  int       `json:"choice"`
	Text    string    `json:"text"`
	Vote    VoteState `json:"vote"`
	Results *Results  `json:"results"`
	At      time.Time `json:"at"`
}

// BallotView is the polls for the window, newest first.
type BallotView struct {
	Cards []Card `json:"cards"`
	// Arrived is set only in the event that announces a new poll, so the
	// window marks it «Nuevo» once.
	Arrived string `json:"arrived"`
}

type card struct {
	Card
	pending   *protocol.PollVote // the answer waiting for its ack
	pendingID string             // its envelope ID; "" until sent on this connection
	orphan    bool               // open, and not sent again since the reconnection
}

// Ballot is the agent side of the polls. It is safe for concurrent use.
type Ballot struct {
	mu       sync.Mutex
	now      func() time.Time
	cards    []*card // newest first
	lastVote time.Time
	hasVoted bool
}

// NewBallot returns an empty ballot. now is time.Now when nil.
func NewBallot(now func() time.Time) *Ballot {
	if now == nil {
		now = time.Now
	}
	return &Ballot{now: now}
}

// Open applies a poll.open (refID is its envelope ID). It returns the ack,
// the answer to send again if one waits for its ack (a resend after a
// reconnection), and whether the card is new (notify, never raise).
func (b *Ballot) Open(refID string, p protocol.PollOpen) (protocol.PollAck, *protocol.PollVote, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ack := protocol.PollAck{RefID: refID, PollID: p.PollID, Status: protocol.PollAckShown}
	if c := b.find(p.PollID); c != nil {
		c.Open, c.orphan = true, false
		if c.Vote == VoteLate {
			c.Vote = VoteNone
		}
		return ack, c.pending, false
	}
	opts := p.Options
	if opts == nil {
		opts = []string{}
	}
	c := &card{Card: Card{PollID: p.PollID, Kind: p.Kind, Question: p.Question, Options: opts,
		Named: p.Named, Open: true, Choice: -1, Vote: VoteNone, At: b.now()}}
	b.cards = append([]*card{c}, b.cards...)
	if len(b.cards) > MaxCards {
		b.cards = b.cards[:MaxCards]
	}
	return ack, nil, true
}

// Vote checks the student's answer (choice for choice and yes/no polls, text
// for open ones) and returns the poll.vote to send; call Sending with its
// envelope ID, or Unsent when it could not be sent.
func (b *Ballot) Vote(pollID string, choice int, text string) (protocol.PollVote, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.find(pollID)
	if c == nil {
		return protocol.PollVote{}, ErrUnknown
	}
	if !c.Open {
		return protocol.PollVote{}, ErrClosed
	}
	if c.Vote == VoteUnsupported {
		return protocol.PollVote{}, ErrNoAnswer
	}
	v := protocol.PollVote{PollID: pollID}
	if c.Kind == protocol.PollKindOpen {
		text = strings.TrimSpace(text)
		switch {
		case text == "":
			return v, ErrEmpty
		case utf8.RuneCountInString(text) > protocol.MaxPollText:
			return v, ErrTooLong
		}
		v.Text = text
	} else {
		v.Choice = &choice
	}
	if v.Validate() != nil || v.ValidFor(c.Kind, c.Options) != nil {
		return protocol.PollVote{}, ErrBadVote
	}
	now := b.now()
	if b.hasVoted && now.Sub(b.lastVote) < VoteGap {
		return protocol.PollVote{}, ErrTooSoon
	}
	b.lastVote, b.hasVoted = now, true
	return v, nil
}

// Sending records that v went out with envelope ID id.
func (b *Ballot) Sending(id string, v protocol.PollVote) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if c := b.find(v.PollID); c != nil {
		b.setPending(c, v)
		c.pendingID = id
	}
}

// Unsent records that v could not be sent: it waits for the next
// connection, where the console sends the poll again.
func (b *Ballot) Unsent(v protocol.PollVote) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if c := b.find(v.PollID); c != nil {
		b.setPending(c, v)
		c.pendingID = ""
	}
}

func (b *Ballot) setPending(c *card, v protocol.PollVote) {
	v2 := v
	c.pending = &v2
	c.Vote = VoteSending
	c.Choice, c.Text = -1, v.Text
	if v.Choice != nil {
		c.Choice, c.Text = *v.Choice, ""
	}
}

// Reconnected forgets the envelope IDs of the answers waiting for an ack:
// they are sent again when the console sends their poll again. Call Orphans
// ReconnectGrace later.
func (b *Ballot) Reconnected() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, c := range b.cards {
		c.pendingID = ""
		c.orphan = c.Open
	}
}

// Orphans closes the open cards the console did not send again after the
// reconnection; an answer still waiting cannot count. It reports whether a
// card changed.
func (b *Ballot) Orphans() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	changed := false
	for _, c := range b.cards {
		if !c.orphan {
			continue
		}
		c.orphan, c.Open, changed = false, false, true
		if c.pending != nil {
			c.pending, c.pendingID, c.Vote = nil, "", VoteLate
		}
	}
	return changed
}

// VoteAck applies a poll.vote.ack. It reports whether the view changed.
func (b *Ballot) VoteAck(a protocol.PollVoteAck) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.find(a.PollID)
	if c == nil || c.pending == nil || c.pendingID != a.RefID {
		return false
	}
	c.pending, c.pendingID = nil, ""
	switch a.Status {
	case protocol.PollVoteAccepted:
		c.Vote = VoteAccepted
	case protocol.PollVoteClosed:
		c.Vote, c.Open = VoteLate, false
	default:
		c.Vote = VoteUnknown
	}
	return true
}

// Refused applies an error the console answered to an answer. It reports
// whether it was about one.
func (b *Ballot) Refused(e protocol.ErrorPayload) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, c := range b.cards {
		if c.pending == nil || c.pendingID == "" || c.pendingID != e.RefID {
			continue
		}
		c.pending, c.pendingID = nil, ""
		switch e.Code {
		case protocol.CodeUnknownType, protocol.CodeUnsupportedCapability:
			c.Vote = VoteUnsupported
		default:
			c.Vote = VoteRefused
		}
		return true
	}
	return false
}

// Close applies a poll.close. It is ignored (false) for an unknown poll.
func (b *Ballot) Close(refID string, p protocol.PollClose) (protocol.PollAck, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.find(p.PollID)
	if c == nil {
		return protocol.PollAck{}, false
	}
	c.Open = false
	if c.pending != nil && c.pendingID == "" {
		// It never reached the console and now cannot count.
		c.pending, c.Vote = nil, VoteLate
	}
	return protocol.PollAck{RefID: refID, PollID: p.PollID, Status: protocol.PollAckClosed}, true
}

// Results applies a poll.results. It is ignored (false) for an unknown poll
// or totals that do not match its options.
func (b *Ballot) Results(refID string, p protocol.PollResults) (protocol.PollAck, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.find(p.PollID)
	if c == nil || c.Kind == protocol.PollKindOpen || len(p.Counts) != c.Kind.Options(c.Options) {
		return protocol.PollAck{}, false
	}
	c.Open = false
	c.Results = &Results{Counts: append([]int(nil), p.Counts...), Voters: p.Voters, Recipients: p.Recipients}
	return protocol.PollAck{RefID: refID, PollID: p.PollID, Status: protocol.PollAckResults}, true
}

// Reset forgets every card (the agent left the class for good).
func (b *Ballot) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cards = nil
}

// View returns the cards for the window. The app sets Arrived in the event
// that announces a new poll.
func (b *Ballot) View() BallotView {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := BallotView{Cards: make([]Card, 0, len(b.cards))}
	for _, c := range b.cards {
		cc := c.Card
		cc.Options = append([]string{}, c.Options...)
		out.Cards = append(out.Cards, cc)
	}
	return out
}

// find returns the card of pollID; b.mu is held.
func (b *Ballot) find(pollID string) *card {
	for _, c := range b.cards {
		if c.PollID == pollID {
			return c
		}
	}
	return nil
}
