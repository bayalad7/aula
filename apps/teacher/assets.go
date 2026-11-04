package main

import (
	"embed"
	"io/fs"

	"github.com/bayalad7/aula/internal/ui"
)

//go:embed all:frontend/dist
var assets embed.FS

// frontend is what the window serves: frontend/dist with the shared design
// system (internal/ui) under sistema/.
func frontend() (fs.FS, error) {
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return nil, err
	}
	return ui.Assets(dist), nil
}
