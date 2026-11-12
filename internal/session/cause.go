package session

// Cause classifies why an agent is not connected, the same set of answers
// as the "¿Falta un alumno?" diagram (docs/diagramas/12): the agent
// reports it with its diagnostic text, and the console's connection checker
// (internal/connectcheck) gives the same cause for the same failure.
type Cause string

// Causes, from the student's side.
const (
	CauseNone           Cause = ""               // connected
	CauseNoConsole      Cause = "noConsole"      // no console of the channel announced
	CauseUnreachable    Cause = "unreachable"    // announced, but no address answers
	CauseCertMismatch   Cause = "certMismatch"   // the certificate is not the announced one
	CauseWrongKey       Cause = "wrongKey"       // the proof does not verify with the site key
	CauseAuthIncomplete Cause = "authIncomplete" // the handshake did not finish
)
