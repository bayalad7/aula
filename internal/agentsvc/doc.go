// Package agentsvc runs the student agent as a service (D-016) without
// touching the OS: Supervisor keeps a helper process in the student's console
// session, Link is the local channel between both, and Clipboard, Capture and
// Indicator let the agent in the service reach the student's screen through
// the helper.
//
// Privacy (docs/privacidad.md rules 17-19): the service has no window and
// cannot capture by itself; everything shown or captured goes through the
// helper, which shows the identity banner and the notices. Without a helper
// the agent declares no capability.
package agentsvc
