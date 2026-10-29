package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/grandcat/zeroconf"

	"github.com/bayalad7/aula/internal/protocol"
)

// Announcement is what a console publishes over mDNS.
type Announcement struct {
	Channel         string // e.g. "1"
	ConsoleName     string // shown to students; 1-63 bytes
	WSPort          int    // TCP port of the WebSocket server
	CertFingerprint string // "sha256:<64 hex>"; PlaceholderFingerprint until TLS exists
}

func (a Announcement) validate() error {
	if !protocol.ValidChannel(a.Channel) {
		return fmt.Errorf("announce: invalid channel %q", a.Channel)
	}
	if len(a.ConsoleName) == 0 || len(a.ConsoleName) > maxConsoleName {
		return fmt.Errorf("announce: console name must be 1-%d bytes", maxConsoleName)
	}
	if a.WSPort < 1 || a.WSPort > 65535 {
		return fmt.Errorf("announce: invalid port %d", a.WSPort)
	}
	if !fingerprintPattern.MatchString(a.CertFingerprint) {
		return fmt.Errorf("announce: cert fingerprint must be sha256:<64 hex>")
	}
	return nil
}

func (a Announcement) txt() []string {
	return []string{
		txtProto + "=" + strconv.Itoa(protocol.Version),
		txtChannel + "=" + a.Channel,
		txtConsoleName + "=" + a.ConsoleName,
		txtWSPort + "=" + strconv.Itoa(a.WSPort),
		txtFingerprint + "=" + a.CertFingerprint,
	}
}

// Announcer publishes a console on the LAN while Run is active.
type Announcer struct {
	Announcement Announcement
	// Interfaces restricts the announcement; nil means every multicast-capable
	// interface, so students on any of the console's networks can find it.
	Interfaces []net.Interface
	Logger     *slog.Logger
}

// Run registers the service and keeps it published until ctx is canceled, then
// withdraws it (mDNS goodbye) and returns nil.
func (a *Announcer) Run(ctx context.Context) error {
	if err := a.Announcement.validate(); err != nil {
		return err
	}
	srv, err := zeroconf.Register(instanceName(a.Announcement.ConsoleName), ServiceType, Domain,
		a.Announcement.WSPort, a.Announcement.txt(), a.Interfaces)
	if err != nil {
		return fmt.Errorf("announce: register mDNS service: %w", err)
	}
	logger(a.Logger).Info("mdns announce started", "channel", a.Announcement.Channel, "port", a.Announcement.WSPort)
	<-ctx.Done()
	srv.Shutdown()
	logger(a.Logger).Info("mdns announce stopped", "channel", a.Announcement.Channel)
	return nil
}

// instanceName derives a DNS-safe, unique instance name from the console name.
// Display uses the console_name TXT record, so the instance only needs to be a
// stable key; the random suffix avoids collisions between consoles with the
// same name.
func instanceName(consoleName string) string {
	var b strings.Builder
	for _, r := range consoleName {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if len(name) > 40 {
		name = name[:40]
	}
	if name == "" {
		name = "aula"
	}
	return name + "-" + protocol.NewID()[:8]
}

func logger(l *slog.Logger) *slog.Logger {
	if l == nil {
		return slog.Default()
	}
	return l
}
