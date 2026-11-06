package simclass

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/participation"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/screen"
	"github.com/bayalad7/aula/internal/session"
	"github.com/bayalad7/aula/internal/snapshot"
	"github.com/bayalad7/aula/internal/transport"
)

// MaxStudents caps a simulated class: enough to see a full lab, little enough
// to keep the console light.
const MaxStudents = 40

// MaxDelay bounds the "slow" fault. It stays under the protocol's 15 s of
// silence so a slow student is late, not disconnected.
const MaxDelay = 10 * time.Second

// Fault is a failure a simulated student reproduces on demand.
type Fault string

// Faults. They only produce states the console already handles.
const (
	FaultNone         Fault = "none"
	FaultSlow         Fault = "slow"         // answers after Delay
	FaultNoAck        Fault = "noack"        // shows what it gets but never acknowledges it
	FaultDisconnected Fault = "disconnected" // closes its session and stays away
)

// Capabilities a simulated student may declare: what it can honour without an
// OS. Power, restrictions, broadcast and remote control are left out on
// purpose: the console never sends them to simulated students. A screenshot
// changes no computer: it is its drawn screen, marked as simulated, and the
// console files it apart (docs/privacidad.md rule 46). Raising a hand and
// writing to the teacher start on the student's side: the teacher makes a
// simulated student do it (Raise, Write) to try «Participación» without a lab.
var Capabilities = []protocol.Capability{protocol.CapCodeView, protocol.CapMsgShow, protocol.CapScreenThumb, protocol.CapScreenSnapshot,
	protocol.CapHand, protocol.CapMsgStudent}

// DefaultCapabilities are those of a new student when none are given.
var DefaultCapabilities = Capabilities

// Errors.
var (
	ErrFull            = errors.New("simclass: too many simulated students")
	ErrUnknownStudent  = errors.New("simclass: no simulated student with that ID")
	ErrBadCapability   = errors.New("simclass: capability not available to simulated students")
	ErrBadFault        = errors.New("simclass: unknown fault")
	ErrNotConnected    = errors.New("simclass: the simulated student is not connected")
	ErrClosed          = errors.New("simclass: class closed")
	errConsoleRequired = errors.New("simclass: console address, channel, key and fingerprint are required")
)

// Console is where the simulated students connect: the console's loopback
// listener for simulated sessions.
type Console struct {
	Addr            *net.TCPAddr // 127.0.0.1:<port>
	Channel         string
	Name            string
	PublicKey       ed25519.PublicKey
	CertFingerprint string // "sha256:<hex>"
}

// Snippet is what a student has on screen, without the code itself.
type Snippet struct {
	Exercise string `json:"exercise"`
	Title    string `json:"title"`
	Lang     string `json:"lang"`
	Lines    int    `json:"lines"`
}

// Student is a snapshot of one simulated student.
type Student struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"` // also its hostname
	OS           protocol.OS           `json:"os"`
	Capabilities []protocol.Capability `json:"capabilities"`
	Connected    bool                  `json:"connected"`
	Fault        Fault                 `json:"fault"`
	DelayMs      int                   `json:"delayMs"`
	Viewed       bool                  `json:"viewed"` // the teacher is receiving its screen
	Snippets     []Snippet             `json:"snippets"`
	Notices      int                   `json:"notices"`
	Private      int                   `json:"private"` // of Notices, those sent to this student only (msg.show personal)
	LastNotice   string                `json:"lastNotice"`
	Snapshots    int                   `json:"snapshots"` // screenshots the teacher took of it
	// Hand is its hand (participation.HandState) and Sent how many messages
	// it wrote to the teacher.
	Hand participation.HandState `json:"hand"`
	Sent int                     `json:"sent"`
}

// Class is a set of simulated students connected to one console. It is safe
// for concurrent use.
type Class struct {
	console  Console
	version  string // agent_version they declare
	onChange func()
	log      *slog.Logger

	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	students []*student
	next     int
	closed   bool
}

// New returns an empty class. version is the console's SemVer; students
// declare it with "+sim". onChange, if set, is called after every change
// (connection, code received, fault…); keep it short.
func New(c Console, version string, onChange func(), log *slog.Logger) (*Class, error) {
	if c.Addr == nil || c.Channel == "" || len(c.PublicKey) != ed25519.PublicKeySize || c.CertFingerprint == "" {
		return nil, errConsoleRequired
	}
	if log == nil {
		log = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Class{console: c, version: version, onChange: onChange, log: log.With("simulated", true), ctx: ctx, cancel: cancel}, nil
}

// Add starts a new simulated student with the given OS and capabilities (nil
// means DefaultCapabilities) and returns its snapshot.
func (c *Class) Add(os protocol.OS, caps []protocol.Capability) (Student, error) {
	if os == "" {
		os = protocol.OSWindows
	}
	if caps == nil {
		caps = DefaultCapabilities
	}
	for _, k := range caps {
		if !slices.Contains(Capabilities, k) {
			return Student{}, fmt.Errorf("%w: %s", ErrBadCapability, k)
		}
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return Student{}, ErrClosed
	}
	if len(c.students) >= MaxStudents {
		c.mu.Unlock()
		return Student{}, ErrFull
	}
	c.next++
	s := &student{
		class: c, id: protocol.NewID(), name: "Simulado " + strconv.Itoa(c.next), os: os,
		caps: slices.Clone(caps), index: c.next, fault: FaultNone, board: codedist.NewBoard(0),
		desk: participation.NewDesk(),
	}
	if err := s.hello().Validate(); err != nil {
		c.mu.Unlock()
		return Student{}, fmt.Errorf("simclass: %w", err)
	}
	c.students = append(c.students, s)
	c.mu.Unlock()
	s.start()
	c.changed()
	return s.snapshot(), nil
}

// Remove stops a student and forgets it.
func (c *Class) Remove(id string) error {
	c.mu.Lock()
	i := slices.IndexFunc(c.students, func(s *student) bool { return s.id == id })
	if i < 0 {
		c.mu.Unlock()
		return ErrUnknownStudent
	}
	s := c.students[i]
	c.students = slices.Delete(c.students, i, i+1)
	c.mu.Unlock()
	s.stop()
	c.changed()
	return nil
}

// SetFault makes a student reproduce a fault; FaultNone ends it (a
// disconnected student connects again). delay is used by FaultSlow and is
// clamped to (0, MaxDelay].
func (c *Class) SetFault(id string, f Fault, delay time.Duration) error {
	switch f {
	case FaultNone, FaultSlow, FaultNoAck, FaultDisconnected:
	default:
		return fmt.Errorf("%w: %q", ErrBadFault, f)
	}
	s := c.find(id)
	if s == nil {
		return ErrUnknownStudent
	}
	if f == FaultSlow {
		delay = min(max(delay, 500*time.Millisecond), MaxDelay)
	}
	s.setFault(f, delay)
	c.changed()
	return nil
}

// Students returns every student, in creation order.
func (c *Class) Students() []Student {
	c.mu.Lock()
	list := slices.Clone(c.students)
	c.mu.Unlock()
	out := make([]Student, 0, len(list))
	for _, s := range list {
		out = append(out, s.snapshot())
	}
	return out
}

// Close stops every student. The class cannot be used afterwards.
func (c *Class) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	list := c.students
	c.students = nil
	c.mu.Unlock()
	c.cancel()
	for _, s := range list {
		s.stop()
	}
}

func (c *Class) find(id string) *student {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.students {
		if s.id == id {
			return s
		}
	}
	return nil
}

func (c *Class) changed() {
	if c.onChange != nil {
		c.onChange()
	}
}

// student is one simulated agent.
type student struct {
	class *Class
	id    string
	name  string
	os    protocol.OS
	caps  []protocol.Capability
	index int
	board *codedist.Board
	desk  *participation.Desk // its hand and messages to the teacher

	mu         sync.Mutex
	cl         *transport.Client // the connection, nil while not connected
	sent       int
	fault      Fault
	delay      time.Duration
	connected  bool
	viewed     bool
	notices    int
	private    int
	lastNotice string
	snapshots  int
	cancel     context.CancelFunc // stops the running agent; nil while disconnected
	done       chan struct{}
	streamer   *screen.Streamer
	taker      *snapshot.Taker
}

func (s *student) hello() protocol.HelloInfo {
	return protocol.HelloInfo{
		Hostname: s.name, Username: "Alumno de prueba", OS: s.os, OSVersion: "simulado",
		AgentVersion: s.class.version + "+sim", Capabilities: s.caps,
	}
}

// start runs the agent until stop or a "disconnected" fault.
func (s *student) start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(s.class.ctx)
	done := make(chan struct{})
	s.cancel, s.done = cancel, done
	log := s.class.log.With("student", s.name)
	s.streamer = &screen.Streamer{Logger: log}
	if slices.Contains(s.caps, protocol.CapScreenThumb) || slices.Contains(s.caps, protocol.CapScreenSnapshot) {
		s.streamer.Capture = drawnScreen{s}
		s.streamer.Indicator = viewedFlag{s}
	}
	s.taker = &snapshot.Taker{Indicator: s.streamer, Settle: -1, Logger: log,
		Identity: func() (string, string) { h := s.hello(); return h.Hostname, h.Username },
		OnTaken:  func(time.Time) { s.tookSnapshot() }}
	if slices.Contains(s.caps, protocol.CapScreenSnapshot) {
		s.taker.Capture = drawnScreen{s}
	}
	recv := &codedist.Receiver{
		Board:    s.board,
		OnBoard:  func([]codedist.Snippet) { s.class.changed() },
		OnNotice: s.noticed,
	}
	con := s.class.console
	agent := &session.Agent{
		Channel:          con.Channel,
		ConsolePublicKey: con.PublicKey,
		Hello:            s.hello(),
		Source: session.StaticSource{{
			Instance: "aula-simulada", Channel: con.Channel, Name: con.Name,
			Addrs: []net.IP{con.Addr.IP}, Port: con.Addr.Port, CertFingerprint: con.CertFingerprint,
		}},
		Backoff:   session.Backoff{Base: 200 * time.Millisecond, Max: 2 * time.Second, Jitter: 0.1},
		Logger:    log,
		OnMessage: func(ctx context.Context, cl *transport.Client, env protocol.Envelope) { s.handle(ctx, cl, recv, env) },
		OnConnect: func(ctx context.Context, cl *transport.Client) {
			s.mu.Lock()
			s.cl = cl
			s.mu.Unlock()
			for _, p := range s.desk.Reconnected() {
				s.send(ctx, cl, p)
			}
		},
		OnState: func(st session.AgentState, _ discovery.Console) {
			s.mu.Lock()
			s.connected = st == session.AgentConnected
			if !s.connected {
				s.cl = nil
			}
			s.mu.Unlock()
			if st != session.AgentConnected {
				s.streamer.Stop() // the stream ends with the session (§5.6)
			}
			s.class.changed()
		},
	}
	go func() {
		defer close(done)
		if err := agent.Run(ctx); err != nil {
			log.Warn("simulated agent stopped", "err", err)
		}
	}()
}

// stop ends the agent and waits for it.
func (s *student) stop() {
	s.mu.Lock()
	cancel, done, st := s.cancel, s.done, s.streamer
	s.cancel, s.done = nil, nil
	s.connected = false
	s.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
	if st != nil {
		st.Stop()
	}
}

func (s *student) setFault(f Fault, delay time.Duration) {
	s.mu.Lock()
	s.fault, s.delay = f, delay
	running := s.cancel != nil
	s.mu.Unlock()
	switch {
	case f == FaultDisconnected && running:
		s.stop()
	case f != FaultDisconnected && !running:
		s.start()
	}
}

// handle answers one message from the console like the real app, applying
// the student's fault.
func (s *student) handle(ctx context.Context, cl *transport.Client, recv *codedist.Receiver, env protocol.Envelope) {
	s.mu.Lock()
	fault, delay := s.fault, s.delay
	s.mu.Unlock()
	if s.handleParticipation(env) {
		return
	}
	run := func() {
		send := func(ctx context.Context, p protocol.Payload) error {
			_, err := cl.Send(ctx, p)
			return err
		}
		reply, handled := s.taker.Handle(ctx, snapshot.Sender(send), env)
		if !handled {
			reply, handled = s.streamer.Handle(ctx, send, env)
		}
		if !handled {
			reply = recv.Handle(env)
		}
		if reply == nil {
			return
		}
		if _, isAck := reply.(protocol.CodeAck); isAck && fault == FaultNoAck {
			return
		}
		if _, isAck := reply.(protocol.MsgAck); isAck && fault == FaultNoAck {
			return
		}
		if err := send(ctx, reply); err != nil {
			s.class.log.Debug("cannot reply", "student", s.name, "type", reply.MessageType(), "err", err)
		}
	}
	if fault == FaultSlow {
		// Later, without blocking the session: heartbeats keep flowing.
		time.AfterFunc(delay, func() {
			if ctx.Err() == nil {
				run()
			}
		})
		return
	}
	run()
}

func (s *student) noticed(n codedist.Notice) {
	s.mu.Lock()
	s.notices++
	if n.Personal {
		s.private++
	}
	s.lastNotice = n.Text
	s.mu.Unlock()
	s.class.changed()
}

func (s *student) tookSnapshot() {
	s.mu.Lock()
	s.snapshots++
	s.mu.Unlock()
	s.class.changed()
}

func (s *student) setViewed(v bool) {
	s.mu.Lock()
	changed := s.viewed != v
	s.viewed = v
	s.mu.Unlock()
	if changed {
		s.class.changed()
	}
}

func (s *student) snapshot() Student {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Student{
		ID: s.id, Name: s.name, OS: s.os, Capabilities: slices.Clone(s.caps), Connected: s.connected,
		Fault: s.fault, Viewed: s.viewed, Notices: s.notices, Private: s.private, LastNotice: s.lastNotice, Snapshots: s.snapshots, Snippets: []Snippet{},
		Hand: s.desk.View().Hand, Sent: s.sent,
	}
	if s.fault == FaultSlow {
		out.DelayMs = int(s.delay / time.Millisecond)
	}
	for _, sn := range s.board.Snippets() {
		out.Snippets = append(out.Snippets, Snippet{Exercise: sn.Exercise, Title: sn.Title, Lang: sn.Lang, Lines: lines(sn.Content)})
	}
	return out
}

func lines(s string) int {
	if s == "" {
		return 0
	}
	n := 1
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '\n' {
			n++
		}
	}
	return n
}
