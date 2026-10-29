//go:build windows

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// A process of this session can be listed with its name and path, and
// closed; closing one that already exited is not an error.
func TestProcessControlListsAndKills(t *testing.T) {
	pc, err := NewProcessControl()
	if err != nil {
		t.Fatal(err)
	}
	var session uint32
	if err := windows.ProcessIdToSessionId(uint32(os.Getpid()), &session); err != nil {
		t.Fatal(err)
	}
	// A harmless child that waits: ping to loopback.
	child := exec.Command(filepath.Join(windowsDir(), "System32", "PING.EXE"), "-n", "60", "127.0.0.1")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer child.Process.Kill() //nolint:errcheck // already killed below
	procs, err := pc.List(session)
	if err != nil {
		t.Fatal(err)
	}
	var self, ping bool
	for _, p := range procs {
		if p.PID == os.Getpid() && p.Path != "" {
			self = true
		}
		if p.PID == child.Process.Pid && strings.EqualFold(p.Name, "PING.EXE") && strings.HasSuffix(strings.ToLower(p.Path), `\system32\ping.exe`) {
			ping = true
		}
	}
	if !self || !ping {
		t.Fatalf("listed self=%v ping=%v among %d processes", self, ping, len(procs))
	}
	if err := pc.Kill(child.Process.Pid); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("killed process still running")
	}
	if err := pc.Kill(child.Process.Pid); err != nil {
		t.Fatalf("kill of an exited process: %v", err)
	}
	// Other sessions are not listed.
	if other, err := pc.List(session + 1000); err != nil || len(other) != 0 {
		t.Fatalf("session %d: %d processes, %v", session+1000, len(other), err)
	}
}

// Reading the DNS configuration and the browser policies, and flushing the
// cache, need no administrator.
func TestDNSAndDoHReadOnly(t *testing.T) {
	d, err := NewDNSRedirect()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := d.Upstreams()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range cfg.Servers {
		if s == "127.0.0.1" || s == "::1" || strings.HasPrefix(s, "fec0:") {
			t.Errorf("upstream %s should have been skipped", s)
		}
	}
	t.Logf("DNS servers %v, suffixes %v", cfg.Servers, cfg.Suffixes)
	if err := d.Flush(); err != nil {
		t.Fatal(err)
	}
	b, err := NewBrowserDoH()
	if err != nil {
		t.Fatal(err)
	}
	saved, err := b.Snapshot()
	if err != nil || len(saved) != len(dohPolicies) {
		t.Fatalf("snapshot %+v, %v", saved, err)
	}
}
