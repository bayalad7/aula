// Package openapp opens a web page or an allowed app in the student's session
// (open.*, docs/protocol.md §5.22; docs/privacidad.md rules 66–70).
//
// It never runs a command: open.url takes only a validated http/https URL
// (protocol.CheckOpenURL), and open.app only an .exe already installed under
// Program Files, Windows or %LOCALAPPDATA%\Programs, outside the user folders
// and never an interpreter, started with no arguments. Those rules are fixed
// here, in the agent; no console can change them.
//
// Agent side: Opener answers open.url and open.app, at most 3 in a row and
// then 1 every 3 s, and reports what opened for the window's corner notice.
// It opens through a Session: Local (window mode, or the helper in the
// student's session) or the service's proxy to the helper (internal/agentsvc),
// which checks everything again.
//
// Console side: Catalog is the list of allowed apps the administrator edits
// in Ajustes (with examples), and Tracker counts each order's acks.
//
// The package calls no OS API: folders, links and launching go through
// platform.Launcher.
package openapp
