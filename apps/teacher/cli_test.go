package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/security"
	"github.com/bayalad7/aula/internal/session"
)

func TestKeygenPrintsPublicKeyOnly(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	if code := runCLI([]string{"keygen", "-dir", dir}, &out, &errOut); code != 0 {
		t.Fatalf("keygen exit %d: %s", code, errOut.String())
	}
	id, err := security.LoadIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), security.EncodePublicKey(id.Public())) {
		t.Fatalf("public key not printed:\n%s", out.String())
	}

	data, _ := os.ReadFile(filepath.Join(dir, security.IdentityFile))
	block, _ := pem.Decode(data)
	key, _ := x509.ParsePKCS8PrivateKey(block.Bytes)
	seed := key.(ed25519.PrivateKey).Seed()
	all := out.String() + errOut.String()
	for _, secret := range []string{
		base64.StdEncoding.EncodeToString(block.Bytes)[:40],
		base64.StdEncoding.EncodeToString(seed)[:40],
		hex.EncodeToString(seed),
		"PRIVATE KEY",
	} {
		if strings.Contains(all, secret) {
			t.Fatalf("keygen output contains private key material %q", secret)
		}
	}

	// A second keygen keeps the key, shows the same public key and fails.
	out.Reset()
	errOut.Reset()
	if code := runCLI([]string{"keygen", "-dir", dir}, &out, &errOut); code != 1 {
		t.Fatalf("second keygen exit %d, want 1", code)
	}
	again, _ := security.LoadIdentity(dir)
	if !again.Public().Equal(id.Public()) || !strings.Contains(out.String(), security.EncodePublicKey(id.Public())) {
		t.Fatal("existing key was replaced or not shown")
	}

	out.Reset()
	if code := runCLI([]string{"pubkey", "-dir", dir}, &out, &errOut); code != 0 ||
		!strings.Contains(out.String(), security.EncodePublicKey(id.Public())) {
		t.Fatalf("pubkey exit %d:\n%s", code, out.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runCLI([]string{"borrar-todo"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "Uso:") {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
}

func TestConfigAlumnoAndVersion(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	if code := runCLI([]string{"keygen", "-dir", dir}, &out, &errOut); code != 0 {
		t.Fatal(errOut.String())
	}
	file := filepath.Join(t.TempDir(), "aula-alumno.json")
	if code := runCLI([]string{"config-alumno", "-dir", dir, "-canal", "aula_101", "-o", file}, &out, &errOut); code != 0 {
		t.Fatalf("config-alumno exit %d: %s", code, errOut.String())
	}
	data, _ := os.ReadFile(file)
	var cfg session.AgentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	id, _ := security.LoadIdentity(dir)
	if pub, err := cfg.Validate(); err != nil || cfg.Channel != "AULA_101" || !pub.Equal(id.Public()) {
		t.Fatalf("generated config = %+v (%v)", cfg, err)
	}
	if strings.Contains(string(data), "PRIVATE") {
		t.Fatal("agent config contains private key material")
	}
	errOut.Reset()
	if code := runCLI([]string{"config-alumno", "-dir", dir, "-o", file}, &out, &errOut); code != 1 {
		t.Fatal("config-alumno without channel succeeded")
	}

	out.Reset()
	if code := runCLI([]string{"version"}, &out, &errOut); code != 0 || out.String() != "aula-consola "+version+"\n" {
		t.Fatalf("version = %q", out.String())
	}
}
