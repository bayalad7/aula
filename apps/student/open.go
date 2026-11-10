package main

import (
	"slices"
	"time"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/openapp"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// --- Opening a page or an app (docs/protocol.md §5.22) --------------------------
//
// The teacher opens a page in the student's browser or an allowed app. After
// it opens, a notice in the corner says what opened and which teacher asked
// for it, until «Entendido» (docs/privacidad.md rule 66). What opens takes
// the focus by itself, so the window uses notify and never raise (rule 67).
// Go sends states; the window words them.

// EventOpened carries Opened whenever something opened.
const EventOpened = "aula:opened"

// NoticeOpened is the corner notice of an opening; Ref is its open_id.
const NoticeOpened = "opened"

// maxOpened is how many openings the window remembers.
const maxOpened = 20

// Opened is what opened in this session of the agent, oldest first.
type Opened struct {
	List []openapp.Opened `json:"list"`
}

// enableOpen lets the console open pages and allowed apps in this session
// and declares open.url and open.app. Call it before start.
func (a *App) enableOpen(l platform.Launcher) {
	a.opener.Session = openapp.Local{Launcher: l}
	a.hello.Capabilities = append(slices.Clone(a.hello.Capabilities), a.opener.Capabilities()...)
}

// who is the teacher and channel the strip shows, for the notice.
func (a *App) who() (teacher, channel string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status.Teacher, a.status.Channel
}

// onOpened shows the corner notice. Never raise: the browser or the app
// already took the focus (rule 67).
func (a *App) onOpened(o openapp.Opened) {
	a.mu.Lock()
	a.opened = append(a.opened, o)
	if len(a.opened) > maxOpened {
		a.opened = slices.Clone(a.opened[len(a.opened)-maxOpened:])
	}
	a.mu.Unlock()
	a.emit(EventOpened, a.openedView())
	a.onNotice(codedist.Notice{ID: protocol.NewID(), Kind: NoticeOpened, Ref: o.ID, Level: protocol.LevelInfo, At: time.Now()})
}

func (a *App) openedView() Opened {
	a.mu.Lock()
	defer a.mu.Unlock()
	list := slices.Clone(a.opened)
	if list == nil {
		list = []openapp.Opened{}
	}
	return Opened{List: list}
}
