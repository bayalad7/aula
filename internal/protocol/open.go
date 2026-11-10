package protocol

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits of the open.* messages (docs/protocol.md §5.22, §7).
const (
	MaxOpenURL     = 2048
	MaxOpenAppName = 60
	MaxOpenAppPath = 260
)

// OpenStatus is the result an open.ack reports.
type OpenStatus string

// Open statuses.
const (
	OpenOpened OpenStatus = "opened" // the OS accepted to open it
	OpenFailed OpenStatus = "failed" // nothing was opened
)

// OpenReason says why an open.* failed.
type OpenReason string

// Open failure reasons.
const (
	OpenNotAllowed   OpenReason = "not_allowed"   // the path breaks the agent's fixed rules
	OpenNotInstalled OpenReason = "not_installed" // the .exe is not on that computer
	OpenLaunchFailed OpenReason = "launch_failed" // the OS could not open it
	OpenBusy         OpenReason = "busy"          // over the agent's pace
)

var openAppID = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// CheckOpenURL reports whether raw may travel in open.url: http or https,
// with a host, without user or password, spaces or control characters.
func CheckOpenURL(raw string) error {
	if raw == "" || utf8.RuneCountInString(raw) > MaxOpenURL {
		return invalid(TypeOpenURL, "url must be 1-%d characters", MaxOpenURL)
	}
	for _, r := range raw {
		if unicode.IsSpace(r) || unicode.IsControl(r) || r == '\\' {
			return invalid(TypeOpenURL, "url has spaces, backslashes or control characters")
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return invalid(TypeOpenURL, "url does not parse")
	}
	if s := strings.ToLower(u.Scheme); s != "http" && s != "https" {
		return invalid(TypeOpenURL, "url scheme must be http or https")
	}
	if u.Opaque != "" || u.Hostname() == "" {
		return invalid(TypeOpenURL, "url has no host")
	}
	if u.User != nil {
		return invalid(TypeOpenURL, "url must not carry a user or password")
	}
	return nil
}

// OpenURL opens a web page in the student's default browser (P → A).
type OpenURL struct {
	OpenID string `json:"open_id"`
	URL    string `json:"url"`
}

// MessageType implements Payload.
func (OpenURL) MessageType() Type { return TypeOpenURL }

// Validate implements Payload.
func (p OpenURL) Validate() error {
	if !isUUIDv4(p.OpenID) {
		return invalid(TypeOpenURL, "open_id is not a UUID v4")
	}
	return CheckOpenURL(p.URL)
}

// OpenApp launches an app of the console's allowed list (P → A). Path is only
// a request: the agent launches it only if it passes its fixed rules, with no
// arguments (internal/openapp).
type OpenApp struct {
	OpenID string `json:"open_id"`
	AppID  string `json:"app_id"`
	Name   string `json:"name"`
	Path   string `json:"path"`
}

// MessageType implements Payload.
func (OpenApp) MessageType() Type { return TypeOpenApp }

// Validate implements Payload. It checks lengths only: a path that breaks the
// rules is answered with open.ack not_allowed, not with an error.
func (p OpenApp) Validate() error {
	if !isUUIDv4(p.OpenID) {
		return invalid(TypeOpenApp, "open_id is not a UUID v4")
	}
	if !openAppID.MatchString(p.AppID) {
		return invalid(TypeOpenApp, "app_id must match %s", openAppID)
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(p.Name)); n == 0 || utf8.RuneCountInString(p.Name) > MaxOpenAppName {
		return invalid(TypeOpenApp, "name must be 1-%d characters", MaxOpenAppName)
	}
	if strings.ContainsFunc(p.Name, unicode.IsControl) {
		return invalid(TypeOpenApp, "name has control characters")
	}
	if p.Path == "" || utf8.RuneCountInString(p.Path) > MaxOpenAppPath {
		return invalid(TypeOpenApp, "path must be 1-%d characters", MaxOpenAppPath)
	}
	return nil
}

// OpenAck reports whether an open.* was opened (A → P).
type OpenAck struct {
	RefID  string     `json:"ref_id"`
	OpenID string     `json:"open_id"`
	Status OpenStatus `json:"status"`
	Reason OpenReason `json:"reason,omitempty"`
}

// MessageType implements Payload.
func (OpenAck) MessageType() Type { return TypeOpenAck }

// Validate implements Payload.
func (p OpenAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeOpenAck, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.OpenID) {
		return invalid(TypeOpenAck, "open_id is not a UUID v4")
	}
	switch p.Status {
	case OpenOpened:
		if p.Reason != "" {
			return invalid(TypeOpenAck, "reason goes only with failed")
		}
	case OpenFailed:
		switch p.Reason {
		case OpenNotAllowed, OpenNotInstalled, OpenLaunchFailed, OpenBusy:
		default:
			return invalid(TypeOpenAck, "reason must be not_allowed, not_installed, launch_failed or busy")
		}
	default:
		return invalid(TypeOpenAck, "status must be opened or failed")
	}
	return nil
}
