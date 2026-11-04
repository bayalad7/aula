package platform

import (
	"reflect"
	"testing"
)

func TestParseNetworkState(t *testing.T) {
	// As printed by the Windows script; single results are still arrays.
	data := `{"networks":[{"name":"Red 2","interface":"Ethernet","category":"DomainAuthenticated"}],` +
		`"firewall":[{"name":"Domain","enabled":true,"inboundAllowed":false},{"name":"Public","enabled":false,"inboundAllowed":false}],` +
		`"rules":[{"name":"Aula Consola","allow":true,"profiles":3},{"name":"todas","allow":false,"profiles":2147483647}]}`
	st, err := parseNetworkState([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	want := NetworkState{
		Networks: []Network{{Name: "Red 2", Interface: "Ethernet", Category: NetworkDomain}},
		Firewall: []FirewallProfile{{Name: NetworkDomain, Enabled: true}, {Name: NetworkPublic}},
		Rules: []FirewallRule{
			{Name: "Aula Consola", Allow: true, Profiles: []string{NetworkDomain, NetworkPrivate}},
			{Name: "todas"},
		},
	}
	if !reflect.DeepEqual(st, want) {
		t.Fatalf("got  %+v\nwant %+v", st, want)
	}
	if _, err := parseNetworkState([]byte("no es json")); err == nil {
		t.Fatal("bad output accepted")
	}
}
