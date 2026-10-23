package protocol

import "regexp"

var channelPattern = regexp.MustCompile(`^[A-Z0-9_]{1,32}$`)

// ValidChannel reports whether s is a well-formed classroom channel name, such
// as "AULA_101" (docs/protocol.md §1.1). Channels are compared exactly.
func ValidChannel(s string) bool {
	return channelPattern.MatchString(s)
}
