//go:build !windows || !(amd64 || arm64)

package platform

// Media Foundation is called with 64-bit argument passing; other systems and
// 32-bit Windows have no H.264 encoder yet.

func newH264Encoder(VideoEncoderConfig) (VideoEncoder, error) { return nil, ErrNotSupported }
