package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/security"
)

func TestConfigureWritesTheMachineFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Aula", "alumno", "config.json")
	old := machineConfigPath
	machineConfigPath = func() string { return path }
	t.Cleanup(func() { machineConfigPath = old })

	id, err := security.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	key := security.EncodePublicKey(id.Public())
	pubFile := filepath.Join(dir, "aula-centro.pub")
	if err := os.WriteFile(pubFile, []byte(key+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := runConfigure([]string{"-canal", "2", "-clave", pubFile}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "canal 2") || !strings.Contains(out.String(), security.KeyFingerprint(id.Public())) {
		t.Errorf("output = %q", out.String())
	}
	var cfg Config
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &cfg); err != nil || cfg.Channel != "2" || cfg.ConsolePublicKey != key {
		t.Fatalf("config = %+v, %v (%s)", cfg, err, data)
	}

	// Changing only the channel keeps the key and the direct address; the
	// key can also be given inline.
	if code := runConfigure([]string{"-directo", "10.0.0.5:47400"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if code := runConfigure([]string{"-canal", "7", "-clave", key}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	data, _ = os.ReadFile(path)
	_ = json.Unmarshal(data, &cfg)
	if cfg.Channel != "7" || cfg.ConsolePublicKey != key || cfg.Direct != "10.0.0.5:47400" {
		t.Fatalf("config = %+v", cfg)
	}
	// loadConfig reads it as the installer's file.
	if got, _, err := loadConfig(nil); err != nil || got.Channel != "7" {
		t.Fatalf("loadConfig = %+v, %v", got, err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("temporary files left: %v", entries)
	}
}

func TestConfigureRefusesInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	old := machineConfigPath
	machineConfigPath = func() string { return path }
	t.Cleanup(func() { machineConfigPath = old })

	for _, args := range [][]string{
		nil,
		{"-canal", "0", "-clave", "aula-ed25519:AAAA"},
		{"-canal", "1", "-clave", "aula-ed25519:no-es-una-clave"},
		{"-canal", "1"}, // no key yet
		{"-canal", "1", "-clave", filepath.Join(t.TempDir(), "no-existe.pub")},
		{"extra"},
	} {
		var out, errOut bytes.Buffer
		if code := runConfigure(args, &out, &errOut); code == 0 {
			t.Errorf("runConfigure(%q) = 0, want an error", args)
		}
		if errOut.Len() == 0 {
			t.Errorf("runConfigure(%q) says nothing", args)
		}
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("an invalid configuration was written")
	}
}

// A service has no user folder (systemd gives root no $HOME): the machine
// file must still be read, or the agent stays "not configured" for ever.
func TestLoadConfigWithoutUserFolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	oldM, oldU := machineConfigPath, userConfigPath
	machineConfigPath = func() string { return path }
	userConfigPath = func() (string, error) { return "", errors.New("neither $XDG_CONFIG_HOME nor $HOME are defined") }
	t.Cleanup(func() { machineConfigPath, userConfigPath = oldM, oldU })
	id, err := security.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := runConfigure([]string{"-canal", "3", "-clave", security.EncodePublicKey(id.Public())}, &out, &errOut); code != 0 {
		t.Fatalf("configure: %s", errOut.String())
	}
	if cfg, _, err := loadConfig(nil); err != nil || cfg.Channel != "3" {
		t.Fatalf("loadConfig = %+v, %v", cfg, err)
	}
}
