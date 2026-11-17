package health

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// AlertID names an alert. The console has a text "qué pasa + qué hacer"
// and a manual section for each.
type AlertID string

// The alerts (docs/administracion/laboratorio.md).
const (
	AlertDiskSystem    AlertID = "diskSystem"    // the system disk is almost full
	AlertDiskDocuments AlertID = "diskDocuments" // the Documents disk is almost full
	AlertDiskFailing   AlertID = "diskFailing"   // S.M.A.R.T. predicts a failure
	AlertMemory        AlertID = "memory"        // little memory available
	AlertCPUBusy       AlertID = "cpuBusy"       // the processor is saturated
	AlertCPUHot        AlertID = "cpuHot"        // the processor is held back (heat)
	AlertReboot        AlertID = "rebootPending" // Windows waits for a reboot
	AlertClock         AlertID = "clock"         // the clock is off (breaks TLS)
	AlertNoDNS         AlertID = "noDns"         // names do not resolve
	AlertNoInternet    AlertID = "noInternet"    // the internet destination is unreachable
	AlertLinkSlow      AlertID = "linkSlow"      // the cable negotiated a low speed
	AlertWiFiWeak      AlertID = "wifiWeak"      // weak Wi-Fi signal
	AlertWiFiCable     AlertID = "wifiCable"     // on Wi-Fi with a cable plugged in
	AlertOnBattery     AlertID = "onBattery"     // unplugged from the power
	AlertBatteryLow    AlertID = "batteryLow"    // unplugged and almost empty
	AlertBatteryWorn   AlertID = "batteryWorn"   // the battery lost much capacity
)

// AllAlerts lists every alert, in the order the console shows them.
var AllAlerts = []AlertID{
	AlertDiskFailing, AlertClock, AlertNoDNS, AlertNoInternet, AlertDiskSystem, AlertDiskDocuments,
	AlertMemory, AlertCPUBusy, AlertCPUHot, AlertReboot, AlertLinkSlow, AlertWiFiCable, AlertWiFiWeak,
	AlertBatteryLow, AlertOnBattery, AlertBatteryWorn,
}

// Level of an alert.
type Level string

// Alert levels: an error keeps the computer from working in class; a
// warning will bother.
const (
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Alert is one alert of a computer, with the values its text shows.
type Alert struct {
	ID     AlertID        `json:"id"`
	Level  Level          `json:"level"`
	Values map[string]any `json:"values,omitempty"`
}

// Thresholds are the limits of Ajustes › Salud.
type Thresholds struct {
	DiskMinGB         int       `json:"diskMinGB"`
	DiskMinPct        int       `json:"diskMinPct"`
	MemoryMinPct      int       `json:"memoryMinPct"`
	CPUMaxPct         int       `json:"cpuMaxPct"`
	CPULimitMinPct    int       `json:"cpuLimitMinPct"`
	ClockMaxS         int       `json:"clockMaxS"`
	LinkMinMbps       int       `json:"linkMinMbps"`
	WiFiMinPct        int       `json:"wifiMinPct"`
	BatteryMinPct     int       `json:"batteryMinPct"`
	BatteryWearMaxPct int       `json:"batteryWearMaxPct"`
	Off               []AlertID `json:"off,omitempty"` // alerts turned off
}

// DefaultThresholds are the limits by default (prompt 10.25).
func DefaultThresholds() Thresholds {
	return Thresholds{
		DiskMinGB: 5, DiskMinPct: 10, MemoryMinPct: 10, CPUMaxPct: 90, CPULimitMinPct: 90,
		ClockMaxS: 120, LinkMinMbps: 100, WiFiMinPct: 40, BatteryMinPct: 20, BatteryWearMaxPct: 50,
	}
}

// Errors of the settings; the console maps each to its code.
var (
	ErrThreshold = errors.New("health: threshold out of range")
	ErrTarget    = errors.New("health: destination is not host:port")
	ErrService   = errors.New("health: service needs a name and host:port")
)

// LinkSpeeds are the cable speeds the minimum may take.
var LinkSpeeds = []int{10, 100, 1000}

// Validate checks every threshold against its range.
func (t Thresholds) Validate() error {
	in := func(v, lo, hi int) bool { return v >= lo && v <= hi }
	switch {
	case !in(t.DiskMinGB, 1, 500), !in(t.DiskMinPct, 1, 50), !in(t.MemoryMinPct, 1, 50),
		!in(t.CPUMaxPct, 50, 100), !in(t.CPULimitMinPct, 10, 100), !in(t.ClockMaxS, 10, 3600),
		!slices.Contains(LinkSpeeds, t.LinkMinMbps), !in(t.WiFiMinPct, 1, 100),
		!in(t.BatteryMinPct, 1, 90), !in(t.BatteryWearMaxPct, 10, 90):
		return ErrThreshold
	}
	for _, id := range t.Off {
		if !slices.Contains(AllAlerts, id) {
			return ErrThreshold
		}
	}
	return nil
}

func (t Thresholds) on(id AlertID) bool { return !slices.Contains(t.Off, id) }

// Evaluate turns a report and the clock skew (nil: unknown) into alerts,
// in the order of AllAlerts.
func Evaluate(r *protocol.HealthReport, skew *time.Duration, t Thresholds) []Alert {
	var out []Alert
	add := func(id AlertID, l Level, v map[string]any) {
		if t.on(id) {
			out = append(out, Alert{ID: id, Level: l, Values: v})
		}
	}
	if skew != nil {
		if d := *skew; d > time.Duration(t.ClockMaxS)*time.Second || -d > time.Duration(t.ClockMaxS)*time.Second {
			add(AlertClock, LevelError, map[string]any{"seconds": int(d / time.Second)})
		}
	}
	if r != nil {
		if r.DiskHealth == protocol.DiskDegraded {
			add(AlertDiskFailing, LevelError, nil)
		}
		if i := r.Internet; i != nil {
			if i.DNS == protocol.ProbeFail {
				add(AlertNoDNS, LevelError, nil)
			} else if i.Reach == protocol.ProbeFail {
				add(AlertNoInternet, LevelError, nil)
			}
		}
		for _, d := range r.Disks {
			id := AlertDiskSystem
			if d.Role == protocol.DiskDocuments {
				id = AlertDiskDocuments
			}
			p := pct(d.FreeBytes, d.TotalBytes)
			if d.FreeBytes < int64(t.DiskMinGB)*gib || p < int64(t.DiskMinPct) {
				l := LevelWarn
				if d.FreeBytes < gib {
					l = LevelError
				}
				add(id, l, map[string]any{"free": d.FreeBytes, "total": d.TotalBytes, "pct": p})
			}
		}
		if m := r.Memory; m != nil {
			if p := pct(m.AvailableBytes, m.TotalBytes); p < int64(t.MemoryMinPct) {
				add(AlertMemory, LevelWarn, map[string]any{"available": m.AvailableBytes, "total": m.TotalBytes, "pct": p})
			}
		}
		if c := r.CPU; c != nil {
			if c.LoadPct != nil && *c.LoadPct > t.CPUMaxPct {
				add(AlertCPUBusy, LevelWarn, map[string]any{"pct": *c.LoadPct})
			}
			if c.LimitPct != nil && *c.LimitPct < t.CPULimitMinPct {
				add(AlertCPUHot, LevelWarn, map[string]any{"pct": *c.LimitPct})
			}
		}
		if r.RebootPending != nil && *r.RebootPending {
			v := map[string]any{}
			if r.UptimeS != nil {
				v["uptime"] = *r.UptimeS
			}
			add(AlertReboot, LevelWarn, v)
		}
		if n := r.Network; n != nil {
			if n.Kind == protocol.NetEthernet && n.LinkMbps != nil && *n.LinkMbps > 0 && *n.LinkMbps < t.LinkMinMbps {
				add(AlertLinkSlow, LevelWarn, map[string]any{"mbps": *n.LinkMbps, "min": t.LinkMinMbps})
			}
			if n.Kind == protocol.NetWiFi && n.CableIdle != nil && *n.CableIdle {
				add(AlertWiFiCable, LevelWarn, nil)
			}
			if n.Kind == protocol.NetWiFi && n.WiFiSignalPct != nil && *n.WiFiSignalPct < t.WiFiMinPct {
				add(AlertWiFiWeak, LevelWarn, map[string]any{"pct": *n.WiFiSignalPct})
			}
		}
		if b := r.Battery; b != nil {
			switch {
			case !b.OnAC && b.ChargePct < t.BatteryMinPct:
				add(AlertBatteryLow, LevelError, map[string]any{"pct": b.ChargePct})
			case !b.OnAC:
				add(AlertOnBattery, LevelWarn, map[string]any{"pct": b.ChargePct})
			}
			if b.WearPct != nil && *b.WearPct > t.BatteryWearMaxPct {
				add(AlertBatteryWorn, LevelWarn, map[string]any{"pct": *b.WearPct})
			}
		}
	}
	slices.SortStableFunc(out, func(a, b Alert) int {
		return slices.Index(AllAlerts, a.ID) - slices.Index(AllAlerts, b.ID)
	})
	return out
}

// Service is a destination of the self-test as Ajustes keeps it.
type Service struct {
	Name    string `json:"name"`
	Address string `json:"address"` // host:port
}

// Targets are the destinations of Ajustes › Salud.
type Targets struct {
	Internet string    `json:"internet,omitempty"` // host:port; "" not probed
	Services []Service `json:"services,omitempty"`
}

// ParseTarget parses "host:port" (an IPv6 address in brackets).
func ParseTarget(s string) (protocol.HealthTarget, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(s))
	if err != nil {
		return protocol.HealthTarget{}, ErrTarget
	}
	n, err := strconv.Atoi(port)
	t := protocol.HealthTarget{Host: host, Port: n}
	if err != nil || !protocol.ValidHealthHost(host) || n < 1 || n > 65535 {
		return protocol.HealthTarget{}, ErrTarget
	}
	return t, nil
}

// Config turns the targets into the health.config every agent receives.
func (t Targets) Config() (protocol.HealthConfig, error) {
	var c protocol.HealthConfig
	if strings.TrimSpace(t.Internet) != "" {
		it, err := ParseTarget(t.Internet)
		if err != nil {
			return c, err
		}
		c.Internet = &it
	}
	if len(t.Services) > protocol.MaxHealthServices {
		return c, ErrService
	}
	for _, s := range t.Services {
		tg, err := ParseTarget(s.Address)
		if err != nil || !protocol.ValidHealthServiceName(strings.TrimSpace(s.Name)) {
			return c, ErrService
		}
		c.Services = append(c.Services, protocol.HealthService{Name: strings.TrimSpace(s.Name), Host: tg.Host, Port: tg.Port})
	}
	return c, c.Validate()
}

// Console rhythm (docs/protocol.md §5.27.4–5.27.5).
const (
	RequestGap     = 10 * time.Second // «Revisar ahora» at most this often
	ReportMinGap   = 2 * time.Second  // a report closer to the previous is dropped
	SelfTestNoWait = 30 * time.Second // a self-test without answer by then
)

// TestStatus is the state of a computer's last self-test.
type TestStatus string

// Self-test states in the console.
const (
	TestPending     TestStatus = "pending"
	TestDone        TestStatus = "done"
	TestBusy        TestStatus = "busy"
	TestUnsupported TestStatus = "unsupported"
	TestNoAnswer    TestStatus = "noAnswer"
)

// Test is a computer's last self-test.
type Test struct {
	TestID string                 `json:"testId"`
	Status TestStatus             `json:"status"`
	Checks []protocol.HealthCheck `json:"checks,omitempty"`
	Failed int                    `json:"failed"`
	At     time.Time              `json:"at"`
}

// Computer is what the console knows of one computer's health.
type Computer struct {
	Key       string                 `json:"key"`
	Hostname  string                 `json:"hostname"`
	Simulated bool                   `json:"simulated"`
	Connected bool                   `json:"connected"`
	Report    *protocol.HealthReport `json:"report,omitempty"`
	At        time.Time              `json:"at"`
	ClockMS   *int64                 `json:"clockMs,omitempty"`
	Alerts    []Alert                `json:"alerts"`
	Test      *Test                  `json:"test,omitempty"`
}

type computer struct {
	Computer
	skew *time.Duration
}

// Lab is the console's side: the last report of every computer, only in
// memory while the console runs (docs/privacidad.md rule 95).
type Lab struct {
	Logger *slog.Logger
	Now    func() time.Time // nil: time.Now

	mu          sync.Mutex
	th          Thresholds
	computers   map[string]*computer
	lastRequest time.Time
}

// NewLab returns an empty lab with the given thresholds.
func NewLab(th Thresholds, log *slog.Logger) *Lab {
	if log == nil {
		log = slog.Default()
	}
	return &Lab{Logger: log, th: th, computers: map[string]*computer{}}
}

func (l *Lab) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now()
}

// Thresholds returns the thresholds in use.
func (l *Lab) Thresholds() Thresholds {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.th
}

// SetThresholds changes the thresholds and evaluates every computer again.
func (l *Lab) SetThresholds(th Thresholds) error {
	if err := th.Validate(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.th = th
	for _, c := range l.computers {
		l.evaluate(c)
	}
	return nil
}

func (l *Lab) get(key, host string, sim bool) *computer {
	c := l.computers[key]
	if c == nil {
		c = &computer{Computer: Computer{Key: key}}
		l.computers[key] = c
	}
	c.Hostname, c.Simulated = host, sim
	return c
}

// evaluate recomputes c's alerts and logs those that come and go: the
// computer and the alert, never the values.
func (l *Lab) evaluate(c *computer) {
	old := c.Alerts
	c.Alerts = Evaluate(c.Report, c.skew, l.th)
	has := func(list []Alert, id AlertID) bool {
		return slices.ContainsFunc(list, func(a Alert) bool { return a.ID == id })
	}
	for _, a := range c.Alerts {
		if !has(old, a.ID) {
			l.Logger.Info("health alert", "host", c.Hostname, "alert", a.ID, "simulated", c.Simulated)
		}
	}
	for _, a := range old {
		if !has(c.Alerts, a.ID) {
			l.Logger.Info("health alert cleared", "host", c.Hostname, "alert", a.ID, "simulated", c.Simulated)
		}
	}
}

// Report keeps a report from a computer. A report closer than ReportMinGap
// to the previous one is dropped, unless it answers a request. It reports
// whether it was kept.
func (l *Lab) Report(key, host string, sim bool, r protocol.HealthReport) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.get(key, host, sim)
	now := l.now()
	if r.Reason != protocol.HealthOnDemand && c.Report != nil && now.Sub(c.At) < ReportMinGap {
		return false
	}
	c.Report, c.At, c.Connected = &r, now, true
	l.evaluate(c)
	return true
}

// Clock keeps the clock skew of a connected computer, from its heartbeats.
// It reports whether it changed by a second or more (or became known): the
// network jitter of each heartbeat is not news.
func (l *Lab) Clock(key, host string, sim bool, skew time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.get(key, host, sim)
	c.Connected = true
	if c.skew != nil && (*c.skew-skew).Abs() < time.Second {
		return false
	}
	c.skew = &skew
	ms := skew.Milliseconds()
	c.ClockMS = &ms
	l.evaluate(c)
	return true
}

// Reset forgets every computer: another channel is another lab.
func (l *Lab) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.computers = map[string]*computer{}
	l.lastRequest = time.Time{}
}

// SetConnectedOnly marks connected exactly the computers in keys and
// reports whether any changed.
func (l *Lab) SetConnectedOnly(keys map[string]bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	changed := false
	for k, c := range l.computers {
		if c.Connected != keys[k] {
			c.Connected, changed = keys[k], true
		}
	}
	return changed
}

// TryRequest reports whether «Revisar ahora» may go now (RequestGap), and
// if so counts it.
func (l *Lab) TryRequest() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if !l.lastRequest.IsZero() && now.Sub(l.lastRequest) < RequestGap {
		return false
	}
	l.lastRequest = now
	return true
}

// NextRequest is when «Revisar ahora» may go again (zero: now).
func (l *Lab) NextRequest() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lastRequest.IsZero() {
		return time.Time{}
	}
	return l.lastRequest.Add(RequestGap)
}

// Testing reports whether a self-test is pending on any of keys.
func (l *Lab) Testing(keys []string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, k := range keys {
		if c := l.computers[k]; c != nil && c.Test != nil && c.Test.Status == TestPending {
			return true
		}
	}
	return false
}

// StartTest marks a self-test pending on each computer (key → hostname,
// simulated) and returns the test_id for all of them.
func (l *Lab) StartTest(targets map[string]Target) string {
	id := protocol.NewID()
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for k, t := range targets {
		c := l.get(k, t.Hostname, t.Simulated)
		c.Test = &Test{TestID: id, Status: TestPending, At: now}
	}
	return id
}

// Target names a computer for StartTest.
type Target struct {
	Hostname  string
	Simulated bool
}

// TestResult keeps the answer of a computer to its pending self-test; an
// answer to another test_id is ignored.
func (l *Lab) TestResult(key string, res protocol.HealthSelfTestResult) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.computers[key]
	if c == nil || c.Test == nil || c.Test.TestID != res.TestID || c.Test.Status != TestPending {
		return false
	}
	t := &Test{TestID: res.TestID, Status: TestStatus(res.Status), Checks: slices.Clone(res.Checks), At: l.now()}
	for _, ch := range res.Checks {
		if ch.Status == protocol.CheckFail {
			t.Failed++
		}
	}
	c.Test = t
	l.Logger.Info("self-test result", "host", c.Hostname, "test_id", res.TestID, "status", res.Status, "failed", t.Failed)
	return true
}

// Expire marks «sin respuesta» the self-tests pending for SelfTestNoWait.
// It reports whether any changed.
func (l *Lab) Expire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now, changed := l.now(), false
	for _, c := range l.computers {
		if c.Test != nil && c.Test.Status == TestPending && now.Sub(c.Test.At) >= SelfTestNoWait {
			c.Test.Status, changed = TestNoAnswer, true
		}
	}
	return changed
}

// Computers returns a copy of every computer, by hostname.
func (l *Lab) Computers() []Computer {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Computer, 0, len(l.computers))
	for _, c := range l.computers {
		cp := c.Computer
		cp.Alerts = slices.Clone(c.Alerts)
		if cp.Alerts == nil {
			cp.Alerts = []Alert{}
		}
		if c.Test != nil {
			t := *c.Test
			t.Checks = slices.Clone(t.Checks)
			cp.Test = &t
		}
		out = append(out, cp)
	}
	slices.SortFunc(out, func(a, b Computer) int {
		return strings.Compare(strings.ToLower(a.Hostname)+fmt.Sprint(a.Simulated), strings.ToLower(b.Hostname)+fmt.Sprint(b.Simulated))
	})
	return out
}

// Get returns one computer.
func (l *Lab) Get(key string) (Computer, bool) {
	for _, c := range l.Computers() {
		if c.Key == key {
			return c, true
		}
	}
	return Computer{}, false
}
