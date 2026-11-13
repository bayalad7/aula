package main

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/power"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// packetLog stands in for power.UDPSender: it records, never sends.
type packetLog struct {
	mu   sync.Mutex
	list []string // "to:hex mac"
	fail bool
}

func (p *packetLog) send(_, to net.IP, packet []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fail {
		return errors.New("sin red")
	}
	p.list = append(p.list, to.String()+" "+net.HardwareAddr(packet[6:12]).String())
	return nil
}

func (p *packetLog) all() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.list)
}

func withMAC(s session.Session, ip string, macs ...string) session.Session {
	s.MACs, s.RemoteAddr = macs, ip+":50123"
	return s
}

func consoleNIC(cidr string) platform.NIC {
	ip, n, _ := net.ParseCIDR(cidr)
	return platform.NIC{Name: "Ethernet", MAC: net.HardwareAddr{2, 0, 0, 0, 0, 1}, Addrs: []net.IPNet{{IP: ip.To4(), Mask: n.Mask}}}
}

func TestWakeRemembersComputersAndTurnsThemOn(t *testing.T) {
	h := newHarness(t,
		withMAC(student("s1", "LAB3-PC07", protocol.CapPower), "192.168.10.57", "3c:52:82:1a:9f:07"),
		withMAC(student("s2", "LAB3-PC08", protocol.CapPower), "192.168.10.58", "3c:52:82:1a:9f:08"),
		student("s3", "VIEJO", protocol.CapPower)) // an older agent: no macs
	h.app.wake.nics = func() ([]platform.NIC, error) { return []platform.NIC{consoleNIC("192.168.10.2/24")}, nil }
	h.hooks.onChange() // they said hello

	// Nobody is off: nothing to turn on.
	if _, err := h.app.WakeComputers(nil); errorCode(err) != codeNoWakeTargets {
		t.Fatalf("err = %v, want %s", err, codeNoWakeTargets)
	}

	// PC07 and the old one leave; PC08 is gone from this console (restarted).
	h.class.mu.Lock()
	h.class.sessions[0].State = session.StateDisconnected
	h.class.sessions[2].State = session.StateDisconnected
	h.class.sessions = slices.Delete(h.class.sessions, 1, 2)
	h.class.mu.Unlock()
	st := h.app.View().Students
	if !st.Students[0].CanWake || st.Students[1].CanWake {
		t.Fatalf("canWake = %v %v, want PC07 only", st.Students[0].CanWake, st.Students[1].CanWake)
	}
	if st.Students[0].Wake != nil {
		t.Fatal("nobody asked to turn it on")
	}
	if len(st.Off) != 1 || st.Off[0].Hostname != "LAB3-PC08" || st.Off[0].LastSeen == "" {
		t.Fatalf("off = %+v, want LAB3-PC08", st.Off)
	}

	// Only the old one: it never reported a MAC.
	if r, err := h.app.WakeComputers([]string{"viejo"}); errorCode(err) != codeWakeNoMAC || !slices.Equal(r.NoMAC, []string{"viejo"}) {
		t.Fatalf("old agent: %+v %v", r, err)
	}

	res, err := h.app.WakeComputers(nil)
	if err != nil || res.Sent != 2 || len(res.OtherSubnet) != 0 {
		t.Fatalf("wake all: %+v %v", res, err)
	}
	waitPackets(t, h.packets, 2*power.WakeRounds)
	if got := h.packets.all()[:2]; !slices.Equal(got, []string{"192.168.10.255 3c:52:82:1a:9f:07", "192.168.10.255 3c:52:82:1a:9f:08"}) {
		t.Fatalf("packets = %v", got)
	}
	st = h.rec.last(EventStudents).(Students)
	if w := st.Students[0].Wake; w == nil || w.State != power.WakeWaking {
		t.Fatalf("PC07 wake = %+v", w)
	}
	if w := st.Off[0].Wake; w == nil || w.State != power.WakeWaking {
		t.Fatalf("PC08 wake = %+v", w)
	}

	// PC08 wakes up and says hello: it is a student again, not «Encendiendo…».
	h.class.mu.Lock()
	h.class.sessions = append(h.class.sessions, withMAC(student("s4", "LAB3-PC08", protocol.CapPower), "192.168.10.58", "3c:52:82:1a:9f:08"))
	h.class.mu.Unlock()
	h.hooks.onChange()
	st = h.rec.last(EventStudents).(Students)
	if len(st.Off) != 0 || st.Students[2].Wake != nil {
		t.Fatalf("after hello: off = %+v wake = %+v", st.Off, st.Students[2].Wake)
	}
}

func TestWakeFromAnotherSubnetAndWithoutNetwork(t *testing.T) {
	h := newHarness(t, withMAC(student("s1", "LAB3-PC07"), "172.16.0.9", "3c:52:82:1a:9f:07"))
	h.hooks.onChange()
	h.class.mu.Lock()
	h.class.sessions[0].State = session.StateDisconnected
	h.class.mu.Unlock()

	h.app.wake.nics = func() ([]platform.NIC, error) { return []platform.NIC{consoleNIC("192.168.10.2/24")}, nil }
	res, err := h.app.WakeComputers([]string{"LAB3-PC07"})
	if err != nil || !slices.Equal(res.OtherSubnet, []string{"LAB3-PC07"}) {
		t.Fatalf("other subnet: %+v %v", res, err)
	}
	if v := h.app.View().Students.Students[0].Wake; v == nil || !v.OtherSubnet {
		t.Fatalf("wake = %+v, want other subnet", v)
	}

	h.packets.mu.Lock()
	h.packets.fail = true
	h.packets.mu.Unlock()
	if res, err := h.app.WakeComputers(nil); errorCode(err) != codeWakeFailed || !slices.Equal(res.Failed, []string{"LAB3-PC07"}) {
		t.Fatalf("no network: %+v %v", res, err)
	}
}

func TestWakeTimesOutAsNoResponse(t *testing.T) {
	h := newHarness(t, withMAC(student("s1", "LAB3-PC07"), "192.168.10.57", "3c:52:82:1a:9f:07"))
	h.app.wake.waker = power.NewWaker(30 * time.Millisecond)
	h.hooks.onChange()
	h.class.mu.Lock()
	h.class.sessions[0].State = session.StateDisconnected
	h.class.mu.Unlock()
	if _, err := h.app.WakeComputers(nil); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if st, ok := h.rec.last(EventStudents).(Students); ok && st.Students[0].Wake != nil && st.Students[0].Wake.State == power.WakeNoResponse {
			return // the window was told when it ran out
		}
		if time.Now().After(deadline) {
			t.Fatal("no event with «No respondió»")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A simulated student never reaches the record: they live in their own
// sessions (simSessionsOf), not in class.Sessions(), and send no macs
// (internal/simclass TestSimulatedHelloHasNoMACs).
func TestOnlyRealSessionsAreRemembered(t *testing.T) {
	h := newHarness(t, withMAC(student("s1", "LAB3-PC07"), "192.168.10.57", "3c:52:82:1a:9f:07"))
	h.hooks.onChange()
	if l := h.app.wake.machines.List(h.app.wakeChannel()); len(l) != 1 || l[0].Hostname != "LAB3-PC07" {
		t.Fatalf("known = %+v", l)
	}
}

func TestForgetComputerRemovesItsMAC(t *testing.T) {
	h := newHarness(t, withMAC(student("s1", "LAB3-PC07"), "192.168.10.57", "3c:52:82:1a:9f:07"))
	h.hooks.onChange()
	path := filepath.Join(h.app.dir, machinesFile)
	if data, err := os.ReadFile(path); err != nil || !strings.Contains(string(data), "3c:52:82:1a:9f:07") {
		t.Fatalf("file = %q %v", data, err)
	}
	if err := h.app.ForgetComputer("lab3-pc07"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); strings.Contains(string(data), "3c:52:82:1a:9f:07") {
		t.Fatal("the MAC is still in the file")
	}
}

func waitPackets(t *testing.T, p *packetLog, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for len(p.all()) < n {
		if time.Now().After(deadline) {
			t.Fatalf("packets = %d, want %d", len(p.all()), n)
		}
		time.Sleep(time.Millisecond)
	}
}
