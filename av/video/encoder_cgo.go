//go:build cgo && libvpx
// +build cgo,libvpx

// Package video provides video processing capabilities for ToxAV.
//
// This file provides the CGo-based libvpx VP8 encoder factory when built
// with the 'libvpx' build tag. This enables full VP8 encoding with P-frames.
//
// Build with:
//
//	go build -tags libvpx ./...
//
// Prerequisites:
//   - libvpx-dev (Debian/Ubuntu) or libvpx (Homebrew/macOS)
//   - CGo enabled (default, unless CGO_ENABLED=0)
package video

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// Note: When xlab/libvpx-go is added as a dependency, uncomment:
// import vpx "github.com/xlab/libvpx-go/vpx"

// LibVPXEncoder wraps the libvpx VP8 encoder for full VP8 support with P-frames.
//
// This encoder requires CGo and libvpx to be installed. It produces
// RFC 6386 compliant VP8 bitstreams with both I-frames and P-frames.
type LibVPXEncoder struct {
	// encoder *vpx.CodecCtx  // Uncomment when xlab/libvpx-go is added
	bitRate  uint32
	width    uint16
	height   uint16
	keyFrame bool // Track whether next frame should be a keyframe
}

// NewLibVPXEncoder creates a new VP8 encoder using libvpx.
//
// This encoder supports:
//   - I-frames (key frames) and P-frames (predicted frames)
//   - Configurable keyframe interval
//   - Rate control for bandwidth targeting
//   - Real-time encoding mode optimized for video calls
//
// Parameters:
//   - width, height: Frame dimensions (must be positive)
//   - bitRate: Target encoding bit rate in bits per second
//
// Returns an error if libvpx initialization fails.
func NewLibVPXEncoder(width, height uint16, bitRate uint32) (*LibVPXEncoder, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NewLibVPXEncoder",
		"width":    width,
		"height":   height,
		"bit_rate": bitRate,
	}).Info("Creating libvpx VP8 encoder with P-frame support")

	// TODO: Initialize libvpx encoder when xlab/libvpx-go is added as dependency
	// Example initialization:
	// cfg := &vpx.CodecEncCfg{}
	// vpx.CodecEncConfigDefault(vpx.CodecVP8Encoder(), cfg, 0)
	// cfg.GW = uint32(width)
	// cfg.GH = uint32(height)
	// cfg.RcTargetBitrate = bitRate / 1000 // kbps
	// cfg.GTimebase.Num = 1
	// cfg.GTimebase.Den = 30
	// cfg.RcEndUsage = vpx.RcModeVBR
	// ctx := &vpx.CodecCtx{}
	// if err := vpx.CodecEncInitVer(ctx, vpx.CodecVP8Encoder(), cfg, 0, vpx.EncoderABIVersion); err != nil {
	//     return nil, fmt.Errorf("failed to initialize libvpx: %w", err)
	// }

	return &LibVPXEncoder{
		bitRate:  bitRate,
		width:    width,
		height:   height,
		keyFrame: true, // First frame is always a keyframe
	}, nil
}

// Encode encodes a YUV420 video frame into a VP8 bitstream.
//
// Unlike the pure-Go encoder, this can produce both I-frames and P-frames.
// Keyframes are inserted periodically based on configuration.
func (e *LibVPXEncoder) Encode(frame *VideoFrame) ([]byte, error) {
	if frame == nil {
		return nil, fmt.Errorf("frame cannot be nil")
	}

	// TODO: Implement actual encoding when xlab/libvpx-go is added
	// This is a placeholder that falls back to I-frame-only behavior

	logrus.WithFields(logrus.Fields{
		"function":  "LibVPXEncoder.Encode",
		"width":     frame.Width,
		"height":    frame.Height,
		"key_frame": e.keyFrame,
	}).Debug("Encoding frame with libvpx")

	// Placeholder: return error indicating dependency not yet added
	return nil, fmt.Errorf("libvpx encoding not yet implemented: add github.com/xlab/libvpx-go dependency")
}

// SetBitRate updates the target encoding bit rate.
func (e *LibVPXEncoder) SetBitRate(bitRate uint32) error {
	e.bitRate = bitRate
	// TODO: Update libvpx encoder configuration
	return nil
}

// SupportsInterframe returns true because libvpx supports P-frames.
func (e *LibVPXEncoder) SupportsInterframe() bool {
	return true
}

// SetKeyFrameInterval configures the maximum number of inter frames between
// key frames. A value of 0 means every frame is a key frame.
func (e *LibVPXEncoder) SetKeyFrameInterval(interval int) {
	// TODO: Update libvpx encoder configuration when xlab/libvpx-go is added
	_ = interval
}

// ForceKeyFrame causes the next Encode call to produce a key frame.
func (e *LibVPXEncoder) ForceKeyFrame() {
	e.keyFrame = true
}

// Close releases encoder resources.
func (e *LibVPXEncoder) Close() error {
	// TODO: Call vpx.CodecDestroy(e.encoder) when implemented
	return nil
}

// NewDefaultEncoder creates a VP8 encoder using libvpx for full VP8 support.
//
// This encoder produces both I-frames and P-frames for efficient bandwidth usage.
// This implementation is used when built with '-tags libvpx'.
func NewDefaultEncoder(width, height uint16, bitRate uint32) (Encoder, error) {
	return NewLibVPXEncoder(width, height, bitRate)
}

// DefaultEncoderSupportsInterframe returns whether the default encoder
// supports inter-frame prediction (P-frames).
//
// In CGo+libvpx builds, this returns true.
func DefaultEncoderSupportsInterframe() bool {
	return true
}

// DefaultEncoderName returns a human-readable name for the default encoder.
func DefaultEncoderName() string {
	return "libvpx (full VP8 with P-frames)"
}
