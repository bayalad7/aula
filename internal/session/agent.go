package session

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
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

// untrustedFor is how long a console that failed authentication is skipped
// (docs/protocol.md §1.3).
const untrustedFor = 5 * time.Minute

// Agent runs the student side: find a console, connect over TLS, authenticate
// it, send hello and heartbeats, and reconnect with backoff when the session
// drops.
type Agent struct {
	Channel string
	// ConsolePublicKey is the only console key this agent obeys.
	ConsolePublicKey ed25519.PublicKey
	HandshakeTimeout time.Duration // 0 means DefaultHelloTimeout
	Hello            protocol.HelloInfo
	Source           ConsoleSource
	Backoff          Backoff
	Conn             transport.Options
	// OnMessage receives every message from the console except heartbeat. It
	// runs on the agent goroutine: keep it short.
	OnMessage func(ctx context.Context, c *transport.Client, env protocol.Envelope)
	// OnState is called on state changes; console is empty while searching.
	OnState func(state AgentState, console discovery.Console)
	// OnDiagnostic explains, in Spanish for the student's screen, why the agent
	// is not connected yet (no console found, wrong key, unreachable…); it is
	// called with "" once connected.
	OnDiagnostic func(msg string)
	// SearchHint is how long to search before reporting that no console of
	// the channel was found; 0 means DefaultSearchHint.
	SearchHint time.Duration
	Logger     *slog.Logger
}

// DefaultSearchHint is how long the agent searches silently before it says no
// console of its channel was found.
const DefaultSearchHint = 10 * time.Second

// Run keeps the agent connected until ctx is canceled; then it returns nil.
func (a *Agent) Run(ctx context.Context) error {
	if err := a.Hello.Validate(); err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	if a.Source == nil {
		return fmt.Errorf("agent: nil console source")
	}
	if !protocol.ValidChannel(a.Channel) {
		return fmt.Errorf("agent: invalid channel %q", a.Channel)
	}
	if len(a.ConsolePublicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("agent: console public key is required")
	}
	log := a.Logger
	if log == nil {
		log = slog.Default()
	}

	var (
		known     []discovery.Console
		updates   <-chan discovery.Console
		stop      context.CancelFunc = func() {}
		failures  int
		next      int
		untrusted = map[string]time.Time{} // instance → until
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
			hint := a.SearchHint
			if hint <= 0 {
				hint = DefaultSearchHint
			}
			var ok bool
			known, updates, ok = waitConsole(ctx, known, updates, hint, func() {
				a.diagnose(fmt.Sprintf("No se encontró ninguna consola del canal %s. Revisa que la consola "+
					"del profesor esté abierta en ese canal, que la red de este equipo sea Privada o de "+
					"Dominio (no Pública) y que el firewall permita Aula.", a.Channel))
			})
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

		target, ok := pick(known, next, untrusted, time.Now())
		if !ok { // every known console failed authentication recently
			failures++
			if !sleep(ctx, a.Backoff.Delay(failures)) {
				return nil
			}
			known, updates = drain(known, updates)
			continue
		}
		established, authFailed := a.session(ctx, log, target)
		if ctx.Err() != nil {
			return nil
		}
		if authFailed {
			untrusted[target.Instance] = time.Now().Add(untrustedFor)
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

// session connects to target, authenticates it and serves the session until
// it ends. It reports whether the session was established (hello sent) and
// whether the console failed authentication.
func (a *Agent) session(ctx context.Context, log *slog.Logger, target discovery.Console) (established, authFailed bool) {
	var (
		cl      *transport.Client
		certSHA [32]byte
		tried   []string
	)
	name := target.Name
	if name == "" {
		name = target.Instance
	}
	for _, hp := range target.HostPorts() {
		url := "wss://" + hp + transport.Path
		var seen [32]byte
		tlsConf, err := security.ClientTLS(target.CertFingerprint, &seen)
		if err != nil {
			log.Warn("bad console fingerprint", "console", target.Name, "err", err)
			return false, true
		}
		opts := a.Conn
		opts.TLS = tlsConf
		c, err := transport.Dial(ctx, url, opts)
		if err != nil {
			if errors.Is(err, security.ErrFingerprintMismatch) {
				log.Warn("console certificate does not match its announcement", "console", target.Name, "url", url)
				a.diagnose(fmt.Sprintf("La consola «%s» (%s) presentó un certificado distinto del que anuncia; "+
					"se ignora.", name, hp))
				return false, true
			}
			log.Warn("dial failed", "url", url, "err", err)
			tried = append(tried, hp)
			continue
		}
		cl, certSHA = c, seen
		break
	}
	if cl == nil {
		a.diagnose(fmt.Sprintf("Se encontró la consola «%s» del canal %s, pero no se pudo conectar a %s. "+
			"Revisa el firewall del equipo del profesor (TCP %d) y que ambos estén en la misma red.",
			name, a.Channel, strings.Join(tried, ", "), target.Port))
		return false, false
	}
	if err := a.authenticate(ctx, cl, certSHA); err != nil {
		log.Warn("console rejected", "console", target.Name, "addr", cl.RemoteAddr(), "reason", err)
		cl.Close(transport.ClosePolicyViolation, "authentication failed")
		<-cl.Done()
		if errors.Is(err, errBadProof) {
			a.diagnose(fmt.Sprintf("Se encontró la consola «%s» del canal %s, pero su clave no es la de este "+
				"centro: no se le obedece. Si es la consola de tu profesor, este equipo tiene otra clave del "+
				"centro; avisa al administrador.", name, a.Channel))
			return false, true
		}
		a.diagnose(fmt.Sprintf("La consola «%s» no completó la autenticación; se reintentará.", name))
		return false, false
	}
	if _, err := cl.Send(ctx, a.Hello); err != nil {
		log.Warn("cannot send hello", "err", err)
		cl.Close(transport.CloseNormal, "")
		<-cl.Done()
		return false, false
	}
	cl.StartHeartbeat()
	a.diagnose("")
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
	return true, false
}

var (
	errBadProof        = errors.New("auth.proof signature does not verify with the configured key")
	errUnexpected      = errors.New("message before authentication")
	errHandshakeClosed = errors.New("connection closed during authentication")
	errHandshakeTime   = errors.New("authentication timeout")
)

// authenticate runs auth.challenge → auth.proof → auth.ok (docs/protocol.md
// §3.2). Until it succeeds, no other message is processed: anything but the
// matching auth.proof is answered with unauthorized and ends the attempt.
func (a *Agent) authenticate(ctx context.Context, cl *transport.Client, certSHA [32]byte) error {
	timeout := a.HandshakeTimeout
	if timeout <= 0 {
		timeout = DefaultHelloTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	nonce := security.NewNonce()
	challengeID, err := cl.Send(ctx, protocol.AuthChallenge{Nonce: nonce})
	if err != nil {
		return fmt.Errorf("send auth.challenge: %w", err)
	}
	select {
	case env, ok := <-cl.Incoming():
		if !ok {
			return errHandshakeClosed
		}
		proof, isProof := env.Payload.(protocol.AuthProof)
		if !isProof || proof.RefID != challengeID {
			if env.Type != protocol.TypeError {
				_, _ = cl.Send(ctx, protocol.NewError(env.ID, protocol.CodeUnauthorized, "authentication required"))
			}
			return fmt.Errorf("%w: %s", errUnexpected, env.Type)
		}
		if !security.VerifyProof(a.ConsolePublicKey, nonce, a.Channel, certSHA, proof.Signature) {
			_, _ = cl.Send(ctx, protocol.NewError(env.ID, protocol.CodeUnauthorized, "invalid signature"))
			return errBadProof
		}
		if _, err := cl.Send(ctx, protocol.AuthOK{RefID: env.ID}); err != nil {
			return fmt.Errorf("send auth.ok: %w", err)
		}
		return nil
	case <-timer.C:
		return errHandshakeTime
	case <-ctx.Done():
		return ctx.Err()
	}
}

// pick returns the next console to try, skipping those that failed
// authentication recently.
func pick(known []discovery.Console, next int, untrusted map[string]time.Time, now time.Time) (discovery.Console, bool) {
	for i := range known {
		c := known[(next+i)%len(known)]
		if until, bad := untrusted[c.Instance]; bad && now.Before(until) {
			continue
		}
		return c, true
	}
	return discovery.Console{}, false
}

func (a *Agent) state(s AgentState, c discovery.Console) {
	if a.OnState != nil {
		a.OnState(s, c)
	}
}

func (a *Agent) diagnose(msg string) {
	if a.OnDiagnostic != nil {
		a.OnDiagnostic(msg)
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
// finished without any. If none appears within slowAfter, onSlow is called
// once (the search goes on).
func waitConsole(ctx context.Context, known []discovery.Console, updates <-chan discovery.Console,
	slowAfter time.Duration, onSlow func()) ([]discovery.Console, <-chan discovery.Console, bool) {
	slow := time.NewTimer(slowAfter)
	defer slow.Stop()
	for len(known) == 0 {
		if updates == nil {
			onSlow()
			return known, nil, false
		}
		select {
		case <-ctx.Done():
			return known, updates, false
		case <-slow.C:
			onSlow()
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
