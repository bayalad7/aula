package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bayalad7/aula/empaquetado"
)

// The full cycle the release workflow runs: keygen, sums, sign, verify; and
// verification fails on tampering or with another key.
func TestReleaseSigningCycle(t *testing.T) {
	dir := t.TempDir()
	pub := filepath.Join(dir, "llave.pub")
	var priv bytes.Buffer
	if err := run([]string{"keygen", "-pub", pub}, &priv, os.Getenv); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(priv.String(), "PRIVATE KEY") {
		t.Fatal("keygen did not print the private key")
	}
	if data, _ := os.ReadFile(pub); strings.Contains(string(data), "PRIVATE") {
		t.Fatal("the public key file contains the private key")
	}

	a := filepath.Join(dir, "Aula-Alumno-Setup-1.0.0.exe")
	b := filepath.Join(dir, "aula-consola.exe")
	_ = os.WriteFile(a, []byte("setup"), 0o644)
	_ = os.WriteFile(b, []byte("consola"), 0o644)
	sums := filepath.Join(dir, "checksums.txt")
	if err := run([]string{"sums", "-o", sums, b, a}, &bytes.Buffer{}, os.Getenv); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(sums)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[0], "  Aula-Alumno-Setup-1.0.0.exe") || len(strings.Fields(lines[0])[0]) != 64 {
		t.Fatalf("checksums.txt = %q", data)
	}

	env := func(k string) string {
		if k == keyEnv {
			return priv.String()
		}
		return ""
	}
	sig := filepath.Join(dir, "checksums.txt.sig")
	if err := run([]string{"sign", "-in", sums, "-out", sig}, &bytes.Buffer{}, env); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"verify", "-in", sums, "-sig", sig, "-pub", pub}, &out, os.Getenv); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !strings.HasPrefix(out.String(), "OK:") {
		t.Fatalf("verify output %q", out.String())
	}

	// Tampered checksums.
	_ = os.WriteFile(sums, append(data, []byte("0000  extra.exe\n")...), 0o644)
	if err := run([]string{"verify", "-in", sums, "-sig", sig, "-pub", pub}, &out, os.Getenv); !errors.Is(err, empaquetado.ErrBadSignature) {
		t.Fatalf("tampered file: %v", err)
	}
	_ = os.WriteFile(sums, data, 0o644)

	// Another key.
	other := filepath.Join(dir, "otra.pub")
	if err := run([]string{"keygen", "-pub", other}, &bytes.Buffer{}, os.Getenv); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"verify", "-in", sums, "-sig", sig, "-pub", other}, &out, os.Getenv); !errors.Is(err, empaquetado.ErrBadSignature) {
		t.Fatalf("other key: %v", err)
	}

	// Missing secret.
	if err := run([]string{"sign", "-in", sums, "-out", sig}, &bytes.Buffer{}, func(string) string { return "" }); err == nil {
		t.Fatal("sign without the secret succeeded")
	}
}

func TestEmbeddedKeyIsValid(t *testing.T) {
	if _, err := empaquetado.ReleasePublicKey(); err != nil {
		t.Fatalf("empaquetado/llave-releases.pub: %v", err)
	}
}
