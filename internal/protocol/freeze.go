package protocol

import "unicode/utf8"

// MaxFreezeMessage is the limit of screen.freeze.message in characters
// (docs/protocol.md §5.9, §7).
const MaxFreezeMessage = 300

// ScreenFreeze pauses the student's screen (P → A): the agent covers every
// monitor with its pause window until screen.unfreeze or the end of the
// connection. On a screen already paused it only replaces the message.
type ScreenFreeze struct {
	// Message is the teacher's optional note, plain text shown under the
	// fixed text. Never logged.
	Message string `json:"message,omitempty"`
}

// MessageType implements Payload.
func (ScreenFreeze) MessageType() Type { return TypeScreenFreeze }

// Validate implements Payload.
func (p ScreenFreeze) Validate() error {
	if !utf8.ValidString(p.Message) {
		return invalid(TypeScreenFreeze, "message is not valid UTF-8")
	}
	if utf8.RuneCountInString(p.Message) > MaxFreezeMessage {
		return invalid(TypeScreenFreeze, "message exceeds %d characters", MaxFreezeMessage)
	}
	return nil
}

// ScreenUnfreeze removes the pause (P → A). Idempotent.
type ScreenUnfreeze struct{}

// MessageType implements Payload.
func (ScreenUnfreeze) MessageType() Type { return TypeScreenUnfreeze }

// Validate implements Payload.
func (ScreenUnfreeze) Validate() error { return nil }

// ScreenFreezeAck confirms the state the screen was left in (A → P): Frozen
// true after screen.freeze, false after screen.unfreeze.
type ScreenFreezeAck struct {
	RefID  string `json:"ref_id"` // ID of the screen.freeze or screen.unfreeze
	Frozen bool   `json:"frozen"`
}

// MessageType implements Payload.
func (ScreenFreezeAck) MessageType() Type { return TypeScreenFreezeAck }

// Validate implements Payload.
func (p ScreenFreezeAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeScreenFreezeAck, "ref_id is not a UUID v4")
	}
	return nil
}
