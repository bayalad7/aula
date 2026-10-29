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
)

// Capabilities reserved for future phases. No message requires them yet.
const (
	CapScreenFreeze Capability = "screen.freeze" // documented, pending (§6)
	CapAppBlock     Capability = "app.block"
	CapWebFilter    Capability = "web.filter"
	CapFileCollect  Capability = "file.collect"
	CapCodeCollect  Capability = "code.collect"
	CapPower        Capability = "power"
	CapRemote       Capability = "remote"
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
