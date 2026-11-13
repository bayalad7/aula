package simclass

import (
	"context"
	"time"

	"github.com/bayalad7/aula/internal/poll"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/transport"
)

// Live polls (docs/protocol.md §5.23): a simulated student shows the poll like
// the real window and answers it by itself after a moment, so the teacher sees
// the bars move without a lab. Its answer is marked simulated by the console
// and never enters the totals shared with the real students. "noack" shows
// the poll without acknowledging it (it still answers); "slow" answers late.

// SampleAnswers are what a simulated student answers to an open question.
var SampleAnswers = []string{
	"El ciclo for",
	"Nada, todo claro",
	"Los índices de la lista",
	"Cuándo usar while",
}

// pollAnswerAfter is how long a simulated student takes to answer: from 1 s,
// a little more for each student, so the bars move one by one.
func (s *student) pollAnswerAfter() time.Duration {
	return time.Second + time.Duration(s.index%5)*400*time.Millisecond
}

// handlePoll applies poll.*; it reports whether env was one.
func (s *student) handlePoll(ctx context.Context, cl *transport.Client, env protocol.Envelope, fault Fault) bool {
	switch p := env.Payload.(type) {
	case protocol.PollOpen:
		ack, resend, isNew := s.ballot.Open(env.ID, p)
		if fault != FaultNoAck {
			s.sendPoll(ctx, cl, ack)
		}
		if resend != nil {
			s.sendVote(ctx, cl, *resend)
		}
		if isNew && p.Kind == protocol.PollKindQuiz {
			time.AfterFunc(s.pollAnswerAfter(), func() { s.answerQuiz(p) })
		} else if isNew {
			time.AfterFunc(s.pollAnswerAfter(), func() { s.answerPoll(p) })
		}
	case protocol.PollClose:
		if ack, ok := s.ballot.Close(env.ID, p); ok && fault != FaultNoAck {
			s.sendPoll(ctx, cl, ack)
		}
	case protocol.PollResults:
		if ack, ok := s.ballot.Results(env.ID, p); ok && fault != FaultNoAck {
			s.sendPoll(ctx, cl, ack)
		}
	case protocol.PollQuizResult:
		if ack, ok := s.ballot.QuizResult(env.ID, p); ok && fault != FaultNoAck {
			s.sendPoll(ctx, cl, ack)
		}
	case protocol.PollVoteAck:
		s.ballot.VoteAck(p)
	case protocol.ErrorPayload:
		if !s.ballot.Refused(p) {
			return false
		}
	default:
		return false
	}
	s.class.changed()
	return true
}

// answerPoll picks an answer the way a class would: different students,
// different options.
func (s *student) answerPoll(p protocol.PollOpen) {
	choice, text := 0, ""
	switch p.Kind {
	case protocol.PollKindChoice:
		choice = (s.index + len(p.Question)) % len(p.Options)
	case protocol.PollKindYesNo:
		if s.index%4 == 3 {
			choice = 1
		}
	case protocol.PollKindOpen:
		text = SampleAnswers[s.index%len(SampleAnswers)]
	}
	v, err := s.ballot.Vote(p.PollID, choice, text)
	if err != nil {
		return // closed meanwhile
	}
	cl := s.connection()
	if cl == nil {
		s.ballot.Unsent(v)
		return
	}
	ctx, cancel := context.WithTimeout(s.class.ctx, 5*time.Second)
	defer cancel()
	s.sendVote(ctx, cl, v)
	s.class.changed()
}

// answerQuiz answers every question of a quiz by chance (docs/privacidad.md
// rule 91: the simulated students do not know the correct options either)
// and hands it in.
func (s *student) answerQuiz(p protocol.PollOpen) {
	var ans []protocol.PollQuizAnswer
	for i, q := range p.Questions {
		pick := (s.index*7 + i*3 + len(q.Text)) % q.OptionCount()
		choices := []int{pick}
		if q.Kind == protocol.PollKindChoice {
			choices = []int{q.Options[pick].ID}
			if q.Multi && s.index%2 == 0 {
				other := q.Options[(pick+1)%len(q.Options)].ID
				choices = append(choices, other)
			}
		}
		ans = append(ans, protocol.PollQuizAnswer{Q: q.ID, Choices: choices})
	}
	if s.ballot.AnswerQuiz(p.PollID, ans, true) != nil {
		return // closed meanwhile
	}
	s.flushQuiz(p.PollID)
}

// flushQuiz sends the quiz answers waiting, at the pace of the ballot.
func (s *student) flushQuiz(pollID string) {
	v, wait := s.ballot.NextQuizVote(pollID)
	if wait > 0 {
		time.AfterFunc(wait, func() { s.flushQuiz(pollID) })
		return
	}
	if v == nil {
		return
	}
	cl := s.connection()
	if cl == nil {
		s.ballot.Unsent(*v)
		return
	}
	ctx, cancel := context.WithTimeout(s.class.ctx, 5*time.Second)
	defer cancel()
	s.sendVote(ctx, cl, *v)
	s.class.changed()
}

func (s *student) sendVote(ctx context.Context, cl *transport.Client, v protocol.PollVote) {
	id, err := cl.Send(ctx, v)
	if err != nil {
		s.class.log.Debug("cannot send poll.vote", "student", s.name, "err", err)
		s.ballot.Unsent(v)
		return
	}
	s.ballot.Sending(id, v)
}

func (s *student) sendPoll(ctx context.Context, cl *transport.Client, ack protocol.PollAck) {
	if _, err := cl.Send(ctx, ack); err != nil {
		s.class.log.Debug("cannot send poll.ack", "student", s.name, "err", err)
	}
}

// pollView is the newest poll card of the student, if any.
func (s *student) pollView() *poll.Card {
	v := s.ballot.View()
	if len(v.Cards) == 0 {
		return nil
	}
	return &v.Cards[0]
}
