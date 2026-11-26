package platform

import (
	"context"
	"errors"
	"testing"
	"time"
)

// The inspector only reads; it runs on any Windows machine. It checks what is
// read, not how fast: on a CI runner busy with every package under -race, WMI
// once took longer than the console's 30 s.
func TestInspectNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("runs PowerShell")
	}
	prev := inspectTimeout
	inspectTimeout = 3 * time.Minute
	t.Cleanup(func() { inspectTimeout = prev })
	n, err := NewNetworkInspector()
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	st, err := n.Inspect(context.Background(), `C:\Program Files\Aula\aula-consola.exe`, 47830)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%.1fs %+v", time.Since(start).Seconds(), st)
	if len(st.Firewall) != 3 {
		t.Fatalf("firewall profiles = %+v, want Domain, Private and Public", st.Firewall)
	}
	for _, net := range st.Networks {
		switch net.Category {
		case NetworkPublic, NetworkPrivate, NetworkDomain:
		default:
			t.Fatalf("network %+v: unknown category", net)
		}
	}
}

// A run past its deadline says so, instead of the bare "exit status 1" that
// Windows gives for a killed process.
func TestInspectNetworkDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("runs PowerShell")
	}
	prev := inspectTimeout
	inspectTimeout = time.Millisecond
	t.Cleanup(func() { inspectTimeout = prev })
	n, _ := NewNetworkInspector()
	_, err := n.Inspect(context.Background(), `C:\Program Files\Aula\aula-consola.exe`, 47830)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}
