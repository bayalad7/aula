package participation

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/protocol"
)

// Limits of the agent side (docs/protocol.md §5.18).
const (
	// MaxAwaitingAck is how many messages may wait for their ack; the window
	// does not let the student write another.
	MaxAwaitingAck = 5
	// AgentMessageEvery and AgentHandEvery are the agent's own pace.
	AgentMessageEvery = 3 * time.Second
	AgentHandEvery    = time.Second
	// keptMessages is how many of their own messages the window shows.
	keptMessages = 20
)

// Errors of the agent side. The window words them.
var (
	ErrEmpty       = errors.New("participation: the message is empty")
	ErrTooLong     = errors.New("participation: the text is too long")
	ErrTooSoon     = errors.New("participation: too soon after the last one")
	ErrWaiting     = errors.New("participation: too many messages wait for their ack")
	ErrUnsupported = errors.New("participation: the console does not take hands or messages")
)

// SentStatus is where one of the student's own messages stands.
type SentStatus string

// Statuses of a sent message.
const (
	SentSending  SentStatus = "sending"  // waits for msg.student.ack
	SentReceived SentStatus = "received" // reached the console (not "read")
	SentFailed   SentStatus = "failed"   // the console refused it
)

// Sent is one of the student's own messages, as the window shows it.
type Sent struct {
	MessageID string     `json:"messageId"`
	Text      string     `json:"text"`
	At        time.Time  `json:"at"`
	Status    SentStatus `json:"status"`
}

// HandState is the student's own hand, as the window shows it.
type HandState string

// Hand states. Go sends states; the window words them.
const (
	HandDown             HandState = "down"
	HandUp               HandState = "up"
	HandLoweredByTeacher HandState = "lowered_by_teacher" // down, and the teacher did it
)

// DeskView is the agent side's state for the window.
type DeskView struct {
	Hand     HandState `json:"hand"`
	Note     string    `json:"note,omitempty"`
	Messages []Sent    `json:"messages"` // oldest first
	// Unsupported: the console answered unknown_type (an older console).
	Unsupported bool `json:"unsupported"`
}

// Desk is the agent side: the student's hand and the messages that wait for
// their ack. It only builds payloads: the caller sends them and reports the
// envelope ID with Sending. It is safe for concurrent use.
type Desk struct {
	now func() time.Time // replaced in tests

	mu          sync.Mutex
	handID      string // "" while the hand is down
	note        string
	byTeacher   bool // the last hand was lowered by the teacher
	lastHand    time.Time
	lastMsg     time.Time
	sent        []Sent
	envelopes   map[string]string // envelope ID → message_id, or hand_id for a hand.raise
	unsupported bool
}

// NewDesk returns a desk with the hand down.
func NewDesk() *Desk { return &Desk{now: time.Now, envelopes: map[string]string{}} }

// Raise raises the hand, or updates its note if it is already up (it keeps
// its place in line).
func (d *Desk) Raise(note string) (protocol.HandRaise, error) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > protocol.MaxHandNote {
		return protocol.HandRaise{}, ErrTooLong
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.unsupported {
		return protocol.HandRaise{}, ErrUnsupported
	}
	now := d.now()
	if now.Sub(d.lastHand) < AgentHandEvery {
		return protocol.HandRaise{}, ErrTooSoon
	}
	d.lastHand = now
	if d.handID == "" {
		d.handID = protocol.NewID()
	}
	d.note, d.byTeacher = note, false
	return protocol.HandRaise{HandID: d.handID, Note: note}, nil
}

// Lower lowers the hand; ok is false if it was down.
func (d *Desk) Lower() (p protocol.HandLower, ok bool, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.handID == "" {
		return protocol.HandLower{}, false, nil
	}
	now := d.now()
	if now.Sub(d.lastHand) < AgentHandEvery {
		return protocol.HandLower{}, false, ErrTooSoon
	}
	d.lastHand = now
	p = protocol.HandLower{HandID: d.handID, By: protocol.HandByStudent}
	d.handID, d.note, d.byTeacher = "", "", false
	return p, true, nil
}

// LoweredByTeacher handles a hand.lower from the console. It reports whether
// it lowered the current hand.
func (d *Desk) LoweredByTeacher(p protocol.HandLower) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.handID == "" || p.HandID != d.handID {
		return false
	}
	d.handID, d.note, d.byTeacher = "", "", true
	return true
}

// Write builds a msg.student with text.
func (d *Desk) Write(text string) (protocol.MsgStudent, error) {
	text = strings.TrimSpace(text)
	switch n := utf8.RuneCountInString(text); {
	case n == 0:
		return protocol.MsgStudent{}, ErrEmpty
	case n > protocol.MaxStudentMsgText:
		return protocol.MsgStudent{}, ErrTooLong
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.unsupported {
		return protocol.MsgStudent{}, ErrUnsupported
	}
	now := d.now()
	if now.Sub(d.lastMsg) < AgentMessageEvery {
		return protocol.MsgStudent{}, ErrTooSoon
	}
	waiting := 0
	for _, s := range d.sent {
		if s.Status == SentSending {
			waiting++
		}
	}
	if waiting >= MaxAwaitingAck {
		return protocol.MsgStudent{}, ErrWaiting
	}
	d.lastMsg = now
	p := protocol.MsgStudent{MessageID: protocol.NewID(), Text: text}
	d.sent = append(d.sent, Sent{MessageID: p.MessageID, Text: text, At: now, Status: SentSending})
	if len(d.sent) > keptMessages {
		d.sent = slices.Delete(d.sent, 0, len(d.sent)-keptMessages)
	}
	return p, nil
}

// Sending records the envelope ID a hand.raise or msg.student went out with,
// so an error that answers it (ref_id) can be matched.
func (d *Desk) Sending(envelopeID string, p protocol.Payload) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch p := p.(type) {
	case protocol.HandRaise:
		d.envelopes[envelopeID] = p.HandID
	case protocol.MsgStudent:
		d.envelopes[envelopeID] = p.MessageID
	}
	if len(d.envelopes) > 4*keptMessages {
		clear(d.envelopes) // old ones: their errors would have come long ago
		d.envelopes[envelopeID] = ""
	}
}

// Acked handles a msg.student.ack. It reports whether it changed a message.
func (d *Desk) Acked(p protocol.MsgStudentAck) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.sent {
		if d.sent[i].MessageID == p.MessageID && d.sent[i].Status != SentReceived {
			d.sent[i].Status = SentReceived
			return true
		}
	}
	return false
}

// Refused handles an error from the console. It reports whether the error
// answered a hand.raise or a msg.student of this desk. unknown_type means an
// older console: the hand comes down and nothing more can be sent until the
// next connection.
func (d *Desk) Refused(p protocol.ErrorPayload) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	id, ok := d.envelopes[p.RefID]
	if !ok {
		return false
	}
	delete(d.envelopes, p.RefID)
	if p.Code == protocol.CodeUnknownType || p.Code == protocol.CodeUnsupportedCapability {
		d.unsupported = true
		d.handID, d.note = "", ""
		for i := range d.sent {
			if d.sent[i].Status == SentSending {
				d.sent[i].Status = SentFailed
			}
		}
		return true
	}
	if id == d.handID {
		d.handID, d.note = "", ""
		return true
	}
	for i := range d.sent {
		if d.sent[i].MessageID == id && d.sent[i].Status == SentSending {
			d.sent[i].Status = SentFailed
		}
	}
	return true
}

// Reconnected is called on each new connection: it returns what must be sent
// again (the hand, with its hand_id, and the messages without an ack, with
// their message_id) and gives an older console's refusal another chance.
func (d *Desk) Reconnected() []protocol.Payload {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.unsupported = false
	clear(d.envelopes)
	var out []protocol.Payload
	if d.handID != "" {
		out = append(out, protocol.HandRaise{HandID: d.handID, Note: d.note})
	}
	for _, s := range d.sent {
		if s.Status == SentSending {
			out = append(out, protocol.MsgStudent{MessageID: s.MessageID, Text: s.Text})
		}
	}
	return out
}

// Own returns one of the student's own messages, to quote it under the
// teacher's reply.
func (d *Desk) Own(messageID string) (Sent, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range d.sent {
		if s.MessageID == messageID {
			return s, true
		}
	}
	return Sent{}, false
}

// View returns the state for the window.
func (d *Desk) View() DeskView {
	d.mu.Lock()
	defer d.mu.Unlock()
	v := DeskView{Hand: HandDown, Note: d.note, Messages: slices.Clone(d.sent), Unsupported: d.unsupported}
	switch {
	case d.handID != "":
		v.Hand = HandUp
	case d.byTeacher:
		v.Hand = HandLoweredByTeacher
	}
	if v.Messages == nil {
		v.Messages = []Sent{}
	}
	return v
}
