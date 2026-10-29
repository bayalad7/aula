package main

import (
	"os"
	"os/user"
	"runtime"
	"strings"

	"github.com/bayalad7/aula/internal/protocol"
)

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
		Capabilities: []protocol.Capability{protocol.CapCodeView, protocol.CapMsgShow},
	}
}
