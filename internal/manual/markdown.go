package manual

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Inline is a run of text with one style. Href is set for links: a page
// ("conceptos"), a section ("conceptos#el-canal" or "#el-canal"), a console
// view ("pantalla:alumnos") or a diagram ("diagrama:12-diagnostico-conexion").
type Inline struct {
	Kind string `json:"kind"` // text | strong | em | code | link
	Text string `json:"text"`
	Href string `json:"href,omitempty"`
}

// Block is one piece of a page.
type Block struct {
	Kind    string       `json:"kind"`              // heading | paragraph | list | code | note | table | image | diagram
	Level   int          `json:"level,omitempty"`   // heading: 2 or 3
	Anchor  string       `json:"anchor,omitempty"`  // heading: its id
	Text    []Inline     `json:"text,omitempty"`    // heading, paragraph, note
	Ordered bool         `json:"ordered,omitempty"` // list
	Items   [][]Inline   `json:"items,omitempty"`   // list
	Lang    string       `json:"lang,omitempty"`    // code
	Code    string       `json:"code,omitempty"`    // code
	Head    [][]Inline   `json:"head,omitempty"`    // table
	Rows    [][][]Inline `json:"rows,omitempty"`    // table
	Src     string       `json:"src,omitempty"`     // image: file name inside img/
	Alt     string       `json:"alt,omitempty"`     // image
	Diagram string       `json:"diagram,omitempty"` // diagram: its id
}

// Page is one document of the manual.
type Page struct {
	Slug     string   `json:"slug"` // file name without .md
	Title    string   `json:"title"`
	Summary  string   `json:"summary"` // one sentence for the index and the search
	Group    string   `json:"group"`   // index group, e.g. "Primeros pasos"
	Order    int      `json:"order"`   // position in the index
	Screens  []string `json:"screens"` // console views it explains (contextual help)
	Blocks   []Block  `json:"blocks"`
	Headings []Block  `json:"headings"` // the ## and ### of the page, for the table of contents
}

var (
	reHeading = regexp.MustCompile(`^(#{1,3})\s+(.+?)\s*$`)
	reOrdered = regexp.MustCompile(`^\d+\.\s+(.*)$`)
	reBullet  = regexp.MustCompile(`^[-*]\s+(.*)$`)
	reImage   = regexp.MustCompile(`^!\[([^\]]*)\]\(img/([A-Za-z0-9._-]+)\)\s*$`)
	reInclude = regexp.MustCompile(`^\{\{diagrama:([a-z0-9-]+)\}\}\s*$`)
	reInline  = regexp.MustCompile("`([^`]+)`|\\*\\*(.+?)\\*\\*|\\[([^\\]]+)\\]\\(([^)\\s]+)\\)|(?:^|[^\\w*])\\*([^*\\s][^*]*?)\\*")
)

// ParsePage reads one manual page: a front-matter block, then the body.
func ParsePage(slug, src string) (Page, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	fm, body, err := frontMatter(src)
	if err != nil {
		return Page{}, fmt.Errorf("%s: %w", slug, err)
	}
	p := Page{Slug: slug, Title: fm["titulo"], Summary: fm["resumen"], Group: fm["grupo"]}
	if p.Title == "" || p.Group == "" {
		return Page{}, fmt.Errorf("%s: front-matter needs titulo and grupo", slug)
	}
	if o := fm["orden"]; o != "" {
		if p.Order, err = strconv.Atoi(o); err != nil {
			return Page{}, fmt.Errorf("%s: orden %q is not a number", slug, o)
		}
	}
	p.Screens = list(fm["pantallas"])
	p.Blocks = parseBlocks(body)
	seen := map[string]int{}
	for i, b := range p.Blocks {
		if b.Kind != "heading" {
			continue
		}
		a := Anchor(plain(b.Text))
		if n := seen[a]; n > 0 {
			a = a + "-" + strconv.Itoa(n)
		}
		seen[Anchor(plain(b.Text))]++
		p.Blocks[i].Anchor = a
		p.Headings = append(p.Headings, p.Blocks[i])
	}
	return p, nil
}

// frontMatter reads "clave: valor" lines between --- markers. Values may be
// quoted; lists are written [a, b].
func frontMatter(src string) (map[string]string, string, error) {
	if !strings.HasPrefix(src, "---\n") {
		return nil, "", fmt.Errorf("missing front-matter")
	}
	head, body, ok := strings.Cut(src[4:], "\n---\n")
	if !ok {
		return nil, "", fmt.Errorf("front-matter not closed")
	}
	out := map[string]string{}
	for _, line := range strings.Split(head, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			return nil, "", fmt.Errorf("front-matter line %q", line)
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
	}
	return out, body, nil
}

func list(v string) []string {
	v = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(v), "["), "]"))
	out := []string{}
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// parseBlocks splits the body into blocks, line by line.
func parseBlocks(body string) []Block {
	lines := strings.Split(body, "\n")
	var out []Block
	var para []string
	flush := func() {
		if len(para) > 0 {
			out = append(out, Block{Kind: "paragraph", Text: parseInline(strings.Join(para, " "))})
			para = nil
		}
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		t := strings.TrimSpace(line)
		switch {
		case t == "":
			flush()
		case strings.HasPrefix(t, "```"):
			flush()
			lang := strings.TrimSpace(strings.TrimPrefix(t, "```"))
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			out = append(out, Block{Kind: "code", Lang: lang, Code: strings.Join(code, "\n")})
		case reHeading.MatchString(t) && !strings.HasPrefix(line, " "):
			flush()
			m := reHeading.FindStringSubmatch(t)
			if len(m[1]) == 1 {
				continue // the page title comes from the front-matter
			}
			out = append(out, Block{Kind: "heading", Level: len(m[1]), Text: parseInline(m[2])})
		case reImage.MatchString(t):
			flush()
			m := reImage.FindStringSubmatch(t)
			out = append(out, Block{Kind: "image", Alt: m[1], Src: m[2]})
		case reInclude.MatchString(t):
			flush()
			out = append(out, Block{Kind: "diagram", Diagram: reInclude.FindStringSubmatch(t)[1]})
		case strings.HasPrefix(t, ">"):
			flush()
			var note []string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">"); i++ {
				note = append(note, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")))
			}
			i--
			out = append(out, Block{Kind: "note", Text: parseInline(strings.Join(note, " "))})
		case strings.HasPrefix(t, "|"):
			flush()
			var rows [][]string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|"); i++ {
				rows = append(rows, cells(lines[i]))
			}
			i--
			b := Block{Kind: "table"}
			for j, r := range rows {
				if j == 1 && isSeparator(r) {
					continue
				}
				var row [][]Inline
				for _, c := range r {
					row = append(row, parseInline(c))
				}
				if j == 0 {
					b.Head = row
				} else {
					b.Rows = append(b.Rows, row)
				}
			}
			out = append(out, b)
		case reBullet.MatchString(t) || reOrdered.MatchString(t):
			flush()
			ordered := reOrdered.MatchString(t)
			b := Block{Kind: "list", Ordered: ordered}
			var item []string
			for ; i < len(lines); i++ {
				l := lines[i]
				lt := strings.TrimSpace(l)
				switch {
				case lt == "":
					// A blank line ends the list unless the next line is an item of
					// the same kind.
					if i+1 < len(lines) && !strings.HasPrefix(lines[i+1], " ") {
						next := strings.TrimSpace(lines[i+1])
						if (ordered && reOrdered.MatchString(next)) || (!ordered && reBullet.MatchString(next)) {
							continue
						}
					}
					goto endList
				case !strings.HasPrefix(l, " ") && (reBullet.MatchString(lt) || reOrdered.MatchString(lt)):
					if item != nil {
						b.Items = append(b.Items, parseInline(strings.Join(item, " ")))
					}
					if m := reOrdered.FindStringSubmatch(lt); m != nil {
						item = []string{m[1]}
					} else {
						item = []string{reBullet.FindStringSubmatch(lt)[1]}
					}
				case strings.HasPrefix(l, " "):
					// Continuation, or a nested item flattened into the item text.
					item = append(item, strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(lt, "- "), "* ")))
				default:
					goto endList
				}
			}
		endList:
			if item != nil {
				b.Items = append(b.Items, parseInline(strings.Join(item, " ")))
			}
			i--
			out = append(out, b)
		default:
			para = append(para, t)
		}
	}
	flush()
	return out
}

func cells(line string) []string {
	t := strings.Trim(strings.TrimSpace(line), "|")
	var out []string
	for _, c := range strings.Split(t, "|") {
		out = append(out, strings.TrimSpace(c))
	}
	return out
}

func isSeparator(r []string) bool {
	for _, c := range r {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}

// parseInline splits text into styled runs: `code`, **strong**, *em* and
// [links](href). Anything else is text.
func parseInline(s string) []Inline {
	var out []Inline
	add := func(kind, text, href string) {
		if text == "" {
			return
		}
		if kind == "text" && len(out) > 0 && out[len(out)-1].Kind == "text" {
			out[len(out)-1].Text += text
			return
		}
		out = append(out, Inline{Kind: kind, Text: text, Href: href})
	}
	for len(s) > 0 {
		loc := reInline.FindStringSubmatchIndex(s)
		if loc == nil {
			add("text", s, "")
			break
		}
		start := loc[0]
		m := func(n int) string {
			if loc[2*n] < 0 {
				return ""
			}
			return s[loc[2*n]:loc[2*n+1]]
		}
		switch {
		case loc[2] >= 0:
			add("text", s[:start], "")
			add("code", m(1), "")
		case loc[4] >= 0:
			add("text", s[:start], "")
			add("strong", m(2), "")
		case loc[6] >= 0:
			add("text", s[:start], "")
			add("link", m(3), m(4))
		default:
			// *em*: the match may include the character before the star.
			star := strings.Index(s[start:loc[1]], "*") + start
			add("text", s[:star], "")
			add("em", m(5), "")
		}
		s = s[loc[1]:]
	}
	return out
}

// plain returns the text of inlines, without styles.
func plain(in []Inline) string {
	var b strings.Builder
	for _, x := range in {
		b.WriteString(x.Text)
	}
	return b.String()
}

// Anchor is the id of a heading: lower case, letters and digits, words
// joined with hyphens (like GitHub, so links work there too).
func Anchor(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}
