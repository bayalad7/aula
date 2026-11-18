package security

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What the administrator sees when the key file is wrong: an error, never a
// half-loaded identity, and never the key bytes in the message.
func TestLoadIdentityRefusesBadFiles(t *testing.T) {
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecDER, err := x509.MarshalPKCS8PrivateKey(ec)
	if err != nil {
		t.Fatal(err)
	}
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	good, err := keyPEM(mustIdentity(t))
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"empty":       {},
		"not PEM":     []byte("esto no es una clave"),
		"truncated":   good[:len(good)/2],
		"public key":  pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}),
		"ECDSA":       pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ecDER}),
		"bad PKCS#8":  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{0x30, 0x03, 0x02, 0x01, 0x00}}),
		"wrong label": pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecDER}),
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, IdentityFile), data, 0o600); err != nil {
			t.Fatal(err)
		}
		id, err := LoadIdentity(dir)
		if err == nil || id != nil {
			t.Errorf("%s: LoadIdentity = %v, %v; want an error", name, id, err)
			continue
		}
		if len(data) > 0 && strings.Contains(err.Error(), string(data)) {
			t.Errorf("%s: the error carries the file: %v", name, err)
		}
	}
	if _, err := LoadIdentity(t.TempDir()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file: %v, want fs.ErrNotExist", err)
	}
}

// A secret is never left half-written or unrestricted: if restricting the
// folder or the temporary file fails, nothing is in place and no temporary
// file stays behind.
func TestWriteSecretFileFailures(t *testing.T) {
	boom := errors.New("restrict failed")
	for name, failOn := range map[string]int{"folder": 1, "temporary file": 2} {
		dir := filepath.Join(t.TempDir(), "consola")
		path := filepath.Join(dir, IdentityFile)
		calls := 0
		restrict := func(string) error {
			calls++
			if calls == failOn {
				return boom
			}
			return nil
		}
		if err := writeSecretFile(path, []byte("secreto"), restrict); !errors.Is(err, boom) {
			t.Fatalf("%s: err = %v, want the restrict error", name, err)
		}
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%s: the secret is in place after a failure (%v)", name, err)
		}
		if left, _ := filepath.Glob(filepath.Join(dir, ".tmp-*")); len(left) != 0 {
			t.Fatalf("%s: temporary files left: %v", name, left)
		}
	}
	// The folder cannot be created: a file is in its place.
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, "consola"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeSecretFile(filepath.Join(parent, "consola", IdentityFile), []byte("x"), func(string) error { return nil }); err == nil {
		t.Fatal("writing under a file succeeded")
	}
	if err := writeMachineSecret(filepath.Join(parent, "consola", "clave-centro.pem"), []byte("x")); err == nil {
		t.Fatal("machine secret under a file succeeded")
	}
}

func mustIdentity(t *testing.T) *Identity {
	t.Helper()
	id, err := CreateIdentity(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
