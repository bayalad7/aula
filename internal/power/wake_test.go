package power

import (
	"bytes"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
)

// These tests never send a real datagram: fakeSender only records.

func nic(name, mac string, cidrs ...string) platform.NIC {
	hw, err := net.ParseMAC(mac)
	if err != nil {
		panic(err)
	}
	n := platform.NIC{Name: name, MAC: hw}
	for _, c := range cidrs {
		ip, ipnet, err := net.ParseCIDR(c)
		if err != nil {
			panic(err)
		}
		n.Addrs = append(n.Addrs, net.IPNet{IP: ip.To4(), Mask: ipnet.Mask})
	}
	return n
}

func TestMagicPacketIsExact(t *testing.T) {
	p, err := MagicPacket("3c:52:82:1a:9f:07")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Repeat("ff", 6) + strings.Repeat("3c52821a9f07", 16)
	if got := hex.EncodeToString(p); got != want {
		t.Fatalf("packet = %s\nwant      %s", got, want)
	}
	if len(p) != 102 {
		t.Fatalf("len = %d, want 102", len(p))
	}
	for _, bad := range []string{"", "3C:52:82:1A:9F:07", "3c-52-82-1a-9f-07", "3c:52:82:1a:9f", "00:00:00:00:00:00:00:01"} {
		if _, err := MagicPacket(bad); !errors.Is(err, ErrBadMAC) {
			t.Errorf("MagicPacket(%q) err = %v, want ErrBadMAC", bad, err)
		}
	}
}

func TestAgentMACsPicksTheCardThatReachesTheConsole(t *testing.T) {
	nics := []platform.NIC{
		nic("Wi-Fi", "aa:aa:aa:aa:aa:01", "10.20.0.5/16"),
		nic("Ethernet", "3C:52:82:1A:9F:07", "192.168.10.7/24"),
	}
	if got := AgentMACs(nics, net.ParseIP("192.168.10.2")); len(got) != 1 || got[0] != "3c:52:82:1a:9f:07" {
		t.Fatalf("same subnet = %v, want the Ethernet MAC", got)
	}
	// Behind a router: every card, so the console may still try.
	if got := AgentMACs(nics, net.ParseIP("172.16.0.1")); len(got) != 2 {
		t.Fatalf("other subnet = %v, want both", got)
	}
	many := []platform.NIC{
		nic("a", "00:00:00:00:00:01", "10.0.1.1/24"), nic("b", "00:00:00:00:00:02", "10.0.2.1/24"),
		nic("c", "00:00:00:00:00:03", "10.0.3.1/24"), nic("d", "00:00:00:00:00:04", "10.0.4.1/24"),
		nic("e", "00:00:00:00:00:05", "10.0.5.1/24"),
	}
	if got := AgentMACs(many, nil); len(got) != 4 {
		t.Fatalf("got %d MACs, want at most 4", len(got))
	}
	if got := AgentMACs(nil, net.ParseIP("10.0.0.1")); len(got) != 0 {
		t.Fatalf("no cards = %v", got)
	}
}

func TestRoutesForPicksTheBroadcastOfTheCardOnTheSubnet(t *testing.T) {
	nics := []platform.NIC{
		nic("Administrativa", "aa:aa:aa:aa:aa:01", "10.0.5.20/24"),
		nic("Laboratorio", "aa:aa:aa:aa:aa:02", "192.168.10.2/23"),
	}
	routes, other := RoutesFor(nics, net.ParseIP("192.168.11.40"))
	if other || len(routes) != 1 {
		t.Fatalf("routes = %v other = %v, want one on the lab card", routes, other)
	}
	if routes[0].From.String() != "192.168.10.2" || routes[0].To.String() != "192.168.11.255" {
		t.Fatalf("route = %v → %v, want 192.168.10.2 → 192.168.11.255", routes[0].From, routes[0].To)
	}

	routes, other = RoutesFor(nics, net.ParseIP("172.16.3.4"))
	if !other || len(routes) != 2 {
		t.Fatalf("other subnet: routes = %v other = %v, want the limited broadcast from each card", routes, other)
	}
	for _, r := range routes {
		if !r.To.Equal(net.IPv4bcast) {
			t.Fatalf("route to %v, want 255.255.255.255", r.To)
		}
	}

	routes, other = RoutesFor(nil, nil)
	if !other || len(routes) != 1 || routes[0].From != nil || !routes[0].To.Equal(net.IPv4bcast) {
		t.Fatalf("no cards: routes = %v other = %v", routes, other)
	}
}

type sent struct {
	from, to string
	packet   []byte
}

type fakeSender struct {
	list []sent
	fail map[string]bool // by "to"
}

func (f *fakeSender) send(from, to net.IP, packet []byte) error {
	if f.fail[to.String()] {
		return errors.New("red caída")
	}
	f.list = append(f.list, sent{from: from.String(), to: to.String(), packet: packet})
	return nil
}

func TestPlanWakeSendsEveryMACToEveryRoute(t *testing.T) {
	nics := []platform.NIC{nic("Ethernet", "aa:aa:aa:aa:aa:01", "192.168.10.2/24")}
	plans, noMAC := PlanWake(nics, []Machine{
		{Hostname: "LAB3-PC07", MACs: []string{"3c:52:82:1a:9f:07", "3c:52:82:1a:9f:08"}, IP: "192.168.10.57"},
		{Hostname: "LAB3-PC08", MACs: []string{"MALA"}, IP: "192.168.10.58"},
	})
	if len(plans) != 1 || len(noMAC) != 1 || noMAC[0] != "LAB3-PC08" {
		t.Fatalf("plans = %d noMAC = %v", len(plans), noMAC)
	}
	f := &fakeSender{}
	if err := plans[0].Send(f.send); err != nil {
		t.Fatal(err)
	}
	if len(f.list) != 2 {
		t.Fatalf("sent %d packets, want 2", len(f.list))
	}
	want, _ := MagicPacket("3c:52:82:1a:9f:08")
	if f.list[1].to != "192.168.10.255" || f.list[1].from != "192.168.10.2" || !bytes.Equal(f.list[1].packet, want) {
		t.Fatalf("second packet = %+v", f.list[1])
	}
}

func TestWakePlanFailsOnlyWhenNothingWentOut(t *testing.T) {
	nics := []platform.NIC{
		nic("A", "aa:aa:aa:aa:aa:01", "10.0.1.2/24"),
		nic("B", "aa:aa:aa:aa:aa:02", "10.0.2.2/24"),
	}
	plans, _ := PlanWake(nics, []Machine{{Hostname: "PC", MACs: []string{"3c:52:82:1a:9f:07"}, IP: "172.16.0.9"}})
	if !plans[0].OtherSubnet {
		t.Fatal("want other subnet")
	}
	f := &fakeSender{fail: map[string]bool{"255.255.255.255": true}}
	if err := plans[0].Send(f.send); err == nil {
		t.Fatal("every send failed: want an error")
	}
	plans, _ = PlanWake(nics, []Machine{{Hostname: "PC", MACs: []string{"3c:52:82:1a:9f:07"}, IP: "10.0.1.9"}})
	f = &fakeSender{fail: map[string]bool{"10.0.2.255": true}}
	if err := plans[0].Send(f.send); err != nil {
		t.Fatalf("the route on the subnet worked: err = %v", err)
	}
}

func TestMachinesRecordAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consola", "equipos-conocidos.json")
	m, err := LoadMachines(path)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return clock }

	// Never seen with a MAC: not recorded (it could not be woken).
	if err := m.Seen("1", "LAB3-PC01", nil, "192.168.10.51"); err != nil {
		t.Fatal(err)
	}
	if len(m.List("1")) != 0 {
		t.Fatal("a computer without MAC was recorded")
	}
	if err := m.Seen("1", "LAB3-PC07", []string{"3C:52:82:1A:9F:07", "basura"}, "192.168.10.57"); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Hour)
	// An older agent says hello without MACs: the known one stays, the IP updates.
	if err := m.Seen("1", "lab3-pc07", nil, "192.168.10.70"); err != nil {
		t.Fatal(err)
	}
	if err := m.Seen("2", "LAB3-PC07", []string{"00:00:00:00:00:02"}, "10.0.0.2"); err != nil {
		t.Fatal(err)
	}

	again, err := LoadMachines(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := again.Get("1", "LAB3-PC07")
	if !ok || len(got.MACs) != 1 || got.MACs[0] != "3c:52:82:1a:9f:07" || got.IP != "192.168.10.70" || !got.LastSeen.Equal(clock) {
		t.Fatalf("reloaded = %+v %v", got, ok)
	}
	if l := again.List("2"); len(l) != 1 || l[0].MACs[0] != "00:00:00:00:00:02" {
		t.Fatalf("channel 2 = %+v", l)
	}

	if ok, err := again.Forget("1", "Lab3-PC07"); !ok || err != nil {
		t.Fatalf("Forget = %v %v", ok, err)
	}
	third, _ := LoadMachines(path)
	if len(third.List("1")) != 0 || len(third.List("2")) != 1 {
		t.Fatal("forget did not reach the file, or removed the other channel")
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "username") {
		t.Fatal("the file must not keep who used the computer")
	}
}

func TestMachinesDropTheOldestPastTheLimit(t *testing.T) {
	m, _ := LoadMachines("")
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range MaxKnownMachines + 1 {
		m.now = func() time.Time { return start.Add(time.Duration(i) * time.Minute) }
		mac := net.HardwareAddr{0, 0, 0, 0, byte(i >> 8), byte(i)}
		if err := m.Seen("1", "PC"+hex.EncodeToString(mac), []string{MACString(mac)}, ""); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(m.List("1")); n != MaxKnownMachines {
		t.Fatalf("known = %d, want %d", n, MaxKnownMachines)
	}
	if _, ok := m.Get("1", "PC000000000000"); ok {
		t.Fatal("the oldest should have been dropped")
	}
}

func TestLoadMachinesSkipsBadEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "equipos.json")
	data := `{"version":1,"channels":{"1":[{"hostname":"BIEN","macs":["3c:52:82:1a:9f:07"]},{"hostname":"","macs":["3c:52:82:1a:9f:08"]},{"hostname":"SIN","macs":["x"]}],"canal-malo":[{"hostname":"X","macs":["3c:52:82:1a:9f:09"]}]}}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := LoadMachines(path)
	if err != nil {
		t.Fatal(err)
	}
	if l := m.List("1"); len(l) != 1 || l[0].Hostname != "BIEN" {
		t.Fatalf("list = %+v", l)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if m, err := LoadMachines(path); err == nil || len(m.List("1")) != 0 {
		t.Fatal("a broken file must report an error and start empty")
	}
}

func TestWakerWakingUntilHelloOrTimeout(t *testing.T) {
	w := NewWaker(0)
	if w.Timeout() != WakeTimeout {
		t.Fatalf("timeout = %v", w.Timeout())
	}
	clock := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	w.now = func() time.Time { return clock }
	w.Start("1", "LAB3-PC07", false)
	w.Start("1", "LAB3-PC08", true)

	v, ok := w.View("1", "lab3-pc07")
	if !ok || v.State != WakeWaking || v.DeadlineMS != clock.Add(WakeTimeout).UnixMilli() {
		t.Fatalf("view = %+v %v", v, ok)
	}
	if _, ok := w.View("2", "LAB3-PC07"); ok {
		t.Fatal("another channel")
	}
	if !w.Arrived("1", "LAB3-PC07") || w.Arrived("1", "LAB3-PC07") {
		t.Fatal("Arrived should end the wake once")
	}
	if _, ok := w.View("1", "LAB3-PC07"); ok {
		t.Fatal("arrived: no state")
	}

	clock = clock.Add(WakeTimeout - time.Second)
	if v, _ := w.View("1", "LAB3-PC08"); v.State != WakeWaking || !v.OtherSubnet {
		t.Fatalf("before the timeout = %+v", v)
	}
	clock = clock.Add(time.Second)
	if v, _ := w.View("1", "LAB3-PC08"); v.State != WakeNoResponse {
		t.Fatalf("at the timeout = %+v, want noresponse", v)
	}
	// Turning it on again restarts the wait.
	w.Start("1", "LAB3-PC08", false)
	if v, _ := w.View("1", "LAB3-PC08"); v.State != WakeWaking {
		t.Fatalf("restarted = %+v", v)
	}
}
