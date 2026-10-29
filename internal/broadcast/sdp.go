package broadcast

import (
	"errors"
	"fmt"

	"github.com/pion/sdp/v3"
)

// errBadSDP is wrapped by every rejection of checkSDP.
var errBadSDP = errors.New("sdp not allowed for a screen broadcast")

// checkSDP enforces the shape of a broadcast (docs/protocol.md §5.12): one
// video section and nothing else (no audio, no data channel), with the given
// direction: "sendonly" for the console's offer, "recvonly" for the answer.
func checkSDP(raw, direction string) error {
	var d sdp.SessionDescription
	if err := d.UnmarshalString(raw); err != nil {
		return fmt.Errorf("%w: %w", errBadSDP, err)
	}
	if len(d.MediaDescriptions) != 1 {
		return fmt.Errorf("%w: %d media sections, want 1", errBadSDP, len(d.MediaDescriptions))
	}
	m := d.MediaDescriptions[0]
	if m.MediaName.Media != "video" {
		return fmt.Errorf("%w: media %q, want video", errBadSDP, m.MediaName.Media)
	}
	got := directionOf(m.Attributes)
	if got == "" {
		got = directionOf(d.Attributes)
	}
	if got == "" {
		got = "sendrecv" // RFC 4566 default
	}
	if got != direction {
		return fmt.Errorf("%w: direction %q, want %q", errBadSDP, got, direction)
	}
	return nil
}

func directionOf(attrs []sdp.Attribute) string {
	for _, a := range attrs {
		switch a.Key {
		case "sendrecv", "sendonly", "recvonly", "inactive":
			return a.Key
		}
	}
	return ""
}
