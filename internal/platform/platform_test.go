package platform

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrNotSupportedWrapping(t *testing.T) {
	err := fmt.Errorf("capture screen: %w", ErrNotSupported)
	if !errors.Is(err, ErrNotSupported) {
		t.Fatalf("errors.Is(%v, ErrNotSupported) = false", err)
	}
}
