package main

import (
	"context"
	"time"

	"github.com/bayalad7/aula/internal/classend"
	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/openapp"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/remote"
	"github.com/bayalad7/aula/internal/transport"
)

// EventClassEnd carries the «La clase terminó» card (*classend.Card), or null
// when the student dismissed it (docs/protocol.md §5.25).
const EventClassEnd = "aula:class-end"

// handleClassEnd leaves the window as freshly opened, in the order of
// §5.25.1: first what acts on the computer, then the window. It never
// touches a pending power order nor the student's files, and it tells
// without taking the focus (notify): what it removes had its own band.
func (a *App) handleClassEnd(ctx context.Context, cl *transport.Client, env protocol.Envelope) bool {
	p, ok := env.Payload.(protocol.ClassEnd)
	if !ok {
		return false
	}
	ack := a.endClass(env.ID, p)
	if _, err := cl.Send(ctx, ack); err != nil {
		a.log.Debug("cannot send class.end.ack", "err", err)
	}
	return true
}

// endClass clears everything and returns the acknowledgement.
func (a *App) endClass(refID string, p protocol.ClassEnd) protocol.ClassEndAck {
	var r classend.Removed
	var failed []protocol.ClassEndReason

	a.mu.Lock()
	r.Control, r.Present, r.Broadcast = a.ctrl != remote.Off, a.presenting, a.watching
	r.Pause, r.Restrictions = a.paused, a.restrict.Active()
	a.mu.Unlock()

	// 1. What acts on the computer, as when the connection closes.
	a.control.Stop() // first: it releases the keys and gives the stream back
	a.present.Stop()
	a.viewer.Stop()
	// 2. The pause.
	a.freeze.Stop()
	a.mu.Lock()
	if a.paused {
		failed = append(failed, protocol.ClassEndPause)
	}
	a.mu.Unlock()
	// 3. The restrictions, with their journal (service mode only).
	if a.endRestrictions != nil {
		if err := a.endRestrictions(); err != nil {
			a.log.Warn("class ended: restrictions not fully removed", "err", err)
			failed = append(failed, protocol.ClassEndRestrictions)
		}
	}
	// 4. The timer and the polls.
	r.Timer = a.countdown.View().TimerID != ""
	a.countdown.Reset()
	a.stopTimer()
	a.emit(EventTimer, a.countdown.View())
	r.Polls = len(a.ballot.View().Cards)
	a.ballot.Reset()
	a.emitPolls("")
	// 5. The window: code, notices, files, collections, what was opened,
	// the hand and the messages. The files stay on the disk.
	r.Snippets = len(a.board.Snippets())
	a.board.Clear()
	a.emit(EventBoard, Board{Snippets: []codedist.Snippet{}})
	a.mu.Lock()
	r.Notices, r.Opened = len(a.notices), len(a.opened)
	a.notices, a.opened = nil, nil
	a.mu.Unlock()
	a.emit(EventNotices, Notices{Notices: []codedist.Notice{}})
	a.emit(EventOpened, Opened{List: []openapp.Opened{}})
	r.Files = a.inbox.Clear()
	a.emit(EventFiles, a.filesView(a.inbox.List()))
	r.Collections = a.collect.Forget()
	a.emit(EventCollections, a.collectionsView())
	r.Hand, r.Messages = a.desk.Reset()
	a.emitParticipation()
	// 6. The card, with the hour and what was removed (rule 83).
	card := classend.NewCard(p, time.Now(), r)
	a.mu.Lock()
	a.endCard = &card
	a.mu.Unlock()
	a.emit(EventClassEnd, &card)
	a.notify()

	a.log.Info("class ended", "end", p.EndID, "failed", len(failed), "snippets", r.Snippets, "notices", r.Notices,
		"files", r.Files, "collections", r.Collections, "polls", r.Polls, "timer", r.Timer, "restrictions", r.Restrictions,
		"pause", r.Pause, "control", r.Control, "present", r.Present, "broadcast", r.Broadcast)
	return classend.Ack(refID, p, failed...)
}

// DismissClassEnd closes the «La clase terminó» card. It only hides a card:
// every band went away with what caused it.
func (a *App) DismissClassEnd() {
	if a.remote != nil {
		_ = a.forward("DismissClassEnd", nil)
		return
	}
	a.mu.Lock()
	a.endCard = nil
	a.mu.Unlock()
	a.emit(EventClassEnd, (*classend.Card)(nil))
}
