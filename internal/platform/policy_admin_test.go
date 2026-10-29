//go:build windows && admintest

package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

// These change the machine (NRPT rule, browser policies): CI job "servicio"
// or a VM only, as administrator, with AULA_ADMIN_TEST=1.
//
//	go test -tags admintest -run Admin ./internal/platform/

func requireAdmin(t *testing.T) {
	if os.Getenv("AULA_ADMIN_TEST") != "1" {
		t.Skip("set AULA_ADMIN_TEST=1 (administrator, CI or VM only)")
	}
}

func nrptRules(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	exe := filepath.Join(windowsDir(), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	out, err := exec.CommandContext(ctx, exe, "-NoProfile", "-Command",
		"Get-DnsClientNrptRule | ForEach-Object { $_.Namespace + '|' + ($_.NameServers -join ',') + '|' + $_.Comment }").CombinedOutput()
	if err != nil {
		t.Fatalf("Get-DnsClientNrptRule: %v %s", err, out)
	}
	return string(out)
}

// The NRPT rule is added, tagged, and removed without leaving anything.
func TestAdminNRPTApplyAndClear(t *testing.T) {
	requireAdmin(t)
	d, _ := NewDNSRedirect()
	before := nrptRules(t)
	t.Cleanup(func() { _ = d.Clear() })
	if err := d.Apply("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if rules := nrptRules(t); !strings.Contains(rules, "127.0.0.1|"+nrptComment) {
		t.Fatalf("rule not found:\n%s", rules)
	}
	// A second Apply (a crash before the journal was written) is also
	// cleared by Clear, which goes by the tag.
	if err := d.Apply("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if err := d.Clear(); err != nil {
		t.Fatal(err)
	}
	if after := nrptRules(t); after != before {
		t.Fatalf("rules left behind:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if err := d.Clear(); err != nil {
		t.Fatalf("second Clear: %v", err)
	}
}

// Disabling DNS over HTTPS and restoring gives back exactly the previous
// values, including "did not exist" and the keys created for them.
func TestAdminBrowserDoHRestoresExactly(t *testing.T) {
	requireAdmin(t)
	b, _ := NewBrowserDoH()
	// A pre-existing value of the school's own policy must survive.
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\Policies\Microsoft\Edge`, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	if err := k.SetStringValue("DnsOverHttpsMode", "automatic"); err != nil {
		t.Fatal(err)
	}
	k.Close()
	t.Cleanup(func() {
		_ = registry.DeleteKey(registry.LOCAL_MACHINE, `SOFTWARE\Policies\Microsoft\Edge`)
	})
	saved, err := b.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Disable(); err != nil {
		t.Fatal(err)
	}
	for _, p := range dohPolicies {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, p.key, registry.QUERY_VALUE)
		if err != nil {
			t.Fatalf("%s not created: %v", p.key, err)
		}
		k.Close()
	}
	if err := b.Restore(saved); err != nil {
		t.Fatal(err)
	}
	again, err := b.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, again) {
		t.Fatalf("not restored exactly:\nbefore %+v\nafter  %+v", saved, again)
	}
	k, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Policies\Microsoft\Edge`, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	if v, _, err := k.GetStringValue("DnsOverHttpsMode"); err != nil || v != "automatic" {
		t.Fatalf("school's own policy changed: %q %v", v, err)
	}
	k.Close()
}
