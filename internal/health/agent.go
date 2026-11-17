package health

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// Rhythm of the reports (docs/protocol.md §5.27.3).
const (
	SampleEvery    = 5 * time.Second  // processor samples and change checks
	ReportEvery    = 60 * time.Second // periodic reports, ±10 %
	ChangeGap      = 10 * time.Second // at most one change report this often
	ProbeReuse     = 10 * time.Second // a request reuses a probe this recent
	loadWindow     = time.Minute      // the load is averaged over this
	maxCPUSamples  = int(loadWindow/SampleEvery) + 1
	reportJitter   = 0.10
	gib            = 1 << 30
	diskBandBytes  = 1 * gib
	diskBandPoints = 5
	memBandPoints  = 10
	cpuBusyPct     = 90
	limitBand      = 10
	batteryLowPct  = 20
)

// Sender sends one message to the console of the session.
type Sender func(ctx context.Context, p protocol.Payload) error

type cpuSample struct {
	at    time.Time
	times platform.CPUTimes
}

// Monitor is the agent's side of the report: it lives as long as the
// agent, and Start and Stop attach it to each session.
type Monitor struct {
	Sys platform.SystemHealth
	// Documents gives the Documents folder of the session's user, "" with
	// nobody logged on. Nil: the documents disk is not reported.
	Documents func() string
	// LocalIP gives this computer's address towards the console, nil if
	// unknown. Nil: the network card is not reported.
	LocalIP func() net.IP
	Prober  Prober
	Logger  *slog.Logger
	Now     func() time.Time // nil: time.Now
	Jitter  func() float64   // in [-1, 1); nil: random

	sendMu sync.Mutex // one report at a time, in seq order

	mu           sync.Mutex
	samples      []cpuSample
	send         Sender
	cfg          *protocol.HealthConfig
	seq          int
	last         *protocol.HealthReport
	lastSent     time.Time
	nextPeriodic time.Time
	probe        *protocol.HealthInternet
	probedAt     time.Time
}

func (m *Monitor) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m *Monitor) log() *slog.Logger {
	if m.Logger != nil {
		return m.Logger
	}
	return slog.Default()
}

// Run samples the processor and checks the reports every SampleEvery
// until ctx ends.
func (m *Monitor) Run(ctx context.Context) {
	t := time.NewTicker(SampleEvery)
	defer t.Stop()
	m.Tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.Tick(ctx)
		}
	}
}

// Start attaches the monitor to a new session. It reports nothing until
// the console sends its first health.config.
func (m *Monitor) Start(send Sender) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.send, m.cfg, m.seq, m.last, m.probe = send, nil, 0, nil, nil
	m.lastSent, m.probedAt = time.Time{}, time.Time{}
}

// Stop detaches the monitor from its session and forgets its config.
func (m *Monitor) Stop() { m.Start(nil) }

// Config returns the destinations of the session, nil before the first
// health.config.
func (m *Monitor) Config() *protocol.HealthConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

// Configure keeps the destinations of health.config; the first of the
// session sends the first report.
func (m *Monitor) Configure(ctx context.Context, cfg protocol.HealthConfig) {
	m.mu.Lock()
	first := m.cfg == nil
	m.cfg = &cfg
	m.probe, m.probedAt = nil, time.Time{}
	m.mu.Unlock()
	if first {
		m.report(ctx, protocol.HealthFirst, "", true)
	}
}

// Request answers health.request with id refID.
func (m *Monitor) Request(ctx context.Context, refID string) {
	m.mu.Lock()
	fresh := m.probe != nil && m.now().Sub(m.probedAt) < ProbeReuse
	m.mu.Unlock()
	m.report(ctx, protocol.HealthOnDemand, refID, !fresh)
}

// Tick takes a processor sample and sends the periodic report when due, or
// a change report on a notable change.
func (m *Monitor) Tick(ctx context.Context) {
	if m.Sys == nil {
		return
	}
	now := m.now()
	if t, err := m.Sys.CPUTimes(); err == nil {
		m.mu.Lock()
		m.samples = append(m.samples, cpuSample{now, t})
		if len(m.samples) > maxCPUSamples {
			m.samples = m.samples[len(m.samples)-maxCPUSamples:]
		}
		m.mu.Unlock()
	}
	m.mu.Lock()
	active := m.send != nil && m.cfg != nil
	due := !now.Before(m.nextPeriodic)
	gap := now.Sub(m.lastSent) >= ChangeGap
	last := m.last
	m.mu.Unlock()
	switch {
	case !active:
	case due:
		m.report(ctx, protocol.HealthPeriodic, "", true)
	case gap && last != nil:
		cur := m.read()
		m.mu.Lock()
		cur.Internet = m.probe
		m.mu.Unlock()
		if Notable(*last, cur) {
			m.report(ctx, protocol.HealthChange, "", false)
		}
	}
}

// report reads, probes internet if asked and configured, and sends.
func (m *Monitor) report(ctx context.Context, reason protocol.HealthReason, refID string, probe bool) {
	m.sendMu.Lock()
	defer m.sendMu.Unlock()
	r := m.read()
	m.mu.Lock()
	cfg, send := m.cfg, m.send
	m.mu.Unlock()
	if send == nil {
		return
	}
	if cfg != nil && cfg.Internet != nil {
		if probe {
			p := probeTarget(ctx, m.prober(), *cfg.Internet)
			m.mu.Lock()
			m.probe, m.probedAt = &p, m.now()
			m.mu.Unlock()
		}
		m.mu.Lock()
		r.Internet = m.probe
		m.mu.Unlock()
	}
	m.mu.Lock()
	if m.send == nil { // the session ended meanwhile
		m.mu.Unlock()
		return
	}
	m.seq++
	r.Seq, r.Reason, r.RefID = m.seq, reason, refID
	now := m.now()
	m.last, m.lastSent = &r, now
	m.nextPeriodic = now.Add(time.Duration(float64(ReportEvery) * (1 + reportJitter*m.jitter())))
	m.mu.Unlock()
	if err := send(ctx, r); err != nil {
		m.log().Debug("health report not sent", "seq", r.Seq, "err", err)
		return
	}
	m.log().Debug("health report", "seq", r.Seq, "reason", reason)
}

func (m *Monitor) jitter() float64 {
	if m.Jitter != nil {
		return m.Jitter()
	}
	return rand.Float64()*2 - 1
}

func (m *Monitor) prober() Prober {
	if m.Prober != nil {
		return m.Prober
	}
	return NetProber{}
}

// loadPct is the average processor use over the samples of the last
// minute, or -1 with fewer than two.
func (m *Monitor) loadPct() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.samples) < 2 {
		return -1
	}
	first, last := m.samples[0], m.samples[len(m.samples)-1]
	for _, s := range m.samples {
		if last.at.Sub(s.at) <= loadWindow {
			first = s
			break
		}
	}
	total := last.times.Total - first.times.Total
	idle := last.times.Idle - first.times.Idle
	if total == 0 || idle > total || last.times.Total < first.times.Total {
		return -1
	}
	return int((total - idle) * 100 / total)
}

func ptr[T any](v T) *T { return &v }

// Read is the report of the closed list as it would go now, without seq,
// reason nor the internet probe ("aula-cliente diagnostico").
func (m *Monitor) Read() protocol.HealthReport { return m.read() }

// read builds the report of the closed list (§5.27.3) with what the OS can
// give; whatever fails does not travel.
func (m *Monitor) read() protocol.HealthReport {
	var r protocol.HealthReport
	s := m.Sys
	if root, err := s.SystemDrive(); err == nil {
		if d, err := s.DiskSpace(root); err == nil {
			r.Disks = append(r.Disks, protocol.HealthDisk{Role: protocol.DiskSystem, FreeBytes: d.Free, TotalBytes: d.Total})
		}
	}
	if m.Documents != nil {
		if dir := m.Documents(); dir != "" {
			if d, err := s.DiskSpace(dir); err == nil {
				r.Disks = append(r.Disks, protocol.HealthDisk{Role: protocol.DiskDocuments, FreeBytes: d.Free, TotalBytes: d.Total})
			}
		}
	}
	if failing, err := s.DiskFailing(); err == nil {
		r.DiskHealth = protocol.DiskOK
		if failing {
			r.DiskHealth = protocol.DiskDegraded
		}
	}
	if mem, err := s.Memory(); err == nil && mem.Total > 0 {
		r.Memory = &protocol.HealthMemory{TotalBytes: mem.Total, AvailableBytes: min(mem.Available, mem.Total)}
	}
	var cpu protocol.HealthCPU
	if l := m.loadPct(); l >= 0 {
		cpu.LoadPct = ptr(min(l, 100))
	}
	if l, err := s.CPULimitPct(); err == nil {
		cpu.LimitPct = ptr(max(1, min(l, 100)))
	}
	if cpu.LoadPct != nil || cpu.LimitPct != nil {
		r.CPU = &cpu
	}
	if u, err := s.Uptime(); err == nil {
		r.UptimeS = ptr(min(int64(u/time.Second), protocol.MaxHealthUptimeS))
	}
	if p, err := s.RebootPending(); err == nil {
		r.RebootPending = ptr(p)
	}
	if m.LocalIP != nil {
		if ip := m.LocalIP(); ip != nil {
			if l, err := s.Link(ip); err == nil {
				r.Network = network(l)
			}
		}
	}
	if b, err := s.Battery(); err == nil && b.Present {
		bat := &protocol.HealthBattery{ChargePct: max(0, min(b.ChargePct, 100)), OnAC: b.OnAC}
		if b.WearPct >= 0 {
			bat.WearPct = ptr(min(b.WearPct, 100))
		}
		r.Battery = bat
	}
	return r
}

func network(l platform.Link) *protocol.HealthNetwork {
	n := &protocol.HealthNetwork{Kind: protocol.NetOther}
	switch l.Kind {
	case platform.LinkEthernet:
		n.Kind = protocol.NetEthernet
	case platform.LinkWiFi:
		n.Kind = protocol.NetWiFi
		if l.SignalPct >= 0 {
			n.WiFiSignalPct = ptr(min(l.SignalPct, 100))
		}
		n.CableIdle = ptr(l.CableIdle)
	}
	if l.Mbps >= 0 {
		n.LinkMbps = ptr(min(l.Mbps, protocol.MaxHealthLinkMbps))
	}
	return n
}

// Notable reports whether cur differs enough from prev to send it before
// its turn, with the fixed bands of docs/protocol.md §5.27.3.
func Notable(prev, cur protocol.HealthReport) bool {
	disk := func(r protocol.HealthReport, role protocol.DiskRole) *protocol.HealthDisk {
		for i := range r.Disks {
			if r.Disks[i].Role == role {
				return &r.Disks[i]
			}
		}
		return nil
	}
	for _, role := range []protocol.DiskRole{protocol.DiskSystem, protocol.DiskDocuments} {
		a, b := disk(prev, role), disk(cur, role)
		if (a == nil) != (b == nil) {
			return true
		}
		if a != nil && (abs(a.FreeBytes-b.FreeBytes) > diskBandBytes || abs(pct(a.FreeBytes, a.TotalBytes)-pct(b.FreeBytes, b.TotalBytes)) > diskBandPoints) {
			return true
		}
	}
	if prev.DiskHealth != cur.DiskHealth {
		return true
	}
	if a, b := prev.Memory, cur.Memory; (a == nil) != (b == nil) ||
		a != nil && abs(pct(a.AvailableBytes, a.TotalBytes)-pct(b.AvailableBytes, b.TotalBytes)) > memBandPoints {
		return true
	}
	if a, b := prev.CPU, cur.CPU; a != nil && b != nil {
		if a.LoadPct != nil && b.LoadPct != nil && (*a.LoadPct > cpuBusyPct) != (*b.LoadPct > cpuBusyPct) {
			return true
		}
		if a.LimitPct != nil && b.LimitPct != nil && abs(int64(*a.LimitPct-*b.LimitPct)) > limitBand {
			return true
		}
	}
	if !sameBool(prev.RebootPending, cur.RebootPending) {
		return true
	}
	if a, b := prev.Network, cur.Network; (a == nil) != (b == nil) ||
		a != nil && (a.Kind != b.Kind || !sameInt(a.LinkMbps, b.LinkMbps) || !sameBool(a.CableIdle, b.CableIdle)) {
		return true
	}
	if a, b := prev.Internet, cur.Internet; (a == nil) != (b == nil) || a != nil && *a != *b {
		return true
	}
	if a, b := prev.Battery, cur.Battery; (a == nil) != (b == nil) ||
		a != nil && (a.OnAC != b.OnAC || (a.ChargePct < batteryLowPct) != (b.ChargePct < batteryLowPct)) {
		return true
	}
	return false
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func pct(part, total int64) int64 {
	if total <= 0 {
		return 0
	}
	return part * 100 / total
}

func sameBool(a, b *bool) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
func sameInt(a, b *int) bool   { return a == nil && b == nil || a != nil && b != nil && *a == *b }

// SelfTestCooldown is how long after a self-test another one answers busy.
const (
	SelfTestCooldown = time.Minute
	SelfTestTimeout  = 15 * time.Second
)

// SelfTest runs the self-test of §5.27.5, one at a time.
type SelfTest struct {
	Sys platform.SystemHealth
	// TempDir is the agent's temporary folder, where disk_write writes and
	// deletes its file. Never the student's Documents.
	TempDir string
	Prober  Prober
	Now     func() time.Time
	// Supported is false where the OS has no self-test: it answers
	// unsupported.
	Supported bool

	mu      sync.Mutex
	running bool
	lastAt  time.Time
}

func (s *SelfTest) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Run runs the self-test with the destinations of cfg (nil: none) and
// returns its result for test_id testID, answering refID.
func (s *SelfTest) Run(ctx context.Context, cfg *protocol.HealthConfig, refID, testID string) protocol.HealthSelfTestResult {
	res := protocol.HealthSelfTestResult{RefID: refID, TestID: testID}
	if !s.Supported || s.Sys == nil {
		res.Status = protocol.SelfTestUnsupported
		return res
	}
	s.mu.Lock()
	if s.running || !s.lastAt.IsZero() && s.now().Sub(s.lastAt) < SelfTestCooldown {
		s.mu.Unlock()
		res.Status = protocol.SelfTestBusy
		return res
	}
	s.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running, s.lastAt = false, s.now()
		s.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(ctx, SelfTestTimeout)
	defer cancel()

	res.Status = protocol.SelfTestDone
	res.Checks = append(res.Checks, s.diskWrite())
	p := s.Prober
	if p == nil {
		p = NetProber{}
	}
	if cfg != nil {
		if cfg.Internet != nil {
			res.Checks = append(res.Checks, probeChecks(ctx, p, nil, *cfg.Internet)...)
		}
		for i, sv := range cfg.Services {
			res.Checks = append(res.Checks, probeChecks(ctx, p, ptr(i), protocol.HealthTarget{Host: sv.Host, Port: sv.Port})...)
		}
	}
	res.Checks = append(res.Checks, s.devices()...)
	return res
}

func (s *SelfTest) diskWrite() protocol.HealthCheck {
	c := protocol.HealthCheck{ID: protocol.CheckDiskWrite, Status: protocol.CheckOK}
	if s.TempDir == "" {
		c.Status, c.Reason = protocol.CheckUnknown, protocol.ReasonNotSupported
		return c
	}
	if err := platform.DiskWriteTest(s.TempDir, protocol.HealthSelfTestFileLen); err != nil {
		c.Status = protocol.CheckFail
		switch {
		case errors.Is(err, platform.ErrNoSpace):
			c.Reason = protocol.ReasonNoSpace
		case errors.Is(err, platform.ErrReadOnly):
			c.Reason = protocol.ReasonReadOnly
		case errors.Is(err, platform.ErrDataMismatch):
			c.Reason = protocol.ReasonMismatch
		}
	}
	return c
}

// probeChecks resolves and connects to t: a dns check and a tcp check.
func probeChecks(ctx context.Context, p Prober, service *int, t protocol.HealthTarget) []protocol.HealthCheck {
	dns := protocol.HealthCheck{ID: protocol.CheckDNS, Service: service, Status: protocol.CheckOK}
	tcp := protocol.HealthCheck{ID: protocol.CheckTCP, Service: service, Status: protocol.CheckOK}
	if err := p.Resolve(ctx, t.Host); err != nil {
		dns.Status, dns.Reason = protocol.CheckFail, Reason(err)
		tcp.Status, tcp.Reason = protocol.CheckFail, protocol.ReasonNotFound
		return []protocol.HealthCheck{dns, tcp}
	}
	if err := p.Dial(ctx, t.Host, t.Port); err != nil {
		tcp.Status, tcp.Reason = protocol.CheckFail, Reason(err)
	}
	return []protocol.HealthCheck{dns, tcp}
}

func (s *SelfTest) devices() []protocol.HealthCheck {
	d, err := s.Sys.Devices()
	ids := []protocol.CheckID{protocol.CheckAudioOut, protocol.CheckNIC, protocol.CheckGPU, protocol.CheckKeyboard, protocol.CheckMouse}
	states := []platform.DeviceState{d.AudioOut, d.NIC, d.GPU, d.Keyboard, d.Mouse}
	out := make([]protocol.HealthCheck, len(ids))
	for i, id := range ids {
		c := protocol.HealthCheck{ID: id}
		switch {
		case err != nil:
			c.Status, c.Reason = protocol.CheckUnknown, protocol.ReasonNotSupported
		case states[i] == platform.DeviceOK:
			c.Status = protocol.CheckOK
		case states[i] == platform.DeviceMissing:
			c.Status, c.Reason = protocol.CheckFail, protocol.ReasonMissing
		case states[i] == platform.DeviceBasicDriver:
			c.Status, c.Reason = protocol.CheckFail, protocol.ReasonBasicDriver
		default:
			c.Status = protocol.CheckUnknown
		}
		out[i] = c
	}
	return out
}

// Agent joins the monitor and the self-test of an agent and handles the
// health messages of the console.
type Agent struct {
	Monitor  *Monitor
	SelfTest *SelfTest
	Logger   *slog.Logger
}

// Capabilities are what the agent declares: health always (a datum the OS
// cannot read does not travel), health.selftest where it runs.
func (a *Agent) Capabilities() []protocol.Capability {
	caps := []protocol.Capability{protocol.CapHealth}
	if a.SelfTest != nil && a.SelfTest.Supported {
		caps = append(caps, protocol.CapHealthSelfTest)
	}
	return caps
}

// Handle handles a health message from the console and reports whether it
// was one. send answers on the same session. The self-test runs in its own
// goroutine, bounded by SelfTestTimeout.
func (a *Agent) Handle(ctx context.Context, send Sender, env protocol.Envelope) bool {
	switch p := env.Payload.(type) {
	case protocol.HealthConfig:
		a.Monitor.Configure(ctx, p)
	case protocol.HealthRequest:
		a.Monitor.Request(ctx, env.ID)
	case protocol.HealthSelfTest:
		go func() {
			res := a.SelfTest.Run(ctx, a.Monitor.Config(), env.ID, p.TestID)
			if err := send(ctx, res); err != nil {
				a.log().Debug("self-test result not sent", "test_id", p.TestID, "err", err)
				return
			}
			a.log().Info("self-test", "test_id", p.TestID, "status", res.Status, "checks", len(res.Checks))
		}()
	default:
		return false
	}
	return true
}

func (a *Agent) log() *slog.Logger {
	if a.Logger != nil {
		return a.Logger
	}
	return slog.Default()
}

// LocalIPTowards gives the address this computer uses to reach host, with a
// UDP socket that sends nothing; nil if there is no route.
func LocalIPTowards(host string) net.IP {
	c, err := net.Dial("udp", net.JoinHostPort(host, "9"))
	if err != nil {
		return nil
	}
	defer c.Close()
	if a, ok := c.LocalAddr().(*net.UDPAddr); ok {
		return a.IP
	}
	return nil
}
