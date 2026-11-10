package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"testing"

	"github.com/bayalad7/aula/internal/agentsvc"
	"github.com/bayalad7/aula/internal/discovery"
	"github.com/bayalad7/aula/internal/openapp"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// fakeLauncher records what it would open: no browser and no app reach the
// desktop.
type fakeLauncher struct {
	mu      sync.Mutex
	opened  []string
	started []string
}

func (f *fakeLauncher) Folders() (platform.AppFolders, error) {
	return platform.AppFolders{
		Vars:      map[string]string{"ProgramFiles": `C:\Program Files`, "SystemRoot": `C:\Windows`},
		Roots:     []string{`C:\Program Files`, `C:\Windows`},
		Forbidden: []string{`C:\Users\ana\Documents`},
	}, nil
}

func (f *fakeLauncher) Resolve(p string) (string, error) {
	if strings.Contains(p, "Falta") {
		return "", fmt.Errorf("resolve: %w", fs.ErrNotExist)
	}
	return p, nil
}

func (f *fakeLauncher) OpenURL(u string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, u)
	return nil
}

func (f *fakeLauncher) StartApp(p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, p)
	return nil
}

// An opened page (docs/privacidad.md rules 66–67): the notice in the corner
// says what and who, and the window never takes the focus from the browser.
func TestOpenedPageNotifiesNeverRaises(t *testing.T) {
	a, rec := newTestApp(nil)
	l := &fakeLauncher{}
	a.enableOpen(l)
	for _, c := range []protocol.Capability{protocol.CapOpenURL, protocol.CapOpenApp} {
		if !protocol.Has(a.hello.Capabilities, c) {
			t.Fatalf("hello capabilities = %v, want %s", a.hello.Capabilities, c)
		}
	}
	a.onState(session.AgentConnected, discovery.Console{Name: "Prof. Ruiz"})
	before := rec.notified
	acks := make(chan protocol.Payload, 4)
	send := func(_ context.Context, p protocol.Payload) error { acks <- p; return nil }
	id := protocol.NewID()
	if reply, ok := a.opener.Handle(t.Context(), openapp.Sender(send), protocol.New(protocol.OpenURL{OpenID: id, URL: "https://docs.python.org/3/?q=1"})); !ok || reply != nil {
		t.Fatalf("Handle = %v, %v", reply, ok)
	}
	a.opener.Wait()
	if ack := (<-acks).(protocol.OpenAck); ack.Status != protocol.OpenOpened || ack.OpenID != id {
		t.Fatalf("ack = %+v", ack)
	}
	opened, _ := rec.last(EventOpened).(Opened)
	if len(opened.List) != 1 || opened.List[0].Shown != "docs.python.org/3" || opened.List[0].Teacher != "Prof. Ruiz" || opened.List[0].Channel != "1" {
		t.Fatalf("opened = %+v", opened)
	}
	notices, _ := rec.last(EventNotices).(Notices)
	if n := notices.Notices[len(notices.Notices)-1]; n.Kind != NoticeOpened || n.Ref != id || n.Text != "" {
		t.Fatalf("notice = %+v: Go sends the state, the window words it", n)
	}
	if rec.raised != 0 || rec.notified != before+1 {
		t.Fatalf("raised %d, notified %d; want 0 and %d", rec.raised, rec.notified, before+1)
	}
	if v := a.View(); len(v.Opened.List) != 1 {
		t.Fatalf("View().Opened = %+v", v.Opened)
	}

	// An app outside the fixed rules does not start, and the student sees nothing.
	notified := rec.notified
	a.opener.Handle(t.Context(), openapp.Sender(send), protocol.New(protocol.OpenApp{OpenID: protocol.NewID(), AppID: "consola", Name: "Consola", Path: `%SystemRoot%\System32\cmd.exe`}))
	a.opener.Wait()
	if ack := (<-acks).(protocol.OpenAck); ack.Reason != protocol.OpenNotAllowed {
		t.Fatalf("cmd.exe: ack = %+v", ack)
	}
	if len(l.started) != 0 || rec.notified != notified {
		t.Fatalf("started %q, notified %d", l.started, rec.notified)
	}
}

func TestNoOpenCapabilityWithoutLauncher(t *testing.T) {
	a, _ := newTestApp(nil)
	if protocol.Has(a.hello.Capabilities, protocol.CapOpenURL) || protocol.Has(a.hello.Capabilities, protocol.CapOpenApp) {
		t.Fatal("open.* declared without a launcher")
	}
	reply, _ := a.opener.Handle(t.Context(), nil, protocol.New(protocol.OpenURL{OpenID: protocol.NewID(), URL: "https://python.org"}))
	if e, ok := reply.(protocol.ErrorPayload); !ok || e.Code != protocol.CodeUnsupportedCapability {
		t.Fatalf("reply = %#v", reply)
	}
	if v := a.View(); v.Opened.List == nil {
		t.Fatal("the window expects a list, not null")
	}
}

// As a service, open.* is declared only with a helper that can open, and the
// service never opens itself: the helper does, as the student (rule 69).
func TestServiceOpensOnlyThroughItsHelper(t *testing.T) {
	sa := newServiceAgent(Config{Channel: "1"}, nil, nil, quiet)
	for _, tc := range []struct {
		ready *agentsvc.Ready
		want  bool
	}{
		{nil, false},
		{&agentsvc.Ready{Capture: true, Files: true}, false},
		{&agentsvc.Ready{Open: true}, true},
	} {
		sa.mu.Lock()
		sa.ready = tc.ready
		sa.mu.Unlock()
		h := sa.hello()
		if protocol.Has(h.Capabilities, protocol.CapOpenURL) != tc.want || protocol.Has(h.Capabilities, protocol.CapOpenApp) != tc.want {
			t.Errorf("ready %+v: open.* declared in %v, want %v", tc.ready, h.Capabilities, tc.want)
		}
	}
	if _, ok := sa.app.opener.Session.(agentsvc.Opener); !ok {
		t.Fatalf("the service opens with %T, want the helper's proxy", sa.app.opener.Session)
	}
	if err := sa.app.opener.Session.OpenURL("https://python.org"); err == nil {
		t.Fatal("without a helper nothing opens")
	}
}

// The helper checks again with its own folders: what the service let through
// by mistake still does not start.
func TestHelperChecksOpenAgain(t *testing.T) {
	l := &fakeLauncher{}
	h := &helper{launch: l, log: quiet}
	call := func(name, arg string) protocol.OpenReason {
		data, _ := json.Marshal(arg)
		r, err := h.call(t.Context(), name, data)
		if err != nil {
			t.Fatal(err)
		}
		return r.(agentsvc.OpenResult).Reason
	}
	for _, tc := range []struct {
		name, arg string
		want      protocol.OpenReason
	}{
		{agentsvc.CallOpenURL, "https://python.org", ""},
		{agentsvc.CallOpenURL, "file:///C:/Windows/System32/cmd.exe", protocol.OpenNotAllowed},
		{agentsvc.CallOpenApp, `%ProgramFiles%\Thonny\thonny.exe`, ""},
		{agentsvc.CallOpenApp, `C:\Users\ana\Documents\Aula\virus.exe`, protocol.OpenNotAllowed},
		{agentsvc.CallOpenApp, `C:\Windows\System32\cmd.exe`, protocol.OpenNotAllowed},
		{agentsvc.CallOpenApp, `C:\Program Files\Falta\falta.exe`, protocol.OpenNotInstalled},
	} {
		if got := call(tc.name, tc.arg); got != tc.want {
			t.Errorf("%s %q: reason %q, want %q", tc.name, tc.arg, got, tc.want)
		}
	}
	if len(l.opened) != 1 || len(l.started) != 1 || l.started[0] != `C:\Program Files\Thonny\thonny.exe` {
		t.Fatalf("opened %q, started %q", l.opened, l.started)
	}
	none := &helper{log: quiet}
	data, _ := json.Marshal("https://python.org")
	if r, _ := none.call(t.Context(), agentsvc.CallOpenURL, data); r.(agentsvc.OpenResult).Reason != protocol.OpenLaunchFailed {
		t.Fatalf("without a launcher: %+v", r)
	}
}
