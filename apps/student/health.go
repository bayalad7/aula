package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/bayalad7/aula/internal/health"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/transport"
)

// The health of this computer (docs/protocol.md §5.27, docs/privacidad.md
// rules 92–95): a few data of the computer, never of the student. It only
// reads; the window shows nothing but its line in «Qué puede hacer Aula en
// este equipo». As a service, the service reads it itself, also without a
// session: no helper is involved.

// healthCapabilities are the capabilities of the health of this computer:
// health everywhere (what the OS cannot read does not travel) and the
// self-test where it runs (Windows).
func healthCapabilities() []protocol.Capability {
	caps := []protocol.Capability{protocol.CapHealth}
	if selfTestSupported() {
		caps = append(caps, protocol.CapHealthSelfTest)
	}
	return caps
}

func selfTestSupported() bool { return runtime.GOOS == "windows" }

// newHealth builds the health agent of this App. documents gives the
// Documents folder of the session's user ("" without one).
func (a *App) newHealth(sys platform.SystemHealth, documents func() string) *health.Agent {
	return &health.Agent{
		Monitor: &health.Monitor{
			Sys: sys, Documents: documents, LocalIP: a.consoleLocalIP, Logger: a.log,
		},
		SelfTest: &health.SelfTest{Sys: sys, TempDir: os.TempDir(), Supported: selfTestSupported()},
		Logger:   a.log,
	}
}

// windowDocuments is the Documents folder of the user running the window.
func windowDocuments() string {
	dir, err := platform.DocumentsDir()
	if err != nil {
		return ""
	}
	return dir
}

// consoleLocalIP is this computer's address towards the console of the
// session, nil while not connected.
func (a *App) consoleLocalIP() net.IP {
	cl := a.connection()
	if cl == nil {
		return nil
	}
	host, _, err := net.SplitHostPort(cl.RemoteAddr())
	if err != nil {
		return nil
	}
	return health.LocalIPTowards(host)
}

// healthConnected attaches the report to a new session.
func (a *App) healthConnected(cl *transport.Client) {
	if a.health == nil {
		return
	}
	a.health.Monitor.Start(func(ctx context.Context, p protocol.Payload) error {
		_, err := cl.Send(ctx, p)
		return err
	})
}

// healthDisconnected detaches it: the destinations were only for the
// session (§5.27.1).
func (a *App) healthDisconnected() {
	if a.health != nil {
		a.health.Monitor.Stop()
	}
}

// handleHealth answers health.config, health.request and health.selftest.
func (a *App) handleHealth(ctx context.Context, cl *transport.Client, env protocol.Envelope) bool {
	if a.health == nil {
		return false
	}
	return a.health.Handle(ctx, func(ctx context.Context, p protocol.Payload) error {
		_, err := cl.Send(ctx, p)
		return err
	}, env)
}

// healthSampleGap separates the two processor samples of the diagnostic.
var healthSampleGap = time.Second

// printHealth is section 3 of "aula-cliente diagnostico": what this
// computer would report to the console (§5.27.3) and its self-test without
// destinations, for the administrator at the computer.
func printHealth(out io.Writer) {
	fmt.Fprintln(out, "\n3. Salud del equipo (lo que Aula informa a la consola)…")
	sys := platform.NewSystemHealth()
	m := &health.Monitor{Sys: sys, Documents: windowDocuments, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	m.Tick(context.Background())
	time.Sleep(healthSampleGap)
	m.Tick(context.Background())
	r := m.Read()
	gb := func(b int64) string { return fmt.Sprintf("%.1f GB", float64(b)/(1<<30)) }
	none := true
	line := func(format string, args ...any) { none = false; fmt.Fprintf(out, "  "+format+"\n", args...) }
	for _, d := range r.Disks {
		name := "Disco del sistema"
		if d.Role == protocol.DiskDocuments {
			name = "Disco de Documentos"
		}
		line("%s: %s libres de %s", name, gb(d.FreeBytes), gb(d.TotalBytes))
	}
	switch r.DiskHealth {
	case protocol.DiskOK:
		line("Estado del disco (S.M.A.R.T.): bien")
	case protocol.DiskDegraded:
		line("Estado del disco (S.M.A.R.T.): CON FALLAS. Respalda y cambia el disco.")
	}
	if r.Memory != nil {
		line("Memoria: %s disponibles de %s", gb(r.Memory.AvailableBytes), gb(r.Memory.TotalBytes))
	}
	if r.CPU != nil && r.CPU.LoadPct != nil {
		line("Procesador: %d %% de uso", *r.CPU.LoadPct)
	}
	if r.CPU != nil && r.CPU.LimitPct != nil {
		line("Límite de frecuencia: %d %% de la máxima", *r.CPU.LimitPct)
	}
	if r.UptimeS != nil {
		line("Encendido desde hace: %s", (time.Duration(*r.UptimeS) * time.Second).String())
	}
	if r.RebootPending != nil && *r.RebootPending {
		line("Windows espera un reinicio por actualizaciones.")
	}
	if r.Battery != nil {
		line("Batería: %d %%, conectada a la corriente: %v", r.Battery.ChargePct, r.Battery.OnAC)
	}
	if none {
		line("Este sistema no informa la salud del equipo.")
	}
	if !selfTestSupported() {
		return
	}
	fmt.Fprintln(out, "  Autodiagnóstico:")
	st := &health.SelfTest{Sys: sys, TempDir: os.TempDir(), Supported: true}
	res := st.Run(context.Background(), nil, protocol.NewID(), protocol.NewID())
	for _, c := range res.Checks {
		fmt.Fprintf(out, "    %-10s %s %s\n", c.ID, c.Status, c.Reason)
	}
}
