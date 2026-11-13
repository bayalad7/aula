package main

import (
	"errors"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/power"
	"github.com/bayalad7/aula/internal/session"
)

// --- Turn computers on: Wake-on-LAN (docs/protocol.md §5.15.2) --------------------

// machinesFile is the known computers, next to the console settings.
const machinesFile = "equipos-conocidos.json"

// wakeState is what turning computers on needs: the known computers, who is
// being turned on, the console's cards and the UDP sender (both replaced in
// tests: never a real datagram).
type wakeState struct {
	machines *power.Machines
	waker    *power.Waker
	nics     func() ([]platform.NIC, error)
	send     power.PacketSender
	gap      time.Duration   // between the WakeRounds sends
	recorded map[string]bool // session IDs already recorded; guarded by App.mu
	// channel is the open class's, readable without App.mu (studentsIn runs
	// under it).
	channel *atomic.Value
}

func newWakeState(dir string, log *slog.Logger) wakeState {
	path := ""
	if dir != "" {
		path = filepath.Join(dir, machinesFile)
	}
	m, err := power.LoadMachines(path)
	if err != nil {
		log.Warn("known computers not read; starting empty", "err", err)
	}
	return wakeState{machines: m, waker: power.NewWaker(0), nics: platform.Interfaces, send: power.UDPSender,
		gap: power.WakeGap, recorded: map[string]bool{}, channel: &atomic.Value{}}
}

// wakeChannel is the channel of the open class ("" before the first one).
func (a *App) wakeChannel() string {
	c, _ := a.wake.channel.Load().(string)
	return c
}

// OffMachine is a known computer of the channel with no session in this
// console: off, or on without its agent. «Mostrar apagados» lists them.
type OffMachine struct {
	Hostname string `json:"hostname"`
	// LastSeen is when its agent last said hello (RFC 3339).
	LastSeen string          `json:"lastSeen"`
	Wake     *power.WakeView `json:"wake"`
}

// WakeResult says what «Encender» did.
type WakeResult struct {
	Sent        int      `json:"sent"`
	NoMAC       []string `json:"noMac"`       // never reported a MAC: an older agent
	Failed      []string `json:"failed"`      // no packet could go out
	OtherSubnet []string `json:"otherSubnet"` // the console has no card on their subnet
}

// recordMachines remembers the MACs of the real sessions that just said hello
// and ends the wake of those that were being turned on. Simulated students
// never get here (they are not in class.Sessions() and send no MACs).
func (a *App) recordMachines(class classroom) bool {
	channel := a.wakeChannel()
	changed := false
	for _, s := range class.Sessions() {
		if s.State != session.StateConnected {
			continue
		}
		a.mu.Lock()
		done := a.wake.recorded[s.ID]
		a.wake.recorded[s.ID] = true
		a.mu.Unlock()
		if done {
			continue
		}
		if err := a.wake.machines.Seen(channel, s.Hostname, s.MACs, ipOf(s.RemoteAddr)); err != nil {
			a.log.Warn("known computer not saved", "err", err)
		}
		if a.wake.waker.Arrived(channel, s.Hostname) {
			a.log.Info("computer turned on", "host", s.Hostname)
			changed = true
		}
	}
	return changed
}

// ipOf is the address of "host:port".
func ipOf(remote string) string {
	if h, _, err := net.SplitHostPort(remote); err == nil {
		return h
	}
	return remote
}

// markWake says which students can be turned on, fills the wake state of
// the disconnected ones and lists the known computers with no session.
func (a *App) markWake(channel string, st *Students, sessions []session.Session) {
	present := map[string]bool{}
	for _, s := range sessions {
		present[strings.ToLower(s.Hostname)] = true
	}
	for i := range st.Students {
		s := &st.Students[i]
		if s.Simulated {
			continue
		}
		if m, ok := a.wake.machines.Get(channel, s.Hostname); ok && len(m.MACs) > 0 {
			s.CanWake = true // the card says so; only a disconnected one shows «Encender»
		}
		if s.State == "connected" {
			continue
		}
		if v, ok := a.wake.waker.View(channel, s.Hostname); ok {
			s.Wake = &v
		}
	}
	st.Off = []OffMachine{}
	for _, m := range a.wake.machines.List(channel) {
		if present[strings.ToLower(m.Hostname)] {
			continue
		}
		o := OffMachine{Hostname: m.Hostname, LastSeen: m.LastSeen.Format(time.RFC3339)}
		if v, ok := a.wake.waker.View(channel, m.Hostname); ok {
			o.Wake = &v
		}
		st.Off = append(st.Off, o)
	}
}

// WakeComputers sends the Wake-on-LAN packet to the known computers among
// hosts that are not connected; with hosts empty, to every one of the channel.
// Each then shows «Encendiendo…» until its agent says hello, or «No
// respondió» after power.WakeTimeout.
func (a *App) WakeComputers(hosts []string) (WakeResult, error) {
	class := a.currentClass()
	if class == nil {
		return WakeResult{}, errNoClass
	}
	channel := a.wakeChannel()
	res := WakeResult{NoMAC: []string{}, Failed: []string{}, OtherSubnet: []string{}}
	connected := map[string]bool{}
	asleep := map[string]bool{} // disconnected sessions, which may be unknown
	for _, s := range class.Sessions() {
		if s.State == session.StateConnected {
			connected[strings.ToLower(s.Hostname)] = true
		} else {
			asleep[strings.ToLower(s.Hostname)] = true
		}
	}
	want := map[string]bool{}
	for _, h := range hosts {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" && !connected[h] {
			want[h] = true
		}
	}
	var targets []power.Machine
	seen := map[string]bool{}
	for _, m := range a.wake.machines.List(channel) {
		k := strings.ToLower(m.Hostname)
		if connected[k] || (len(hosts) > 0 && !want[k]) {
			continue
		}
		targets = append(targets, m)
		seen[k] = true
	}
	for h := range want { // asked for, but never reported a MAC
		if !seen[h] && asleep[h] {
			res.NoMAC = append(res.NoMAC, h)
		}
	}
	if len(targets) == 0 {
		if len(res.NoMAC) > 0 {
			return res, codedf(codeWakeNoMAC, "ninguno de esos equipos informó su dirección física: actualiza su agente o enciéndelos a mano")
		}
		return res, codedf(codeNoWakeTargets, "no hay equipos apagados conocidos en este canal")
	}
	if len(targets) > power.MaxWakeMachines {
		targets = targets[:power.MaxWakeMachines]
	}
	nics, err := a.wake.nics()
	if err != nil && !errors.Is(err, platform.ErrNotSupported) {
		a.log.Warn("network cards not read", "err", err)
	}
	plans, noMAC := power.PlanWake(nics, targets)
	res.NoMAC = append(res.NoMAC, noMAC...)
	var sent []power.WakePlan
	for _, p := range plans {
		if err := p.Send(a.wake.send); err != nil {
			a.log.Debug("magic packet not sent", "host", p.Hostname, "err", err)
			res.Failed = append(res.Failed, p.Hostname)
			continue
		}
		a.wake.waker.Start(channel, p.Hostname, p.OtherSubnet)
		if p.OtherSubnet {
			res.OtherSubnet = append(res.OtherSubnet, p.Hostname)
		}
		sent = append(sent, p)
		res.Sent++
	}
	a.log.Info("wake on lan", "sent", res.Sent, "failed", len(res.Failed), "no_mac", len(res.NoMAC), "other_subnet", len(res.OtherSubnet))
	if res.Sent == 0 {
		return res, codedf(codeWakeFailed, "no se pudo mandar la señal de encendido por la red: revisa que la consola esté conectada")
	}
	go a.resendWake(sent)
	a.emit(EventStudents, a.studentsIn(class))
	return res, nil
}

// resendWake sends the plans WakeRounds-1 more times, and refreshes the
// window when «Encendiendo…» runs out.
func (a *App) resendWake(plans []power.WakePlan) {
	for range power.WakeRounds - 1 {
		time.Sleep(a.wake.gap)
		for _, p := range plans {
			_ = p.Send(a.wake.send)
		}
	}
	time.Sleep(a.wake.waker.Timeout() - time.Duration(power.WakeRounds-1)*a.wake.gap)
	if class := a.currentClass(); class != nil {
		a.emit(EventStudents, a.studentsIn(class))
	}
}

// ForgetComputer removes a known computer of the channel («Olvidar equipo»):
// its MAC is deleted from this console.
func (a *App) ForgetComputer(host string) error {
	class := a.currentClass()
	if class == nil {
		return errNoClass
	}
	if _, err := a.wake.machines.Forget(a.wakeChannel(), host); err != nil {
		a.log.Warn("known computer not forgotten", "err", err)
	}
	a.emit(EventStudents, a.studentsIn(class))
	return nil
}
