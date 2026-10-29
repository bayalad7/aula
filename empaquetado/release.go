// Package empaquetado holds what releases need: the release public key, which
// is embedded so that binaries can verify signed releases (auto-update, stage
// 15), and the signing format of checksums.txt. The private key only exists as
// the RELEASE_SIGNING_KEY secret of the repository.
package empaquetado

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "embed"

	"github.com/bayalad7/aula/internal/security"
)

//go:embed llave-releases.pub
var releasePublicKey string

// signLabel separates release signatures from any other use of an Ed25519 key.
const signLabel = "aula-checksums-v1\n"

// ErrBadSignature means checksums.txt was not signed by the release key.
var ErrBadSignature = errors.New("checksums signature does not verify")

// ReleasePublicKey returns the embedded release public key.
func ReleasePublicKey() (ed25519.PublicKey, error) {
	return security.ParsePublicKey(releasePublicKey)
}

// Sign signs the exact bytes of checksums.txt and returns the signature in
// Base64 with a trailing newline, the content of checksums.txt.sig.
func Sign(priv ed25519.PrivateKey, checksums []byte) []byte {
	sig := ed25519.Sign(priv, append([]byte(signLabel), checksums...))
	return []byte(base64.StdEncoding.EncodeToString(sig) + "\n")
}

// Verify checks checksums.txt.sig against checksums.txt with pub.
func Verify(pub ed25519.PublicKey, checksums, sigFile []byte) error {
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigFile)))
	if err != nil {
		return fmt.Errorf("verify: signature is not Base64: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, append([]byte(signLabel), checksums...), sig) {
		return ErrBadSignature
	}
	return nil
}

// Checksums returns checksums.txt for files: one "<sha256 hex>  <base name>"
// line per file, sorted by name (the format of sha256sum).
func Checksums(files []string) ([]byte, error) {
	type line struct{ name, sum string }
	var lines []line
	for _, f := range files {
		h := sha256.New()
		fh, err := os.Open(f)
		if err != nil {
			return nil, fmt.Errorf("checksums: %w", err)
		}
		_, err = io.Copy(h, fh)
		fh.Close()
		if err != nil {
			return nil, fmt.Errorf("checksums %s: %w", f, err)
		}
		lines = append(lines, line{filepath.Base(f), hex.EncodeToString(h.Sum(nil))})
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].name < lines[j].name })
	var b strings.Builder
	for _, l := range lines {
		fmt.Fprintf(&b, "%s  %s\n", l.sum, l.name)
	}
	return []byte(b.String()), nil
}
