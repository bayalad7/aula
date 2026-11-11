package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/poll"
	"github.com/bayalad7/aula/internal/protocol"
)

func pollHarness(t *testing.T) (*harness, *bytes.Buffer) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapMsgShow, protocol.CapPoll),
		student("s2", "PC02", protocol.CapMsgShow, protocol.CapPoll),
		student("s3", "PC03", protocol.CapMsgShow)) // an older agent
	logs := &bytes.Buffer{}
	h.app.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return h, logs
}

func ackOfVote(t *testing.T, h *harness, sessionID string) protocol.PollVoteAck {
	t.Helper()
	for _, s := range h.class.takeDirect() {
		if a, ok := s.p.(protocol.PollVoteAck); ok && s.sessionID == sessionID {
			return a
		}
	}
	t.Fatalf("no poll.vote.ack to %s", sessionID)
	return protocol.PollVoteAck{}
}

// «Lanzar» sends poll.open to who can show it; votes count once per student
// (the last one), «Cerrar» stops them and «Compartir resultados» sends only
// totals. The question, the options and the answers never reach the log.
func TestPollOpenVoteCloseShare(t *testing.T) {
	h, logs := pollHarness(t)
	v, err := h.app.OpenPoll(poll.Request{Kind: protocol.PollKindChoice, Question: "¿Pregunta secreta?",
		Options: []string{"opción secreta", "otra", "  "}})
	if err != nil {
		t.Fatal(err)
	}
	p := v.Current
	if p == nil || !p.All || p.Eligible != 2 || len(p.Unsupported) != 1 || p.Unsupported[0] != "PC03" || len(p.Options) != 2 {
		t.Fatalf("view = %+v", p)
	}
	sent := h.class.takeDirect()
	if len(sent) != 2 {
		t.Fatalf("sent %+v", sent)
	}
	open, ok := sent[0].p.(protocol.PollOpen)
	if !ok || open.Question != "¿Pregunta secreta?" || len(open.Options) != 2 || open.Named {
		t.Fatalf("poll.open = %+v", sent[0].p)
	}
	h.hooks.onPoll("s1", protocol.Envelope{ID: protocol.NewID(), Payload: protocol.PollAck{RefID: sent[0].envID, PollID: open.PollID, Status: protocol.PollAckShown}})
	if ev := h.rec.last(EventPolls).(poll.View); ev.Current.Received != 1 {
		t.Fatalf("received after an ack = %+v", ev.Current)
	}

	vote := func(sessionID string, n int) protocol.PollVoteAck {
		t.Helper()
		h.hooks.onPoll(sessionID, protocol.Envelope{ID: protocol.NewID(), Payload: protocol.PollVote{PollID: open.PollID, Choice: &n}})
		return ackOfVote(t, h, sessionID)
	}
	if a := vote("s1", 0); a.Status != protocol.PollVoteAccepted {
		t.Fatalf("vote = %+v", a)
	}
	vote("s2", 1)
	vote("s1", 1) // changes its mind: still one vote
	ev := h.rec.last(EventPolls).(poll.View).Current
	if ev.Answered != 2 || ev.Options[0].Count != 0 || ev.Options[1].Count != 2 || len(ev.Answers) != 0 {
		t.Fatalf("totals = %+v (an anonymous poll lists no answers)", ev)
	}
	// An option that does not exist is an invalid message, not a vote.
	n := 5
	h.hooks.onPoll("s2", protocol.Envelope{ID: protocol.NewID(), Payload: protocol.PollVote{PollID: open.PollID, Choice: &n}})
	if e, ok := h.class.takeDirect()[0].p.(protocol.ErrorPayload); !ok || e.Code != protocol.CodeInvalidMessage {
		t.Fatalf("an invalid vote answered %+v", e)
	}

	if _, err := h.app.SharePollResults(open.PollID); errorCode(err) != codePollNotShareable {
		t.Errorf("sharing an open poll = %v", err)
	}
	v, err = h.app.ClosePoll()
	if err != nil || v.Current != nil || len(v.Closed) != 1 || !v.Closed[0].Shareable {
		t.Fatalf("ClosePoll = %+v, %v", v, err)
	}
	sent = h.class.takeDirect()
	if len(sent) != 2 {
		t.Fatalf("close sent %+v", sent)
	}
	if _, ok := sent[0].p.(protocol.PollClose); !ok {
		t.Fatalf("poll.close = %+v", sent[0].p)
	}
	if a := vote("s2", 0); a.Status != protocol.PollVoteClosed {
		t.Fatalf("a vote after the close = %+v", a)
	}
	if _, err := h.app.ClosePoll(); errorCode(err) != codePollGone {
		t.Errorf("second close = %v", err)
	}

	if _, err := h.app.SharePollResults(open.PollID); err != nil {
		t.Fatal(err)
	}
	sent = h.class.takeDirect()
	if len(sent) != 2 {
		t.Fatalf("share sent %+v", sent)
	}
	res, ok := sent[0].p.(protocol.PollResults)
	if !ok || res.Voters != 2 || res.Counts[1] != 2 || res.Recipients != 2 {
		t.Fatalf("poll.results = %+v", sent[0].p)
	}
	if _, err := h.app.SharePollResults(protocol.NewID()); errorCode(err) != codePollGone {
		t.Errorf("sharing an unknown poll = %v", err)
	}
	if strings.Contains(logs.String(), "secreta") {
		t.Errorf("the poll text is in the log:\n%s", logs.String())
	}
}

func TestPollErrors(t *testing.T) {
	h, _ := pollHarness(t)
	if _, err := h.app.OpenPoll(poll.Request{Kind: protocol.PollKindYesNo, Question: "¿Sí?", Hosts: []string{"PC03"}}); errorCode(err) != codeNoPollTargets {
		t.Errorf("only an older agent = %v", err)
	}
	if _, err := h.app.OpenPoll(poll.Request{Kind: protocol.PollKindChoice, Question: "¿?", Options: []string{"solo una"}}); errorCode(err) != codeInvalidPoll {
		t.Errorf("one option = %v", err)
	}
	// A named open question, to a selection: only PC02 gets it.
	v, err := h.app.OpenPoll(poll.Request{Kind: protocol.PollKindOpen, Question: "¿Qué?", Named: true, Hosts: []string{"pc02"}})
	if err != nil || v.Current.All || v.Current.Eligible != 1 || len(v.Current.Missing) != 1 {
		t.Fatalf("selection = %+v, %v", v.Current, err)
	}
	sent := h.class.takeDirect()
	if len(sent) != 1 || sent[0].sessionID != "s2" {
		t.Fatalf("sent %+v", sent)
	}
	id := sent[0].p.(protocol.PollOpen).PollID
	h.hooks.onPoll("s2", protocol.Envelope{ID: protocol.NewID(), Payload: protocol.PollVote{PollID: id, Text: "el for"}})
	if a := ackOfVote(t, h, "s2"); a.Status != protocol.PollVoteAccepted {
		t.Fatalf("vote = %+v", a)
	}
	h.hooks.onPoll("s1", protocol.Envelope{ID: protocol.NewID(), Payload: protocol.PollVote{PollID: id, Text: "yo no"}})
	if a := ackOfVote(t, h, "s1"); a.Status != protocol.PollVoteUnknown {
		t.Fatalf("a vote from outside the selection = %+v", a)
	}
	c := h.app.Polls().Current
	if len(c.Answers) != 1 || c.Answers[0].Hostname != "PC02" || c.Answers[0].Text != "el for" {
		t.Fatalf("named answers = %+v", c.Answers)
	}
	h.app.ClosePoll()
	if _, err := h.app.SharePollResults(id); errorCode(err) != codePollNotShareable {
		t.Errorf("sharing open answers = %v", err)
	}
}
