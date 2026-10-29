package security

import (
	"crypto/ed25519"
	"crypto/rand"

	"github.com/bayalad7/aula/internal/protocol"
)

// proofLabel separates auth.proof signatures from any other use of the key.
const proofLabel = "aula-auth-v1"

// NewNonce returns protocol.NonceSize random bytes for auth.challenge.
func NewNonce() []byte {
	b := make([]byte, protocol.NonceSize)
	_, _ = rand.Read(b) // crypto/rand.Read never fails since Go 1.24.
	return b
}

// proofMessage builds the exact bytes signed in auth.proof (docs/protocol.md §3.2):
// "aula-auth-v1" 0x00 nonce 0x00 channel 0x00 cert_sha256.
func proofMessage(nonce []byte, channel string, certSHA256 [32]byte) []byte {
	m := make([]byte, 0, len(proofLabel)+len(nonce)+len(channel)+len(certSHA256)+3)
	m = append(m, proofLabel...)
	m = append(m, 0)
	m = append(m, nonce...)
	m = append(m, 0)
	m = append(m, channel...)
	m = append(m, 0)
	m = append(m, certSHA256[:]...)
	return m
}

// SignProof signs an agent's challenge, binding it to the channel and to the
// console's TLS certificate.
func (id Identity) SignProof(nonce []byte, channel string, certSHA256 [32]byte) []byte {
	return ed25519.Sign(id.priv, proofMessage(nonce, channel, certSHA256))
}

// VerifyProof checks an auth.proof signature. certSHA256 must be the hash of
// the certificate the agent saw on this connection.
func VerifyProof(pub ed25519.PublicKey, nonce []byte, channel string, certSHA256 [32]byte, sig []byte) bool {
	if len(pub) != ed25519.PublicKeySize || len(nonce) != protocol.NonceSize {
		return false
	}
	return ed25519.Verify(pub, proofMessage(nonce, channel, certSHA256), sig)
}
