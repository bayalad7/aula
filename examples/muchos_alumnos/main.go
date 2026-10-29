// Command muchos_alumnos launches N simulated students against a real console,
// for load and reconnection tests (Hito 1). Each one finds the console (mDNS or
// -directo), authenticates it, shows code like the real app (codedist.Receiver)
// and acknowledges it. Every few seconds it prints how many are connected and
// what they have on screen.
//
//	muchos_alumnos -n 12 -config C:\ProgramData\Aula\alumno\config.json
//	muchos_alumnos -n 12 -canal 1 -clave aula-ed25519:… -directo 192.168.1.10:47830
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/transport"
)

type student struct {
	name      string
	board     *codedist.Board
	mu        sync.Mutex
	connected bool
	sessions  int // times it got connected (reconnections show here)
}

func main() {
	n := flag.Int("n", 12, "número de alumnos simulados")
	cfgPath := flag.String("config", "", "config.json del agente (el de la máquina o uno de prueba)")
	channel := flag.String("canal", "", "canal (si no usas -config)")
	key := flag.String("clave", "", "clave pública de la consola (si no usas -config)")
	direct := flag.String("directo", "", "IP:puerto de la consola, sin mDNS")
	prefix := flag.String("prefijo", "SIM", "prefijo del nombre de equipo")
	every := flag.Duration("cada", 3*time.Second, "cada cuánto imprimir el resumen")
	verbose := flag.Bool("v", false, "logs detallados")
	flag.Parse()

	cfg := session.AgentConfig{Channel: *channel, ConsolePublicKey: *key, Direct: *direct}
	if *cfgPath != "" {
		data, err := os.ReadFile(*cfgPath)
		if err == nil {
			err = json.Unmarshal(data, &cfg)
		}
		if err != nil {
			fail(err)
		}
		if *direct != "" {
			cfg.Direct = *direct
		}
	}
	pub, err := cfg.Validate()
	if err != nil {
		fail(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if *verbose {
		log = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	students := make([]*student, *n)
	var wg sync.WaitGroup
	for i := range students {
		s := &student{name: fmt.Sprintf("%s-%02d", *prefix, i+1), board: codedist.NewBoard(0)}
		students[i] = s
		agent := &session.Agent{
			Channel:          cfg.Channel,
			ConsolePublicKey: pub,
			Hello: protocol.HelloInfo{
				Hostname: s.name, Username: "simulado", OS: protocol.OSWindows, OSVersion: "simulado",
				AgentVersion: "0.1.0-sim", Capabilities: []protocol.Capability{protocol.CapCodeView, protocol.CapMsgShow},
			},
			Source:    source(cfg),
			Logger:    log.With("agent", s.name),
			OnMessage: s.onMessage(),
			OnState: func(st session.AgentState, _ discovery.Console) {
				s.mu.Lock()
				s.connected = st == session.AgentConnected
				if s.connected {
					s.sessions++
				}
				s.mu.Unlock()
			},
		}
		wg.Add(1)
		go func() { defer wg.Done(); _ = agent.Run(ctx) }()
	}
	fmt.Printf("%d alumnos simulados en el canal %s. Ctrl+C para salir.\n", *n, cfg.Channel)

	t := time.NewTicker(*every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-t.C:
			summary(students)
		}
	}
}

func (s *student) onMessage() func(context.Context, *transport.Client, protocol.Envelope) {
	recv := &codedist.Receiver{Board: s.board}
	return func(ctx context.Context, cl *transport.Client, env protocol.Envelope) {
		if reply := recv.Handle(env); reply != nil {
			_, _ = cl.Send(ctx, reply)
		}
	}
}

// summary prints one line: connected count, reconnections and, grouped, what
// the students have on screen.
func summary(students []*student) {
	connected, reconnects := 0, 0
	screens := map[string]int{}
	for _, s := range students {
		s.mu.Lock()
		if s.connected {
			connected++
		}
		if s.sessions > 1 {
			reconnects += s.sessions - 1
		}
		s.mu.Unlock()
		var parts []string
		for _, sn := range s.board.Snippets() {
			if sn.Exercise != "" {
				parts = append(parts, "ej. "+sn.Exercise)
			} else {
				parts = append(parts, strconv.Quote(sn.Title))
			}
		}
		screen := "vacía"
		if len(parts) > 0 {
			screen = strings.Join(parts, " + ")
		}
		screens[screen]++
	}
	var keys []string
	for k := range screens {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, " [%s: %d]", k, screens[k])
	}
	fmt.Printf("[%s] conectados %d/%d · reconexiones %d · pantallas:%s\n",
		time.Now().Format("15:04:05"), connected, len(students), reconnects, b.String())
}

func source(cfg session.AgentConfig) session.ConsoleSource {
	if cfg.Direct == "" {
		return &discovery.Browser{Channel: cfg.Channel}
	}
	host, portStr, _ := net.SplitHostPort(cfg.Direct) // checked by Validate
	port, _ := strconv.Atoi(portStr)
	ips, err := net.LookupIP(host)
	if err != nil {
		fail(fmt.Errorf("-directo: %w", err))
	}
	return session.StaticSource{{Instance: "directo", Channel: cfg.Channel, Name: cfg.Direct, Addrs: ips, Port: port}}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
