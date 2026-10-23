package discovery

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/grandcat/zeroconf"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func announcement(channel string) Announcement {
	return Announcement{Channel: channel, ConsoleName: "Lab 3 — Prof. Ayala", WSPort: 47830, CertFingerprint: PlaceholderFingerprint}
}

func entryFor(a Announcement, ips ...string) *zeroconf.ServiceEntry {
	e := zeroconf.NewServiceEntry(instanceName(a.ConsoleName), ServiceType, Domain)
	e.Port = a.WSPort
	e.Text = a.txt()
	for _, s := range ips {
		ip := net.ParseIP(s)
		if ip.To4() != nil {
			e.AddrIPv4 = append(e.AddrIPv4, ip)
		} else {
			e.AddrIPv6 = append(e.AddrIPv6, ip)
		}
	}
	return e
}

func TestParseEntryMatchingChannel(t *testing.T) {
	a := announcement("AULA_101")
	c, err := parseEntry(entryFor(a, "192.168.1.10"), "AULA_101", nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Channel != "AULA_101" || c.Name != a.ConsoleName || c.Port != 47830 || c.CertFingerprint != PlaceholderFingerprint {
		t.Fatalf("console = %+v", c)
	}
	if got := c.HostPorts(); len(got) != 1 || got[0] != "192.168.1.10:47830" {
		t.Fatalf("HostPorts = %v", got)
	}
}

func TestParseEntryIgnoresOtherChannel(t *testing.T) {
	_, err := parseEntry(entryFor(announcement("CURSO_PYTHON_A"), "192.168.1.11"), "AULA_101", nil)
	if !errors.Is(err, errOtherChannel) {
		t.Fatalf("err = %v, want errOtherChannel", err)
	}
}

func TestParseEntryRejects(t *testing.T) {
	base := announcement("AULA_101")
	tests := []struct {
		name string
		edit func(e *zeroconf.ServiceEntry)
		want error
	}{
		{"other proto", func(e *zeroconf.ServiceEntry) { e.Text[0] = "proto=2" }, errOtherProto},
		{"missing proto", func(e *zeroconf.ServiceEntry) { e.Text = e.Text[1:] }, errOtherProto},
		{"bad port", func(e *zeroconf.ServiceEntry) { e.Text[3] = "ws_port=99999" }, errMalformed},
		{"bad fingerprint", func(e *zeroconf.ServiceEntry) { e.Text[4] = "cert_fingerprint=abc" }, errMalformed},
		{"only ipv6 link-local", func(e *zeroconf.ServiceEntry) {
			e.AddrIPv4 = nil
			e.AddrIPv6 = []net.IP{net.ParseIP("fe80::1")}
		}, errNoAddress},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := entryFor(base, "192.168.1.10")
			tt.edit(e)
			if _, err := parseEntry(e, "AULA_101", nil); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestParseTXTFirstKeyWinsAndCaseInsensitive(t *testing.T) {
	m := parseTXT([]string{"Channel=AULA_101", "channel=OTRA", "flag", "extra=1"})
	if m["channel"] != "AULA_101" || m["extra"] != "1" {
		t.Fatalf("parseTXT = %v", m)
	}
	if _, ok := m["flag"]; !ok {
		t.Fatal("boolean attribute lost")
	}
}

func TestOrderAddrsPrefersOwnSubnet(t *testing.T) {
	_, labNet, _ := net.ParseCIDR("10.20.0.0/16")
	in := []net.IP{
		net.ParseIP("172.17.0.1"),  // e.g. a Docker/Hyper-V adapter of the console
		net.ParseIP("2001:db8::5"), // global IPv6
		net.ParseIP("169.254.3.4"), // APIPA
		net.ParseIP("10.20.30.40"), // the lab network we are on
		net.ParseIP("fe80::abcd"),  // unusable without zone
		net.ParseIP("10.20.30.40"), // duplicate from a second interface
		net.ParseIP("0.0.0.0"),     // unspecified
	}
	got := orderAddrs(in, []*net.IPNet{labNet})
	want := []string{"10.20.30.40", "172.17.0.1", "169.254.3.4", "2001:db8::5"}
	if len(got) != len(want) {
		t.Fatalf("orderAddrs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i].String() != want[i] {
			t.Fatalf("orderAddrs = %v, want %v", got, want)
		}
	}
}

func TestAnnouncementValidation(t *testing.T) {
	ok := announcement("AULA_101")
	if err := ok.validate(); err != nil {
		t.Fatal(err)
	}
	bad := []Announcement{
		{Channel: "aula", ConsoleName: "x", WSPort: 1, CertFingerprint: PlaceholderFingerprint},
		{Channel: "AULA", ConsoleName: "", WSPort: 1, CertFingerprint: PlaceholderFingerprint},
		{Channel: "AULA", ConsoleName: strings.Repeat("x", 64), WSPort: 1, CertFingerprint: PlaceholderFingerprint},
		{Channel: "AULA", ConsoleName: "x", WSPort: 0, CertFingerprint: PlaceholderFingerprint},
		{Channel: "AULA", ConsoleName: "x", WSPort: 1, CertFingerprint: "sha256:00"},
	}
	for _, a := range bad {
		if a.validate() == nil {
			t.Errorf("validate(%+v) = nil", a)
		}
	}
}

func TestInstanceNameIsDNSSafeAndUnique(t *testing.T) {
	a, b := instanceName("Lab 3 — Prof. Ayala"), instanceName("Lab 3 — Prof. Ayala")
	if a == b {
		t.Fatal("instance names collide")
	}
	if !strings.HasPrefix(a, "Lab-3---Prof--Ayala-") || len(a) > 63 {
		t.Fatalf("instanceName = %q", a)
	}
	if got := instanceName("ñññ"); !strings.HasPrefix(got, "aula-") {
		t.Fatalf("instanceName(non-ASCII) = %q", got)
	}
}

// TestBrowseOverMulticast announces two consoles on the real network stack and
// checks the browser only reports the one of its channel. It is skipped when
// multicast does not work here (common on CI runners); set AULA_REQUIRE_MDNS=1
// to make that a failure.
func TestBrowseOverMulticast(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	mine := Announcement{Channel: "AULA_TEST_" + randSuffix(), ConsoleName: "Consola de prueba", WSPort: 47001, CertFingerprint: PlaceholderFingerprint}
	other := mine
	other.Channel = mine.Channel + "_X"
	other.WSPort = 47002
	for _, a := range []Announcement{mine, other} {
		an := &Announcer{Announcement: a, Logger: quiet}
		errc := make(chan error, 1)
		go func() { errc <- an.Run(ctx) }()
		t.Cleanup(func() {
			if err := <-errc; err != nil {
				t.Logf("announcer: %v", err)
			}
		})
	}

	b := &Browser{Channel: mine.Channel, RequeryInterval: 2 * time.Second, Logger: quiet}
	found, err := b.Browse(ctx)
	if err != nil {
		skipOrFail(t, "browse: "+err.Error())
	}
	for {
		select {
		case c, ok := <-found:
			if !ok {
				skipOrFail(t, "no announcement received over multicast")
				return
			}
			if c.Channel != mine.Channel || c.Port != mine.WSPort {
				t.Fatalf("browser reported %+v from another channel", c)
			}
			// Keep listening a little so the other announcement has time to
			// arrive, then make sure it was never reported.
			time.AfterFunc(1500*time.Millisecond, cancel)
			for c := range found {
				if c.Channel != mine.Channel {
					t.Fatalf("browser reported %+v from another channel", c)
				}
			}
			return
		case <-ctx.Done():
			skipOrFail(t, "no announcement received over multicast")
			return
		}
	}
}

func skipOrFail(t *testing.T, msg string) {
	t.Helper()
	if os.Getenv("AULA_REQUIRE_MDNS") == "1" {
		t.Fatal(msg)
	}
	t.Skip(msg + " (multicast unavailable?)")
}

func randSuffix() string {
	return strings.ToUpper(strings.ReplaceAll(instanceName("x")[2:8], "-", "0"))
}
