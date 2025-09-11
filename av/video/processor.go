// Package video provides video processing capabilities for ToxAV.
//
// This package handles video encoding, decoding, scaling, and effects
// processing for audio/video calls. It integrates with pure Go video
// libraries to provide VP8 codec support and video processing.
//
// The video processing pipeline:
//
//	YUV420 Input → Scaling → Effects → VP8 Encoding → RTP Packetization
//	YUV420 Output ← Scaling ← Effects ← VP8 Decoding ← RTP Depacketization
//
// This package follows the same patterns as the audio package for consistency.
package video

import (
	"fmt"
)

// Encoder provides a simplified video encoder interface.
//
// For Phase 3 implementation, this starts as a YUV420 passthrough encoder
// that can be enhanced with proper VP8 encoding in future phases.
// This follows the SIMPLICITY RULE: provide basic functionality first.
type Encoder interface {
	// Encode converts YUV420 frame to encoded video data
	Encode(frame *VideoFrame) ([]byte, error)
	// SetBitRate updates the target encoding bit rate
	SetBitRate(bitRate uint32) error
	// Close releases encoder resources
	Close() error
}

// SimpleVP8Encoder is a basic encoder that passes through YUV420 data.
// This provides immediate functionality while maintaining the interface
// for future VP8 encoder integration.
type SimpleVP8Encoder struct {
	bitRate uint32
	width   uint16
	height  uint16
}

// NewSimpleVP8Encoder creates a new YUV420 passthrough encoder.
func NewSimpleVP8Encoder(width, height uint16, bitRate uint32) *SimpleVP8Encoder {
	return &SimpleVP8Encoder{
		bitRate: bitRate,
		width:   width,
		height:  height,
	}
}

// Encode passes through YUV420 data as-is for now.
// In future phases, this will be replaced with proper VP8 encoding.
func (e *SimpleVP8Encoder) Encode(frame *VideoFrame) ([]byte, error) {
	if frame.Width != e.width || frame.Height != e.height {
		return nil, fmt.Errorf("frame size mismatch: expected %dx%d, got %dx%d",
			e.width, e.height, frame.Width, frame.Height)
	}

	// For now, pack YUV420 data into a simple format
	// Format: [width:2][height:2][y_data][u_data][v_data]
	ySize := len(frame.Y)
	uSize := len(frame.U)
	vSize := len(frame.V)

	data := make([]byte, 4+ySize+uSize+vSize)

	// Pack dimensions (little-endian)
	data[0] = byte(frame.Width)
	data[1] = byte(frame.Width >> 8)
	data[2] = byte(frame.Height)
	data[3] = byte(frame.Height >> 8)

	// Pack YUV data
	offset := 4
	copy(data[offset:], frame.Y)
	offset += ySize
	copy(data[offset:], frame.U)
	offset += uSize
	copy(data[offset:], frame.V)

	return data, nil
}

// SetBitRate updates the target bit rate.
func (e *SimpleVP8Encoder) SetBitRate(bitRate uint32) error {
	e.bitRate = bitRate
	return nil
}

// Close releases encoder resources.
func (e *SimpleVP8Encoder) Close() error {
	return nil
}

// VideoFrame represents a video frame in YUV420 format.
//
// This type provides a structured representation of video data
// that can be processed by the video pipeline.
type VideoFrame struct {
	Width   uint16
	Height  uint16
	Y       []byte // Luminance plane
	U       []byte // Chrominance U plane
	V       []byte // Chrominance V plane
	YStride int    // Stride for Y plane
	UStride int    // Stride for U plane
	VStride int    // Stride for V plane
}

// Processor handles video processing for ToxAV video calls.
//
// Provides encoding/decoding pipeline using SimpleVP8Encoder for encoding and
// support for future VP8 decoder integration. Follows the same patterns
// as the audio processor for consistency.
//
// The processing pipeline:
//   - Input validation and format checking
//   - Video encoding using SimpleVP8Encoder (future: proper VP8 encoding)
//   - Output formatting for network transmission
//   - Incoming data decoding and format conversion
type Processor struct {
	encoder Encoder
	bitRate uint32
	width   uint16
	height  uint16
}

// NewProcessor creates a new video processor instance.
//
// Initializes with standard settings suitable for video calling:
// - Default resolution: 640x480 (VGA)
// - Default bit rate: 512 kbps
// - SimpleVP8Encoder for basic functionality
func NewProcessor() *Processor {
	const (
		defaultWidth   = 640
		defaultHeight  = 480
		defaultBitRate = 512000 // 512 kbps
	)

	return &Processor{
		encoder: NewSimpleVP8Encoder(defaultWidth, defaultHeight, defaultBitRate),
		bitRate: defaultBitRate,
		width:   defaultWidth,
		height:  defaultHeight,
	}
}

// NewProcessorWithSettings creates a processor with specific settings.
func NewProcessorWithSettings(width, height uint16, bitRate uint32) *Processor {
	return &Processor{
		encoder: NewSimpleVP8Encoder(width, height, bitRate),
		bitRate: bitRate,
		width:   width,
		height:  height,
	}
}

// ProcessOutgoing processes outgoing video data for transmission.
//
// Validates the input frame and encodes it using the configured encoder.
// Handles format validation and error reporting.
//
// Parameters:
//   - frame: Video frame to encode and transmit
//
// Returns:
//   - []byte: Encoded video data ready for transmission
//   - error: Any error that occurred during processing
func (p *Processor) ProcessOutgoing(frame *VideoFrame) ([]byte, error) {
	if frame == nil {
		return nil, fmt.Errorf("video frame cannot be nil")
	}

	// Validate frame dimensions
	if frame.Width == 0 || frame.Height == 0 {
		return nil, fmt.Errorf("invalid frame dimensions: %dx%d", frame.Width, frame.Height)
	}

	// Validate YUV data
	expectedYSize := int(frame.Width) * int(frame.Height)
	expectedUVSize := expectedYSize / 4 // U and V are quarter size in YUV420

	if len(frame.Y) != expectedYSize {
		return nil, fmt.Errorf("invalid Y plane size: expected %d, got %d", expectedYSize, len(frame.Y))
	}
	if len(frame.U) != expectedUVSize {
		return nil, fmt.Errorf("invalid U plane size: expected %d, got %d", expectedUVSize, len(frame.U))
	}
	if len(frame.V) != expectedUVSize {
		return nil, fmt.Errorf("invalid V plane size: expected %d, got %d", expectedUVSize, len(frame.V))
	}

	// Encode the frame
	return p.encoder.Encode(frame)
}

// ProcessIncoming processes incoming video data from transmission.
//
// Decodes received video data and converts it to VideoFrame format for display.
// For now, this handles the SimpleVP8Encoder format; future versions will
// handle proper VP8 decoding.
//
// Parameters:
//   - data: Encoded video data received from network
//
// Returns:
//   - *VideoFrame: Decoded video frame
//   - error: Any error that occurred during processing
func (p *Processor) ProcessIncoming(data []byte) (*VideoFrame, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("invalid video data: too short (%d bytes)", len(data))
	}

	// Unpack dimensions (little-endian)
	width := uint16(data[0]) | (uint16(data[1]) << 8)
	height := uint16(data[2]) | (uint16(data[3]) << 8)

	// Validate dimensions
	if width == 0 || height == 0 {
		return nil, fmt.Errorf("invalid frame dimensions in data: %dx%d", width, height)
	}

	// Calculate expected sizes
	ySize := int(width) * int(height)
	uvSize := ySize / 4
	expectedDataSize := 4 + ySize + uvSize + uvSize

	if len(data) != expectedDataSize {
		return nil, fmt.Errorf("invalid video data size: expected %d, got %d", expectedDataSize, len(data))
	}

	// Create frame and unpack YUV data
	frame := &VideoFrame{
		Width:   width,
		Height:  height,
		Y:       make([]byte, ySize),
		U:       make([]byte, uvSize),
		V:       make([]byte, uvSize),
		YStride: int(width),
		UStride: int(width) / 2,
		VStride: int(width) / 2,
	}

	// Unpack YUV data
	offset := 4
	copy(frame.Y, data[offset:offset+ySize])
	offset += ySize
	copy(frame.U, data[offset:offset+uvSize])
	offset += uvSize
	copy(frame.V, data[offset:offset+uvSize])

	return frame, nil
}

// SetBitRate updates the video encoding bit rate.
//
// Adjusts the encoder settings to use the specified bit rate for encoding.
// Also updates the processor's internal bit rate tracking.
//
// Parameters:
//   - bitRate: Target bit rate in bits per second
//
// Returns:
//   - error: Any error that occurred during bit rate update
func (p *Processor) SetBitRate(bitRate uint32) error {
	if bitRate == 0 {
		return fmt.Errorf("bit rate cannot be zero")
	}

	p.bitRate = bitRate
	return p.encoder.SetBitRate(bitRate)
}

// Close releases processor resources.
//
// Properly cleans up encoder and any other resources used by the processor.
func (p *Processor) Close() error {
	if p.encoder != nil {
		return p.encoder.Close()
	}
	return nil
}

// GetBitRate returns the current encoding bit rate.
func (p *Processor) GetBitRate() uint32 {
	return p.bitRate
}

// GetFrameSize returns the current frame dimensions.
func (p *Processor) GetFrameSize() (width, height uint16) {
	return p.width, p.height
}

// SetFrameSize updates the frame dimensions and recreates the encoder.
func (p *Processor) SetFrameSize(width, height uint16) error {
	if width == 0 || height == 0 {
		return fmt.Errorf("invalid frame dimensions: %dx%d", width, height)
	}

	// Close old encoder
	if p.encoder != nil {
		p.encoder.Close()
	}

	// Create new encoder with new dimensions
	p.encoder = NewSimpleVP8Encoder(width, height, p.bitRate)
	p.width = width
	p.height = height

	return nil
}
