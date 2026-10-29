package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PublicKeyPrefix starts the text form of a console public key.
const PublicKeyPrefix = "aula-ed25519:"

// IdentityFile is the file name of the console private key in its directory.
const IdentityFile = "identidad-ed25519.pem"

// ErrIdentityExists is returned by CreateIdentity when a key is already saved.
var ErrIdentityExists = errors.New("identity already exists")

// Identity is the console's Ed25519 key pair. Its String, GoString, Format and
// LogValue methods only show the public fingerprint, so logging or printing an
// Identity can never leak the private key.
type Identity struct {
	priv ed25519.PrivateKey
}

// GenerateIdentity creates a new random identity (not saved).
func GenerateIdentity() (*Identity, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate identity: %w", err)
	}
	return &Identity{priv: priv}, nil
}

// The methods below use value receivers on purpose: a copied Identity value
// (not only *Identity) must also format without its private key.

// Public returns the public key that agents are configured with.
func (id Identity) Public() ed25519.PublicKey {
	if len(id.priv) != ed25519.PrivateKeySize {
		return nil
	}
	return id.priv.Public().(ed25519.PublicKey)
}

// String implements fmt.Stringer without revealing the private key.
func (id Identity) String() string {
	return "Identity(" + KeyFingerprint(id.Public()) + ")"
}

// GoString implements fmt.GoStringer for %#v.
func (id Identity) GoString() string { return id.String() }

// Format makes every fmt verb (%v, %+v, %x, %s…) print String.
func (id Identity) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, id.String()) }

// LogValue implements slog.LogValuer.
func (id Identity) LogValue() slog.Value {
	return slog.GroupValue(slog.String("public", KeyFingerprint(id.Public())))
}

// EncodePublicKey returns the text form "aula-ed25519:<Base64>" used to
// configure agents.
func EncodePublicKey(pub ed25519.PublicKey) string {
	return PublicKeyPrefix + base64.StdEncoding.EncodeToString(pub)
}

// ParsePublicKey parses the text form produced by EncodePublicKey. Surrounding
// spaces are ignored, since the key is usually pasted.
func ParsePublicKey(s string) (ed25519.PublicKey, error) {
	s = strings.TrimSpace(s)
	b64, ok := strings.CutPrefix(s, PublicKeyPrefix)
	if !ok {
		return nil, fmt.Errorf("parse public key: missing %q prefix", PublicKeyPrefix)
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("parse public key: %d bytes, want %d", len(raw), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}

// KeyFingerprint is a short form of a public key ("XXXX-XXXX-XXXX-XXXX") to
// compare it by voice or at a glance. It is not used for verification.
func KeyFingerprint(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	h := strings.ToUpper(hex.EncodeToString(sum[:8]))
	return h[0:4] + "-" + h[4:8] + "-" + h[8:12] + "-" + h[12:16]
}

// CreateIdentity generates an identity and saves it in dir with owner-only
// access. Unless force is true it fails with ErrIdentityExists when one is
// already saved: replacing it means reconfiguring every agent.
func CreateIdentity(dir string, force bool) (*Identity, error) {
	path := filepath.Join(dir, IdentityFile)
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil, fmt.Errorf("create identity %s: %w", path, ErrIdentityExists)
		}
	}
	id, err := GenerateIdentity()
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(id.priv)
	if err != nil {
		return nil, fmt.Errorf("create identity: %w", err)
	}
	if err := writePrivateFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})); err != nil {
		return nil, fmt.Errorf("create identity: %w", err)
	}
	return id, nil
}

// LoadIdentity reads the identity saved in dir. On Linux and macOS it refuses
// a key file readable by other users, like ssh does.
func LoadIdentity(dir string) (*Identity, error) {
	path := filepath.Join(dir, IdentityFile)
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(path); err == nil && info.Mode().Perm()&0o077 != 0 {
			return nil, fmt.Errorf("load identity: %s is accessible by other users (mode %o); run chmod 600", path, info.Mode().Perm())
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load identity: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("load identity %s: not a PEM private key", path)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("load identity %s: %w", path, err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("load identity %s: key is %T, want Ed25519", path, key)
	}
	return &Identity{priv: priv}, nil
}
