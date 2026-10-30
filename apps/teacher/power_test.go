package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/power"
	"github.com/bayalad7/aula/internal/protocol"
)

func TestPowerTargetsAcksAndCancel(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapCodeView, protocol.CapPower),
		student("s2", "PC02", protocol.CapCodeView, protocol.CapPower),
		student("s3", "MAC03", protocol.CapCodeView)) // no power (not Windows)
	now := time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC)
	h.app.now = func() time.Time { return now }
	if st := h.app.View().Students; st.CanPower != 2 || !st.Students[0].CanPower || st.Students[2].CanPower {
		t.Fatalf("counts %+v", st)
	}

	// The whole channel: only those with power get it.
	res, err := h.app.PowerAction(PowerRequest{Action: "reboot", DelayS: 60})
	if err != nil {
		t.Fatal(err)
	}
	if res.Sent != 2 || !slices.Equal(res.Unsupported, []string{"MAC03"}) {
		t.Fatalf("result %+v", res)
	}
	ds := h.class.takeDirect()
	if got := sentTypes(ds); !slices.Equal(got, []string{"s1:power.action", "s2:power.action"}) {
		t.Fatalf("sent %v", got)
	}
	order := ds[0].p.(protocol.PowerAction)
	if order.Action != protocol.PowerReboot || order.DelayS != 60 || order.Force || ds[1].p.(protocol.PowerAction).ActionID != order.ActionID {
		t.Fatalf("order %+v", order)
	}
	if st := h.rec.last(EventStudents).(Students); st.Students[0].Power == nil || st.Students[0].Power.Status != power.StatusSent {
		t.Fatalf("card %+v", st.Students[0].Power)
	}

	// PC01 counts down and postpones; PC02 cannot (the OS refused).
	h.hooks.onPower("s1", protocol.Envelope{Payload: protocol.PowerAck{RefID: ds[0].envID, ActionID: order.ActionID,
		Status: protocol.PowerPostponed, RemainingS: 330, PostponesLeft: 1}})
	h.hooks.onPower("s2", protocol.Envelope{Payload: protocol.NewError(ds[1].envID, protocol.CodeInternal, "no se pudo reiniciar el equipo")})
	st := h.rec.last(EventStudents).(Students)
	p1, p2 := st.Students[0].Power, st.Students[1].Power
	if p1.Status != protocol.PowerPostponed || p1.PostponesLeft != 1 || p1.DeadlineMs != now.Add(330*time.Second).UnixMilli() {
		t.Fatalf("PC01 card %+v", p1)
	}
	if p2.Status != power.StatusError {
		t.Fatalf("PC02 card %+v", p2)
	}
	// An error about something else does not touch the card.
	h.hooks.onPower("s1", protocol.Envelope{Payload: protocol.NewError(protocol.NewID(), protocol.CodeInternal, "x")})
	if p := h.app.View().Students.Students[0].Power; p.Status != protocol.PowerPostponed {
		t.Fatalf("PC01 card after an unrelated error %+v", p)
	}

	// Cancel: only the pending one gets power.cancel.
	if r := h.app.PowerCancel(nil); r.Sent != 1 {
		t.Fatalf("cancel %+v", r)
	}
	ds = h.class.takeDirect()
	if got := sentTypes(ds); !slices.Equal(got, []string{"s1:power.cancel"}) || ds[0].p.(protocol.PowerCancel).ActionID != order.ActionID {
		t.Fatalf("cancel sent %v", got)
	}
}

func TestPowerMarkedForcedAndInvalid(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapPower),
		student("s2", "PC02", protocol.CapPower))
	res, err := h.app.PowerAction(PowerRequest{Action: "shutdown", Hosts: []string{"pc02"}, Force: true})
	if err != nil || res.Sent != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	ds := h.class.takeDirect()
	if got := sentTypes(ds); !slices.Equal(got, []string{"s2:power.action"}) {
		t.Fatalf("sent %v", got)
	}
	if o := ds[0].p.(protocol.PowerAction); !o.Force || o.Action != protocol.PowerShutdown {
		t.Fatalf("order %+v", o)
	}

	for _, req := range []PowerRequest{{Action: "hibernate"}, {Action: "reboot", DelayS: 5}, {Action: "reboot", DelayS: 3600}} {
		if _, err := h.app.PowerAction(req); err == nil || !strings.Contains(err.Error(), "no válida") {
			t.Fatalf("%+v: %v", req, err)
		}
	}
	if _, err := h.app.PowerAction(PowerRequest{Action: "reboot", Hosts: []string{"PC99"}}); err == nil {
		t.Fatal("an order for nobody was accepted")
	}
	if d := h.class.takeDirect(); len(d) != 0 {
		t.Fatalf("sent %v after invalid requests", sentTypes(d))
	}
}
