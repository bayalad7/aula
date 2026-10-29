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

	"github.com/bayalad7/aula/internal/platform"
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

func TestVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runCLI([]string{"version"}, &out, &errOut); code != 0 || out.String() != "aula-consola "+version+"\n" {
		t.Fatalf("version = %q", out.String())
	}
}

// The site key cycle an administrator runs: create on the first console,
// publish the public key for the students, export, import on a second console.
func TestCentroCommands(t *testing.T) {
	first, second, files := t.TempDir(), t.TempDir(), t.TempDir()
	t.Cleanup(func() { unprotect(first, second) })
	run := func(dir string, args ...string) (string, int) {
		t.Helper()
		t.Setenv("AULA_SITE_KEY_DIR", dir)
		var out, errOut bytes.Buffer
		code := runCLI(append([]string{"centro"}, args...), &out, &errOut)
		return out.String() + errOut.String(), code
	}

	if out, code := run(first, "crear"); code != 0 || !strings.Contains(out, "Clave del centro creada") {
		t.Fatalf("crear: %d %s", code, out)
	}
	if out, code := run(first, "crear"); code != 1 || !strings.Contains(out, "ya tiene clave del centro") {
		t.Fatalf("second crear: %d %s", code, out)
	}
	pubFile := filepath.Join(files, "aula-centro.pub")
	if out, code := run(first, "publica", "-o", pubFile); code != 0 {
		t.Fatalf("publica: %d %s", code, out)
	}
	pubData, _ := os.ReadFile(pubFile)
	pub, err := security.ParsePublicKey(string(pubData))
	if err != nil {
		t.Fatalf("aula-centro.pub: %v", err)
	}
	if strings.Contains(string(pubData), "PRIVATE") {
		t.Fatal("aula-centro.pub contains the private key")
	}
	keyFile := filepath.Join(files, "aula-centro.key")
	if out, code := run(first, "exportar", keyFile); code != 0 || !strings.Contains(out, "SECRETA") {
		t.Fatalf("exportar: %d %s", code, out)
	}
	if out, code := run(second, "importar", keyFile); code != 0 {
		t.Fatalf("importar: %d %s", code, out)
	}
	imported, err := security.LoadSiteKey(second)
	if err != nil || !imported.Public().Equal(pub) {
		t.Fatalf("imported key differs: %v", err)
	}
	if out, code := run(second, "borrar"); code != 1 || !strings.Contains(out, "acción desconocida") {
		t.Fatalf("unknown action: %d %s", code, out)
	}

	// pubkey reports that the console uses the site key.
	t.Setenv("AULA_SITE_KEY_DIR", second)
	var out, errOut bytes.Buffer
	if code := runCLI([]string{"pubkey", "-dir", t.TempDir()}, &out, &errOut); code != 0 ||
		!strings.Contains(out.String(), "clave del centro") || !strings.Contains(out.String(), security.EncodePublicKey(pub)) {
		t.Fatalf("pubkey: %d %s %s", code, out.String(), errOut.String())
	}
}

// unprotect gives the test back access to site key directories (read-only for
// non-administrators by design) so TempDir can delete them.
func unprotect(dirs ...string) {
	for _, d := range dirs {
		_ = platform.RestrictToOwner(d)
		_ = platform.RestrictToOwner(filepath.Join(d, security.SiteKeyFile))
	}
}
