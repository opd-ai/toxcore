// Package video/codec provides video codec integration for ToxAV.
//
// This file implements codec-specific functionality including VP8 packet
// formatting and integration with the core video processor.
package video

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// VP8Codec provides VP8-specific video processing functionality.
//
// Wraps the generic video processor with VP8-specific behavior including
// packet formatting, resolution handling, and proper integration with
// VP8 encoding/decoding.
type VP8Codec struct {
	processor *Processor
}

// NewVP8Codec creates a new VP8 codec instance.
//
// Initializes the codec with a standard video processor configured
// for VP8-compatible settings (standard resolutions, appropriate bit rates).
func NewVP8Codec() *VP8Codec {
	logrus.WithFields(logrus.Fields{
		"function": "NewVP8Codec",
	}).Info("Creating new VP8 codec instance")

	codec := &VP8Codec{
		processor: NewProcessor(),
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewVP8Codec",
	}).Info("VP8 codec created successfully")

	return codec
}

// EncodeFrame encodes a video frame using VP8-compatible encoding.
//
// Currently uses the SimpleVP8Encoder but maintains the VP8 interface
// for future enhancement with proper VP8 encoding.
//
// Parameters:
//   - frame: Video frame in YUV420 format
//
// Returns:
//   - []byte: Encoded video frame
//   - error: Any error that occurred during encoding
func (c *VP8Codec) EncodeFrame(frame *VideoFrame) ([]byte, error) {
	if frame == nil {
		logrus.WithFields(logrus.Fields{
			"function": "VP8Codec.EncodeFrame",
			"error":    "frame cannot be nil",
		}).Error("Invalid input frame")
		return nil, fmt.Errorf("frame cannot be nil")
	}

	logrus.WithFields(logrus.Fields{
		"function":     "VP8Codec.EncodeFrame",
		"frame_width":  frame.Width,
		"frame_height": frame.Height,
	}).Debug("Starting VP8 frame encoding")

	data, err := c.processor.ProcessOutgoingLegacy(frame)

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "VP8Codec.EncodeFrame",
			"error":    err.Error(),
		}).Error("Failed to encode VP8 frame")
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"function":     "VP8Codec.EncodeFrame",
		"encoded_size": len(data),
	}).Debug("VP8 frame encoded successfully")

	return data, nil
}

// DecodeFrame decodes a VP8 video frame to YUV420 format.
//
// Uses a pure Go VP8 decoder to handle actual VP8-encoded data.
// Provides width, height, and YUV plane information.
//
// Parameters:
//   - data: VP8-encoded video frame
//
// Returns:
//   - *VideoFrame: Decoded video frame in YUV420 format
//   - error: Any error that occurred during decoding
func (c *VP8Codec) DecodeFrame(data []byte) (*VideoFrame, error) {
	logrus.WithFields(logrus.Fields{
		"function":  "VP8Codec.DecodeFrame",
		"data_size": len(data),
	}).Debug("Starting VP8 frame decoding")

	frame, err := c.processor.ProcessIncomingLegacy(data)

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "VP8Codec.DecodeFrame",
			"error":    err.Error(),
		}).Error("Failed to decode VP8 frame")
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"function":     "VP8Codec.DecodeFrame",
		"frame_width":  frame.Width,
		"frame_height": frame.Height,
	}).Debug("VP8 frame decoded successfully")

	return frame, nil
}

// SetBitRate updates the codec bit rate.
//
// Configures both encoder and any future decoder settings to use
// the specified bit rate.
func (c *VP8Codec) SetBitRate(bitRate uint32) error {
	logrus.WithFields(logrus.Fields{
		"function": "VP8Codec.SetBitRate",
		"bitrate":  bitRate,
	}).Info("Setting VP8 codec bit rate")

	err := c.processor.SetBitRate(bitRate)

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "VP8Codec.SetBitRate",
			"bitrate":  bitRate,
			"error":    err.Error(),
		}).Error("Failed to set VP8 codec bit rate")
		return err
	}

	logrus.WithFields(logrus.Fields{
		"function": "VP8Codec.SetBitRate",
		"bitrate":  bitRate,
	}).Info("VP8 codec bit rate set successfully")

	return nil
}

// GetSupportedResolutions returns the resolutions supported by this codec.
//
// VP8 supports a wide range of resolutions, these are common ones for video calls.
func (c *VP8Codec) GetSupportedResolutions() []Resolution {
	logrus.WithFields(logrus.Fields{
		"function": "VP8Codec.GetSupportedResolutions",
	}).Debug("Retrieving supported VP8 resolutions")

	resolutions := []Resolution{
		{Width: 160, Height: 120},   // QQVGA
		{Width: 320, Height: 240},   // QVGA
		{Width: 640, Height: 480},   // VGA
		{Width: 800, Height: 600},   // SVGA
		{Width: 1024, Height: 768},  // XGA
		{Width: 1280, Height: 720},  // HD 720p
		{Width: 1920, Height: 1080}, // HD 1080p
	}

	logrus.WithFields(logrus.Fields{
		"function":         "VP8Codec.GetSupportedResolutions",
		"resolution_count": len(resolutions),
	}).Debug("Retrieved supported VP8 resolutions")

	return resolutions
}

// GetSupportedBitRates returns the bit rates supported by this codec.
//
// VP8 supports a wide range of bit rates suitable for different quality levels.
func (c *VP8Codec) GetSupportedBitRates() []uint32 {
	logrus.WithFields(logrus.Fields{
		"function": "VP8Codec.GetSupportedBitRates",
	}).Debug("Retrieving supported VP8 bit rates")

	bitRates := []uint32{64000, 128000, 256000, 512000, 1000000, 2000000, 4000000, 8000000}

	logrus.WithFields(logrus.Fields{
		"function":      "VP8Codec.GetSupportedBitRates",
		"bitrate_count": len(bitRates),
	}).Debug("Retrieved supported VP8 bit rates")

	return bitRates
}

// ValidateFrameSize checks if the frame dimensions are valid for VP8 encoding.
//
// VP8 requires dimensions to be multiples of certain values for optimal encoding.
func (c *VP8Codec) ValidateFrameSize(width, height uint16) error {
	logrus.WithFields(logrus.Fields{
		"function": "VP8Codec.ValidateFrameSize",
		"width":    width,
		"height":   height,
	}).Debug("Validating VP8 frame size")

	// VP8 requires width and height to be even numbers
	if width%2 != 0 {
		logrus.WithFields(logrus.Fields{
			"function": "VP8Codec.ValidateFrameSize",
			"width":    width,
			"error":    "width must be even",
		}).Error("Invalid VP8 frame width")
		return fmt.Errorf("invalid VP8 frame width: %d - must be even", width)
	}
	if height%2 != 0 {
		logrus.WithFields(logrus.Fields{
			"function": "VP8Codec.ValidateFrameSize",
			"height":   height,
			"error":    "height must be even",
		}).Error("Invalid VP8 frame height")
		return fmt.Errorf("invalid VP8 frame height: %d - must be even", height)
	}

	// Check minimum dimensions
	if width < 16 || height < 16 {
		logrus.WithFields(logrus.Fields{
			"function": "VP8Codec.ValidateFrameSize",
			"width":    width,
			"height":   height,
			"error":    "dimensions too small",
		}).Error("VP8 frame size below minimum")
		return fmt.Errorf("invalid VP8 frame size: %dx%d - minimum size is 16x16", width, height)
	}

	// Check maximum dimensions (VP8 supports up to 16383x16383)
	if width > 16383 || height > 16383 {
		logrus.WithFields(logrus.Fields{
			"function": "VP8Codec.ValidateFrameSize",
			"width":    width,
			"height":   height,
			"error":    "dimensions too large",
		}).Error("VP8 frame size above maximum")
		return fmt.Errorf("invalid VP8 frame size: %dx%d - maximum size is 16383x16383", width, height)
	}

	logrus.WithFields(logrus.Fields{
		"function": "VP8Codec.ValidateFrameSize",
		"width":    width,
		"height":   height,
	}).Debug("VP8 frame size validation successful")

	return nil
}

// Close releases codec resources.
func (c *VP8Codec) Close() error {
	logrus.WithFields(logrus.Fields{
		"function": "VP8Codec.Close",
	}).Info("Closing VP8 codec")

	if c.processor != nil {
		err := c.processor.Close()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "VP8Codec.Close",
				"error":    err.Error(),
			}).Error("Failed to close VP8 processor")
			return err
		}
	}

	logrus.WithFields(logrus.Fields{
		"function": "VP8Codec.Close",
	}).Info("VP8 codec closed successfully")

	return nil
}

// Resolution represents a video resolution.
type Resolution struct {
	Width  uint16
	Height uint16
}

// String returns a string representation of the resolution.
func (r Resolution) String() string {
	return fmt.Sprintf("%dx%d", r.Width, r.Height)
}

// GetBitrateForResolution returns an appropriate bitrate for a given resolution.
//
// Provides reasonable default bitrates based on resolution for video calls.
func GetBitrateForResolution(resolution Resolution) uint32 {
	logrus.WithFields(logrus.Fields{
		"function":   "GetBitrateForResolution",
		"resolution": resolution.String(),
	}).Debug("Calculating appropriate bitrate for resolution")

	pixels := uint32(resolution.Width) * uint32(resolution.Height)
	var bitrate uint32

	switch {
	case pixels <= 19200: // 160x120 and smaller
		bitrate = 64000 // 64 kbps
	case pixels <= 76800: // 320x240 and smaller
		bitrate = 128000 // 128 kbps
	case pixels <= 307200: // 640x480 and smaller
		bitrate = 512000 // 512 kbps
	case pixels <= 480000: // 800x600 and smaller
		bitrate = 1000000 // 1 Mbps
	case pixels <= 786432: // 1024x768 and smaller
		bitrate = 1500000 // 1.5 Mbps
	case pixels <= 921600: // 1280x720 and smaller
		bitrate = 2000000 // 2 Mbps
	case pixels <= 2073600: // 1920x1080 and smaller
		bitrate = 4000000 // 4 Mbps
	default:
		bitrate = 8000000 // 8 Mbps for larger resolutions
	}

	logrus.WithFields(logrus.Fields{
		"function":   "GetBitrateForResolution",
		"resolution": resolution.String(),
		"pixels":     pixels,
		"bitrate":    bitrate,
	}).Debug("Calculated bitrate for resolution")

	return bitrate
}
