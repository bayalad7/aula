package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/security"
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
