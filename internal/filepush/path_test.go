package filepush

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckNameRejectsDangerousNames(t *testing.T) {
	for _, name := range []string{
		"", ".", "..", "../x.txt", `..\x.txt`, "a/b.txt", `a\b.txt`,
		`C:\Windows\system32\x.dll`, "C:x.txt", `\\servidor\recurso\x`, "/etc/passwd",
		"datos.csv:oculto", "x\x00.txt", "x\n.txt", "x\x7f.txt",
		"tarea\u202etxt.exe", "a\u200bb.txt", "a\ufeffb.txt",
		"punto.", "espacio ", "a<b", "a>b", `a"b`, "a|b", "a?b", "a*b",
		"CON", "con", "nul.txt", "NUL.tar.gz", "Com1.py", "lpt9", "AUX ", "aux .txt", "COM¹",
		strings.Repeat("a", 101), "\xff\xfe.txt",
	} {
		if err := CheckName(name); !errors.Is(err, ErrInvalidPath) {
			t.Errorf("CheckName(%q) = %v, want ErrInvalidPath", name, err)
		}
	}
}

func TestCheckNameAcceptsOrdinaryNames(t *testing.T) {
	for _, name := range []string{
		"datos.csv", "main.py", ".gitignore", "__init__.py", "Ejercicio 4 – plantilla.docx",
		"año.txt", "consola", "CONSOLE.txt", "com10.py", "lpt.txt", "a..b", "setup.exe", "run.bat",
		strings.Repeat("ñ", 100),
	} {
		if err := CheckName(name); err != nil {
			t.Errorf("CheckName(%q) = %v", name, err)
		}
	}
}

func TestCheckDest(t *testing.T) {
	for _, dest := range []string{"", "Ejercicio 4", "Ejercicio 4/datos", "a/b/c/d", strings.Repeat("d", 100)} {
		if err := CheckDest(dest); err != nil {
			t.Errorf("CheckDest(%q) = %v", dest, err)
		}
	}
	for _, dest := range []string{
		"/", "/abs", "abs/", "a//b", "..", "a/..", "../..", "a/../../b", "./a", `a\b`, "C:", "C:/x",
		`\\srv\share`, "a/b/c/d/e", "a/con", "a/b.", strings.Repeat("d/", 100) + "d", strings.Repeat("d", 201),
	} {
		if err := CheckDest(dest); !errors.Is(err, ErrInvalidPath) {
			t.Errorf("CheckDest(%q) = %v, want ErrInvalidPath", dest, err)
		}
	}
}

func TestCleanDest(t *testing.T) {
	for in, want := range map[string]string{
		"":                       "",
		"  Ejercicio 4 ":         "Ejercicio 4",
		`\Ejercicio 4\datos\`:    "Ejercicio 4/datos",
		"/a / b /":               "a/b",
		"../fuera":               "../fuera", // still refused by CheckDest
		`C:\Windows`:             "C:/Windows",
		"Ejercicio 4//datos":     "Ejercicio 4//datos",
		" Unidad 2 / Práctica 1": "Unidad 2/Práctica 1",
	} {
		if got := CleanDest(in); got != want {
			t.Errorf("CleanDest(%q) = %q, want %q", in, got, want)
		}
	}
	for _, in := range []string{"../fuera", `C:\Windows`, "a//b", `\\srv\x`} {
		if CheckDest(CleanDest(in)) == nil {
			t.Errorf("CleanDest(%q) must not make it valid", in)
		}
	}
}

func TestAlternative(t *testing.T) {
	for _, c := range []struct {
		name string
		n    int
		want string
	}{
		{"datos.csv", 1, "datos.csv"},
		{"datos.csv", 2, "datos (2).csv"},
		{"datos.tar.gz", 3, "datos.tar (3).gz"},
		{"Makefile", 2, "Makefile (2)"},
		{".gitignore", 2, ".gitignore (2)"},
		{"datos.csv", 99, "datos (99).csv"},
	} {
		if got := Alternative(c.name, c.n); got != c.want {
			t.Errorf("Alternative(%q, %d) = %q, want %q", c.name, c.n, got, c.want)
		}
		if err := CheckName(Alternative(c.name, c.n)); err != nil {
			t.Errorf("alternative of %q is not a valid name: %v", c.name, err)
		}
	}
}

func TestCheckRel(t *testing.T) {
	for _, rel := range []string{"x.txt", "Ejercicio 4/x.txt", "a/b/c/d/x.txt", "Ejercicio 4/datos (2).csv"} {
		if err := CheckRel(rel); err != nil {
			t.Errorf("CheckRel(%q) = %v", rel, err)
		}
	}
	for _, rel := range []string{"", "/x.txt", "a/../x.txt", "../x.txt", "a//x.txt", "a/", `a\x.txt`, "a/b/c/d/e/x.txt", "C:/x.txt"} {
		if err := CheckRel(rel); !errors.Is(err, ErrInvalidPath) {
			t.Errorf("CheckRel(%q) = %v, want ErrInvalidPath", rel, err)
		}
	}
}
