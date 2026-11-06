package collect

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"log/slog"
	"path"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// Limits of the agent.
const (
	DefaultAckWait   = 60 * time.Second // for the console's ack of one file
	maxCollections   = 20               // kept for the window
	maxRoundsPerColl = 20
)

// Sender sends one message on the current connection to the console.
type Sender func(ctx context.Context, p protocol.Payload) error

// State is where a collection stands for the student's window.
type State string

// Collection states.
const (
	StateOpen      State = "open"      // the folder is ready: save your work there
	StateRequested State = "requested" // the teacher asked: being read and sent
	StateCollected State = "collected" // the last round ended
)

// Event says what just changed, so the window picks its notice.
type Event string

// Events of OnChange.
const (
	EventOpened    Event = "opened"    // a new collection: card with «Nuevo»
	EventRequested Event = "requested" // before reading: «El profesor solicitó recoger…»
	EventCollected Event = "collected" // after the round: «Tu profesor recogió…»
)

// File is one collected file as the window shows it.
type File struct {
	Path   string                     `json:"path"` // relative to the activity folder
	Size   int                        `json:"size"`
	Saved  bool                       `json:"saved"`            // the console saved it
	Reason protocol.FileCollectReason `json:"reason,omitempty"` // not saved: why
}

// Round is one «Recoger» of the teacher.
type Round struct {
	Round   int                           `json:"round"`
	At      time.Time                     `json:"at"`     // when the teacher asked
	DoneAt  time.Time                     `json:"doneAt"` // zero while sending
	Files   []File                        `json:"files"`
	Skipped []protocol.FileCollectSkipped `json:"skipped"`
	// Failed: the round could not be read at all (folder unavailable).
	Failed protocol.FileCollectReason `json:"failed,omitempty"`
}

// Collection is one collection as the window shows it: where to save and
// what was collected, round by round (docs/privacidad.md rules 61–62).
type Collection struct {
	ID       string    `json:"id"`
	Date     string    `json:"date"`
	Activity string    `json:"activity"`
	Folder   string    `json:"folder"` // relative to the work folder, "/"-separated
	Patterns []string  `json:"patterns"`
	OpenedAt time.Time `json:"openedAt"`
	State    State     `json:"state"`
	Rounds   []Round   `json:"rounds"`
}

func (c *Collection) clone() Collection {
	out := *c
	out.Patterns = append([]string(nil), c.Patterns...)
	out.Rounds = make([]Round, len(c.Rounds))
	for i, r := range c.Rounds {
		r.Files = append([]File(nil), r.Files...)
		r.Skipped = append([]protocol.FileCollectSkipped(nil), r.Skipped...)
		out.Rounds[i] = r
	}
	return out
}

// Responder is the agent side of file.collect.
type Responder struct {
	// Reader reads the work folder; nil when the OS has none, and then the
	// agent does not declare file.collect.
	Reader platform.WorkFolderReader
	// OnChange runs after every change, with the collection and why: the
	// window shows it (notify, never raise). EventRequested runs before
	// anything is read. Optional.
	OnChange func(Collection, Event)

	AckWait time.Duration // 0: DefaultAckWait
	Now     func() time.Time
	Logger  *slog.Logger

	mu     sync.Mutex
	list   []*Collection // oldest first
	busy   bool
	cancel context.CancelFunc
	waits  map[string]chan protocol.FileCollectResponseAck // file_id →
	wg     sync.WaitGroup
}

// Available reports whether the agent can collect.
func (r *Responder) Available() bool { return r.Reader != nil }

// Capabilities lists what this responder lets the agent declare in hello.
func (r *Responder) Capabilities() []protocol.Capability {
	if !r.Available() {
		return nil
	}
	return []protocol.Capability{protocol.CapFileCollect}
}

// Collections is what the window shows, oldest first.
func (r *Responder) Collections() []Collection {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Collection, len(r.list))
	for i, c := range r.list {
		out[i] = c.clone()
	}
	return out
}

// Folder returns the activity folder of a known collection, for «Abrir
// carpeta»: the window cannot ask for any other folder.
func (r *Responder) Folder(id string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c := r.find(id); c != nil {
		return c.Folder, true
	}
	return "", false
}

// Handle processes file.collect.* and reports false for any other message.
// A request is read and sent in the background, tied to ctx (the
// connection); reply is set when the answer is immediate.
func (r *Responder) Handle(ctx context.Context, send Sender, env protocol.Envelope) (reply protocol.Payload, handled bool) {
	switch p := env.Payload.(type) {
	case protocol.FileCollectOpen:
		if !r.Available() {
			return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta file.collect"), true
		}
		return r.open(env.ID, p), true
	case protocol.FileCollectRequest:
		if !r.Available() {
			return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta file.collect"), true
		}
		return r.request(ctx, send, env.ID, p), true
	case protocol.FileCollectResponseAck:
		r.mu.Lock()
		ch := r.waits[p.FileID]
		r.mu.Unlock()
		if ch != nil {
			select {
			case ch <- p:
			default:
			}
		}
		return nil, true
	}
	return nil, false
}

func failedAck(refID string, id string, round int, reason protocol.FileCollectReason) protocol.FileCollectAck {
	return protocol.FileCollectAck{RefID: refID, CollectID: id, Round: round, Status: protocol.FileCollectFailed, Reason: reason}
}

func (r *Responder) open(refID string, p protocol.FileCollectOpen) protocol.Payload {
	log := r.log().With("collect_id", p.CollectID)
	folder, err := Folder(p.Date, p.Activity)
	if err == nil {
		err = CheckPatterns(p.Patterns)
	}
	if err != nil {
		log.Info("collection refused", "reason", protocol.CollectInvalidPath)
		return failedAck(refID, p.CollectID, 0, protocol.CollectInvalidPath)
	}
	if err := r.Reader.MkdirAll(folder); err != nil {
		reason := reasonFor(err)
		log.Warn("collection folder not created", "reason", reason, "err", err)
		return failedAck(refID, p.CollectID, 0, reason)
	}
	r.mu.Lock()
	c, isNew := r.upsert(p.CollectID, p.Date, p.Activity, folder, p.Patterns)
	snap := c.clone()
	r.mu.Unlock()
	if isNew {
		log.Info("collection opened")
		r.changed(snap, EventOpened)
	}
	return protocol.FileCollectAck{RefID: refID, CollectID: p.CollectID, Status: protocol.FileCollectReady, Path: folder}
}

// upsert finds or adds a collection; the caller holds r.mu.
func (r *Responder) upsert(id, date, activity, folder string, patterns []string) (*Collection, bool) {
	if c := r.find(id); c != nil {
		return c, false
	}
	c := &Collection{ID: id, Date: date, Activity: activity, Folder: folder,
		Patterns: append([]string(nil), patterns...), OpenedAt: r.now(), State: StateOpen}
	r.list = append(r.list, c)
	if len(r.list) > maxCollections {
		r.list = r.list[len(r.list)-maxCollections:]
	}
	return c, true
}

func (r *Responder) find(id string) *Collection {
	for _, c := range r.list {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func (r *Responder) request(ctx context.Context, send Sender, refID string, p protocol.FileCollectRequest) protocol.Payload {
	log := r.log().With("collect_id", p.CollectID, "round", p.Round)
	folder, err := Folder(p.Date, p.Activity)
	if err == nil {
		err = CheckPatterns(p.Patterns)
	}
	if err != nil {
		log.Info("collection request refused", "reason", protocol.CollectInvalidPath)
		return failedAck(refID, p.CollectID, p.Round, protocol.CollectInvalidPath)
	}
	r.mu.Lock()
	if r.busy {
		r.mu.Unlock()
		log.Info("collection request refused", "reason", protocol.CollectBusy)
		return failedAck(refID, p.CollectID, p.Round, protocol.CollectBusy)
	}
	r.busy = true
	c, _ := r.upsert(p.CollectID, p.Date, p.Activity, folder, p.Patterns)
	c.Patterns = append([]string(nil), p.Patterns...)
	c.State = StateRequested
	c.Rounds = append(c.Rounds, Round{Round: p.Round, At: r.now()})
	if len(c.Rounds) > maxRoundsPerColl {
		c.Rounds = c.Rounds[len(c.Rounds)-maxRoundsPerColl:]
	}
	snap := c.clone()
	rctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.wg.Add(1)
	r.mu.Unlock()
	// The student is told before anything is read (docs/privacidad.md rule 61).
	r.changed(snap, EventRequested)
	log.Info("collection requested")
	go func() {
		defer r.wg.Done()
		defer cancel()
		r.run(rctx, send, refID, p, folder)
	}()
	return nil
}

// picked is a file read and ready to send.
type picked struct {
	path    string
	data    []byte
	modTime time.Time
}

func (r *Responder) run(ctx context.Context, send Sender, refID string, p protocol.FileCollectRequest, folder string) {
	log := r.log().With("collect_id", p.CollectID, "round", p.Round)
	round := Round{Round: p.Round}
	finish := func() {
		round.DoneAt = r.now()
		r.mu.Lock()
		r.busy, r.cancel = false, nil
		var snap Collection
		if c := r.find(p.CollectID); c != nil {
			c.State = StateCollected
			if n := len(c.Rounds); n > 0 && c.Rounds[n-1].Round == p.Round {
				round.At = c.Rounds[n-1].At
				c.Rounds[n-1] = round
			}
			snap = c.clone()
		}
		r.mu.Unlock()
		if snap.ID != "" {
			r.changed(snap, EventCollected)
		}
	}
	fail := func(reason protocol.FileCollectReason, err error) {
		log.Warn("collection round failed", "reason", reason, "err", err)
		round.Failed = reason
		_ = send(ctx, failedAck(refID, p.CollectID, p.Round, reason))
		finish()
	}
	if err := r.Reader.MkdirAll(folder); err != nil {
		fail(reasonFor(err), err)
		return
	}
	listed, err := r.Reader.List(folder, protocol.MaxFileCollectDepth)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fail(reasonFor(err), err)
		return
	}
	files, skipped := r.pick(folder, listed, p)
	round.Skipped = skipped
	total := 0
	for _, f := range files {
		total += len(f.data)
	}
	ack := protocol.FileCollectAck{RefID: refID, CollectID: p.CollectID, Round: p.Round, Path: folder,
		Status: protocol.FileCollectEmpty, Skipped: skipped}
	if len(files) > 0 {
		ack.Status, ack.Files, ack.Total = protocol.FileCollectSending, len(files), total
	}
	if err := send(ctx, ack); err != nil {
		log.Info("collection ack not sent", "err", err)
		round.Failed = protocol.CollectInternal
		finish()
		return
	}
	log.Info("collection read", "files", len(files), "bytes", total, "skipped", len(skipped))
	for i, f := range files {
		res := File{Path: f.path, Size: len(f.data), Reason: protocol.CollectIncomplete}
		if ctx.Err() == nil {
			res = r.sendFile(ctx, send, p, f, i+1, len(files))
		}
		files[i].data = nil // release as soon as it is sent
		round.Files = append(round.Files, res)
	}
	finish()
}

// pick filters the listing by the patterns and the limits, reads what fits
// and reports the rest as skipped.
func (r *Responder) pick(folder string, listed []platform.WorkFile, p protocol.FileCollectRequest) ([]picked, []protocol.FileCollectSkipped) {
	var files []picked
	var skipped []protocol.FileCollectSkipped
	skip := func(f platform.WorkFile, reason protocol.FileCollectReason) {
		if len(skipped) < protocol.MaxFileCollectFiles {
			skipped = append(skipped, protocol.FileCollectSkipped{Path: f.Path, Size: f.Size, Reason: reason})
		}
	}
	total := 0
	for _, f := range listed {
		if !f.Link && !Match(p.Patterns, path.Base(f.Path)) {
			continue
		}
		switch {
		case f.Link:
			skip(f, protocol.CollectLink)
			continue
		case CheckPath(f.Path) != nil:
			skip(f, protocol.CollectInvalidPath)
			continue
		case f.Offline:
			skip(f, protocol.CollectOffline)
			continue
		case f.Size > int64(p.MaxFileSize):
			skip(f, protocol.CollectTooLarge)
			continue
		case len(files) >= p.MaxFiles:
			skip(f, protocol.CollectTooMany)
			continue
		case total+int(f.Size) > p.MaxTotal:
			skip(f, protocol.CollectTooLarge)
			continue
		}
		data, err := r.Reader.Read(folder+"/"+f.Path, int64(p.MaxFileSize))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue // removed since the listing
		case errors.Is(err, platform.ErrFileTooLarge):
			skip(f, protocol.CollectTooLarge)
			continue
		case errors.Is(err, platform.ErrOffline):
			skip(f, protocol.CollectOffline)
			continue
		case errors.Is(err, platform.ErrLinkInPath):
			skip(f, protocol.CollectLink)
			continue
		case err != nil:
			skip(f, protocol.CollectDenied)
			continue
		}
		if total+len(data) > p.MaxTotal { // grew since the listing
			skip(f, protocol.CollectTooLarge)
			continue
		}
		total += len(data)
		files = append(files, picked{path: f.Path, data: data, modTime: f.ModTime})
	}
	return files, skipped
}

// sendFile sends every part of one file and waits for the console's ack.
func (r *Responder) sendFile(ctx context.Context, send Sender, p protocol.FileCollectRequest, f picked, index, count int) File {
	res := File{Path: f.path, Size: len(f.data)}
	fileID := protocol.NewID()
	sum := sha256.Sum256(f.data)
	parts := protocol.SplitParts(f.data, protocol.FileCollectPartSize)
	if len(parts) == 0 {
		parts = [][]byte{{}}
	}
	ch := make(chan protocol.FileCollectResponseAck, 1)
	r.mu.Lock()
	if r.waits == nil {
		r.waits = map[string]chan protocol.FileCollectResponseAck{}
	}
	r.waits[fileID] = ch
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.waits, fileID)
		r.mu.Unlock()
	}()
	log := r.log().With("collect_id", p.CollectID, "round", p.Round, "file_id", fileID)
	for i, data := range parts {
		msg := protocol.FileCollectResponse{CollectID: p.CollectID, Round: p.Round, FileID: fileID,
			Part: i + 1, Parts: len(parts), Data: data}
		if i == 0 {
			msg.Path, msg.Size, msg.SHA256 = f.path, len(f.data), hex.EncodeToString(sum[:])
			msg.Index, msg.Files = index, count
			if !f.modTime.IsZero() {
				msg.ModifiedAt = f.modTime.UnixMilli()
			}
		}
		if err := send(ctx, msg); err != nil {
			log.Info("collected file not sent", "part", i+1, "err", err)
			res.Reason = protocol.CollectIncomplete
			return res
		}
	}
	t := time.NewTimer(r.ackWait())
	defer t.Stop()
	select {
	case ack := <-ch:
		if ack.Status == protocol.FilePushSaved {
			res.Saved = true
		} else {
			res.Reason = ack.Reason
		}
		log.Info("collected file acked", "status", ack.Status, "size", len(f.data), "parts", len(parts))
	case <-t.C:
		log.Info("collected file not acked in time")
		res.Reason = protocol.CollectIncomplete
	case <-ctx.Done():
		res.Reason = protocol.CollectIncomplete
	}
	return res
}

// Reset abandons the round in course (the connection closed). What was
// collected stays in Collections.
func (r *Responder) Reset() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Wait blocks until the round in course ends.
func (r *Responder) Wait() { r.wg.Wait() }

func (r *Responder) changed(c Collection, ev Event) {
	if r.OnChange != nil {
		r.OnChange(c, ev)
	}
}

// reasonFor turns an error of the work folder into a reason of the ack.
func reasonFor(err error) protocol.FileCollectReason {
	switch {
	case errors.Is(err, ErrInvalid), errors.Is(err, platform.ErrLinkInPath), errors.Is(err, platform.ErrPathTooLong),
		errors.Is(err, platform.ErrFileExists):
		return protocol.CollectInvalidPath
	case errors.Is(err, platform.ErrDenied):
		return protocol.CollectDenied
	case errors.Is(err, platform.ErrNotSupported):
		return protocol.CollectUnsupported
	}
	return protocol.CollectInternal
}

func (r *Responder) ackWait() time.Duration {
	if r.AckWait > 0 {
		return r.AckWait
	}
	return DefaultAckWait
}

func (r *Responder) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Responder) log() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
