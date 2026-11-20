//go:build !(windows && (amd64 || arm64)) && !(linux && cgo)

package platform

// Media Foundation is called with 64-bit argument passing, and Linux loads
// openh264 through cgo; other systems, 32-bit Windows and Linux built
// without cgo have no H.264 encoder.

func newH264Encoder(VideoEncoderConfig) (VideoEncoder, error) { return nil, ErrNotSupported }
