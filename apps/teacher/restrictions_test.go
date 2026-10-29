package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/protocol"
)

func sentTypes(ds []directSend) []string {
	var out []string
	for _, d := range ds {
		out = append(out, d.sessionID+":"+string(d.p.MessageType()))
	}
	slices.Sort(out)
	return out
}

func TestRestrictionsTargetsAcksAndClears(t *testing.T) {
	both := []protocol.Capability{protocol.CapCodeView, protocol.CapAppBlock, protocol.CapWebFilter}
	h := newHarness(t,
		student("s1", "PC01", both...),
		student("s2", "PC02", both...),
		student("s3", "PC03", protocol.CapCodeView)) // an agent without the service
	if st := h.app.View().Students; st.CanAppBlock != 2 || st.CanWebFilter != 2 {
		t.Fatalf("counts %+v", st)
	}

	// Apps for the whole channel, typed as a teacher would.
	v, err := h.app.SetRestriction(RestrictionRequest{Kind: "apps", Mode: "deny",
		Items: []string{"Steam", `C:\Games\Discord\discord.exe`, "steam.exe", " "}})
	if err != nil {
		t.Fatal(err)
	}
	if v.Apps == nil || !slices.Equal(v.Apps.Items, []string{"Steam", "discord.exe"}) || len(v.Apps.Hosts) != 0 {
		t.Fatalf("apps rule %+v", v.Apps)
	}
	if got := sentTypes(h.class.takeDirect()); !slices.Equal(got, []string{"s1:app.block.set", "s2:app.block.set"}) {
		t.Fatalf("sent %v", got)
	}

	// Sites for PC02 only, pasted as URLs.
	v, err = h.app.SetRestriction(RestrictionRequest{Kind: "web", Mode: "allow", Hosts: []string{"PC02"},
		Items: []string{"https://www.python.org/3/library/", "docs.python.org", "w3schools.com:443"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(v.Web.Items, []string{"python.org", "docs.python.org", "w3schools.com"}) || !slices.Equal(v.Web.Hosts, []string{"pc02"}) {
		t.Fatalf("web rule %+v", v.Web)
	}
	ds := h.class.takeDirect()
	if got := sentTypes(ds); !slices.Equal(got, []string{"s2:web.filter.set"}) {
		t.Fatalf("sent %v", got)
	}
	if set := ds[0].p.(protocol.WebFilterSet); set.Mode != protocol.PolicyAllow {
		t.Fatalf("set %+v", set)
	}

	// Bad input is explained and nothing is sent.
	if _, err := h.app.SetRestriction(RestrictionRequest{Kind: "web", Mode: "deny", Items: []string{"español.org"}}); err == nil || !strings.Contains(err.Error(), "no es válido") {
		t.Fatalf("invalid domain: %v", err)
	}
	if _, err := h.app.SetRestriction(RestrictionRequest{Kind: "apps", Mode: "deny"}); err == nil || !strings.Contains(err.Error(), "entre 1 y 100") {
		t.Fatalf("empty list: %v", err)
	}
	if d := h.class.takeDirect(); len(d) != 0 {
		t.Fatalf("sent %v after invalid input", sentTypes(d))
	}

	// PC01 acknowledges: its card shows it.
	h.hooks.onPolicyAck("s1", protocol.AppBlockAck{RefID: protocol.NewID(), PolicyID: v.Apps.PolicyID, Active: true})
	st := h.rec.last(EventStudents).(Students)
	if !st.Students[0].AppsActive || st.Students[1].AppsActive || st.Students[0].WebActive {
		t.Fatalf("cards %+v", st.Students[:2])
	}

	// PC01 reconnects: it gets the apps again, without asking the teacher.
	h.class.mu.Lock()
	h.class.sessions[0] = student("s1b", "PC01", both...)
	h.class.mu.Unlock()
	h.hooks.onChange()
	if got := sentTypes(h.class.takeDirect()); !slices.Equal(got, []string{"s1b:app.block.set"}) {
		t.Fatalf("after reconnecting %v", got)
	}

	// "Quitar restricciones": clears to whoever had something.
	v = h.app.ClearAllRestrictions()
	if v.Apps != nil || v.Web != nil {
		t.Fatalf("view after clearing %+v", v)
	}
	want := []string{"s1b:app.block.clear", "s2:app.block.clear", "s2:web.filter.clear"}
	if got := sentTypes(h.class.takeDirect()); !slices.Equal(got, want) {
		t.Fatalf("clears %v, want %v", got, want)
	}

	// Closing the class with something on sends its clears.
	if _, err := h.app.SetRestriction(RestrictionRequest{Kind: "apps", Mode: "allow", Items: []string{"code"}}); err != nil {
		t.Fatal(err)
	}
	h.class.takeDirect()
	h.app.EndClass()
	if got := sentTypes(h.class.takeDirect()); !slices.Equal(got, []string{"s1b:app.block.clear", "s2:app.block.clear"}) {
		t.Fatalf("on EndClass %v", got)
	}
	if _, err := h.app.SetRestriction(RestrictionRequest{Kind: "apps", Mode: "deny", Items: []string{"x"}}); err != errNoClass {
		t.Fatalf("without class: %v", err)
	}
}
