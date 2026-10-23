package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/transport"
)

// ConsoleSource yields the consoles an agent may connect to. discovery.Browser
// implements it with mDNS; StaticSource gives fixed addresses.
type ConsoleSource interface {
	// Browse sends consoles as they are found. Closing the channel means the
	// source has nothing more to report (the agent keeps what it has).
	Browse(ctx context.Context) (<-chan discovery.Console, error)
}

// StaticSource is a fixed list of consoles, for direct connections without
// mDNS (tests, CI, networks that block multicast).
type StaticSource []discovery.Console

// Browse implements ConsoleSource.
func (s StaticSource) Browse(context.Context) (<-chan discovery.Console, error) {
	ch := make(chan discovery.Console, len(s))
	for _, c := range s {
		ch <- c
	}
	close(ch)
	return ch, nil
}

// AgentState is reported through Agent.OnState.
type AgentState int

// Agent states.
const (
	AgentSearching AgentState = iota + 1
	AgentConnected
	AgentDisconnected
)

func (s AgentState) String() string {
	switch s {
	case AgentSearching:
		return "buscando consola"
	case AgentConnected:
		return "conectado"
	case AgentDisconnected:
		return "desconectado"
	default:
		return "desconocido"
	}
}

// failuresBeforeRediscover is how many consecutive failures against the same
// console trigger a new discovery (docs/protocol.md §3.5).
const failuresBeforeRediscover = 3

// Agent runs the student side: find a console, connect, send hello and
// heartbeats, and reconnect with backoff when the session drops.
type Agent struct {
	Hello   protocol.HelloInfo
	Source  ConsoleSource
	Backoff Backoff
	Conn    transport.Options
	// OnMessage receives every message from the console except heartbeat. It
	// runs on the agent goroutine: keep it short.
	OnMessage func(ctx context.Context, c *transport.Client, env protocol.Envelope)
	// OnState is called on state changes; console is empty while searching.
	OnState func(state AgentState, console discovery.Console)
	Logger  *slog.Logger
}

// Run keeps the agent connected until ctx is canceled; then it returns nil.
func (a *Agent) Run(ctx context.Context) error {
	if err := a.Hello.Validate(); err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	if a.Source == nil {
		return fmt.Errorf("agent: nil console source")
	}
	log := a.Logger
	if log == nil {
		log = slog.Default()
	}

	var (
		known    []discovery.Console
		updates  <-chan discovery.Console
		stop     context.CancelFunc = func() {}
		failures int
		next     int
	)
	defer func() { stop() }()
	rediscover := func() error {
		stop()
		var bctx context.Context
		bctx, stop = context.WithCancel(ctx)
		ch, err := a.Source.Browse(bctx)
		if err != nil {
			return fmt.Errorf("agent: browse: %w", err)
		}
		updates, known, next = ch, nil, 0
		return nil
	}
	if err := rediscover(); err != nil {
		return err
	}

	for {
		known, updates = drain(known, updates)
		if len(known) == 0 {
			a.state(AgentSearching, discovery.Console{})
			var ok bool
			known, updates, ok = waitConsole(ctx, known, updates)
			if ctx.Err() != nil {
				return nil
			}
			if !ok { // source exhausted without results: retry later
				failures++
				if !sleep(ctx, a.Backoff.Delay(failures)) {
					return nil
				}
				if err := rediscover(); err != nil {
					log.Warn("rediscovery failed", "err", err)
				}
				continue
			}
		}

		target := known[next%len(known)]
		established := a.session(ctx, log, target)
		if ctx.Err() != nil {
			return nil
		}
		if established {
			failures = 0
		} else {
			failures++
			if failures%failuresBeforeRediscover == 0 {
				next++
				log.Info("console unreachable, searching again", "failures", failures)
				if err := rediscover(); err != nil {
					log.Warn("rediscovery failed", "err", err)
				}
			}
		}
		if !sleep(ctx, a.Backoff.Delay(failures)) {
			return nil
		}
	}
}

// session connects to target and serves the session until it ends. It reports
// whether the session was established (hello sent).
func (a *Agent) session(ctx context.Context, log *slog.Logger, target discovery.Console) bool {
	var cl *transport.Client
	for _, hp := range target.HostPorts() {
		url := "ws://" + hp + transport.Path
		c, err := transport.Dial(ctx, url, a.Conn)
		if err != nil {
			log.Debug("dial failed", "url", url, "err", err)
			continue
		}
		cl = c
		break
	}
	if cl == nil {
		return false
	}
	if _, err := cl.Send(ctx, a.Hello); err != nil {
		log.Warn("cannot send hello", "err", err)
		cl.Close(transport.CloseNormal, "")
		<-cl.Done()
		return false
	}
	cl.StartHeartbeat()
	a.state(AgentConnected, target)
	log.Info("connected to console", "console", target.Name, "addr", cl.RemoteAddr())

	for env := range cl.Incoming() {
		if env.Type == protocol.TypeHeartbeat {
			continue
		}
		if a.OnMessage != nil {
			a.OnMessage(ctx, cl, env)
		}
	}
	<-cl.Done()
	a.state(AgentDisconnected, target)
	log.Info("disconnected from console", "console", target.Name)
	return true
}

func (a *Agent) state(s AgentState, c discovery.Console) {
	if a.OnState != nil {
		a.OnState(s, c)
	}
}

// drain applies pending updates without blocking.
func drain(known []discovery.Console, updates <-chan discovery.Console) ([]discovery.Console, <-chan discovery.Console) {
	for updates != nil {
		select {
		case c, ok := <-updates:
			if !ok {
				return known, nil
			}
			known = upsert(known, c)
		default:
			return known, updates
		}
	}
	return known, nil
}

// waitConsole blocks until a console is known. ok is false if the source
// finished without any.
func waitConsole(ctx context.Context, known []discovery.Console, updates <-chan discovery.Console) ([]discovery.Console, <-chan discovery.Console, bool) {
	for len(known) == 0 {
		if updates == nil {
			return known, nil, false
		}
		select {
		case <-ctx.Done():
			return known, updates, false
		case c, ok := <-updates:
			if !ok {
				updates = nil
				continue
			}
			known = upsert(known, c)
		}
	}
	return known, updates, true
}

func upsert(known []discovery.Console, c discovery.Console) []discovery.Console {
	for i := range known {
		if known[i].Instance == c.Instance {
			known[i] = c
			return known
		}
	}
	return append(known, c)
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
