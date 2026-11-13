package power

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// Wake-on-LAN (docs/protocol.md §5.15.2): the console sends the magic packet
// of each known MAC to the broadcast address of its network card that reaches
// the computer. Nothing else ever goes out over UDP.
const (
	// WakePort is where the magic packet goes ("discard"), the port network
	// cards listen on by convention.
	WakePort = 9
	// MaxWakeMachines bounds one order.
	MaxWakeMachines = 64
	// WakeRounds is how many times each packet is sent, WakeGap apart: one
	// datagram may be lost and nothing acknowledges it.
	WakeRounds = 3
	// magicSize is 6 × FF and 16 × the MAC.
	magicSize = 6 + 16*6
)

// ErrBadMAC means a MAC is not 6 bytes "aa:bb:cc:dd:ee:ff".
var ErrBadMAC = errors.New("power: malformed mac")

// MagicPacket returns the Wake-on-LAN packet for mac: 6 bytes FF followed by
// the MAC 16 times (102 bytes).
func MagicPacket(mac string) ([]byte, error) {
	if !protocol.ValidMAC(mac) {
		return nil, ErrBadMAC
	}
	hw, err := net.ParseMAC(mac)
	if err != nil || len(hw) != 6 {
		return nil, ErrBadMAC
	}
	out := make([]byte, 0, magicSize)
	for range 6 {
		out = append(out, 0xff)
	}
	for range 16 {
		out = append(out, hw...)
	}
	return out, nil
}

// MACString writes hw as hello.macs carries it.
func MACString(hw net.HardwareAddr) string { return strings.ToLower(hw.String()) }

// AgentMACs picks the MACs the agent reports in hello: the card whose subnet
// contains the console's address; if none does (the console is behind a
// router), every card with IPv4, up to protocol.MaxHelloMACs.
func AgentMACs(nics []platform.NIC, console net.IP) []string {
	var near, all []string
	for _, n := range nics {
		if len(n.MAC) != 6 {
			continue
		}
		mac := MACString(n.MAC)
		if !slices.Contains(all, mac) {
			all = append(all, mac)
		}
		for _, a := range n.Addrs {
			if console != nil && a.Contains(console) && !slices.Contains(near, mac) {
				near = append(near, mac)
			}
		}
	}
	out := near
	if len(out) == 0 {
		out = all
	}
	if len(out) > protocol.MaxHelloMACs {
		out = out[:protocol.MaxHelloMACs]
	}
	return out
}

// Route is where one magic packet goes: from a local address (nil: any) to a
// broadcast address.
type Route struct {
	From net.IP
	To   net.IP
}

// RoutesFor returns where to send the packets for a computer last seen at ip:
// the broadcast address of every console card whose subnet contains ip. When
// none does, otherSubnet is true and the routes are the limited broadcast
// (255.255.255.255) from every card: it only reaches the console's own
// network, so the computer probably will not wake.
func RoutesFor(nics []platform.NIC, ip net.IP) (routes []Route, otherSubnet bool) {
	ip4 := ip.To4()
	for _, n := range nics {
		for _, a := range n.Addrs {
			if ip4 != nil && a.Contains(ip4) {
				routes = append(routes, Route{From: a.IP.To4(), To: broadcastOf(a)})
			}
		}
	}
	if len(routes) > 0 {
		return routes, false
	}
	for _, n := range nics {
		for _, a := range n.Addrs {
			routes = append(routes, Route{From: a.IP.To4(), To: net.IPv4bcast.To4()})
		}
	}
	if len(routes) == 0 {
		routes = []Route{{To: net.IPv4bcast.To4()}}
	}
	return routes, true
}

// broadcastOf returns the directed broadcast address of an IPv4 network.
func broadcastOf(n net.IPNet) net.IP {
	ip := n.IP.To4()
	mask := n.Mask
	if len(mask) == net.IPv6len {
		mask = mask[12:]
	}
	out := make(net.IP, net.IPv4len)
	for i := range out {
		out[i] = ip[i] | ^mask[i]
	}
	return out
}

// PacketSender sends one datagram to to:WakePort from the local address from (nil:
// any). Tests use a fake; the console uses UDPSender.
type PacketSender func(from, to net.IP, packet []byte) error

// UDPSender sends packet over UDP. Go enables SO_BROADCAST on UDP sockets, so
// a broadcast address works without privileges.
func UDPSender(from, to net.IP, packet []byte) error {
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: from})
	if err != nil {
		return fmt.Errorf("open udp socket: %w", err)
	}
	defer c.Close()
	if _, err := c.WriteToUDP(packet, &net.UDPAddr{IP: to, Port: WakePort}); err != nil {
		return fmt.Errorf("send magic packet to %s: %w", to, err)
	}
	return nil
}

// WakePlan is what waking one computer sends.
type WakePlan struct {
	Hostname    string
	OtherSubnet bool
	sends       []planned
}

type planned struct {
	route  Route
	packet []byte
}

// PlanWake prepares the packets for each machine: every MAC to every route.
// Machines without a valid MAC are left out and returned in noMAC.
func PlanWake(nics []platform.NIC, machines []Machine) (plans []WakePlan, noMAC []string) {
	for _, m := range machines {
		var packets [][]byte
		for _, mac := range m.MACs {
			if p, err := MagicPacket(mac); err == nil {
				packets = append(packets, p)
			}
		}
		if len(packets) == 0 {
			noMAC = append(noMAC, m.Hostname)
			continue
		}
		routes, other := RoutesFor(nics, net.ParseIP(m.IP))
		plan := WakePlan{Hostname: m.Hostname, OtherSubnet: other}
		for _, r := range routes {
			for _, p := range packets {
				plan.sends = append(plan.sends, planned{route: r, packet: p})
			}
		}
		plans = append(plans, plan)
	}
	return plans, noMAC
}

// Send sends the plan once. It succeeds when at least one packet went out: a
// card that is down fails, the other routes may still reach.
func (p WakePlan) Send(send PacketSender) error {
	var errs []error
	for _, s := range p.sends {
		if err := send(s.route.From, s.route.To, s.packet); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == len(p.sends) {
		return errors.Join(errs...)
	}
	return nil
}
