package filepush

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// Limits of the agent (docs/protocol.md §5.20).
const (
	MaxOpenFiles     = 4                         // files reassembling or being written at once
	MaxOpenBytes     = protocol.MaxFilePushTotal // memory they may hold
	DefaultPartsWait = 60 * time.Second          // from part 1 to the last part
	maxRemembered    = 200                       // resolved file_ids answered again on a resend
)

// Writer saves a file in the work folder: platform.WorkFolder, or in
// service mode the helper's proxy (agentsvc). rel is "/"-separated and
// checked. With overwrite false it never replaces and fails with
// platform.ErrFileExists.
type Writer interface {
	Create(rel string, data []byte, overwrite bool) (replaced bool, err error)
}

// Sender sends one message on the current connection to the console.
type Sender func(ctx context.Context, p protocol.Payload) error

// Receiver is the agent side of file.push.
type Receiver struct {
	// Writer saves the files; nil when the OS has no work folder, and then
	// the agent does not declare file.push.
	Writer Writer
	// OnFile runs once per file, saved or failed, after its ack: the window
	// shows it (notify, never raise). Optional.
	OnFile func(Item)

	PartsWait time.Duration // 0: DefaultPartsWait
	Now       func() time.Time
	Logger    *slog.Logger

	mu    sync.Mutex
	open  map[string]*assembly // file_id → reassembling or being written
	bytes int                  // reserved by open
	done  map[string]protocol.FilePushAck
	order []string // done, oldest first
	wg    sync.WaitGroup
}

// assembly is one file on its way.
type assembly struct {
	head  protocol.FilePush // part 1, without its data
	parts *protocol.Reassembly
	timer *time.Timer
	ready bool // all parts arrived: being written
}

// Item is one file as the window shows it: what arrived and where it went.
type Item struct {
	PushID   string                  `json:"pushId"`
	FileID   string                  `json:"fileId"`
	Label    string                  `json:"label"`
	Index    int                     `json:"index"`
	Files    int                     `json:"files"`
	Name     string                  `json:"name"` // as the teacher sent it
	Dest     string                  `json:"dest"`
	Path     string                  `json:"path"` // saved: final path, relative, "/"-separated
	Size     int                     `json:"size"`
	Renamed  bool                    `json:"renamed"`
	Replaced bool                    `json:"replaced"`
	Reason   protocol.FilePushReason `json:"reason"` // failed: why
	At       time.Time               `json:"at"`
}

// Saved reports whether the file is on disk.
func (i Item) Saved() bool { return i.Reason == "" }

// Available reports whether the agent can save files.
func (r *Receiver) Available() bool { return r.Writer != nil }

// Capabilities lists what this receiver lets the agent declare in hello.
func (r *Receiver) Capabilities() []protocol.Capability {
	if !r.Available() {
		return nil
	}
	return []protocol.Capability{protocol.CapFilePush}
}

// Handle processes file.push and reports false for any other message. The
// file is written in the background, tied to ctx (the connection); reply is
// set only when a file is refused or answered right away.
func (r *Receiver) Handle(ctx context.Context, send Sender, env protocol.Envelope) (reply protocol.Payload, handled bool) {
	p, ok := env.Payload.(protocol.FilePush)
	if !ok {
		return nil, false
	}
	if !r.Available() {
		return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta file.push"), true
	}
	log := r.log().With("push_id", p.PushID, "file_id", p.FileID)
	r.mu.Lock()
	if r.open == nil {
		r.open, r.done = map[string]*assembly{}, map[string]protocol.FilePushAck{}
	}
	if ack, seen := r.done[p.FileID]; seen {
		if p.Part != 1 {
			r.mu.Unlock()
			return nil, true // the rest of a refused file
		}
		ack.RefID = env.ID // a resend: same answer, nothing written again
		r.mu.Unlock()
		log.Info("file push answered again", "status", ack.Status)
		return ack, true
	}
	a := r.open[p.FileID]
	if a == nil {
		if p.Part != 1 {
			log.Debug("file push part of an unknown file dropped", "part", p.Part)
			r.mu.Unlock()
			return nil, true
		}
		if ack, refused := r.admit(env.ID, p); refused {
			log.Info("file push refused", "reason", ack.Reason, "size", p.Size)
			item := r.finish(ack, p)
			r.mu.Unlock()
			r.notify(item)
			return ack, true
		}
		head := p
		head.Data = nil
		a = &assembly{head: head, parts: protocol.NewReassembly(p.Parts)}
		r.open[p.FileID] = a
		r.bytes += p.Size
		a.timer = time.AfterFunc(r.partsWait(), func() { r.expire(ctx, send, env.ID, p.FileID) })
	} else if a.ready || p.Parts != a.head.Parts {
		log.Debug("file push part dropped", "part", p.Part)
		r.mu.Unlock()
		return nil, true
	}
	if err := a.parts.Add(p.Part, p.Data); err != nil {
		log.Debug("file push part dropped", "part", p.Part, "err", err)
		r.mu.Unlock()
		return nil, true
	}
	if !a.parts.Complete() {
		r.mu.Unlock()
		return nil, true
	}
	a.ready = true
	a.timer.Stop()
	data := a.parts.Bytes()
	head := a.head
	r.wg.Add(1)
	r.mu.Unlock()
	go func() {
		defer r.wg.Done()
		ack := r.save(env.ID, head, data)
		r.mu.Lock()
		r.release(head)
		item := r.finish(ack, head)
		r.mu.Unlock()
		r.notify(item)
		if ctx.Err() == nil {
			if err := send(ctx, ack); err != nil {
				log.Debug("file push ack not sent", "err", err)
			}
		}
	}()
	return nil, true
}

// admit checks part 1 of a new file; r.mu is held.
func (r *Receiver) admit(refID string, p protocol.FilePush) (protocol.FilePushAck, bool) {
	fail := func(reason protocol.FilePushReason) (protocol.FilePushAck, bool) {
		return failed(refID, p, reason), true
	}
	if CheckName(p.Name) != nil || CheckDest(p.Dest) != nil {
		return fail(protocol.FilePushInvalidPath)
	}
	if len(r.open) >= MaxOpenFiles {
		return fail(protocol.FilePushBusy)
	}
	if r.bytes+p.Size > MaxOpenBytes {
		return fail(protocol.FilePushTooLarge)
	}
	return protocol.FilePushAck{}, false
}

// expire answers incomplete for a file whose parts stopped coming.
func (r *Receiver) expire(ctx context.Context, send Sender, refID, fileID string) {
	r.mu.Lock()
	a := r.open[fileID]
	if a == nil || a.ready {
		r.mu.Unlock()
		return
	}
	got, _ := a.parts.Received()
	r.release(a.head)
	ack := failed(refID, a.head, protocol.FilePushIncomplete)
	item := r.finish(ack, a.head)
	r.mu.Unlock()
	r.notify(item)
	r.log().Info("file push incomplete", "push_id", a.head.PushID, "file_id", fileID, "parts", a.head.Parts, "received", got)
	if ctx.Err() == nil {
		if err := send(ctx, ack); err != nil {
			r.log().Debug("file push ack not sent", "err", err)
		}
	}
}

// save checks the reassembled file and writes it, under another name when
// one is taken and overwrite is false. It never holds r.mu: writing may be
// slow (the helper, a network Documents folder).
func (r *Receiver) save(refID string, head protocol.FilePush, data []byte) protocol.FilePushAck {
	log := r.log().With("push_id", head.PushID, "file_id", head.FileID)
	sum := sha256.Sum256(data)
	if len(data) != head.Size || hex.EncodeToString(sum[:]) != head.SHA256 {
		log.Warn("file push corrupt", "size", len(data), "want_size", head.Size)
		return failed(refID, head, protocol.FilePushCorrupt)
	}
	if head.Overwrite {
		rel := Join(head.Dest, head.Name)
		replaced, err := r.Writer.Create(rel, data, true)
		if err != nil {
			return r.writeFailed(log, refID, head, err)
		}
		log.Info("file push saved", "size", len(data), "replaced", replaced)
		return protocol.FilePushAck{RefID: refID, PushID: head.PushID, FileID: head.FileID,
			Status: protocol.FilePushSaved, Path: rel, Replaced: replaced}
	}
	for n := 1; n <= MaxAlternative; n++ {
		rel := Join(head.Dest, Alternative(head.Name, n))
		_, err := r.Writer.Create(rel, data, false)
		if errors.Is(err, platform.ErrFileExists) {
			continue
		}
		if err != nil {
			return r.writeFailed(log, refID, head, err)
		}
		log.Info("file push saved", "size", len(data), "renamed", n > 1)
		return protocol.FilePushAck{RefID: refID, PushID: head.PushID, FileID: head.FileID,
			Status: protocol.FilePushSaved, Path: rel, Renamed: n > 1}
	}
	log.Warn("file push not saved: every alternative name is taken")
	return failed(refID, head, protocol.FilePushExists)
}

func (r *Receiver) writeFailed(log *slog.Logger, refID string, head protocol.FilePush, err error) protocol.FilePushAck {
	reason := ReasonFor(err)
	log.Warn("file push not saved", "reason", reason, "err", err)
	return failed(refID, head, reason)
}

// ReasonFor maps an error of Writer to the reason of the ack.
func ReasonFor(err error) protocol.FilePushReason {
	switch {
	case errors.Is(err, platform.ErrNotSupported):
		return protocol.FilePushUnsupported
	case errors.Is(err, platform.ErrLinkInPath), errors.Is(err, ErrInvalidPath):
		return protocol.FilePushInvalidPath
	case errors.Is(err, platform.ErrFileExists):
		return protocol.FilePushExists
	case errors.Is(err, platform.ErrNoSpace):
		return protocol.FilePushNoSpace
	case errors.Is(err, platform.ErrDenied):
		return protocol.FilePushDenied
	case errors.Is(err, platform.ErrPathTooLong):
		return protocol.FilePushPathTooLong
	}
	return protocol.FilePushInternalFail
}

// release frees what an open file reserved; r.mu is held.
func (r *Receiver) release(head protocol.FilePush) {
	if a, ok := r.open[head.FileID]; ok {
		delete(r.open, head.FileID)
		r.bytes -= a.head.Size
	}
}

// finish remembers the answer and returns what the window shows; r.mu is
// held.
func (r *Receiver) finish(ack protocol.FilePushAck, head protocol.FilePush) Item {
	if r.done == nil {
		r.done = map[string]protocol.FilePushAck{}
	}
	r.done[head.FileID] = ack
	r.order = append(r.order, head.FileID)
	if len(r.order) > maxRemembered {
		delete(r.done, r.order[0])
		r.order = r.order[1:]
	}
	return Item{PushID: head.PushID, FileID: head.FileID, Label: head.Label, Index: head.Index,
		Files: head.Files, Name: head.Name, Dest: head.Dest, Path: ack.Path, Size: head.Size,
		Renamed: ack.Renamed, Replaced: ack.Replaced, Reason: ack.Reason, At: r.now()}
}

// notify tells the window, without r.mu.
func (r *Receiver) notify(item Item) {
	if r.OnFile != nil {
		r.OnFile(item)
	}
}

// Reset forgets the files of a connection that ended: what was on its way
// is dropped (never written), and a resend on the next connection is a new
// delivery.
func (r *Receiver) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, a := range r.open {
		if !a.ready {
			a.timer.Stop()
			delete(r.open, id)
			r.bytes -= a.head.Size
		}
	}
	r.done, r.order = map[string]protocol.FilePushAck{}, nil
}

// Wait blocks until the files being written are written.
func (r *Receiver) Wait() { r.wg.Wait() }

func failed(refID string, p protocol.FilePush, reason protocol.FilePushReason) protocol.FilePushAck {
	return protocol.FilePushAck{RefID: refID, PushID: p.PushID, FileID: p.FileID,
		Status: protocol.FilePushFailed, Reason: reason}
}

func (r *Receiver) partsWait() time.Duration {
	if r.PartsWait > 0 {
		return r.PartsWait
	}
	return DefaultPartsWait
}

func (r *Receiver) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Receiver) log() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.New(slog.DiscardHandler)
}
