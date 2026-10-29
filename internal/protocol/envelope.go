package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// Version is the protocol version carried in every envelope ("v") and in the
// mDNS TXT record "proto".
const Version = 1

// MaxMessageSize is the maximum size in bytes of an encoded envelope.
const MaxMessageSize = 1 << 20

// Type is the message type, in "domain.action" form.
type Type string

// Message types of protocol v0 (docs/protocol.md §5).
const (
	TypeAuthChallenge Type = "auth.challenge"
	TypeAuthProof     Type = "auth.proof"
	TypeAuthOK        Type = "auth.ok"
	TypeHello         Type = "hello"
	TypeHeartbeat     Type = "heartbeat"
	TypeError         Type = "error"
	TypeCodeShare     Type = "code.share"
	TypeCodeAck       Type = "code.ack"
	TypeCodeClear     Type = "code.clear"
	TypeMsgShow       Type = "msg.show"
	TypeMsgAck        Type = "msg.ack"
	// Screen thumbnails (docs/protocol.md §5.6–5.8).
	TypeScreenThumbStart Type = "screen.thumb.start"
	TypeScreenThumbFrame Type = "screen.thumb.frame"
	TypeScreenThumbStop  Type = "screen.thumb.stop"
)

var typePattern = regexp.MustCompile(`^[a-z]+(\.[a-z]+)+$`)

// Payload is implemented by every concrete payload type.
type Payload interface {
	// MessageType returns the envelope type this payload travels under.
	MessageType() Type
	// Validate checks the payload against the limits of docs/protocol.md. Errors
	// wrap ErrInvalidMessage or ErrMessageTooLarge.
	Validate() error
}

// Envelope is the common wrapper of every message.
type Envelope struct {
	V       int     // protocol version, always Version
	ID      string  // lowercase UUID v4
	TS      int64   // send time in Unix milliseconds; informational only
	Type    Type    // must equal Payload.MessageType()
	Payload Payload // one of the payload types of this package
}

// New wraps p in an envelope with a fresh ID and the current time.
func New(p Payload) Envelope {
	return Envelope{
		V:       Version,
		ID:      NewID(),
		TS:      time.Now().UnixMilli(),
		Type:    p.MessageType(),
		Payload: p,
	}
}

// wireEnvelope is the JSON form of Envelope, with the payload left raw so it
// can be decoded once the type is known.
type wireEnvelope struct {
	V       int             `json:"v"`
	ID      string          `json:"id"`
	TS      int64           `json:"ts"`
	Type    Type            `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Encode validates e and serializes it to JSON.
func Encode(e Envelope) ([]byte, error) {
	if e.V != Version {
		return nil, fmt.Errorf("encode: version %d: %w", e.V, ErrUnsupportedVersion)
	}
	if err := checkHeader(e.ID, e.TS); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	if e.Payload == nil {
		return nil, fmt.Errorf("encode %s: missing payload: %w", e.Type, ErrInvalidMessage)
	}
	if e.Type != e.Payload.MessageType() {
		return nil, fmt.Errorf("encode %s: payload is %s: %w", e.Type, e.Payload.MessageType(), ErrInvalidMessage)
	}
	if _, ok := decoders[e.Type]; !ok {
		return nil, fmt.Errorf("encode %s: %w", e.Type, ErrUnknownType)
	}
	if err := e.Payload.Validate(); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	raw, err := marshal(e.Payload)
	if err != nil {
		return nil, fmt.Errorf("encode %s payload: %w", e.Type, err)
	}
	data, err := marshal(wireEnvelope{V: e.V, ID: e.ID, TS: e.TS, Type: e.Type, Payload: raw})
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", e.Type, err)
	}
	if len(data) > MaxMessageSize {
		return nil, fmt.Errorf("encode %s: %d bytes exceeds %d: %w", e.Type, len(data), MaxMessageSize, ErrMessageTooLarge)
	}
	return data, nil
}

// Decode parses and validates one message.
//
// On error, the returned Envelope still carries the header fields that could be
// read (V, a valid ID, TS and Type), so the caller can reply with an error
// message whose ref_id is the ID. Use ErrorCodeFor to pick the code. An unknown
// but well-formed type yields ErrUnknownType and a nil Payload.
func Decode(data []byte) (Envelope, error) {
	if len(data) > MaxMessageSize {
		return Envelope{}, fmt.Errorf("decode: %d bytes exceeds %d: %w", len(data), MaxMessageSize, ErrMessageTooLarge)
	}
	var w wireEnvelope
	if err := json.Unmarshal(data, &w); err != nil {
		return Envelope{}, fmt.Errorf("decode envelope: %w: %w", ErrInvalidMessage, err)
	}
	e := Envelope{V: w.V, TS: w.TS}
	// Only echo the ID back if it is well formed; it ends up in ref_id.
	if isUUIDv4(w.ID) {
		e.ID = w.ID
	}
	if w.V != Version {
		return e, fmt.Errorf("decode: version %d: %w", w.V, ErrUnsupportedVersion)
	}
	if err := checkHeader(w.ID, w.TS); err != nil {
		return e, fmt.Errorf("decode: %w", err)
	}
	if !isValidTypeName(w.Type) {
		return e, fmt.Errorf("decode: malformed type: %w", ErrInvalidMessage)
	}
	e.Type = w.Type
	decode, ok := decoders[w.Type]
	if !ok {
		return e, fmt.Errorf("decode %s: %w", w.Type, ErrUnknownType)
	}
	if p := bytes.TrimSpace(w.Payload); len(p) == 0 || p[0] != '{' {
		return e, fmt.Errorf("decode %s: payload must be an object: %w", w.Type, ErrInvalidMessage)
	}
	p, err := decode(w.Payload)
	if err != nil {
		return e, fmt.Errorf("decode %s payload: %w: %w", w.Type, ErrInvalidMessage, err)
	}
	if err := p.Validate(); err != nil {
		return e, fmt.Errorf("decode: %w", err)
	}
	e.Payload = p
	return e, nil
}

// marshal is json.Marshal without HTML escaping: code is full of <, > and &,
// and escaping each one as < would inflate the message sixfold.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// checkHeader validates the envelope fields shared by Encode and Decode.
func checkHeader(id string, ts int64) error {
	if !isUUIDv4(id) {
		return fmt.Errorf("id is not a lowercase UUID v4: %w", ErrInvalidMessage)
	}
	if ts <= 0 {
		return fmt.Errorf("ts must be a positive Unix time in ms: %w", ErrInvalidMessage)
	}
	return nil
}

// isValidTypeName reports whether t has the syntax of a message type: either
// "domain.action" or one of the single-word types.
func isValidTypeName(t Type) bool {
	switch t {
	case TypeHello, TypeHeartbeat, TypeError:
		return true
	}
	return typePattern.MatchString(string(t))
}

// decoders maps each v0 type to the function that decodes its payload.
var decoders = map[Type]func(json.RawMessage) (Payload, error){
	TypeAuthChallenge:    decodeAs[AuthChallenge],
	TypeAuthProof:        decodeAs[AuthProof],
	TypeAuthOK:           decodeAs[AuthOK],
	TypeHello:            decodeAs[HelloInfo],
	TypeHeartbeat:        decodeAs[Heartbeat],
	TypeError:            decodeAs[ErrorPayload],
	TypeCodeShare:        decodeAs[CodeShare],
	TypeCodeAck:          decodeAs[CodeAck],
	TypeCodeClear:        decodeAs[CodeClear],
	TypeMsgShow:          decodeAs[MsgShow],
	TypeMsgAck:           decodeAs[MsgAck],
	TypeScreenThumbStart: decodeAs[ScreenThumbStart],
	TypeScreenThumbFrame: decodeAs[ScreenThumbFrame],
	TypeScreenThumbStop:  decodeAs[ScreenThumbStop],
}

func decodeAs[T Payload](raw json.RawMessage) (Payload, error) {
	var p T
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return p, nil
}
