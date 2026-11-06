// Package classtimer is the exercise timer of the class (docs/protocol.md
// §5.19): a countdown with a label that the teacher starts, extends and
// stops, and every student sees in a discreet badge.
//
// Both sides count by what is left, never by the wall clock: every message
// carries remaining_ms and each side sets its own end with its monotonic
// clock, so computers whose clocks differ still show the same count.
//
//   - Class is the console side: the timer in force, who it is for, what each
//     session was sent and acknowledged, and the messages that bring every
//     connected student up to date (also after a reconnection).
//   - Countdown is the agent side: it applies timer.start, timer.extend and
//     timer.stop, ends once at zero and says what the window shows.
//
// It does not call the OS. The label is the teacher's text and is never
// logged (docs/privacidad.md rule 54).
package classtimer
