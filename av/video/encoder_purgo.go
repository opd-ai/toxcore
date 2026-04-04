//go:build !cgo || !libvpx
// +build !cgo !libvpx

// Package video provides video processing capabilities for ToxAV.
//
// This file provides the default (pure Go) VP8 encoder factory.
// The opd-ai/vp8 library supports both I-frames and P-frames with
// motion estimation. For an alternative VP8 backend via libvpx, build with:
//
//	go build -tags libvpx ./...
//
// This requires libvpx development headers to be installed.
package video

// NewDefaultEncoder creates a VP8 encoder using the pure-Go opd-ai/vp8 library.
//
// This encoder supports both I-frames (key frames) and P-frames (inter frames)
// with motion estimation. Key frames are emitted periodically based on the
// configured key frame interval (default: every 30 frames).
//
// Parameters:
//   - width, height: Frame dimensions (must be positive, even integers)
//   - bitRate: Target encoding bit rate in bits per second
//
// Returns the encoder and any error from initialization.
func NewDefaultEncoder(width, height uint16, bitRate uint32) (Encoder, error) {
	return NewRealVP8Encoder(width, height, bitRate)
}

// DefaultEncoderSupportsInterframe returns whether the default encoder
// supports inter-frame prediction (P-frames).
//
// Returns true because the opd-ai/vp8 library supports P-frames with
// motion estimation.
func DefaultEncoderSupportsInterframe() bool {
	return true
}

// DefaultEncoderName returns a human-readable name for the default encoder.
func DefaultEncoderName() string {
	return "opd-ai/vp8 (I-frames and P-frames)"
}
