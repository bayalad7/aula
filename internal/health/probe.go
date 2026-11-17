package health

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// ProbeTimeout bounds each DNS lookup and each TCP connection.
const ProbeTimeout = 3 * time.Second

// Prober resolves names and opens TCP connections to the destinations of
// health.config. The errors it returns are classified by Reason.
type Prober interface {
	Resolve(ctx context.Context, host string) error
	Dial(ctx context.Context, host string, port int) error
}

// NetProber probes with the standard library.
type NetProber struct{}

// Resolve looks host up; an IP address resolves to itself.
func (NetProber) Resolve(ctx context.Context, host string) error {
	if net.ParseIP(host) != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()
	_, err := net.DefaultResolver.LookupHost(ctx, host)
	return err
}

// Dial opens and closes a TCP connection.
func (NetProber) Dial(ctx context.Context, host string, port int) error {
	d := net.Dialer{Timeout: ProbeTimeout}
	c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return err
	}
	return c.Close()
}

// Reason classifies a probe error into the reasons of §5.27.5.
func Reason(err error) string {
	var dns *net.DNSError
	var ne net.Error
	switch {
	case err == nil:
		return ""
	case errors.As(err, &dns) && dns.IsNotFound:
		return protocol.ReasonNotFound
	case errors.Is(err, context.DeadlineExceeded) || errors.As(err, &ne) && ne.Timeout():
		return protocol.ReasonTimeout
	case strings.Contains(strings.ToLower(err.Error()), "refused"):
		return protocol.ReasonRefused
	}
	return protocol.ReasonUnreachable
}

// probeTarget resolves and connects to t: the dns and reach of a report.
func probeTarget(ctx context.Context, p Prober, t protocol.HealthTarget) protocol.HealthInternet {
	if p.Resolve(ctx, t.Host) != nil {
		return protocol.HealthInternet{DNS: protocol.ProbeFail, Reach: protocol.ProbeFail}
	}
	r := protocol.HealthInternet{DNS: protocol.ProbeOK, Reach: protocol.ProbeOK}
	if p.Dial(ctx, t.Host, t.Port) != nil {
		r.Reach = protocol.ProbeFail
	}
	return r
}
