package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
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
	printAbilities(out, platform.DisplayServer(), platformCapabilities(log))

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
	}
	printHealth(out)
	if !connected {
		return 1
	}
	fmt.Fprintln(out, "\nTodo bien: si la ventana de Aula Cliente sigue sin conectar, ciérrala y vuelve a abrirla.")
	return 0
}

// platformCapabilities is what this agent declares in this session with
// what the operating system offers (enablePlatform), without connecting.
func platformCapabilities(log *slog.Logger) []protocol.Capability {
	a := NewApp(Config{}, nil, nil, localHelloWithHealth(), nil, nil, log)
	enablePlatform(a, slog.New(slog.DiscardHandler))
	return a.hello.Capabilities
}

// abilities are the functions the diagnosis lists, in the order of «Qué
// puede hacer su equipo»: the ones that depend on the operating system
// (docs/compatibilidad.md). Code, notices, hands, timer and polls work on
// every system and are not listed.
var abilities = []struct {
	cap   protocol.Capability
	label string
}{
	{protocol.CapScreenThumb, "Ver su pantalla en miniatura"},
	{protocol.CapScreenSnapshot, "Captura de su pantalla"},
	{protocol.CapRemote, "Control remoto"},
	{protocol.CapScreenFreeze, "Pausar su pantalla"},
	{protocol.CapScreenPresent, "Proyectar su pantalla a la clase"},
	{protocol.CapFilePush, "Recibir archivos"},
	{protocol.CapFileCollect, "Recoger su trabajo"},
	{protocol.CapOpenURL, "Abrir páginas"},
	{protocol.CapOpenApp, "Abrir apps permitidas"},
	{protocol.CapPower, "Apagar, reiniciar y cerrar sesión"},
	{protocol.CapAppBlock, "Bloquear apps"},
	{protocol.CapWebFilter, "Filtro web"},
}

// printAbilities says what this computer can do and, on Linux, why the
// screen is missing (docs/compatibilidad.md). The transmission of the
// teacher's screen depends on the window and is not listed here.
func printAbilities(out io.Writer, display string, caps []protocol.Capability) {
	fmt.Fprintln(out, "\nQué puede hacer Aula en este equipo (en esta sesión):")
	switch display {
	case platform.DisplayX11:
		fmt.Fprintln(out, "  Sesión gráfica: X11.")
	case platform.DisplayWayland:
		fmt.Fprintln(out, "  Sesión gráfica: Wayland. Ver la pantalla y capturarla necesitan una sesión X11:")
		fmt.Fprintln(out, "  cierra la sesión y elige «Ubuntu en Xorg» (o desactiva Wayland en el equipo).")
	case "":
		fmt.Fprintln(out, "  Sin sesión gráfica: ejecuta el diagnóstico desde la sesión del alumno para ver su pantalla.")
	}
	var missing []string
	for _, a := range abilities {
		if slices.Contains(caps, a.cap) {
			fmt.Fprintf(out, "  ✓ %s\n", a.label)
		} else {
			missing = append(missing, a.label)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(out, "  No disponible en este sistema o sesión: %s.\n", strings.Join(missing, ", "))
	}
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
