package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"time"

	"github.com/grandcat/zeroconf"

	"github.com/bayalad7/aula/internal/protocol"
)

// DefaultRequeryInterval is how often the browser restarts its mDNS query.
// zeroconf stops probing after the first answer, so without restarts a console
// started later would never be seen.
const DefaultRequeryInterval = 30 * time.Second

// Browser finds consoles of one channel.
type Browser struct {
	Channel string
	// Interfaces restricts the query; nil means every multicast-capable
	// interface, which covers machines with several network cards.
	Interfaces      []net.Interface
	RequeryInterval time.Duration // 0 means DefaultRequeryInterval
	Logger          *slog.Logger
}

// Browse starts looking for consoles. Each console of b.Channel is sent once
// when found, and again if its set of addresses changes (keyed by Instance).
// Announcements for other channels or protocol versions are ignored. The
// returned channel is closed when ctx is canceled.
func (b *Browser) Browse(ctx context.Context) (<-chan Console, error) {
	if !protocol.ValidChannel(b.Channel) {
		return nil, fmt.Errorf("browse: invalid channel %q", b.Channel)
	}
	interval := b.RequeryInterval
	if interval <= 0 {
		interval = DefaultRequeryInterval
	}
	// Create the first resolver here to fail fast if multicast cannot be set up.
	first, err := b.newResolver()
	if err != nil {
		return nil, err
	}

	out := make(chan Console)
	go func() {
		defer close(out)
		seen := map[string]Console{}
		resolver := first
		for ctx.Err() == nil {
			err := b.round(ctx, resolver, interval, seen, out)
			resolver = nil
			if err != nil {
				logger(b.Logger).Warn("mdns browse round failed", "err", err)
				select {
				case <-ctx.Done():
				case <-time.After(interval / 6):
				}
			}
		}
	}()
	return out, nil
}

// round runs one zeroconf query for at most interval. A nil resolver means
// create a new one: each zeroconf query owns and closes its sockets.
func (b *Browser) round(ctx context.Context, resolver *zeroconf.Resolver, interval time.Duration, seen map[string]Console, out chan<- Console) error {
	if resolver == nil {
		var err error
		if resolver, err = b.newResolver(); err != nil {
			return err
		}
	}
	rctx, cancel := context.WithTimeout(ctx, interval)
	defer cancel()
	entries := make(chan *zeroconf.ServiceEntry)
	if err := resolver.Browse(rctx, ServiceType, Domain, entries); err != nil {
		return fmt.Errorf("browse: start mDNS query: %w", err)
	}
	localNets := localNetworks()
	// zeroconf closes entries when rctx is done; keep draining until then so
	// its loop never blocks.
	for e := range entries {
		c, err := parseEntry(e, b.Channel, localNets)
		if err != nil {
			logger(b.Logger).Debug("mdns announcement skipped", "instance", e.Instance, "reason", err)
			continue
		}
		if prev, ok := seen[c.Instance]; ok && sameAddrs(prev, c) {
			continue
		}
		seen[c.Instance] = c
		select {
		case out <- c:
		case <-ctx.Done():
			return nil
		}
	}
	return nil
}

func (b *Browser) newResolver() (*zeroconf.Resolver, error) {
	var opts []zeroconf.ClientOption
	if len(b.Interfaces) > 0 {
		opts = append(opts, zeroconf.SelectIfaces(b.Interfaces))
	}
	r, err := zeroconf.NewResolver(opts...)
	if err != nil {
		return nil, fmt.Errorf("browse: create mDNS resolver: %w", err)
	}
	return r, nil
}

func sameAddrs(a, b Console) bool {
	return a.Port == b.Port && a.CertFingerprint == b.CertFingerprint &&
		slices.EqualFunc(a.Addrs, b.Addrs, net.IP.Equal)
}
