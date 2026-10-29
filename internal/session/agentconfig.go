package session

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/security"
)

// AgentConfigFile is the name the console gives the agent configuration it
// exports, and the name the agent installer looks for next to itself.
const AgentConfigFile = "aula-alumno.json"

// AgentConfig is the agent configuration file (docs/configuracion.md). The
// console generates it; the installer copies it to the machine.
type AgentConfig struct {
	Channel          string `json:"channel"`
	ConsolePublicKey string `json:"console_public_key"` // aula-ed25519:…
	Direct           string `json:"direct,omitempty"`   // host:port, to skip mDNS
}

// Validate checks the configuration and returns the parsed console key. Error
// messages are in Spanish: they are shown to whoever installs the agent.
func (c AgentConfig) Validate() (ed25519.PublicKey, error) {
	if c.Channel == "" || c.ConsolePublicKey == "" {
		return nil, fmt.Errorf("falta el canal o la clave de la consola")
	}
	if !protocol.ValidChannel(c.Channel) {
		return nil, fmt.Errorf("canal %q inválido: usa mayúsculas, dígitos y _", c.Channel)
	}
	pub, err := security.ParsePublicKey(c.ConsolePublicKey)
	if err != nil {
		return nil, fmt.Errorf("clave de la consola inválida: %w", err)
	}
	if c.Direct != "" {
		host, port, err := net.SplitHostPort(c.Direct)
		if p, perr := strconv.Atoi(port); err != nil || host == "" || perr != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("dirección directa %q inválida: usa IP:puerto", c.Direct)
		}
	}
	return pub, nil
}

// Marshal renders the file, indented for people who open it.
func (c AgentConfig) Marshal() ([]byte, error) {
	if _, err := c.Validate(); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
