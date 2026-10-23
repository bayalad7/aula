package protocol

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

// Field limits of protocol v0 (docs/protocol.md §7).
const (
	NonceSize          = 32
	SignatureSize      = 64 // ed25519.SignatureSize; kept here to avoid importing crypto
	MaxHostname        = 255
	MaxUsername        = 256
	MaxOSVersion       = 64
	MaxCapabilities    = 64
	MaxCapabilityLen   = 64
	MaxErrorMessage    = 500
	MaxExercise        = 16
	MaxTitle           = 200
	MaxCodeContent     = 256 << 10 // bytes
	MaxMsgText         = 2000
	maxErrorCodeLength = 64
)

var (
	semverPattern     = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)
	capabilityPattern = regexp.MustCompile(`^[a-z]+(\.[a-z]+)*$`)
	langPattern       = regexp.MustCompile(`^[a-z0-9+#-]{1,32}$`)
	errorCodePattern  = regexp.MustCompile(`^[a-z]+(_[a-z]+)*$`)
)

// AuthChallenge is the first message of a connection (A → P).
type AuthChallenge struct {
	Nonce []byte `json:"nonce"` // NonceSize random bytes, Base64 on the wire
}

// MessageType implements Payload.
func (AuthChallenge) MessageType() Type { return TypeAuthChallenge }

// Validate implements Payload.
func (p AuthChallenge) Validate() error {
	if len(p.Nonce) != NonceSize {
		return invalid(TypeAuthChallenge, "nonce must be %d bytes", NonceSize)
	}
	return nil
}

// AuthProof carries the console's Ed25519 signature over the challenge (P → A).
type AuthProof struct {
	RefID     string `json:"ref_id"`
	Signature []byte `json:"signature"` // SignatureSize bytes, Base64 on the wire
}

// MessageType implements Payload.
func (AuthProof) MessageType() Type { return TypeAuthProof }

// Validate implements Payload.
func (p AuthProof) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeAuthProof, "ref_id is not a UUID v4")
	}
	if len(p.Signature) != SignatureSize {
		return invalid(TypeAuthProof, "signature must be %d bytes", SignatureSize)
	}
	return nil
}

// AuthOK tells the console that its proof was accepted (A → P).
type AuthOK struct {
	RefID string `json:"ref_id"`
}

// MessageType implements Payload.
func (AuthOK) MessageType() Type { return TypeAuthOK }

// Validate implements Payload.
func (p AuthOK) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeAuthOK, "ref_id is not a UUID v4")
	}
	return nil
}

// OS identifies the agent's operating system in hello.
type OS string

// Operating systems accepted in hello.
const (
	OSWindows OS = "windows"
	OSLinux   OS = "linux"
	OSDarwin  OS = "darwin"
)

// HelloInfo is the hello payload: the agent introduces itself right after
// auth.ok (A → P).
type HelloInfo struct {
	Hostname     string       `json:"hostname"`
	Username     string       `json:"username"`
	OS           OS           `json:"os"`
	OSVersion    string       `json:"os_version"`
	AgentVersion string       `json:"agent_version"` // SemVer x.y.z of the release
	Capabilities []Capability `json:"capabilities"`
}

// MessageType implements Payload.
func (HelloInfo) MessageType() Type { return TypeHello }

// Validate implements Payload. Unknown but well-formed capabilities are
// accepted: a newer agent may declare features this console does not know.
func (p HelloInfo) Validate() error {
	if n := utf8.RuneCountInString(p.Hostname); n < 1 || n > MaxHostname {
		return invalid(TypeHello, "hostname must be 1-%d characters", MaxHostname)
	}
	if utf8.RuneCountInString(p.Username) > MaxUsername {
		return invalid(TypeHello, "username exceeds %d characters", MaxUsername)
	}
	switch p.OS {
	case OSWindows, OSLinux, OSDarwin:
	default:
		return invalid(TypeHello, "os must be windows, linux or darwin")
	}
	if utf8.RuneCountInString(p.OSVersion) > MaxOSVersion {
		return invalid(TypeHello, "os_version exceeds %d characters", MaxOSVersion)
	}
	if !semverPattern.MatchString(p.AgentVersion) {
		return invalid(TypeHello, "agent_version is not SemVer")
	}
	if len(p.Capabilities) > MaxCapabilities {
		return invalid(TypeHello, "more than %d capabilities", MaxCapabilities)
	}
	seen := make(map[Capability]bool, len(p.Capabilities))
	for _, c := range p.Capabilities {
		if len(c) > MaxCapabilityLen || !capabilityPattern.MatchString(string(c)) {
			return invalid(TypeHello, "malformed capability")
		}
		if seen[c] {
			return invalid(TypeHello, "duplicate capability")
		}
		seen[c] = true
	}
	return nil
}

// Heartbeat is sent by both sides every 5 s after auth.ok.
type Heartbeat struct{}

// MessageType implements Payload.
func (Heartbeat) MessageType() Type { return TypeHeartbeat }

// Validate implements Payload.
func (Heartbeat) Validate() error { return nil }

// ErrorPayload reports that a message could not be processed (A ↔ P).
type ErrorPayload struct {
	RefID   string    `json:"ref_id"` // "" when the offending message was unreadable
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// MessageType implements Payload.
func (ErrorPayload) MessageType() Type { return TypeError }

// Validate implements Payload. Unknown but well-formed codes are accepted for
// forward compatibility.
func (p ErrorPayload) Validate() error {
	if p.RefID != "" && !isUUIDv4(p.RefID) {
		return invalid(TypeError, "ref_id is not a UUID v4")
	}
	if len(p.Code) > maxErrorCodeLength || !errorCodePattern.MatchString(string(p.Code)) {
		return invalid(TypeError, "malformed code")
	}
	if utf8.RuneCountInString(p.Message) > MaxErrorMessage {
		return invalid(TypeError, "message exceeds %d characters", MaxErrorMessage)
	}
	return nil
}

// NewError builds the error reply for refID, truncating message to
// MaxErrorMessage characters as the sender must.
func NewError(refID string, code ErrorCode, message string) ErrorPayload {
	if utf8.RuneCountInString(message) > MaxErrorMessage {
		message = string([]rune(message)[:MaxErrorMessage])
	}
	return ErrorPayload{RefID: refID, Code: code, Message: message}
}

// CodeMode tells the agent how to combine a snippet with what it shows.
type CodeMode string

// Snippet modes.
const (
	ModeReplace CodeMode = "replace"
	ModeAppend  CodeMode = "append"
)

// CodeShare shows a code snippet in the student's overlay (P → A).
type CodeShare struct {
	SnippetID string   `json:"snippet_id"`
	Exercise  string   `json:"exercise,omitempty"` // optional exercise label, e.g. "3.b"
	Title     string   `json:"title"`
	Lang      string   `json:"lang"` // Monaco language id; unknown ids render as plaintext
	Content   string   `json:"content"`
	Mode      CodeMode `json:"mode"`
}

// MessageType implements Payload.
func (CodeShare) MessageType() Type { return TypeCodeShare }

// Validate implements Payload. Oversized content wraps ErrMessageTooLarge; the
// other violations wrap ErrInvalidMessage.
func (p CodeShare) Validate() error {
	if !isUUIDv4(p.SnippetID) {
		return invalid(TypeCodeShare, "snippet_id is not a UUID v4")
	}
	if utf8.RuneCountInString(p.Exercise) > MaxExercise {
		return invalid(TypeCodeShare, "exercise exceeds %d characters", MaxExercise)
	}
	if utf8.RuneCountInString(p.Title) > MaxTitle {
		return invalid(TypeCodeShare, "title exceeds %d characters", MaxTitle)
	}
	if !langPattern.MatchString(p.Lang) {
		return invalid(TypeCodeShare, "malformed lang")
	}
	if len(p.Content) > MaxCodeContent {
		return fmt.Errorf("%s: content is %d bytes, limit %d: %w", TypeCodeShare, len(p.Content), MaxCodeContent, ErrMessageTooLarge)
	}
	if !utf8.ValidString(p.Content) {
		return invalid(TypeCodeShare, "content is not valid UTF-8")
	}
	switch p.Mode {
	case ModeReplace, ModeAppend:
	default:
		return invalid(TypeCodeShare, "mode must be replace or append")
	}
	return nil
}

// CodeAck confirms that a code.share was shown or a code.clear applied (A → P).
type CodeAck struct {
	RefID     string `json:"ref_id"`
	SnippetID string `json:"snippet_id"` // "" when acknowledging code.clear
}

// MessageType implements Payload.
func (CodeAck) MessageType() Type { return TypeCodeAck }

// Validate implements Payload.
func (p CodeAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeCodeAck, "ref_id is not a UUID v4")
	}
	if p.SnippetID != "" && !isUUIDv4(p.SnippetID) {
		return invalid(TypeCodeAck, "snippet_id is not a UUID v4")
	}
	return nil
}

// CodeClear empties the student's code overlay (P → A).
type CodeClear struct{}

// MessageType implements Payload.
func (CodeClear) MessageType() Type { return TypeCodeClear }

// Validate implements Payload.
func (CodeClear) Validate() error { return nil }

// MsgLevel is the visual level of a msg.show notice.
type MsgLevel string

// Message levels.
const (
	LevelInfo    MsgLevel = "info"
	LevelWarning MsgLevel = "warning"
)

// MsgShow displays a plain-text notice to the student (P → A).
type MsgShow struct {
	Text  string   `json:"text"`
	Level MsgLevel `json:"level"`
}

// MessageType implements Payload.
func (MsgShow) MessageType() Type { return TypeMsgShow }

// Validate implements Payload.
func (p MsgShow) Validate() error {
	if n := utf8.RuneCountInString(p.Text); n < 1 || n > MaxMsgText {
		return invalid(TypeMsgShow, "text must be 1-%d characters", MaxMsgText)
	}
	switch p.Level {
	case LevelInfo, LevelWarning:
	default:
		return invalid(TypeMsgShow, "level must be info or warning")
	}
	return nil
}

// MsgAck confirms that a msg.show was displayed (A → P).
type MsgAck struct {
	RefID string `json:"ref_id"`
}

// MessageType implements Payload.
func (MsgAck) MessageType() Type { return TypeMsgAck }

// Validate implements Payload.
func (p MsgAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeMsgAck, "ref_id is not a UUID v4")
	}
	return nil
}

// invalid builds a validation error wrapping ErrInvalidMessage. The reason
// must name fields and limits only, never field contents.
func invalid(t Type, format string, args ...any) error {
	return fmt.Errorf("%s: %s: %w", t, fmt.Sprintf(format, args...), ErrInvalidMessage)
}
