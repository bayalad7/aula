package remote

import (
	"errors"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// Video parameters of the console (docs/protocol.md §5.16).
var (
	// ViewParams while the teacher has the control view open.
	ViewParams = protocol.RemoteStart{IntervalMS: 125, Quality: 60, MaxWidth: 1280}
	// HiddenParams while the view is closed and the control goes on.
	HiddenParams = protocol.RemoteStart{IntervalMS: 1000, Quality: 50, MaxWidth: 640}
)

// Console-side statuses besides those of remote.ack.
const (
	StatusStarting protocol.RemoteStatus = "starting" // remote.start sent, no ack yet
	StatusFailed   protocol.RemoteStatus = "failed"   // the agent answered error
	StatusGone     protocol.RemoteStatus = "gone"     // the student disconnected
)

// maxBacklog bounds the input kept while the console waits for its pace: past
// it, pointer moves are dropped first.
const maxBacklog = 8 * protocol.MaxRemoteEvents

// ErrBusy is returned by Take while another computer is controlled.
var ErrBusy = errors.New("another computer is already controlled")

// ErrNoControl is returned when there is no control in force.
var ErrNoControl = errors.New("no remote control in force")

// Control is the state of the console's remote control.
type Control struct {
	SessionID string                `json:"sessionID"`
	Hostname  string                `json:"hostname"`
	ControlID string                `json:"controlID"`
	Status    protocol.RemoteStatus `json:"status"`
	Reason    protocol.RemoteReason `json:"reason,omitempty"`
	Viewing   bool                  `json:"viewing"` // the control view is open
}

// Live reports whether the control still holds the computer.
func (c Control) Live() bool {
	switch c.Status {
	case StatusStarting, protocol.RemoteActive, protocol.RemotePaused, protocol.RemoteResumed:
		return true
	}
	return false
}

// Desk is the console's remote control: one computer at a time.
type Desk struct {
	Now func() time.Time // time.Now if nil

	mu       sync.Mutex
	c        Control
	envID    string // envelope of the remote.start, for error ref_id
	seq      int
	backlog  []protocol.InputEvent
	winStart time.Time
	winCount int
}

// Current returns the control on record (the last one, if it ended).
func (d *Desk) Current() Control {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.c
}

// Take records a new control of sessionID and returns its remote.start, to be
// sent in the envelope given to Started. Another live control → ErrBusy (with
// it, so the console can name it).
func (d *Desk) Take(sessionID, hostname string) (protocol.RemoteStart, Control, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.c.Live() {
		return protocol.RemoteStart{}, d.c, ErrBusy
	}
	d.c = Control{SessionID: sessionID, Hostname: hostname, ControlID: protocol.NewID(), Status: StatusStarting, Viewing: true}
	d.envID, d.seq, d.backlog, d.winCount = "", 0, nil, 0
	p := ViewParams
	p.ControlID = d.c.ControlID
	return p, d.c, nil
}

// Started records the envelope that carried the remote.start.
func (d *Desk) Started(envID string) {
	d.mu.Lock()
	d.envID = envID
	d.mu.Unlock()
}

// View records whether the control view is open and returns the remote.start
// that retunes the video for it.
func (d *Desk) View(visible bool) (string, protocol.RemoteStart, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.c.Live() {
		return "", protocol.RemoteStart{}, ErrNoControl
	}
	d.c.Viewing = visible
	p := HiddenParams
	if visible {
		p = ViewParams
	}
	p.ControlID = d.c.ControlID
	if !visible {
		d.backlog = nil // no input without the view
	}
	return d.c.SessionID, p, nil
}

// Release returns the remote.stop for the control in force. The control
// stays live until the agent's ended ack (or its disconnection).
func (d *Desk) Release() (string, protocol.RemoteStop, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.c.Live() {
		return "", protocol.RemoteStop{}, ErrNoControl
	}
	d.backlog = nil
	return d.c.SessionID, protocol.RemoteStop{ControlID: d.c.ControlID}, nil
}

// Ack applies a remote.ack; it reports whether it was for the control on
// record, with the new state.
func (d *Desk) Ack(sessionID string, a protocol.RemoteAck) (Control, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.c.SessionID != sessionID || d.c.ControlID != a.ControlID {
		return d.c, false
	}
	d.c.Status, d.c.Reason = a.Status, a.Reason
	if a.Status == protocol.RemoteResumed {
		d.c.Status = protocol.RemoteActive
	}
	if a.Status != protocol.RemoteActive && a.Status != protocol.RemoteResumed {
		d.backlog = nil
	}
	return d.c, true
}

// Error applies an error whose ref_id is refID; it reports whether it was
// for the remote.start on record.
func (d *Desk) Error(sessionID, refID string) (Control, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.c.SessionID != sessionID || d.envID == "" || d.envID != refID || !d.c.Live() {
		return d.c, false
	}
	d.c.Status, d.backlog = StatusFailed, nil
	return d.c, true
}

// Gone ends the control of a session that disconnected: the agent ended it
// on its own (docs/protocol.md §5.16).
func (d *Desk) Gone(sessionID string) (Control, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.c.SessionID != sessionID || !d.c.Live() {
		return d.c, false
	}
	d.c.Status, d.c.Reason, d.backlog = StatusGone, "", nil
	return d.c, true
}

// Input batches the teacher's events for the controlled computer: runs of
// pointer moves keep only their last one, batches hold at most
// protocol.MaxRemoteEvents, and no more than protocol.MaxRemoteRate leave
// per second; the rest waits for the next call. Input is only sent while the
// control is active and its view open.
func (d *Desk) Input(events []protocol.InputEvent) (string, []protocol.RemoteInput, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.c.Live() || !d.c.Viewing {
		return "", nil, ErrNoControl
	}
	if d.c.Status != protocol.RemoteActive {
		return d.c.SessionID, nil, nil // starting or paused: the agent would drop it
	}
	d.backlog = coalesce(append(d.backlog, events...))
	if len(d.backlog) > maxBacklog {
		d.backlog = trim(d.backlog, maxBacklog)
	}
	now := time.Now()
	if d.Now != nil {
		now = d.Now()
	}
	if now.Sub(d.winStart) >= time.Second {
		d.winStart, d.winCount = now, 0
	}
	var out []protocol.RemoteInput
	for len(d.backlog) > 0 && d.winCount < protocol.MaxRemoteRate {
		n := min(len(d.backlog), protocol.MaxRemoteEvents)
		d.seq++
		d.winCount++
		out = append(out, protocol.RemoteInput{ControlID: d.c.ControlID, Seq: d.seq, Events: append([]protocol.InputEvent(nil), d.backlog[:n]...)})
		d.backlog = d.backlog[n:]
	}
	if len(d.backlog) == 0 {
		d.backlog = nil
	}
	return d.c.SessionID, out, nil
}

// coalesce keeps only the last of each run of consecutive pointer moves.
func coalesce(in []protocol.InputEvent) []protocol.InputEvent {
	out := in[:0:0]
	for _, e := range in {
		if e.T == protocol.InputMove && len(out) > 0 && out[len(out)-1].T == protocol.InputMove {
			out[len(out)-1] = e
			continue
		}
		out = append(out, e)
	}
	return out
}

// trim drops pointer moves, oldest first, until in fits n; keys, buttons and
// wheel steps are kept so nothing stays pressed.
func trim(in []protocol.InputEvent, n int) []protocol.InputEvent {
	extra := len(in) - n
	out := make([]protocol.InputEvent, 0, len(in))
	for _, e := range in {
		if extra > 0 && e.T == protocol.InputMove {
			extra--
			continue
		}
		out = append(out, e)
	}
	return out
}

// Frame reports whether a screen.thumb.frame with ref_id refID belongs to the
// live control of sessionID (its remote.start), and not to the thumbnails.
func (d *Desk) Frame(sessionID, refID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.c.Live() && d.c.SessionID == sessionID && d.envID != "" && d.envID == refID
}
