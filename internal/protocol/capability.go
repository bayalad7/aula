package protocol

// Capability is a feature an agent declares in hello (docs/protocol.md §4).
type Capability string

// Capabilities implemented in v0.
const (
	CapCodeView Capability = "code.view"
	CapMsgShow  Capability = "msg.show"
)

// Capabilities reserved for future phases. No v0 message requires them.
const (
	CapScreenThumb     Capability = "screen.thumb"
	CapScreenFreeze    Capability = "screen.freeze"
	CapScreenBroadcast Capability = "screen.broadcast"
	CapAppBlock        Capability = "app.block"
	CapWebFilter       Capability = "web.filter"
	CapFileCollect     Capability = "file.collect"
	CapCodeCollect     Capability = "code.collect"
	CapPower           Capability = "power"
	CapRemote          Capability = "remote"
	CapPoll            Capability = "poll"
)

// requiredCapability lists the capability an agent must declare before the
// console may send it a message of the given type.
var requiredCapability = map[Type]Capability{
	TypeCodeShare: CapCodeView,
	TypeCodeClear: CapCodeView,
	TypeMsgShow:   CapMsgShow,
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
