package health

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// fakeSys is the test double of platform.SystemHealth. With none set,
// every datum is "not supported", like an OS without them.
type fakeSys struct {
	mu        sync.Mutex
	none      bool
	free      int64
	failing   bool
	mem       platform.Memory
	idle, tot uint64
	limit     int
	reboot    bool
	link      platform.Link
	battery   platform.Battery
	devices   platform.Devices
}

func newFakeSys() *fakeSys {
	return &fakeSys{
		free: 80 * gib, mem: platform.Memory{Total: 8 * gib, Available: 4 * gib}, limit: 100,
		link:    platform.Link{Kind: platform.LinkEthernet, Mbps: 1000, SignalPct: -1},
		devices: platform.Devices{AudioOut: platform.DeviceOK, NIC: platform.DeviceOK, GPU: platform.DeviceOK, Keyboard: platform.DeviceOK, Mouse: platform.DeviceOK},
	}
}

func (f *fakeSys) err() error {
	if f.none {
		return platform.ErrNotSupported
	}
	return nil
}
func (f *fakeSys) SystemDrive() (string, error) { return `C:\`, f.err() }
func (f *fakeSys) DiskSpace(string) (platform.DiskSpace, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return platform.DiskSpace{Free: f.free, Total: 238 * gib}, f.err()
}
func (f *fakeSys) DiskFailing() (bool, error) { return f.failing, f.err() }
func (f *fakeSys) Memory() (platform.Memory, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.mem, f.err()
}
func (f *fakeSys) CPUTimes() (platform.CPUTimes, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return platform.CPUTimes{Idle: f.idle, Total: f.tot}, f.err()
}
func (f *fakeSys) CPULimitPct() (int, error)          { return f.limit, f.err() }
func (f *fakeSys) Uptime() (time.Duration, error)     { return 26 * time.Hour, f.err() }
func (f *fakeSys) RebootPending() (bool, error)       { return f.reboot, f.err() }
func (f *fakeSys) Link(net.IP) (platform.Link, error) { return f.link, f.err() }
func (f *fakeSys) Battery() (platform.Battery, error) { return f.battery, f.err() }
func (f *fakeSys) Devices() (platform.Devices, error) { return f.devices, f.err() }

// busy adds 5 s of processor time at load pct.
func (f *fakeSys) busy(pct uint64) {
	f.mu.Lock()
	f.tot += 500
	f.idle += 500 - 5*pct
	f.mu.Unlock()
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type outbox struct {
	mu   sync.Mutex
	sent []protocol.Payload
}

func (o *outbox) send(_ context.Context, p protocol.Payload) error {
	if err := p.Validate(); err != nil {
		return err
	}
	o.mu.Lock()
	o.sent = append(o.sent, p)
	o.mu.Unlock()
	return nil
}

func (o *outbox) reports() []protocol.HealthReport {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []protocol.HealthReport
	for _, p := range o.sent {
		if r, ok := p.(protocol.HealthReport); ok {
			out = append(out, r)
		}
	}
	return out
}

type fakeProber struct {
	mu            sync.Mutex
	dnsErr, tcErr error
	calls         int
}

func (p *fakeProber) Resolve(context.Context, string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	return p.dnsErr
}
func (p *fakeProber) Dial(context.Context, string, int) error { return p.tcErr }

func newMonitor(sys *fakeSys, clk *clock, pr Prober) *Monitor {
	return &Monitor{
		Sys: sys, Prober: pr, Logger: quiet, Now: clk.now, Jitter: func() float64 { return 0 },
		Documents: func() string { return `C:\Users\alumno\Documents` },
		LocalIP:   func() net.IP { return net.IPv4(10, 0, 0, 7) },
	}
}

var internetCfg = protocol.HealthConfig{Internet: &protocol.HealthTarget{Host: "www.google.com", Port: 443}}

// The rhythm of §5.27.3: nothing before the first health.config, then
// first, periodic every 60 s, change at most every 10 s.
func TestMonitorRhythm(t *testing.T) {
	sys, clk, out := newFakeSys(), &clock{t: time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)}, &outbox{}
	m := newMonitor(sys, clk, &fakeProber{})
	ctx := context.Background()
	m.Start(out.send)
	for range 3 {
		sys.busy(10)
		m.Tick(ctx)
		clk.add(SampleEvery)
	}
	if n := len(out.reports()); n != 0 {
		t.Fatalf("%d reports before health.config", n)
	}
	m.Configure(ctx, protocol.HealthConfig{})
	rs := out.reports()
	if len(rs) != 1 || rs[0].Reason != protocol.HealthFirst || rs[0].Seq != 1 {
		t.Fatalf("first report: %+v", rs)
	}
	if rs[0].CPU == nil || rs[0].CPU.LoadPct == nil || *rs[0].CPU.LoadPct != 10 {
		t.Fatalf("load of the samples so far: %+v", rs[0].CPU)
	}
	// Quiet for 55 s: no report.
	for range 11 {
		clk.add(SampleEvery)
		sys.busy(10)
		m.Tick(ctx)
	}
	if n := len(out.reports()); n != 1 {
		t.Fatalf("%d reports within the minute", n)
	}
	clk.add(SampleEvery) // 60 s
	m.Tick(ctx)
	rs = out.reports()
	if len(rs) != 2 || rs[1].Reason != protocol.HealthPeriodic || rs[1].Seq != 2 {
		t.Fatalf("periodic: %+v", rs)
	}
	// The disk fills by 2 GiB: a change report, but not twice within 10 s.
	clk.add(SampleEvery)
	sys.mu.Lock()
	sys.free -= 2 * gib
	sys.mu.Unlock()
	m.Tick(ctx) // 5 s after the periodic: too soon
	clk.add(SampleEvery)
	m.Tick(ctx) // 10 s after: goes
	rs = out.reports()
	if len(rs) != 3 || rs[2].Reason != protocol.HealthChange {
		t.Fatalf("change: %+v", rs)
	}
	clk.add(ChangeGap)
	m.Tick(ctx)
	if n := len(out.reports()); n != 3 {
		t.Fatalf("no change, no report; got %d", n)
	}
	// The session ends: the config is forgotten, nothing more is sent.
	m.Stop()
	clk.add(2 * ReportEvery)
	m.Tick(ctx)
	if n := len(out.reports()); n != 3 {
		t.Fatalf("report after Stop: %d", n)
	}
}

func TestMonitorRequestAndProbe(t *testing.T) {
	sys, clk, out, pr := newFakeSys(), &clock{t: time.Unix(1_800_000_000, 0)}, &outbox{}, &fakeProber{}
	m := newMonitor(sys, clk, pr)
	ctx := context.Background()
	m.Start(out.send)
	m.Configure(ctx, internetCfg)
	clk.add(2 * time.Second)
	m.Request(ctx, "5b0e8f3c-2a6d-4c1e-9b7a-0f3d2e1c4b5a")
	rs := out.reports()
	if len(rs) != 2 || rs[1].Reason != protocol.HealthOnDemand || rs[1].RefID != "5b0e8f3c-2a6d-4c1e-9b7a-0f3d2e1c4b5a" {
		t.Fatalf("request: %+v", rs)
	}
	if pr.calls != 1 {
		t.Fatalf("a request 2 s after a probe reuses it; probes = %d", pr.calls)
	}
	if i := rs[1].Internet; i == nil || i.DNS != protocol.ProbeOK || i.Reach != protocol.ProbeOK {
		t.Fatalf("internet: %+v", i)
	}
	pr.tcErr = errors.New("connection refused")
	clk.add(ProbeReuse)
	m.Request(ctx, "0c9a7e52-6a4b-4f7e-8d33-2b1f4e6a9c10")
	if i := out.reports()[2].Internet; pr.calls != 2 || i.Reach != protocol.ProbeFail || i.DNS != protocol.ProbeOK {
		t.Fatalf("second probe: calls %d, %+v", pr.calls, i)
	}
	// Without a destination, no internet datum and no probe.
	m.Configure(ctx, protocol.HealthConfig{})
	m.Request(ctx, "0c9a7e52-6a4b-4f7e-8d33-2b1f4e6a9c10")
	if r := out.reports()[3]; r.Internet != nil || pr.calls != 2 {
		t.Fatalf("no destination: %+v, calls %d", r.Internet, pr.calls)
	}
}

// An OS that has no datum sends a report with none: still valid.
func TestMonitorOSWithoutData(t *testing.T) {
	sys := newFakeSys()
	sys.none = true
	clk, out := &clock{t: time.Unix(1_800_000_000, 0)}, &outbox{}
	m := newMonitor(sys, clk, &fakeProber{})
	m.Start(out.send)
	m.Configure(context.Background(), protocol.HealthConfig{})
	rs := out.reports()
	if len(rs) != 1 {
		t.Fatalf("reports = %d", len(rs))
	}
	r := rs[0]
	if r.Disks != nil || r.Memory != nil || r.CPU != nil || r.UptimeS != nil || r.RebootPending != nil || r.Network != nil || r.Battery != nil || r.DiskHealth != "" {
		t.Fatalf("an OS without data reported %+v", r)
	}
}

func TestMonitorReadsEveryDatum(t *testing.T) {
	sys := newFakeSys()
	sys.link = platform.Link{Kind: platform.LinkWiFi, Mbps: 144, SignalPct: 34, CableIdle: true}
	sys.battery = platform.Battery{Present: true, ChargePct: 54, OnAC: true, WearPct: -1}
	sys.reboot, sys.failing = true, true
	m := newMonitor(sys, &clock{t: time.Unix(1_800_000_000, 0)}, &fakeProber{})
	r := m.read()
	r.Seq, r.Reason = 1, protocol.HealthPeriodic
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(r.Disks) != 2 || r.DiskHealth != protocol.DiskDegraded || *r.RebootPending != true ||
		r.Network.Kind != protocol.NetWiFi || *r.Network.WiFiSignalPct != 34 || !*r.Network.CableIdle ||
		r.Battery.WearPct != nil || *r.UptimeS != 26*3600 {
		t.Fatalf("read %+v", r)
	}
	// Without a session, no Documents disk.
	m.Documents = func() string { return "" }
	if r := m.read(); len(r.Disks) != 1 || r.Disks[0].Role != protocol.DiskSystem {
		t.Fatalf("without session: %+v", r.Disks)
	}
}

func TestNotableBands(t *testing.T) {
	base := Simulated(1, 1, protocol.HealthPeriodic, "")
	same := Simulated(1, 2, protocol.HealthPeriodic, "")
	if Notable(base, same) {
		t.Fatal("the same values are not notable")
	}
	for name, f := range map[string]func(*protocol.HealthReport){
		"disk -2 GiB":   func(r *protocol.HealthReport) { r.Disks[0].FreeBytes -= 2 * gib },
		"disk gone":     func(r *protocol.HealthReport) { r.Disks = r.Disks[:1] },
		"degraded":      func(r *protocol.HealthReport) { r.DiskHealth = protocol.DiskDegraded },
		"memory -20":    func(r *protocol.HealthReport) { r.Memory.AvailableBytes = gib / 2 },
		"cpu saturated": func(r *protocol.HealthReport) { r.CPU.LoadPct = ptr(97) },
		"cpu limited":   func(r *protocol.HealthReport) { r.CPU.LimitPct = ptr(60) },
		"reboot":        func(r *protocol.HealthReport) { r.RebootPending = ptr(true) },
		"link":          func(r *protocol.HealthReport) { r.Network.LinkMbps = ptr(100) },
		"wifi":          func(r *protocol.HealthReport) { r.Network = &protocol.HealthNetwork{Kind: protocol.NetWiFi} },
		"internet":      func(r *protocol.HealthReport) { r.Internet = &protocol.HealthInternet{DNS: "ok", Reach: "fail"} },
		"battery":       func(r *protocol.HealthReport) { r.Battery = &protocol.HealthBattery{ChargePct: 90, OnAC: true} },
	} {
		cur := Simulated(1, 2, protocol.HealthPeriodic, "")
		f(&cur)
		if !Notable(base, cur) {
			t.Errorf("%s should be notable", name)
		}
	}
	small := Simulated(1, 2, protocol.HealthPeriodic, "")
	small.Disks[0].FreeBytes -= gib / 2
	small.CPU.LoadPct = ptr(40)
	if Notable(base, small) {
		t.Fatal("half a GiB and a load under 90 % are not notable")
	}
}

func alertIDs(as []Alert) []AlertID {
	var ids []AlertID
	for _, a := range as {
		ids = append(ids, a.ID)
	}
	return ids
}

func has(as []Alert, id AlertID) *Alert {
	for i := range as {
		if as[i].ID == id {
			return &as[i]
		}
	}
	return nil
}

// Each threshold, on its edge.
func TestEvaluateThresholds(t *testing.T) {
	th := DefaultThresholds()
	healthy := Simulated(1, 1, protocol.HealthPeriodic, "")
	if a := Evaluate(&healthy, nil, th); len(a) != 0 {
		t.Fatalf("a healthy computer has alerts %v", alertIDs(a))
	}
	cases := []struct {
		name  string
		f     func(*protocol.HealthReport)
		id    AlertID
		level Level
	}{
		{"disk under 5 GB", func(r *protocol.HealthReport) { r.Disks[0].FreeBytes = 5*gib - 1 }, AlertDiskSystem, LevelWarn},
		{"disk under 10 %", func(r *protocol.HealthReport) { r.Disks[0].FreeBytes, r.Disks[0].TotalBytes = 90*gib, 1000*gib }, AlertDiskSystem, LevelWarn},
		{"disk under 1 GB", func(r *protocol.HealthReport) { r.Disks[0].FreeBytes = gib - 1 }, AlertDiskSystem, LevelError},
		{"documents disk", func(r *protocol.HealthReport) { r.Disks[1].FreeBytes = 2 * gib }, AlertDiskDocuments, LevelWarn},
		{"smart", func(r *protocol.HealthReport) { r.DiskHealth = protocol.DiskDegraded }, AlertDiskFailing, LevelError},
		{"memory under 10 %", func(r *protocol.HealthReport) { r.Memory.AvailableBytes = 8*gib/10 - 1 }, AlertMemory, LevelWarn},
		{"cpu over 90", func(r *protocol.HealthReport) { r.CPU.LoadPct = ptr(91) }, AlertCPUBusy, LevelWarn},
		{"cpu limited", func(r *protocol.HealthReport) { r.CPU.LimitPct = ptr(89) }, AlertCPUHot, LevelWarn},
		{"reboot", func(r *protocol.HealthReport) { r.RebootPending = ptr(true) }, AlertReboot, LevelWarn},
		{"no dns", func(r *protocol.HealthReport) { r.Internet = &protocol.HealthInternet{DNS: "fail", Reach: "fail"} }, AlertNoDNS, LevelError},
		{"no internet", func(r *protocol.HealthReport) { r.Internet = &protocol.HealthInternet{DNS: "ok", Reach: "fail"} }, AlertNoInternet, LevelError},
		{"cable 10 Mbps", func(r *protocol.HealthReport) { r.Network.LinkMbps = ptr(10) }, AlertLinkSlow, LevelWarn},
		{"wifi weak", func(r *protocol.HealthReport) {
			r.Network = &protocol.HealthNetwork{Kind: protocol.NetWiFi, WiFiSignalPct: ptr(39), CableIdle: ptr(false)}
		}, AlertWiFiWeak, LevelWarn},
		{"wifi with cable", func(r *protocol.HealthReport) {
			r.Network = &protocol.HealthNetwork{Kind: protocol.NetWiFi, WiFiSignalPct: ptr(90), CableIdle: ptr(true)}
		}, AlertWiFiCable, LevelWarn},
		{"on battery", func(r *protocol.HealthReport) { r.Battery = &protocol.HealthBattery{ChargePct: 60} }, AlertOnBattery, LevelWarn},
		{"battery low", func(r *protocol.HealthReport) { r.Battery = &protocol.HealthBattery{ChargePct: 19} }, AlertBatteryLow, LevelError},
		{"battery worn", func(r *protocol.HealthReport) {
			r.Battery = &protocol.HealthBattery{ChargePct: 80, OnAC: true, WearPct: ptr(51)}
		}, AlertBatteryWorn, LevelWarn},
	}
	for _, c := range cases {
		r := Simulated(1, 1, protocol.HealthPeriodic, "")
		c.f(&r)
		as := Evaluate(&r, nil, th)
		a := has(as, c.id)
		if a == nil || a.Level != c.level {
			t.Errorf("%s: alerts %v, want %s %s", c.name, alertIDs(as), c.id, c.level)
			continue
		}
		// Turned off in Ajustes, it disappears.
		off := th
		off.Off = []AlertID{c.id}
		if has(Evaluate(&r, nil, off), c.id) != nil {
			t.Errorf("%s: still shown when turned off", c.name)
		}
	}
	// On the edge, no alert.
	r := Simulated(1, 1, protocol.HealthPeriodic, "")
	r.Disks[0].FreeBytes = 24 * gib // over 5 GB and ~10 % of 238
	r.CPU.LoadPct, r.CPU.LimitPct = ptr(90), ptr(90)
	r.Network.LinkMbps = ptr(100)
	if a := Evaluate(&r, nil, th); len(a) != 0 {
		t.Fatalf("on the edge: %v", alertIDs(a))
	}
}

// The clock, from the heartbeats: over 2 min either way is an error.
func TestEvaluateClock(t *testing.T) {
	th := DefaultThresholds()
	for skew, want := range map[time.Duration]bool{
		0: false, 119 * time.Second: false, 121 * time.Second: true, -5 * time.Minute: true,
	} {
		a := has(Evaluate(nil, &skew, th), AlertClock)
		if (a != nil) != want {
			t.Errorf("skew %v: alert %v, want %v", skew, a, want)
		}
		if a != nil && (a.Level != LevelError || a.Values["seconds"] != int(skew/time.Second)) {
			t.Errorf("skew %v: %+v", skew, a)
		}
	}
	if a := Evaluate(nil, nil, th); len(a) != 0 {
		t.Fatalf("no report and no clock: %v", alertIDs(a))
	}
}

func TestThresholdsAndTargets(t *testing.T) {
	if err := DefaultThresholds().Validate(); err != nil {
		t.Fatal(err)
	}
	for _, f := range []func(*Thresholds){
		func(t *Thresholds) { t.DiskMinGB = 0 },
		func(t *Thresholds) { t.LinkMinMbps = 50 },
		func(t *Thresholds) { t.ClockMaxS = 5 },
		func(t *Thresholds) { t.Off = []AlertID{"processes"} },
	} {
		th := DefaultThresholds()
		f(&th)
		if !errors.Is(th.Validate(), ErrThreshold) {
			t.Errorf("%+v should be out of range", th)
		}
	}
	c, err := Targets{Internet: "www.google.com:443", Services: []Service{{Name: "Licencias", Address: "10.0.0.5:27000"}}}.Config()
	if err != nil || c.Internet.Port != 443 || c.Services[0].Host != "10.0.0.5" {
		t.Fatalf("config %+v, %v", c, err)
	}
	if c, err := (Targets{}).Config(); err != nil || c.Internet != nil || c.Services != nil {
		t.Fatalf("empty targets: %+v, %v", c, err)
	}
	for _, bad := range []Targets{
		{Internet: "www.google.com"}, {Internet: "http://x.com:80"}, {Internet: "x.com:0"},
		{Services: []Service{{Name: "", Address: "x.com:1"}}},
		{Services: []Service{{Name: "a", Address: "x.com"}}},
		{Services: make([]Service, protocol.MaxHealthServices+1)},
	} {
		if _, err := bad.Config(); err == nil {
			t.Errorf("%+v should be refused", bad)
		}
	}
}

func TestLabReportsClockAndRate(t *testing.T) {
	clk := &clock{t: time.Unix(1_800_000_000, 0)}
	l := NewLab(DefaultThresholds(), quiet)
	l.Now = clk.now
	r := Simulated(2, 1, protocol.HealthFirst, "")
	if !l.Report("pc02", "PC02", false, r) {
		t.Fatal("first report refused")
	}
	clk.add(time.Second)
	if l.Report("pc02", "PC02", false, Simulated(2, 2, protocol.HealthChange, "")) {
		t.Fatal("a report 1 s after another must be dropped")
	}
	if !l.Report("pc02", "PC02", false, Simulated(2, 3, protocol.HealthOnDemand, "0c9a7e52-6a4b-4f7e-8d33-2b1f4e6a9c10")) {
		t.Fatal("an answer to «Revisar ahora» is never dropped")
	}
	if !l.Clock("pc02", "PC02", false, 4*time.Minute) || l.Clock("pc02", "PC02", false, 4*time.Minute+300*time.Millisecond) {
		t.Fatal("a new clock is news; 300 ms of jitter is not")
	}
	c, _ := l.Get("pc02")
	if ids := alertIDs(c.Alerts); len(ids) != 3 || ids[0] != AlertClock || ids[1] != AlertDiskSystem || ids[2] != AlertDiskDocuments {
		t.Fatalf("alerts %v", ids)
	}
	if c.ClockMS == nil || *c.ClockMS != 240000 {
		t.Fatalf("clock %v", c.ClockMS)
	}
	// Raising the disk threshold re-evaluates; disconnecting keeps the report.
	th := DefaultThresholds()
	th.Off = []AlertID{AlertClock, AlertDiskSystem, AlertDiskDocuments}
	if err := l.SetThresholds(th); err != nil {
		t.Fatal(err)
	}
	l.SetConnectedOnly(map[string]bool{})
	if c, _ := l.Get("pc02"); len(c.Alerts) != 0 || c.Connected || c.Report == nil {
		t.Fatalf("after thresholds and disconnect: %+v", c)
	}
	if !l.TryRequest() || l.TryRequest() {
		t.Fatal("«Revisar ahora» twice within 10 s")
	}
	clk.add(RequestGap)
	if !l.TryRequest() {
		t.Fatal("«Revisar ahora» after 10 s")
	}
}

func TestLabSelfTest(t *testing.T) {
	clk := &clock{t: time.Unix(1_800_000_000, 0)}
	l := NewLab(DefaultThresholds(), quiet)
	l.Now = clk.now
	id := l.StartTest(map[string]Target{"pc02": {Hostname: "PC02"}, "pc03": {Hostname: "PC03"}})
	if !l.Testing([]string{"pc03"}) {
		t.Fatal("pending test not seen")
	}
	if l.TestResult("pc02", protocol.HealthSelfTestResult{TestID: "other", Status: protocol.SelfTestDone}) {
		t.Fatal("a result of another test was kept")
	}
	if !l.TestResult("pc02", SimulatedSelfTest(2, nil, "r", id)) {
		t.Fatal("result refused")
	}
	clk.add(SelfTestNoWait)
	if !l.Expire() {
		t.Fatal("pc03 should expire")
	}
	c2, _ := l.Get("pc02")
	c3, _ := l.Get("pc03")
	if c2.Test.Status != TestDone || c2.Test.Failed != 1 || c3.Test.Status != TestNoAnswer {
		t.Fatalf("pc02 %+v, pc03 %+v", c2.Test, c3.Test)
	}
}

func TestSelfTestBusyAndChecks(t *testing.T) {
	sys, clk := newFakeSys(), &clock{t: time.Unix(1_800_000_000, 0)}
	sys.devices.GPU = platform.DeviceBasicDriver
	sys.devices.Mouse = platform.DeviceMissing
	pr := &fakeProber{tcErr: errors.New("dial tcp: connection refused")}
	s := &SelfTest{Sys: sys, TempDir: t.TempDir(), Prober: pr, Now: clk.now, Supported: true}
	cfg := &protocol.HealthConfig{Internet: &protocol.HealthTarget{Host: "x.com", Port: 443},
		Services: []protocol.HealthService{{Name: "Git", Host: "git.local", Port: 22}}}
	ref, id := "0c9a7e52-6a4b-4f7e-8d33-2b1f4e6a9c10", "5b0e8f3c-2a6d-4c1e-9b7a-0f3d2e1c4b5a"
	res := s.Run(context.Background(), cfg, ref, id)
	if err := res.Validate(); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"disk_write": "ok", "dns": "ok", "tcp": "fail:refused", "dns#0": "ok", "tcp#0": "fail:refused",
		"audio_out": "ok", "nic": "ok", "gpu": "fail:basic_driver", "keyboard": "ok", "mouse": "fail:missing",
	}
	if len(res.Checks) != len(want) {
		t.Fatalf("checks %+v", res.Checks)
	}
	for _, c := range res.Checks {
		k := string(c.ID)
		if c.Service != nil {
			k += "#0"
		}
		got := string(c.Status)
		if c.Reason != "" {
			got += ":" + c.Reason
		}
		if want[k] != got {
			t.Errorf("%s = %s, want %s", k, got, want[k])
		}
	}
	if r := s.Run(context.Background(), cfg, ref, id); r.Status != protocol.SelfTestBusy {
		t.Fatalf("a second test within a minute: %s", r.Status)
	}
	clk.add(SelfTestCooldown)
	if r := s.Run(context.Background(), nil, ref, id); r.Status != protocol.SelfTestDone || len(r.Checks) != 6 {
		t.Fatalf("after a minute, without destinations: %+v", r)
	}
	if r := (&SelfTest{}).Run(context.Background(), nil, ref, id); r.Status != protocol.SelfTestUnsupported {
		t.Fatalf("unsupported: %s", r.Status)
	}
}

func TestReasonClassification(t *testing.T) {
	for err, want := range map[error]string{
		&net.DNSError{IsNotFound: true}:           protocol.ReasonNotFound,
		context.DeadlineExceeded:                  protocol.ReasonTimeout,
		errors.New("connectex: actively refused"): protocol.ReasonRefused,
		errors.New("network is unreachable"):      protocol.ReasonUnreachable,
	} {
		if got := Reason(err); got != want {
			t.Errorf("Reason(%v) = %s, want %s", err, got, want)
		}
	}
}

func TestSimulatedValid(t *testing.T) {
	th := DefaultThresholds()
	alerts := 0
	for n := 1; n <= 10; n++ {
		r := Simulated(n, 1, protocol.HealthPeriodic, "")
		if err := r.Validate(); err != nil {
			t.Fatalf("simulated %d: %v", n, err)
		}
		if len(Evaluate(&r, nil, th)) > 0 {
			alerts++
		}
		cfg := &protocol.HealthConfig{Internet: &protocol.HealthTarget{Host: "x", Port: 1}, Services: []protocol.HealthService{{Name: "a", Host: "b", Port: 1}}}
		if err := SimulatedSelfTest(n, cfg, "0c9a7e52-6a4b-4f7e-8d33-2b1f4e6a9c10", "5b0e8f3c-2a6d-4c1e-9b7a-0f3d2e1c4b5a").Validate(); err != nil {
			t.Fatalf("simulated self-test %d: %v", n, err)
		}
	}
	if alerts < 4 || alerts > 8 {
		t.Fatalf("%d of 10 simulated have alerts; some should, not all", alerts)
	}
}

func TestAgentHandle(t *testing.T) {
	sys, clk, out := newFakeSys(), &clock{t: time.Unix(1_800_000_000, 0)}, &outbox{}
	a := &Agent{Monitor: newMonitor(sys, clk, &fakeProber{}), SelfTest: &SelfTest{Sys: sys, TempDir: t.TempDir(), Now: clk.now, Supported: true}, Logger: quiet}
	if caps := a.Capabilities(); len(caps) != 2 {
		t.Fatalf("caps %v", caps)
	}
	a.Monitor.Start(out.send)
	ctx := context.Background()
	if !a.Handle(ctx, out.send, protocol.New(protocol.HealthConfig{})) {
		t.Fatal("health.config not handled")
	}
	if !a.Handle(ctx, out.send, protocol.New(protocol.HealthSelfTest{TestID: "5b0e8f3c-2a6d-4c1e-9b7a-0f3d2e1c4b5a"})) {
		t.Fatal("health.selftest not handled")
	}
	if a.Handle(ctx, out.send, protocol.New(protocol.CodeClear{})) {
		t.Fatal("code.clear is not a health message")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		out.mu.Lock()
		n := len(out.sent)
		out.mu.Unlock()
		if n == 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the self-test result never came")
}
