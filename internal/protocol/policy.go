package protocol

import (
	"regexp"
	"strings"
)

// Limits of the app.block.* and web.filter.* messages (docs/protocol.md
// §5.13–5.14, §7).
const (
	MaxBlockedApps  = 100
	MaxFilterDomain = 500
	MaxAppName      = 64
	MaxDomain       = 253
)

// PolicyMode says how a list is read.
type PolicyMode string

// Policy modes.
const (
	PolicyDeny  PolicyMode = "deny"  // the listed ones are blocked
	PolicyAllow PolicyMode = "allow" // only the listed ones are allowed
)

var (
	appNamePattern = regexp.MustCompile(`^[A-Za-z0-9 ._+-]{1,64}$`)
	labelPattern   = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
)

// AppKey is how process names are compared: lowercase, without ".exe", so
// "Chrome", "chrome.exe" and "CHROME.EXE" are the same app.
func AppKey(name string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".exe")
}

// DomainKey is how domains are compared: lowercase, without a final dot.
func DomainKey(domain string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
}

// ValidDomain reports whether d is a host name the filter can use: ASCII
// (punycode for accents), at least two labels, no scheme, path or port.
func ValidDomain(d string) bool {
	d = DomainKey(d)
	if len(d) == 0 || len(d) > MaxDomain {
		return false
	}
	labels := strings.Split(d, ".")
	if len(labels) < 2 {
		return false
	}
	for _, l := range labels {
		if !labelPattern.MatchString(l) {
			return false
		}
	}
	return true
}

func validMode(m PolicyMode) bool { return m == PolicyDeny || m == PolicyAllow }

// AppBlockSet sets the class policy on applications (P → A).
type AppBlockSet struct {
	PolicyID string     `json:"policy_id"`
	Mode     PolicyMode `json:"mode"`
	Apps     []string   `json:"apps"`
}

// MessageType implements Payload.
func (AppBlockSet) MessageType() Type { return TypeAppBlockSet }

// Validate implements Payload.
func (p AppBlockSet) Validate() error {
	if !isUUIDv4(p.PolicyID) {
		return invalid(TypeAppBlockSet, "policy_id is not a UUID v4")
	}
	if !validMode(p.Mode) {
		return invalid(TypeAppBlockSet, "mode must be deny or allow")
	}
	if len(p.Apps) < 1 || len(p.Apps) > MaxBlockedApps {
		return invalid(TypeAppBlockSet, "apps must have 1-%d names", MaxBlockedApps)
	}
	seen := map[string]bool{}
	for _, a := range p.Apps {
		if !appNamePattern.MatchString(a) || AppKey(a) == "" {
			return invalid(TypeAppBlockSet, "app name must be 1-%d characters of letters, digits, space, . _ + -", MaxAppName)
		}
		if seen[AppKey(a)] {
			return invalid(TypeAppBlockSet, "duplicate app")
		}
		seen[AppKey(a)] = true
	}
	return nil
}

// AppBlockClear removes the policy on applications (P → A).
type AppBlockClear struct{}

// MessageType implements Payload.
func (AppBlockClear) MessageType() Type { return TypeAppBlockClear }

// Validate implements Payload.
func (AppBlockClear) Validate() error { return nil }

// WebFilterSet sets the class policy on web sites (P → A).
type WebFilterSet struct {
	PolicyID string     `json:"policy_id"`
	Mode     PolicyMode `json:"mode"`
	Domains  []string   `json:"domains"`
}

// MessageType implements Payload.
func (WebFilterSet) MessageType() Type { return TypeWebFilterSet }

// Validate implements Payload.
func (p WebFilterSet) Validate() error {
	if !isUUIDv4(p.PolicyID) {
		return invalid(TypeWebFilterSet, "policy_id is not a UUID v4")
	}
	if !validMode(p.Mode) {
		return invalid(TypeWebFilterSet, "mode must be deny or allow")
	}
	if len(p.Domains) < 1 || len(p.Domains) > MaxFilterDomain {
		return invalid(TypeWebFilterSet, "domains must have 1-%d names", MaxFilterDomain)
	}
	seen := map[string]bool{}
	for _, d := range p.Domains {
		if !ValidDomain(d) {
			return invalid(TypeWebFilterSet, "invalid domain (use names like example.com, punycode for accents)")
		}
		if seen[DomainKey(d)] {
			return invalid(TypeWebFilterSet, "duplicate domain")
		}
		seen[DomainKey(d)] = true
	}
	return nil
}

// WebFilterClear removes the policy on web sites and restores the computer
// (P → A).
type WebFilterClear struct{}

// MessageType implements Payload.
func (WebFilterClear) MessageType() Type { return TypeWebFilterClear }

// Validate implements Payload.
func (WebFilterClear) Validate() error { return nil }

// PolicyAck confirms a set or a clear (A → P). Active says whether a policy
// of that kind remains applied; PolicyID is the one in force, "" after clear.
type PolicyAck struct {
	RefID    string `json:"ref_id"`
	PolicyID string `json:"policy_id"`
	Active   bool   `json:"active"`
}

func (p PolicyAck) validate(t Type) error {
	if !isUUIDv4(p.RefID) {
		return invalid(t, "ref_id is not a UUID v4")
	}
	if p.Active != isUUIDv4(p.PolicyID) || (!p.Active && p.PolicyID != "") {
		return invalid(t, "policy_id must be a UUID v4 when active and empty otherwise")
	}
	return nil
}

// AppBlockAck acknowledges app.block.set and app.block.clear.
type AppBlockAck PolicyAck

// MessageType implements Payload.
func (AppBlockAck) MessageType() Type { return TypeAppBlockAck }

// Validate implements Payload.
func (p AppBlockAck) Validate() error { return PolicyAck(p).validate(TypeAppBlockAck) }

// WebFilterAck acknowledges web.filter.set and web.filter.clear.
type WebFilterAck PolicyAck

// MessageType implements Payload.
func (WebFilterAck) MessageType() Type { return TypeWebFilterAck }

// Validate implements Payload.
func (p WebFilterAck) Validate() error { return PolicyAck(p).validate(TypeWebFilterAck) }
