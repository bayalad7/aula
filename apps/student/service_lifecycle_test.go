//go:build windows && servicetest

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
)

// testServiceName is never the real one: the lifecycle test must not touch a
// real installation.
const testServiceName = "AulaAgentePrueba"

// TestServiceLifecycle installs a real service, so it only runs in CI (job
// "servicio") or in a VM, as an administrator, with AULA_SERVICE_TEST=1:
//
//	go test -tags servicetest -run TestServiceLifecycle ./apps/student/
//
// install → status (automatic, restart on failure) → start → kill the
// process → the service manager starts it again → stop → uninstall. The
// cleanup uninstalls it even if the test fails; the CI job also deletes it
// with sc.exe in a step that always runs.
func TestServiceLifecycle(t *testing.T) {
	if os.Getenv("AULA_SERVICE_TEST") != "1" {
		t.Skip("set AULA_SERVICE_TEST=1 (administrator, CI or VM only)")
	}
	exe := filepath.Join(t.TempDir(), "aula-cliente.exe")
	build := exec.Command("go", "build", "-o", exe, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	cli := func(args ...string) (int, string) {
		t.Helper()
		cmd := exec.Command(exe, append(append([]string{"servicio"}, args...), "-nombre", testServiceName)...)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return code, out.String()
	}
	ctl := platform.NewServiceControl(testServiceName)
	t.Cleanup(func() {
		if code, out := cli("desinstalar"); code != 0 {
			t.Errorf("cleanup: %s", out)
		}
	})
	waitFor := func(what string, d time.Duration, cond func(platform.ServiceStatus) bool) platform.ServiceStatus {
		t.Helper()
		deadline := time.Now().Add(d)
		for {
			st, err := ctl.Status()
			if err == nil && cond(st) {
				return st
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s: %+v, %v", what, st, err)
			}
			time.Sleep(250 * time.Millisecond)
		}
	}

	if code, out := cli("instalar"); code != 0 {
		t.Fatalf("instalar: %s", out)
	}
	st := waitFor("installed", 5*time.Second, func(s platform.ServiceStatus) bool { return s.State == platform.ServiceStopped })
	// Starting with the computer cannot be tested without a reboot: check the
	// configuration the service manager uses at boot.
	if !st.Auto || !st.RestartOnFailure {
		t.Fatalf("installed with %+v, want automatic start and restart on failure", st)
	}
	// Installing again (an upgrade) keeps it working.
	if code, out := cli("instalar"); code != 0 {
		t.Fatalf("instalar again: %s", out)
	}

	if code, out := cli("iniciar"); code != 0 {
		t.Fatalf("iniciar: %s", out)
	}
	first := waitFor("running", 20*time.Second, func(s platform.ServiceStatus) bool {
		return s.State == platform.ServiceRunning && s.PID != 0
	})
	if code, out := cli("estado"); code != exitRunning || !strings.Contains(out, "en ejecución") {
		t.Fatalf("estado: %d %s", code, out)
	}

	// A crash: the service manager restarts it (first delay 5 s).
	p, err := os.FindProcess(first.PID)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Kill(); err != nil {
		t.Fatalf("kill %d: %v", first.PID, err)
	}
	second := waitFor("restart after the crash", 45*time.Second, func(s platform.ServiceStatus) bool {
		return s.State == platform.ServiceRunning && s.PID != 0 && s.PID != first.PID
	})
	t.Logf("killed PID %d; the service manager restarted it as PID %d", first.PID, second.PID)

	// It ran as the service: its log is in ProgramData, not in a profile.
	logPath := filepath.Join(platform.AgentServiceLogDir(), "aula-agente.log")
	if data, err := os.ReadFile(logPath); err != nil || !strings.Contains(string(data), "aula-cliente service starting") {
		t.Errorf("service log %s: %v", logPath, err)
	}

	if code, out := cli("detener"); code != 0 {
		t.Fatalf("detener: %s", out)
	}
	waitFor("stopped", 30*time.Second, func(s platform.ServiceStatus) bool { return s.State == platform.ServiceStopped })
	if code, _ := cli("estado"); code != exitStopped {
		t.Fatalf("estado when stopped: %d", code)
	}

	if code, out := cli("desinstalar"); code != 0 {
		t.Fatalf("desinstalar: %s", out)
	}
	waitFor("uninstalled", 30*time.Second, func(s platform.ServiceStatus) bool { return s.State == platform.ServiceNotInstalled })
}
