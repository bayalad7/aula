package main

import (
	"context"
	"slices"
	"time"

	"github.com/bayalad7/aula/internal/classend"
	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/collect"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// «Terminar la clase» (prompt 10.14, docs/protocol.md §5.25): one button
// under the Selector de clase that stops what is still on, with the stop of
// each feature, asks every agent to leave its window as freshly opened and
// leaves the console «sin clase». It never deletes a file, and pending power
// orders are only cancelled when the teacher ticks it.

// ClassEndPreview is what the dialog lists before ending: what is still on
// (the same as the Franja global) and what is optional.
type ClassEndPreview struct {
	Broadcast    bool   `json:"broadcast"`
	Present      string `json:"present"` // the student shown to the class; "" for none
	Control      string `json:"control"` // the computer under control; "" for none
	Paused       int    `json:"paused"`
	Apps         bool   `json:"apps"` // apps blocked
	Web          bool   `json:"web"`  // websites filtered
	Timer        bool   `json:"timer"`
	Poll         bool   `json:"poll"`
	Power        int    `json:"power"` // computers with a pending power order
	Hands        int    `json:"hands"` // raised hands and pending messages are forgotten
	Messages     int    `json:"messages"`
	Snippets     int    `json:"snippets"` // the code history, emptied
	Students     int    `json:"students"` // connected, real
	Simulated    int    `json:"simulated"`
	Old          int    `json:"old"` // connected agents without class.end: only the stops and code.clear
	Disconnected int    `json:"disconnected"`
	// Collection is the newest collection, for «Recoger el trabajo antes de
	// terminar»; nil when there is none.
	Collection *ClassEndCollection `json:"collection"`
	Running    bool                `json:"running"` // an end is still running
}

// ClassEndCollection is the collection «Recoger el trabajo» would run.
type ClassEndCollection struct {
	ID       string `json:"id"`
	Activity string `json:"activity"`
	Missing  int    `json:"missing"` // students it would collect from
}

// ClassEndRequest are the optional steps the teacher ticked.
type ClassEndRequest struct {
	Collect     bool `json:"collect"`     // collect the newest collection first
	CancelPower bool `json:"cancelPower"` // cancel the pending power orders
}

// ClassEndPreview returns what «Terminar la clase» would stop.
func (a *App) ClassEndPreview() (ClassEndPreview, error) {
	class := a.currentClass()
	if class == nil {
		return ClassEndPreview{}, errNoClass
	}
	p := ClassEndPreview{Broadcast: a.BroadcastState().Active, Running: a.ends.View().Running}
	if ps := a.PresentState(); ps.Active {
		p.Present = ps.Hostname
	}
	if c := a.desk.Current(); c.Live() {
		p.Control = c.Hostname
	}
	p.Paused = a.pauses.Summary().Paused
	r := a.Restrictions()
	p.Apps, p.Web = r.Apps != nil, r.Web != nil
	p.Timer = a.timer.Snapshot(nil, nil).Active
	p.Poll = a.polls.Snapshot(nil, nil).Current != nil
	for _, s := range class.Sessions() {
		if s.State != session.StateConnected {
			p.Disconnected++
			continue
		}
		p.Students++
		if !s.Can(protocol.CapClassEnd) {
			p.Old++
		}
		if e, ok := a.power.Get(s.ID); ok && e.Pending() {
			p.Power++
		}
	}
	if host, ok := class.(simulationHost); ok {
		for _, s := range host.SimSessions() {
			if s.State == session.StateConnected {
				p.Simulated++
			}
		}
	}
	in := a.Participation()
	p.Hands, p.Messages = len(in.Hands), len(in.Messages)
	a.mu.Lock()
	p.Snippets = len(a.history.Entries())
	a.mu.Unlock()
	if list := a.collector.List(); len(list) > 0 {
		c := list[0]
		p.Collection = &ClassEndCollection{ID: c.ID, Activity: c.Activity, Missing: len(collectMissing(c))}
	}
	return p, nil
}

// collectMissing are the students «Recoger el trabajo» collects from: those
// without their work yet (all of them before the first round).
func collectMissing(c collect.Summary) []string {
	var keys []string
	for _, s := range c.Students {
		if c.Round == 0 || (s.Status != collect.StatusCollected && s.Status != collect.StatusEmpty) {
			keys = append(keys, s.Key)
		}
	}
	return keys
}

// FinishClass ends the class: the steps run in order (§5.25) without
// stopping on a failure, and the dialog follows them by EventClassEnd. It
// returns at once with the end running.
func (a *App) FinishClass(req ClassEndRequest) (classend.View, error) {
	class := a.currentClass()
	if class == nil {
		return classend.View{}, errNoClass
	}
	endID, ok := a.ends.Begin()
	if !ok {
		return classend.View{}, codedf(codeEndBusy, "ya se está terminando la clase")
	}
	a.log.Info("class ending", "end", endID, "collect", req.Collect, "cancel_power", req.CancelPower)
	go a.runClassEnd(class, endID, req)
	return a.ends.View(), nil
}

// ClassEnd returns the last end of class, for the dialog.
func (a *App) ClassEnd() classend.View { return a.ends.View() }

// RetryClassEnd sends class.end again to the connected computers without an
// answer or that failed («Reintentar a los que faltan»).
func (a *App) RetryClassEnd() (classend.View, error) {
	class := a.currentClass()
	if class == nil {
		return classend.View{}, errNoClass
	}
	endID := a.ends.EndID()
	if endID == "" {
		return classend.View{}, codedf(codeEndBusy, "todavía no terminaste la clase")
	}
	retry := a.ends.Retry()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, s := range a.endSessions(class) {
		if s.connected && slices.Contains(retry, s.Hostname) {
			a.sendClassEnd(ctx, class, s, endID)
		}
	}
	a.expireClassEnd()
	v := a.ends.View()
	a.emit(EventClassEnd, v)
	return v, nil
}

func (a *App) runClassEnd(class classroom, endID string, req ClassEndRequest) {
	step := func(s classend.Step) {
		a.ends.AddStep(s)
		a.emit(EventClassEnd, a.ends.View())
	}
	// 1. Optional: the work, before anything else changes.
	if req.Collect {
		if list := a.collector.List(); len(list) > 0 {
			step(a.collectBeforeEnd(list[0]))
		}
	}
	// 2. What acts on the computers.
	if st := a.BroadcastState(); st.Active {
		a.StopBroadcast()
		step(classend.Step{Kind: classend.StepBroadcast, Status: classend.StepDone, Count: st.Receivers})
	}
	if a.PresentState().Active {
		a.StopPresent()
		step(classend.Step{Kind: classend.StepPresent, Status: classend.StepDone, Count: 1})
	}
	if a.desk.Current().Live() {
		a.endControl(class)
		step(classend.Step{Kind: classend.StepControl, Status: classend.StepDone, Count: 1})
	}
	if n := a.pauses.Summary().Paused; n > 0 {
		res := a.ResumeScreens(nil)
		step(classend.Step{Kind: classend.StepPause, Status: stepStatus(len(res.Failed)), Count: n})
	}
	if r := a.Restrictions(); r.Apps != nil || r.Web != nil {
		a.ClearAllRestrictions()
		n := 0
		if r.Apps != nil {
			n++
		}
		if r.Web != nil {
			n++
		}
		step(classend.Step{Kind: classend.StepRestrictions, Status: classend.StepDone, Count: n})
	}
	// 3. What informs the students.
	if a.timer.Snapshot(nil, nil).Active {
		_, err := a.StopTimer()
		step(classend.Step{Kind: classend.StepTimer, Status: stepStatus(errCount(err)), Count: 1})
	}
	if a.polls.Snapshot(nil, nil).Current != nil {
		_, err := a.ClosePoll()
		step(classend.Step{Kind: classend.StepPoll, Status: stepStatus(errCount(err)), Count: 1})
	}
	// 4. Pending power orders: kept unless the teacher asked.
	if n := a.pendingPower(class); n > 0 {
		if req.CancelPower {
			res := a.PowerCancel(nil)
			step(classend.Step{Kind: classend.StepPower, Status: stepStatus(len(res.Failed)), Count: n})
		} else {
			step(classend.Step{Kind: classend.StepPower, Status: classend.StepKept, Count: n})
		}
	}
	// 5. Each window.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	for _, s := range a.endSessions(class) {
		if s.connected {
			a.sendClassEnd(ctx, class, s, endID)
		} else {
			a.ends.Away(s.Hostname, s.simulated)
		}
	}
	cancel()
	// 6. The console, «sin clase».
	a.clearConsoleClass()
	a.ends.Done()
	a.expireClassEnd()
	a.log.Info("class ended", "end", endID)
	a.emit(EventClassEnd, a.ends.View())
}

func stepStatus(failed int) classend.StepStatus {
	if failed > 0 {
		return classend.StepFailed
	}
	return classend.StepDone
}

func errCount(err error) int {
	if err != nil {
		return 1
	}
	return 0
}

// collectBeforeEnd collects the newest collection from who is missing and
// waits for it, at most classend.CollectWait.
func (a *App) collectBeforeEnd(c collect.Summary) classend.Step {
	s := classend.Step{Kind: classend.StepCollect}
	keys := collectMissing(c)
	if len(keys) == 0 {
		s.Status = classend.StepDone
		return s
	}
	if c.Round == 0 {
		keys = nil // the first round: everyone
	}
	sum, err := a.Collect(c.ID, keys)
	if err != nil {
		s.Status = classend.StepFailed
		return s
	}
	s.Count = len(sum.Students)
	deadline := a.now().Add(a.collectWait())
	for {
		cur, ok := a.collector.Get(c.ID)
		if !ok || !cur.Collecting {
			s.Status = classend.StepDone
			return s
		}
		if !a.now().Before(deadline) {
			s.Status = classend.StepTimeout
			return s
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (a *App) collectWait() time.Duration {
	if a.endCollectWait > 0 {
		return a.endCollectWait
	}
	return classend.CollectWait
}

func (a *App) pendingPower(class classroom) int {
	n := 0
	for _, s := range class.Sessions() {
		if e, ok := a.power.Get(s.ID); ok && e.Pending() {
			n++
		}
	}
	return n
}

// endSession is one computer the end of class goes to.
type endSession struct {
	session.Session
	connected bool
	simulated bool
}

// endSessions are the real and simulated sessions of the class.
func (a *App) endSessions(class classroom) []endSession {
	var out []endSession
	for _, s := range class.Sessions() {
		out = append(out, endSession{Session: s, connected: s.State == session.StateConnected})
	}
	if host, ok := class.(simulationHost); ok {
		for _, s := range host.SimSessions() {
			out = append(out, endSession{Session: s, connected: s.State == session.StateConnected, simulated: true})
		}
	}
	return out
}

// sendClassEnd sends class.end to one connected session; an earlier agent
// gets code.clear instead (it already got the stops).
func (a *App) sendClassEnd(ctx context.Context, class classroom, s endSession, endID string) {
	if !s.Can(protocol.CapClassEnd) {
		if s.Can(protocol.CapCodeView) {
			if _, err := class.Send(ctx, s.ID, protocol.CodeClear{}); err != nil {
				a.log.Debug("class end: code.clear not sent", "session", s.ID, "err", err)
			}
		}
		a.ends.Partial(s.Hostname, s.ID, s.simulated)
		return
	}
	// endMu: a fast agent's ack is never applied before Sent records it.
	a.endMu.Lock()
	defer a.endMu.Unlock()
	id, err := class.Send(ctx, s.ID, protocol.ClassEnd{EndID: endID})
	a.ends.Sent(s.Hostname, s.ID, id, s.simulated)
	if err != nil {
		a.log.Debug("class end not sent", "session", s.ID, "err", err)
		a.ends.Gone(s.ID) // it gets it when it reconnects
	}
}

// expireClassEnd marks «sin acuse» whoever did not answer in AckWait.
func (a *App) expireClassEnd() {
	time.AfterFunc(classend.AckWait+100*time.Millisecond, func() {
		if a.ends.Expire() {
			a.emit(EventClassEnd, a.ends.View())
		}
	})
}

// onClassEndAck applies a class.end.ack, real or simulated.
func (a *App) onClassEndAck(sessionID string, p protocol.ClassEndAck) {
	a.endMu.Lock()
	ok := a.ends.Ack(sessionID, p)
	a.endMu.Unlock()
	if ok {
		a.log.Info("class end acknowledged", "session", sessionID, "status", p.Status, "reason", p.Reason)
		a.emit(EventClassEnd, a.ends.View())
	}
}

// syncClassEnd sends class.end to a computer that reconnected without
// having acknowledged it, before anything else is resent (§5.25.3).
func (a *App) syncClassEnd(class classroom) {
	endID := a.ends.EndID()
	if endID == "" {
		return
	}
	var owed []endSession
	for _, s := range a.endSessions(class) {
		if s.connected && a.ends.Owed(s.Hostname, s.ID) {
			owed = append(owed, s)
		}
	}
	if len(owed) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, s := range owed {
		a.log.Info("class end resent on reconnecting", "session", s.ID)
		a.sendClassEnd(ctx, class, s, endID)
	}
	a.expireClassEnd()
	a.emit(EventClassEnd, a.ends.View())
}

// clearConsoleClass leaves the console «sin clase»: what the class made is
// forgotten (in memory only; no folder is touched) and the next class starts
// from scratch. The channel and the connections stay.
func (a *App) clearConsoleClass() {
	a.board.Reset() // hands and messages (rule 51)
	a.emit(EventParticipation, a.participation(nil, nil))
	a.snaps.Reset() // unsaved screenshots (rule 45)
	a.stopFiles()
	a.files.mu.Lock()
	a.files.sends, a.files.pushes, a.files.byDel = nil, map[string]string{}, map[string]string{}
	a.files.mu.Unlock()
	a.emitFiles()
	a.stopCollections() // their folders stay
	a.opens.Clear()
	a.emit(EventOpens, a.OpenOrders())
	a.pauses.Reset()
	a.resetTimer()
	a.resetPolls()
	a.mu.Lock()
	a.history, a.deliveries, a.payloads = &codedist.History{}, &codedist.Deliveries{}, nil
	a.mu.Unlock()
	a.emitHistory()
	a.emitDeliveries()
	if class := a.currentClass(); class != nil {
		a.emitPauses(class)
		a.emit(EventStudents, a.studentsIn(class))
	}
}
