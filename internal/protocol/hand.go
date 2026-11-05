package protocol

import "unicode/utf8"

// Limits of the hand.* and msg.student* messages (docs/protocol.md §5.18, §7).
const (
	MaxHandNote       = 140
	MaxStudentMsgText = 500
)

// HandBy says who lowered a hand.
type HandBy string

// Who lowers a hand: the student (A → P) or the teacher (P → A).
const (
	HandByStudent HandBy = "student"
	HandByTeacher HandBy = "teacher"
)

// HandRaise raises the student's hand, optionally with a short reason (A → P).
type HandRaise struct {
	HandID string `json:"hand_id"`
	Note   string `json:"note,omitempty"`
}

// MessageType implements Payload.
func (HandRaise) MessageType() Type { return TypeHandRaise }

// Validate implements Payload.
func (p HandRaise) Validate() error {
	if !isUUIDv4(p.HandID) {
		return invalid(TypeHandRaise, "hand_id is not a UUID v4")
	}
	if utf8.RuneCountInString(p.Note) > MaxHandNote {
		return invalid(TypeHandRaise, "note exceeds %d characters", MaxHandNote)
	}
	return nil
}

// HandLower lowers a raised hand (A ↔ P). By must match the direction: the
// receiver checks it with ValidFrom.
type HandLower struct {
	HandID string `json:"hand_id"`
	By     HandBy `json:"by"`
}

// MessageType implements Payload.
func (HandLower) MessageType() Type { return TypeHandLower }

// Validate implements Payload.
func (p HandLower) Validate() error {
	if !isUUIDv4(p.HandID) {
		return invalid(TypeHandLower, "hand_id is not a UUID v4")
	}
	if p.By != HandByStudent && p.By != HandByTeacher {
		return invalid(TypeHandLower, "by must be student or teacher")
	}
	return nil
}

// ValidFrom checks that by matches the sender: HandByStudent from the agent,
// HandByTeacher from the console.
func (p HandLower) ValidFrom(by HandBy) error {
	if p.By != by {
		return invalid(TypeHandLower, "by must be %s in this direction", by)
	}
	return nil
}

// MsgStudent is a short plain-text message from the student to the teacher
// (A → P).
type MsgStudent struct {
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
}

// MessageType implements Payload.
func (MsgStudent) MessageType() Type { return TypeMsgStudent }

// Validate implements Payload.
func (p MsgStudent) Validate() error {
	if !isUUIDv4(p.MessageID) {
		return invalid(TypeMsgStudent, "message_id is not a UUID v4")
	}
	if n := utf8.RuneCountInString(p.Text); n < 1 || n > MaxStudentMsgText {
		return invalid(TypeMsgStudent, "text must be 1-%d characters", MaxStudentMsgText)
	}
	return nil
}

// MsgStudentAck confirms that a msg.student reached the console (P → A). It
// does not say that the teacher read it.
type MsgStudentAck struct {
	RefID     string `json:"ref_id"`
	MessageID string `json:"message_id"`
}

// MessageType implements Payload.
func (MsgStudentAck) MessageType() Type { return TypeMsgStudentAck }

// Validate implements Payload.
func (p MsgStudentAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeMsgStudentAck, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.MessageID) {
		return invalid(TypeMsgStudentAck, "message_id is not a UUID v4")
	}
	return nil
}
