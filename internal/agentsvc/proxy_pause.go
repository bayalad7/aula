package agentsvc

import (
	"context"
	"errors"

	"github.com/bayalad7/aula/internal/platform"
)

// Service → helper: the pause window (docs/protocol.md §5.9). The service
// has no desktop: the helper shows the windows in the student's session, and
// removes them by itself if the local channel is lost (docs/privacidad.md
// rule 10).
const (
	CallPauseShow  = "pause.show"  // platform.PauseText: show, or replace the text
	CallPauseYield = "pause.yield" // bool: stay below the window (true) or above everything
	CallPauseHide  = "pause.hide"  // remove it
)

// Pause shows the pause window through the helper.
type Pause struct{ Helper *Current }

// Show implements platform.PauseOverlay.
func (p Pause) Show(text platform.PauseText) error {
	return p.Helper.call(context.Background(), CallPauseShow, text, nil)
}

// Yield implements platform.PauseOverlay. Without a helper there is nothing
// to move.
func (p Pause) Yield(on bool) error {
	return ignoreGone(p.Helper.call(context.Background(), CallPauseYield, on, nil))
}

// Hide implements platform.PauseOverlay. Without a helper there is nothing
// on screen to hide: the helper hid it when it lost the channel.
func (p Pause) Hide() error {
	return ignoreGone(p.Helper.call(context.Background(), CallPauseHide, nil, nil))
}

func ignoreGone(err error) error {
	if errors.Is(err, ErrNoHelper) || errors.Is(err, ErrClosed) {
		return nil
	}
	return err
}
