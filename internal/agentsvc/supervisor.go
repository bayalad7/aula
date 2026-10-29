package agentsvc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/session"
)

// HelperFlag is the argument that starts the executable as the helper; the
// next argument is the name of the local channel.
const HelperFlag = "--ayudante"

// Defaults of the Supervisor.
const (
	DefaultPoll        = 2 * time.Second
	DefaultStableAfter = 30 * time.Second
)

// Supervisor keeps one helper running in the console session of the student:
// it starts it when a user logs on, starts it again (with backoff) if it dies,
// moves it when the user changes and stops it at logoff. Only the helper it
// started may connect to the channel it opened.
type Supervisor struct {
	Sessions platform.SessionManager
	Exe      string // the agent executable, started with HelperFlag
	// Listen and PeerPID are platform.ListenLocal and platform.PeerPID;
	// tests replace them.
	Listen  func(name, allowSID string) (net.Listener, error)
	PeerPID func(net.Conn) (int, error)
	// Serve runs the channel with the helper until it ends. It is called on
	// its own goroutine once per helper process, after the PID check.
	Serve func(ctx context.Context, s platform.Session, conn net.Conn)
	// Poll is how often the active session is checked besides the session
	// change notices; 0 means DefaultPoll.
	Poll time.Duration
	// StableAfter is how long a helper must run for its failures to be
	// forgotten; 0 means DefaultStableAfter.
	StableAfter time.Duration
	Backoff     session.Backoff
	Logger      *slog.Logger
	now         func() time.Time // replaced in tests
}

type running struct {
	sess    platform.Session
	proc    platform.Process
	cancel  context.CancelFunc
	started time.Time
}

// Run supervises until ctx is canceled; then it stops the helper.
// sessionChanged receives the OS notices of logon, logoff and session switch.
func (s *Supervisor) Run(ctx context.Context, sessionChanged <-chan struct{}) error {
	if s.Sessions == nil || s.Listen == nil || s.PeerPID == nil || s.Serve == nil {
		return fmt.Errorf("supervisor: sessions, listen, peer pid and serve are required")
	}
	now := s.now
	if now == nil {
		now = time.Now
	}
	poll := s.Poll
	if poll <= 0 {
		poll = DefaultPoll
	}
	stable := s.StableAfter
	if stable <= 0 {
		stable = DefaultStableAfter
	}
	var (
		cur      *running
		failures int
		next     time.Time // earliest next launch
		exited   = make(chan *running, 1)
	)
	stop := func(why string) {
		if cur == nil {
			return
		}
		s.log().Info("helper stopped", "session", cur.sess.ID, "user", cur.sess.User, "why", why)
		cur.cancel()
		_ = cur.proc.Kill()
		cur = nil
	}
	defer stop("service stopping")

	reconcile := func() {
		active, ok, err := s.Sessions.Active()
		if err != nil {
			s.log().Warn("cannot read the active session", "err", err)
			return
		}
		if cur != nil && (!ok || active.ID != cur.sess.ID || active.SID != cur.sess.SID) {
			stop("session changed")
			failures, next = 0, time.Time{} // a new session starts fresh
		}
		if cur != nil || !ok || now().Before(next) {
			return
		}
		r, err := s.launch(ctx, active, exited)
		if err != nil {
			failures++
			next = now().Add(s.Backoff.Delay(failures))
			s.log().Warn("helper not started", "session", active.ID, "err", err, "retry_in", next.Sub(now()).Round(time.Second))
			return
		}
		cur = r
	}

	t := time.NewTicker(poll)
	defer t.Stop()
	reconcile()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-sessionChanged:
			reconcile()
		case <-t.C:
			reconcile()
		case r := <-exited:
			if r != cur {
				continue // one we stopped ourselves
			}
			cur = nil
			if now().Sub(r.started) >= stable {
				failures = 0
			}
			next = now().Add(s.Backoff.Delay(failures))
			failures++
			s.log().Info("helper exited, starting it again", "session", r.sess.ID, "retry_in", next.Sub(now()).Round(time.Second))
			reconcile()
		}
	}
}

// launch opens a fresh channel, starts the helper with its name and serves
// the first connection that comes from that very process.
func (s *Supervisor) launch(ctx context.Context, sess platform.Session, exited chan<- *running) (*running, error) {
	name, err := channelName(sess)
	if err != nil {
		return nil, err
	}
	ln, err := s.Listen(name, sess.SID)
	if err != nil {
		return nil, err
	}
	proc, err := s.Sessions.Launch(sess, s.Exe, []string{HelperFlag, name})
	if err != nil {
		ln.Close()
		return nil, err
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	rctx, cancel := context.WithCancel(ctx)
	r := &running{sess: sess, proc: proc, cancel: cancel, started: now()}
	s.log().Info("helper started", "session", sess.ID, "user", sess.User, "pid", proc.PID())

	go func() {
		<-rctx.Done()
		ln.Close()
	}()
	go s.accept(rctx, ln, r)
	go func() {
		_ = proc.Wait(rctx)
		if rctx.Err() == nil {
			cancel()
			select {
			case exited <- r:
			case <-ctx.Done():
			}
		}
	}()
	return r, nil
}

// accept serves the helper's connection and refuses any other process.
func (s *Supervisor) accept(ctx context.Context, ln net.Listener, r *running) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		pid, err := s.PeerPID(conn)
		if err != nil || pid != r.proc.PID() {
			s.log().Warn("local channel: connection refused, not the helper", "pid", pid, "helper_pid", r.proc.PID(), "err", err)
			conn.Close()
			continue
		}
		s.Serve(ctx, r.sess, conn)
		// Without its channel the helper cannot work: end it, and it is
		// started again.
		_ = r.proc.Kill()
		return
	}
}

// channelName is unique per launch, so no other process can open it first.
func channelName(sess platform.Session) (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("channel name: %w", err)
	}
	return fmt.Sprintf("aula-agente-%d-%s", sess.ID, hex.EncodeToString(b)), nil
}

func (s *Supervisor) log() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.New(slog.DiscardHandler)
}
