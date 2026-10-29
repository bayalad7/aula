package security

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPublicKeyTextRoundTrip(t *testing.T) {
	id, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	text := EncodePublicKey(id.Public())
	if !strings.HasPrefix(text, PublicKeyPrefix) || len(text) != len(PublicKeyPrefix)+44 {
		t.Fatalf("EncodePublicKey = %q", text)
	}
	got, err := ParsePublicKey("  " + text + "\n")
	if err != nil || !got.Equal(id.Public()) {
		t.Fatalf("ParsePublicKey = %v, %v", got, err)
	}
	for _, bad := range []string{"", "ed25519:" + text[len(PublicKeyPrefix):], PublicKeyPrefix + "!!", PublicKeyPrefix + "AAAA"} {
		if _, err := ParsePublicKey(bad); err == nil {
			t.Errorf("ParsePublicKey(%q) succeeded", bad)
		}
	}
	if fp := KeyFingerprint(id.Public()); len(fp) != 19 || strings.Count(fp, "-") != 3 {
		t.Fatalf("KeyFingerprint = %q", fp)
	}
}

func TestCreateLoadIdentity(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "consola")
	id, err := CreateIdentity(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateIdentity(dir, false); !errors.Is(err, ErrIdentityExists) {
		t.Fatalf("second CreateIdentity: %v, want ErrIdentityExists", err)
	}
	loaded, err := LoadIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Public().Equal(id.Public()) {
		t.Fatal("loaded identity differs")
	}
	replaced, err := CreateIdentity(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Public().Equal(id.Public()) {
		t.Fatal("force did not create a new key")
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(filepath.Join(dir, IdentityFile))
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("key file mode %o, want 600", info.Mode().Perm())
		}
		_ = os.Chmod(filepath.Join(dir, IdentityFile), 0o644)
		if _, err := LoadIdentity(dir); err == nil {
			t.Fatal("LoadIdentity accepted a world-readable key")
		}
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Fatalf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestIdentityNeverFormatsPrivateKey(t *testing.T) {
	id, _ := GenerateIdentity()
	seedHex := fmt.Sprintf("%x", []byte(id.priv.Seed()))
	var logBuf bytes.Buffer
	slog.New(slog.NewJSONHandler(&logBuf, nil)).Info("x", "id", id)
	outputs := []string{
		fmt.Sprintf("%v", id), fmt.Sprintf("%+v", id), fmt.Sprintf("%#v", id),
		fmt.Sprintf("%s", id), fmt.Sprintf("%x", id), fmt.Sprintf("%q", id),
		fmt.Sprint(*id), logBuf.String(),
	}
	for _, out := range outputs {
		if strings.Contains(out, seedHex) || strings.Contains(out, fmt.Sprint([]byte(id.priv))) {
			t.Fatalf("private key leaked in %q", out)
		}
	}
	if !strings.Contains(logBuf.String(), KeyFingerprint(id.Public())) {
		t.Fatalf("log does not identify the key by fingerprint: %s", logBuf.String())
	}
}

func TestProofVerification(t *testing.T) {
	id, _ := GenerateIdentity()
	other, _ := GenerateIdentity()
	nonce := NewNonce()
	cert := [32]byte{1, 2, 3}
	sig := id.SignProof(nonce, "AULA_101", cert)

	if !VerifyProof(id.Public(), nonce, "AULA_101", cert, sig) {
		t.Fatal("valid proof rejected")
	}
	otherCert := cert
	otherCert[0] = 9
	cases := map[string]bool{
		"other key":     VerifyProof(other.Public(), nonce, "AULA_101", cert, sig),
		"other nonce":   VerifyProof(id.Public(), NewNonce(), "AULA_101", cert, sig),
		"other channel": VerifyProof(id.Public(), nonce, "AULA_102", cert, sig),
		"other cert":    VerifyProof(id.Public(), nonce, "AULA_101", otherCert, sig),
		"short nonce":   VerifyProof(id.Public(), nonce[:16], "AULA_101", cert, sig),
		"bad signature": VerifyProof(id.Public(), nonce, "AULA_101", cert, sig[:63]),
	}
	for name, ok := range cases {
		if ok {
			t.Errorf("%s: proof accepted", name)
		}
	}
	if bytes.Equal(NewNonce(), NewNonce()) {
		t.Fatal("nonces repeat")
	}
}

func TestCertificateIsPersisted(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreateCertificate(dir)
	if err != nil {
		t.Fatal(err)
	}
	again, err := LoadOrCreateCertificate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first.SHA256 != again.SHA256 {
		t.Fatal("certificate regenerated on second start")
	}
	sum, err := ParseFingerprint(first.Fingerprint())
	if err != nil || sum != first.SHA256 {
		t.Fatalf("fingerprint round trip: %v", err)
	}
	for _, bad := range []string{"", "sha256:00", "md5:" + strings.Repeat("0", 64), "sha256:" + strings.Repeat("g", 64)} {
		if _, err := ParseFingerprint(bad); err == nil {
			t.Errorf("ParseFingerprint(%q) succeeded", bad)
		}
	}
}

// handshake runs a TLS handshake between ServerTLS(cert) and the given client
// config over loopback TCP (net.Pipe is unbuffered and deadlocks TLS 1.3).
func handshake(t *testing.T, cert Certificate, client *tls.Config) (tls.ConnectionState, error) {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", ServerTLS(cert))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		if c, err := ln.Accept(); err == nil {
			_ = c.(*tls.Conn).Handshake()
			c.Close()
		}
	}()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	cl := tls.Client(conn, client)
	defer cl.Close()
	err = cl.Handshake()
	return cl.ConnectionState(), err
}

func TestClientTLSPinning(t *testing.T) {
	cert, err := LoadOrCreateCertificate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	other, err := LoadOrCreateCertificate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	var seen [32]byte
	conf, _ := ClientTLS(cert.Fingerprint(), &seen)
	state, err := handshake(t, cert, conf)
	if err != nil {
		t.Fatalf("pinned handshake: %v", err)
	}
	if state.Version != tls.VersionTLS13 || seen != cert.SHA256 {
		t.Fatalf("version %x, seen matches=%v", state.Version, seen == cert.SHA256)
	}

	var seen2 [32]byte
	conf, _ = ClientTLS(other.Fingerprint(), &seen2)
	if _, err := handshake(t, cert, conf); !errors.Is(err, ErrFingerprintMismatch) {
		t.Fatalf("mismatched fingerprint: %v, want ErrFingerprintMismatch", err)
	}
	if seen2 != ([32]byte{}) {
		t.Fatal("hash of a rejected certificate was recorded")
	}

	var seen3 [32]byte
	conf, _ = ClientTLS("", &seen3) // direct connection: accept, but record the hash
	if _, err := handshake(t, cert, conf); err != nil || seen3 != cert.SHA256 {
		t.Fatalf("unpinned handshake: %v", err)
	}

	if _, err := ClientTLS("sha256:zz", &seen3); err == nil {
		t.Fatal("ClientTLS accepted a malformed fingerprint")
	}
}
