// Package snapshot is the still screenshot of one student (docs/protocol.md
// §5.17, docs/privacidad.md rules 7 and 44-49).
//
// On the agent, Taker answers screen.snapshot.request: it shows the native
// indicator, captures the primary display through platform.ScreenCapture
// (in service mode, the helper's CallScreenCapture), tells the window to
// show its notice and sends the JPEG in parts. The image lives only in
// memory: the agent never writes it to disk or to a log.
//
// On the console, Store tracks each request, reassembles its parts, checks
// them, keeps the image in memory for the preview and, only when the teacher
// presses «Guardar», writes it under
// <Documentos>\Aula\capturas\{alumno}\{fecha}\{hora}.jpg (Save).
//
// The package never calls the OS: capture and the indicator come through
// interfaces, and files are plain os calls.
package snapshot
