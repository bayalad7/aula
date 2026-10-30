package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/bayalad7/aula/internal/manual"
)

// TestExportManualForUX writes the manual as Manual() returns it, plus one
// search, for the web harness of scripts/ux (which has no Go backend). It
// only runs when AULA_UX_MANUAL names the file to write.
func TestExportManualForUX(t *testing.T) {
	out := os.Getenv("AULA_UX_MANUAL")
	if out == "" {
		t.Skip("AULA_UX_MANUAL not set: only scripts/ux uses this")
	}
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	v, err := app.Manual()
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(struct {
		Manual ManualView      `json:"manual"`
		Search []manual.Result `json:"search"`
	}{v, app.ManualSearch("firewall")})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
