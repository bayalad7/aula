package codedist

import (
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// Notice is a msg.show to display to the student.
type Notice struct {
	ID    string            `json:"id"` // envelope ID of the msg.show
	Text  string            `json:"text"`
	Level protocol.MsgLevel `json:"level"`
	At    time.Time         `json:"at"`
}

// Receiver applies the console's messages on the student side.
type Receiver struct {
	Board *Board
	// OnBoard is called with the new board after code.share or code.clear.
	OnBoard func([]Snippet)
	// OnNotice is called for each msg.show.
	OnNotice func(Notice)

	now func() time.Time // replaced in tests
}

// Handle processes one message from an authenticated console and returns the
// reply to send back, or nil when none is due. Acks mean "shown on screen",
// not "read by the student" (docs/protocol.md §5.2, §5.5).
func (r *Receiver) Handle(env protocol.Envelope) protocol.Payload {
	now := time.Now
	if r.now != nil {
		now = r.now
	}
	switch p := env.Payload.(type) {
	case protocol.CodeShare:
		board := r.Board.Apply(p, now())
		if r.OnBoard != nil {
			r.OnBoard(board)
		}
		return protocol.CodeAck{RefID: env.ID, SnippetID: p.SnippetID}
	case protocol.CodeClear:
		r.Board.Clear()
		if r.OnBoard != nil {
			r.OnBoard(nil)
		}
		return protocol.CodeAck{RefID: env.ID}
	case protocol.MsgShow:
		if r.OnNotice != nil {
			r.OnNotice(Notice{ID: env.ID, Text: p.Text, Level: p.Level, At: now()})
		}
		return protocol.MsgAck{RefID: env.ID}
	case protocol.ErrorPayload, protocol.Heartbeat:
		return nil
	default:
		// A valid message that a console must not send to an agent.
		return protocol.NewError(env.ID, protocol.CodeInvalidMessage, "unexpected message for an agent")
	}
}
