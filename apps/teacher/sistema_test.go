package main

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const tokensFile = "frontend/dist/sistema/tokens.css"

var (
	tokenRe = regexp.MustCompile(`--([a-z0-9-]+):\s*([^;]+);`)
	hexRe   = regexp.MustCompile(`^#([0-9a-fA-F]{6})$`)
)

// themeBlocks returns the custom properties of the light theme (the first
// :root block), of the dark theme chosen by the system and of the dark theme
// set by hand.
func themeBlocks(t *testing.T) (light, darkAuto, darkManual map[string]string) {
	t.Helper()
	data, err := os.ReadFile(tokensFile)
	if err != nil {
		t.Fatal(err)
	}
	css := string(data)
	block := func(start string) map[string]string {
		i := strings.Index(css, start)
		if i < 0 {
			t.Fatalf("%s: no encuentro %q", tokensFile, start)
		}
		body := css[i+len(start):]
		body = body[:strings.Index(body, "}")]
		out := map[string]string{}
		for _, m := range tokenRe.FindAllStringSubmatch(body, -1) {
			out[m[1]] = strings.TrimSpace(m[2])
		}
		return out
	}
	return block(":root {"), block(`:root:not([data-theme="light"]) {`), block(`:root[data-theme="dark"] {`)
}

func luminance(hex string) (float64, error) {
	m := hexRe.FindStringSubmatch(hex)
	if m == nil {
		return 0, fmt.Errorf("%q no es un color #rrggbb", hex)
	}
	v, _ := strconv.ParseUint(m[1], 16, 32)
	ch := func(c uint64) float64 {
		s := float64(c) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(v>>16&0xff) + 0.7152*ch(v>>8&0xff) + 0.0722*ch(v&0xff), nil
}

func contrast(a, b string) (float64, error) {
	la, err := luminance(a)
	if err != nil {
		return 0, err
	}
	lb, err := luminance(b)
	if err != nil {
		return 0, err
	}
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05), nil
}

// TestTokensContrast checks WCAG 2.1 AA on the pairs the components use:
// 4.5:1 for text and 3:1 for control borders, focus rings and state marks.
func TestTokensContrast(t *testing.T) {
	text := [][2]string{
		{"c-texto", "c-fondo"}, {"c-texto", "c-superficie"}, {"c-texto", "c-superficie-2"},
		{"c-texto-suave", "c-fondo"}, {"c-texto-suave", "c-superficie"}, {"c-texto-suave", "c-superficie-2"},
		{"c-enlace", "c-fondo"}, {"c-enlace", "c-superficie"},
		{"c-acento-texto", "c-acento"}, {"c-acento-texto", "c-acento-hover"},
		{"c-acento-sobre-suave", "c-acento-suave"},
		{"c-conectado", "c-conectado-fondo"}, {"c-conectado", "c-superficie"},
		{"c-desconectado", "c-desconectado-fondo"}, {"c-desconectado", "c-superficie"},
		{"c-advertencia", "c-advertencia-fondo"}, {"c-advertencia", "c-superficie"},
		{"c-error", "c-error-fondo"}, {"c-error", "c-superficie"},
		{"c-error-solido-texto", "c-error-solido"},
		{"c-simulado", "c-simulado-fondo"}, {"c-simulado", "c-superficie"},
		{"c-info", "c-info-fondo"},
	}
	ui := [][2]string{
		{"c-borde-fuerte", "c-superficie"}, {"c-foco", "c-superficie"}, {"c-foco", "c-fondo"},
		{"c-conectado-solido", "c-superficie"}, {"c-desconectado-solido", "c-superficie"},
		{"c-advertencia-solido", "c-superficie"}, {"c-error-solido", "c-superficie"},
		{"c-simulado-solido", "c-superficie"}, {"c-acento", "c-superficie"},
	}
	light, darkAuto, darkManual := themeBlocks(t)
	for name, theme := range map[string]map[string]string{"claro": light, "oscuro": darkManual} {
		check := func(pairs [][2]string, min float64) {
			for _, p := range pairs {
				fg, bg := theme[p[0]], theme[p[1]]
				r, err := contrast(fg, bg)
				if err != nil {
					t.Errorf("%s: %s/%s: %v", name, p[0], p[1], err)
					continue
				}
				if r < min {
					t.Errorf("%s: --%s %s sobre --%s %s = %.2f:1, mínimo %.1f:1", name, p[0], fg, p[1], bg, r, min)
				}
			}
		}
		check(text, 4.5)
		check(ui, 3)
	}
	// The dark theme is written twice (by the system and by hand): keep them equal.
	if len(darkAuto) != len(darkManual) {
		t.Errorf("el oscuro automático tiene %d tokens y el manual %d", len(darkAuto), len(darkManual))
	}
	for k, v := range darkManual {
		if darkAuto[k] != v {
			t.Errorf("--%s: oscuro automático %q, manual %q", k, darkAuto[k], v)
		}
	}
	// Every color of the light theme exists in the dark one.
	for k, v := range light {
		if strings.HasPrefix(k, "c-") && darkManual[k] == "" {
			t.Errorf("--%s (%s) no tiene valor en el tema oscuro", k, v)
		}
	}
}
