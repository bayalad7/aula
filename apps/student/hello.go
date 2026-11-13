package main

import (
	"net"
	"os"
	"os/user"
	"runtime"
	"strings"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/power"
	"github.com/bayalad7/aula/internal/protocol"
)

// agentMACs gives hello.macs: the MAC of the network card that reaches the
// console, so the teacher can turn this computer on with Wake-on-LAN
// (docs/protocol.md §5.15.2). It identifies the computer, not the student
// (docs/privacidad.md rule 31 bis). Where the cards cannot be read, none.
func agentMACs(console net.IP) []string {
	nics, err := platform.Interfaces()
	if err != nil {
		return nil
	}
	return power.AgentMACs(nics, console)
}

// version is the agent's SemVer, sent in hello. Releases set it with
// -ldflags "-X main.version=x.y.z".
var version = "0.1.0-dev"

// localHello describes this machine. The agent always identifies itself with
// its real hostname and user: there is no anonymous or hidden mode.
func localHello() protocol.HelloInfo {
	host, _ := os.Hostname()
	if host == "" {
		host = "desconocido"
	}
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
		if i := strings.LastIndexAny(username, `\/`); i >= 0 {
			username = username[i+1:] // drop the Windows domain
		}
	}
	return protocol.HelloInfo{
		Hostname:     host,
		Username:     username,
		OS:           protocol.OS(runtime.GOOS),
		OSVersion:    runtime.GOOS + "/" + runtime.GOARCH,
		AgentVersion: version,
		// hand and msg.student: the window has the buttons (§5.18); timer:
		// the badge of the strip (§5.19); poll: the poll cards (§5.23); poll.quiz:
		// the quiz cards of several questions (§5.23.1).
		Capabilities: []protocol.Capability{protocol.CapCodeView, protocol.CapMsgShow, protocol.CapHand, protocol.CapMsgStudent, protocol.CapTimer, protocol.CapPoll, protocol.CapPollQuiz, protocol.CapClassEnd},
	}
}
