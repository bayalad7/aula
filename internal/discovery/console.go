package discovery

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/grandcat/zeroconf"

	"github.com/bayalad7/aula/internal/protocol"
)

// DNS-SD service type and domain of Aula consoles.
const (
	ServiceType = "_aula._tcp"
	Domain      = "local."
)

// PlaceholderFingerprint is announced until the console has a TLS certificate
// (stage 05). It has the format of a real SHA-256 fingerprint.
const PlaceholderFingerprint = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

// TXT record keys (docs/protocol.md §1.1).
const (
	txtProto       = "proto"
	txtChannel     = "channel"
	txtConsoleName = "console_name"
	txtWSPort      = "ws_port"
	txtFingerprint = "cert_fingerprint"
)

const maxConsoleName = 63 // bytes; also the DNS-SD instance name limit

var fingerprintPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Console is a console found on the network for the agent's channel.
type Console struct {
	Instance        string   // DNS-SD instance name; unique key of the announcement
	Channel         string   // classroom channel
	Name            string   // console_name, for display only
	Addrs           []net.IP // best candidates first; see orderAddrs
	Port            int      // ws_port
	CertFingerprint string   // "sha256:<hex>"
}

// HostPorts returns "host:port" for every address, in preference order.
func (c Console) HostPorts() []string {
	out := make([]string, len(c.Addrs))
	for i, ip := range c.Addrs {
		out[i] = net.JoinHostPort(ip.String(), strconv.Itoa(c.Port))
	}
	return out
}

// Reasons an announcement is skipped by the browser.
var (
	errOtherChannel = errors.New("announcement for another channel")
	errOtherProto   = errors.New("announcement for another protocol version")
	errMalformed    = errors.New("malformed announcement")
	errNoAddress    = errors.New("announcement without usable address")
)

// parseEntry turns a DNS-SD entry into a Console if it belongs to channel and
// speaks our protocol version. localNets orders the addresses.
func parseEntry(e *zeroconf.ServiceEntry, channel string, localNets []*net.IPNet) (Console, error) {
	txt := parseTXT(e.Text)
	if txt[txtProto] != strconv.Itoa(protocol.Version) {
		return Console{}, errOtherProto
	}
	if txt[txtChannel] != channel {
		return Console{}, errOtherChannel
	}
	port, err := strconv.Atoi(txt[txtWSPort])
	if err != nil || port < 1 || port > 65535 {
		return Console{}, fmt.Errorf("%w: ws_port", errMalformed)
	}
	if !fingerprintPattern.MatchString(txt[txtFingerprint]) {
		return Console{}, fmt.Errorf("%w: cert_fingerprint", errMalformed)
	}
	addrs := orderAddrs(append(append([]net.IP{}, e.AddrIPv4...), e.AddrIPv6...), localNets)
	if len(addrs) == 0 {
		return Console{}, errNoAddress
	}
	return Console{
		Instance:        e.Instance,
		Channel:         channel,
		Name:            txt[txtConsoleName],
		Addrs:           addrs,
		Port:            port,
		CertFingerprint: txt[txtFingerprint],
	}, nil
}

// parseTXT parses "key=value" strings. Unknown keys are kept and ignored by
// callers; the first occurrence of a key wins (RFC 6763 §6.4).
func parseTXT(records []string) map[string]string {
	m := make(map[string]string, len(records))
	for _, r := range records {
		k, v, _ := strings.Cut(r, "=")
		k = strings.ToLower(k)
		if _, dup := m[k]; !dup {
			m[k] = v
		}
	}
	return m
}

// orderAddrs drops unusable addresses and sorts the rest so that a console with
// several network interfaces is reached through the most likely one first:
// addresses inside one of our own subnets, then IPv4 before IPv6, then
// link-local IPv4 last. IPv6 link-local addresses are dropped because they need
// a zone that mDNS does not give us.
func orderAddrs(addrs []net.IP, localNets []*net.IPNet) []net.IP {
	type scored struct {
		ip    net.IP
		score int
	}
	seen := map[string]bool{}
	var list []scored
	for _, ip := range addrs {
		if ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
			continue
		}
		if ip.To4() == nil && ip.IsLinkLocalUnicast() {
			continue
		}
		if seen[ip.String()] {
			continue
		}
		seen[ip.String()] = true
		score := 0
		if !inAny(ip, localNets) {
			score += 4
		}
		if ip.To4() == nil {
			score += 2
		}
		if ip.IsLinkLocalUnicast() {
			score++
		}
		list = append(list, scored{ip, score})
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].score < list[j].score })
	out := make([]net.IP, len(list))
	for i, s := range list {
		out[i] = s.ip
	}
	return out
}

func inAny(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// localNetworks returns the subnets of this machine's interfaces. Errors are
// ignored: ordering is only a preference.
func localNetworks() []*net.IPNet {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var nets []*net.IPNet
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && !n.IP.IsLoopback() {
			nets = append(nets, n)
		}
	}
	return nets
}
