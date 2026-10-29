package main

import (
	"time"

	"github.com/bayalad7/aula/internal/codedist"
)

// UI contract between this Go backend and frontend/dist (documented in
// docs/eventos-ui.md). Event names must match frontend/dist/events.js; a test
// checks it. The state lives here; the frontend only draws it.
const (
	// EventStudents carries Students when an agent joins, leaves, goes silent
	// or acknowledges code.
	EventStudents = "aula:students"
	// EventHistory carries History after every code emission.
	EventHistory = "aula:history"
	// EventDeliveries carries Deliveries when something is sent or acked.
	EventDeliveries = "aula:deliveries"
	// EventClass carries a View when the automatic start on the default
	// channel finishes (or fails).
	EventClass = "aula:class"
	// EventCapture carries a Capture after the hotkey or the button.
	EventCapture = "aula:capture"
)

// Setup is what the channel selection screen needs.
type Setup struct {
	RecentChannels []string `json:"recentChannels"`
	ConsoleName    string   `json:"consoleName"`
	PublicKey      string   `json:"publicKey"` // "" until the first class creates the key
	Fingerprint    string   `json:"fingerprint"`
	Version        string   `json:"version"`
	DefaultChannel string   `json:"defaultChannel"`
	StartError     string   `json:"startError"` // why the automatic start failed
	KeySource      string   `json:"keySource"`  // "centro" or "personal"
}

// ClassInfo identifies the open class.
type ClassInfo struct {
	Channel     string `json:"channel"`
	ConsoleName string `json:"consoleName"`
	PublicKey   string `json:"publicKey"` // aula-ed25519:… to configure agents
	Fingerprint string `json:"fingerprint"`
	KeySource   string `json:"keySource"` // "centro" (site key) or "personal" (no site key on this machine)
	Port        int    `json:"port"`
}

// Student is one agent as shown in its card.
type Student struct {
	ID           string   `json:"id"`
	Hostname     string   `json:"hostname"`
	Username     string   `json:"username"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"osVersion"`
	AgentVersion string   `json:"agentVersion"`
	State        string   `json:"state"` // "connected" | "disconnected"
	Capabilities []string `json:"capabilities"`
	CanCode      bool     `json:"canCode"`
	CanNotice    bool     `json:"canNotice"`
	LastCodeAck  string   `json:"lastCodeAck"` // snippet_id; "" after a clear
	AckedAt      string   `json:"ackedAt"`     // "" if nothing acknowledged yet
}

// Students is the list for the channel plus the counts that enable actions.
type Students struct {
	Students  []Student `json:"students"`
	Connected int       `json:"connected"`
	CanCode   int       `json:"canCode"`   // connected agents with code.view
	CanNotice int       `json:"canNotice"` // connected agents with msg.show
}

// History is the numbered list of snippets of the session.
type History struct {
	Entries []codedist.HistoryEntry `json:"entries"`
}

// Deliveries are the recent sends with their ack counts, newest first.
type Deliveries struct {
	Deliveries []codedist.Delivery `json:"deliveries"`
}

// Draft is clipboard content captured for the code panel. It is never sent
// on its own: the teacher reviews it and presses an emit button.
type Draft struct {
	Content    string    `json:"content"`
	Lang       string    `json:"lang"`
	Lines      int       `json:"lines"`
	Source     string    `json:"source"` // "hotkey" | "button"
	CapturedAt time.Time `json:"capturedAt"`
}

// Capture reports a capture attempt; Error is set when it failed.
type Capture struct {
	Draft *Draft `json:"draft"`
	Error string `json:"error"`
}

// HotkeyStatus tells whether the global capture hotkey works.
type HotkeyStatus struct {
	Combo  string `json:"combo"`
	Active bool   `json:"active"`
	Error  string `json:"error"`
}

// View is the whole console state for the first render.
type View struct {
	Active     bool                    `json:"active"`
	Class      ClassInfo               `json:"class"`
	Students   Students                `json:"students"`
	History    []codedist.HistoryEntry `json:"history"`
	Deliveries []codedist.Delivery     `json:"deliveries"`
	Hotkey     HotkeyStatus            `json:"hotkey"`
}

// EmitRequest is the code panel content when an emit button is pressed.
type EmitRequest struct {
	Title   string `json:"title"`
	Lang    string `json:"lang"`
	Content string `json:"content"`
	Mode    string `json:"mode"` // "replace" | "append"
}
