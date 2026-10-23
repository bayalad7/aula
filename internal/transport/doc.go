// Package transport carries protocol envelopes over WebSocket: the Server runs
// on the teacher's console and the Client on each student's agent.
//
// Each connection has exactly one reading goroutine and one writing goroutine
// fed by a queue, so the socket is never written concurrently. Messages are
// limited to protocol.MaxMessageSize in both directions, malformed messages are
// answered with a protocol error message, and a connection that receives
// nothing for the idle timeout is closed (docs/protocol.md §3.4).
//
// TLS is added in stage 05; until then URLs use ws://.
package transport
