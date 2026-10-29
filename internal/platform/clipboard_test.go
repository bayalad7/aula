package platform

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// pythonSnippet has what usually breaks on copy: 4-space and 8-space
// indentation, a tab, a blank line, trailing spaces and a final newline.
const pythonSnippet = "def factorial(n):\n" +
	"    if n <= 1:\n" +
	"        return 1\n" +
	"\treturn n * factorial(n - 1)\n" +
	"\n" +
	"print(factorial(5))  \n"

// TestClipboardKeepsIndentationAndNewlines copies through the real system
// clipboard. It is skipped where there is none (CI without a display); set
// AULA_REQUIRE_CLIPBOARD=1 to make that a failure.
func TestClipboardKeepsIndentationAndNewlines(t *testing.T) {
	cb, err := NewClipboard()
	if err != nil {
		if errors.Is(err, ErrNotSupported) && os.Getenv("AULA_REQUIRE_CLIPBOARD") != "1" {
			t.Skipf("no system clipboard here: %v", err)
		}
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Leave the developer's clipboard as it was.
	if prev, err := cb.ReadText(ctx); err == nil {
		defer func() { _ = cb.WriteText(context.Background(), strings.ReplaceAll(prev, "\r\n", "\n")) }()
	}

	if err := cb.WriteText(ctx, pythonSnippet); err != nil {
		if os.Getenv("AULA_REQUIRE_CLIPBOARD") != "1" {
			t.Skipf("clipboard not writable here: %v", err)
		}
		t.Fatal(err)
	}
	got, err := cb.ReadText(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		if strings.Count(got, "\r\n") != strings.Count(pythonSnippet, "\n") {
			t.Fatalf("clipboard text is not CRLF: %q", got)
		}
		got = strings.ReplaceAll(got, "\r\n", "\n")
	}
	if got != pythonSnippet {
		t.Fatalf("clipboard altered the snippet:\n got %q\nwant %q", got, pythonSnippet)
	}
}

func TestToCRLF(t *testing.T) {
	for in, want := range map[string]string{
		"a\nb":       "a\r\nb",
		"a\r\nb":     "a\r\nb",
		"a\n\nb\n":   "a\r\n\r\nb\r\n",
		"  x\n\ty\r": "  x\r\n\ty\r",
		"":           "",
	} {
		if got := toCRLF(in); got != want {
			t.Errorf("toCRLF(%q) = %q, want %q", in, got, want)
		}
	}
}
