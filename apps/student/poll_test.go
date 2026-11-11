package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/poll"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// The teacher opens a poll: the card arrives with notify (never raise), the
// student answers and changes the answer, the teacher closes it and shares
// the totals. The question, the options and the answer never reach the log
// (docs/privacidad.md rule 74).
func TestPollWithLiveConsole(t *testing.T) {
	got := &heard{}
	c := startLiveConsoleWith(t, got.onMessage)
	cfg := Config{Channel: "1", ConsolePublicKey: c.key, Direct: c.addr}
	pub, _ := cfg.Validate()
	a, rec := newTestApp(nil)
	logs := &bytes.Buffer{}
	a.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	a.cfg, a.pub = cfg, pub

	if _, err := a.AnswerPoll(protocol.NewID(), 0, ""); err == nil || err.Error() != "aula:"+pollNoConnection {
		t.Fatalf("AnswerPoll before connecting = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx)
	defer a.stop()
	s := c.waitAgent(t, "agent", func(s session.Session) bool { return s.Can(protocol.CapPoll) })
	waitConnected(t, a)

	ackOf := func(what, refID string, st protocol.PollAckStatus) {
		t.Helper()
		got.wait(t, what, func(e protocol.Envelope) bool {
			p, ok := e.Payload.(protocol.PollAck)
			return ok && p.RefID == refID && p.Status == st
		})
	}
	voteWith := func(what string, cond func(protocol.PollVote) bool) protocol.Envelope {
		t.Helper()
		return got.wait(t, what, func(e protocol.Envelope) bool {
			p, ok := e.Payload.(protocol.PollVote)
			return ok && cond(p)
		})
	}

	pollID := protocol.NewID()
	openID, err := c.console.Send(ctx, s.ID, protocol.PollOpen{PollID: pollID, Kind: protocol.PollKindChoice,
		Question: "¿Pregunta secreta?", Options: []string{"opción secreta", "otra", "tercera"}})
	if err != nil {
		t.Fatal(err)
	}
	ackOf("shown", openID, protocol.PollAckShown)
	waitFor(t, "the arrived view", func() bool {
		v, ok := rec.last(EventPolls).(poll.BallotView)
		return ok && v.Arrived == pollID && len(v.Cards) == 1 && v.Cards[0].Open
	})

	if _, err := a.AnswerPoll(pollID, 7, ""); err == nil || err.Error() != "aula:"+pollBadVote {
		t.Fatalf("an option that does not exist = %v", err)
	}
	v, err := a.AnswerPoll(pollID, 0, "")
	if err != nil || v.Cards[0].Vote != poll.VoteSending {
		t.Fatalf("AnswerPoll = %+v, %v", v, err)
	}
	env := voteWith("the vote", func(p protocol.PollVote) bool { return p.Choice != nil && *p.Choice == 0 })
	if _, err := c.console.Send(ctx, s.ID, protocol.PollVoteAck{RefID: env.ID, PollID: pollID, Status: protocol.PollVoteAccepted}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "accepted", func() bool { return a.View().Polls.Cards[0].Vote == poll.VoteAccepted })

	closeID, err := c.console.Send(ctx, s.ID, protocol.PollClose{PollID: pollID})
	if err != nil {
		t.Fatal(err)
	}
	ackOf("closed", closeID, protocol.PollAckClosed)
	if _, err := a.AnswerPoll(pollID, 1, ""); err == nil || err.Error() != "aula:"+pollClosed {
		t.Fatalf("answering a closed poll = %v", err)
	}
	resID, err := c.console.Send(ctx, s.ID, protocol.PollResults{PollID: pollID, Counts: []int{3, 1, 0}, Voters: 4, Recipients: 5})
	if err != nil {
		t.Fatal(err)
	}
	ackOf("results", resID, protocol.PollAckResults)
	if c := a.View().Polls.Cards[0]; c.Results == nil || c.Results.Voters != 4 || c.Open || c.Choice != 0 {
		t.Fatalf("after the results = %+v", c)
	}

	// An open question: the text goes to the console, never to the log.
	openQ := protocol.NewID()
	if _, err := c.console.Send(ctx, s.ID, protocol.PollOpen{PollID: openQ, Kind: protocol.PollKindOpen, Question: "¿Qué?", Named: true}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the second card", func() bool { return len(a.View().Polls.Cards) == 2 })
	if _, err := a.AnswerPoll(openQ, 0, "   "); err == nil || err.Error() != "aula:"+pollEmpty {
		t.Fatalf("an empty answer = %v", err)
	}
	waitFor(t, "a second to pass", func() bool {
		_, err := a.AnswerPoll(openQ, 0, "respuesta secreta")
		return err == nil
	})
	voteWith("the text", func(p protocol.PollVote) bool { return p.Text == "respuesta secreta" })

	rec.mu.Lock()
	notified, raised := rec.notified, rec.raised
	rec.mu.Unlock()
	if raised != 0 || notified < 2 {
		t.Errorf("notified %d, raised %d: a poll notifies and never raises", notified, raised)
	}
	if strings.Contains(logs.String(), "secret") {
		t.Errorf("the poll text is in the log:\n%s", logs.String())
	}
}
