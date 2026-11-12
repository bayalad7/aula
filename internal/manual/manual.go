package manual

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Diagram is a diagram of docs/diagramas, as the manual includes it: its
// metadata, the «Lo que no es obvio» prose, and the SVGs drawn beforehand
// (diagrams.go). Without them the console shows the Mermaid text.
type Diagram struct {
	ID          string   `json:"id"` // file name without .md, e.g. "12-diagnostico-conexion"
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Audience    []string `json:"audience"`
	Screens     []string `json:"screens"` // console views whose «?» shows it
	Alt         string   `json:"alt"`     // text alternative of the drawing
	Explanation []Block  `json:"explanation"`
	Mermaid     string   `json:"mermaid"`
	Light       string   `json:"light,omitempty"` // data: URI of the light SVG
	Dark        string   `json:"dark,omitempty"`  // data: URI of the dark SVG
}

// Manual is the whole manual: pages in index order and the diagrams.
type Manual struct {
	Pages    []Page             `json:"pages"`
	Diagrams map[string]Diagram `json:"diagrams"`
	Images   map[string][]byte  `json:"-"` // img/<name> → bytes
}

// Load reads the pages in pagesDir (with their img/ folder) and the diagrams
// in diagramsDir.
func Load(fsys fs.FS, pagesDir, diagramsDir string) (*Manual, error) {
	m := &Manual{Diagrams: map[string]Diagram{}, Images: map[string][]byte{}}
	names, err := fs.Glob(fsys, path.Join(pagesDir, "*.md"))
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if path.Base(name) == "README.md" {
			continue
		}
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		p, err := ParsePage(strings.TrimSuffix(path.Base(name), ".md"), string(b))
		if err != nil {
			return nil, fmt.Errorf("manual: %w", err)
		}
		m.Pages = append(m.Pages, p)
	}
	sort.SliceStable(m.Pages, func(i, j int) bool { return m.Pages[i].Order < m.Pages[j].Order })
	imgs, _ := fs.Glob(fsys, path.Join(pagesDir, "img", "*"))
	for _, name := range imgs {
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		m.Images[path.Base(name)] = b
	}
	diags, err := fs.Glob(fsys, path.Join(diagramsDir, "[0-9][0-9]-*.md"))
	if err != nil {
		return nil, err
	}
	for _, name := range diags {
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		d, err := parseDiagram(strings.TrimSuffix(path.Base(name), ".md"), string(b))
		if err != nil {
			return nil, fmt.Errorf("manual: %w", err)
		}
		m.Diagrams[d.ID] = d
	}
	if err := m.attachDrawings(fsys, diagramsDir); err != nil {
		return nil, fmt.Errorf("manual: diagrams: %w", err)
	}
	return m, nil
}

func parseDiagram(id, src string) (Diagram, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	fm, body, err := frontMatter(src)
	if err != nil {
		return Diagram{}, fmt.Errorf("%s: %w", id, err)
	}
	d := Diagram{ID: id, Title: fm["titulo"], Description: fm["descripcion"], Audience: list(fm["audiencia"]),
		Screens: list(fm["pantallas"]), Explanation: section(body, "Lo que no es obvio")}
	d.Alt = d.Title + ": " + d.Description
	_, rest, ok := strings.Cut(body, "```mermaid\n")
	if !ok {
		return Diagram{}, fmt.Errorf("%s: no mermaid block", id)
	}
	d.Mermaid, _, _ = strings.Cut(rest, "\n```")
	return d, nil
}

// Page returns the page with that slug.
func (m *Manual) Page(slug string) (Page, bool) {
	i := slices.IndexFunc(m.Pages, func(p Page) bool { return p.Slug == slug })
	if i < 0 {
		return Page{}, false
	}
	return m.Pages[i], true
}

// Resolves reports whether a link target exists: "page", "page#anchor",
// "diagrama:id" or "pantalla:view" (views are checked against views).
func (m *Manual) Resolves(href, fromSlug string, views []string) bool {
	switch {
	case strings.HasPrefix(href, "pantalla:"):
		return slices.Contains(views, strings.TrimPrefix(href, "pantalla:"))
	case strings.HasPrefix(href, "diagrama:"):
		_, ok := m.Diagrams[strings.TrimPrefix(href, "diagrama:")]
		return ok
	}
	slug, anchor, _ := strings.Cut(href, "#")
	if slug == "" {
		slug = fromSlug
	}
	p, ok := m.Page(slug)
	if !ok {
		return false
	}
	return anchor == "" || slices.ContainsFunc(p.Headings, func(h Block) bool { return h.Anchor == anchor })
}

// Validate checks every link, include and image of every page against the
// manual and the console's views, and that each view has its help page.
func (m *Manual) Validate(views []string) []error {
	var errs []error
	slugs := map[string]bool{}
	helped := map[string]bool{}
	for _, p := range m.Pages {
		if slugs[p.Slug] {
			errs = append(errs, fmt.Errorf("%s: duplicated page", p.Slug))
		}
		slugs[p.Slug] = true
		for _, s := range p.Screens {
			if !slices.Contains(views, s) {
				errs = append(errs, fmt.Errorf("%s: pantallas lists %q, which is not a console view", p.Slug, s))
			}
			helped[s] = true
		}
		check := func(in []Inline) {
			for _, x := range in {
				if x.Kind == "link" && !m.Resolves(x.Href, p.Slug, views) {
					errs = append(errs, fmt.Errorf("%s: link %q goes nowhere", p.Slug, x.Href))
				}
			}
		}
		for _, b := range p.Blocks {
			check(b.Text)
			for _, it := range b.Items {
				check(it)
			}
			for _, row := range append(slices.Clone(b.Rows), b.Head) {
				for _, c := range row {
					check(c)
				}
			}
			if b.Kind == "diagram" {
				if _, ok := m.Diagrams[b.Diagram]; !ok {
					errs = append(errs, fmt.Errorf("%s: {{diagrama:%s}} does not exist", p.Slug, b.Diagram))
				}
			}
			if b.Kind == "image" {
				if _, ok := m.Images[b.Src]; !ok {
					errs = append(errs, fmt.Errorf("%s: image img/%s does not exist", p.Slug, b.Src))
				}
				if strings.TrimSpace(b.Alt) == "" {
					errs = append(errs, fmt.Errorf("%s: image img/%s needs a description", p.Slug, b.Src))
				}
			}
		}
	}
	for _, v := range views {
		if !helped[v] {
			errs = append(errs, fmt.Errorf("console view %q has no help page (pantallas: [%s])", v, v))
		}
	}
	return errs
}

// Result is a search hit: a page, or a section of it, or a diagram.
type Result struct {
	Slug    string `json:"slug"`    // page; "" for a diagram
	Anchor  string `json:"anchor"`  // section, if any
	Diagram string `json:"diagram"` // diagram id, if the hit is a diagram
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	score   int
}

// Search finds the sections and diagrams that contain every word of q,
// without minding case or accents. Titles weigh more than text.
func (m *Manual) Search(q string) []Result {
	words := strings.Fields(fold(q))
	if len(words) == 0 {
		return []Result{}
	}
	var out []Result
	consider := func(r Result, title, text string) {
		ft, fx := fold(title), fold(text)
		score := 0
		for _, w := range words {
			switch {
			case strings.Contains(ft, w):
				score += 3
			case strings.Contains(fx, w):
				score++
			default:
				return
			}
		}
		r.score = score
		r.Snippet = snippet(text, words)
		out = append(out, r)
	}
	for _, p := range m.Pages {
		// The page intro, then one entry per section.
		sec := Result{Slug: p.Slug, Title: p.Title}
		var text strings.Builder
		text.WriteString(p.Summary + " ")
		flush := func() { consider(sec, sec.Title, text.String()) }
		for _, b := range p.Blocks {
			if b.Kind == "heading" {
				flush()
				sec = Result{Slug: p.Slug, Anchor: b.Anchor, Title: p.Title + " › " + plain(b.Text)}
				text.Reset()
				continue
			}
			text.WriteString(blockText(b) + " ")
		}
		flush()
	}
	for _, d := range m.Diagrams {
		var text strings.Builder
		text.WriteString(d.Description + " ")
		for _, b := range d.Explanation {
			text.WriteString(blockText(b) + " ")
		}
		consider(Result{Diagram: d.ID, Title: d.Title}, d.Title, text.String())
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

func blockText(b Block) string {
	parts := []string{plain(b.Text), b.Code}
	for _, it := range b.Items {
		parts = append(parts, plain(it))
	}
	for _, row := range append(slices.Clone(b.Rows), b.Head) {
		for _, c := range row {
			parts = append(parts, plain(c))
		}
	}
	return strings.Join(parts, " ")
}

// fold lowers case and removes accents, for matching.
func fold(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, strings.ToLower(s))
	if err != nil {
		return strings.ToLower(s)
	}
	return out
}

// snippet returns about 140 characters of text around the first word found.
func snippet(text string, words []string) string {
	text = strings.Join(strings.Fields(text), " ")
	r := []rune(text)
	f := []rune(fold(text))
	at := 0
	for _, w := range words {
		if i := strings.Index(string(f), w); i >= 0 {
			at = len([]rune(string(f)[:i]))
			break
		}
	}
	start := max(0, at-50)
	end := min(len(r), start+140)
	s := string(r[start:end])
	if start > 0 {
		s = "…" + s
	}
	if end < len(r) {
		s += "…"
	}
	return s
}
