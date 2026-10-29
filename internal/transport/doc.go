// Package transport carries protocol envelopes over WebSocket: the Server runs
// on the teacher's console and the Client on each student's agent.
//
// Each connection has exactly one reading goroutine and one writing goroutine
// fed by a queue, so the socket is never written concurrently. Messages are
// limited to protocol.MaxMessageSize in both directions, malformed messages are
// answered with a protocol error message, and a connection that receives
// nothing for the idle timeout is closed (docs/protocol.md §3.4).
//
// Agents and console always use TLS (Options.TLS, built by package security);
// plain ws:// is only used by this package's own tests.
package transport
