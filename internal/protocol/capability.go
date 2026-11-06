package protocol

// Capability is a feature an agent declares in hello (docs/protocol.md §4).
type Capability string

// Implemented capabilities.
const (
	CapCodeView    Capability = "code.view"
	CapMsgShow     Capability = "msg.show"
	CapScreenThumb Capability = "screen.thumb" // Windows (docs/protocol.md §5.6)
	// CapScreenBroadcast receives the teacher's screen (§5.12). Windows.
	CapScreenBroadcast Capability = "screen.broadcast"
	// CapAppBlock and CapWebFilter apply the class restrictions (§5.13–5.14).
	// Windows, with the agent running as a service.
	CapAppBlock  Capability = "app.block"
	CapWebFilter Capability = "web.filter"
	// CapPower shuts down, reboots or logs off (§5.15). Windows.
	CapPower Capability = "power"
	// CapRemote lets the teacher see and control the keyboard and mouse (§5.16).
	// Windows, in window mode or as a service with its helper.
	CapRemote Capability = "remote"
	// CapScreenSnapshot sends one still image of the screen (§5.17). Windows,
	// with the same capture and indicator as screen.thumb.
	CapScreenSnapshot Capability = "screen.snapshot"
	// CapHand and CapMsgStudent let the student raise a hand and write to the
	// teacher (§5.18). Every OS, whenever the agent has a window.
	CapHand       Capability = "hand"
	CapMsgStudent Capability = "msg.student"
	// CapTimer shows the class countdown in the window (§5.19). Every OS,
	// whenever the agent has a window.
	CapTimer Capability = "timer"
)

// Capabilities reserved for future phases. No message requires them yet.
const (
	CapScreenFreeze Capability = "screen.freeze" // documented, pending (§6)
	CapFileCollect  Capability = "file.collect"
	CapCodeCollect  Capability = "code.collect"
	CapPoll         Capability = "poll"
)

// requiredCapability lists the capability an agent must declare before the
// console may send it a message of the given type.
var requiredCapability = map[Type]Capability{
	TypeCodeShare: CapCodeView,
	TypeCodeClear: CapCodeView,
	TypeMsgShow:   CapMsgShow,

	TypeScreenThumbStart: CapScreenThumb,
	TypeScreenThumbStop:  CapScreenThumb,

	TypeScreenBroadcastStart: CapScreenBroadcast,
	TypeScreenBroadcastOffer: CapScreenBroadcast,
	TypeScreenBroadcastICE:   CapScreenBroadcast, // only when sent P → A
	TypeScreenBroadcastStop:  CapScreenBroadcast,

	TypeAppBlockSet:    CapAppBlock,
	TypeAppBlockClear:  CapAppBlock,
	TypeWebFilterSet:   CapWebFilter,
	TypeWebFilterClear: CapWebFilter,

	TypePowerAction: CapPower,
	TypePowerCancel: CapPower,

	TypeRemoteStart: CapRemote,
	TypeRemoteInput: CapRemote,
	TypeRemoteStop:  CapRemote,

	TypeScreenSnapshotRequest: CapScreenSnapshot,

	TypeHandLower:     CapHand, // only when sent P → A
	TypeMsgStudentAck: CapMsgStudent,

	TypeTimerStart:  CapTimer,
	TypeTimerExtend: CapTimer,
	TypeTimerStop:   CapTimer,
}

// agentCapability lists the capability an agent must have declared before the
// console accepts a message it starts (docs/protocol.md §4).
var agentCapability = map[Type]Capability{
	TypeHandRaise:  CapHand,
	TypeHandLower:  CapHand,
	TypeMsgStudent: CapMsgStudent,
}

// AgentCapability reports the capability an agent must have declared for the
// console to accept a message of type t that the agent starts, if any.
func AgentCapability(t Type) (Capability, bool) {
	c, ok := agentCapability[t]
	return c, ok
}

// RequiredCapability reports the capability needed to receive a message of
// type t, if any.
func RequiredCapability(t Type) (Capability, bool) {
	c, ok := requiredCapability[t]
	return c, ok
}

// Has reports whether caps contains c.
func Has(caps []Capability, c Capability) bool {
	for _, x := range caps {
		if x == c {
			return true
		}
	}
	return false
}
