package ui

import (
	"fmt"
	"image/color"
	"regexp"
	"strconv"
	"strings"
)

var (
	colorTokenRe = regexp.MustCompile(`--([a-z0-9-]+):\s*([^;]+);`)
	colorHexRe   = regexp.MustCompile(`^#([0-9a-fA-F]{6})$`)
)

// Color returns the token --<name> of tokens.css as a color: the light theme
// (the first :root block) or, with dark, the dark theme chosen by the system.
// It lets a native window (the student's pause window) use the same colors
// as the pages, with tokens.css as the only source.
func Color(name string, dark bool) (color.RGBA, error) {
	data, err := files.ReadFile("tokens.css")
	if err != nil {
		return color.RGBA{}, fmt.Errorf("color %s: %w", name, err)
	}
	css := string(data)
	start := ":root {"
	if dark {
		start = `:root:not([data-theme="light"]) {`
	}
	i := strings.Index(css, start)
	if i < 0 {
		return color.RGBA{}, fmt.Errorf("color %s: no theme block %q", name, start)
	}
	body := css[i+len(start):]
	if j := strings.Index(body, "}"); j >= 0 {
		body = body[:j]
	}
	for _, m := range colorTokenRe.FindAllStringSubmatch(body, -1) {
		if m[1] != name {
			continue
		}
		h := colorHexRe.FindStringSubmatch(strings.TrimSpace(m[2]))
		if h == nil {
			return color.RGBA{}, fmt.Errorf("color %s: %q is not #rrggbb", name, m[2])
		}
		v, _ := strconv.ParseUint(h[1], 16, 32)
		return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}, nil
	}
	return color.RGBA{}, fmt.Errorf("color %s: not in the theme", name)
}
