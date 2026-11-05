package snapshot

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/jpeg"
	"log/slog"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// Defaults of Store.
const (
	DefaultTimeout = 30 * time.Second
	// DefaultMaxHeld bounds the screenshots kept in memory (a whole lab in
	// one batch, with room to spare). Each is at most 16 MiB, usually 1-3.
	DefaultMaxHeld = 64
)

// Status is where a screenshot is.
type Status string

// Statuses.
const (
	StatusPending Status = "pending" // asked, parts arriving
	StatusReady   Status = "ready"   // complete and checked, in memory
	StatusFailed  Status = "failed"  // see Reason
)

// Reasons a screenshot fails. The console turns each into an error code
// with its text (apps/teacher/errores.go).
const (
	ReasonTimeout     = "timeout"     // not complete in Timeout
	ReasonUnsupported = "unsupported" // unsupported_capability
	ReasonCapture     = "capture"     // the agent could not capture (locked session, secure desktop)
	ReasonBusy        = "busy"        // the agent was taking another one
	ReasonTooLarge    = "tooLarge"    // message_too_large
	ReasonCorrupt     = "corrupt"     // size, sha256 or JPEG do not match
	ReasonGone        = "gone"        // the student disconnected, or it could not be sent
)

// ErrTooMany is returned by Request when DefaultMaxHeld screenshots are
// already in memory: the teacher must save or discard some first.
var ErrTooMany = errors.New("too many screenshots in memory")

// ErrNotReady is returned by Save for a screenshot that is not complete.
var ErrNotReady = errors.New("screenshot not ready")

// Snapshot is what the console window sees of one screenshot: never the
// image, which it asks for with Image.
type Snapshot struct {
	ID          string    `json:"id"` // snapshot_id
	SessionID   string    `json:"sessionId"`
	Hostname    string    `json:"hostname"`
	Username    string    `json:"username"`
	Simulated   bool      `json:"simulated"`
	Status      Status    `json:"status"`
	Reason      string    `json:"reason,omitempty"`
	Width       int       `json:"width,omitempty"`
	Height      int       `json:"height,omitempty"`
	Size        int       `json:"size,omitempty"`
	RequestedAt time.Time `json:"requestedAt"`
	TakenAt     time.Time `json:"takenAt,omitzero"`
	Path        string    `json:"path,omitempty"` // where «Guardar» wrote it
}

type item struct {
	Snapshot
	envID string // the request's envelope ID, for error replies
	parts *protocol.Reassembly
	sum   string // sha256 announced in part 1
	jpeg  []byte // the image once ready; only in memory
	timer *time.Timer
}

// Store is the console side: one entry per requested screenshot, in the
// order they were asked for. Safe for concurrent use.
type Store struct {
	Timeout time.Duration // 0: DefaultTimeout
	MaxHeld int           // 0: DefaultMaxHeld
	Now     func() time.Time
	// OnChange receives the whole list whenever a screenshot changes
	// (also from the timeout's goroutine). Optional.
	OnChange func([]Snapshot)
	Logger   *slog.Logger

	mu    sync.Mutex
	items map[string]*item
	order []string
}

// Request registers a screenshot of a student and returns the message to
// send. Call Sent with the envelope ID once it is sent, or Unsent if it
// could not be.
func (s *Store) Request(sessionID, hostname, username string, simulated bool) (protocol.ScreenSnapshotRequest, Snapshot, error) {
	s.mu.Lock()
	if s.items == nil {
		s.items = map[string]*item{}
	}
	if len(s.items) >= s.maxHeld() {
		s.mu.Unlock()
		return protocol.ScreenSnapshotRequest{}, Snapshot{}, ErrTooMany
	}
	id := protocol.NewID()
	it := &item{Snapshot: Snapshot{ID: id, SessionID: sessionID, Hostname: hostname, Username: username,
		Simulated: simulated, Status: StatusPending, RequestedAt: s.now()}}
	s.items[id] = it
	s.order = append(s.order, id)
	it.timer = time.AfterFunc(s.timeout(), func() { s.expire(id) })
	snap := it.Snapshot
	s.mu.Unlock()
	s.log().Info("screenshot requested", "snapshot_id", id, "session", sessionID, "simulated", simulated)
	s.changed()
	return protocol.ScreenSnapshotRequest{SnapshotID: id, Quality: protocol.DefaultSnapshotQuality}, snap, nil
}

// Sent records the envelope ID of the request, which error replies refer to.
func (s *Store) Sent(id, envID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if it, ok := s.items[id]; ok {
		it.envID = envID
	}
}

// Unsent fails a request that could not be sent.
func (s *Store) Unsent(id string) {
	s.mu.Lock()
	it, ok := s.items[id]
	changed := ok && it.Status == StatusPending && s.failLocked(it, ReasonGone)
	s.mu.Unlock()
	if changed {
		s.changed()
	}
}

// Response takes one part from a student. Parts of a screenshot this
// console did not ask that student for are dropped.
func (s *Store) Response(sessionID string, p protocol.ScreenSnapshotResponse) {
	s.mu.Lock()
	it, ok := s.items[p.SnapshotID]
	if !ok || it.SessionID != sessionID || it.Status != StatusPending {
		s.mu.Unlock()
		s.log().Debug("screenshot part dropped", "snapshot_id", p.SnapshotID, "session", sessionID)
		return
	}
	if it.parts == nil {
		it.parts = protocol.NewReassembly(p.Parts)
	}
	if p.Part == 1 {
		it.Hostname, it.Username = p.Hostname, p.Username
		it.Width, it.Height, it.Size, it.sum = p.Width, p.Height, p.Size, p.SHA256
		it.TakenAt = time.UnixMilli(p.TakenAt)
	}
	if err := it.parts.Add(p.Part, p.Data); err != nil {
		s.mu.Unlock()
		s.log().Debug("screenshot part dropped", "snapshot_id", p.SnapshotID, "err", err)
		return
	}
	if !it.parts.Complete() {
		s.mu.Unlock()
		return
	}
	data := it.parts.Bytes()
	it.parts = nil
	if reason := check(data, it); reason != "" {
		s.failLocked(it, reason)
	} else {
		it.jpeg, it.Status = data, StatusReady
		it.timer.Stop()
		s.log().Info("screenshot received", "snapshot_id", it.ID, "bytes", len(data), "width", it.Width, "height", it.Height)
	}
	s.mu.Unlock()
	s.changed()
}

// check verifies a reassembled image against what part 1 announced.
func check(data []byte, it *item) string {
	if len(data) != it.Size || it.sum == "" {
		return ReasonCorrupt
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != it.sum {
		return ReasonCorrupt
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != it.Width || cfg.Height != it.Height {
		return ReasonCorrupt
	}
	return ""
}

// Error takes an error a student sent; it reports whether it answered one
// of this console's requests.
func (s *Store) Error(sessionID string, e protocol.ErrorPayload) bool {
	s.mu.Lock()
	var it *item
	for _, x := range s.items {
		if x.envID != "" && x.envID == e.RefID && x.SessionID == sessionID {
			it = x
			break
		}
	}
	changed := it != nil && it.Status == StatusPending && s.failLocked(it, reasonFor(e))
	s.mu.Unlock()
	if changed {
		s.changed()
	}
	return it != nil
}

func reasonFor(e protocol.ErrorPayload) string {
	switch e.Code {
	case protocol.CodeUnsupportedCapability, protocol.CodeUnknownType:
		return ReasonUnsupported
	case protocol.CodeMessageTooLarge:
		return ReasonTooLarge
	}
	if e.Message == busyMessage {
		return ReasonBusy
	}
	return ReasonCapture
}

// Gone fails the pending screenshots of students no longer connected.
// connected reports whether a session is still there.
func (s *Store) Gone(connected func(sessionID string) bool) {
	s.mu.Lock()
	changed := false
	for _, it := range s.items {
		if it.Status == StatusPending && !connected(it.SessionID) {
			changed = s.failLocked(it, ReasonGone) || changed
		}
	}
	s.mu.Unlock()
	if changed {
		s.changed()
	}
}

func (s *Store) expire(id string) {
	s.mu.Lock()
	it, ok := s.items[id]
	changed := ok && it.Status == StatusPending && s.failLocked(it, ReasonTimeout)
	s.mu.Unlock()
	if changed {
		s.changed()
	}
}

// failLocked marks it failed and drops whatever arrived. s.mu is held.
func (s *Store) failLocked(it *item, reason string) bool {
	it.Status, it.Reason = StatusFailed, reason
	it.parts, it.jpeg = nil, nil
	it.timer.Stop()
	s.log().Info("screenshot failed", "snapshot_id", it.ID, "session", it.SessionID, "reason", reason)
	return true
}

// Get returns one screenshot.
func (s *Store) Get(id string) (Snapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[id]
	if !ok {
		return Snapshot{}, false
	}
	return it.Snapshot, true
}

// Image returns the JPEG of a ready screenshot, for the preview.
func (s *Store) Image(id string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[id]
	if !ok || it.Status != StatusReady {
		return nil, false
	}
	return it.jpeg, true
}

// List returns every screenshot, oldest request first.
func (s *Store) List() []Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listLocked()
}

func (s *Store) listLocked() []Snapshot {
	out := make([]Snapshot, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.items[id].Snapshot)
	}
	return out
}

// Save writes a ready screenshot under root (the console's Aula folder in
// Documents) and returns it with its Path. It is the only way a screenshot
// reaches the disk (docs/privacidad.md rule 45). Saving it again returns the
// same path.
func (s *Store) Save(id, root string) (Snapshot, error) {
	s.mu.Lock()
	it, ok := s.items[id]
	if !ok || it.Status != StatusReady {
		s.mu.Unlock()
		return Snapshot{}, fmt.Errorf("save screenshot %s: %w", id, ErrNotReady)
	}
	if it.Path != "" {
		snap := it.Snapshot
		s.mu.Unlock()
		return snap, nil
	}
	snap, data := it.Snapshot, it.jpeg
	s.mu.Unlock()

	path, err := WriteFile(root, snap, data)
	if err != nil {
		s.log().Warn("screenshot not saved", "snapshot_id", id, "err", errors.Unwrap(err))
		return Snapshot{}, err
	}
	s.mu.Lock()
	if it, ok := s.items[id]; ok {
		it.Path = path
		snap = it.Snapshot
	}
	s.mu.Unlock()
	// The path names the student: it stays out of the log (rule 48).
	s.log().Info("screenshot saved", "snapshot_id", id, "bytes", len(data))
	s.changed()
	return snap, nil
}

// Discard forgets a screenshot and frees its image; it reports whether it
// existed. A saved file stays on disk.
func (s *Store) Discard(id string) bool {
	s.mu.Lock()
	it, ok := s.items[id]
	if ok {
		it.timer.Stop()
		delete(s.items, id)
		for i, x := range s.order {
			if x == id {
				s.order = append(s.order[:i:i], s.order[i+1:]...)
				break
			}
		}
	}
	s.mu.Unlock()
	if ok {
		s.log().Info("screenshot discarded", "snapshot_id", id, "saved", it.Path != "")
		s.changed()
	}
	return ok
}

// Reset forgets every screenshot (the class closed or changed channel):
// nothing unsaved survives.
func (s *Store) Reset() {
	s.mu.Lock()
	n := len(s.items)
	for _, it := range s.items {
		it.timer.Stop()
	}
	s.items, s.order = nil, nil
	s.mu.Unlock()
	if n > 0 {
		s.log().Info("screenshots dropped", "count", n)
		s.changed()
	}
}

func (s *Store) changed() {
	if s.OnChange == nil {
		return
	}
	s.OnChange(s.List())
}

func (s *Store) timeout() time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return DefaultTimeout
}

func (s *Store) maxHeld() int {
	if s.MaxHeld > 0 {
		return s.MaxHeld
	}
	return DefaultMaxHeld
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Store) log() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.New(slog.DiscardHandler)
}
