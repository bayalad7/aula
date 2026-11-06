package collect

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// Limits and waits of the console (docs/protocol.md §5.21).
const (
	DefaultParallel  = 4                // students asked at once
	DefaultReplyWait = 30 * time.Second // for the ack of a request
	DefaultFileWait  = 60 * time.Second // between parts of the round
	maxOpen          = 10               // collections of the class
)

// Reasons of the console, besides the protocol's.
const (
	ReasonTimeout = "timeout" // no answer in time
	ReasonGone    = "gone"    // the student left during the round
	ReasonSave    = "save"    // the teacher's disk refused the round
)

// DefaultLimits is what «Recoger» asks for.
var DefaultLimits = Limits{MaxFileSize: 4 << 20, MaxTotal: 16 << 20, MaxFiles: 20}

// Limits of one round, per student.
type Limits struct {
	MaxFileSize int `json:"maxFileSize"`
	MaxTotal    int `json:"maxTotal"`
	MaxFiles    int `json:"maxFiles"`
}

// SendFunc sends p to one session and returns the envelope ID.
type SendFunc func(ctx context.Context, sessionID string, p protocol.Payload) (string, error)

// Student is a student the console can ask: its session and who it is.
type Student struct {
	Session   string `json:"session"`
	Hostname  string `json:"hostname"`
	Username  string `json:"username"`
	Simulated bool   `json:"simulated"`
}

// Key names a student across reconnections.
func (s Student) Key() string {
	k := strings.ToLower(s.Hostname) + "|" + strings.ToLower(s.Username)
	if s.Simulated {
		k += "|sim"
	}
	return k
}

// Status is where a student stands in a collection.
type Status string

// Student statuses.
const (
	StatusWaiting    Status = "waiting"    // open sent, no answer yet
	StatusReady      Status = "ready"      // the folder is on the student's computer
	StatusQueued     Status = "queued"     // «Recoger»: waiting for its turn
	StatusCollecting Status = "collecting" // asked: files arriving
	StatusCollected  Status = "collected"  // the round is saved
	StatusEmpty      Status = "empty"      // nothing to collect
	StatusFailed     Status = "failed"     // see Reason
)

// StudentResult is one student in a collection: what arrived, when and
// where it is.
type StudentResult struct {
	Student
	Key     string         `json:"key"`
	Status  Status         `json:"status"`
	Reason  string         `json:"reason,omitempty"` // failed: a protocol reason or Reason*
	Round   int            `json:"round"`            // of the last answer
	At      time.Time      `json:"at"`               // when it ended
	Files   []ManifestFile `json:"files"`            // of the last saved round
	Failed  int            `json:"failedFiles"`      // files of the last round not saved
	Skipped []SkippedFile  `json:"skipped"`
	Dir     string         `json:"dir"` // the teacher's folder of this student
}

// Summary is one collection for the console's view.
type Summary struct {
	ID         string          `json:"id"`
	Date       string          `json:"date"`
	Activity   string          `json:"activity"`
	Patterns   []string        `json:"patterns"`
	Limits     Limits          `json:"limits"`
	OpenedAt   time.Time       `json:"openedAt"`
	Round      int             `json:"round"`
	Collecting bool            `json:"collecting"`
	Dir        string          `json:"dir"` // root\<date>\<activity>
	Students   []StudentResult `json:"students"`
}

// Collector is the console side of file.collect. It is safe for concurrent
// use.
type Collector struct {
	Send SendFunc
	// Root is the teacher's folder of every collection (Root(documents)).
	Root func() (string, error)
	// OnChange runs after any change of a collection, from any goroutine.
	OnChange func(id string)

	Parallel  int           // 0: DefaultParallel
	ReplyWait time.Duration // 0: DefaultReplyWait
	FileWait  time.Duration // 0: DefaultFileWait
	Now       func() time.Time
	Logger    *slog.Logger

	mu    sync.Mutex
	cols  map[string]*collection
	order []string // oldest first
	wg    sync.WaitGroup
}

type collection struct {
	sum      Summary
	students map[string]*student // by key
	keys     []string
	running  int
}

type student struct {
	res StudentResult
	run *run // the round in course
}

// run is one round of one student on its way.
type run struct {
	round    int
	request  string // envelope ID of the request
	ack      chan protocol.FileCollectAck
	failed   chan string
	progress chan struct{}
	done     chan struct{}
	expect   int
	resolved int
	stage    *staging
	asm      map[string]*fileAsm
	files    []ManifestFile
	bad      int
	cancel   context.CancelFunc
}

type fileAsm struct {
	head  protocol.FileCollectResponse
	parts *protocol.Reassembly
}

// Open starts a collection for date and activity and sends file.collect.open
// to the students. It creates the teacher's activity folder.
func (c *Collector) Open(ctx context.Context, date, activity string, patterns []string, limits Limits, students []Student) (string, error) {
	if _, err := Folder(date, activity); err != nil {
		return "", err
	}
	if err := CheckPatterns(patterns); err != nil {
		return "", err
	}
	if limits.MaxFileSize < 1 || limits.MaxFileSize > protocol.MaxFileCollectBytes ||
		limits.MaxTotal < 1 || limits.MaxTotal > protocol.MaxFileCollectTotal ||
		limits.MaxFiles < 1 || limits.MaxFiles > protocol.MaxFileCollectFiles {
		return "", fmt.Errorf("%w: limits", ErrInvalid)
	}
	root, err := c.Root()
	if err != nil {
		return "", fmt.Errorf("collect: teacher folder: %w", err)
	}
	dir := ActivityDir(root, date, activity)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("collect: teacher folder: %w", err)
	}
	col := &collection{students: map[string]*student{}, sum: Summary{ID: protocol.NewID(), Date: date,
		Activity: activity, Patterns: append([]string(nil), patterns...), Limits: limits, OpenedAt: c.now(), Dir: dir}}
	c.mu.Lock()
	if c.cols == nil {
		c.cols = map[string]*collection{}
	}
	c.cols[col.sum.ID] = col
	c.order = append(c.order, col.sum.ID)
	if len(c.order) > maxOpen {
		old := c.order[0]
		c.order = c.order[1:]
		delete(c.cols, old)
	}
	c.mu.Unlock()
	c.log().Info("collection opened", "collect_id", col.sum.ID, "students", len(students))
	for _, s := range students {
		c.sendOpen(ctx, col, s)
	}
	c.changed(col.sum.ID)
	return col.sum.ID, nil
}

func (c *Collector) sendOpen(ctx context.Context, col *collection, s Student) {
	c.mu.Lock()
	st := c.upsert(col, s)
	if st.res.Status == "" || st.res.Status == StatusFailed && st.res.Round == 0 {
		st.res.Status, st.res.Reason = StatusWaiting, ""
	}
	open := protocol.FileCollectOpen{CollectID: col.sum.ID, Date: col.sum.Date, Activity: col.sum.Activity, Patterns: col.sum.Patterns}
	c.mu.Unlock()
	if _, err := c.Send(ctx, s.Session, open); err != nil {
		c.mu.Lock()
		if st.res.Status == StatusWaiting {
			st.res.Status, st.res.Reason = StatusFailed, ReasonGone
		}
		c.mu.Unlock()
		c.log().Info("collection open not sent", "collect_id", col.sum.ID, "conn", s.Session, "err", err)
	}
}

// upsert finds or adds a student and takes its current session; the caller
// holds c.mu.
func (c *Collector) upsert(col *collection, s Student) *student {
	key := s.Key()
	st := col.students[key]
	if st == nil {
		st = &student{res: StudentResult{Key: key, Dir: StudentDir(col.sum.Dir, s)}}
		col.students[key] = st
		col.keys = append(col.keys, key)
	}
	st.res.Student = s
	return st
}

// Reopen sends the open collections to a student that just connected (late,
// or again), so its folder exists too.
func (c *Collector) Reopen(ctx context.Context, s Student) {
	c.mu.Lock()
	cols := make([]*collection, 0, len(c.order))
	for _, id := range c.order {
		cols = append(cols, c.cols[id])
	}
	c.mu.Unlock()
	for _, col := range cols {
		c.sendOpen(ctx, col, s)
		c.changed(col.sum.ID)
	}
}

// Collect asks the students for a new round of the collection, at most
// Parallel at a time. It returns at once; OnChange tells the progress.
func (c *Collector) Collect(ctx context.Context, id string, students []Student) error {
	c.mu.Lock()
	col := c.cols[id]
	if col == nil {
		c.mu.Unlock()
		return fmt.Errorf("collect: unknown collection %s", id)
	}
	var queue []*student
	for _, s := range students {
		st := c.upsert(col, s)
		if st.run != nil {
			continue // still on its way
		}
		st.res.Status, st.res.Reason = StatusQueued, ""
		queue = append(queue, st)
	}
	if len(queue) == 0 {
		c.mu.Unlock()
		return nil
	}
	if col.sum.Round >= protocol.MaxFileCollectRound {
		c.mu.Unlock()
		return fmt.Errorf("%w: too many rounds", ErrInvalid)
	}
	col.sum.Round++
	round := col.sum.Round
	col.running += len(queue)
	col.sum.Collecting = true
	c.mu.Unlock()
	c.log().Info("collection round started", "collect_id", id, "round", round, "students", len(queue))
	c.changed(id)
	slots := make(chan struct{}, c.parallel())
	c.wg.Add(len(queue))
	for _, st := range queue {
		go func() {
			defer c.wg.Done()
			select {
			case slots <- struct{}{}:
				c.collectFrom(ctx, col, st, round)
				<-slots
			case <-ctx.Done():
				c.collectFrom(ctx, col, st, round) // ends at once as gone
			}
			c.mu.Lock()
			col.running--
			col.sum.Collecting = col.running > 0
			c.mu.Unlock()
			c.changed(id)
		}()
	}
	return nil
}

func (c *Collector) collectFrom(ctx context.Context, col *collection, st *student, round int) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	r := &run{round: round, ack: make(chan protocol.FileCollectAck, 1), failed: make(chan string, 1),
		progress: make(chan struct{}, 1), done: make(chan struct{}), asm: map[string]*fileAsm{}, cancel: cancel}
	c.mu.Lock()
	st.run = r
	st.res.Status = StatusCollecting
	s := st.res.Student
	sum := col.sum
	req := protocol.FileCollectRequest{CollectID: sum.ID, Date: sum.Date, Activity: sum.Activity, Round: round,
		Patterns: sum.Patterns, MaxFileSize: sum.Limits.MaxFileSize, MaxTotal: sum.Limits.MaxTotal, MaxFiles: sum.Limits.MaxFiles}
	c.mu.Unlock()
	c.changed(sum.ID)
	log := c.log().With("collect_id", sum.ID, "round", round, "conn", s.Session)

	end := func(status Status, reason string) {
		c.mu.Lock()
		if r.stage != nil && status != StatusCollected {
			r.stage.discard()
		}
		st.run = nil
		st.res.Status, st.res.Reason, st.res.Round, st.res.At = status, reason, round, c.now()
		c.mu.Unlock()
		log.Info("collection round ended", "status", status, "reason", reason)
	}

	if ctx.Err() != nil {
		end(StatusFailed, ReasonGone)
		return
	}
	ref, err := c.Send(ctx, s.Session, req)
	if err != nil {
		end(StatusFailed, ReasonGone)
		return
	}
	c.mu.Lock()
	r.request = ref
	c.mu.Unlock()

	var ack protocol.FileCollectAck
	t := time.NewTimer(c.replyWait())
	select {
	case ack = <-r.ack:
	case reason := <-r.failed:
		t.Stop()
		end(StatusFailed, reason)
		return
	case <-t.C:
		end(StatusFailed, ReasonTimeout)
		return
	case <-ctx.Done():
		t.Stop()
		end(StatusFailed, ReasonGone)
		return
	}
	t.Stop()
	skipped := make([]SkippedFile, 0, len(ack.Skipped))
	for _, sk := range ack.Skipped {
		skipped = append(skipped, SkippedFile{Path: sk.Path, Size: sk.Size, Reason: string(sk.Reason)})
	}
	c.mu.Lock()
	st.res.Skipped = skipped
	c.mu.Unlock()
	switch ack.Status {
	case protocol.FileCollectFailed:
		end(StatusFailed, string(ack.Reason))
		return
	case protocol.FileCollectEmpty:
		end(StatusEmpty, "")
		return
	case protocol.FileCollectSending:
	default:
		end(StatusFailed, string(protocol.CollectInternal))
		return
	}
	c.mu.Lock()
	stage := r.stage // set by Ack
	c.mu.Unlock()
	c.changed(sum.ID)

	wait := time.NewTimer(c.fileWait())
	defer wait.Stop()
	for done := false; !done; {
		select {
		case <-r.done:
			done = true
		case <-r.progress:
			if !wait.Stop() {
				<-wait.C
			}
			wait.Reset(c.fileWait())
		case <-wait.C:
			end(StatusFailed, ReasonTimeout)
			return
		case <-ctx.Done():
			end(StatusFailed, ReasonGone)
			return
		}
	}
	c.mu.Lock()
	files := append([]ManifestFile(nil), r.files...)
	bad := r.bad
	c.mu.Unlock()
	if len(files) == 0 {
		end(StatusFailed, string(protocol.CollectCorrupt))
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	m := Manifest{CollectID: sum.ID, Date: sum.Date, Activity: sum.Activity, Round: round,
		Hostname: s.Hostname, Username: s.Username, Simulated: s.Simulated, At: c.now(), Files: files, Skipped: skipped}
	if err := stage.commit(m); err != nil {
		log.Warn("collection not saved", "err", err)
		end(StatusFailed, ReasonSave)
		return
	}
	c.mu.Lock()
	st.res.Files, st.res.Failed = files, bad
	c.mu.Unlock()
	log.Info("collection round saved", "files", len(files), "failed", bad)
	end(StatusCollected, "")
}

// find returns the round in course of a session in a collection; the
// caller holds c.mu.
func (c *Collector) find(id, session string) (*collection, *student) {
	col := c.cols[id]
	if col == nil {
		return nil, nil
	}
	for _, st := range col.students {
		if st.res.Session == session {
			return col, st
		}
	}
	return col, nil
}

// Ack takes a file.collect.ack of session. It reports whether it belonged to
// a collection.
func (c *Collector) Ack(session string, ack protocol.FileCollectAck) bool {
	c.mu.Lock()
	col, st := c.find(ack.CollectID, session)
	if st == nil {
		c.mu.Unlock()
		return false
	}
	if ack.Round == 0 {
		if st.run == nil && (st.res.Status == StatusWaiting || st.res.Status == StatusFailed && st.res.Round == 0) {
			if ack.Status == protocol.FileCollectReady {
				st.res.Status, st.res.Reason = StatusReady, ""
			} else {
				st.res.Status, st.res.Reason = StatusFailed, string(ack.Reason)
			}
		}
		c.mu.Unlock()
		c.changed(col.sum.ID)
		return true
	}
	r := st.run
	if r == nil || r.round != ack.Round || r.stage != nil {
		c.mu.Unlock()
		return true
	}
	if ack.Status == protocol.FileCollectSending {
		// The parts follow this ack right away: the staging must be ready
		// before Ack returns.
		dir := st.res.Dir
		c.mu.Unlock()
		stage, err := newStaging(dir)
		c.mu.Lock()
		if err != nil {
			c.log().Warn("collection not saved", "collect_id", ack.CollectID, "err", err)
			select {
			case r.failed <- ReasonSave:
			default:
			}
			c.mu.Unlock()
			return true
		}
		if st.run != r {
			stage.discard()
			c.mu.Unlock()
			return true
		}
		r.stage, r.expect = stage, ack.Files
	}
	select {
	case r.ack <- ack:
	default:
	}
	c.mu.Unlock()
	return true
}

// Error takes an error of session; it reports whether it answered a request.
func (c *Collector) Error(session string, e protocol.ErrorPayload) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, col := range c.cols {
		for _, st := range col.students {
			if st.res.Session == session && st.run != nil && st.run.request == e.RefID && e.RefID != "" {
				select {
				case st.run.failed <- string(e.Code):
				default:
				}
				return true
			}
		}
	}
	return false
}

// Response takes one part of a collected file from session and returns the
// file.collect.response.ack to send back (envID is the part's envelope ID),
// or nil while the file is incomplete or when the part is not expected.
func (c *Collector) Response(session, envID string, p protocol.FileCollectResponse) protocol.Payload {
	c.mu.Lock()
	_, st := c.find(p.CollectID, session)
	if st == nil || st.run == nil || st.run.round != p.Round || st.run.stage == nil {
		c.mu.Unlock()
		return nil
	}
	r := st.run
	reply := func(reason protocol.FileCollectReason) protocol.Payload {
		ack := protocol.FileCollectResponseAck{RefID: envID, CollectID: p.CollectID, Round: p.Round, FileID: p.FileID,
			Status: protocol.FilePushSaved}
		if reason != "" {
			ack.Status, ack.Reason = protocol.FilePushFailed, reason
		}
		return ack
	}
	resolve := func(f *ManifestFile) {
		// The caller holds c.mu.
		if f != nil {
			r.files = append(r.files, *f)
		} else {
			r.bad++
		}
		r.resolved++
		if r.resolved == r.expect {
			close(r.done)
		}
	}
	a := r.asm[p.FileID]
	if a == nil {
		if p.Part != 1 || r.resolved+len(r.asm) >= r.expect {
			c.mu.Unlock()
			return nil
		}
		if CheckPath(p.Path) != nil {
			resolve(nil)
			c.mu.Unlock()
			return reply(protocol.CollectDenied)
		}
		head := p
		head.Data = nil
		a = &fileAsm{head: head, parts: protocol.NewReassembly(p.Parts)}
		r.asm[p.FileID] = a
	}
	if err := a.parts.Add(p.Part, p.Data); err != nil {
		c.mu.Unlock()
		return nil // repeated or out of range: dropped
	}
	select {
	case r.progress <- struct{}{}:
	default:
	}
	if !a.parts.Complete() {
		c.mu.Unlock()
		return nil
	}
	delete(r.asm, p.FileID)
	data := a.parts.Bytes()
	stage := r.stage
	c.mu.Unlock()

	sum := sha256.Sum256(data)
	var reason protocol.FileCollectReason
	switch {
	case len(data) != a.head.Size || hex.EncodeToString(sum[:]) != a.head.SHA256:
		reason = protocol.CollectCorrupt
	default:
		if err := stage.write(a.head.Path, data); err != nil {
			reason = saveReason(err)
			c.log().Warn("collected file not saved", "collect_id", p.CollectID, "file_id", p.FileID, "err", err)
		}
	}
	c.mu.Lock()
	if st.run == r {
		if reason == "" {
			f := ManifestFile{Path: a.head.Path, Size: len(data), SHA256: a.head.SHA256}
			if a.head.ModifiedAt > 0 {
				f.ModifiedAt = time.UnixMilli(a.head.ModifiedAt).UTC()
			}
			resolve(&f)
		} else {
			resolve(nil)
		}
	}
	c.mu.Unlock()
	return reply(reason)
}

func saveReason(err error) protocol.FileCollectReason {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return protocol.CollectDenied
	case strings.Contains(strings.ToLower(err.Error()), "space"):
		return protocol.CollectNoSpace
	}
	return protocol.CollectInternal
}

// Gone abandons the rounds in course of a session that closed.
func (c *Collector) Gone(session string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, col := range c.cols {
		for _, st := range col.students {
			if st.res.Session == session && st.run != nil {
				st.run.cancel()
			}
		}
	}
}

// List returns every collection of the class, newest first.
func (c *Collector) List() []Summary {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Summary, 0, len(c.order))
	for i := len(c.order) - 1; i >= 0; i-- {
		out = append(out, c.summary(c.cols[c.order[i]]))
	}
	return out
}

// Get returns one collection.
func (c *Collector) Get(id string) (Summary, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	col := c.cols[id]
	if col == nil {
		return Summary{}, false
	}
	return c.summary(col), true
}

// StudentDir returns the teacher's folder of one student of a collection,
// for «Abrir».
func (c *Collector) StudentDir(id, key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if col := c.cols[id]; col != nil {
		if st := col.students[key]; st != nil {
			return st.res.Dir, true
		}
	}
	return "", false
}

func (c *Collector) summary(col *collection) Summary {
	s := col.sum
	s.Patterns = append([]string(nil), s.Patterns...)
	s.Students = make([]StudentResult, 0, len(col.keys))
	for _, k := range col.keys {
		r := col.students[k].res
		r.Files = append([]ManifestFile(nil), r.Files...)
		r.Skipped = append([]SkippedFile(nil), r.Skipped...)
		s.Students = append(s.Students, r)
	}
	return s
}

// Forget drops every collection (the class closed or the channel changed).
// The folders stay; rounds in course are abandoned.
func (c *Collector) Forget() {
	c.mu.Lock()
	for _, col := range c.cols {
		for _, st := range col.students {
			if st.run != nil {
				st.run.cancel()
			}
		}
	}
	c.cols, c.order = nil, nil
	c.mu.Unlock()
}

// Wait blocks until every round in course ends.
func (c *Collector) Wait() { c.wg.Wait() }

func (c *Collector) changed(id string) {
	if c.OnChange != nil {
		c.OnChange(id)
	}
}

func (c *Collector) parallel() int {
	if c.Parallel > 0 {
		return c.Parallel
	}
	return DefaultParallel
}

func (c *Collector) replyWait() time.Duration {
	if c.ReplyWait > 0 {
		return c.ReplyWait
	}
	return DefaultReplyWait
}

func (c *Collector) fileWait() time.Duration {
	if c.FileWait > 0 {
		return c.FileWait
	}
	return DefaultFileWait
}

func (c *Collector) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Collector) log() *slog.Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return slog.Default()
}
