package discovery

import (
	"context"
	"math/rand/v2"
	"net"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// The per-interface announcement alone, without zeroconf's server, is enough
// for the browser the students already have (zeroconf's client) to find the
// console. This is what reaches students when Windows sends zeroconf's own
// packets through another interface (Hyper-V's virtual switch).
func TestPerInterfaceAnnouncementIsFound(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a := Announcement{Channel: strconv.Itoa(10000 + rand.IntN(5000)), ConsoleName: "Consola por interfaz",
		WSPort: 47003, CertFingerprint: PlaceholderFingerprint}
	an := &Announcer{Announcement: a, Logger: quiet}
	r := record{instance: instanceName(a.ConsoleName), host: "aula-prueba.local.", port: a.WSPort, txt: a.txt()}
	actx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); an.announceEach(actx, r) }()
	defer func() { stop(); <-done }()

	b := &Browser{Channel: a.Channel, RequeryInterval: 3 * time.Second, Logger: quiet}
	found, err := b.Browse(ctx)
	if err != nil {
		skipOrFail(t, "browse: "+err.Error())
	}
	select {
	case c, ok := <-found:
		if !ok {
			skipOrFail(t, "no announcement received over multicast")
			return
		}
		if c.Channel != a.Channel || c.Port != a.WSPort || c.Name != a.ConsoleName || c.Instance != r.instance {
			t.Fatalf("browser reported %+v", c)
		}
		if len(c.Addrs) == 0 {
			t.Fatal("console found without addresses")
		}
	case <-ctx.Done():
		skipOrFail(t, "no announcement received over multicast")
	}
}

func TestAnnouncementMessage(t *testing.T) {
	r := record{instance: "Prof-Ayala-1a2b3c4d", host: "LAB-PROF.local.", port: 47830, txt: []string{"proto=1", "channel=1"}}
	v4 := []net.IP{net.IPv4(148, 213, 122, 245), net.IPv4(172, 22, 144, 1)}
	for _, ttl := range []uint32{announceTTL, 0} {
		raw, err := r.message(v4, nil, ttl)
		if err != nil {
			t.Fatal(err)
		}
		var m dns.Msg
		if err := m.Unpack(raw); err != nil {
			t.Fatal(err)
		}
		if !m.Response || len(m.Answer) != 1 {
			t.Fatalf("message %v", &m)
		}
		ptr := m.Answer[0].(*dns.PTR)
		if ptr.Hdr.Name != "_aula._tcp.local." || ptr.Ptr != "Prof-Ayala-1a2b3c4d._aula._tcp.local." || ptr.Hdr.Ttl != ttl {
			t.Fatalf("PTR %v", ptr)
		}
		var ips []string
		for _, rr := range m.Extra {
			switch rr := rr.(type) {
			case *dns.SRV:
				if rr.Port != 47830 || rr.Target != "LAB-PROF.local." {
					t.Fatalf("SRV %v", rr)
				}
			case *dns.TXT:
				if !slices.Equal(rr.Txt, r.txt) {
					t.Fatalf("TXT %v", rr)
				}
			case *dns.A:
				ips = append(ips, rr.A.String())
			}
		}
		if !slices.Equal(ips, []string{"148.213.122.245", "172.22.144.1"}) {
			t.Fatalf("A records %v", ips)
		}
	}
}
