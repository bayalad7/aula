//go:build windows && admintest

package policy

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// resolveAsStudent resolves through the system's resolver, as any app would.
func resolveAsStudent(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := net.DefaultResolver.LookupHost(ctx, name)
	return err
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
