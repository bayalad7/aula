package protocol

import (
	"encoding/json"
	"net"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits of the health messages (docs/protocol.md §5.27 and §7).
const (
	MaxHealthServices     = 8
	MaxHealthHost         = 253
	MaxHealthServiceName  = 40
	MaxHealthReportBytes  = 4 << 10
	MaxHealthChecks       = 24
	MaxHealthUptimeS      = 315_360_000 // ten years
	MaxHealthLinkMbps     = 1_000_000
	MaxHealthDisks        = 2
	HealthSelfTestFileLen = 4 << 20 // bytes the disk_write check writes
)

// HealthTarget is a host and port the agent tries to reach.
type HealthTarget struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// HealthService is a destination of the self-test, with the name the
// console shows (the agent never shows it).
type HealthService struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

var healthHostPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)*\.?$`)

// ValidHealthHost reports whether h is a DNS name or an IP address the
// health checks may try (docs/protocol.md §5.27.1).
func ValidHealthHost(h string) bool {
	if h == "" || len(h) > MaxHealthHost {
		return false
	}
	if net.ParseIP(h) != nil {
		return true
	}
	return healthHostPattern.MatchString(h)
}

// ValidHealthServiceName reports whether a service name is 1–40
// characters of valid UTF-8 without control characters.
func ValidHealthServiceName(n string) bool {
	return utf8.ValidString(n) && strings.TrimSpace(n) != "" &&
		utf8.RuneCountInString(n) <= MaxHealthServiceName && !strings.ContainsFunc(n, unicode.IsControl)
}

func validPort(p int) bool { return p >= 1 && p <= 65535 }

// HealthConfig gives the agent the destinations its health checks try
// (P → A, §5.27.1). The agent keeps it in memory for the session.
type HealthConfig struct {
	Internet *HealthTarget   `json:"internet,omitempty"`
	Services []HealthService `json:"services,omitempty"`
}

// MessageType implements Payload.
func (HealthConfig) MessageType() Type { return TypeHealthConfig }

// Validate implements Payload.
func (p HealthConfig) Validate() error {
	if p.Internet != nil && (!ValidHealthHost(p.Internet.Host) || !validPort(p.Internet.Port)) {
		return invalid(TypeHealthConfig, "internet is not a valid host and port")
	}
	if len(p.Services) > MaxHealthServices {
		return invalid(TypeHealthConfig, "more than %d services", MaxHealthServices)
	}
	for i, s := range p.Services {
		if !ValidHealthServiceName(s.Name) || !ValidHealthHost(s.Host) || !validPort(s.Port) {
			return invalid(TypeHealthConfig, "service %d is not a valid name, host and port", i)
		}
	}
	return nil
}

// HealthRequest asks the agent for a report now ("Revisar ahora", P → A).
type HealthRequest struct{}

// MessageType implements Payload.
func (HealthRequest) MessageType() Type { return TypeHealthRequest }

// Validate implements Payload.
func (HealthRequest) Validate() error { return nil }

// HealthReason says why a report was sent.
type HealthReason string

// Report reasons.
const (
	HealthFirst    HealthReason = "first"
	HealthPeriodic HealthReason = "periodic"
	HealthChange   HealthReason = "change"
	HealthOnDemand HealthReason = "request"
)

// DiskRole names a disk of the report.
type DiskRole string

// Disk roles.
const (
	DiskSystem    DiskRole = "system"
	DiskDocuments DiskRole = "documents"
)

// DiskHealth is the S.M.A.R.T. state of the system disk.
type DiskHealth string

// Disk health states.
const (
	DiskOK       DiskHealth = "ok"
	DiskDegraded DiskHealth = "degraded"
)

// NetKind is the kind of network card that reaches the console.
type NetKind string

// Network kinds.
const (
	NetEthernet NetKind = "ethernet"
	NetWiFi     NetKind = "wifi"
	NetOther    NetKind = "other"
)

// ProbeResult is the outcome of a network probe.
type ProbeResult string

// Probe results.
const (
	ProbeOK   ProbeResult = "ok"
	ProbeFail ProbeResult = "fail"
)

// HealthDisk is the space of one disk.
type HealthDisk struct {
	Role       DiskRole `json:"role"`
	FreeBytes  int64    `json:"free_bytes"`
	TotalBytes int64    `json:"total_bytes"`
}

// HealthMemory is the physical memory.
type HealthMemory struct {
	TotalBytes     int64 `json:"total_bytes"`
	AvailableBytes int64 `json:"available_bytes"`
}

// HealthCPU is the processor's load and frequency limit. A field the OS
// cannot read is nil.
type HealthCPU struct {
	LoadPct  *int `json:"load_pct,omitempty"`
	LimitPct *int `json:"limit_pct,omitempty"`
}

// HealthNetwork describes the card that reaches the console. Never the
// network's name nor addresses.
type HealthNetwork struct {
	Kind          NetKind `json:"kind"`
	LinkMbps      *int    `json:"link_mbps,omitempty"`
	WiFiSignalPct *int    `json:"wifi_signal_pct,omitempty"`
	CableIdle     *bool   `json:"cable_idle,omitempty"`
}

// HealthInternet is the result of probing the internet destination.
type HealthInternet struct {
	DNS   ProbeResult `json:"dns"`
	Reach ProbeResult `json:"reach"`
}

// HealthBattery is the battery, when the computer has one.
type HealthBattery struct {
	ChargePct int  `json:"charge_pct"`
	OnAC      bool `json:"on_ac"`
	WearPct   *int `json:"wear_pct,omitempty"`
}

// HealthReport is the closed list of what the agent reports about its
// computer (A → P, §5.27.3). Every field but Seq and Reason is optional:
// what the OS cannot read does not travel. Adding a field changes the
// protocol and rule 92 of docs/privacidad.md.
type HealthReport struct {
	Seq           int             `json:"seq"`
	Reason        HealthReason    `json:"reason"`
	RefID         string          `json:"ref_id,omitempty"`
	Disks         []HealthDisk    `json:"disks,omitempty"`
	DiskHealth    DiskHealth      `json:"disk_health,omitempty"`
	Memory        *HealthMemory   `json:"memory,omitempty"`
	CPU           *HealthCPU      `json:"cpu,omitempty"`
	UptimeS       *int64          `json:"uptime_s,omitempty"`
	RebootPending *bool           `json:"reboot_pending,omitempty"`
	Network       *HealthNetwork  `json:"network,omitempty"`
	Internet      *HealthInternet `json:"internet,omitempty"`
	Battery       *HealthBattery  `json:"battery,omitempty"`
}

// MessageType implements Payload.
func (HealthReport) MessageType() Type { return TypeHealthReport }

func pctOK(p *int, lo int) bool { return p == nil || *p >= lo && *p <= 100 }

// Validate implements Payload.
func (p HealthReport) Validate() error {
	t := TypeHealthReport
	if p.Seq < 1 {
		return invalid(t, "seq must be at least 1")
	}
	switch p.Reason {
	case HealthOnDemand:
		if !isUUIDv4(p.RefID) {
			return invalid(t, "ref_id is not a UUID v4")
		}
	case HealthFirst, HealthPeriodic, HealthChange:
		if p.RefID != "" {
			return invalid(t, "ref_id is only for request")
		}
	default:
		return invalid(t, "reason must be first, periodic, change or request")
	}
	if len(p.Disks) > MaxHealthDisks {
		return invalid(t, "more than %d disks", MaxHealthDisks)
	}
	seen := map[DiskRole]bool{}
	for _, d := range p.Disks {
		if d.Role != DiskSystem && d.Role != DiskDocuments || seen[d.Role] {
			return invalid(t, "disk role must be system or documents, once")
		}
		seen[d.Role] = true
		if d.FreeBytes < 0 || d.TotalBytes < 0 || d.FreeBytes > d.TotalBytes {
			return invalid(t, "disk free_bytes must be within 0 and total_bytes")
		}
	}
	switch p.DiskHealth {
	case "", DiskOK, DiskDegraded:
	default:
		return invalid(t, "disk_health must be ok or degraded")
	}
	if m := p.Memory; m != nil && (m.TotalBytes <= 0 || m.AvailableBytes < 0 || m.AvailableBytes > m.TotalBytes) {
		return invalid(t, "memory available_bytes must be within 0 and total_bytes")
	}
	if c := p.CPU; c != nil && (!pctOK(c.LoadPct, 0) || !pctOK(c.LimitPct, 1)) {
		return invalid(t, "cpu load_pct must be 0–100 and limit_pct 1–100")
	}
	if p.UptimeS != nil && (*p.UptimeS < 0 || *p.UptimeS > MaxHealthUptimeS) {
		return invalid(t, "uptime_s out of range")
	}
	if n := p.Network; n != nil {
		switch n.Kind {
		case NetEthernet, NetWiFi, NetOther:
		default:
			return invalid(t, "network kind must be ethernet, wifi or other")
		}
		if n.LinkMbps != nil && (*n.LinkMbps < 0 || *n.LinkMbps > MaxHealthLinkMbps) {
			return invalid(t, "network link_mbps out of range")
		}
		if n.Kind != NetWiFi && (n.WiFiSignalPct != nil || n.CableIdle != nil) {
			return invalid(t, "wifi_signal_pct and cable_idle are only for wifi")
		}
		if !pctOK(n.WiFiSignalPct, 0) {
			return invalid(t, "wifi_signal_pct must be 0–100")
		}
	}
	if i := p.Internet; i != nil && (!validProbe(i.DNS) || !validProbe(i.Reach)) {
		return invalid(t, "internet dns and reach must be ok or fail")
	}
	if b := p.Battery; b != nil && (b.ChargePct < 0 || b.ChargePct > 100 || !pctOK(b.WearPct, 0)) {
		return invalid(t, "battery percentages must be 0–100")
	}
	raw, err := json.Marshal(p)
	if err != nil || len(raw) > MaxHealthReportBytes {
		return invalid(t, "report exceeds %d bytes", MaxHealthReportBytes)
	}
	return nil
}

func validProbe(r ProbeResult) bool { return r == ProbeOK || r == ProbeFail }

// HealthSelfTest asks the agent to run its self-test (P → A, §5.27.5).
type HealthSelfTest struct {
	TestID string `json:"test_id"`
}

// MessageType implements Payload.
func (HealthSelfTest) MessageType() Type { return TypeHealthSelfTest }

// Validate implements Payload.
func (p HealthSelfTest) Validate() error {
	if !isUUIDv4(p.TestID) {
		return invalid(TypeHealthSelfTest, "test_id is not a UUID v4")
	}
	return nil
}

// SelfTestStatus is the outcome of a self-test request.
type SelfTestStatus string

// Self-test statuses.
const (
	SelfTestDone        SelfTestStatus = "done"
	SelfTestBusy        SelfTestStatus = "busy"
	SelfTestUnsupported SelfTestStatus = "unsupported"
)

// CheckID names a self-test check (closed list).
type CheckID string

// Self-test checks.
const (
	CheckDiskWrite CheckID = "disk_write"
	CheckDNS       CheckID = "dns"
	CheckTCP       CheckID = "tcp"
	CheckAudioOut  CheckID = "audio_out"
	CheckNIC       CheckID = "nic"
	CheckGPU       CheckID = "gpu"
	CheckKeyboard  CheckID = "keyboard"
	CheckMouse     CheckID = "mouse"
)

// CheckStatus is the result of one check.
type CheckStatus string

// Check statuses.
const (
	CheckOK      CheckStatus = "ok"
	CheckFail    CheckStatus = "fail"
	CheckUnknown CheckStatus = "unknown"
)

// Check reasons the console has a text for; another reason is shown as a
// plain failure.
const (
	ReasonReadOnly     = "read_only"
	ReasonNoSpace      = "no_space"
	ReasonMismatch     = "mismatch"
	ReasonNotFound     = "not_found"
	ReasonTimeout      = "timeout"
	ReasonRefused      = "refused"
	ReasonUnreachable  = "unreachable"
	ReasonMissing      = "missing"
	ReasonBasicDriver  = "basic_driver"
	ReasonNotSupported = "not_supported"
	ReasonNoPermission = "no_permission"
)

var checkReasonPattern = regexp.MustCompile(`^[a-z_]{1,32}$`)

// HealthCheck is one check of the self-test. Service is set only on dns and
// tcp, with the index in HealthConfig.Services; nil is the internet target.
type HealthCheck struct {
	ID      CheckID     `json:"id"`
	Service *int        `json:"service,omitempty"`
	Status  CheckStatus `json:"status"`
	Reason  string      `json:"reason,omitempty"`
}

// HealthSelfTestResult answers a health.selftest (A → P).
type HealthSelfTestResult struct {
	RefID  string         `json:"ref_id"`
	TestID string         `json:"test_id"`
	Status SelfTestStatus `json:"status"`
	Checks []HealthCheck  `json:"checks,omitempty"`
}

// MessageType implements Payload.
func (HealthSelfTestResult) MessageType() Type { return TypeHealthSelfTestResult }

// Validate implements Payload.
func (p HealthSelfTestResult) Validate() error {
	t := TypeHealthSelfTestResult
	if !isUUIDv4(p.RefID) || !isUUIDv4(p.TestID) {
		return invalid(t, "ref_id and test_id must be UUID v4")
	}
	switch p.Status {
	case SelfTestDone:
		if len(p.Checks) < 1 || len(p.Checks) > MaxHealthChecks {
			return invalid(t, "done needs 1–%d checks", MaxHealthChecks)
		}
	case SelfTestBusy, SelfTestUnsupported:
		if len(p.Checks) > 0 {
			return invalid(t, "checks are only for done")
		}
	default:
		return invalid(t, "status must be done, busy or unsupported")
	}
	for _, c := range p.Checks {
		switch c.ID {
		case CheckDNS, CheckTCP:
			if c.Service != nil && (*c.Service < 0 || *c.Service >= MaxHealthServices) {
				return invalid(t, "check service must be 0–%d", MaxHealthServices-1)
			}
		case CheckDiskWrite, CheckAudioOut, CheckNIC, CheckGPU, CheckKeyboard, CheckMouse:
			if c.Service != nil {
				return invalid(t, "service is only for dns and tcp")
			}
		default:
			return invalid(t, "unknown check id")
		}
		switch c.Status {
		case CheckOK, CheckFail, CheckUnknown:
		default:
			return invalid(t, "check status must be ok, fail or unknown")
		}
		if c.Reason != "" && !checkReasonPattern.MatchString(c.Reason) {
			return invalid(t, "check reason must match ^[a-z_]{1,32}$")
		}
	}
	return nil
}
