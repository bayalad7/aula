package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// --- Power: shut down, reboot, log off (docs/protocol.md §5.15) --------------------

// PowerRequest is what the Energía panel sends, after the teacher confirmed.
type PowerRequest struct {
	Action string   `json:"action"` // "shutdown" | "reboot" | "logoff"
	Hosts  []string `json:"hosts"`  // marked students; empty: the whole channel
	DelayS int      `json:"delayS"` // countdown; 0: 30 s
	Force  bool     `json:"force"`  // no countdown, applications closed without saving
}

// PowerResult says what happened to an order or a cancellation.
type PowerResult struct {
	Sent        int      `json:"sent"`
	Unsupported []string `json:"unsupported"` // hostnames without power
	Failed      []string `json:"failed"`      // hostnames the message did not reach
}

// PowerAction sends a power order to the class or to the marked students.
// Each card then follows the agent's power.ack.
func (a *App) PowerAction(req PowerRequest) (PowerResult, error) {
	class := a.currentClass()
	if class == nil {
		return PowerResult{}, errNoClass
	}
	order := protocol.PowerAction{ActionID: protocol.NewID(), Action: protocol.PowerKind(req.Action), DelayS: req.DelayS, Force: req.Force}
	if err := order.Validate(); err != nil {
		return PowerResult{}, fmt.Errorf("orden de energía no válida: elige apagar, reiniciar o cerrar sesión y una cuenta de %d s a %d min",
			protocol.MinPowerDelay, protocol.MaxPowerDelay/60)
	}
	res := PowerResult{Unsupported: []string{}, Failed: []string{}}
	targets := targetsOf(class.Sessions(), req.Hosts)
	if len(targets) == 0 {
		return res, errors.New("no hay alumnos conectados a quienes enviarla")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, s := range targets {
		if !s.Can(protocol.CapPower) {
			res.Unsupported = append(res.Unsupported, s.Hostname)
			continue
		}
		envID, err := class.Send(ctx, s.ID, order)
		if err != nil {
			a.log.Debug("power order not sent", "session", s.ID, "err", err)
			res.Failed = append(res.Failed, s.Hostname)
			continue
		}
		a.power.Sent(s.ID, envID, order)
		res.Sent++
	}
	a.log.Info("power order", "action", order.Action, "action_id", order.ActionID, "force", order.Force,
		"delay_s", order.Delay(), "sent", res.Sent, "unsupported", len(res.Unsupported))
	a.emit(EventStudents, a.studentsOf(class.Sessions()))
	return res, nil
}

// PowerCancel cancels the countdowns still pending in the class or in the
// marked students.
func (a *App) PowerCancel(hosts []string) PowerResult {
	res := PowerResult{Unsupported: []string{}, Failed: []string{}}
	class := a.currentClass()
	if class == nil {
		return res
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, s := range targetsOf(class.Sessions(), hosts) {
		e, ok := a.power.Get(s.ID)
		if !ok || !e.Pending() {
			continue
		}
		if _, err := class.Send(ctx, s.ID, protocol.PowerCancel{ActionID: e.ActionID}); err != nil {
			res.Failed = append(res.Failed, s.Hostname)
			continue
		}
		res.Sent++
	}
	a.log.Info("power orders cancelled", "sent", res.Sent)
	return res
}

// onPowerMessage takes the agents' power.ack and the errors that answer a
// power order; their cards show it.
func (a *App) onPowerMessage(sessionID string, env protocol.Envelope) {
	changed := false
	switch p := env.Payload.(type) {
	case protocol.PowerAck:
		changed = a.power.Ack(sessionID, p)
	case protocol.ErrorPayload:
		changed = a.power.Error(sessionID, p.RefID)
	}
	if !changed {
		return
	}
	if class := a.currentClass(); class != nil {
		a.emit(EventStudents, a.studentsOf(class.Sessions()))
	}
}

// targetsOf returns the connected sessions among hosts (by hostname, without
// case); all the connected ones when hosts is empty.
func targetsOf(list []session.Session, hosts []string) []session.Session {
	want := map[string]bool{}
	for _, h := range hosts {
		want[strings.ToLower(strings.TrimSpace(h))] = true
	}
	var out []session.Session
	for _, s := range list {
		if s.State != session.StateConnected {
			continue
		}
		if len(want) == 0 || want[strings.ToLower(s.Hostname)] {
			out = append(out, s)
		}
	}
	return out
}
