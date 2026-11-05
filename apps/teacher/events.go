package main

import (
	"time"

	"github.com/bayalad7/aula/internal/codedist"
	"github.com/bayalad7/aula/internal/power"
	"github.com/bayalad7/aula/internal/remote"
	"github.com/bayalad7/aula/internal/snapshot"
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
	// EventFrame carries a screen.Thumb for every screen thumbnail received
	// while the screens view is open. Frames only live in memory.
	EventFrame = "aula:frame"
	// EventRemote carries a remote.Control whenever the remote control starts,
	// pauses, resumes or ends (docs/protocol.md §5.16).
	EventRemote = "aula:remote"
	// EventRemoteFrame carries a screen.Thumb of the controlled screen.
	EventRemoteFrame = "aula:remote-frame"
	// EventBroadcast carries a BroadcastState when the broadcast of the
	// teacher's screen starts, stops or its receiver count changes.
	EventBroadcast = "aula:broadcast"
	// EventRestrictions carries a RestrictionsView when the class
	// restrictions change (per-student acknowledgements arrive with
	// aula:students).
	EventRestrictions = "aula:restrictions"
	// EventSimulation carries a SimulationView when the aula simulada opens,
	// closes or one of its students changes.
	EventSimulation = "aula:simulation"
	// EventConnectCheck carries a connectcheck.Report whenever a step of
	// CheckConnection starts or ends; the last one has done set.
	EventConnectCheck = "aula:connectcheck"
	// EventSnapshots carries Snapshots whenever a screenshot of a student is
	// asked for, arrives, fails, is saved or discarded (docs/protocol.md
	// §5.17). Never the image: the window asks for it with SnapshotImage.
	EventSnapshots = "aula:snapshots"
	// EventParticipation carries Participation whenever a hand is raised or
	// lowered, a student's message arrives or is answered (docs/protocol.md
	// §5.18). Arrival is set only when a hand or a message just arrived.
	EventParticipation = "aula:participation"
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
	// CertExpires is when the connection certificate expires ("2006-01-02";
	// "" if unknown). Informative: agents pin it and ignore its dates.
	CertExpires string `json:"certExpires"`
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
	CanThumb     bool     `json:"canThumb"`     // screen.thumb (Windows agents)
	CanBroadcast bool     `json:"canBroadcast"` // screen.broadcast (Windows agents)
	CanAppBlock  bool     `json:"canAppBlock"`  // app.block (Windows service agents)
	CanWebFilter bool     `json:"canWebFilter"` // web.filter (Windows service agents)
	CanPower     bool     `json:"canPower"`     // power (Windows agents)
	CanRemote    bool     `json:"canRemote"`    // remote (Windows agents with their screen)
	CanSnapshot  bool     `json:"canSnapshot"`  // screen.snapshot (Windows agents and simulated students)
	CanHand      bool     `json:"canHand"`      // hand (every OS with a window, simulated students)
	CanMessage   bool     `json:"canMessage"`   // msg.student (the same)
	// Hand is this student's place in the line of raised hands (0: down);
	// NewMessages, their messages not answered yet. The badge in Notas.
	Hand        int `json:"hand"`
	NewMessages int `json:"newMessages"`
	// Power is the last power order sent to this session and its state.
	Power       *power.Entry `json:"power"`
	AppsActive  bool         `json:"appsActive"`  // acknowledged the class's app policy
	WebActive   bool         `json:"webActive"`   // acknowledged the class's web filter
	LastCodeAck string       `json:"lastCodeAck"` // snippet_id; "" after a clear
	AckedAt     string       `json:"ackedAt"`     // "" if nothing acknowledged yet
	// Simulated marks a student of the aula simulada: shown with its badge
	// and never counted as real.
	Simulated bool `json:"simulated"`
	// ConnectedAt is when this session opened ("15:04"); LastError the last
	// protocol error it reported, nil if none.
	ConnectedAt string      `json:"connectedAt"`
	LastError   *AgentError `json:"lastError"`
}

// Students is the list for the channel plus the counts that enable actions.
type Students struct {
	Students  []Student `json:"students"`
	Connected int       `json:"connected"`
	CanCode   int       `json:"canCode"`   // connected agents with code.view
	CanNotice int       `json:"canNotice"` // connected agents with msg.show
	CanThumb  int       `json:"canThumb"`  // connected agents with screen.thumb
	// CanBroadcast counts connected agents with screen.broadcast.
	CanBroadcast int `json:"canBroadcast"`
	// CanAppBlock and CanWebFilter count connected agents with each.
	CanAppBlock  int `json:"canAppBlock"`
	CanWebFilter int `json:"canWebFilter"`
	CanPower     int `json:"canPower"` // connected agents with power
	// CanSnapshot counts connected agents with screen.snapshot, simulated
	// ones included.
	CanSnapshot int `json:"canSnapshot"`
	// Connected counts real students only; Simulated counts the connected
	// simulated ones. CanCode, CanNotice and CanThumb include both.
	Simulated int `json:"simulated"`
}

// DisplayInfo is a monitor the teacher can broadcast.
type DisplayInfo struct {
	Index   int    `json:"index"`
	Label   string `json:"label"` // "Monitor 1 (principal) · 1920×1080"
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Primary bool   `json:"primary"`
}

// BroadcastState is the broadcast of the teacher's screen.
type BroadcastState struct {
	Active    bool `json:"active"`
	Display   int  `json:"display"`   // monitor on air; -1 when inactive
	Receivers int  `json:"receivers"` // students whose video connection is up
	Eligible  int  `json:"eligible"`  // connected students who can receive it
	Available bool `json:"available"` // this machine can capture its monitors
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
	// Reason says why it is not active, for the interface text
	// (ajustes.atajo<Reason>): hotkeyInUse or hotkeyNotSupported; empty with
	// any other failure, whose technical detail is in Error.
	Reason string `json:"reason"`
	Error  string `json:"error"`
}

// HotkeyStatus.Reason values.
const (
	hotkeyInUse        = "InUse"
	hotkeyNotSupported = "NotSupported"
)

// View is the whole console state for the first render.
type View struct {
	Active     bool                    `json:"active"`
	Class      ClassInfo               `json:"class"`
	Students   Students                `json:"students"`
	History    []codedist.HistoryEntry `json:"history"`
	Deliveries []codedist.Delivery     `json:"deliveries"`
	Hotkey     HotkeyStatus            `json:"hotkey"`
	Broadcast  BroadcastState          `json:"broadcast"`
	// Restrictions are the class restrictions.
	Restrictions RestrictionsView `json:"restrictions"`
	// Remote is the remote control on record (the last one, if it ended).
	Remote remote.Control `json:"remote"`
	// Snapshots are the screenshots in memory (pending, ready or failed).
	Snapshots []snapshot.Snapshot `json:"snapshots"`
	// Participation is the «Participación» view: hands and messages.
	Participation Participation `json:"participation"`
}

// Snapshots is the list of screenshots in memory, oldest request first.
type Snapshots struct {
	Snapshots []snapshot.Snapshot `json:"snapshots"`
}

// SnapshotBatch answers TakeSnapshots: the screenshots asked for, and the
// selected students whose computer cannot take one.
type SnapshotBatch struct {
	Snapshots []snapshot.Snapshot `json:"snapshots"`
	Skipped   []string            `json:"skipped"` // hostnames
}

// SnapshotFolder is where saved screenshots go (Ajustes › Acerca de).
type SnapshotFolder struct {
	Path   string `json:"path"`   // <Documentos>/Aula/capturas
	Exists bool   `json:"exists"` // false until the first one is saved
	Error  string `json:"error"`  // why the folder is unknown, if it is
}

// EmitRequest is the code panel content when an emit button is pressed.
type EmitRequest struct {
	Title   string `json:"title"`
	Lang    string `json:"lang"`
	Content string `json:"content"`
	Mode    string `json:"mode"` // "replace" | "append"
	// Targets are the sessions (real or simulated) to send it to; empty
	// means every student.
	Targets []string `json:"targets"`
}
