package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"slices"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/session"
)

// errNotConnected is a target that is no longer connected.
var errNotConnected = fmt.Errorf("el alumno ya no está conectado")

// sendToTargets sends p to the given sessions, real or simulated, and splits
// what was sent so simulated students are counted apart.
func sendToTargets(ctx context.Context, class classroom, p protocol.Payload, targets []string) (sent, simSent map[string]string, failed map[string]error) {
	sent, simSent, failed = map[string]string{}, map[string]string{}, map[string]error{}
	connected := func(list []session.Session) map[string]bool {
		out := map[string]bool{}
		for _, s := range list {
			if s.State == session.StateConnected {
				out[s.ID] = true
			}
		}
		return out
	}
	real, sims := connected(class.Sessions()), connected(simSessionsOf(class))
	for _, id := range targets {
		if !real[id] && !sims[id] {
			failed[id] = errNotConnected
			continue
		}
		envID, err := class.Send(ctx, id, p)
		switch {
		case err != nil:
			failed[id] = err
		case sims[id]:
			simSent[id] = envID
		default:
			sent[id] = envID
		}
	}
	return sent, simSent, failed
}

// rememberPayload keeps what a delivery sent while it is among the recent
// ones, so Resend can send it again; a.mu is held.
func (a *App) rememberPayload(deliveryID string, p protocol.Payload) {
	if a.payloads == nil {
		a.payloads = map[string]protocol.Payload{}
	}
	a.payloads[deliveryID] = p
	recent := a.deliveries.Recent()
	for id := range a.payloads {
		if !slices.ContainsFunc(recent, func(d codedist.Delivery) bool { return d.ID == id }) {
			delete(a.payloads, id)
		}
	}
}

// InstallerInfo is what whoever installs the lab needs to know about this
// console (Ajustes › Acerca de and «¿Falta un alumno?»).
type InstallerInfo struct {
	Channel     string   `json:"channel"`
	ConsoleName string   `json:"consoleName"`
	Hostname    string   `json:"hostname"`
	Addresses   []string `json:"addresses"` // IPv4 of this computer, without loopback
	Port        int      `json:"port"`      // TCP, WebSocket over TLS
	MediaPort   int      `json:"mediaPort"` // UDP, the broadcast video
	Fingerprint string   `json:"fingerprint"`
	KeySource   string   `json:"keySource"` // "centro" or "personal"
	Version     string   `json:"version"`
}

// InstallerInfo returns the data of the open class for the installer.
func (a *App) InstallerInfo() InstallerInfo {
	a.mu.Lock()
	info := a.info
	a.mu.Unlock()
	host, _ := os.Hostname()
	out := InstallerInfo{
		Channel: info.Channel, ConsoleName: info.ConsoleName, Hostname: host, Port: info.Port,
		MediaPort: a.broadcastUDPPort, Fingerprint: info.Fingerprint, KeySource: info.KeySource,
		Version: version, Addresses: []string{},
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, ad := range addrs {
			ipn, ok := ad.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil || ipn.IP.IsLoopback() || ipn.IP.IsLinkLocalUnicast() {
				continue
			}
			out.Addresses = append(out.Addresses, ipn.IP.String())
		}
	}
	return out
}
