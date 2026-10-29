package protocol

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// MaxChannel is the highest channel number, as in LanSchool.
const MaxChannel = 16000

var channelPattern = regexp.MustCompile(`^[1-9][0-9]{0,4}$`)

// ValidChannel reports whether s is a channel in its canonical form: a number
// from 1 to MaxChannel without leading zeros, such as "1" or "12"
// (docs/protocol.md §1.1). Channels are compared exactly.
func ValidChannel(s string) bool {
	if !channelPattern.MatchString(s) {
		return false
	}
	n, err := strconv.Atoi(s)
	return err == nil && n <= MaxChannel
}

// ParseChannel accepts what a person types ("1", " 02 ") and returns the
// canonical channel ("1", "2"), or an error in Spanish for the UI.
func ParseChannel(s string) (string, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 || n > MaxChannel {
		return "", fmt.Errorf("el canal debe ser un número del 1 al %d", MaxChannel)
	}
	return strconv.Itoa(n), nil
}
