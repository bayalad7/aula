//go:build windows && admintest

package policy

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// TestAdminWebFilterEndToEnd applies the real web filter (resolver on
// 127.0.0.1:53, NRPT rule, browser policies) and resolves through the
// system's resolver, as any app would. Administrator only: CI job "servicio"
// or a VM, with AULA_ADMIN_TEST=1.
//
//	go test -tags admintest -run Admin ./internal/policy/
func TestAdminWebFilterEndToEnd(t *testing.T) {
	if os.Getenv("AULA_ADMIN_TEST") != "1" {
		t.Skip("set AULA_ADMIN_TEST=1 (administrator, CI or VM only)")
	}
	dnsR, err := platform.NewDNSRedirect()
	if err != nil {
		t.Fatal(err)
	}
	doh, err := platform.NewBrowserDoH()
	if err != nil {
		t.Fatal(err)
	}
	before, err := doh.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	m := &Manager{DNS: dnsR, DoH: doh, Journal: Journal{Path: filepath.Join(t.TempDir(), JournalName)}}
	t.Cleanup(func() { _ = m.Clear("test end"); _ = m.Restore() })

	resolve := func(name string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := net.DefaultResolver.LookupHost(ctx, name)
		return err
	}
	if err := resolve("www.youtube.com"); err != nil {
		t.Skipf("no internet DNS on this runner: %v", err)
	}

	set := protocol.New(protocol.WebFilterSet{PolicyID: protocol.NewID(), Mode: protocol.PolicyDeny, Domains: []string{"youtube.com"}})
	if reply, _ := m.Handle(set); reply.MessageType() != protocol.TypeWebFilterAck {
		t.Fatalf("not applied: %#v", reply)
	}
	if err := resolve("www.youtube.com"); err == nil {
		t.Fatal("youtube.com still resolves with the filter on: the NRPT rule does not reach the resolver")
	}
	if err := resolve("github.com"); err != nil {
		t.Fatalf("an allowed name does not resolve: %v", err)
	}

	clear := protocol.New(protocol.WebFilterClear{})
	if reply, _ := m.Handle(clear); reply.MessageType() != protocol.TypeWebFilterAck {
		t.Fatalf("not cleared: %#v", reply)
	}
	if err := resolve("www.youtube.com"); err != nil {
		t.Fatalf("youtube.com does not resolve after clearing: %v", err)
	}
	after, err := doh.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("browser policies not restored:\nbefore %+v\nafter  %+v", before, after)
	}
	if _, ok, _ := m.Journal.Read(); ok {
		t.Fatal("journal left after clearing")
	}
	// Name resolution alone does not prove the rule is gone: with the rule
	// and no resolver listening, Windows falls back to the adapters' DNS.
	if rules := aulaRules(t); rules != "" {
		t.Fatalf("NRPT rules left after clearing:\n%s", rules)
	}
}

// aulaRules lists the NRPT rules tagged as Aula's ("" if none).
func aulaRules(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	exe := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	out, err := exec.CommandContext(ctx, exe, "-NoProfile", "-NonInteractive", "-Command",
		"Get-DnsClientNrptRule | Where-Object { $_.Comment -eq 'aula-filtro-web' } | ForEach-Object { $_.Name } | Out-String").CombinedOutput()
	if err != nil {
		t.Fatalf("Get-DnsClientNrptRule: %v %s", err, out)
	}
	return strings.TrimSpace(string(out))
}
