//go:build linux

package platform

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

// The connection checker's view of this computer on Linux. Linux networks
// have no Public/Private category, so every one is reported as Private; the
// firewall is ufw or firewalld, read without changing anything. What cannot
// be read without root (ufw's rules) is not guessed: Inspect fails with
// ErrNotSupported and the checker does not judge that step.

// ufwConf is ufw's switch, readable by every user; replaced in tests.
var ufwConf = "/etc/ufw/ufw.conf"

func newNetworkInspector() (NetworkInspector, error) { return linuxNetwork{}, nil }

type linuxNetwork struct{}

func (linuxNetwork) Inspect(ctx context.Context, _ string, port int) (NetworkState, error) {
	st := NetworkState{Networks: upNetworks()}
	if ufwEnabled() {
		out, err := runCommand(ctx, "ufw", "status")
		if err != nil {
			return st, fmt.Errorf("network: ufw is on and its rules need root: %w", ErrNotSupported)
		}
		fw, rules := parseUFWStatus(out, port)
		st.Firewall, st.Rules = []FirewallProfile{fw}, rules
		return st, nil
	}
	if _, err := runCommand(ctx, "firewall-cmd", "--state"); err == nil {
		out, err := runCommand(ctx, "firewall-cmd", "--query-port="+strconv.Itoa(port)+"/tcp")
		answer := strings.TrimSpace(out)
		if err != nil && answer != "no" { // "no" comes with exit status 1
			return st, fmt.Errorf("network: firewalld: %w", ErrNotSupported)
		}
		st.Firewall = []FirewallProfile{{Name: NetworkPrivate, Enabled: true}}
		if answer == "yes" {
			st.Rules = []FirewallRule{{Name: "firewalld " + strconv.Itoa(port) + "/tcp", Allow: true}}
		}
		return st, nil
	}
	st.Firewall = []FirewallProfile{{Name: NetworkPrivate, Enabled: false}}
	return st, nil
}

// upNetworks are the cards that are up with an IPv4 address, loopback out.
func upNetworks() []Network {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []Network
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
				out = append(out, Network{Interface: ifc.Name, Category: NetworkPrivate})
				break
			}
		}
	}
	return out
}

func ufwEnabled() bool {
	f, err := os.Open(ufwConf)
	if err != nil {
		return false
	}
	defer f.Close()
	return ufwConfEnabled(f)
}

// ufwConfEnabled reads ENABLED=yes from ufw.conf.
func ufwConfEnabled(r io.Reader) bool {
	s := bufio.NewScanner(r)
	on := false
	for s.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(s.Text()), "=")
		if ok && k == "ENABLED" {
			on = strings.Trim(v, `"' `) == "yes"
		}
	}
	return on
}

// parseUFWStatus reads "ufw status": active or not, and the rules for the
// console's TCP port (or any port, "Anywhere"). "Default: allow (incoming)"
// only appears in the verbose form, so a plain status means deny.
func parseUFWStatus(out string, port int) (FirewallProfile, []FirewallRule) {
	fw := FirewallProfile{Name: NetworkPrivate}
	var rules []FirewallRule
	p := strconv.Itoa(port)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Status:"):
			fw.Enabled = strings.Contains(line, "active") && !strings.Contains(line, "inactive")
		case strings.HasPrefix(line, "Default:"):
			fw.InboundAllowed = strings.Contains(line, "allow (incoming)")
		default:
			f := strings.Fields(line)
			if len(f) > 1 && f[1] == "(v6)" { // "47830/tcp (v6) ALLOW Anywhere (v6)"
				f = append(f[:1], f[2:]...)
			}
			if len(f) < 3 {
				continue
			}
			target := f[0]
			if target != p && target != p+"/tcp" && target != "Anywhere" {
				continue
			}
			switch strings.ToUpper(f[1]) {
			case "ALLOW":
				rules = append(rules, FirewallRule{Name: "ufw " + target, Allow: true})
			case "DENY", "REJECT":
				rules = append(rules, FirewallRule{Name: "ufw " + target, Allow: false})
			}
		}
	}
	return fw, rules
}
