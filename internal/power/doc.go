// Package power implements power.* (docs/protocol.md §5.15, D-018) without
// touching the OS: the agent's countdown with its notice and postponing,
// which ends in a call to platform.Power, and the console's record of the
// state of each order.
//
// Safety (docs/privacidad.md rules 26-31): without force the student sees the
// notice before anything happens and may postpone twice; the agent logs the
// order and sends power.ack executing before calling the OS. Messages only
// arrive here after auth.ok: the session rejects everything earlier.
package power
