package protocol

import "math"

// Limits of the remote.* messages (docs/protocol.md §5.16, §7).
const (
	MinRemoteInterval = 100  // ms: at most 10 frames per second
	MaxRemoteInterval = 1000 // ms
	MinRemoteWidth    = 640
	MaxRemoteWidth    = 1920
	MaxRemoteEvents   = 32   // events per remote.input
	MaxRemoteRate     = 60   // remote.input messages per second
	MaxRemoteWheel    = 1200 // wheel units per event (120 = one notch)
)

// RemoteStatus is the state a remote.ack reports.
type RemoteStatus string

// Remote control statuses.
const (
	RemoteActive  RemoteStatus = "active"  // the notice is shown; input is accepted
	RemotePaused  RemoteStatus = "paused"  // the student is using the computer
	RemoteResumed RemoteStatus = "resumed" // 2 s without input from the student
	RemoteEnded   RemoteStatus = "ended"   // see RemoteReason
)

// RemoteReason says why a control ended.
type RemoteReason string

// Reasons of remote.ack ended.
const (
	RemoteByTeacher RemoteReason = "teacher"  // remote.stop
	RemoteReplaced  RemoteReason = "replaced" // another remote.start
	RemoteIdle      RemoteReason = "idle"     // 5 minutes without remote.input
	RemoteSession   RemoteReason = "session"  // the student's session locked, changed or lost its helper
	RemoteError     RemoteReason = "error"    // the OS stopped accepting input
)

// Kinds of input event (InputEvent.T).
const (
	InputMove   = "move"
	InputButton = "button"
	InputWheel  = "wheel"
	InputKey    = "key"
)

// Mouse buttons (InputEvent.B).
const (
	ButtonLeft   = "left"
	ButtonRight  = "right"
	ButtonMiddle = "middle"
)

// KeyCodes is the closed list of physical keys remote.input may press, as
// KeyboardEvent.code values of a 102-key keyboard. The OS injects them by
// scancode, so the student's keyboard layout decides the character.
var KeyCodes = []string{
	"KeyA", "KeyB", "KeyC", "KeyD", "KeyE", "KeyF", "KeyG", "KeyH", "KeyI", "KeyJ", "KeyK", "KeyL", "KeyM",
	"KeyN", "KeyO", "KeyP", "KeyQ", "KeyR", "KeyS", "KeyT", "KeyU", "KeyV", "KeyW", "KeyX", "KeyY", "KeyZ",
	"Digit0", "Digit1", "Digit2", "Digit3", "Digit4", "Digit5", "Digit6", "Digit7", "Digit8", "Digit9",
	"F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12",
	"Escape", "Tab", "CapsLock", "Space", "Enter", "Backspace",
	"ShiftLeft", "ShiftRight", "ControlLeft", "ControlRight", "AltLeft", "AltRight", "ContextMenu",
	"Minus", "Equal", "BracketLeft", "BracketRight", "Backslash", "Semicolon", "Quote", "Backquote",
	"Comma", "Period", "Slash", "IntlBackslash",
	"ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight",
	"Insert", "Delete", "Home", "End", "PageUp", "PageDown", "PrintScreen", "ScrollLock", "Pause",
	"NumLock", "NumpadDivide", "NumpadMultiply", "NumpadSubtract", "NumpadAdd", "NumpadEnter", "NumpadDecimal",
	"Numpad0", "Numpad1", "Numpad2", "Numpad3", "Numpad4", "Numpad5", "Numpad6", "Numpad7", "Numpad8", "Numpad9",
}

var keyCodeSet = func() map[string]bool {
	m := make(map[string]bool, len(KeyCodes))
	for _, c := range KeyCodes {
		m[c] = true
	}
	return m
}()

// ValidKeyCode reports whether code is in KeyCodes.
func ValidKeyCode(code string) bool { return keyCodeSet[code] }

// RemoteStart starts (or, with the same control_id, retunes the video of) a
// remote control (P → A).
type RemoteStart struct {
	ControlID  string `json:"control_id"`
	IntervalMS int    `json:"interval_ms"`
	Quality    int    `json:"quality"`
	MaxWidth   int    `json:"max_width"`
}

// MessageType implements Payload.
func (RemoteStart) MessageType() Type { return TypeRemoteStart }

// Validate implements Payload.
func (p RemoteStart) Validate() error {
	if !isUUIDv4(p.ControlID) {
		return invalid(TypeRemoteStart, "control_id is not a UUID v4")
	}
	if p.IntervalMS < MinRemoteInterval || p.IntervalMS > MaxRemoteInterval {
		return invalid(TypeRemoteStart, "interval_ms must be %d-%d", MinRemoteInterval, MaxRemoteInterval)
	}
	if p.Quality < MinThumbQuality || p.Quality > MaxThumbQuality {
		return invalid(TypeRemoteStart, "quality must be %d-%d", MinThumbQuality, MaxThumbQuality)
	}
	if p.MaxWidth < MinRemoteWidth || p.MaxWidth > MaxRemoteWidth {
		return invalid(TypeRemoteStart, "max_width must be %d-%d", MinRemoteWidth, MaxRemoteWidth)
	}
	return nil
}

// InputEvent is one keyboard or mouse event of remote.input. Which fields
// apply depends on T (docs/protocol.md §5.16).
type InputEvent struct {
	T    string  `json:"t"`
	X    float64 `json:"x,omitempty"` // move, button: 0-1 of the primary display
	Y    float64 `json:"y,omitempty"`
	B    string  `json:"b,omitempty"`    // button
	Down bool    `json:"down,omitempty"` // button, key
	DX   int     `json:"dx,omitempty"`   // wheel
	DY   int     `json:"dy,omitempty"`
	Code string  `json:"code,omitempty"` // key: one of KeyCodes
}

func unit(v float64) bool { return !math.IsNaN(v) && v >= 0 && v <= 1 }

// validate checks one event.
func (e InputEvent) validate() error {
	switch e.T {
	case InputMove:
		if !unit(e.X) || !unit(e.Y) {
			return invalid(TypeRemoteInput, "move: x and y must be 0-1")
		}
	case InputButton:
		if e.B != ButtonLeft && e.B != ButtonRight && e.B != ButtonMiddle {
			return invalid(TypeRemoteInput, "button: b must be left, right or middle")
		}
		if !unit(e.X) || !unit(e.Y) {
			return invalid(TypeRemoteInput, "button: x and y must be 0-1")
		}
	case InputWheel:
		if e.DX == 0 && e.DY == 0 {
			return invalid(TypeRemoteInput, "wheel: dx or dy must be non-zero")
		}
		if abs(e.DX) > MaxRemoteWheel || abs(e.DY) > MaxRemoteWheel {
			return invalid(TypeRemoteInput, "wheel: dx and dy must be within ±%d", MaxRemoteWheel)
		}
	case InputKey:
		if !ValidKeyCode(e.Code) {
			return invalid(TypeRemoteInput, "key: unknown code")
		}
	default:
		return invalid(TypeRemoteInput, "event type must be move, button, wheel or key")
	}
	return nil
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// RemoteInput carries a batch of input events (P → A). No reply.
type RemoteInput struct {
	ControlID string       `json:"control_id"`
	Seq       int          `json:"seq"`
	Events    []InputEvent `json:"events"`
}

// MessageType implements Payload.
func (RemoteInput) MessageType() Type { return TypeRemoteInput }

// Validate implements Payload.
func (p RemoteInput) Validate() error {
	if !isUUIDv4(p.ControlID) {
		return invalid(TypeRemoteInput, "control_id is not a UUID v4")
	}
	if p.Seq < 1 {
		return invalid(TypeRemoteInput, "seq must be at least 1")
	}
	if len(p.Events) < 1 || len(p.Events) > MaxRemoteEvents {
		return invalid(TypeRemoteInput, "events must hold 1-%d events", MaxRemoteEvents)
	}
	for _, e := range p.Events {
		if err := e.validate(); err != nil {
			return err
		}
	}
	return nil
}

// RemoteStop ends a remote control (P → A).
type RemoteStop struct {
	ControlID string `json:"control_id"`
}

// MessageType implements Payload.
func (RemoteStop) MessageType() Type { return TypeRemoteStop }

// Validate implements Payload.
func (p RemoteStop) Validate() error {
	if !isUUIDv4(p.ControlID) {
		return invalid(TypeRemoteStop, "control_id is not a UUID v4")
	}
	return nil
}

// RemoteAck reports the state of a remote control (A → P).
type RemoteAck struct {
	RefID     string       `json:"ref_id"`
	ControlID string       `json:"control_id"`
	Status    RemoteStatus `json:"status"`
	Reason    RemoteReason `json:"reason,omitempty"` // only with ended
}

// MessageType implements Payload.
func (RemoteAck) MessageType() Type { return TypeRemoteAck }

// Validate implements Payload.
func (p RemoteAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeRemoteAck, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.ControlID) {
		return invalid(TypeRemoteAck, "control_id is not a UUID v4")
	}
	switch p.Status {
	case RemoteActive, RemotePaused, RemoteResumed:
		if p.Reason != "" {
			return invalid(TypeRemoteAck, "reason is only for ended")
		}
	case RemoteEnded:
		switch p.Reason {
		case RemoteByTeacher, RemoteReplaced, RemoteIdle, RemoteSession, RemoteError:
		default:
			return invalid(TypeRemoteAck, "ended needs reason teacher, replaced, idle, session or error")
		}
	default:
		return invalid(TypeRemoteAck, "status must be active, paused, resumed or ended")
	}
	return nil
}
