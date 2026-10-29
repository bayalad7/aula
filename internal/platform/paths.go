package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// MachineConfigDir is the system-wide configuration directory, writable only
// by administrators: %ProgramData% on Windows, /Library/Application Support
// on macOS and /etc elsewhere. Installers put machine settings under it.
func MachineConfigDir() string {
	switch runtime.GOOS {
	case "windows":
		if d := os.Getenv("ProgramData"); d != "" {
			return d
		}
		return `C:\ProgramData`
	case "darwin":
		return "/Library/Application Support"
	default:
		return "/etc"
	}
}

// AgentMachineConfigPath is where the agent installer writes the agent
// configuration, e.g. C:\ProgramData\Aula\alumno\config.json.
func AgentMachineConfigPath() string {
	return filepath.Join(MachineConfigDir(), "Aula", "alumno", "config.json")
}
