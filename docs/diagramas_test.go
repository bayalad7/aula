package docs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"
)

// Paths relative to this package (docs/).
const (
	diagramsDir  = "v1/diagramas"
	promptsDir   = "v1/prompts"
	repoRoot     = ".."
	consoleIndex = "../apps/teacher/frontend/dist/index.html"
)

// audiences are the valid values of "audiencia" (README, «Metadatos»).
var audiences = []string{"profesor", "alumno", "instalador", "desarrollador", "auditor"}

// diagramTypes are the Mermaid diagrams the README uses.
var diagramTypes = []string{"flowchart", "sequenceDiagram", "stateDiagram-v2", "erDiagram"}

// classDefs are the color classes every flowchart and state diagram defines,
// exactly as in the README.
var classDefs = []string{
	"classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827",
	"classDef aula fill:#dcfce7,stroke:#15803d,color:#111827",
	"classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827",
	"classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827",
}

// boxColors are the same colors for the box and rect of sequence diagrams,
// which have no classDef.
var boxColors = []string{"rgb(219,234,254)", "rgb(220,252,231)", "rgb(255,237,213)", "rgb(254,226,226)"}

var (
	fileNameRe  = regexp.MustCompile(`^(\d{2})-[a-z0-9-]+\.md$`)
	fieldRe     = regexp.MustCompile(`^([a-z_]+): (.*)$`)
	listItemRe  = regexp.MustCompile(`^[A-Za-z0-9_./#=:-]+$`)
	docRefRe    = regexp.MustCompile(`^[a-z0-9-]+(/[a-z0-9-]+)*$`)
	screenRe    = regexp.MustCompile(`<section\s+id="([a-z0-9-]+)"`)
	includeRe   = regexp.MustCompile(`\{\{diagrama:([^}]*)\}\}`)
	codeSpanRe  = regexp.MustCompile("`([^`]+)`")
	sourcePath  = regexp.MustCompile(`^[a-z][A-Za-z0-9_.*-]*(/[A-Za-z0-9_.*-]+)+$`)
	readmeRowRe = regexp.MustCompile("^\\| (\\d{2}) \\| (.+?) \\| `([A-Za-z0-9-]+)` \\| (.+?) \\|$")
	readmeLink  = regexp.MustCompile(`^\[(.+)\]\((\d{2}-[a-z0-9-]+\.md)\)$`)
	classUseRe  = regexp.MustCompile(`(?m)^\s*class \S+ (\S+)\s*$|:::([A-Za-z0-9_-]+)`)
	boxRe       = regexp.MustCompile(`(?m)^\s*(?:box|rect) (rgb\([^)]*\))`)
)

// frontMatter is the YAML header of a diagram (README, «Metadatos»).
type frontMatter struct {
	Titulo          string
	Descripcion     string
	Audiencia       []string
	Pantallas       []string
	SeccionesManual []string
	Enlaces         []string
	hasEnlaces      bool
}

// readmeRow is a row of the diagram index of the README.
type readmeRow struct {
	title, file, kind string
	audience          []string
}

// env is what validate checks a diagram against: the real screens of the
// console, the documents of docs/ and the files of the repository.
type env struct {
	screens map[string]bool
	// docAnchor reports whether docs/<doc>.md exists and, if anchor is not
	// empty, has a heading with that GitHub anchor.
	docAnchor func(doc, anchor string) bool
	// sourceExists reports whether a path (or glob) relative to the
	// repository root matches a file.
	sourceExists func(path string) bool
	readme       map[string]readmeRow // by file name
}

// parseFrontMatter reads the header of a diagram. It accepts only the subset
// of YAML the README allows: `clave: "texto"` and `clave: [a, b]`, one per
// line, so any other form is an error rather than a surprise.
func parseFrontMatter(content string) (frontMatter, string, error) {
	var fm frontMatter
	if !strings.HasPrefix(content, "---\n") {
		return fm, "", errors.New("falta el front-matter: el archivo debe empezar con «---»")
	}
	head, body, ok := strings.Cut(content[len("---\n"):], "\n---\n")
	if !ok {
		return fm, "", errors.New("front-matter sin cerrar: falta la línea «---»")
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(head, "\n") {
		m := fieldRe.FindStringSubmatch(line)
		if m == nil {
			return fm, "", fmt.Errorf("front-matter: línea inválida %q (usa «clave: \"texto\"» o «clave: [a, b]»)", line)
		}
		key, raw := m[1], m[2]
		if seen[key] {
			return fm, "", fmt.Errorf("front-matter: %q repetido", key)
		}
		seen[key] = true
		var err error
		switch key {
		case "titulo":
			fm.Titulo, err = parseString(raw)
		case "descripcion":
			fm.Descripcion, err = parseString(raw)
		case "audiencia":
			fm.Audiencia, err = parseList(raw)
		case "pantallas":
			fm.Pantallas, err = parseList(raw)
		case "secciones_manual":
			fm.SeccionesManual, err = parseList(raw)
		case "enlaces":
			fm.Enlaces, err = parseList(raw)
			fm.hasEnlaces = true
		default:
			return fm, "", fmt.Errorf("front-matter: campo desconocido %q", key)
		}
		if err != nil {
			return fm, "", fmt.Errorf("front-matter: %s: %w", key, err)
		}
	}
	for _, k := range []string{"titulo", "descripcion", "audiencia", "pantallas", "secciones_manual"} {
		if !seen[k] {
			return fm, "", fmt.Errorf("front-matter: falta %q", k)
		}
	}
	return fm, body, nil
}

func parseString(raw string) (string, error) {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return "", errors.New("debe ir entre comillas dobles")
	}
	s := raw[1 : len(raw)-1]
	if strings.ContainsAny(s, `"\`) {
		return "", errors.New("sin comillas ni barras invertidas dentro del texto")
	}
	if strings.TrimSpace(s) == "" {
		return "", errors.New("vacío")
	}
	return s, nil
}

func parseList(raw string) ([]string, error) {
	if len(raw) < 2 || raw[0] != '[' || raw[len(raw)-1] != ']' {
		return nil, errors.New("debe ser una lista en una línea: [a, b]")
	}
	inner := strings.TrimSpace(raw[1 : len(raw)-1])
	if inner == "" {
		return []string{}, nil
	}
	var out []string
	for _, item := range strings.Split(inner, ",") {
		item = strings.TrimSpace(item)
		if !listItemRe.MatchString(item) {
			return nil, fmt.Errorf("elemento inválido %q", item)
		}
		if slices.Contains(out, item) {
			return nil, fmt.Errorf("elemento repetido %q", item)
		}
		out = append(out, item)
	}
	return out, nil
}

// validate checks one diagram file and returns every problem found.
func validate(name, content string, e env) []error {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !fileNameRe.MatchString(name) {
		fail("nombre %q: debe ser NN-nombre.md en minúsculas", name)
	}
	fm, body, err := parseFrontMatter(content)
	if err != nil {
		return append(errs, err)
	}

	if h := firstHeading(body); h != fm.Titulo {
		fail("titulo %q no coincide con el encabezado «# %s»", fm.Titulo, h)
	}
	if len(fm.Audiencia) == 0 {
		fail("audiencia vacía")
	}
	for _, a := range fm.Audiencia {
		if !slices.Contains(audiences, a) {
			fail("audiencia %q desconocida (válidas: %s)", a, strings.Join(audiences, ", "))
		}
	}
	for _, p := range fm.Pantallas {
		if !e.screens[p] {
			fail("pantalla %q no existe en la consola (%s)", p, consoleIndex)
		}
	}
	if len(fm.SeccionesManual) == 0 {
		fail("secciones_manual vacía")
	}
	for _, s := range fm.SeccionesManual {
		if !docRefRe.MatchString(s) {
			fail("secciones_manual %q: ruta de docs/ sin .md (p. ej. privacidad)", s)
		} else if !e.docAnchor(s, "") {
			fail("secciones_manual %q: no existe docs/%s.md", s, s)
		}
	}

	blocks := mermaidBlocks(body)
	if len(blocks) != 1 {
		fail("debe tener exactamente un bloque ```mermaid (tiene %d)", len(blocks))
		return errs
	}
	block := blocks[0]
	kind, err := diagramKind(block)
	if err != nil {
		fail("mermaid: %v", err)
	}
	switch kind {
	case "flowchart", "stateDiagram-v2":
		for _, c := range classDefs {
			if !containsLine(block, c) {
				fail("mermaid: falta la clase del README %q", c)
			}
		}
		for _, m := range classUseRe.FindAllStringSubmatch(block, -1) {
			c := m[1] + m[2]
			if !slices.Contains([]string{"externo", "aula", "datos", "seguridad"}, c) {
				fail("mermaid: clase %q fuera de la paleta del README", c)
			}
		}
	case "sequenceDiagram":
		for _, m := range boxRe.FindAllStringSubmatch(block, -1) {
			if !slices.Contains(boxColors, strings.ReplaceAll(m[1], " ", "")) {
				fail("mermaid: color %s fuera de la paleta del README", m[1])
			}
		}
		for _, line := range strings.Split(block, "\n") {
			// GitHub's parser takes ";" as the end of a statement.
			if strings.Contains(line, ";") && !strings.HasPrefix(strings.TrimSpace(line), "%%") {
				fail("mermaid: «;» corta el mensaje en sequenceDiagram: %q", strings.TrimSpace(line))
			}
		}
	}

	if fm.hasEnlaces && kind != "flowchart" {
		fail("enlaces solo se permite en diagramas flowchart")
	}
	for _, l := range fm.Enlaces {
		node, dest, ok := strings.Cut(l, "=")
		if !ok || node == "" || dest == "" {
			fail("enlace %q: usa nodo=destino", l)
			continue
		}
		if !regexp.MustCompile(`(?m)(^|[\s&>-])` + regexp.QuoteMeta(node) + `\s*[\[({]`).MatchString(block) {
			fail("enlace %q: el nodo %q no está en el diagrama", l, node)
		}
		if screen, isScreen := strings.CutPrefix(dest, "pantalla:"); isScreen {
			if !e.screens[screen] {
				fail("enlace %q: la pantalla %q no existe en la consola", l, screen)
			}
			continue
		}
		doc, anchor, _ := strings.Cut(dest, "#")
		if !docRefRe.MatchString(doc) || !e.docAnchor(doc, anchor) {
			fail("enlace %q: no existe docs/%s.md con el ancla %q", l, doc, anchor)
		}
	}

	if !strings.Contains(body, "\n## Lo que no es obvio\n") {
		fail("falta la sección «## Lo que no es obvio»")
	}
	_, sources, ok := strings.Cut(body, "\n## Fuentes en el código\n")
	if !ok {
		fail("falta la sección «## Fuentes en el código»")
	} else {
		n := 0
		for _, m := range codeSpanRe.FindAllStringSubmatch(sources, -1) {
			p := strings.TrimSuffix(m[1], "/")
			if !sourcePath.MatchString(p) {
				continue
			}
			n++
			if !e.sourceExists(p) {
				fail("fuente %q no existe en el repositorio", p)
			}
		}
		if n == 0 {
			fail("«Fuentes en el código» no cita ninguna ruta")
		}
	}

	row, listed := e.readme[name]
	switch {
	case !listed:
		fail("no figura en la tabla del README")
	default:
		if row.title != fm.Titulo {
			fail("el README lo llama %q y el front-matter %q", row.title, fm.Titulo)
		}
		if kind != "" && row.kind != kind {
			fail("el README dice que es %s y el bloque es %s", row.kind, kind)
		}
		if !sameSet(row.audience, fm.Audiencia) {
			fail("audiencia %v distinta de la del README %v", fm.Audiencia, row.audience)
		}
	}
	return errs
}

func firstHeading(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if h, ok := strings.CutPrefix(line, "# "); ok {
			return strings.TrimSpace(h)
		}
	}
	return ""
}

func mermaidBlocks(body string) []string {
	var (
		blocks []string
		cur    []string
		in     bool
	)
	for _, line := range strings.Split(body, "\n") {
		switch {
		case !in && line == "```mermaid":
			in, cur = true, nil
		case in && line == "```":
			in = false
			blocks = append(blocks, strings.Join(cur, "\n"))
		case in:
			cur = append(cur, line)
		}
	}
	return blocks
}

// diagramKind checks the init directive (htmlLabels off, so the pre-rendered
// SVG has no foreignObject) and returns the diagram type.
func diagramKind(block string) (string, error) {
	var lines []string
	for _, l := range strings.Split(block, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, strings.TrimSpace(l))
		}
	}
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "%%{init:") || !strings.Contains(lines[0], `"htmlLabels": false`) {
		return "", errors.New(`la primera línea debe ser %%{init: {"htmlLabels": false…}}%%`)
	}
	kind, _, _ := strings.Cut(lines[1], " ")
	if !slices.Contains(diagramTypes, kind) {
		return "", fmt.Errorf("tipo %q no admitido (usa %s)", kind, strings.Join(diagramTypes, ", "))
	}
	if kind == "flowchart" && !strings.Contains(lines[0], `"flowchart": {"htmlLabels": false}`) {
		return kind, errors.New(`flowchart necesita también "flowchart": {"htmlLabels": false} en init`)
	}
	return kind, nil
}

func containsLine(block, want string) bool {
	for _, l := range strings.Split(block, "\n") {
		if strings.TrimSpace(l) == want {
			return true
		}
	}
	return false
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// githubAnchor is the id GitHub gives a heading: lower case, punctuation
// dropped, spaces turned into hyphens.
func githubAnchor(heading string) string {
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

// anchorsOf returns the GitHub anchors of the headings of a Markdown file,
// with the -1, -2… suffixes of repeated ones.
func anchorsOf(content string) map[string]bool {
	out := map[string]bool{}
	count := map[string]int{}
	fence := false
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			fence = !fence
			continue
		}
		if fence || !strings.HasPrefix(line, "#") {
			continue
		}
		text := strings.TrimSpace(strings.TrimLeft(line, "#"))
		a := githubAnchor(text)
		if n := count[a]; n > 0 {
			out[fmt.Sprintf("%s-%d", a, n)] = true
		} else {
			out[a] = true
		}
		count[a]++
	}
	return out
}

func parseReadme(content string) (map[string]readmeRow, []string, error) {
	rows := map[string]readmeRow{}
	var pending []string
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		m := readmeRowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		var aud []string
		for _, a := range strings.Split(m[4], ",") {
			aud = append(aud, strings.TrimSpace(a))
		}
		link := readmeLink.FindStringSubmatch(m[2])
		if link == nil {
			// A diagram still to write: its row says so and has no link.
			if !strings.Contains(m[2], "(pendiente)") {
				return nil, nil, fmt.Errorf("README: la fila %s no enlaza un diagrama ni dice «(pendiente)»", m[1])
			}
			pending = append(pending, m[1])
			continue
		}
		if !strings.HasPrefix(link[2], m[1]+"-") {
			return nil, nil, fmt.Errorf("README: la fila %s enlaza %s", m[1], link[2])
		}
		rows[link[2]] = readmeRow{title: link[1], file: link[2], kind: m[3], audience: aud}
	}
	if len(rows) == 0 {
		return nil, nil, errors.New("README: no se encontró la tabla de diagramas")
	}
	return rows, pending, nil
}

// realEnv builds the environment from the repository.
func realEnv(t *testing.T) env {
	t.Helper()
	html, err := os.ReadFile(consoleIndex)
	if err != nil {
		t.Fatalf("pantallas de la consola: %v", err)
	}
	screens := map[string]bool{}
	for _, m := range screenRe.FindAllStringSubmatch(string(html), -1) {
		screens[m[1]] = true
	}
	if len(screens) == 0 {
		t.Fatalf("no se encontraron <section id=…> en %s", consoleIndex)
	}
	readme, err := os.ReadFile(filepath.Join(diagramsDir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	rows, _, err := parseReadme(string(readme))
	if err != nil {
		t.Fatal(err)
	}
	return env{
		screens: screens,
		docAnchor: func(doc, anchor string) bool {
			data, err := os.ReadFile(filepath.FromSlash(doc) + ".md")
			if err != nil {
				return false
			}
			return anchor == "" || anchorsOf(string(data))[anchor]
		},
		sourceExists: func(p string) bool {
			matches, err := filepath.Glob(filepath.Join(repoRoot, filepath.FromSlash(p)))
			return err == nil && len(matches) > 0
		},
		readme: rows,
	}
}

func diagramFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(diagramsDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "README.md" {
			names = append(names, e.Name())
		}
	}
	return names
}

func TestDiagrams(t *testing.T) {
	e := realEnv(t)
	names := diagramFiles(t)
	if len(names) == 0 {
		t.Fatal("no hay diagramas en " + diagramsDir)
	}
	numbers := map[string]string{}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if n := name[:2]; numbers[n] != "" {
				t.Errorf("el número %s ya lo usa %s", n, numbers[n])
			} else {
				numbers[n] = name
			}
			data, err := os.ReadFile(filepath.Join(diagramsDir, name))
			if err != nil {
				t.Fatal(err)
			}
			for _, err := range validate(name, string(data), e) {
				t.Error(err)
			}
		})
	}
	for file := range e.readme {
		if !slices.Contains(names, file) {
			t.Errorf("el README enlaza %s, que no existe", file)
		}
	}
}

// TestDiagramIncludes checks that every {{diagrama:NN-nombre}} of the manual
// names an existing diagram. The prompts of docs/v1/prompts describe the
// syntax and are skipped.
func TestDiagramIncludes(t *testing.T) {
	ids := map[string]bool{}
	for _, name := range diagramFiles(t) {
		ids[strings.TrimSuffix(name, ".md")] = true
	}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && filepath.ToSlash(path) == promptsDir {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, id := range includes(string(data)) {
			if !ids[id] {
				t.Errorf("docs/%s: {{diagrama:%s}} no existe en %s", filepath.ToSlash(path), id, diagramsDir)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// includes returns the diagram ids of the {{diagrama:...}} of a Markdown
// file. Those inside code (spans or fenced blocks) only show the syntax.
func includes(content string) []string {
	var (
		ids   []string
		fence bool
	)
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") || strings.HasPrefix(strings.TrimSpace(line), "~~~") {
			fence = !fence
			continue
		}
		if fence {
			continue
		}
		for _, m := range includeRe.FindAllStringSubmatch(codeSpanRe.ReplaceAllString(line, ""), -1) {
			ids = append(ids, m[1])
		}
	}
	return ids
}

func TestIncludesSkipsCode(t *testing.T) {
	md := "Ver {{diagrama:04-autenticacion-tls}}.\n\nLa sintaxis es `{{diagrama:NN-nombre}}`.\n\n```\n{{diagrama:XX}}\n```\n{{diagrama:99-falta}}\n"
	if got, want := includes(md), []string{"04-autenticacion-tls", "99-falta"}; !slices.Equal(got, want) {
		t.Errorf("includes = %v, se esperaba %v", got, want)
	}
}

// TestValidateRejects proves the validator fails on broken diagrams.
func TestValidateRejects(t *testing.T) {
	good := strings.Join([]string{
		"---",
		`titulo: "Prueba"`,
		`descripcion: "Un diagrama de prueba"`,
		"audiencia: [desarrollador]",
		"pantallas: [console]",
		"secciones_manual: [protocol]",
		"enlaces: [a=protocol#seccion, b=pantalla:console]",
		"---",
		"",
		"# Prueba",
		"",
		"```mermaid",
		`%%{init: {"htmlLabels": false, "flowchart": {"htmlLabels": false}}}%%`,
		"flowchart LR",
		`    a["A"] --> b["B"]`,
		"    " + strings.Join(classDefs, "\n    "),
		"    class a,b aula",
		"```",
		"",
		"## Lo que no es obvio",
		"",
		"Nada.",
		"",
		"## Fuentes en el código",
		"",
		"- `internal/protocol/doc.go`",
		"",
	}, "\n")
	e := env{
		screens:      map[string]bool{"console": true},
		docAnchor:    func(doc, anchor string) bool { return doc == "protocol" && (anchor == "" || anchor == "seccion") },
		sourceExists: func(p string) bool { return p == "internal/protocol/doc.go" },
		readme: map[string]readmeRow{"99-prueba.md": {
			title: "Prueba", file: "99-prueba.md", kind: "flowchart", audience: []string{"desarrollador"},
		}},
	}
	if errs := validate("99-prueba.md", good, e); len(errs) > 0 {
		t.Fatalf("el diagrama bueno no pasa: %v", errs)
	}

	cases := []struct {
		name, old, new, want string
	}{
		{"sin front-matter", good[:strings.Index(good, "# Prueba")], "", "falta el front-matter"},
		{"campo faltante", "pantallas: [console]\n", "", `falta "pantallas"`},
		{"campo desconocido", "pantallas:", "autor: \"x\"\npantallas:", "campo desconocido"},
		{"lista multilínea", "audiencia: [desarrollador]", "audiencia:\n  - desarrollador", "línea inválida"},
		{"titulo sin comillas", `titulo: "Prueba"`, "titulo: Prueba", "comillas dobles"},
		{"titulo distinto", "# Prueba", "# Otra", "no coincide"},
		{"audiencia inválida", "audiencia: [desarrollador]", "audiencia: [director]", "desconocida"},
		{"audiencia vacía", "audiencia: [desarrollador]", "audiencia: []", "audiencia vacía"},
		{"pantalla inexistente", "pantallas: [console]", "pantallas: [ajustes]", "no existe en la consola"},
		{"sección con .md", "secciones_manual: [protocol]", "secciones_manual: [protocol.md]", "sin .md"},
		{"sección inexistente", "secciones_manual: [protocol]", "secciones_manual: [administracion/seguridad]", "no existe docs/"},
		{"enlace a nodo ausente", "a=protocol#seccion", "z=protocol#seccion", "no está en el diagrama"},
		{"enlace a ancla ausente", "a=protocol#seccion", "a=protocol#otra", "con el ancla"},
		{"enlace a pantalla ausente", "b=pantalla:console", "b=pantalla:ajustes", "la pantalla"},
		{"sin htmlLabels", `%%{init: {"htmlLabels": false, "flowchart": {"htmlLabels": false}}}%%`, "%%{init: {}}%%", "htmlLabels"},
		{"tipo desconocido", "flowchart LR", "graph LR", "no admitido"},
		{"falta una clase", classDefs[2] + "\n", "", "falta la clase"},
		{"clase ajena", "class a,b aula", "class a,b verde", "fuera de la paleta"},
		{"dos bloques", "## Lo que no es obvio", "```mermaid\nflowchart LR\n```\n\n## Lo que no es obvio", "exactamente un bloque"},
		{"sin prosa", "## Lo que no es obvio", "## Notas", "Lo que no es obvio"},
		{"sin fuentes", "## Fuentes en el código", "## Fuentes", "Fuentes en el código"},
		{"fuente inexistente", "internal/protocol/doc.go", "internal/protocolo/doc.go", "no existe en el repositorio"},
		{"tipo distinto del README", "flowchart LR\n    a[\"A\"] --> b[\"B\"]\n    " + strings.Join(classDefs, "\n    ") + "\n    class a,b aula",
			"stateDiagram-v2\n    a --> b\n    " + strings.Join(classDefs, "\n    "), "el README dice"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(good, c.old) {
				t.Fatalf("el caso no aplica: %q no está en el diagrama bueno", c.old)
			}
			bad := strings.Replace(good, c.old, c.new, 1)
			errs := validate("99-prueba.md", bad, e)
			if !slices.ContainsFunc(errs, func(err error) bool { return strings.Contains(err.Error(), c.want) }) {
				t.Errorf("se esperaba un error con %q, hubo %v", c.want, errs)
			}
		})
	}

	t.Run("nombre inválido", func(t *testing.T) {
		if errs := validate("Prueba.md", good, e); len(errs) == 0 {
			t.Error("se aceptó un nombre sin número")
		}
	})
	t.Run("punto y coma en sequenceDiagram", func(t *testing.T) {
		seq := strings.Replace(good, "flowchart LR\n    a[\"A\"] --> b[\"B\"]", "sequenceDiagram\n    A->>B: uno; dos", 1)
		seq = strings.Replace(seq, "enlaces: [a=protocol#seccion, b=pantalla:console]\n", "", 1)
		e := e
		e.readme = map[string]readmeRow{"99-prueba.md": {title: "Prueba", kind: "sequenceDiagram", audience: []string{"desarrollador"}}}
		errs := validate("99-prueba.md", seq, e)
		if !slices.ContainsFunc(errs, func(err error) bool { return strings.Contains(err.Error(), "«;»") }) {
			t.Errorf("se esperaba el error del punto y coma, hubo %v", errs)
		}
	})
}

func TestGithubAnchor(t *testing.T) {
	for in, want := range map[string]string{
		"10.1 Identidades y claves":             "101-identidades-y-claves",
		"6. Reservados para fases futuras":      "6-reservados-para-fases-futuras",
		"Etapa 09 (Módulo B): notas — en curso": "etapa-09-módulo-b-notas--en-curso",
		"`code.share` (P → A)":                  "codeshare-p--a",
	} {
		if got := githubAnchor(in); got != want {
			t.Errorf("githubAnchor(%q) = %q, se esperaba %q", in, got, want)
		}
	}
}
