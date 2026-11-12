// Package docs is the only source of the user manual and the diagrams: it
// embeds docs/administracion (the manual, with its img/ folder) and
// docs/diagramas so the console shows them without copies. Its tests
// check the documentation: the diagrams' front-matter and Mermaid blocks, the
// manual's links and includes, and the {{diagrama:...}} references.
package docs

import (
	"embed"
	"io/fs"
)

// Paths inside Files.
const (
	ManualDir   = "administracion"
	DiagramsDir = "diagramas"
)

//go:embed administracion diagramas
var files embed.FS

// Files returns the manual and the diagrams.
func Files() fs.FS { return files }
