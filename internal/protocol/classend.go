package protocol

// ClassEndStatus is the result a class.end.ack reports (docs/protocol.md §5.25).
type ClassEndStatus string

// Class end statuses.
const (
	ClassEndClean  ClassEndStatus = "clean"  // the window is as freshly opened
	ClassEndFailed ClassEndStatus = "failed" // something could not be removed; the rest was
)

// ClassEndReason says what could not be removed, with ClassEndFailed.
type ClassEndReason string

// Class end reasons.
const (
	ClassEndRestrictions ClassEndReason = "restrictions" // the class restrictions stayed
	ClassEndPause        ClassEndReason = "pause"        // the pause window stayed
	ClassEndInternal     ClassEndReason = "internal"
)

// ClassEnd asks the agent to leave its window as freshly opened (P → A):
// it ends what acts on the computer and empties the window, in the order of
// §5.25.1. It never touches a pending power order nor the student's files.
type ClassEnd struct {
	EndID string `json:"end_id"`
}

// MessageType implements Payload.
func (ClassEnd) MessageType() Type { return TypeClassEnd }

// Validate implements Payload.
func (p ClassEnd) Validate() error {
	if !isUUIDv4(p.EndID) {
		return invalid(TypeClassEnd, "end_id is not a UUID v4")
	}
	return nil
}

// ClassEndAck reports how the window was left (A → P).
type ClassEndAck struct {
	RefID  string         `json:"ref_id"`
	EndID  string         `json:"end_id"`
	Status ClassEndStatus `json:"status"`
	Reason ClassEndReason `json:"reason,omitempty"`
}

// MessageType implements Payload.
func (ClassEndAck) MessageType() Type { return TypeClassEndAck }

// Validate implements Payload.
func (p ClassEndAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeClassEndAck, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.EndID) {
		return invalid(TypeClassEndAck, "end_id is not a UUID v4")
	}
	switch p.Status {
	case ClassEndClean:
		if p.Reason != "" {
			return invalid(TypeClassEndAck, "reason is only for failed")
		}
	case ClassEndFailed:
		switch p.Reason {
		case ClassEndRestrictions, ClassEndPause, ClassEndInternal:
		default:
			return invalid(TypeClassEndAck, "reason must be restrictions, pause or internal")
		}
	default:
		return invalid(TypeClassEndAck, "status must be clean or failed")
	}
	return nil
}
