package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.design/x/clipboard"
)

// Clipboard is the system clipboard, used by the student app to copy snippets
// natively instead of through the browser API (which needs HTTPS).
type Clipboard interface {
	// WriteText puts text on the clipboard exactly as given, apart from line
	// endings, which are converted to the convention of the OS.
	WriteText(ctx context.Context, text string) error
	// ReadText returns the text on the clipboard.
	ReadText(ctx context.Context) (string, error)
}

// NewClipboard returns the system clipboard. It fails with an error wrapping
// ErrNotSupported when there is none (for example on Linux without an X server
// or Wayland compositor).
func NewClipboard() (Clipboard, error) {
	if err := clipboard.Init(); err != nil {
		if errors.Is(err, clipboard.ErrUnavailable) {
			return nil, fmt.Errorf("clipboard: %w: %w", ErrNotSupported, err)
		}
		return nil, fmt.Errorf("clipboard: %w", err)
	}
	return systemClipboard{}, nil
}

type systemClipboard struct{}

func (systemClipboard) WriteText(ctx context.Context, text string) error {
	if _, err := clipboard.Write(ctx, clipboard.FmtText, []byte(nativeNewlines(text))); err != nil {
		return fmt.Errorf("clipboard write: %w", err)
	}
	return nil
}

func (systemClipboard) ReadText(ctx context.Context) (string, error) {
	b, err := clipboard.Read(ctx, clipboard.FmtText)
	if err != nil {
		return "", fmt.Errorf("clipboard read: %w", err)
	}
	return string(b), nil
}

// toCRLF turns every line ending into \r\n, without doubling existing ones.
func toCRLF(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
}
