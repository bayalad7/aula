// Package manual turns the user manual (docs/administracion/*.md) and the
// diagrams (docs/diagramas/*.md) into data the console draws, so docs/
// stays the only source of both.
//
// Pages are written in a small subset of Markdown (headings, paragraphs,
// lists, tables, code, notes, images, links and {{diagrama:id}} includes).
// They are parsed into blocks and inlines, never into HTML: the frontend
// builds the page with DOM calls and text nodes, so nothing in a page can run
// or inject markup. Unknown syntax, raw HTML included, is shown as text.
//
// It also builds the local search index and reads the "¿Falta un alumno?"
// decision tree from the diagnostic flowchart (diagram 12).
//
// It does not touch the network or the OS: it reads an fs.FS.
package manual
