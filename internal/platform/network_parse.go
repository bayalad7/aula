package platform

import (
	"encoding/json"
	"fmt"
)

// parseNetworkState decodes the JSON printed by the Windows inspector. It
// lives outside the _windows file so its tests run everywhere.
func parseNetworkState(data []byte) (NetworkState, error) {
	var raw struct {
		Networks []Network         `json:"networks"`
		Firewall []FirewallProfile `json:"firewall"`
		Rules    []struct {
			Name     string `json:"name"`
			Allow    bool   `json:"allow"`
			Profiles int    `json:"profiles"` // bits: 1 Domain, 2 Private, 4 Public
		} `json:"rules"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return NetworkState{}, fmt.Errorf("network inspect: %w", err)
	}
	st := NetworkState{Networks: raw.Networks, Firewall: raw.Firewall}
	for i := range st.Networks {
		st.Networks[i].Category = profileName(st.Networks[i].Category)
	}
	for _, r := range raw.Rules {
		rule := FirewallRule{Name: r.Name, Allow: r.Allow}
		if r.Profiles&7 != 7 { // all three bits (or NET_FW_PROFILE2_ALL) means any
			for _, b := range []struct {
				bit  int
				name string
			}{{1, NetworkDomain}, {2, NetworkPrivate}, {4, NetworkPublic}} {
				if r.Profiles&b.bit != 0 {
					rule.Profiles = append(rule.Profiles, b.name)
				}
			}
		}
		st.Rules = append(st.Rules, rule)
	}
	return st, nil
}

// profileName maps NetworkCategory names to firewall profile names.
func profileName(s string) string {
	if s == "DomainAuthenticated" {
		return NetworkDomain
	}
	return s
}
