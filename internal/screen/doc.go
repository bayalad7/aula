// Package screen implements screen thumbnails (docs/protocol.md §5.6–5.8) on
// both ends, without touching the OS: the agent side (Streamer) captures
// through platform.ScreenCapture and shows platform.ScreenIndicator, and the
// console side (Wall) decides which agents stream and keeps their last frame.
// It also implements the pause (screen.freeze, §5.9–5.11): the agent side
// (Freezer) shows platform.PauseOverlay and removes it when the connection
// closes, and the console side (Pauses) follows who is paused and who acked.
//
// Privacy rules enforced here (docs/privacidad.md rules 6–11): the indicator
// is on screen before the first capture and for as long as the stream lasts;
// a stream ends with screen.thumb.stop or when the connection closes; frames
// only live in memory. The pause says who paused the screen and why, and
// nobody stays paused without the class.
package screen
