package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Certificate file names in the console directory.
const (
	CertFile    = "tls-cert.pem"
	CertKeyFile = "tls-key.pem"
)

const fingerprintPrefix = "sha256:"

// ErrFingerprintMismatch means the console presented a certificate other than
// the one it announced.
var ErrFingerprintMismatch = errors.New("certificate does not match the announced fingerprint")

// Certificate is the console's TLS certificate and its SHA-256 fingerprint.
type Certificate struct {
	TLS    tls.Certificate
	SHA256 [32]byte
}

// Fingerprint returns the "sha256:<hex>" form announced in mDNS.
func (c Certificate) Fingerprint() string { return FormatFingerprint(c.SHA256) }

// FormatFingerprint renders a certificate hash as "sha256:<64 hex>".
func FormatFingerprint(sum [32]byte) string { return fingerprintPrefix + hex.EncodeToString(sum[:]) }

// ParseFingerprint parses "sha256:<64 hex>".
func ParseFingerprint(s string) ([32]byte, error) {
	var sum [32]byte
	h, ok := strings.CutPrefix(s, fingerprintPrefix)
	if !ok || len(h) != 64 {
		return sum, fmt.Errorf("parse fingerprint: want sha256:<64 hex>")
	}
	if _, err := hex.Decode(sum[:], []byte(h)); err != nil {
		return sum, fmt.Errorf("parse fingerprint: %w", err)
	}
	return sum, nil
}

// LoadOrCreateCertificate returns the console certificate saved in dir,
// creating a self-signed one (ECDSA P-256, 10 years) on first start. The
// private key file gets owner-only access.
func LoadOrCreateCertificate(dir string) (Certificate, error) {
	certPath, keyPath := filepath.Join(dir, CertFile), filepath.Join(dir, CertKeyFile)
	if _, err := os.Stat(certPath); err == nil {
		pair, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return Certificate{}, fmt.Errorf("load certificate: %w", err)
		}
		return Certificate{TLS: pair, SHA256: sha256.Sum256(pair.Certificate[0])}, nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Certificate{}, fmt.Errorf("create certificate: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return Certificate{}, fmt.Errorf("create certificate: %w", err)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Aula consola", Organization: []string{"Aula"}},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return Certificate{}, fmt.Errorf("create certificate: %w", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return Certificate{}, fmt.Errorf("create certificate: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := writePrivateFile(keyPath, keyPEM); err != nil {
		return Certificate{}, fmt.Errorf("create certificate: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil { // public
		return Certificate{}, fmt.Errorf("create certificate: %w", err)
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return Certificate{}, fmt.Errorf("create certificate: %w", err)
	}
	return Certificate{TLS: pair, SHA256: sha256.Sum256(der)}, nil
}

// ServerTLS returns the console's TLS configuration (TLS 1.3 only).
func ServerTLS(c Certificate) *tls.Config {
	return &tls.Config{Certificates: []tls.Certificate{c.TLS}, MinVersion: tls.VersionTLS13}
}

// ClientTLS returns an agent TLS configuration for one connection attempt.
// The console certificate is self-signed, so it is checked by fingerprint
// instead of against certificate authorities: if fingerprint is not empty the
// peer certificate must match it (pinning). Either way the hash of the
// certificate actually presented is stored in seen during the handshake, to be
// used when verifying auth.proof (docs/protocol.md §10.2).
func ClientTLS(fingerprint string, seen *[32]byte) (*tls.Config, error) {
	var want [32]byte
	pinned := fingerprint != ""
	if pinned {
		var err error
		if want, err = ParseFingerprint(fingerprint); err != nil {
			return nil, err
		}
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		// Chain and hostname checks are replaced by VerifyConnection below,
		// which Go runs even when this is set.
		InsecureSkipVerify: true, //nolint:gosec // certificate pinning, see doc comment
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("console sent no certificate")
			}
			sum := sha256.Sum256(cs.PeerCertificates[0].Raw)
			if pinned && sum != want {
				return ErrFingerprintMismatch
			}
			*seen = sum
			return nil
		},
	}, nil
}
