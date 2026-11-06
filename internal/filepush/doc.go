// Package filepush sends the teacher's files to the students' work folder
// (file.push, docs/protocol.md §5.20, docs/privacidad.md rules 55-59).
//
// The path rules (CheckName, CheckDest) live here and run on both ends: the
// console refuses a bad name before sending and the agent answers
// invalid_path; neither ever fixes a path.
//
// On the console, NewPush reads and checks the files of one push and cuts
// them in parts (protocol.SplitParts); Pusher sends them one file after the
// other to each session, up to 4 sessions at a time, and waits for each
// file's ack. Templates keeps the reusable exercise templates as folders
// under <Documentos>\Aula\plantillas.
//
// On the agent, Receiver reassembles the parts (protocol.Reassembly) in
// memory, checks size and sha256, saves the file through a Writer
// (platform.WorkFolder; in service mode the helper's proxy, so the file
// belongs to the student) under another name when one is taken, and acks.
// Inbox is what the window shows: what arrived and where it is.
//
// The package never calls the OS: the student's disk is behind Writer, and
// the console's files are plain os calls.
package filepush
