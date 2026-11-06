// Package collect gathers the students' work (file.collect.*,
// docs/protocol.md §5.21, docs/privacidad.md rules 60–65).
//
// A collection is a date and an activity. The console never sends a path:
// Folder turns them into recolecciones/<date>/<activity>, relative to the
// student's work folder, and both ends check it with the path rules of
// internal/filepush.
//
// On the agent, Responder creates the activity folder (open), and on a
// request tells the window before reading, lists and reads that folder only
// (platform.WorkFolderReader; in service mode the helper's proxy, so the
// files are read as the student), sends the files one after the other in
// parts (protocol.SplitParts) and waits for each ack. What it collected and
// when is kept for the window (Collections).
//
// On the console, Collector opens the collections, asks the students in
// batches of four, reassembles the parts (protocol.Reassembly), checks size
// and sha256 and saves each round in the teacher's
// Documentos\Aula\recolecciones\<date>\<activity>\<student>\ with a manifest
// (aula-recoleccion.json) ready for running tests later.
//
// The package never calls OS APIs beyond plain file calls on the console:
// the student's disk is behind platform.WorkFolderReader.
package collect
