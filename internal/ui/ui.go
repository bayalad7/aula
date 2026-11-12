// Package ui is the design system shared by the teacher console and the
// student app: CSS tokens with light and dark themes (tokens.css), the
// components (componentes.css) and the icon sprite (iconos.svg). There is a
// single copy, here; each Wails frontend gets it under sistema/ through
// Assets, so its HTML keeps linking "sistema/tokens.css".
//
// The package never calls the operating system (CLAUDE.md, golden rule).
package ui

import (
	"embed"
	"io/fs"
	"path"
	"strings"
)

// Dir is where the design system appears in each frontend.
const Dir = "sistema"

//go:embed tokens.css componentes.css iconos.svg receptor-video.js
var files embed.FS

// Files returns the design system, with its files at the root.
func Files() fs.FS { return files }

// Assets returns app (a frontend rooted at its index.html) with the design
// system mounted under sistema/. A sistema/ folder inside app is hidden: the
// design system has one source.
func Assets(app fs.FS) fs.FS { return overlay{app: app} }

type overlay struct{ app fs.FS }

func (o overlay) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if name == Dir {
		return files.Open(".")
	}
	if rest, ok := strings.CutPrefix(name, Dir+"/"); ok {
		return files.Open(path.Clean(rest))
	}
	return o.app.Open(name)
}
