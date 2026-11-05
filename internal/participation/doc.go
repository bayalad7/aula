// Package participation is what the students answer live in class (docs/protocol.md
// §5.18, docs/privacidad.md rules 50-52). For now: raised hands and short
// messages to the teacher.
//
// On the console, Board keeps the hands in order of arrival (one per student,
// a repeated hand.raise keeps its place) and the students' messages, with the
// pace of each agent. On the agent, Desk keeps the student's own hand and the
// messages that wait for their ack, so they are sent again after a
// reconnection with the same IDs.
//
// Everything lives in memory and the package never logs the text of a message
// or a hand's note. It never calls the OS.
package participation
