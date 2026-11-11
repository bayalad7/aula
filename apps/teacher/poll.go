package main

import (
	"context"
	"errors"
	"time"

	"github.com/bayalad7/aula/internal/poll"
	"github.com/bayalad7/aula/internal/protocol"
)

// --- Live polls (docs/protocol.md §5.23) ------------------------------------------
//
// The «Encuesta» panel of the action bar, its franja while a poll is open and
// the «Encuestas» section of «Participación». One poll open at a time, one
// answer per student (the last counts until it closes). Simulated students
// answer too, counted apart and left out of the shared totals (rules 32 and
// 73). The question, the options and the answers are never logged; in an
// anonymous poll the view never says who chose what (rule 72).

// Polls returns the polls of the class, for the franja and «Participación».
func (a *App) Polls() poll.View {
	return a.pollsSnapshot(a.currentClass())
}

// OpenPoll opens a poll for the class or the marked students; it closes the
// one in force.
func (a *App) OpenPoll(req poll.Request) (poll.View, error) {
	class := a.currentClass()
	if class == nil {
		return poll.View{}, errNoClass
	}
	if !anyCanPoll(class, req.Hosts) {
		return poll.View{}, codedf(codeNoPollTargets, "ninguno de los alumnos elegidos puede mostrar encuestas")
	}
	id, err := a.polls.Open(req)
	if err != nil {
		return poll.View{}, codedf(codeInvalidPoll, "encuesta no válida: una pregunta de hasta %d caracteres y, si es de opción múltiple, de %d a %d opciones de hasta %d",
			protocol.MaxPollQuestion, protocol.MinPollOptions, protocol.MaxPollOptions, protocol.MaxPollOption)
	}
	a.log.Info("poll opened", "poll", id, "kind", req.Kind, "named", req.Named, "options", len(req.Options), "hosts", len(req.Hosts))
	return a.pollsChanged(class), nil
}

// ClosePoll closes the poll in force: no more answers count.
func (a *App) ClosePoll() (poll.View, error) {
	class := a.currentClass()
	if class == nil {
		return poll.View{}, errNoClass
	}
	id, err := a.polls.Close()
	if err != nil {
		return poll.View{}, codedf(codePollGone, "ya no hay una encuesta abierta")
	}
	a.log.Info("poll closed", "poll", id)
	return a.pollsChanged(class), nil
}

// SharePollResults sends the totals of a closed poll to the students who got
// it («Compartir resultados»): only totals, never who chose what.
func (a *App) SharePollResults(pollID string) (poll.View, error) {
	class := a.currentClass()
	if class == nil {
		return poll.View{}, errNoClass
	}
	switch err := a.polls.Share(pollID); {
	case errors.Is(err, poll.ErrGone):
		return poll.View{}, codedf(codePollGone, "esa encuesta ya no está en la clase")
	case err != nil:
		return poll.View{}, codedf(codePollNotShareable, "solo se comparten los resultados de una encuesta cerrada de opción múltiple o de sí o no")
	}
	a.log.Info("poll results shared", "poll", pollID)
	return a.pollsChanged(class), nil
}

// onPollMessage takes poll.vote and poll.ack from a student, real or
// simulated (the session already checked the poll capability of a vote).
func (a *App) onPollMessage(sessionID string, env protocol.Envelope) {
	class := a.currentClass()
	if class == nil {
		return
	}
	switch p := env.Payload.(type) {
	case protocol.PollAck:
		if a.polls.Ack(sessionID, p) {
			a.emitPolls(class)
		}
	case protocol.PollVote:
		who, _, ok := participant(class, sessionID)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		log := a.log.With("conn", sessionID, "host", who.Hostname, "simulated", who.Simulated)
		status, err := a.polls.Vote(sessionID, poll.Student(who), p)
		var answer protocol.Payload = protocol.PollVoteAck{RefID: env.ID, PollID: p.PollID, Status: status}
		switch {
		case errors.Is(err, poll.ErrTooFast):
			answer = protocol.NewError(env.ID, protocol.CodeInvalidMessage, "too fast")
		case err != nil:
			answer = protocol.NewError(env.ID, protocol.CodeInvalidMessage, "the answer does not fit the poll")
		}
		if _, err := class.Send(ctx, sessionID, answer); err != nil {
			log.Debug("cannot answer a vote", "err", err)
		}
		if err == nil {
			// The poll, the status and whether it was a text; never what was chosen.
			log.Info("poll answered", "poll", p.PollID, "status", status, "text", p.Choice == nil)
			a.emitPolls(class)
		}
	}
}

// syncPolls brings every connected student up to date: the open poll, or the
// close and the shared totals of the last one (also after a reconnection).
func (a *App) syncPolls(class classroom) {
	cmds := a.polls.Plan(class.Sessions(), simSessionsOf(class))
	if len(cmds) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, cmd := range cmds {
		if _, err := class.Send(ctx, cmd.SessionID, cmd.Payload); err != nil {
			a.log.Debug("poll message not sent", "conn", cmd.SessionID, "type", cmd.Payload.MessageType(), "err", err)
			a.polls.Failed(cmd)
		}
	}
}

// pollsChanged sends what changed and tells the window.
func (a *App) pollsChanged(class classroom) poll.View {
	a.syncPolls(class)
	v := a.pollsSnapshot(class)
	a.emit(EventPolls, v)
	return v
}

func (a *App) emitPolls(class classroom) { a.emit(EventPolls, a.pollsSnapshot(class)) }

func (a *App) pollsSnapshot(class classroom) poll.View {
	if class == nil {
		return a.polls.Snapshot(nil, nil)
	}
	return a.polls.Snapshot(class.Sessions(), simSessionsOf(class))
}

// resetPolls forgets the polls when the class closes or changes channel;
// agents close their cards by themselves when no console sends them again
// (poll.ReconnectGrace).
func (a *App) resetPolls() {
	a.polls.Reset()
	a.emit(EventPolls, a.polls.Snapshot(nil, nil))
}

// anyCanPoll reports whether a connected student among hosts (all when
// empty), real or simulated, declared poll.
func anyCanPoll(class classroom, hosts []string) bool {
	for _, s := range targetsOf(append(class.Sessions(), simSessionsOf(class)...), hosts) {
		if s.Can(protocol.CapPoll) {
			return true
		}
	}
	return false
}
