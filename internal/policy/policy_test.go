package policy

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

const (
	pid1 = "3f1c2e3a-4b5c-4d6e-a7f8-9a0b1c2d3e4f"
	pid2 = "0c9a7e52-6a4b-4f7e-8d33-2b1f4e6a9c10"
)

var prot = Protection{SystemRoot: `C:\Windows`, Keep: []string{"aula-cliente", "msedgewebview2", "explorer"}}

func proc(name, path string) platform.ProcessInfo {
	return platform.ProcessInfo{Name: name, Path: path}
}

func TestAppPolicyBlocks(t *testing.T) {
	deny := NewAppPolicy(protocol.AppBlockSet{PolicyID: pid1, Mode: protocol.PolicyDeny, Apps: []string{"Steam", "discord.exe"}})
	allow := NewAppPolicy(protocol.AppBlockSet{PolicyID: pid1, Mode: protocol.PolicyAllow, Apps: []string{"code", "python.exe"}})
	for _, tt := range []struct {
		p    *AppPolicy
		proc platform.ProcessInfo
		want bool
	}{
		{deny, proc("steam.exe", `C:\Games\Steam\steam.exe`), true},
		{deny, proc("Discord.EXE", `C:\Users\a\Discord.exe`), true},
		{deny, proc("code.exe", `C:\Apps\code.exe`), false},
		{allow, proc("Code.exe", `C:\Apps\Code.exe`), false},
		{allow, proc("python.exe", `C:\Python\python.exe`), false},
		{allow, proc("steam.exe", `C:\Games\steam.exe`), true},
		// Protected, whatever the list says.
		{allow, proc("notepad.exe", `C:\Windows\System32\notepad.exe`), false},
		{allow, proc("explorer.exe", `C:\Windows\explorer.exe`), false},
		{allow, proc("aula-cliente.exe", `C:\Program Files\Aula\Cliente\aula-cliente.exe`), false},
		{allow, proc("msedgewebview2.exe", `C:\Program Files (x86)\Microsoft\EdgeWebView\msedgewebview2.exe`), false},
		{allow, proc("secret.exe", ""), false},
		{NewAppPolicy(protocol.AppBlockSet{PolicyID: pid1, Mode: protocol.PolicyDeny, Apps: []string{"explorer"}}),
			proc("explorer.exe", `C:\Windows\explorer.exe`), false},
	} {
		if got := tt.p.Blocks(tt.proc, prot); got != tt.want {
			t.Errorf("%s %s: Blocks = %v, want %v", tt.p.Mode, tt.proc.Name, got, tt.want)
		}
	}
}

type fakeProcs struct {
	mu     sync.Mutex
	procs  []platform.ProcessInfo
	killed []string
}

func (f *fakeProcs) List(uint32) ([]platform.ProcessInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.procs), nil
}

func (f *fakeProcs) Kill(pid int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, p := range f.procs {
		if p.PID == pid {
			f.killed = append(f.killed, p.Name)
			f.procs = slices.Delete(f.procs, i, i+1)
			return nil
		}
	}
	return nil
}

func (f *fakeProcs) start(name, path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.procs = append(f.procs, platform.ProcessInfo{PID: 1000 + len(f.killed) + len(f.procs), Name: name, Path: path})
}

func TestAppEnforcerClosesAndNotifiesOnce(t *testing.T) {
	fp := &fakeProcs{}
	var notices []string
	now := time.Unix(0, 0)
	e := &AppEnforcer{Procs: fp, Protect: prot, Session: func() (uint32, bool) { return 1, true },
		OnClosed: func(n string) { notices = append(notices, n) }, now: func() time.Time { return now }}
	fp.start("steam.exe", `C:\Games\steam.exe`)
	fp.start("code.exe", `C:\Apps\code.exe`)
	e.Enforce() // no policy: nothing
	if len(fp.killed) != 0 {
		t.Fatal("closed apps without a policy")
	}
	e.SetPolicy(NewAppPolicy(protocol.AppBlockSet{PolicyID: pid1, Mode: protocol.PolicyDeny, Apps: []string{"steam"}}))
	e.Enforce()
	fp.start("steam.exe", `C:\Games\steam.exe`) // reopened at once
	e.Enforce()
	if !slices.Equal(fp.killed, []string{"steam.exe", "steam.exe"}) || !slices.Equal(notices, []string{"steam.exe"}) {
		t.Fatalf("killed %v, notices %v", fp.killed, notices)
	}
	now = now.Add(31 * time.Second)
	fp.start("steam.exe", `C:\Games\steam.exe`)
	e.Enforce()
	if len(notices) != 2 {
		t.Fatalf("notice not repeated after 30 s: %v", notices)
	}
	// Removing the policy stops closing; without a session nothing happens.
	e.SetPolicy(nil)
	fp.start("steam.exe", `C:\Games\steam.exe`)
	e.Enforce()
	e.SetPolicy(NewAppPolicy(protocol.AppBlockSet{PolicyID: pid1, Mode: protocol.PolicyDeny, Apps: []string{"steam"}}))
	e.Session = func() (uint32, bool) { return 0, false }
	e.Enforce()
	if len(fp.killed) != 3 {
		t.Fatalf("killed %v", fp.killed)
	}
}

func TestWebPolicyBlocks(t *testing.T) {
	deny := NewWebPolicy(protocol.WebFilterSet{PolicyID: pid1, Mode: protocol.PolicyDeny, Domains: []string{"YouTube.com", "tiktok.com"}})
	allow := NewWebPolicy(protocol.WebFilterSet{PolicyID: pid1, Mode: protocol.PolicyAllow, Domains: []string{"python.org"}})
	local := setOf([]string{"escuela.edu.mx"})
	for _, tt := range []struct {
		p    *WebPolicy
		name string
		want bool
	}{
		{deny, "youtube.com.", true},
		{deny, "www.YOUTUBE.com.", true},
		{deny, "notyoutube.com.", false},
		{deny, "python.org.", false},
		{deny, "use-application-dns.net.", true}, // always, with any list
		{deny, "dns.google.", true},
		{allow, "docs.python.org.", false},
		{allow, "youtube.com.", true},
		{allow, "pc07.", false},                     // single label: Windows' own
		{allow, "dc01.escuela.edu.mx.", false},      // the school's domain
		{allow, "5.1.168.192.in-addr.arpa.", false}, // reverse lookup
		{allow, "www.msftconnecttest.com.", false},  // connectivity check
		{allow, "chrome.cloudflare-dns.com.", true}, // DoH server
		{(*WebPolicy)(nil), "dns.google.", true},    // only DoH without a policy
		{(*WebPolicy)(nil), "youtube.com.", false},
	} {
		if got := tt.p.Blocks(tt.name, local); got != tt.want {
			t.Errorf("%v %s: Blocks = %v, want %v", tt.p, tt.name, got, tt.want)
		}
	}
}

// fakeUpstream is a DNS server that answers every A query with 192.0.2.7.
func fakeUpstream(t *testing.T) (port int, queries *[]string) {
	t.Helper()
	var mu sync.Mutex
	var seen []string
	h := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		mu.Lock()
		seen = append(seen, r.Question[0].Name)
		mu.Unlock()
		m := new(dns.Msg).SetReply(r)
		rr, _ := dns.NewRR(r.Question[0].Name + " 60 IN A 192.0.2.7")
		m.Answer = append(m.Answer, rr)
		_ = w.WriteMsg(m)
	})
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port = pc.LocalAddr().(*net.UDPAddr).Port
	ln, err := net.Listen("tcp", pc.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	udp, tcp := &dns.Server{PacketConn: pc, Handler: h}, &dns.Server{Listener: ln, Handler: h}
	go udp.ActivateAndServe()
	go tcp.ActivateAndServe()
	t.Cleanup(func() { _ = udp.Shutdown(); _ = tcp.Shutdown() })
	return port, &seen
}

func query(t *testing.T, network, addr, name string) *dns.Msg {
	t.Helper()
	c := &dns.Client{Net: network, Timeout: 3 * time.Second}
	m := new(dns.Msg).SetQuestion(dns.Fqdn(name), dns.TypeA)
	var resp *dns.Msg
	var err error
	for range 20 { // the servers may still be starting
		if resp, _, err = c.Exchange(m, addr); err == nil {
			return resp
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("query %s over %s: %v", name, network, err)
	return nil
}

func TestResolverFiltersAndForwards(t *testing.T) {
	port, seen := fakeUpstream(t)
	pol := NewWebPolicy(protocol.WebFilterSet{PolicyID: pid1, Mode: protocol.PolicyDeny, Domains: []string{"youtube.com"}})
	r := &Resolver{Addr: "127.0.0.1:0", UpstreamPort: port,
		Policy:    func() *WebPolicy { return pol },
		Upstreams: func() platform.DNSConfig { return platform.DNSConfig{Servers: []string{"127.0.0.1"}} }}
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if m := query(t, "udp", r.LocalAddr(), "www.youtube.com"); m.Rcode != dns.RcodeNameError {
		t.Fatalf("blocked name: rcode %d", m.Rcode)
	}
	if m := query(t, "udp", r.LocalAddr(), "use-application-dns.net"); m.Rcode != dns.RcodeNameError {
		t.Fatalf("canary: rcode %d", m.Rcode)
	}
	for _, network := range []string{"udp", "tcp"} {
		m := query(t, network, r.LocalAddr(), "docs.python.org")
		if m.Rcode != dns.RcodeSuccess || len(m.Answer) != 1 || m.Answer[0].(*dns.A).A.String() != "192.0.2.7" {
			t.Fatalf("forwarded over %s: %v", network, m)
		}
	}
	if slices.Contains(*seen, "www.youtube.com.") {
		t.Fatal("a blocked name reached the upstream server")
	}
	// Without upstream servers the answer is a failure, not a hang.
	r.Upstreams = func() platform.DNSConfig { return platform.DNSConfig{} }
	if m := query(t, "udp", r.LocalAddr(), "python.org"); m.Rcode != dns.RcodeServerFailure {
		t.Fatalf("no upstream: rcode %d", m.Rcode)
	}
}

// --- Manager ----------------------------------------------------------------------

// machine is the fake computer: the NRPT rules and the browser policies, as
// a registry of values.
type machine struct {
	mu        sync.Mutex
	rules     int
	values    map[string]string // "key\name" → value; absent: does not exist
	failApply bool
	journal   string // to check it is written before anything changes
	order     []string
}

func (m *machine) log(s string) { m.order = append(m.order, s) }

type fakeDNS struct{ m *machine }

func (d fakeDNS) Apply(server string) error {
	d.m.mu.Lock()
	defer d.m.mu.Unlock()
	d.m.log("nrpt.apply " + server)
	if d.m.failApply {
		return errors.New("Add-DnsClientNrptRule falló")
	}
	d.m.rules++
	return nil
}
func (d fakeDNS) Clear() error {
	d.m.mu.Lock()
	d.m.rules = 0
	d.m.log("nrpt.clear")
	d.m.mu.Unlock()
	return nil
}
func (d fakeDNS) Flush() error { return nil }
func (d fakeDNS) Upstreams() (platform.DNSConfig, error) {
	return platform.DNSConfig{Servers: []string{"127.0.0.1"}}, nil
}

var dohKeys = []string{`Chrome\DnsOverHttpsMode`, `Edge\DnsOverHttpsMode`, `Firefox\Enabled`}

type fakeDoH struct{ m *machine }

func (b fakeDoH) Snapshot() ([]platform.SavedSetting, error) {
	b.m.mu.Lock()
	defer b.m.mu.Unlock()
	var out []platform.SavedSetting
	for _, k := range dohKeys {
		v, ok := b.m.values[k]
		out = append(out, platform.SavedSetting{Key: k, Existed: ok, Str: v})
	}
	return out, nil
}

func (b fakeDoH) Disable() error {
	b.m.mu.Lock()
	defer b.m.mu.Unlock()
	if _, err := os.Stat(b.m.journal); err != nil {
		b.m.log("doh.disable WITHOUT JOURNAL")
	} else {
		b.m.log("doh.disable")
	}
	for _, k := range dohKeys {
		b.m.values[k] = "off"
	}
	return nil
}

func (b fakeDoH) Restore(saved []platform.SavedSetting) error {
	b.m.mu.Lock()
	defer b.m.mu.Unlock()
	b.m.log("doh.restore")
	for _, s := range saved {
		if s.Existed {
			b.m.values[s.Key] = s.Str
		} else {
			delete(b.m.values, s.Key)
		}
	}
	return nil
}

func (m *machine) snapshot() (int, map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	vals := map[string]string{}
	for k, v := range m.values {
		vals[k] = v
	}
	return m.rules, vals
}

func newManager(t *testing.T, mach *machine, apps *AppEnforcer) (*Manager, *[]State) {
	var states []State
	m := &Manager{Apps: apps, DNS: fakeDNS{mach}, DoH: fakeDoH{mach}, Journal: Journal{Path: mach.journal},
		ResolverAddr: "127.0.0.1:0", OnChange: func(s State) { states = append(states, s) }}
	return m, &states
}

func newMachine(t *testing.T) *machine {
	// The school already has its own Edge policy: it must survive.
	return &machine{values: map[string]string{`Edge\DnsOverHttpsMode`: "automatic"},
		journal: filepath.Join(t.TempDir(), "Aula", "alumno", JournalName)}
}

func webSet(mode protocol.PolicyMode, domains ...string) protocol.Envelope {
	return protocol.New(protocol.WebFilterSet{PolicyID: protocol.NewID(), Mode: mode, Domains: domains})
}

func TestManagerWebFilterAppliesAndRestoresExactly(t *testing.T) {
	mach := newMachine(t)
	rules0, vals0 := mach.snapshot()
	port, _ := fakeUpstream(t)
	m, states := newManager(t, mach, nil)
	m.upstreamPort = port

	env := webSet(protocol.PolicyDeny, "youtube.com")
	reply, ok := m.Handle(env)
	ack, isAck := reply.(protocol.WebFilterAck)
	if !ok || !isAck || !ack.Active || ack.RefID != env.ID {
		t.Fatalf("reply %#v", reply)
	}
	// The notice came first, the journal before any change, then the rule.
	if len(*states) != 1 || (*states)[0].Web == nil || (*states)[0].Web.Total != 1 {
		t.Fatalf("notice %+v", *states)
	}
	if !slices.Equal(mach.order, []string{"doh.disable", "nrpt.apply 127.0.0.1"}) {
		t.Fatalf("order %v", mach.order)
	}
	if rules, vals := mach.snapshot(); rules != 1 || vals[`Chrome\DnsOverHttpsMode`] != "off" {
		t.Fatalf("not applied: %d rules, %v", rules, vals)
	}
	if _, err := os.Stat(mach.journal); err != nil {
		t.Fatal("no journal while the filter is on")
	}
	addr := m.resolver.LocalAddr()
	if r := query(t, "udp", addr, "www.youtube.com"); r.Rcode != dns.RcodeNameError {
		t.Fatalf("blocked through the manager: rcode %d", r.Rcode)
	}

	// A new list replaces the old one without touching the computer again.
	m.Handle(webSet(protocol.PolicyAllow, "python.org"))
	if r := query(t, "udp", addr, "youtube.com"); r.Rcode != dns.RcodeNameError {
		t.Fatal("allow list not in force")
	}
	if r := query(t, "udp", addr, "python.org"); r.Rcode != dns.RcodeSuccess {
		t.Fatal("allowed name refused")
	}
	if rules, _ := mach.snapshot(); rules != 1 {
		t.Fatalf("%d rules after a replace", rules)
	}

	reply, _ = m.Handle(protocol.New(protocol.WebFilterClear{}))
	if a, ok := reply.(protocol.WebFilterAck); !ok || a.Active {
		t.Fatalf("clear reply %#v", reply)
	}
	rules, vals := mach.snapshot()
	if rules != rules0 || !reflect.DeepEqual(vals, vals0) {
		t.Fatalf("computer not restored exactly: %d rules %v, want %d %v", rules, vals, rules0, vals0)
	}
	if _, err := os.Stat(mach.journal); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("journal left after clearing")
	}
	if last := (*states)[len(*states)-1]; last.Active() {
		t.Fatalf("notice still active: %+v", last)
	}
	c := &dns.Client{Timeout: 200 * time.Millisecond}
	if _, _, err := c.Exchange(new(dns.Msg).SetQuestion("python.org.", dns.TypeA), addr); err == nil {
		t.Fatal("resolver still answering after clear")
	}
}

func TestManagerUndoesAFailedApply(t *testing.T) {
	mach := newMachine(t)
	mach.failApply = true
	rules0, vals0 := mach.snapshot()
	m, states := newManager(t, mach, nil)
	reply, _ := m.Handle(webSet(protocol.PolicyDeny, "youtube.com"))
	if e, ok := reply.(protocol.ErrorPayload); !ok || e.Code != protocol.CodeInternal {
		t.Fatalf("reply %#v", reply)
	}
	rules, vals := mach.snapshot()
	if rules != rules0 || !reflect.DeepEqual(vals, vals0) {
		t.Fatalf("half-applied filter left: %d %v", rules, vals)
	}
	if _, err := os.Stat(mach.journal); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("journal left after a failed apply")
	}
	if m.State().Active() || (*states)[len(*states)-1].Active() {
		t.Fatal("notice says active after a failed apply")
	}
}

// The agent crashes with the filter on; the service manager starts it again:
// Restore undoes everything from the journal before it connects.
func TestManagerRestoresAfterACrash(t *testing.T) {
	mach := newMachine(t)
	rules0, vals0 := mach.snapshot()
	m, _ := newManager(t, mach, nil)
	if reply, _ := m.Handle(webSet(protocol.PolicyDeny, "youtube.com")); reply.MessageType() != protocol.TypeWebFilterAck {
		t.Fatalf("reply %#v", reply)
	}
	_ = m.resolver.Close() // the process died: no clear ran

	restarted, _ := newManager(t, mach, nil)
	if err := restarted.Restore(); err != nil {
		t.Fatal(err)
	}
	rules, vals := mach.snapshot()
	if rules != rules0 || !reflect.DeepEqual(vals, vals0) {
		t.Fatalf("not restored after the crash: %d %v", rules, vals)
	}
	if _, err := os.Stat(mach.journal); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("journal left")
	}
	// Restore without a journal still removes rules left without one.
	mach.rules = 1
	if err := restarted.Restore(); err != nil || mach.rules != 0 {
		t.Fatalf("orphan rule left: %d, %v", mach.rules, err)
	}
}

func TestManagerAppsAndClearOnDisconnect(t *testing.T) {
	mach := newMachine(t)
	fp := &fakeProcs{}
	apps := &AppEnforcer{Procs: fp, Protect: prot, Session: func() (uint32, bool) { return 1, true }}
	m, states := newManager(t, mach, apps)
	if got := m.Capabilities(); !slices.Equal(got, []protocol.Capability{protocol.CapAppBlock, protocol.CapWebFilter}) {
		t.Fatalf("capabilities %v", got)
	}
	set := protocol.New(protocol.AppBlockSet{PolicyID: pid1, Mode: protocol.PolicyDeny, Apps: []string{"steam"}})
	reply, _ := m.Handle(set)
	if a, ok := reply.(protocol.AppBlockAck); !ok || !a.Active || a.PolicyID != pid1 {
		t.Fatalf("reply %#v", reply)
	}
	fp.start("steam.exe", `C:\Games\steam.exe`)
	apps.Enforce()
	if len(fp.killed) != 1 {
		t.Fatal("blocked app not closed")
	}
	m.Handle(webSet(protocol.PolicyDeny, "youtube.com"))
	if s := m.State(); s.Apps == nil || s.Web == nil {
		t.Fatalf("state %+v", s)
	}

	// The connection with the console closes: everything ends.
	if err := m.Clear("disconnected"); err != nil {
		t.Fatal(err)
	}
	fp.start("steam.exe", `C:\Games\steam.exe`)
	apps.Enforce()
	if len(fp.killed) != 1 || m.State().Active() || (*states)[len(*states)-1].Active() {
		t.Fatalf("restrictions left after disconnecting: killed %v, state %+v", fp.killed, m.State())
	}
	if rules, _ := mach.snapshot(); rules != 0 {
		t.Fatal("NRPT rule left after disconnecting")
	}

	// Without the pieces, the agent refuses.
	bare := &Manager{}
	if bare.Capabilities() != nil {
		t.Fatal("capabilities without platform pieces")
	}
	if reply, _ := bare.Handle(set); reply.(protocol.ErrorPayload).Code != protocol.CodeUnsupportedCapability {
		t.Fatalf("reply %#v", reply)
	}
	if _, ok := bare.Handle(protocol.New(protocol.MsgAck{RefID: pid1})); ok {
		t.Fatal("handled a message that is not a restriction")
	}
}

func TestJournalSurvivesDamage(t *testing.T) {
	j := Journal{Path: filepath.Join(t.TempDir(), JournalName)}
	if _, ok, err := j.Read(); ok || err != nil {
		t.Fatalf("empty journal: %v %v", ok, err)
	}
	if err := os.WriteFile(j.Path, []byte("{roto"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, ok, err := j.Read()
	if !ok || err == nil || !r.NRPT {
		t.Fatalf("damaged journal must still mean 'undo the rule': %+v %v %v", r, ok, err)
	}
	if err := j.Remove(); err != nil {
		t.Fatal(err)
	}
	if err := j.Remove(); err != nil {
		t.Fatalf("second remove: %v", err)
	}
}

// --- Console side -------------------------------------------------------------------

func agent(id, host string, caps ...protocol.Capability) session.Session {
	return session.Session{ID: id, Hostname: host, State: session.StateConnected, Capabilities: caps}
}

func kinds(cmds []Command) []string {
	var out []string
	for _, c := range cmds {
		out = append(out, c.SessionID+":"+string(c.Payload.MessageType()))
	}
	slices.Sort(out)
	return out
}

func send(c *ClassPolicy, cmds []Command) {
	for _, cmd := range cmds {
		c.Sent(cmd)
	}
}

func TestClassPolicyTargetsReconnectsAndClears(t *testing.T) {
	c := NewClassPolicy()
	both := []protocol.Capability{protocol.CapAppBlock, protocol.CapWebFilter}
	class := []session.Session{agent("s1", "PC01", both...), agent("s2", "PC02", both...), agent("s3", "PC03")}
	if cmds := c.Plan(class); len(cmds) != 0 {
		t.Fatalf("sent %v without restrictions", kinds(cmds))
	}
	c.SetApps(protocol.AppBlockSet{PolicyID: pid1, Mode: protocol.PolicyDeny, Apps: []string{"steam"}}, nil)
	c.SetWeb(protocol.WebFilterSet{PolicyID: pid2, Mode: protocol.PolicyAllow, Domains: []string{"python.org"}}, []string{"pc02"})
	cmds := c.Plan(class)
	want := []string{"s1:app.block.set", "s2:app.block.set", "s2:web.filter.set"}
	if got := kinds(cmds); !slices.Equal(got, want) {
		t.Fatalf("plan %v, want %v (PC03 cannot; web only for PC02)", got, want)
	}
	send(c, cmds)
	if cmds := c.Plan(class); len(cmds) != 0 {
		t.Fatalf("sent again %v", kinds(cmds))
	}
	c.Ack("s2", protocol.WebFilterAck{RefID: pid1, PolicyID: pid2, Active: true})
	if !c.Active("s2", KindWeb) || c.Active("s1", KindWeb) || c.Active("s2", KindApps) {
		t.Fatal("acknowledgements misread")
	}

	// PC02 reconnects as a new session: it gets both again.
	class[1] = agent("s2b", "PC02", both...)
	if got := kinds(c.Plan(class)); !slices.Equal(got, []string{"s2b:app.block.set", "s2b:web.filter.set"}) {
		t.Fatalf("after reconnecting: %v", got)
	}
	send(c, c.Plan(class))

	// The web filter now goes to PC01 only: PC02 gets a clear.
	c.SetWeb(protocol.WebFilterSet{PolicyID: pid1, Mode: protocol.PolicyAllow, Domains: []string{"python.org"}}, []string{"PC01"})
	if got := kinds(c.Plan(class)); !slices.Equal(got, []string{"s1:web.filter.set", "s2b:web.filter.clear"}) {
		t.Fatalf("retarget: %v", got)
	}
	send(c, c.Plan(class))

	// "Quitar restricciones" of apps: clears to whoever had it.
	c.Clear(KindApps)
	if got := kinds(c.Plan(class)); !slices.Equal(got, []string{"s1:app.block.clear", "s2b:app.block.clear"}) {
		t.Fatalf("clear: %v", got)
	}
	send(c, c.Plan(class))
	if cmds := c.Plan(class); len(cmds) != 0 {
		t.Fatalf("clear sent twice: %v", kinds(cmds))
	}
	rules := c.Rules()
	if rules[KindApps] != nil || rules[KindWeb] == nil || !slices.Equal(rules[KindWeb].Hosts, []string{"pc01"}) {
		t.Fatalf("rules %+v", rules)
	}

	// Closing the class: clears for what remains, and nothing afterwards.
	if got := kinds(c.Reset()); !slices.Equal(got, []string{"s1:web.filter.clear"}) {
		t.Fatalf("reset: %v", got)
	}
	if cmds := c.Plan(class); len(cmds) != 0 {
		t.Fatalf("after reset: %v", kinds(cmds))
	}
}
