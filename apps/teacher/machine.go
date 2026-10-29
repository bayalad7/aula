package main

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/bayalad7/aula/internal/broadcast"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// machineConfig is what the console installer writes for every user of the
// teacher machine (docs/configuracion.md): the channel the console opens on
// start and the server port.
type machineConfig struct {
	DefaultChannel int `json:"default_channel"`
	Port           int `json:"port,omitempty"`
	// BroadcastMaxKbps caps the broadcast of the teacher's screen (D-014);
	// 0 means broadcast.DefaultMaxKbps.
	BroadcastMaxKbps int `json:"broadcast_max_kbps,omitempty"`
}

// machineConfigPath is replaced in tests.
var machineConfigPath = platform.ConsoleMachineConfigPath

// loadMachineConfig reads the machine settings; a missing or damaged file
// means none (the console falls back to the user's last channel, then 1).
func loadMachineConfig() machineConfig {
	var c machineConfig
	if data, err := os.ReadFile(machineConfigPath()); err == nil {
		_ = json.Unmarshal(data, &c)
	}
	return c
}

// defaultChannel is the channel the console opens on start: the installer's,
// else the last one used, else "1".
func defaultChannel(m machineConfig, s settings) string {
	if ch := strconv.Itoa(m.DefaultChannel); protocol.ValidChannel(ch) {
		return ch
	}
	for _, ch := range s.RecentChannels {
		if protocol.ValidChannel(ch) {
			return ch
		}
	}
	return "1"
}

// broadcastKbps is the configured cap, kept at or above broadcast.MinKbps so
// still code stays legible; 0 means the default.
func broadcastKbps(m machineConfig) int {
	if m.BroadcastMaxKbps <= 0 {
		return 0
	}
	return max(m.BroadcastMaxKbps, broadcast.MinKbps)
}

// serverPort is the machine port, else the user's, else DefaultPort.
func serverPort(m machineConfig, s settings) int {
	for _, p := range []int{m.Port, s.Port} {
		if p > 0 && p <= 65535 {
			return p
		}
	}
	return DefaultPort
}
