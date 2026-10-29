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

// SiteKeyDir holds the site key (clave del centro) on teacher machines, e.g.
// C:\ProgramData\Aula\centro. Installers create it; see RestrictMachineSecret.
func SiteKeyDir() string {
	return filepath.Join(MachineConfigDir(), "Aula", "centro")
}

// ConsoleMachineConfigPath is the console settings the installer writes
// (default channel, port), e.g. C:\ProgramData\Aula\consola\config.json.
func ConsoleMachineConfigPath() string {
	return filepath.Join(MachineConfigDir(), "Aula", "consola", "config.json")
}

// LogDir is where an app writes its log file: %LOCALAPPDATA%\Aula\<app> on
// Windows, the user cache directory elsewhere.
func LogDir(app string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Aula", app), nil
}
