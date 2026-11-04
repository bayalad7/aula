package platform

import "context"

// Firewall profile and network categories, as Windows names them.
const (
	NetworkPublic  = "Public"
	NetworkPrivate = "Private"
	NetworkDomain  = "Domain"
)

// NetworkState is what the connection checker needs to know about this
// computer's networks and firewall (internal/connectcheck decides from it).
type NetworkState struct {
	// Networks are the connected networks.
	Networks []Network `json:"networks"`
	// Firewall has one entry per firewall profile (Domain, Private, Public).
	Firewall []FirewallProfile `json:"firewall"`
	// Rules are the enabled inbound rules for the program or its TCP port.
	Rules []FirewallRule `json:"rules"`
}

// Network is one connected network.
type Network struct {
	Name      string `json:"name"`
	Interface string `json:"interface"`
	Category  string `json:"category"` // NetworkPublic, NetworkPrivate or NetworkDomain
}

// FirewallProfile is the state of one firewall profile.
type FirewallProfile struct {
	Name    string `json:"name"` // NetworkPublic, NetworkPrivate or NetworkDomain
	Enabled bool   `json:"enabled"`
	// InboundAllowed is set when the default inbound action is Allow (rare):
	// then no rule is needed.
	InboundAllowed bool `json:"inboundAllowed"`
}

// FirewallRule is an enabled inbound rule that applies to the console.
type FirewallRule struct {
	Name  string `json:"name"`
	Allow bool   `json:"allow"` // false: it blocks
	// Profiles it applies to; empty means every profile.
	Profiles []string `json:"profiles"`
}

// NetworkInspector reads the network profile and the firewall. It only
// reads: it never changes a rule. Windows today; elsewhere
// NewNetworkInspector returns ErrNotSupported.
type NetworkInspector interface {
	// Inspect returns the connected networks, the firewall profiles and the
	// inbound rules for program (an executable path) or TCP port.
	Inspect(ctx context.Context, program string, port int) (NetworkState, error)
}

// NewNetworkInspector returns the OS network inspector, or ErrNotSupported.
func NewNetworkInspector() (NetworkInspector, error) { return newNetworkInspector() }
