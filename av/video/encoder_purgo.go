//go:build !cgo || !libvpx
// +build !cgo !libvpx

// Package video provides video processing capabilities for ToxAV.
//
// This file provides the default (pure Go) VP8 encoder factory.
// For full VP8 support with P-frames, build with:
//
//	go build -tags libvpx ./...
//
// This requires libvpx development headers to be installed.
package video

// NewDefaultEncoder creates a VP8 encoder using the pure-Go opd-ai/vp8 library.
//
// This encoder produces I-frames (key frames) only. For P-frame support,
// build with '-tags libvpx' and ensure libvpx is installed.
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
// In pure-Go builds (default), this returns false.
// In CGo+libvpx builds, this returns true.
func DefaultEncoderSupportsInterframe() bool {
	return false
}

// DefaultEncoderName returns a human-readable name for the default encoder.
func DefaultEncoderName() string {
	return "opd-ai/vp8 (I-frame only)"
}
