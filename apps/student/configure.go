package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bayalad7/aula/internal/security"
)

const configureUsage = `Uso: aula-cliente configurar -canal N -clave aula-centro.pub [-directo IP:puerto]

Escribe la configuración de este equipo (%s), la que usan todas las
sesiones. Requiere administrador (sudo en Linux).

  -canal N        canal de la sala, del 1 al 16000 (1 = centro de cómputo principal)
  -clave CLAVE    archivo aula-centro.pub o la clave misma (aula-ed25519:…)
  -directo H:P    dirección de la consola, solo si el anuncio mDNS no llega
`

// runConfigure is "aula-cliente configurar": the installers' settings page
// for systems without one (the Linux packages, docs/administracion). It
// writes the machine file that loadConfig reads first; an agent already
// running picks it up by itself.
func runConfigure(args []string, stdout, stderr io.Writer) int {
	path := machineConfigPath()
	flags := flag.NewFlagSet("configurar", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	channel := flags.String("canal", "", "")
	key := flags.String("clave", "", "")
	direct := flags.String("directo", "", "")
	if err := flags.Parse(args); err != nil || flags.NArg() > 0 || (*channel == "" && *key == "" && *direct == "") {
		fmt.Fprintf(stderr, configureUsage, path)
		return 2
	}
	cfg, err := readMachineConfig(path)
	if err != nil {
		fmt.Fprintf(stderr, "No se pudo leer %s: %v\n", path, err)
		return 1
	}
	if *channel != "" {
		cfg.Channel = *channel
	}
	if *key != "" {
		k, err := readKeyArg(*key)
		if err != nil {
			fmt.Fprintf(stderr, "No se pudo leer la clave del centro: %v\n", err)
			return 1
		}
		cfg.ConsolePublicKey = k
	}
	if *direct != "" {
		cfg.Direct = *direct
	}
	data, err := cfg.Marshal() // validates
	if err != nil {
		fmt.Fprintf(stderr, "Configuración no válida: %v\n", err)
		return 1
	}
	if err := writeMachineConfig(path, data); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			fmt.Fprintf(stderr, "Sin permiso para escribir %s: ejecuta el comando como administrador (sudo aula-cliente configurar …).\n", path)
		} else {
			fmt.Fprintf(stderr, "No se pudo guardar %s: %v\n", path, err)
		}
		return 1
	}
	pub, _ := cfg.Validate()
	fmt.Fprintf(stdout, "Aula Cliente configurado: canal %s, clave del centro %s.\n", cfg.Channel, security.KeyFingerprint(pub))
	fmt.Fprintf(stdout, "Guardado en %s. El agente abierto lo toma solo; si no, al iniciar la siguiente sesión.\n", path)
	return 0
}

// readMachineConfig reads the machine file, empty if it does not exist yet.
func readMachineConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// readKeyArg takes the key itself or the file aula-centro.pub that holds it.
func readKeyArg(arg string) (string, error) {
	if strings.HasPrefix(arg, "aula-ed25519:") {
		return arg, nil
	}
	data, err := os.ReadFile(arg)
	if err != nil {
		return "", fmt.Errorf("leer %s: %w", arg, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// writeMachineConfig replaces path through a temporary file, so an agent
// that reloads it never reads half a file. The configuration is public (a
// channel and a public key): everyone reads it, only an administrator writes.
func writeMachineConfig(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // after a successful rename it no longer exists
	_, werr := tmp.Write(data)
	if werr == nil {
		werr = tmp.Chmod(0o644)
	}
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return werr
	}
	return os.Rename(tmp.Name(), path)
}
