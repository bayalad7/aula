package main

import (
	"context"
	"testing"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// The service, with nobody logged on, declares health, reports after the
// first health.config and answers «Revisar ahora» (§5.27): it reads the
// computer itself, with no helper. The report is only of the closed list
// and has no Documents disk without a session.
func TestServiceAgentReportsHealthWithoutSession(t *testing.T) {
	got := &heard{}
	c := startLiveConsoleWith(t, got.onMessage)
	cfg := Config{Channel: "1", ConsolePublicKey: c.key, Direct: c.addr}
	pub, _ := cfg.Validate()
	sa := newServiceAgent(cfg, pub, nil, quiet)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sa.app.start(ctx)
	defer sa.app.stop()

	s := c.waitAgent(t, "agent without helper", func(s session.Session) bool { return s.Can(protocol.CapHealth) })
	if s.Username != "" {
		t.Fatalf("user %q without a session", s.Username)
	}
	if _, err := c.console.Send(ctx, s.ID, protocol.HealthConfig{}); err != nil {
		t.Fatal(err)
	}
	first := got.wait(t, "first report", func(e protocol.Envelope) bool {
		r, ok := e.Payload.(protocol.HealthReport)
		return ok && r.Reason == protocol.HealthFirst
	}).Payload.(protocol.HealthReport)
	for _, d := range first.Disks {
		if d.Role == protocol.DiskDocuments {
			t.Fatal("a Documents disk without a session")
		}
	}
	req, err := c.console.Send(ctx, s.ID, protocol.HealthRequest{})
	if err != nil {
		t.Fatal(err)
	}
	got.wait(t, "answer to «Revisar ahora»", func(e protocol.Envelope) bool {
		r, ok := e.Payload.(protocol.HealthReport)
		return ok && r.Reason == protocol.HealthOnDemand && r.RefID == req && r.Seq == first.Seq+1
	})
}
