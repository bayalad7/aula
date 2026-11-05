package main

import (
	"time"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/protocol"
)

// NoticeSnapshot is the Kind of the notice that tells the student the
// teacher took a screenshot of this screen (docs/privacidad.md rule 44).
const NoticeSnapshot = "snapshot"

// identity is this computer and the session's user, for the screenshot's
// header: in service mode, the user of the session in force.
func (a *App) identity() (hostname, username string) {
	h := a.currentHello()
	return h.Hostname, h.Username
}

// onSnapshot adds the «Captura de pantalla» notice right after the capture.
// It does not act on the computer: it uses notify, never raise (prompt 19).
// The window words it with the teacher and the time; Go sends the state.
func (a *App) onSnapshot(at time.Time) {
	a.onNotice(codedist.Notice{ID: protocol.NewID(), Kind: NoticeSnapshot, Level: protocol.LevelWarning, At: at})
}
