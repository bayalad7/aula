package protocol

// Limits and defaults of power.action (docs/protocol.md §5.15, §7).
const (
	DefaultPowerDelay = 30  // seconds, when delay_s is absent or 0
	MinPowerDelay     = 10  // seconds
	MaxPowerDelay     = 600 // seconds
	MaxPowerPostpones = 2   // times the student may postpone one order
	PowerPostponeS    = 300 // seconds each postponement adds
)

// PowerKind is what a power order does.
type PowerKind string

// Power actions.
const (
	PowerShutdown PowerKind = "shutdown"
	PowerReboot   PowerKind = "reboot"
	PowerLogoff   PowerKind = "logoff"
)

// PowerStatus is the state a power.ack reports.
type PowerStatus string

// Power statuses.
const (
	PowerCountdown PowerStatus = "countdown" // the notice is shown and counting
	PowerPostponed PowerStatus = "postponed" // the student added 5 minutes
	PowerExecuting PowerStatus = "executing" // sent right before calling the OS
	PowerCancelled PowerStatus = "cancelled" // cancelled or replaced
)

func validPowerKind(k PowerKind) bool {
	return k == PowerShutdown || k == PowerReboot || k == PowerLogoff
}

// PowerAction orders a shutdown, a reboot or a log off (P → A).
type PowerAction struct {
	ActionID string    `json:"action_id"`
	Action   PowerKind `json:"action"`
	DelayS   int       `json:"delay_s,omitempty"`
	Force    bool      `json:"force"`
}

// MessageType implements Payload.
func (PowerAction) MessageType() Type { return TypePowerAction }

// Validate implements Payload.
func (p PowerAction) Validate() error {
	if !isUUIDv4(p.ActionID) {
		return invalid(TypePowerAction, "action_id is not a UUID v4")
	}
	if !validPowerKind(p.Action) {
		return invalid(TypePowerAction, "action must be shutdown, reboot or logoff")
	}
	if p.DelayS != 0 && (p.DelayS < MinPowerDelay || p.DelayS > MaxPowerDelay) {
		return invalid(TypePowerAction, "delay_s must be %d-%d (or 0 for %d)", MinPowerDelay, MaxPowerDelay, DefaultPowerDelay)
	}
	return nil
}

// Delay is the countdown in seconds: DelayS, or DefaultPowerDelay if 0.
func (p PowerAction) Delay() int {
	if p.DelayS == 0 {
		return DefaultPowerDelay
	}
	return p.DelayS
}

// PowerCancel cancels the countdown of an order (P → A).
type PowerCancel struct {
	ActionID string `json:"action_id"`
}

// MessageType implements Payload.
func (PowerCancel) MessageType() Type { return TypePowerCancel }

// Validate implements Payload.
func (p PowerCancel) Validate() error {
	if !isUUIDv4(p.ActionID) {
		return invalid(TypePowerCancel, "action_id is not a UUID v4")
	}
	return nil
}

// PowerAck reports the state of an order (A → P).
type PowerAck struct {
	RefID         string      `json:"ref_id"`
	ActionID      string      `json:"action_id"`
	Status        PowerStatus `json:"status"`
	RemainingS    int         `json:"remaining_s"`
	PostponesLeft int         `json:"postpones_left"`
}

// MessageType implements Payload.
func (PowerAck) MessageType() Type { return TypePowerAck }

// Validate implements Payload.
func (p PowerAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypePowerAck, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.ActionID) {
		return invalid(TypePowerAck, "action_id is not a UUID v4")
	}
	switch p.Status {
	case PowerCountdown, PowerPostponed, PowerExecuting, PowerCancelled:
	default:
		return invalid(TypePowerAck, "status must be countdown, postponed, executing or cancelled")
	}
	if p.RemainingS < 0 || p.RemainingS > MaxPowerDelay+MaxPowerPostpones*PowerPostponeS {
		return invalid(TypePowerAck, "remaining_s out of range")
	}
	if p.PostponesLeft < 0 || p.PostponesLeft > MaxPowerPostpones {
		return invalid(TypePowerAck, "postpones_left must be 0-%d", MaxPowerPostpones)
	}
	return nil
}
