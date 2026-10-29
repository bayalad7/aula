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
// Privacy rules enforced here (docs/privacidad.md rules 12–16): the agent
// shows the broadcast view before handling the offer; it only accepts
// receive-only video; the broadcast ends with screen.broadcast.stop or when
// the connection closes; nothing is recorded.
package broadcast
