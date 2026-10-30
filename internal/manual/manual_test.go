package manual

import (
	"strings"
	"testing"
	"testing/fstest"
)

const page = `---
titulo: "Conceptos"
resumen: "Las palabras de Aula, sin jerga."
grupo: "Conceptos"
orden: 2
pantallas: [alumnos]
---

# Conceptos

Un párrafo con **negritas**, *cursiva*, ` + "`código`" + ` y un [enlace](otra#sección-dos).
Sigue el mismo párrafo.

## El canal

- Uno
- Dos con [vista](pantalla:alumnos)
  y continuación

1. Primero
2. Segundo

> **Nota:** importante.

| A | B |
|---|---|
| uno | [diagrama](diagrama:12-x) |

![Lista de alumnos](img/alumnos.png)

{{diagrama:12-x}}

` + "```python\nprint(1)\n```" + `

## El canal

<script>alert(1)</script>
`

const other = `---
titulo: "Otra"
grupo: "Guías"
orden: 1
pantallas: [codigo]
---

## Sección dos

Texto con firewall.
`

const diagram = "---\ntitulo: \"Diagnóstico\"\ndescripcion: \"Qué revisar cuando un alumno no aparece\"\naudiencia: [profesor]\n---\n\n# Diagnóstico\n\n```mermaid\n" +
	"flowchart TD\n    a[\"El alumno no aparece\"]\n    q{\"¿Qué dice la barra?\"}\n    s1[\"Sin configurar\"]\n    s2[\"Buscando\"]\n    fix[\"Reinstalar\"]\n    ok[\"Aparece\"]\n" +
	"    a --> q\n    q -- \"sin configurar\" --> s1 --> fix\n    q -- \"buscando\" --> s2\n    fix & s2 --> ok\n```\n"

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"man/conceptos.md":    {Data: []byte(page)},
		"man/otra.md":         {Data: []byte(other)},
		"man/README.md":       {Data: []byte("ignored")},
		"man/img/alumnos.png": {Data: []byte{0x89, 'P', 'N', 'G'}},
		"diag/12-x.md":        {Data: []byte(diagram)},
	}
}

func TestParsePage(t *testing.T) {
	m, err := Load(testFS(), "man", "diag")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Pages) != 2 || m.Pages[0].Slug != "otra" {
		t.Fatalf("pages in index order: %+v", m.Pages)
	}
	p, _ := m.Page("conceptos")
	if p.Title != "Conceptos" || p.Group != "Conceptos" || p.Order != 2 || len(p.Screens) != 1 {
		t.Errorf("front-matter = %+v", p)
	}
	kinds := []string{}
	for _, b := range p.Blocks {
		kinds = append(kinds, b.Kind)
	}
	want := "paragraph heading list list note table image diagram code heading paragraph"
	if got := strings.Join(kinds, " "); got != want {
		t.Errorf("blocks = %s\nwant     %s", got, want)
	}
	para := p.Blocks[0].Text
	styles := []string{}
	for _, in := range para {
		styles = append(styles, in.Kind)
	}
	if got := strings.Join(styles, " "); got != "text strong text em text code text link text" {
		t.Errorf("inlines = %s (%+v)", got, para)
	}
	if !strings.Contains(plain(para), "Sigue el mismo párrafo") {
		t.Error("lines of a paragraph are not joined")
	}
	if items := p.Blocks[2].Items; len(items) != 2 || !strings.Contains(plain(items[1]), "y continuación") {
		t.Errorf("list items = %+v", items)
	}
	if !p.Blocks[3].Ordered {
		t.Error("numbered list not ordered")
	}
	if p.Headings[0].Anchor != "el-canal" || p.Headings[1].Anchor != "el-canal-1" {
		t.Errorf("anchors = %s, %s", p.Headings[0].Anchor, p.Headings[1].Anchor)
	}
	// Raw HTML is plain text: nothing becomes markup.
	if last := p.Blocks[len(p.Blocks)-1]; last.Kind != "paragraph" || plain(last.Text) != "<script>alert(1)</script>" {
		t.Errorf("raw HTML = %+v", last)
	}
}

func TestValidate(t *testing.T) {
	m, err := Load(testFS(), "man", "diag")
	if err != nil {
		t.Fatal(err)
	}
	if errs := m.Validate([]string{"alumnos", "codigo"}); len(errs) != 0 {
		t.Errorf("valid manual: %v", errs)
	}
	errs := m.Validate([]string{"alumnos", "codigo", "ayuda"})
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), `"ayuda" has no help page`) {
		t.Errorf("missing help page: %v", errs)
	}
	bad := testFS()
	bad["man/otra.md"] = &fstest.MapFile{Data: []byte(strings.Replace(other, "Texto con firewall.",
		"[roto](conceptos#no-existe) [vista](pantalla:nada)\n\n![](img/falta.png)\n\n{{diagrama:99-no}}", 1))}
	m2, _ := Load(bad, "man", "diag")
	errs = m2.Validate([]string{"alumnos", "codigo"})
	joined := ""
	for _, e := range errs {
		joined += e.Error() + "\n"
	}
	for _, want := range []string{"conceptos#no-existe", "pantalla:nada", "img/falta.png does not exist", "needs a description", "99-no"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Validate did not report %q:\n%s", want, joined)
		}
	}
}

func TestSearch(t *testing.T) {
	m, _ := Load(testFS(), "man", "diag")
	r := m.Search("FIREWALL")
	if len(r) != 1 || r[0].Slug != "otra" || r[0].Anchor != "sección-dos" {
		t.Errorf("search firewall = %+v", r)
	}
	if r := m.Search("parrafo negritas"); len(r) == 0 || r[0].Slug != "conceptos" {
		t.Errorf("accents and several words: %+v", r)
	}
	if r := m.Search("diagnostico"); len(r) != 1 || r[0].Diagram != "12-x" {
		t.Errorf("diagram titles are searchable: %+v", r)
	}
	if r := m.Search("   "); len(r) != 0 {
		t.Errorf("empty query = %+v", r)
	}
}

func TestParseTree(t *testing.T) {
	m, _ := Load(testFS(), "man", "diag")
	tr, err := ParseTree(m.Diagrams["12-x"].Mermaid)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Start != "a" || tr.Nodes["a"].Next != "q" {
		t.Fatalf("start = %+v", tr)
	}
	q := tr.Nodes["q"]
	if !q.Question || len(q.Answers) != 2 || q.Answers[0].Label != "sin configurar" || q.Answers[0].To != "s1" {
		t.Errorf("question = %+v", q)
	}
	if tr.Nodes["s1"].Next != "fix" || tr.Nodes["fix"].Next != "ok" || tr.Nodes["s2"].Next != "ok" {
		t.Errorf("chains and & = %+v", tr.Nodes)
	}
	for _, bad := range []string{
		"flowchart TD\n a[\"x\"]\n q{\"?\"}\n a --> q\n q --> a",           // answer without label
		"flowchart TD\n a[\"x\"]\n b[\"y\"]",                               // two starts
		"flowchart TD\n a[\"x\"]\n q{\"?\"}\n a --> q\n q -- \"si\" --> a", // one answer only
	} {
		if _, err := ParseTree(bad); err == nil {
			t.Errorf("ParseTree accepted %q", bad)
		}
	}
}
