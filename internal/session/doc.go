// Package session handles the session lifecycle on both ends:
//
//   - Registry keeps the agents connected to the console, per channel.
//   - Console drives the teacher side: it consumes transport.Server events,
//     requires hello as the first message and keeps the Registry up to date.
//   - Agent drives the student side: it finds a console through a
//     ConsoleSource, connects, sends hello and heartbeats, and reconnects with
//     exponential backoff.
//
// Authentication (auth.challenge / auth.proof / auth.ok before hello) is added
// in stage 05; until then hello is the first message of a connection.
package session
