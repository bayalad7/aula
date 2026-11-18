//go:build linux

package platform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
)

// The agent as a systemd service on Linux (D-024): a system unit that runs
// as root, starts with the computer and restarts on failure; the helper runs
// in the student's graphical session (session_linux.go).

// unitDir is where the units are written; replaced in tests.
var unitDir = "/etc/systemd/system"

// runCommand runs a system tool and returns its standard output; the error
// carries its standard error. Replaced in tests.
var runCommand = func(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if strings.Contains(msg, "Access denied") || strings.Contains(msg, "authentication required") {
			return out.String(), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), fs.ErrPermission, msg)
		}
		return out.String(), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, msg)
	}
	return out.String(), nil
}

// unitName turns a service name into a systemd unit: "AulaAgente" is
// "aula-agente.service", "AulaAgentePrueba" "aula-agente-prueba.service".
func unitName(name string) string {
	var b strings.Builder
	for i, r := range name {
		switch {
		case unicode.IsUpper(r):
			if i > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String() + ".service"
}

func newServiceControl(name string) ServiceControl { return systemdService{unit: unitName(name)} }

type systemdService struct{ unit string }

func (s systemdService) path() string { return filepath.Join(unitDir, s.unit) }

// needRoot fails like Windows does without an administrator.
func needRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("systemd: %w (run with sudo)", fs.ErrPermission)
	}
	return nil
}

// unitFile is the unit of cfg. The service runs as root, like LocalSystem on
// Windows, and is restarted 5 s after a failure; stopping it ends the helper
// too (it lives in the unit's cgroup).
func unitFile(cfg ServiceConfig) string {
	exec := []string{systemdQuote(cfg.Exe)}
	for _, a := range cfg.Args {
		exec = append(exec, systemdQuote(a))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n# Escrito por \"aula-cliente servicio instalar\"; se quita con \"servicio desinstalar\".\n", oneLine(cfg.Description))
	b.WriteString("[Unit]\n")
	fmt.Fprintf(&b, "Description=%s\n", oneLine(cfg.DisplayName))
	b.WriteString("After=network-online.target systemd-logind.service\nWants=network-online.target\n\n")
	b.WriteString("[Service]\nType=simple\n")
	fmt.Fprintf(&b, "ExecStart=%s\n", strings.Join(exec, " "))
	b.WriteString("Restart=on-failure\nRestartSec=5\nKillMode=control-group\nTimeoutStopSec=30\n\n")
	b.WriteString("[Install]\nWantedBy=multi-user.target\n")
	return b.String()
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// systemdQuote quotes one ExecStart word: double quotes, with \, " and the
// specifiers' % escaped.
func systemdQuote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%", "\n", " ")
	return `"` + r.Replace(s) + `"`
}

func (s systemdService) Install(cfg ServiceConfig) error {
	if err := needRoot(); err != nil {
		return err
	}
	if !filepath.IsAbs(cfg.Exe) {
		return fmt.Errorf("systemd: %s is not absolute", cfg.Exe)
	}
	if err := os.WriteFile(s.path(), []byte(unitFile(cfg)), 0o644); err != nil {
		return fmt.Errorf("systemd: %w", err)
	}
	ctx := context.Background()
	if _, err := runCommand(ctx, "systemctl", "daemon-reload"); err != nil {
		return err
	}
	_, err := runCommand(ctx, "systemctl", "enable", s.unit)
	return err
}

func (s systemdService) Uninstall() error {
	if err := needRoot(); err != nil {
		return err
	}
	ctx := context.Background()
	if _, err := os.Stat(s.path()); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	_, _ = runCommand(ctx, "systemctl", "disable", s.unit)
	if err := os.Remove(s.path()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("systemd: %w", err)
	}
	_, _ = runCommand(ctx, "systemctl", "reset-failed", s.unit)
	_, err := runCommand(ctx, "systemctl", "daemon-reload")
	return err
}

func (s systemdService) Start() error {
	if err := needRoot(); err != nil {
		return err
	}
	_, err := runCommand(context.Background(), "systemctl", "start", s.unit)
	return err
}

func (s systemdService) Stop(ctx context.Context) error {
	if err := needRoot(); err != nil {
		return err
	}
	_, err := runCommand(ctx, "systemctl", "stop", s.unit)
	return err
}

func (s systemdService) Status() (ServiceStatus, error) {
	out, err := runCommand(context.Background(), "systemctl", "show", s.unit,
		"-p", "LoadState", "-p", "ActiveState", "-p", "MainPID", "-p", "UnitFileState", "-p", "Restart")
	if err != nil {
		return ServiceStatus{}, err
	}
	return parseUnitStatus(out), nil
}

// parseUnitStatus reads "systemctl show" (Key=value lines).
func parseUnitStatus(out string) ServiceStatus {
	p := keyValues(out)
	if p["LoadState"] == "not-found" || p["LoadState"] == "" {
		return ServiceStatus{State: ServiceNotInstalled}
	}
	st := ServiceStatus{Auto: p["UnitFileState"] == "enabled"}
	switch p["Restart"] {
	case "on-failure", "always", "on-abnormal", "on-abort":
		st.RestartOnFailure = true
	}
	switch p["ActiveState"] {
	case "active", "reloading":
		st.State = ServiceRunning
		st.PID, _ = strconv.Atoi(p["MainPID"])
	case "activating":
		st.State = ServiceStartPending
	case "deactivating":
		st.State = ServiceStopPending
	case "inactive", "failed":
		st.State = ServiceStopped
	default:
		st.State = ServiceOther
	}
	return st
}

// keyValues reads Key=value lines; a later key wins.
func keyValues(out string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			m[k] = v
		}
	}
	return m
}

// isService: systemd sets INVOCATION_ID for every unit it starts, and a
// system service's parent is PID 1.
func isService() (bool, error) {
	return os.Getenv("INVOCATION_ID") != "" && os.Getppid() == 1, nil
}

// sessionPoll is how often runAsService looks at logind; replaced in tests.
var sessionPoll = 2 * time.Second

// runAsService runs run until systemd stops the unit (SIGTERM). Logins,
// logouts and a switch of the active session are seen by polling logind
// every 2 s: cheap, and with no D-Bus dependency.
func runAsService(_ string, run func(ctx context.Context, sessionChanged <-chan struct{}) error) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	changed := make(chan struct{}, 1)
	go watchSessions(ctx, changed)
	return run(ctx, changed)
}

// watchSessions signals changed whenever the active session of seat0 or its
// user changes.
func watchSessions(ctx context.Context, changed chan<- struct{}) {
	t := time.NewTicker(sessionPoll)
	defer t.Stop()
	last := sessionSignature(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if sig := sessionSignature(ctx); sig != last {
			last = sig
			select {
			case changed <- struct{}{}:
			default: // one pending signal is enough
			}
		}
	}
}

func sessionSignature(ctx context.Context) string {
	s, ok, err := activeSession(ctx)
	if err != nil || !ok {
		return ""
	}
	return fmt.Sprint(s.ID, "/", s.SID)
}
