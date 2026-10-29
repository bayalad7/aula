package main

import "github.com/bayalad7/aula/internal/codedist"

// UI contract between this Go backend and frontend/dist (documented in
// docs/eventos-ui.md). Event names must match frontend/dist/events.js; a test
// checks it.
//
// Backend → frontend: runtime.EventsEmit with one of the events below.
// Frontend → backend: the bound methods of App (View, Copy, DismissNotice).
const (
	// EventStatus carries a Status whenever the connection state changes.
	EventStatus = "aula:status"
	// EventBoard carries a Board after code.share or code.clear.
	EventBoard = "aula:board"
	// EventNotices carries Notices after msg.show or a dismissal.
	EventNotices = "aula:notices"
)

// Connection states shown in the identity banner.
const (
	StateUnconfigured = "unconfigured"
	StateSearching    = "searching"
	StateConnected    = "connected"
	StateDisconnected = "disconnected"
)

// Status is the always-visible identity banner (docs/privacidad.md: the agent
// always identifies itself).
type Status struct {
	State   string `json:"state"`
	Teacher string `json:"teacher"` // console_name; empty until found
	Channel string `json:"channel"`
	Banner  string `json:"banner"` // ready-to-show Spanish text
	Detail  string `json:"detail"` // extra help, e.g. why it is unconfigured
}

// Board is the list of snippets, oldest first.
type Board struct {
	Snippets []codedist.Snippet `json:"snippets"`
}

// Notices are the msg.show notices not dismissed yet, oldest first.
type Notices struct {
	Notices []codedist.Notice `json:"notices"`
}

// View is everything the frontend needs to draw itself from scratch; it calls
// App.View on load and then follows the events.
type View struct {
	Status   Status             `json:"status"`
	Snippets []codedist.Snippet `json:"snippets"`
	Notices  []codedist.Notice  `json:"notices"`
}
