package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
)

// Durations of the two diagnostic phases; shortened in tests.
var (
	diagBrowseFor  = 6 * time.Second
	diagConnectFor = 15 * time.Second
)

// runDiagnose is "aula-cliente diagnostico": it prints the configuration in
// use, the consoles of the channel visible over mDNS and the outcome of one
// connection attempt, so an administrator can tell a network problem from a
// key or channel mismatch. It returns the process exit code.
func runDiagnose(out io.Writer, log *slog.Logger) int {
	fmt.Fprintf(out, "Aula Cliente %s — diagnóstico\n\n", version)
	cfg, pub, err := loadConfig(nil)
	fmt.Fprintf(out, "Configuración: %s\n", usedConfigPath())
	if err != nil {
		fmt.Fprintf(out, "  ERROR: %v\n", err)
		return 1
	}
	fmt.Fprintf(out, "  Canal: %s\n  Clave del centro: %s\n", cfg.Channel, security.KeyFingerprint(pub))
	if cfg.Direct != "" {
		fmt.Fprintf(out, "  Dirección directa: %s (sin mDNS)\n", cfg.Direct)
	}
	if dir, err := platform.LogDir("alumno"); err == nil {
		fmt.Fprintf(out, "  Log: %s\n", dir)
	}

	if cfg.Direct == "" {
		fmt.Fprintf(out, "\n1. Consolas del canal %s visibles por mDNS (%s)…\n", cfg.Channel, diagBrowseFor)
		ctx, cancel := context.WithTimeout(context.Background(), diagBrowseFor)
		found, err := (&discovery.Browser{Channel: cfg.Channel, Logger: log}).Browse(ctx)
		n := 0
		if err != nil {
			fmt.Fprintf(out, "  ERROR: no se pudo usar mDNS: %v\n", err)
		} else {
			for c := range found {
				n++
				fmt.Fprintf(out, "  - «%s» en %s\n", c.Name, strings.Join(c.HostPorts(), ", "))
			}
		}
		cancel()
		if n == 0 {
			fmt.Fprintln(out, "  Ninguna. Revisa que la consola esté abierta en este canal, que la red sea Privada o de Dominio y el firewall.")
		}
	}

	fmt.Fprintf(out, "\n2. Intento de conexión (%s)…\n", diagConnectFor)
	ctx, cancel := context.WithTimeout(context.Background(), diagConnectFor)
	defer cancel()
	connected := false
	last := ""
	agent := &session.Agent{
		Channel: cfg.Channel, ConsolePublicKey: pub, Hello: localHello(), Logger: log,
		Source: (&App{cfg: cfg, log: log}).source(),
		OnState: func(s session.AgentState, c discovery.Console) {
			if s == session.AgentConnected {
				connected = true
				fmt.Fprintf(out, "  Conectado y autenticado con «%s».\n", c.Name)
				cancel()
			}
		},
		OnDiagnostic: func(msg string) {
			if msg != "" && msg != last {
				last = msg
				fmt.Fprintf(out, "  %s\n", msg)
			}
		},
	}
	_ = agent.Run(ctx)
	if !connected {
		fmt.Fprintln(out, "  No se logró conectar.")
		return 1
	}
	fmt.Fprintln(out, "\nTodo bien: si la ventana de Aula Cliente sigue sin conectar, ciérrala y vuelve a abrirla.")
	return 0
}

// usedConfigPath reports which configuration file loadConfig(nil) reads.
func usedConfigPath() string {
	user, _ := userConfigPath()
	for _, p := range []string{machineConfigPath(), user} {
		if _, err := os.Stat(p); err == nil {
			return p
		} else if !errors.Is(err, fs.ErrNotExist) {
			return p + " (no se puede leer: " + err.Error() + ")"
		}
	}
	return "ninguna (" + machineConfigPath() + " no existe)"
}
