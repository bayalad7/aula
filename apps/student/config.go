package main

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/session"
)

// Config is the agent configuration (docs/configuracion.md): the classroom
// channel and the only console key it obeys.
type Config = session.AgentConfig

// Config locations; replaced in tests.
var (
	machineConfigPath = platform.AgentMachineConfigPath
	userConfigPath    = func() (string, error) {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("config dir: %w", err)
		}
		return filepath.Join(base, "Aula", "alumno", "config.json"), nil
	}
)

// loadConfig finds the configuration and applies command-line overrides
// (-config, -canal, -clave, -directo). Without -config it uses the first file
// that exists:
//
//  1. the machine file the installer writes (%ProgramData%\Aula\alumno\config.json),
//     which only administrators can change;
//  2. the user file (%AppData%\Aula\alumno\config.json), for manual setups.
//
// The machine file wins so that a student cannot point the agent that starts
// with the session at another console.
func loadConfig(args []string) (Config, ed25519.PublicKey, error) {
	fsFlags := flag.NewFlagSet("aula-cliente", flag.ContinueOnError)
	fsFlags.SetOutput(io.Discard)
	path := fsFlags.String("config", "", "archivo de configuración")
	channel := fsFlags.String("canal", "", "canal del aula")
	key := fsFlags.String("clave", "", "clave pública de la consola")
	direct := fsFlags.String("directo", "", "host:puerto de la consola, sin mDNS")
	if err := fsFlags.Parse(args); err != nil {
		return Config{}, nil, fmt.Errorf("argumentos: %w", err)
	}

	candidates := []string{*path}
	if *path == "" {
		user, err := userConfigPath()
		if err != nil {
			return Config{}, nil, err
		}
		candidates = []string{machineConfigPath(), user}
	}
	var cfg Config
	used := ""
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return Config{}, nil, err
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, nil, fmt.Errorf("%s: %w", p, err)
		}
		used = p
		break
	}
	if *channel != "" {
		cfg.Channel = *channel
	}
	if *key != "" {
		cfg.ConsolePublicKey = *key
	}
	if *direct != "" {
		cfg.Direct = *direct
	}

	pub, err := cfg.Validate()
	if err != nil {
		where := used
		if where == "" {
			where = candidates[0]
		}
		return cfg, nil, fmt.Errorf("%w. Instala el agente con su configuración (aula-alumno.json), revisa %s o usa -canal y -clave", err, where)
	}
	return cfg, pub, nil
}
