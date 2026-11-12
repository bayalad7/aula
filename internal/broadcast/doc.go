// Package broadcast implements the broadcast of the teacher's screen to the
// agents of the channel (docs/protocol.md §5.12, D-014), without touching the
// OS: the frames come from platform.DisplayCapture and platform.VideoEncoder.
//
// Console side: Broadcaster captures one monitor, encodes it once and sends
// the same H.264 track to one WebRTC connection per agent (pion, ICE-lite on
// a single UDP port). Caster is its signaling: start, offer, answer, ICE,
// retries and the receiver count, over the Peer interface.
//
// Agent side: Viewer validates the signaling and hands it to the frontend,
// whose RTCPeerConnection draws the video. The agent never sends media.
//
// Showing a student's screen to the class (docs/protocol.md §5.24) reuses
// all of it with the source the other way round. The source agent's
// Presenter shows the band and the native badge (IndicatorMux) before anything
// is captured (the student is not asked, as with a remote control), and its LocalPublisher
// (Source + pion, send-only) answers the console's offer. The console's
// Projector receives that one stream on a relay Media and forwards the RTP
// packets, untouched, through a Caster to every other student with
// screen.present.view and to the console's own view (ConsoleViewID). The
// students only connect to the console, never to each other.
//
// Privacy rules enforced here (docs/privacidad.md rules 12–16, 75–82): the agent
// shows the broadcast view before handling the offer; it only accepts
// receive-only video; the broadcast ends with screen.broadcast.stop or when
// the connection closes; nothing is recorded. A presentation captures
// nothing before its band and badge are on screen, excludes the Aula window
// from the capture, only ends when the teacher (or a failure, or the
// connection) ends it, and is never recorded nor decoded by the console.
package broadcast
