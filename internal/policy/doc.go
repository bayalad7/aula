// Package policy implements the class restrictions (docs/protocol.md
// §5.13-5.14, D-017) without touching the OS: the lists and modes, the app
// enforcer that closes forbidden processes through platform.ProcessControl,
// the local DNS resolver of the web filter, and the journal that undoes every
// change to the computer, also after a crash.
//
// Privacy (docs/privacidad.md rules 20-25): the student's notice is shown
// before a policy is applied; the resolver never records which names were
// asked; every restriction ends with the session.
package policy
