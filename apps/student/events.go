package main

import (
	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/policy"
	"github.com/bayalad7/aula/internal/power"
)

// UI contract between this Go backend and frontend/dist (documented in
// docs/eventos-ui.md). Event names must match frontend/dist/events.js; a test
// checks it.
//
// Backend → frontend: runtime.EventsEmit with one of the events below.
// Frontend → backend: the bound methods of App (View, Copy, DismissNotice,
// ReportMedia, BroadcastAnswer, BroadcastCandidate, BroadcastFailed,
// PowerPostpone, DismissPower).
const (
	// EventStatus carries a Status whenever the connection state changes.
	EventStatus = "aula:status"
	// EventBoard carries a Board after code.share or code.clear.
	EventBoard = "aula:board"
	// EventNotices carries Notices after msg.show or a dismissal.
	EventNotices = "aula:notices"
	// EventBroadcast carries a BroadcastView when the teacher's broadcast
	// starts or ends: the view and its notice are shown before the video.
	EventBroadcast = "aula:broadcast"
	// EventSignal carries a broadcast.Signal (offer or ICE candidate) for the
	// frontend's RTCPeerConnection.
	EventSignal = "aula:broadcast-signal"
	// EventRestrictions carries a policy.State when the class restrictions
	// start, change or end (docs/privacidad.md rules 20-21).
	EventRestrictions = "aula:restrictions"
	// EventAppClosed carries an AppClosed when a blocked app is closed.
	EventAppClosed = "aula:app-closed"
	// EventPower carries a PowerView when a power order starts, is
	// postponed, runs, fails or ends (docs/privacidad.md rules 26-28).
	EventPower = "aula:power"
)

// PowerView is the power notice as the window shows it. Notice is nil when
// there is none; the window counts RemainingS down by itself.
type PowerView struct {
	Notice  *power.Notice `json:"notice"`
	Teacher string        `json:"teacher"` // who gave the order
	Channel string        `json:"channel"`
	Verb    string        `json:"verb"` // "apagar este equipo", "reiniciar este equipo", "cerrar tu sesión"
}

// AppClosed names the app the agent closed (docs/privacidad.md rule 22).
type AppClosed struct {
	App string `json:"app"`
}

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
	State    string `json:"state"`
	Teacher  string `json:"teacher"` // console_name; empty until found
	Channel  string `json:"channel"`
	Banner   string `json:"banner"`   // ready-to-show Spanish text
	Detail   string `json:"detail"`   // extra help, e.g. why it is unconfigured
	Viewed   bool   `json:"viewed"`   // the teacher is receiving this screen
	Watching bool   `json:"watching"` // this screen shows the teacher's broadcast
	// Restricted is set while class restrictions are on.
	Restricted bool `json:"restricted"`
}

// BroadcastView is the broadcast of the teacher's screen as the window shows
// it (docs/privacidad.md rule 12). Notice is the fixed band over the video.
type BroadcastView struct {
	Active      bool   `json:"active"`
	BroadcastID string `json:"broadcastId"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	FPS         int    `json:"fps"`
	Teacher     string `json:"teacher"`
	Channel     string `json:"channel"`
	Notice      string `json:"notice"`
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
	// Broadcast is the teacher's broadcast in force, if any.
	Broadcast BroadcastView `json:"broadcast"`
	// Restrictions are the class restrictions in force.
	Restrictions policy.State `json:"restrictions"`
	// Power is the power notice in force, if any.
	Power PowerView `json:"power"`
}
