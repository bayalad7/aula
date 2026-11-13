package platform

import "net"

// NIC is one network card of this computer, as Wake-on-LAN needs it
// (docs/protocol.md §5.15.2): the agent reports the MAC of the card that
// reaches the console, and the console picks the broadcast address of the
// card that reaches the student's subnet.
type NIC struct {
	Name string
	// MAC is the 6-byte hardware address; cards without one (loopback,
	// tunnels) are not listed.
	MAC net.HardwareAddr
	// Addrs are the IPv4 addresses with their masks. Cards without IPv4 are
	// not listed.
	Addrs []net.IPNet
}

// Interfaces returns the network cards that are up, have a 6-byte MAC and an
// IPv4 address, loopback excluded. Windows today; elsewhere ErrNotSupported.
func Interfaces() ([]NIC, error) { return interfaces() }

// nicsFrom keeps the cards Interfaces reports. Split from interfaces so the
// filter is tested on every OS.
func nicsFrom(ifs []net.Interface, addrs func(net.Interface) ([]net.Addr, error)) []NIC {
	var out []NIC
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 || len(ifc.HardwareAddr) != 6 {
			continue
		}
		list, err := addrs(ifc)
		if err != nil {
			continue
		}
		var v4 []net.IPNet
		for _, a := range list {
			n, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := n.IP.To4()
			if ip == nil || ip.IsLinkLocalUnicast() || len(n.Mask) != net.IPv4len && len(n.Mask) != net.IPv6len {
				continue
			}
			mask := n.Mask
			if len(mask) == net.IPv6len {
				mask = mask[12:]
			}
			v4 = append(v4, net.IPNet{IP: ip, Mask: mask})
		}
		if len(v4) == 0 {
			continue
		}
		out = append(out, NIC{Name: ifc.Name, MAC: ifc.HardwareAddr, Addrs: v4})
	}
	return out
}
