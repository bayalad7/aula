package main

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/poll"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/transport"
)

// --- Live polls (docs/protocol.md §5.23) ------------------------------------------
//
// The poll arrives as a card on top of the list with «Nuevo»: notify, never
// raise (prompt 19; docs/privacidad.md rule 71). It does not observe or
// control this computer, so it has no band. The student answers with the
// buttons of the card and «Enviar», and may change the answer until it
// closes. Go sends states; the window words them. The question, the options
// and the answer never reach the log.

// EventPolls carries a poll.BallotView whenever a poll arrives (Arrived set),
// closes, gets its totals or the student's answer changes state.
const EventPolls = "aula:polls"

// Errors of AnswerPoll. The window shows encuesta.error.<code>
// (textos.es-MX.js); on the wire they read "aula:<code>".
const (
	pollNoConnection = "noConnection"
	pollUnknown      = "unknown"
	pollClosed       = "closed"
	pollEmpty        = "empty"
	pollTooLong      = "tooLong"
	pollBadVote      = "badVote"
	pollTooSoon      = "tooSoon"
	pollUnsupported  = "unsupported" // an older console
	pollHandedIn     = "handedIn"    // a quiz already handed in (§5.23.1)
)

// pollErrorCodes lists them all, for the texts test.
var pollErrorCodes = []string{pollNoConnection, pollUnknown, pollClosed, pollEmpty, pollTooLong, pollBadVote, pollTooSoon, pollUnsupported, pollHandedIn}

func pollErrorOf(err error) error {
	for target, code := range map[error]string{
		poll.ErrUnknown: pollUnknown, poll.ErrClosed: pollClosed, poll.ErrEmpty: pollEmpty,
		poll.ErrTooLong: pollTooLong, poll.ErrBadVote: pollBadVote, poll.ErrTooSoon: pollTooSoon,
		poll.ErrNoAnswer: pollUnsupported, poll.ErrHandedIn: pollHandedIn,
	} {
		if errors.Is(err, target) {
			return partError(code)
		}
	}
	return err
}

// handlePoll applies poll.open, poll.close, poll.results, poll.vote.ack and
// the errors to an answer.
func (a *App) handlePoll(ctx context.Context, cl *transport.Client, env protocol.Envelope) bool {
	var ack *protocol.PollAck
	arrived := ""
	switch p := env.Payload.(type) {
	case protocol.PollOpen:
		got, resend, isNew := a.ballot.Open(env.ID, p)
		ack = &got
		if isNew {
			arrived = p.PollID
			a.log.Info("poll arrived", "poll", p.PollID, "kind", p.Kind, "named", p.Named, "options", len(p.Options), "questions", len(p.Questions))
		}
		if resend != nil {
			a.sendVote(ctx, cl, *resend)
		}
		if p.Kind == protocol.PollKindQuiz {
			a.flushQuiz(p.PollID) // answers marked while disconnected
		}
	case protocol.PollClose:
		got, ok := a.ballot.Close(env.ID, p)
		if !ok {
			return true
		}
		ack = &got
		a.log.Info("poll closed", "poll", p.PollID)
	case protocol.PollResults:
		got, ok := a.ballot.Results(env.ID, p)
		if !ok {
			return true
		}
		ack = &got
		a.log.Info("poll results", "poll", p.PollID, "voters", p.Voters, "recipients", p.Recipients)
	case protocol.PollQuizResult:
		got, ok := a.ballot.QuizResult(env.ID, p)
		if !ok {
			return true
		}
		ack = &got
		// Never the score: only that it arrived (rule 91).
		a.log.Info("quiz result", "poll", p.PollID)
	case protocol.PollVoteAck:
		if !a.ballot.VoteAck(p) {
			return true
		}
		a.log.Info("poll answer acknowledged", "poll", p.PollID, "status", p.Status)
	case protocol.ErrorPayload:
		if !a.ballot.Refused(p) {
			return false
		}
		a.log.Info("the console refused a poll answer", "code", p.Code)
	default:
		return false
	}
	if ack != nil {
		if _, err := cl.Send(ctx, *ack); err != nil {
			a.log.Debug("cannot send poll.ack", "err", err)
		}
	}
	a.emitPolls(arrived)
	if arrived != "" {
		a.notify() // never raise: the student keeps the keyboard
	}
	return true
}

func (a *App) sendVote(ctx context.Context, cl *transport.Client, v protocol.PollVote) {
	id, err := cl.Send(ctx, v)
	if err != nil {
		a.log.Debug("cannot send poll.vote", "err", err)
		a.ballot.Unsent(v)
		return
	}
	a.ballot.Sending(id, v)
}

func (a *App) emitPolls(arrived string) {
	v := a.ballot.View()
	v.Arrived = arrived
	a.emit(EventPolls, v)
}

// AnswerPoll sends the student's answer to a poll: choice (from 0) for a
// multiple-choice or yes/no poll, text for an open one. Answering again
// replaces the answer until the poll closes.
func (a *App) AnswerPoll(pollID string, choice int, text string) (poll.BallotView, error) {
	if a.remote != nil {
		var v poll.BallotView
		err := a.forward("AnswerPoll", &v, pollID, choice, text)
		return v, err
	}
	cl := a.connection()
	if cl == nil {
		return a.ballot.View(), partError(pollNoConnection)
	}
	v, err := a.ballot.Vote(pollID, choice, text)
	if err != nil {
		return a.ballot.View(), pollErrorOf(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// If it cannot go now it waits, "sending", for the console to send the
	// poll again on the next connection.
	a.sendVote(ctx, cl, v)
	a.log.Info("poll answered", "poll", pollID, "text_len", utf8.RuneCountInString(v.Text))
	a.emitPolls("")
	return a.ballot.View(), nil
}

// AnswerQuiz keeps the student's answers to a quiz (all of them so far: the
// question ID and the option IDs) and, with submit, hands it in. The window
// calls it each time the student marks an option and with «Entregar»;
// the answers go at most once per second, only the last (§5.23.1).
func (a *App) AnswerQuiz(pollID string, answers []protocol.PollQuizAnswer, submit bool) (poll.BallotView, error) {
	if a.remote != nil {
		var v poll.BallotView
		err := a.forward("AnswerQuiz", &v, pollID, answers, submit)
		return v, err
	}
	if a.connection() == nil {
		return a.ballot.View(), partError(pollNoConnection)
	}
	if err := a.ballot.AnswerQuiz(pollID, answers, submit); err != nil {
		return a.ballot.View(), pollErrorOf(err)
	}
	a.flushQuiz(pollID)
	a.log.Info("quiz answered", "poll", pollID, "answers", len(answers), "submitted", submit)
	a.emitPolls("")
	return a.ballot.View(), nil
}

// flushQuiz sends the quiz answers waiting, at the pace of the ballot; if
// it is too soon it tries again when it is time.
func (a *App) flushQuiz(pollID string) {
	v, wait := a.ballot.NextQuizVote(pollID)
	if wait > 0 {
		time.AfterFunc(wait, func() {
			a.flushQuiz(pollID)
			a.emitPolls("")
		})
		return
	}
	if v == nil {
		return
	}
	cl := a.connection()
	if cl == nil {
		a.ballot.Unsent(*v) // the console sends the quiz again on the next connection
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	a.sendVote(ctx, cl, *v)
}
