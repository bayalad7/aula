// Package discovery announces and discovers Aula sessions on the LAN via mDNS
// (DNS-SD service "_aula._tcp"), filtering by classroom channel, as described in
// docs/protocol.md §1.1.
//
// mDNS is not trusted: an announcement only tells an agent where a console with
// its channel might be. Trust comes from the Ed25519 authentication.
package discovery
