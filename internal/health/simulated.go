package health

import "github.com/bayalad7/aula/internal/protocol"

// Simulated is the report of the n-th simulated student (n ≥ 1): invented
// values, the same for the same n, so that some carry alerts and the
// Laboratorio view can be tried without a real lab (docs/privacidad.md
// rule 95). seq, reason and refID are those of the report.
func Simulated(n, seq int, reason protocol.HealthReason, refID string) protocol.HealthReport {
	r := protocol.HealthReport{
		Seq: seq, Reason: reason, RefID: refID,
		Disks: []protocol.HealthDisk{
			{Role: protocol.DiskSystem, FreeBytes: int64(60+n) * gib, TotalBytes: 238 * gib},
			{Role: protocol.DiskDocuments, FreeBytes: int64(60+n) * gib, TotalBytes: 238 * gib},
		},
		DiskHealth:    protocol.DiskOK,
		Memory:        &protocol.HealthMemory{TotalBytes: 8 * gib, AvailableBytes: int64(3+n%3) * gib},
		CPU:           &protocol.HealthCPU{LoadPct: ptr(5 + n%7*3), LimitPct: ptr(100)},
		UptimeS:       ptr(int64(3600 * (2 + n%9))),
		RebootPending: ptr(false),
		Network:       &protocol.HealthNetwork{Kind: protocol.NetEthernet, LinkMbps: ptr(1000)},
	}
	switch n % 5 {
	case 2: // almost full system disk
		r.Disks[0].FreeBytes, r.Disks[1].FreeBytes = 3*gib, 3*gib
	case 3: // on Wi-Fi with a cable plugged in, weak signal
		r.Network = &protocol.HealthNetwork{Kind: protocol.NetWiFi, LinkMbps: ptr(144), WiFiSignalPct: ptr(34), CableIdle: ptr(true)}
	case 4: // a bad cable and a pending reboot
		r.Network.LinkMbps = ptr(10)
		r.RebootPending = ptr(true)
	case 0: // a laptop unplugged
		r.Battery = &protocol.HealthBattery{ChargePct: 54, OnAC: false, WearPct: ptr(12)}
	}
	return r
}

// SimulatedSelfTest is the self-test of the n-th simulated student: all
// fine but a display adapter without its driver (n % 5 == 2) and a class
// service that does not answer (n % 5 == 3).
func SimulatedSelfTest(n int, cfg *protocol.HealthConfig, refID, testID string) protocol.HealthSelfTestResult {
	ok := func(id protocol.CheckID, service *int) protocol.HealthCheck {
		return protocol.HealthCheck{ID: id, Service: service, Status: protocol.CheckOK}
	}
	checks := []protocol.HealthCheck{ok(protocol.CheckDiskWrite, nil)}
	if cfg != nil {
		if cfg.Internet != nil {
			checks = append(checks, ok(protocol.CheckDNS, nil), ok(protocol.CheckTCP, nil))
		}
		for i := range cfg.Services {
			tcp := ok(protocol.CheckTCP, ptr(i))
			if n%5 == 3 {
				tcp.Status, tcp.Reason = protocol.CheckFail, protocol.ReasonTimeout
			}
			checks = append(checks, ok(protocol.CheckDNS, ptr(i)), tcp)
		}
	}
	gpu := ok(protocol.CheckGPU, nil)
	if n%5 == 2 {
		gpu.Status, gpu.Reason = protocol.CheckFail, protocol.ReasonBasicDriver
	}
	checks = append(checks, ok(protocol.CheckAudioOut, nil), ok(protocol.CheckNIC, nil), gpu,
		ok(protocol.CheckKeyboard, nil), ok(protocol.CheckMouse, nil))
	return protocol.HealthSelfTestResult{RefID: refID, TestID: testID, Status: protocol.SelfTestDone, Checks: checks}
}
