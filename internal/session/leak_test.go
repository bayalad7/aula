package session

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

// TestMain fails the package if goroutines outlive its tests: every
// long-lived one must end with its context or its owner's Close (CLAUDE.md,
// Convenciones). The same check lives in transport, session and screen.
func TestMain(m *testing.M) {
	before := runtime.NumGoroutine()
	code := m.Run()
	if code == 0 {
		if err := goroutinesBackTo(before, 5*time.Second); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
		}
	}
	os.Exit(code)
}

func goroutinesBackTo(n int, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for {
		now := runtime.NumGoroutine()
		if now <= n {
			return nil
		}
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<20)
			buf = buf[:runtime.Stack(buf, true)]
			return fmt.Errorf("%d goroutines left after the tests (%d before):\n%s", now, n, buf)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
