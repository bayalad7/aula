// Package remote implements the remote control of one student's computer
// (docs/protocol.md §5.16, D-019): the teacher sees the screen live and drives
// its keyboard and mouse.
//
// Agent is the student side: it answers remote.start, remote.input and
// remote.stop, borrows the screen stream for the video, injects the input
// through a platform.InputInjector (in service mode, the helper's proxy) and
// pauses the teacher while the student uses the computer. It shows the
// always-visible notice before accepting any input and releases every key
// and button it pressed when the control ends.
//
// Desk is the console side: one control at a time, the state the agent
// reports, and the batching and pacing of remote.input.
//
// Neither calls the OS: the input goes through internal/platform. No log
// line says which keys were sent or where the pointer went
// (docs/privacidad.md rule 43).
package remote
