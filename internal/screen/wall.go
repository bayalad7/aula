package screen

import (
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// Stream parameters the console asks for (docs/protocol.md §5.6).
var (
	ThumbParams  = protocol.ScreenThumbStart{IntervalMS: 1000, Quality: 50, MaxWidth: 320}
	DetailParams = protocol.ScreenThumbStart{IntervalMS: 1000, Quality: 70, MaxWidth: 1280}
)

// Thumb is the last frame of one agent. It only lives in memory.
type Thumb struct {
	SessionID string    `json:"sessionId"`
	Seq       int       `json:"seq"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	JPEG      []byte    `json:"jpeg"` // Base64 in JSON, ready for a data: URL
	Detail    bool      `json:"detail"`
	At        time.Time `json:"at"`
}

// Command is a message the console must send to one agent.
type Command struct {
	SessionID string
	Payload   protocol.Payload // ScreenThumbStart or ScreenThumbStop
}

// Wall is the console side of screen.thumb: while the teacher has the screens
// view open, every connected agent that supports it streams thumbnails, and
// the one seen in detail streams larger frames. Closing the view stops every
// stream and forgets the frames.
type Wall struct {
	mu      sync.Mutex
	open    bool
	detail  string            // session seen in detail, "" for none
	streams map[string]stream // session ID → stream in force
	thumbs  map[string]Thumb  // session ID → last frame
	now     func() time.Time  // replaced in tests
}

type stream struct {
	startID string // envelope ID of the screen.thumb.start; "" until sent
	params  protocol.ScreenThumbStart
}

// NewWall returns a closed wall.
func NewWall() *Wall {
	return &Wall{streams: map[string]stream{}, thumbs: map[string]Thumb{}, now: time.Now}
}

// Open marks the screens view as open; Plan then starts the streams.
func (w *Wall) Open() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.open = true
}

// IsOpen reports whether the screens view is open.
func (w *Wall) IsOpen() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.open
}

// Close marks the view as closed, forgets every frame and returns the stops
// to send.
func (w *Wall) Close() []Command {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.open, w.detail = false, ""
	var cmds []Command
	for id := range w.streams {
		cmds = append(cmds, Command{SessionID: id, Payload: protocol.ScreenThumbStop{}})
	}
	clear(w.streams)
	clear(w.thumbs)
	return cmds
}

// SetDetail chooses the session seen in detail ("" goes back to the grid).
func (w *Wall) SetDetail(sessionID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.detail = sessionID
}

// Plan returns the screen.thumb.start messages needed so that every connected
// session with screen.thumb streams with the right parameters, and forgets
// sessions that left. Record each start sent with Started.
func (w *Wall) Plan(sessions []session.Session) []Command {
	w.mu.Lock()
	defer w.mu.Unlock()
	live := map[string]bool{}
	var cmds []Command
	for _, s := range sessions {
		if s.State != session.StateConnected || !s.Can(protocol.CapScreenThumb) {
			continue
		}
		live[s.ID] = true
		if !w.open {
			continue
		}
		want := ThumbParams
		if s.ID == w.detail {
			want = DetailParams
		}
		if cur, ok := w.streams[s.ID]; !ok || cur.params != want {
			cmds = append(cmds, Command{SessionID: s.ID, Payload: want})
		}
	}
	for id := range w.streams {
		if !live[id] {
			delete(w.streams, id)
			delete(w.thumbs, id)
		}
	}
	if w.detail != "" && !live[w.detail] {
		w.detail = ""
	}
	return cmds
}

// Started records that a start was sent with the given envelope ID: frames of
// earlier starts are discarded from now on.
func (w *Wall) Started(sessionID, startID string, p protocol.ScreenThumbStart) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.open {
		return
	}
	w.streams[sessionID] = stream{startID: startID, params: p}
}

// Frame stores a frame if it belongs to the stream in force, and returns it.
func (w *Wall) Frame(sessionID string, f protocol.ScreenThumbFrame) (Thumb, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	st, ok := w.streams[sessionID]
	if !w.open || !ok || st.startID != f.RefID {
		return Thumb{}, false
	}
	t := Thumb{SessionID: sessionID, Seq: f.Seq, Width: f.Width, Height: f.Height, JPEG: f.JPEG,
		Detail: sessionID == w.detail, At: w.now()}
	w.thumbs[sessionID] = t
	return t, true
}

// Thumbs returns the last frame of every streaming session.
func (w *Wall) Thumbs() []Thumb {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]Thumb, 0, len(w.thumbs))
	for _, t := range w.thumbs {
		out = append(out, t)
	}
	return out
}
