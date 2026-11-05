package participation

import (
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// Pace of each agent, as the console enforces it (docs/protocol.md §5.18).
const (
	MessageBurst = 5
	MessageEvery = 3 * time.Second
	HandBurst    = 3
	HandEvery    = time.Second
	// MaxMessagesPerStudent is how many messages the console keeps of one
	// student.
	MaxMessagesPerStudent = 20
)

// Student identifies a student the way the rest of the console does: machine
// and user, real or simulated. It survives a reconnection; the session ID
// does not.
type Student struct {
	Hostname  string `json:"hostname"`
	Username  string `json:"username"`
	Simulated bool   `json:"simulated"`
}

// Hand is a raised hand on the console.
type Hand struct {
	Student
	HandID   string    `json:"handId"`
	Note     string    `json:"note"`
	RaisedAt time.Time `json:"raisedAt"`
	Position int       `json:"position"` // 1 = first in line; set by Inbox
}

// MessageStatus is where a student's message stands for the teacher.
type MessageStatus string

// Message statuses.
const (
	MessageNew     MessageStatus = "new"     // not answered yet
	MessageReplied MessageStatus = "replied" // answered with «Responder»
	MessageDone    MessageStatus = "done"    // marked as seen without an answer
)

// Message is a student's message on the console.
type Message struct {
	Student
	MessageID string        `json:"messageId"`
	Text      string        `json:"text"`
	At        time.Time     `json:"at"`
	Status    MessageStatus `json:"status"`
	Reply     string        `json:"reply,omitempty"`
	RepliedAt time.Time     `json:"repliedAt,omitzero"`
}

// Inbox is what the «Participación» view shows.
type Inbox struct {
	Hands    []Hand    `json:"hands"`    // in order of arrival
	Messages []Message `json:"messages"` // newest first
	// Pending counts the raised hands and the new messages: the number next
	// to «Participación» in the menu.
	Pending int `json:"pending"`
}

// Outcome is what the console does after a hand.raise or a msg.student.
type Outcome int

// Outcomes.
const (
	// Added: new in the line or the inbox; tell the teacher.
	Added Outcome = iota
	// Repeated: already there (a resend after a reconnection); a hand keeps
	// its place, a message is acked again.
	Repeated
	// LoweredBefore: the teacher had lowered this hand_id; answer hand.lower
	// by teacher and do not queue it.
	LoweredBefore
	// TooFast: over the agent's pace; answer error invalid_message.
	TooFast
)

type pace struct{ hands, messages *bucket }

// Board is the console side: raised hands in order of arrival and the
// students' messages. It is safe for concurrent use.
type Board struct {
	now func() time.Time // replaced in tests

	mu       sync.Mutex
	hands    []Hand                // in order of arrival
	lowered  map[Student]string    // the last hand_id the teacher lowered
	messages map[Student][]Message // oldest first
	paces    map[Student]*pace
}

// NewBoard returns an empty board.
func NewBoard() *Board {
	return &Board{now: time.Now, lowered: map[Student]string{},
		messages: map[Student][]Message{}, paces: map[Student]*pace{}}
}

func (b *Board) pace(s Student, now time.Time) *pace {
	p, ok := b.paces[s]
	if !ok {
		p = &pace{hands: newBucket(HandBurst, HandEvery, now), messages: newBucket(MessageBurst, MessageEvery, now)}
		b.paces[s] = p
	}
	return p
}

// Raise handles a hand.raise.
func (b *Board) Raise(s Student, p protocol.HandRaise) Outcome {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	if !b.pace(s, now).hands.take(now) {
		return TooFast
	}
	if b.lowered[s] == p.HandID {
		return LoweredBefore
	}
	if i := b.handOf(s); i >= 0 {
		// One hand per student: it keeps its place and takes the new note.
		b.hands[i].HandID, b.hands[i].Note = p.HandID, p.Note
		return Repeated
	}
	b.hands = append(b.hands, Hand{Student: s, HandID: p.HandID, Note: p.Note, RaisedAt: now})
	return Added
}

// LowerByStudent handles a hand.lower from the agent. It reports whether the
// hand left the line (false if it was not the current hand or too fast).
func (b *Board) LowerByStudent(s Student, p protocol.HandLower) (lowered, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	if !b.pace(s, now).hands.take(now) {
		return false, false
	}
	i := b.handOf(s)
	if i < 0 || b.hands[i].HandID != p.HandID {
		return false, true // not the current hand: ignored without error
	}
	b.hands = slices.Delete(b.hands, i, i+1)
	return true, true
}

// LowerByTeacher takes a student's hand out of the line and returns its
// hand_id, to send in hand.lower; ok is false if the hand was not raised.
func (b *Board) LowerByTeacher(s Student) (handID string, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := b.handOf(s)
	if i < 0 {
		return "", false
	}
	handID = b.hands[i].HandID
	b.lowered[s] = handID
	b.hands = slices.Delete(b.hands, i, i+1)
	return handID, true
}

// LowerAll takes every hand out of the line and returns them, so the caller
// can send each hand.lower.
func (b *Board) LowerAll() []Hand {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.hands
	for _, h := range out {
		b.lowered[h.Student] = h.HandID
	}
	b.hands = nil
	return out
}

// Message handles a msg.student.
func (b *Board) Message(s Student, p protocol.MsgStudent) Outcome {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	list := b.messages[s]
	if slices.ContainsFunc(list, func(m Message) bool { return m.MessageID == p.MessageID }) {
		return Repeated // acked again, never counted twice; not paced either
	}
	if !b.pace(s, now).messages.take(now) {
		return TooFast
	}
	list = append(list, Message{Student: s, MessageID: p.MessageID, Text: p.Text, At: now, Status: MessageNew})
	if len(list) > MaxMessagesPerStudent {
		// Drop the oldest answered (or seen) message, else the oldest.
		i := slices.IndexFunc(list, func(m Message) bool { return m.Status != MessageNew })
		if i < 0 {
			i = 0
		}
		list = slices.Delete(list, i, i+1)
	}
	b.messages[s] = list
	return Added
}

// Replied marks a message as answered with text. It reports whether the
// message is on the board.
func (b *Board) Replied(s Student, messageID, text string) bool {
	return b.setStatus(s, messageID, func(m *Message) {
		m.Status, m.Reply, m.RepliedAt = MessageReplied, text, b.now()
	})
}

// Done marks a message as seen without an answer.
func (b *Board) Done(s Student, messageID string) bool {
	return b.setStatus(s, messageID, func(m *Message) {
		if m.Status == MessageNew {
			m.Status = MessageDone
		}
	})
}

func (b *Board) setStatus(s Student, messageID string, f func(*Message)) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	list := b.messages[s]
	for i := range list {
		if list[i].MessageID == messageID {
			f(&list[i])
			return true
		}
	}
	return false
}

// Lookup returns one message.
func (b *Board) Lookup(s Student, messageID string) (Message, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, m := range b.messages[s] {
		if m.MessageID == messageID {
			return m, true
		}
	}
	return Message{}, false
}

// HandOf returns a student's raised hand and its place in line.
func (b *Board) HandOf(s Student) (Hand, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := b.handOf(s)
	if i < 0 {
		return Hand{}, false
	}
	h := b.hands[i]
	h.Position = i + 1
	return h, true
}

func (b *Board) handOf(s Student) int {
	return slices.IndexFunc(b.hands, func(h Hand) bool { return h.Student == s })
}

// Inbox returns a copy of the board for the view.
func (b *Board) Inbox() Inbox {
	b.mu.Lock()
	defer b.mu.Unlock()
	in := Inbox{Hands: make([]Hand, len(b.hands)), Messages: []Message{}}
	for i, h := range b.hands {
		h.Position = i + 1
		in.Hands[i] = h
	}
	for _, list := range b.messages {
		for _, m := range list {
			in.Messages = append(in.Messages, m)
			if m.Status == MessageNew {
				in.Pending++
			}
		}
	}
	slices.SortStableFunc(in.Messages, func(x, y Message) int { return y.At.Compare(x.At) })
	in.Pending += len(in.Hands)
	return in
}

// Reset forgets everything: the class closed or the channel changed
// (docs/privacidad.md rule 51).
func (b *Board) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.hands = nil
	b.lowered = map[Student]string{}
	b.messages = map[Student][]Message{}
	b.paces = map[Student]*pace{}
}

// ForgetSimulated drops what simulated students sent: the aula simulada
// closed (docs/privacidad.md rule 35).
func (b *Board) ForgetSimulated() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.hands = slices.DeleteFunc(b.hands, func(h Hand) bool { return h.Simulated })
	maps.DeleteFunc(b.messages, func(s Student, _ []Message) bool { return s.Simulated })
	maps.DeleteFunc(b.lowered, func(s Student, _ string) bool { return s.Simulated })
	maps.DeleteFunc(b.paces, func(s Student, _ *pace) bool { return s.Simulated })
}
