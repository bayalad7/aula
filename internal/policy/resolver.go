package policy

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"

	"github.com/bayalad7/aula/internal/platform"
)

// DefaultResolverAddr is where the web filter answers; the system's resolver
// is pointed at it (D-017). Loopback only: no firewall rule is needed.
const DefaultResolverAddr = "127.0.0.1:53"

const forwardTimeout = 3 * time.Second

// Resolver is the local DNS server of the web filter. It refuses blocked
// names (NXDOMAIN) and forwards the rest to the computer's own DNS servers.
// It never records which names were asked (docs/privacidad.md rule 23).
type Resolver struct {
	Addr string // e.g. DefaultResolverAddr; port 0 picks one (tests)
	// Policy returns the filter in force.
	Policy func() *WebPolicy
	// Upstreams returns the servers to forward to and the local suffixes.
	Upstreams func() platform.DNSConfig
	// Port of the upstream servers; 0 means 53 (tests use another).
	UpstreamPort int

	mu   sync.Mutex
	udp  *dns.Server
	tcp  *dns.Server
	addr string
}

// Start listens on UDP and TCP; it fails if the port is taken.
func (r *Resolver) Start() error {
	pc, err := net.ListenPacket("udp", r.Addr)
	if err != nil {
		return fmt.Errorf("dns resolver: %w", err)
	}
	ln, err := net.Listen("tcp", pc.LocalAddr().String())
	if err != nil {
		pc.Close()
		return fmt.Errorf("dns resolver: %w", err)
	}
	r.mu.Lock()
	r.udp = &dns.Server{PacketConn: pc, Handler: r}
	r.tcp = &dns.Server{Listener: ln, Handler: r}
	r.addr = pc.LocalAddr().String()
	udp, tcp := r.udp, r.tcp
	r.mu.Unlock()
	started := make(chan struct{}, 2)
	udp.NotifyStartedFunc = func() { started <- struct{}{} }
	tcp.NotifyStartedFunc = func() { started <- struct{}{} }
	go udp.ActivateAndServe() //nolint:errcheck // ends on Close
	go tcp.ActivateAndServe() //nolint:errcheck // ends on Close
	<-started
	<-started
	return nil
}

// LocalAddr is the address it listens on.
func (r *Resolver) LocalAddr() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.addr
}

// Close stops it; idempotent.
func (r *Resolver) Close() error {
	r.mu.Lock()
	udp, tcp := r.udp, r.tcp
	r.udp, r.tcp = nil, nil
	r.mu.Unlock()
	var errs []error
	for _, s := range []*dns.Server{udp, tcp} {
		if s != nil {
			errs = append(errs, s.Shutdown())
		}
	}
	return errors.Join(errs...)
}

// ServeDNS implements dns.Handler.
func (r *Resolver) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	cfg := platform.DNSConfig{}
	if r.Upstreams != nil {
		cfg = r.Upstreams()
	}
	if len(req.Question) > 0 {
		var p *WebPolicy
		if r.Policy != nil {
			p = r.Policy()
		}
		if p.Blocks(req.Question[0].Name, setOf(cfg.Suffixes)) {
			m := new(dns.Msg).SetRcode(req, dns.RcodeNameError)
			m.Authoritative, m.RecursionAvailable = true, true
			_ = w.WriteMsg(m)
			return
		}
	}
	network := "udp"
	if _, ok := w.LocalAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}
	port := r.UpstreamPort
	if port == 0 {
		port = 53
	}
	c := &dns.Client{Net: network, Timeout: forwardTimeout}
	for _, server := range cfg.Servers {
		resp, _, err := c.Exchange(req, net.JoinHostPort(server, fmt.Sprint(port)))
		if err != nil || resp == nil {
			continue
		}
		resp.Id = req.Id
		_ = w.WriteMsg(resp)
		return
	}
	_ = w.WriteMsg(new(dns.Msg).SetRcode(req, dns.RcodeServerFailure))
}
