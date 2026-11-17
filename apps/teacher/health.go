package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/health"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// The health of the lab (prompt 10.25, docs/protocol.md §5.27): the
// Laboratorio view, «Salud del equipo» in the student's file, the badge in
// the list and Ajustes › Salud. Only in memory; it changes no computer.

// healthFile keeps Ajustes › Salud next to the console's settings.
const healthFile = "salud.json"

// healthEvery is how often the clocks and the self-tests are looked at.
const healthEvery = 5 * time.Second

// HealthSettings is Ajustes › Salud: the thresholds and the destinations.
type HealthSettings struct {
	Thresholds health.Thresholds `json:"thresholds"`
	Targets    health.Targets    `json:"targets"`
}

// LabView is what the Laboratorio view shows.
type LabView struct {
	Computers  []health.Computer `json:"computers"`
	WithAlerts int               `json:"withAlerts"` // connected computers with an alert
	Connected  int               `json:"connected"`
	// RequestInMs is how long until «Revisar ahora» may go again (0: now).
	RequestInMs int64 `json:"requestInMs"`
	// HasTargets says whether Ajustes › Salud has destinations (the
	// self-test then also tries them).
	HasTargets bool `json:"hasTargets"`
}

// healthState is the console's side, besides health.Lab.
type healthState struct {
	mu         sync.Mutex
	configured map[string]bool // sessions that got this health.config
	config     protocol.HealthConfig
	expiry     *time.Timer
}

func defaultHealthSettings() HealthSettings {
	return HealthSettings{Thresholds: health.DefaultThresholds()}
}

func (a *App) loadHealthSettings() HealthSettings {
	s := defaultHealthSettings()
	data, err := os.ReadFile(filepath.Join(a.dir, healthFile))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			a.log.Warn("cannot read health settings", "err", err)
		}
		return s
	}
	var got HealthSettings
	if json.Unmarshal(data, &got) != nil || got.Thresholds.Validate() != nil {
		a.log.Warn("health settings damaged, using the defaults")
		return s
	}
	if _, err := got.Targets.Config(); err != nil {
		got.Targets = health.Targets{}
	}
	return got
}

// initHealth builds the lab with the saved settings.
func (a *App) initHealth() {
	s := a.loadHealthSettings()
	a.lab = health.NewLab(s.Thresholds, a.log)
	a.lab.Now = func() time.Time { return a.now() }
	cfg, _ := s.Targets.Config()
	a.healthSt = &healthState{configured: map[string]bool{}, config: cfg}
}

// healthLoop refreshes the clocks of the computers and expires the
// self-tests every healthEvery while ctx lives.
func (a *App) healthLoop(ctx context.Context) {
	t := time.NewTicker(healthEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if class := a.currentClass(); class != nil {
				a.refreshHealth(class)
			}
		}
	}
}

// labKey is how the console names a computer, as the window does
// (claveEquipo in main.js).
func labKey(hostname string, simulated bool) string {
	if simulated {
		return "s:" + strings.ToLower(hostname)
	}
	return "r:" + strings.ToLower(hostname)
}

// labSessions are the connected sessions of the class, real and simulated.
func labSessions(class classroom) (all []session.Session, sim map[string]bool) {
	sim = map[string]bool{}
	for _, s := range class.Sessions() {
		if s.State == session.StateConnected {
			all = append(all, s)
		}
	}
	for _, s := range simSessionsOf(class) {
		if s.State == session.StateConnected {
			all = append(all, s)
			sim[s.ID] = true
		}
	}
	return all, sim
}

// syncHealth sends health.config to who just connected with health, and
// marks who is connected (onChange).
func (a *App) syncHealth(class classroom) {
	sessions, sim := labSessions(class)
	st := a.healthSt
	st.mu.Lock()
	cfg := st.config
	present := map[string]bool{}
	var send []string
	for _, s := range sessions {
		present[s.ID] = true
		if s.Can(protocol.CapHealth) && !st.configured[s.ID] {
			st.configured[s.ID] = true
			send = append(send, s.ID)
		}
	}
	for id := range st.configured {
		if !present[id] {
			delete(st.configured, id)
		}
	}
	st.mu.Unlock()
	ctx := context.Background()
	for _, id := range send {
		if _, err := class.Send(ctx, id, cfg); err != nil {
			a.log.Debug("health.config not sent", "session", id, "err", err)
		}
	}
	keys := map[string]bool{}
	for _, s := range sessions {
		keys[labKey(s.Hostname, sim[s.ID])] = true
	}
	if a.lab.SetConnectedOnly(keys) {
		a.emit(EventLab, a.Lab())
	}
}

// refreshHealth takes the clocks from the heartbeats and expires the
// self-tests; it tells the window if anything changed.
func (a *App) refreshHealth(class classroom) {
	sessions, sim := labSessions(class)
	before := a.alertLevels()
	changed := false
	for _, s := range sessions {
		if s.ClockSamples > 0 && a.lab.Clock(labKey(s.Hostname, sim[s.ID]), s.Hostname, sim[s.ID], s.ClockSkew) {
			changed = true
		}
	}
	if a.lab.Expire() {
		changed = true
	}
	if changed {
		a.emitHealth(class, before)
	}
}

// alertLevels is each computer's alert count and level, to know when the
// list's badges change.
func (a *App) alertLevels() map[string]string {
	out := map[string]string{}
	for _, c := range a.lab.Computers() {
		n, lvl := alertSummary(c.Alerts)
		out[c.Key] = lvl + string(rune('0'+min(n, 9)))
	}
	return out
}

func alertSummary(as []health.Alert) (int, string) {
	lvl := ""
	for _, al := range as {
		if al.Level == health.LevelError {
			lvl = string(health.LevelError)
		} else if lvl == "" {
			lvl = string(health.LevelWarn)
		}
	}
	return len(as), lvl
}

// emitHealth tells the window: the lab always, the list when a badge
// changed.
func (a *App) emitHealth(class classroom, before map[string]string) {
	a.emit(EventLab, a.Lab())
	after := a.alertLevels()
	for k, v := range after {
		if before[k] != v {
			a.emit(EventStudents, a.studentsIn(class))
			return
		}
	}
	if len(before) != len(after) {
		a.emit(EventStudents, a.studentsIn(class))
	}
}

// onHealthMessage receives health.report and health.selftest.result, from
// real and simulated students.
func (a *App) onHealthMessage(sessionID string, env protocol.Envelope) {
	class := a.currentClass()
	if class == nil {
		return
	}
	var who *session.Session
	sim := false
	for _, s := range class.Sessions() {
		if s.ID == sessionID {
			who = &s
		}
	}
	for _, s := range simSessionsOf(class) {
		if s.ID == sessionID {
			who, sim = &s, true
		}
	}
	if who == nil {
		return
	}
	key := labKey(who.Hostname, sim)
	before := a.alertLevels()
	switch p := env.Payload.(type) {
	case protocol.HealthReport:
		if !a.lab.Report(key, who.Hostname, sim, p) {
			return
		}
	case protocol.HealthSelfTestResult:
		if !a.lab.TestResult(key, p) {
			return
		}
	default:
		return
	}
	a.emitHealth(class, before)
}

// markHealth sets each student's health badge (the count and the worst
// level of its alerts) in the list.
func (a *App) markHealth(list []Student) {
	byKey := map[string]health.Computer{}
	for _, c := range a.lab.Computers() {
		byKey[c.Key] = c
	}
	for i := range list {
		s := &list[i]
		if c, ok := byKey[labKey(s.Hostname, s.Simulated)]; ok {
			s.HealthAlerts, s.HealthLevel = alertSummary(c.Alerts)
		}
	}
}

// --- Bindings ------------------------------------------------------------------

// Lab returns the Laboratorio view: every computer the console heard of in
// this class, its alerts and its last self-test.
func (a *App) Lab() LabView {
	v := LabView{Computers: a.lab.Computers()}
	for _, c := range v.Computers {
		if c.Connected {
			v.Connected++
			if len(c.Alerts) > 0 {
				v.WithAlerts++
			}
		}
	}
	if next := a.lab.NextRequest(); !next.IsZero() {
		v.RequestInMs = max(0, next.Sub(a.now()).Milliseconds())
	}
	a.healthSt.mu.Lock()
	v.HasTargets = a.healthSt.config.Internet != nil || len(a.healthSt.config.Services) > 0
	a.healthSt.mu.Unlock()
	return v
}

// healthTargets resolves the sessions to send to: the given IDs, or every
// connected one with capability c when none is given.
func (a *App) healthTargets(class classroom, ids []string, c protocol.Capability) (real, sims []session.Session) {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	sessions, sim := labSessions(class)
	for _, s := range sessions {
		if !s.Can(c) || len(want) > 0 && !want[s.ID] {
			continue
		}
		if sim[s.ID] {
			sims = append(sims, s)
		} else {
			real = append(real, s)
		}
	}
	return real, sims
}

// CheckHealth is «Revisar ahora»: it asks the selection (none: everybody)
// for a report now. At most once every 10 s. It returns how many were
// asked.
func (a *App) CheckHealth(targets []string) (int, error) {
	class := a.currentClass()
	if class == nil {
		return 0, errNoClass
	}
	real, sims := a.healthTargets(class, targets, protocol.CapHealth)
	if len(real)+len(sims) == 0 {
		return 0, codedf(codeHealthNoTargets, "ningún equipo conectado informa su salud")
	}
	if !a.lab.TryRequest() {
		return 0, codedf(codeHealthTooSoon, "se revisó hace menos de 10 s")
	}
	n := 0
	for _, s := range append(real, sims...) {
		if _, err := class.Send(context.Background(), s.ID, protocol.HealthRequest{}); err == nil {
			n++
		}
	}
	a.log.Info("health requested", "computers", len(real), "simulated", len(sims))
	a.emit(EventLab, a.Lab())
	return n, nil
}

// RunSelfTest is «Autodiagnóstico»: it asks the selection (none: everybody
// that can) to run its self-test. Not while one is pending on them.
func (a *App) RunSelfTest(targets []string) (int, error) {
	class := a.currentClass()
	if class == nil {
		return 0, errNoClass
	}
	real, sims := a.healthTargets(class, targets, protocol.CapHealthSelfTest)
	all := append(real, sims...)
	if len(all) == 0 {
		return 0, codedf(codeHealthNoTargets, "ningún equipo conectado puede hacer el autodiagnóstico")
	}
	keys, tg := []string{}, map[string]health.Target{}
	for _, s := range all {
		sim := false
		for _, x := range sims {
			if x.ID == s.ID {
				sim = true
			}
		}
		k := labKey(s.Hostname, sim)
		keys = append(keys, k)
		tg[k] = health.Target{Hostname: s.Hostname, Simulated: sim}
	}
	if a.lab.Testing(keys) {
		return 0, codedf(codeHealthBusy, "ya hay un autodiagnóstico en curso en esos equipos")
	}
	id := a.lab.StartTest(tg)
	n := 0
	for _, s := range all {
		if _, err := class.Send(context.Background(), s.ID, protocol.HealthSelfTest{TestID: id}); err == nil {
			n++
		}
	}
	a.log.Info("self-test requested", "test_id", id, "computers", len(real), "simulated", len(sims))
	a.healthSt.mu.Lock()
	if a.healthSt.expiry != nil {
		a.healthSt.expiry.Stop()
	}
	a.healthSt.expiry = time.AfterFunc(health.SelfTestNoWait, func() {
		if a.lab.Expire() {
			a.emit(EventLab, a.Lab())
		}
	})
	a.healthSt.mu.Unlock()
	a.emit(EventLab, a.Lab())
	return n, nil
}

// HealthSettings returns Ajustes › Salud.
func (a *App) HealthSettings() HealthSettings { return a.loadHealthSettings() }

// DefaultHealthSettings returns the thresholds by default and no
// destinations, for «Restaurar».
func (a *App) DefaultHealthSettings() HealthSettings { return defaultHealthSettings() }

// SaveHealthSettings checks and saves Ajustes › Salud, evaluates every
// computer again and sends the new destinations to the class.
func (a *App) SaveHealthSettings(s HealthSettings) (HealthSettings, error) {
	cur := a.loadHealthSettings()
	if err := s.Thresholds.Validate(); err != nil {
		return cur, coded(codeHealthThreshold, err)
	}
	cfg, err := s.Targets.Config()
	if err != nil {
		return cur, coded(codeHealthTarget, err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err == nil {
		if err = os.MkdirAll(a.dir, 0o700); err == nil {
			err = os.WriteFile(filepath.Join(a.dir, healthFile), data, 0o600)
		}
	}
	if err != nil {
		return cur, coded(codeHealthNotSaved, err)
	}
	_ = a.lab.SetThresholds(s.Thresholds)
	a.healthSt.mu.Lock()
	changedTargets := !sameConfig(a.healthSt.config, cfg)
	a.healthSt.config = cfg
	if changedTargets {
		a.healthSt.configured = map[string]bool{} // everybody gets the new ones
	}
	a.healthSt.mu.Unlock()
	if class := a.currentClass(); class != nil {
		before := a.alertLevels()
		a.syncHealth(class)
		a.emitHealth(class, before)
	}
	a.log.Info("health settings saved", "services", len(cfg.Services), "internet", cfg.Internet != nil)
	return s, nil
}

func sameConfig(a, b protocol.HealthConfig) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// resetHealth forgets the lab of the previous channel.
func (a *App) resetHealth() {
	a.lab.Reset()
	a.healthSt.mu.Lock()
	a.healthSt.configured = map[string]bool{}
	a.healthSt.mu.Unlock()
	a.emit(EventLab, a.Lab())
}
