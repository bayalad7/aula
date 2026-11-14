package participation

import (
	"cmp"
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
	// MaxEntriesPerConversation is how many entries the console keeps of one
	// conversation: the student's messages and the teacher's private
	// messages to them (prompt 10.26).
	MaxEntriesPerConversation = 50
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

// Author is who wrote an entry of a conversation.
type Author string

// Authors.
const (
	FromStudent Author = "student"
	FromTeacher Author = "teacher" // a private msg.show to this student only
)

// Entry is one line of a conversation: a student's message or a private
// message of the teacher to that student (an answer carries ReplyTo).
type Entry struct {
	From      Author        `json:"from"`
	MessageID string        `json:"messageId,omitempty"` // the student's message_id
	ReplyTo   string        `json:"replyTo,omitempty"`   // the teacher's answer to that message_id
	Text      string        `json:"text"`
	At        time.Time     `json:"at"`
	Status    MessageStatus `json:"status,omitempty"` // a student's message only
}

// Conversation is everything one student and the teacher said in this
// class, oldest first.
type Conversation struct {
	Student
	Entries []Entry `json:"entries"`
	// Unread counts the student's messages not answered nor marked: the
	// conversation is «Por atender» while it is not zero.
	Unread int       `json:"unread"`
	LastAt time.Time `json:"lastAt"`
}

// Inbox is what the «Participación» view shows.
type Inbox struct {
	Hands    []Hand    `json:"hands"`    // in order of arrival
	Messages []Message `json:"messages"` // newest first
	// Conversations has one per student, the most recent first.
	Conversations []Conversation `json:"conversations"`
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

	mu      sync.Mutex
	hands   []Hand              // in order of arrival
	lowered map[Student]string  // the last hand_id the teacher lowered
	threads map[Student][]Entry // oldest first
	paces   map[Student]*pace
}

// NewBoard returns an empty board.
func NewBoard() *Board {
	return &Board{now: time.Now, lowered: map[Student]string{},
		threads: map[Student][]Entry{}, paces: map[Student]*pace{}}
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
	if b.find(s, p.MessageID) >= 0 {
		return Repeated // acked again, never counted twice; not paced either
	}
	if !b.pace(s, now).messages.take(now) {
		return TooFast
	}
	b.add(s, Entry{From: FromStudent, MessageID: p.MessageID, Text: p.Text, At: now, Status: MessageNew})
	return Added
}

// add appends an entry and keeps the conversation under
// MaxEntriesPerConversation: the oldest entry that is not waiting for the
// teacher goes first, else the oldest.
func (b *Board) add(s Student, e Entry) {
	list := append(b.threads[s], e)
	if len(list) > MaxEntriesPerConversation {
		i := slices.IndexFunc(list, func(e Entry) bool { return e.Status != MessageNew })
		if i < 0 {
			i = 0
		}
		list = slices.Delete(list, i, i+1)
	}
	b.threads[s] = list
}

// find returns the index of a student's message, or -1.
func (b *Board) find(s Student, messageID string) int {
	return slices.IndexFunc(b.threads[s], func(e Entry) bool {
		return e.From == FromStudent && e.MessageID == messageID
	})
}

// Replied records the teacher's answer to a message: it goes into the
// conversation and that message, and every earlier one still waiting, is
// answered. It reports whether the message is on the board.
func (b *Board) Replied(s Student, messageID, text string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := b.find(s, messageID)
	if i < 0 {
		return false
	}
	list := b.threads[s]
	for j := range list[:i+1] {
		if list[j].Status == MessageNew {
			list[j].Status = MessageReplied
		}
	}
	list[i].Status = MessageReplied
	b.add(s, Entry{From: FromTeacher, ReplyTo: messageID, Text: text, At: b.now()})
	return true
}

// Said records a private message of the teacher that answers no message: a
// reply to a raised hand or «Mensaje privado» (prompt 10.7).
func (b *Board) Said(s Student, text string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.add(s, Entry{From: FromTeacher, Text: text, At: b.now()})
}

// Done marks a message as seen without an answer.
func (b *Board) Done(s Student, messageID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := b.find(s, messageID)
	if i < 0 {
		return false
	}
	if e := &b.threads[s][i]; e.Status == MessageNew {
		e.Status = MessageDone
	}
	return true
}

// DoneAll marks every waiting message of a student as seen («Marcar como
// atendida»). ok is false if there is no conversation with that student.
func (b *Board) DoneAll(s Student) (marked int, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	list, ok := b.threads[s]
	for i := range list {
		if list[i].Status == MessageNew {
			list[i].Status = MessageDone
			marked++
		}
	}
	return marked, ok
}

// message builds the Message of a student's entry, with the teacher's last
// answer to it.
func message(s Student, list []Entry, i int) Message {
	e := list[i]
	m := Message{Student: s, MessageID: e.MessageID, Text: e.Text, At: e.At, Status: e.Status}
	for _, r := range list[i+1:] {
		if r.From == FromTeacher && r.ReplyTo == e.MessageID {
			m.Reply, m.RepliedAt = r.Text, r.At
		}
	}
	return m
}

// Lookup returns one message.
func (b *Board) Lookup(s Student, messageID string) (Message, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if i := b.find(s, messageID); i >= 0 {
		return message(s, b.threads[s], i), true
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
	in := Inbox{Hands: make([]Hand, len(b.hands)), Messages: []Message{}, Conversations: []Conversation{}}
	for i, h := range b.hands {
		h.Position = i + 1
		in.Hands[i] = h
	}
	for s, list := range b.threads {
		if len(list) == 0 {
			continue
		}
		c := Conversation{Student: s, Entries: slices.Clone(list), LastAt: list[len(list)-1].At}
		for i, e := range list {
			if e.From != FromStudent {
				continue
			}
			in.Messages = append(in.Messages, message(s, list, i))
			if e.Status == MessageNew {
				c.Unread++
			}
		}
		in.Pending += c.Unread
		in.Conversations = append(in.Conversations, c)
	}
	slices.SortStableFunc(in.Messages, func(x, y Message) int { return y.At.Compare(x.At) })
	// The most recent first; the same order on every call (a map has none).
	slices.SortFunc(in.Conversations, func(x, y Conversation) int {
		if c := y.LastAt.Compare(x.LastAt); c != 0 {
			return c
		}
		return cmp.Compare(x.Hostname+"|"+x.Username, y.Hostname+"|"+y.Username)
	})
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
	b.threads = map[Student][]Entry{}
	b.paces = map[Student]*pace{}
}

// ForgetSimulated drops what simulated students sent: the aula simulada
// closed (docs/privacidad.md rule 35).
func (b *Board) ForgetSimulated() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.hands = slices.DeleteFunc(b.hands, func(h Hand) bool { return h.Simulated })
	maps.DeleteFunc(b.threads, func(s Student, _ []Entry) bool { return s.Simulated })
	maps.DeleteFunc(b.lowered, func(s Student, _ string) bool { return s.Simulated })
	maps.DeleteFunc(b.paces, func(s Student, _ *pace) bool { return s.Simulated })
}
