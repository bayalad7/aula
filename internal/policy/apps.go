package policy

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// Timing of the app enforcement (D-017).
const (
	DefaultPollEvery  = 500 * time.Millisecond
	closedNoticeEvery = 30 * time.Second // one notice per app, at most
	maxSummaryItems   = 15
)

// AppPolicy is an app.block.set in force.
type AppPolicy struct {
	ID   string
	Mode protocol.PolicyMode
	Apps []string
	keys map[string]bool
}

// NewAppPolicy builds the policy of a valid app.block.set.
func NewAppPolicy(p protocol.AppBlockSet) *AppPolicy {
	ap := &AppPolicy{ID: p.PolicyID, Mode: p.Mode, Apps: p.Apps, keys: map[string]bool{}}
	for _, a := range p.Apps {
		ap.keys[protocol.AppKey(a)] = true
	}
	return ap
}

// Protection lists what is never closed, whatever the policy says: the
// system, the shell and the agent itself (docs/protocol.md §5.13).
type Protection struct {
	SystemRoot string   // e.g. C:\Windows: everything under it is protected
	Keep       []string // app keys, e.g. "aula-cliente", "msedgewebview2", "explorer"
}

func (p Protection) protects(proc platform.ProcessInfo) bool {
	key := protocol.AppKey(proc.Name)
	for _, k := range p.Keep {
		if key == protocol.AppKey(k) {
			return true
		}
	}
	if proc.Path == "" {
		return true // cannot tell where it comes from: leave it
	}
	root := strings.TrimSuffix(strings.ToLower(p.SystemRoot), `\`)
	return root != "" && strings.HasPrefix(strings.ToLower(proc.Path), root+`\`)
}

// Blocks reports whether proc must be closed under the policy.
func (a *AppPolicy) Blocks(proc platform.ProcessInfo, prot Protection) bool {
	if prot.protects(proc) {
		return false
	}
	listed := a.keys[protocol.AppKey(proc.Name)]
	if a.Mode == protocol.PolicyAllow {
		return !listed
	}
	return listed
}

// AppEnforcer closes, every PollEvery, the processes of the student's
// session that the policy forbids. It writes nothing on the system: without
// a policy, or when it stops, apps simply open again.
type AppEnforcer struct {
	Procs   platform.ProcessControl
	Protect Protection
	// Session returns the student's session; ok is false without one.
	Session func() (session uint32, ok bool)
	// OnClosed is told the name of each app closed, at most once per app
	// every 30 s, for the student's notice (docs/privacidad.md rule 22).
	OnClosed  func(name string)
	PollEvery time.Duration
	Logger    *slog.Logger

	mu      sync.Mutex
	policy  *AppPolicy
	noticed map[string]time.Time
	kick    chan struct{}
	now     func() time.Time
}

// SetPolicy replaces the policy (nil removes it) and enforces it at once.
func (e *AppEnforcer) SetPolicy(p *AppPolicy) {
	e.mu.Lock()
	e.policy = p
	e.noticed = map[string]time.Time{}
	kick := e.kick
	e.mu.Unlock()
	if kick != nil {
		select {
		case kick <- struct{}{}:
		default:
		}
	}
}

// Run enforces until ctx is canceled.
func (e *AppEnforcer) Run(ctx context.Context) {
	every := e.PollEvery
	if every <= 0 {
		every = DefaultPollEvery
	}
	e.mu.Lock()
	e.kick = make(chan struct{}, 1)
	kick := e.kick
	e.mu.Unlock()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-kick:
		}
		e.Enforce()
	}
}

// Enforce makes one pass: lists the session's processes and closes the
// forbidden ones.
func (e *AppEnforcer) Enforce() {
	e.mu.Lock()
	p := e.policy
	e.mu.Unlock()
	if p == nil || e.Session == nil {
		return
	}
	session, ok := e.Session()
	if !ok {
		return
	}
	procs, err := e.Procs.List(session)
	if err != nil {
		e.log().Warn("cannot list the session's processes", "err", err)
		return
	}
	for _, proc := range procs {
		if !p.Blocks(proc, e.Protect) {
			continue
		}
		if err := e.Procs.Kill(proc.PID); err != nil {
			e.log().Warn("cannot close a blocked app", "app", proc.Name, "err", err)
			continue
		}
		e.log().Info("blocked app closed", "app", proc.Name, "policy", p.ID)
		e.notice(proc.Name)
	}
}

func (e *AppEnforcer) notice(name string) {
	if e.OnClosed == nil {
		return
	}
	now := time.Now
	if e.now != nil {
		now = e.now
	}
	key := protocol.AppKey(name)
	e.mu.Lock()
	last, seen := e.noticed[key]
	if seen && now().Sub(last) < closedNoticeEvery {
		e.mu.Unlock()
		return
	}
	e.noticed[key] = now()
	e.mu.Unlock()
	e.OnClosed(name)
}

func (e *AppEnforcer) log() *slog.Logger {
	if e.Logger != nil {
		return e.Logger
	}
	return slog.New(slog.DiscardHandler)
}
