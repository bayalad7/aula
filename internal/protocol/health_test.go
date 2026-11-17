package protocol

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func ip(n int) *int      { return &n }
func i64(n int64) *int64 { return &n }
func bp(b bool) *bool    { return &b }

func sampleHealthReport() HealthReport {
	return HealthReport{
		Seq: 14, Reason: HealthPeriodic,
		Disks: []HealthDisk{
			{Role: DiskSystem, FreeBytes: 3 << 30, TotalBytes: 256 << 30},
			{Role: DiskDocuments, FreeBytes: 3 << 30, TotalBytes: 256 << 30},
		},
		DiskHealth:    DiskOK,
		Memory:        &HealthMemory{TotalBytes: 8 << 30, AvailableBytes: 3 << 30},
		CPU:           &HealthCPU{LoadPct: ip(12), LimitPct: ip(100)},
		UptimeS:       i64(93600),
		RebootPending: bp(false),
		Network:       &HealthNetwork{Kind: NetWiFi, LinkMbps: ip(144), WiFiSignalPct: ip(34), CableIdle: bp(true)},
		Internet:      &HealthInternet{DNS: ProbeOK, Reach: ProbeFail},
		Battery:       &HealthBattery{ChargePct: 54, OnAC: true, WearPct: ip(12)},
	}
}

func sampleSelfTestResult() HealthSelfTestResult {
	return HealthSelfTestResult{RefID: testSnippetID, TestID: testRefID, Status: SelfTestDone, Checks: []HealthCheck{
		{ID: CheckDiskWrite, Status: CheckOK},
		{ID: CheckDNS, Status: CheckOK},
		{ID: CheckTCP, Status: CheckOK},
		{ID: CheckTCP, Service: ip(0), Status: CheckFail, Reason: ReasonRefused},
		{ID: CheckAudioOut, Status: CheckOK},
		{ID: CheckNIC, Status: CheckOK},
		{ID: CheckGPU, Status: CheckFail, Reason: ReasonBasicDriver},
		{ID: CheckKeyboard, Status: CheckOK},
		{ID: CheckMouse, Status: CheckUnknown, Reason: ReasonNotSupported},
	}}
}

// TestHealthReportClosedList is the closed list of docs/protocol.md §5.27.3
// and docs/privacidad.md rule 92: a report with every field set carries
// these JSON keys and no other. A new datum fails here until the protocol
// and the privacy rule list it.
func TestHealthReportClosedList(t *testing.T) {
	raw, err := json.Marshal(sampleHealthReport())
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	keys := func(m map[string]any) []string {
		var k []string
		for x := range m {
			k = append(k, x)
		}
		slices.Sort(k)
		return k
	}
	want := map[string][]string{
		"":         {"battery", "cpu", "disk_health", "disks", "internet", "memory", "network", "reason", "reboot_pending", "seq", "uptime_s"},
		"battery":  {"charge_pct", "on_ac", "wear_pct"},
		"cpu":      {"limit_pct", "load_pct"},
		"internet": {"dns", "reach"},
		"memory":   {"available_bytes", "total_bytes"},
		"network":  {"cable_idle", "kind", "link_mbps", "wifi_signal_pct"},
	}
	if k := keys(got); !reflect.DeepEqual(k, want[""]) {
		t.Errorf("report keys = %v, want %v", k, want[""])
	}
	for field, w := range want {
		if field == "" {
			continue
		}
		if k := keys(got[field].(map[string]any)); !reflect.DeepEqual(k, w) {
			t.Errorf("%s keys = %v, want %v", field, k, w)
		}
	}
	disk := got["disks"].([]any)[0].(map[string]any)
	if k := keys(disk); !reflect.DeepEqual(k, []string{"free_bytes", "role", "total_bytes"}) {
		t.Errorf("disk keys = %v", k)
	}
	// The Go type cannot grow behind the test's back either.
	if n := reflect.TypeOf(HealthReport{}).NumField(); n != 12 {
		t.Errorf("HealthReport has %d fields; the closed list has 12 (with ref_id)", n)
	}
	// Nothing about the student nor what they do.
	for _, never := range []string{"process", "app", "url", "site", "file", "user", "ssid", "location", "ip", "mac"} {
		if strings.Contains(string(raw), `"`+never) {
			t.Errorf("the report mentions %q", never)
		}
	}
}

func TestHealthLimits(t *testing.T) {
	r := func(f func(*HealthReport)) HealthReport {
		p := sampleHealthReport()
		f(&p)
		return p
	}
	for _, p := range []Payload{
		HealthConfig{Internet: &HealthTarget{Host: "", Port: 443}},
		HealthConfig{Internet: &HealthTarget{Host: "a b", Port: 443}},
		HealthConfig{Internet: &HealthTarget{Host: "x.com", Port: 0}},
		HealthConfig{Internet: &HealthTarget{Host: "x.com", Port: 65536}},
		HealthConfig{Internet: &HealthTarget{Host: strings.Repeat("a", MaxHealthHost+1), Port: 1}},
		HealthConfig{Services: make([]HealthService, MaxHealthServices+1)},
		HealthConfig{Services: []HealthService{{Name: "", Host: "x", Port: 1}}},
		HealthConfig{Services: []HealthService{{Name: "a\nb", Host: "x", Port: 1}}},
		HealthConfig{Services: []HealthService{{Name: strings.Repeat("ñ", MaxHealthServiceName+1), Host: "x", Port: 1}}},
		r(func(p *HealthReport) { p.Seq = 0 }),
		r(func(p *HealthReport) { p.Reason = "later" }),
		r(func(p *HealthReport) { p.RefID = testRefID }),
		r(func(p *HealthReport) { p.Reason = HealthOnDemand }),
		r(func(p *HealthReport) { p.Disks = append(p.Disks, HealthDisk{Role: DiskSystem}) }),
		r(func(p *HealthReport) { p.Disks = []HealthDisk{{Role: DiskSystem}, {Role: DiskSystem}} }),
		r(func(p *HealthReport) { p.Disks = []HealthDisk{{Role: "usb"}} }),
		r(func(p *HealthReport) { p.Disks[0].FreeBytes = p.Disks[0].TotalBytes + 1 }),
		r(func(p *HealthReport) { p.DiskHealth = "bad" }),
		r(func(p *HealthReport) { p.Memory.TotalBytes = 0 }),
		r(func(p *HealthReport) { p.Memory.AvailableBytes = p.Memory.TotalBytes + 1 }),
		r(func(p *HealthReport) { p.CPU.LoadPct = ip(101) }),
		r(func(p *HealthReport) { p.CPU.LimitPct = ip(0) }),
		r(func(p *HealthReport) { p.UptimeS = i64(-1) }),
		r(func(p *HealthReport) { p.Network.Kind = "bluetooth" }),
		r(func(p *HealthReport) { p.Network.Kind = NetEthernet }), // wifi fields on ethernet
		r(func(p *HealthReport) { p.Network.WiFiSignalPct = ip(101) }),
		r(func(p *HealthReport) { p.Network.LinkMbps = ip(-1) }),
		r(func(p *HealthReport) { p.Internet.DNS = "maybe" }),
		r(func(p *HealthReport) { p.Battery.ChargePct = 101 }),
		r(func(p *HealthReport) { p.Battery.WearPct = ip(-1) }),
		HealthSelfTest{TestID: "x"},
		HealthSelfTestResult{RefID: testRefID, TestID: testRefID, Status: SelfTestDone},
		HealthSelfTestResult{RefID: testRefID, TestID: testRefID, Status: SelfTestBusy, Checks: []HealthCheck{{ID: CheckNIC, Status: CheckOK}}},
		HealthSelfTestResult{RefID: testRefID, TestID: testRefID, Status: "later"},
		HealthSelfTestResult{RefID: testRefID, TestID: testRefID, Status: SelfTestDone, Checks: []HealthCheck{{ID: "processes", Status: CheckOK}}},
		HealthSelfTestResult{RefID: testRefID, TestID: testRefID, Status: SelfTestDone, Checks: []HealthCheck{{ID: CheckNIC, Service: ip(0), Status: CheckOK}}},
		HealthSelfTestResult{RefID: testRefID, TestID: testRefID, Status: SelfTestDone, Checks: []HealthCheck{{ID: CheckTCP, Service: ip(MaxHealthServices), Status: CheckOK}}},
		HealthSelfTestResult{RefID: testRefID, TestID: testRefID, Status: SelfTestDone, Checks: []HealthCheck{{ID: CheckNIC, Status: "maybe"}}},
		HealthSelfTestResult{RefID: testRefID, TestID: testRefID, Status: SelfTestDone, Checks: []HealthCheck{{ID: CheckNIC, Status: CheckFail, Reason: "No Driver"}}},
		HealthSelfTestResult{RefID: testRefID, TestID: testRefID, Status: SelfTestDone, Checks: make([]HealthCheck, MaxHealthChecks+1)},
	} {
		if err := p.Validate(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%T %+v: Validate = %v, want invalid_message", p, p, err)
		}
	}
	for _, h := range []string{"www.google.com", "lic.escuela.local", "10.0.0.5", "fe80::1", "localhost", "a-b.c."} {
		if !ValidHealthHost(h) {
			t.Errorf("host %q should be valid", h)
		}
	}
	for _, h := range []string{"-a.com", "a..b", "a_b.com", "http://x.com", "x.com:443"} {
		if ValidHealthHost(h) {
			t.Errorf("host %q should be invalid", h)
		}
	}
	// An empty report (nothing readable) is still a report.
	if err := (HealthReport{Seq: 1, Reason: HealthFirst}).Validate(); err != nil {
		t.Errorf("empty report: %v", err)
	}
}
