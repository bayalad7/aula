package main

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/bayalad7/aula/internal/broadcast"
)

// --- Broadcast of the teacher's screen (docs/protocol.md §5.12) --------------------
//
// The RTCPeerConnection lives in the frontend; the backend validates the
// signaling (broadcast.Viewer), shows the view before the offer and relays
// the answer and candidates. Nothing of this machine is sent: the
// connection is receive-only (docs/privacidad.md rule 13).

// awaitMedia waits for the frontend to report whether it can play the
// broadcast, up to a.mediaWait, and declares screen.broadcast in hello if it
// can. Without a report in time the capability is not declared.
func (a *App) awaitMedia(ctx context.Context) {
	if a.mediaWait > 0 {
		t := time.NewTimer(a.mediaWait)
		defer t.Stop()
		select {
		case <-a.mediaReady:
		case <-t.C:
			a.log.Info("frontend did not report its media support in time; screen.broadcast not declared")
		case <-ctx.Done():
		}
	}
	a.mediaOnce.Do(func() { close(a.mediaReady) }) // later reports are ignored
	a.hello.Capabilities = append(slices.Clone(a.hello.Capabilities), a.viewer.Capabilities()...)
}

// ReportMedia is called by the frontend on load: whether it has
// RTCPeerConnection and an H.264 decoder.
func (a *App) ReportMedia(webrtc, h264 bool) {
	if a.onMedia != nil { // helper: it tells the service
		a.onMedia(webrtc, h264)
		return
	}
	a.mediaOnce.Do(func() {
		a.viewer.Supported = webrtc && h264
		a.log.Info("frontend media support", "webrtc", webrtc, "h264", h264)
		close(a.mediaReady)
	})
}

// onBroadcastView shows or hides the broadcast view. It runs before the
// offer reaches the frontend: the student sees the notice first.
func (a *App) onBroadcastView(v broadcast.View) error {
	a.mu.Lock()
	a.watching = v.Active
	st := a.status
	a.mu.Unlock()
	a.setStatus(st)
	a.emit(EventBroadcast, a.broadcastView(v, st))
	if v.Active {
		a.raise()
	}
	a.maximize(v.Active)
	a.log.Info("broadcast view", "active", v.Active, "broadcast", v.BroadcastID)
	return nil
}

func (a *App) broadcastView(v broadcast.View, st Status) BroadcastView {
	bv := BroadcastView{Active: v.Active, BroadcastID: v.BroadcastID, Width: v.Width, Height: v.Height,
		FPS: v.FPS, Teacher: st.Teacher, Channel: st.Channel}
	if v.Active {
		bv.Notice = fmt.Sprintf("▶ Estás viendo la pantalla de %s (canal %s)", st.Teacher, st.Channel)
	}
	return bv
}

func (a *App) onBroadcastSignal(s broadcast.Signal) { a.emit(EventSignal, s) }

// BroadcastAnswer sends the frontend's SDP answer.
func (a *App) BroadcastAnswer(broadcastID string, attempt int, sdp string) error {
	if a.remote != nil {
		return a.forward("BroadcastAnswer", nil, broadcastID, attempt, sdp)
	}
	if err := a.viewer.Answer(broadcastID, attempt, sdp); err != nil {
		a.log.Info("broadcast answer not sent", "err", err)
		return err
	}
	return nil
}

// BroadcastCandidate sends one of the frontend's ICE candidates.
func (a *App) BroadcastCandidate(broadcastID string, attempt int, candidate, mid string, mline int) error {
	if a.remote != nil {
		return a.forward("BroadcastCandidate", nil, broadcastID, attempt, candidate, mid, mline)
	}
	return a.viewer.LocalCandidate(broadcastID, attempt, candidate, mid, mline)
}

// BroadcastFailed tells the console the frontend could not open the video
// connection for this offer; it will send a new one.
func (a *App) BroadcastFailed(broadcastID string, attempt int) error {
	if a.remote != nil {
		return a.forward("BroadcastFailed", nil, broadcastID, attempt)
	}
	a.log.Info("broadcast connection failed in the frontend", "attempt", attempt)
	return a.viewer.Failed(broadcastID, attempt)
}
