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

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
)

// Config is the agent configuration: the classroom channel and the only
// console key it obeys. The installer writes it (stage 08); until then it can
// be written by hand or given as flags.
type Config struct {
	Channel          string `json:"channel"`
	ConsolePublicKey string `json:"console_public_key"` // aula-ed25519:…
	Direct           string `json:"direct,omitempty"`   // host:port, to skip mDNS
}

// DefaultConfigPath is %AppData%\Aula\alumno\config.json on Windows (and the
// equivalent user config dir elsewhere).
func DefaultConfigPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(base, "Aula", "alumno", "config.json"), nil
}

// loadConfig reads the config file and applies command-line overrides:
// -config, -canal, -clave, -directo. The returned key is parsed and checked.
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

	var cfg Config
	if *path == "" {
		p, err := DefaultConfigPath()
		if err != nil {
			return Config{}, nil, err
		}
		*path = p
	}
	data, err := os.ReadFile(*path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, nil, fmt.Errorf("%s: %w", *path, err)
		}
	case errors.Is(err, fs.ErrNotExist):
		// Flags alone are enough.
	default:
		return Config{}, nil, err
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

	if cfg.Channel == "" || cfg.ConsolePublicKey == "" {
		return cfg, nil, fmt.Errorf("falta el canal o la clave de la consola: créalos en %s o usa -canal y -clave", *path)
	}
	if !protocol.ValidChannel(cfg.Channel) {
		return cfg, nil, fmt.Errorf("canal %q inválido: usa mayúsculas, dígitos y _", cfg.Channel)
	}
	pub, err := security.ParsePublicKey(cfg.ConsolePublicKey)
	if err != nil {
		return cfg, nil, fmt.Errorf("clave de la consola inválida: %w", err)
	}
	return cfg, pub, nil
}
