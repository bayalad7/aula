package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// The diagnosis says what this computer can do and, in Wayland, what to do
// to see the screen (docs/compatibilidad.md, prompt 13).
func TestPrintAbilities(t *testing.T) {
	var out bytes.Buffer
	printAbilities(&out, platform.DisplayWayland, []protocol.Capability{protocol.CapFilePush, protocol.CapFileCollect})
	s := out.String()
	for _, want := range []string{"Wayland", "Ubuntu en Xorg", "✓ Recibir archivos", "✓ Recoger su trabajo",
		"No disponible en este sistema o sesión: Ver su pantalla en miniatura"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	out.Reset()
	printAbilities(&out, platform.DisplayNative, []protocol.Capability{protocol.CapScreenThumb})
	if s := out.String(); strings.Contains(s, "Sesión gráfica") || !strings.Contains(s, "✓ Ver su pantalla en miniatura") {
		t.Errorf("native:\n%s", s)
	}
	for _, a := range abilities {
		if a.label == "" || a.cap == "" {
			t.Errorf("ability without label: %+v", a)
		}
	}
}
