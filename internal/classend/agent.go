package classend

import (
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// Removed is what the agent took off the computer and the window when the
// class ended, for the «La clase terminó» card (docs/privacidad.md rule 83).
// Counts only: never the code, the notices or the names of the files.
type Removed struct {
	Control      bool `json:"control"`      // the teacher was controlling it
	Present      bool `json:"present"`      // its screen was shown to the class
	Broadcast    bool `json:"broadcast"`    // it was showing the teacher's screen
	Pause        bool `json:"pause"`        // the screen was paused
	Restrictions bool `json:"restrictions"` // apps blocked or websites filtered
	Timer        bool `json:"timer"`
	Hand         bool `json:"hand"`        // the hand was up
	Polls        int  `json:"polls"`       // poll cards
	Snippets     int  `json:"snippets"`    // code cards
	Notices      int  `json:"notices"`     // notices of the teacher
	Files        int  `json:"files"`       // cards of received files (the files stay)
	Collections  int  `json:"collections"` // cards of collections (the files stay)
	Opened       int  `json:"opened"`      // what was opened
	Messages     int  `json:"messages"`    // messages to the teacher not sent yet
}

// Empty reports whether nothing was removed.
func (r Removed) Empty() bool { return r == Removed{} }

// Card is the «La clase terminó» card of the window: Go sends the state, the
// window words it.
type Card struct {
	EndID   string  `json:"endId"`
	At      int64   `json:"at"` // Unix milliseconds, shown as the local hour
	Removed Removed `json:"removed"`
}

// NewCard builds the card of an end of class at t.
func NewCard(p protocol.ClassEnd, t time.Time, r Removed) Card {
	return Card{EndID: p.EndID, At: t.UnixMilli(), Removed: r}
}

// Ack answers a class.end: clean, or failed with the first thing that stayed
// (the order of §5.25.1).
func Ack(refID string, p protocol.ClassEnd, failed ...protocol.ClassEndReason) protocol.ClassEndAck {
	a := protocol.ClassEndAck{RefID: refID, EndID: p.EndID, Status: protocol.ClassEndClean}
	if len(failed) > 0 {
		a.Status, a.Reason = protocol.ClassEndFailed, failed[0]
	}
	return a
}
