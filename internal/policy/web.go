package policy

import (
	"strings"

	"github.com/bayalad7/aula/internal/protocol"
)

// alwaysBlocked are blocked while the web filter is on, whatever the list:
// the canary that tells Firefox to turn its default DNS over HTTPS off, and
// well-known DNS over HTTPS servers, which would skip the filter (D-017).
var alwaysBlocked = []string{
	"use-application-dns.net",
	"dns.google", "dns.google.com", "cloudflare-dns.com", "mozilla.cloudflare-dns.com",
	"chrome.cloudflare-dns.com", "one.one.one.one", "dns.quad9.net", "doh.opendns.com",
	"dns.nextdns.io", "doh.cleanbrowsing.org", "dns.adguard.com", "dns.adguard-dns.com", "doh.dns.sb",
}

// alwaysAllowed are answered even with an allow list, so Windows keeps
// working: reverse lookups, single-label names, Windows' connectivity check.
// The computer's own DNS suffixes (its school domain) are added at runtime.
var alwaysAllowed = []string{
	"in-addr.arpa", "ip6.arpa", "local",
	"msftconnecttest.com", "msftncsi.com",
}

// WebPolicy is a web.filter.set in force.
type WebPolicy struct {
	ID      string
	Mode    protocol.PolicyMode
	Domains []string
	set     map[string]bool
}

// NewWebPolicy builds the policy of a valid web.filter.set.
func NewWebPolicy(p protocol.WebFilterSet) *WebPolicy {
	w := &WebPolicy{ID: p.PolicyID, Mode: p.Mode, Domains: p.Domains, set: map[string]bool{}}
	for _, d := range p.Domains {
		w.set[protocol.DomainKey(d)] = true
	}
	return w
}

// under reports whether name is one of domains or a subdomain of one.
func under(name string, domains map[string]bool) bool {
	for {
		if domains[name] {
			return true
		}
		i := strings.IndexByte(name, '.')
		if i < 0 {
			return false
		}
		name = name[i+1:]
	}
}

func setOf(list []string) map[string]bool {
	m := make(map[string]bool, len(list))
	for _, d := range list {
		m[protocol.DomainKey(d)] = true
	}
	return m
}

var (
	alwaysBlockedSet = setOf(alwaysBlocked)
	alwaysAllowedSet = setOf(alwaysAllowed)
)

// Blocks reports whether a DNS query for name must be refused. localSuffixes
// are the computer's DNS suffixes, always allowed. A nil policy blocks only
// the DNS over HTTPS servers (it is not called without a filter on).
func (w *WebPolicy) Blocks(name string, localSuffixes map[string]bool) bool {
	name = protocol.DomainKey(name)
	if name == "" || under(name, alwaysBlockedSet) {
		return name != ""
	}
	if w == nil {
		return false
	}
	listed := under(name, w.set)
	if w.Mode == protocol.PolicyDeny {
		return listed
	}
	if listed || !strings.Contains(name, ".") || under(name, alwaysAllowedSet) || under(name, localSuffixes) {
		return false
	}
	return true
}

// Summary is what the student's notice shows of a list: the first items and
// how many there are (docs/privacidad.md rule 21).
type Summary struct {
	PolicyID string              `json:"policyId"`
	Mode     protocol.PolicyMode `json:"mode"`
	Items    []string            `json:"items"`
	Total    int                 `json:"total"`
}

func summarize(id string, mode protocol.PolicyMode, items []string) *Summary {
	s := &Summary{PolicyID: id, Mode: mode, Total: len(items)}
	s.Items = append([]string{}, items[:min(len(items), maxSummaryItems)]...)
	return s
}
