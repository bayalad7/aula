// Package poll is the live poll of the class (docs/protocol.md §5.23): a quick
// question (multiple choice, yes or no, or a short open answer) that the
// teacher opens for the channel or a selection, the students answer from a
// card in their window, and the teacher closes and may share as totals.
//
//   - Class is the console side: the poll in force and the closed ones of the
//     class, one vote per student (the last counts until it closes), the
//     totals, and the messages that bring every connected student up to date,
//     also after a reconnection.
//   - Ballot is the agent side: the poll cards of the window, the answer
//     waiting for its ack (sent again on reconnection) and the shared totals.
//   - Quiz is a quiz (§5.23.1): several questions with their correct options
//     and points, graded all or nothing by the console; the correct options
//     never travel before the close and each student gets only their own
//     result. Library keeps the teacher's saved quizzes and Class.Grades
//     writes the CSV of «Exportar calificaciones» (real students only).
//
// It does not call the OS. The question, the options and the answers are text
// of the teacher and the students and are never logged; in an anonymous poll
// the console view never says who chose what (docs/privacidad.md rules 71-74
// and, for quizzes, 87-91).
package poll
