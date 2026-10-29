package session

import (
	"encoding/json"
	"testing"

	"github.com/bayalad7/aula/internal/security"
)

func TestAgentConfig(t *testing.T) {
	id, _ := security.GenerateIdentity()
	key := security.EncodePublicKey(id.Public())
	cfg := AgentConfig{Channel: "1", ConsolePublicKey: key}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	var back AgentConfig
	if err := json.Unmarshal(data, &back); err != nil || back != cfg {
		t.Fatalf("round trip: %+v, %v", back, err)
	}
	if pub, err := back.Validate(); err != nil || !pub.Equal(id.Public()) {
		t.Fatalf("Validate: %v", err)
	}
	for _, bad := range []AgentConfig{
		{Channel: "1"},
		{Channel: "aula 101", ConsolePublicKey: key},
		{Channel: "1", ConsolePublicKey: "aula-ed25519:xx"},
		{Channel: "1", ConsolePublicKey: key, Direct: "10.0.0.5"},
		{Channel: "1", ConsolePublicKey: key, Direct: "10.0.0.5:99999"},
	} {
		if _, err := bad.Marshal(); err == nil {
			t.Errorf("Marshal(%+v) succeeded", bad)
		}
	}
}
