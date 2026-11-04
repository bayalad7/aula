package platform

import (
	"context"
	"testing"
	"time"
)

// The inspector only reads; it runs on any Windows machine.
func TestInspectNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("runs PowerShell")
	}
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
