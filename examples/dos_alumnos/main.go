// Command dos_alumnos runs the session flow end to end (mDNS, TLS,
// authentication, registry) and prints the console's registry live.
//
// With -rol todo (default) it starts, in one process, a console and two
// simulated students that find it over mDNS; after a while the second student
// stops so it can be seen leaving the registry. On separate machines use
// -rol profesor on one and -rol alumno -clave <public key> on the others.
package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"flag"
	"fmt"
	"io/fs"
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
	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

const agentVersion = "0.1.0"

func main() {
	rol := flag.String("rol", "todo", "todo | profesor | alumno")
	canal := flag.String("canal", "AULA_101", "canal del aula")
	puerto := flag.Int("puerto", 47830, "puerto WebSocket de la consola")
	nombre := flag.String("nombre", "Consola de ejemplo", "nombre visible de la consola")
	dir := flag.String("dir", "", "profesor: directorio de la clave y el certificado (por defecto, el de aula-consola keygen)")
	clave := flag.String("clave", "", "alumno: clave pública de la consola (aula-ed25519:…)")
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
		err = runTeacher(ctx, *canal, *puerto, *nombre, *dir)
	case "alumno":
		var pub ed25519.PublicKey
		if pub, err = security.ParsePublicKey(*clave); err != nil {
			err = fmt.Errorf("-clave: %w (cópiala de la salida del profesor o de aula-consola pubkey)", err)
			break
		}
		err = runStudent(ctx, *canal, *directo, pub, localHello())
	default:
		err = fmt.Errorf("rol desconocido %q", *rol)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// startTeacher runs Announcer + Server (TLS) + Registry + Console until ctx
// ends, with the identity and certificate kept in dir.
func startTeacher(ctx context.Context, wg *sync.WaitGroup, channel string, port int, name, dir string) (*session.Registry, *security.Identity, *session.Console, error) {
	id, err := security.LoadIdentity(dir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w (crea la clave con: aula-consola keygen)", err)
	}
	cert, err := security.LoadOrCreateCertificate(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("escuchar en el puerto %d: %w", port, err)
	}
	reg := session.NewRegistry(0)
	srv := transport.NewServer(transport.Options{TLS: security.ServerTLS(cert)})
	console := &session.Console{Channel: channel, Registry: reg, Identity: id, CertSHA256: cert.SHA256}
	announcer := &discovery.Announcer{Announcement: discovery.Announcement{
		Channel: channel, ConsoleName: name, WSPort: port, CertFingerprint: cert.Fingerprint(),
	}}
	wg.Add(3)
	go func() { defer wg.Done(); logErr("servidor", srv.Serve(ctx, ln)) }()
	go func() { defer wg.Done(); logErr("consola", console.Run(ctx, srv.Events())) }()
	go func() { defer wg.Done(); logErr("anuncio mDNS", announcer.Run(ctx)) }()
	fmt.Printf("Consola %q en el canal %s, puerto %d (TLS). Ctrl+C para salir.\n", name, channel, port)
	fmt.Printf("Clave pública para los alumnos: %s\n", security.EncodePublicKey(id.Public()))
	fmt.Printf("Huella: %s\n", security.KeyFingerprint(id.Public()))
	return reg, id, console, nil
}

func runTeacher(ctx context.Context, channel string, port int, name, dir string) error {
	if dir == "" {
		var err error
		if dir, err = security.DefaultDir(); err != nil {
			return err
		}
	}
	// Like aula-consola keygen, so a test machine needs only this binary.
	if _, err := security.LoadIdentity(dir); errors.Is(err, fs.ErrNotExist) {
		if _, err := security.CreateIdentity(dir, false); err != nil {
			return err
		}
		fmt.Printf("No había clave: se creó una nueva en %s\n", dir)
	}
	var wg sync.WaitGroup
	reg, _, console, err := startTeacher(ctx, &wg, channel, port, name, dir)
	if err != nil {
		return err
	}
	fmt.Println("Escribe \"ayuda\" para ver cómo enviar código a los alumnos.")
	go commands(ctx, console, os.Stdin)
	printRegistry(ctx, reg, channel)
	wg.Wait()
	return nil
}

func runStudent(ctx context.Context, channel, direct string, consoleKey ed25519.PublicKey, hello protocol.HelloInfo) error {
	src, err := source(channel, direct)
	if err != nil {
		return err
	}
	a := &session.Agent{
		Channel:          channel,
		ConsolePublicKey: consoleKey,
		Hello:            hello,
		Source:           src,
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

// runAll uses a temporary identity, so it works without running keygen.
func runAll(ctx context.Context, channel string, port int, name string, leaveAfter time.Duration) error {
	dir, err := os.MkdirTemp("", "aula-ejemplo-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if _, err := security.CreateIdentity(dir, false); err != nil {
		return err
	}
	var wg sync.WaitGroup
	reg, id, _, err := startTeacher(ctx, &wg, channel, port, name, dir)
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
			logErr(h.Hostname, runStudent(actx, channel, direct, id.Public(), h))
		}()
	}
	printRegistry(ctx, reg, channel)
	wg.Wait()
	return nil
}

// source finds the console over mDNS, or uses a fixed address. A direct
// connection has no announced fingerprint: the certificate is accepted and the
// console is still authenticated by its signature (docs/protocol.md §10.2).
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
	return session.StaticSource{{Instance: "directo", Channel: channel, Name: direct, Addrs: ips, Port: port}}, nil
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
				fmt.Fprintf(&b, "  %-14s %-12s %-8s %-13s %-22s %s\n", s.Hostname, s.Username, s.OS, s.State, s.RemoteAddr, ackLabel(s))
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
