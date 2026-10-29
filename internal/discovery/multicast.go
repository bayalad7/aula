package discovery

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
)

// Announcements sent on each interface (D-015).
//
// zeroconf chooses the outgoing interface of its multicast packets with
// ipv4.ControlMessage.IfIndex, which Windows ignores: every announcement and
// answer leaves through the system's default multicast interface. On a
// teacher machine with Hyper-V, that is the virtual switch ("vEthernet
// (Default Switch)"), and students on the real network never hear the
// console. The console therefore also sends its announcement on every
// interface itself, choosing the interface with IP_MULTICAST_IF, which
// Windows honours. Students need no change: zeroconf's browser accepts any
// response that carries the PTR, SRV, TXT and A records together.

// announceEvery is how often the per-interface announcement is repeated;
// a student finds a console within this time.
const announceEvery = 5 * time.Second

// announceTTL matches zeroconf's default record TTL, in seconds.
const announceTTL = 3200

var mdnsGroup = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

// record is what the console publishes, shared by zeroconf and the
// per-interface announcements so students see one console.
type record struct {
	instance string // e.g. "Prof-Ayala-1a2b3c4d"
	host     string // "<hostname>.local."
	port     int
	txt      []string
}

// message builds the mDNS response announcing r with the given addresses;
// ttl 0 is the goodbye that withdraws it.
func (r record) message(v4, v6 []net.IP, ttl uint32) ([]byte, error) {
	service := ServiceType + "." + Domain
	instance := r.instance + "." + service
	m := new(dns.Msg)
	m.Response, m.Authoritative = true, true
	m.Answer = []dns.RR{&dns.PTR{
		Hdr: dns.RR_Header{Name: service, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: ttl},
		Ptr: instance,
	}}
	m.Extra = []dns.RR{
		&dns.SRV{Hdr: dns.RR_Header{Name: instance, Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: ttl},
			Port: uint16(r.port), Target: r.host},
		&dns.TXT{Hdr: dns.RR_Header{Name: instance, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: ttl}, Txt: r.txt},
	}
	addrTTL := min(ttl, 120) // RFC 6762 §10, as zeroconf does
	for _, ip := range v4 {
		m.Extra = append(m.Extra, &dns.A{Hdr: dns.RR_Header{Name: r.host, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: addrTTL}, A: ip})
	}
	for _, ip := range v6 {
		m.Extra = append(m.Extra, &dns.AAAA{Hdr: dns.RR_Header{Name: r.host, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: addrTTL}, AAAA: ip})
	}
	b, err := m.Pack()
	if err != nil {
		return nil, fmt.Errorf("announce: pack mDNS message: %w", err)
	}
	return b, nil
}

// multicastInterfaces returns the interfaces to announce on (restricted, if
// set) and the console's addresses, chosen exactly as zeroconf does so both
// announcements carry the same records.
func multicastInterfaces(restrict []net.Interface) (ifaces []net.Interface, v4, v6 []net.IP) {
	ifaces = restrict
	if len(ifaces) == 0 {
		all, err := net.Interfaces()
		if err != nil {
			return nil, nil, nil
		}
		for _, ifi := range all {
			if ifi.Flags&net.FlagUp != 0 && ifi.Flags&net.FlagMulticast != 0 {
				ifaces = append(ifaces, ifi)
			}
		}
	}
	for i := range ifaces {
		a4, a6 := addrsOf(&ifaces[i])
		v4, v6 = append(v4, a4...), append(v6, a6...)
	}
	return ifaces, v4, v6
}

// addrsOf is zeroconf's addrsForInterface: non-loopback IPv4, and global IPv6
// (link-local IPv6 only when there is no global one).
func addrsOf(iface *net.Interface) (v4, v6 []net.IP) {
	var v6local []net.IP
	addrs, _ := iface.Addrs()
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		switch ip := ipnet.IP; {
		case ip.To4() != nil:
			v4 = append(v4, ip)
		case ip.IsGlobalUnicast():
			v6 = append(v6, ip)
		case ip.IsLinkLocalUnicast():
			v6local = append(v6local, ip)
		}
	}
	if len(v6) == 0 {
		v6 = v6local
	}
	return v4, v6
}

// announceEach sends the announcement on every IPv4 interface now and every
// announceEvery until ctx is canceled, then sends the goodbye the same way.
// Interfaces and addresses are read again each time: a laptop may plug into
// the lab network after the class opened.
func (a *Announcer) announceEach(ctx context.Context, r record) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero})
	if err != nil {
		logger(a.Logger).Warn("per-interface mDNS announce unavailable", "err", err)
		return
	}
	defer conn.Close()
	pc := ipv4.NewPacketConn(conn)
	_ = pc.SetMulticastTTL(255)
	_ = pc.SetMulticastLoopback(true) // students on this same machine
	failed := map[string]bool{}       // interfaces already reported, to log once

	send := func(ttl uint32) {
		ifaces, v4, v6 := multicastInterfaces(a.Interfaces)
		msg, err := r.message(v4, v6, ttl)
		if err != nil {
			logger(a.Logger).Warn("per-interface mDNS announce", "err", err)
			return
		}
		for i := range ifaces {
			if a4, _ := addrsOf(&ifaces[i]); len(a4) == 0 {
				continue // no IPv4 on it: an IPv4 multicast cannot leave through it
			}
			err := pc.SetMulticastInterface(&ifaces[i])
			if err == nil {
				_, err = pc.WriteTo(msg, nil, mdnsGroup)
			}
			if err != nil && !failed[ifaces[i].Name] {
				failed[ifaces[i].Name] = true
				logger(a.Logger).Debug("mDNS announce failed on an interface", "iface", ifaces[i].Name, "err", err)
			}
		}
	}

	send(announceTTL)
	t := time.NewTicker(announceEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			send(0) // goodbye: students drop the console right away
			return
		case <-t.C:
			send(announceTTL)
		}
	}
}
