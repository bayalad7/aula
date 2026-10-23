// Package protocol defines the Aula wire messages (protocol version 1) and their
// JSON (de)serialization, as specified in docs/protocol.md.
//
// Every message is an [Envelope] whose Payload is one of the concrete payload
// types of this package. [Decode] works in two steps: it parses the envelope
// keeping the payload as raw JSON, checks the version and the type, and then
// decodes the payload into its concrete Go type and validates it. Callers get a
// typed, validated payload they can switch on, never raw JSON.
//
// It must not perform network I/O nor call operating-system APIs.
package protocol
