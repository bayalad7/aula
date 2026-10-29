package main

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// machineConfig is what the console installer writes for every user of the
// teacher machine (docs/configuracion.md): the channel the console opens on
// start and the server port.
type machineConfig struct {
	DefaultChannel int `json:"default_channel"`
	Port           int `json:"port,omitempty"`
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

// serverPort is the machine port, else the user's, else DefaultPort.
func serverPort(m machineConfig, s settings) int {
	for _, p := range []int{m.Port, s.Port} {
		if p > 0 && p <= 65535 {
			return p
		}
	}
	return DefaultPort
}
