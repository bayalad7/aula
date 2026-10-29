// Package security holds the console identity (Ed25519), the auth.proof
// signature, the console's self-signed TLS certificate and the certificate
// pinning used by agents (docs/protocol.md §3.2 and §10).
//
// The private key never leaves this package unprotected: Identity does not
// expose it, and it formats and logs as its public fingerprint only.
package security
