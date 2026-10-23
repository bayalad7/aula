// Command dos_alumnos runs the stage 04 flow end to end and prints the
// console's registry live.
//
// With -rol todo (default) it starts, in one process, a console and two
// simulated students that find it over mDNS; after a while the second student
// stops so it can be seen leaving the registry. On separate machines use
// -rol profesor on one and -rol alumno on the others.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

const agentVersion = "0.1.0"

func main() {
	rol := flag.String("rol", "todo", "todo | profesor | alumno")
	canal := flag.String("canal", "AULA_101", "canal del aula")
	puerto := flag.Int("puerto", 47830, "puerto WebSocket de la consola")
	nombre := flag.String("nombre", "Consola de ejemplo", "nombre visible de la consola")
	directo := flag.String("directo", "", "alumno: host:puerto de la consola, sin mDNS")
	salida := flag.Duration("salida", 10*time.Second, "rol todo: cuándo se apaga el alumno 2 (0 = nunca)")
	verbose := flag.Bool("v", false, "logs detallados")
	flag.Parse()

	level := slog.LevelWarn
	if *verbose {
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	switch *rol {
	case "todo":
		err = runAll(ctx, *canal, *puerto, *nombre, *salida)
	case "profesor":
		err = runTeacher(ctx, *canal, *puerto, *nombre)
	case "alumno":
		err = runStudent(ctx, *canal, *directo, localHello())
	default:
		err = fmt.Errorf("rol desconocido %q", *rol)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// startTeacher runs Announcer + Server + Registry + Console until ctx ends.
func startTeacher(ctx context.Context, wg *sync.WaitGroup, channel string, port int, name string) (*session.Registry, error) {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return nil, fmt.Errorf("escuchar en el puerto %d: %w", port, err)
	}
	reg := session.NewRegistry(0)
	srv := transport.NewServer(transport.Options{})
	console := &session.Console{Channel: channel, Registry: reg}
	announcer := &discovery.Announcer{Announcement: discovery.Announcement{
		Channel: channel, ConsoleName: name, WSPort: port, CertFingerprint: discovery.PlaceholderFingerprint,
	}}
	wg.Add(3)
	go func() { defer wg.Done(); logErr("servidor", srv.Serve(ctx, ln)) }()
	go func() { defer wg.Done(); logErr("consola", console.Run(ctx, srv.Events())) }()
	go func() { defer wg.Done(); logErr("anuncio mDNS", announcer.Run(ctx)) }()
	fmt.Printf("Consola %q en el canal %s, puerto %d. Ctrl+C para salir.\n", name, channel, port)
	return reg, nil
}

func runTeacher(ctx context.Context, channel string, port int, name string) error {
	var wg sync.WaitGroup
	reg, err := startTeacher(ctx, &wg, channel, port, name)
	if err != nil {
		return err
	}
	printRegistry(ctx, reg, channel)
	wg.Wait()
	return nil
}

func runStudent(ctx context.Context, channel, direct string, hello protocol.HelloInfo) error {
	src, err := source(channel, direct)
	if err != nil {
		return err
	}
	a := &session.Agent{
		Hello:  hello,
		Source: src,
		OnState: func(s session.AgentState, c discovery.Console) {
			if c.Name != "" {
				fmt.Printf("[%s] %s: %s (%s)\n", now(), hello.Hostname, s, c.Name)
			} else {
				fmt.Printf("[%s] %s: %s en %s\n", now(), hello.Hostname, s, channel)
			}
		},
	}
	return a.Run(ctx)
}

func runAll(ctx context.Context, channel string, port int, name string, leaveAfter time.Duration) error {
	var wg sync.WaitGroup
	reg, err := startTeacher(ctx, &wg, channel, port, name)
	if err != nil {
		return err
	}
	direct := ""
	probe, cancelProbe := context.WithCancel(ctx)
	_, err = (&discovery.Browser{Channel: channel}).Browse(probe)
	cancelProbe()
	if err != nil {
		fmt.Println("mDNS no disponible, los alumnos se conectan directo:", err)
		direct = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}

	for i := 1; i <= 2; i++ {
		actx, cancel := context.WithCancel(ctx)
		if i == 2 && leaveAfter > 0 {
			time.AfterFunc(leaveAfter, func() {
				fmt.Printf("[%s] alumno-2 se apaga\n", now())
				cancel()
			})
		}
		h := localHello()
		h.Hostname = fmt.Sprintf("alumno-%d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancel()
			logErr(h.Hostname, runStudent(actx, channel, direct, h))
		}()
	}
	printRegistry(ctx, reg, channel)
	wg.Wait()
	return nil
}

func source(channel, direct string) (session.ConsoleSource, error) {
	if direct == "" {
		return &discovery.Browser{Channel: channel}, nil
	}
	host, portStr, err := net.SplitHostPort(direct)
	if err != nil {
		return nil, fmt.Errorf("-directo: %w", err)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("-directo: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("-directo: puerto: %w", err)
	}
	return session.StaticSource{{
		Instance: "directo", Channel: channel, Name: direct, Addrs: ips, Port: port,
		CertFingerprint: discovery.PlaceholderFingerprint,
	}}, nil
}

// printRegistry prints the registry whenever it changes, until ctx ends.
func printRegistry(ctx context.Context, reg *session.Registry, channel string) {
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()
	last := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			var b strings.Builder
			list := reg.List(channel)
			fmt.Fprintf(&b, "Registro %s: %d equipo(s)\n", channel, len(list))
			for _, s := range list {
				fmt.Fprintf(&b, "  %-14s %-12s %-8s %-13s %s\n", s.Hostname, s.Username, s.OS, s.State, s.RemoteAddr)
			}
			if out := b.String(); out != last {
				fmt.Printf("[%s] %s", now(), out)
				last = out
			}
		}
	}
}

func localHello() protocol.HelloInfo {
	host, _ := os.Hostname()
	if host == "" {
		host = "desconocido"
	}
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
		if i := strings.LastIndexAny(username, `\/`); i >= 0 {
			username = username[i+1:] // drop the Windows domain
		}
	}
	return protocol.HelloInfo{
		Hostname:     host,
		Username:     username,
		OS:           protocol.OS(runtime.GOOS),
		OSVersion:    runtime.GOOS + "/" + runtime.GOARCH,
		AgentVersion: agentVersion,
		Capabilities: []protocol.Capability{protocol.CapCodeView, protocol.CapMsgShow},
	}
}

func logErr(what string, err error) {
	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
	}
}

func now() string { return time.Now().Format("15:04:05") }
