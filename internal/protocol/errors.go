package protocol

import "errors"

// ErrorCode is the machine-readable code carried by an error message.
type ErrorCode string

// Error codes defined by the protocol (docs/protocol.md §2.1).
const (
	CodeUnsupportedVersion    ErrorCode = "unsupported_version"
	CodeInvalidMessage        ErrorCode = "invalid_message"
	CodeMessageTooLarge       ErrorCode = "message_too_large"
	CodeUnknownType           ErrorCode = "unknown_type"
	CodeUnauthorized          ErrorCode = "unauthorized"
	CodeUnsupportedCapability ErrorCode = "unsupported_capability"
	CodeInternal              ErrorCode = "internal"
)

// Sentinel errors returned (wrapped) by Encode and Decode. Use errors.Is, or
// ErrorCodeFor to build the error reply.
var (
	ErrUnsupportedVersion = errors.New("unsupported protocol version")
	ErrInvalidMessage     = errors.New("invalid message")
	ErrMessageTooLarge    = errors.New("message too large")
	ErrUnknownType        = errors.New("unknown message type")
)

// ErrorCodeFor maps an error from Encode or Decode to the protocol error code
// that should be sent back. Unrecognized errors map to CodeInternal.
func ErrorCodeFor(err error) ErrorCode {
	switch {
	case errors.Is(err, ErrUnsupportedVersion):
		return CodeUnsupportedVersion
	case errors.Is(err, ErrMessageTooLarge):
		return CodeMessageTooLarge
	case errors.Is(err, ErrUnknownType):
		return CodeUnknownType
	case errors.Is(err, ErrInvalidMessage):
		return CodeInvalidMessage
	default:
		return CodeInternal
	}
}
