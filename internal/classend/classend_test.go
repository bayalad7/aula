package classend

import (
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestClassAcksAndOwed(t *testing.T) {
	clk := &clock{t: time.Date(2026, 11, 12, 10, 52, 0, 0, time.UTC)}
	c := NewClass(clk.now)
	end, ok := c.Begin()
	if !ok || end == "" {
		t.Fatal("Begin failed")
	}
	if _, ok := c.Begin(); ok {
		t.Fatal("a second Begin while running must be refused")
	}
	c.Sent("PC01", "s1", "e1", false)
	c.Sent("PC02", "s2", "e2", false)
	c.Sent("SIM01", "s3", "e3", true)
	c.Partial("PC03", "s4", false)
	c.Away("PC04", false)
	c.AddStep(Step{Kind: StepTimer, Status: StepDone, Count: 1})
	c.Done()

	if !c.Ack("s1", protocol.ClassEndAck{RefID: "e1", EndID: end, Status: protocol.ClassEndClean}) {
		t.Fatal("ack of PC01 not applied")
	}
	if c.Ack("s1", protocol.ClassEndAck{RefID: "e1", EndID: protocol.NewID(), Status: protocol.ClassEndClean}) {
		t.Fatal("an ack of another end must be ignored")
	}
	if c.Ack("s9", protocol.ClassEndAck{RefID: "e2", EndID: end, Status: protocol.ClassEndClean}) {
		t.Fatal("an ack from another session must be ignored")
	}
	c.Ack("s2", protocol.ClassEndAck{RefID: "e2", EndID: end, Status: protocol.ClassEndFailed, Reason: protocol.ClassEndRestrictions})

	clk.t = clk.t.Add(AckWait)
	if !c.Expire() {
		t.Fatal("SIM01 should be «sin acuse»")
	}
	v := c.View()
	want := map[string]State{"PC01": StateClean, "PC02": StateFailed, "PC03": StatePartial, "PC04": StateAway, "SIM01": StateNoAck}
	for _, s := range v.Students {
		if s.State != want[s.Hostname] {
			t.Errorf("%s: state %s, want %s", s.Hostname, s.State, want[s.Hostname])
		}
	}
	if v.Clean != 2 || v.Missing != 3 || v.Running || len(v.Steps) != 1 {
		t.Errorf("view = %+v", v)
	}
	if got := c.Retry(); len(got) != 2 || got[0] != "PC02" || got[1] != "SIM01" {
		t.Errorf("Retry = %v", got)
	}
	// Owed on a new connection only.
	if !c.Owed("PC04", "s5") || !c.Owed("PC02", "s6") || c.Owed("PC02", "s2") {
		t.Error("owed: away and failed computers get it on a new connection only")
	}
	if c.Owed("PC01", "s7") || c.Owed("PC03", "s8") || c.Owed("PC99", "s9") {
		t.Error("clean, partial and unknown computers are owed nothing")
	}
	c.Sent("PC04", "s5", "e5", false)
	c.Ack("s5", protocol.ClassEndAck{RefID: "e5", EndID: end, Status: protocol.ClassEndClean})
	if c.Owed("PC04", "s10") {
		t.Error("PC04 acknowledged after reconnecting")
	}
	// Away never downgrades a computer that already answered.
	c.Away("PC01", false)
	if c.View().Students[0].State != StateClean {
		t.Error("Away overwrote a clean computer")
	}
}

func TestClassGoneAndReset(t *testing.T) {
	c := NewClass(nil)
	c.Begin()
	c.Sent("PC01", "s1", "e1", false)
	if !c.Gone("s1") || !c.Owed("PC01", "s2") {
		t.Fatal("a session that left while waiting is owed on reconnecting")
	}
	c.Done()
	// A new end forgets who the previous one owed.
	c.Begin()
	if c.Owed("PC01", "s2") {
		t.Fatal("Begin must forget the previous end")
	}
	c.Reset()
	if v := c.View(); v.EndID != "" || len(v.Students) != 0 {
		t.Fatalf("Reset left %+v", v)
	}
}

func TestAgentAckAndCard(t *testing.T) {
	p := protocol.ClassEnd{EndID: protocol.NewID()}
	ref := protocol.NewID()
	if a := Ack(ref, p); a.Status != protocol.ClassEndClean || a.Validate() != nil {
		t.Errorf("clean ack = %+v (%v)", a, a.Validate())
	}
	a := Ack(ref, p, protocol.ClassEndPause, protocol.ClassEndRestrictions)
	if a.Status != protocol.ClassEndFailed || a.Reason != protocol.ClassEndPause || a.Validate() != nil {
		t.Errorf("failed ack = %+v", a)
	}
	at := time.Date(2026, 11, 12, 10, 52, 0, 0, time.UTC)
	card := NewCard(p, at, Removed{Timer: true, Snippets: 2})
	if card.EndID != p.EndID || card.At != at.UnixMilli() || card.Removed.Empty() {
		t.Errorf("card = %+v", card)
	}
	if !(Removed{}).Empty() {
		t.Error("an empty Removed must be Empty")
	}
}
