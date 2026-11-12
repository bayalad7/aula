package ui

import (
	"encoding/xml"
	"os"
	"regexp"
	"testing"
)

// Every icon the apps use is a real <symbol> of the sprite, not text inside
// a comment (an icon pasted after the first "</svg>", which is inside the
// usage comment, renders nothing).
func TestIconsAreSymbolsOfTheSprite(t *testing.T) {
	data, err := os.ReadFile("iconos.svg")
	if err != nil {
		t.Fatal(err)
	}
	var sprite struct {
		Symbols []struct {
			ID string `xml:"id,attr"`
		} `xml:"symbol"`
	}
	if err := xml.Unmarshal(data, &sprite); err != nil {
		t.Fatalf("iconos.svg is not valid XML: %v", err)
	}
	defined := map[string]bool{}
	for _, s := range sprite.Symbols {
		defined[s.ID] = true
	}
	inText := regexp.MustCompile(`<symbol id="([a-z-]+)"`).FindAllSubmatch(data, -1)
	if len(inText) != len(sprite.Symbols) {
		t.Errorf("%d symbols written, %d parsed: one is inside a comment", len(inText), len(sprite.Symbols))
	}
	// data-icono="x" in the HTML and icono: "x" in main.js are drawn by iconos.js.
	used := regexp.MustCompile(`iconos\.svg#([a-z-]+)|icono\("([a-z-]+)"|data-icono="([a-z-]+)"|icono: "([a-z-]+)"`)
	for _, f := range []string{
		"../../apps/teacher/frontend/dist/index.html", "../../apps/teacher/frontend/dist/main.js",
		"../../apps/student/frontend/dist/index.html", "../../apps/student/frontend/dist/main.js",
	} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range used.FindAllSubmatch(src, -1) {
			id := string(m[1]) + string(m[2]) + string(m[3]) + string(m[4])
			if !defined[id] {
				t.Errorf("%s uses the icon %q, which iconos.svg does not define", f, id)
			}
		}
	}
}
