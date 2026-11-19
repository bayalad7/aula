package agentsvc

import (
	"context"
	"time"

	"github.com/bayalad7/aula/internal/openapp"
	"github.com/bayalad7/aula/internal/protocol"
)

// Calls of open.* (docs/protocol.md §5.22): the helper opens the page or the
// app in the student's session, as the student and never as SYSTEM
// (docs/privacidad.md rule 69). It checks the URL and the path again with
// the fixed rules of internal/openapp, with the student's folders.
const (
	CallOpenURL = "open.url" // the URL (string) → OpenResult
	CallOpenApp = "open.app" // OpenAppArgs from the console → OpenResult
)

// OpenAppArgs are the arguments of CallOpenApp: the .exe path (Windows)
// and the .desktop id (Linux) the console sent; the helper uses its own.
type OpenAppArgs struct {
	Path    string `json:"path,omitempty"`
	Desktop string `json:"desktop,omitempty"`
}

// OpenResult is the reply of CallOpenURL and CallOpenApp: the open.ack
// reason, empty when it opened.
type OpenResult struct {
	Reason protocol.OpenReason `json:"reason,omitempty"`
}

// openCallTimeout bounds an opening: the shell may take a while to start a
// browser.
const openCallTimeout = 20 * time.Second

// Opener opens through the helper. It implements openapp.Session.
type Opener struct{ Helper *Current }

func (o Opener) call(name string, arg any) error {
	l := o.Helper.Get()
	if l == nil {
		return ErrNoHelper
	}
	ctx, cancel := context.WithTimeout(context.Background(), openCallTimeout)
	defer cancel()
	var r OpenResult
	if err := l.Call(ctx, name, arg, &r); err != nil {
		return err
	}
	return openapp.ReasonErr(r.Reason)
}

// OpenURL implements openapp.Session.
func (o Opener) OpenURL(url string) error { return o.call(CallOpenURL, url) }

// StartApp implements openapp.Session.
func (o Opener) StartApp(path, desktop string) error {
	return o.call(CallOpenApp, OpenAppArgs{Path: path, Desktop: desktop})
}
