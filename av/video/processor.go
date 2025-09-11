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
	"time"

	"github.com/sirupsen/logrus"
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
	logrus.WithFields(logrus.Fields{
		"function": "NewSimpleVP8Encoder",
		"width":    width,
		"height":   height,
		"bit_rate": bitRate,
	}).Info("Creating new VP8 encoder")

	encoder := &SimpleVP8Encoder{
		bitRate: bitRate,
		width:   width,
		height:  height,
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewSimpleVP8Encoder",
		"width":    width,
		"height":   height,
		"bit_rate": bitRate,
	}).Info("VP8 encoder created successfully")

	return encoder
}

// Encode passes through YUV420 data as-is for now.
// In future phases, this will be replaced with proper VP8 encoding.
func (e *SimpleVP8Encoder) Encode(frame *VideoFrame) ([]byte, error) {
	logrus.WithFields(logrus.Fields{
		"function":     "SimpleVP8Encoder.Encode",
		"frame_width":  frame.Width,
		"frame_height": frame.Height,
		"encoder_width": e.width,
		"encoder_height": e.height,
		"y_data_size":  len(frame.Y),
		"u_data_size":  len(frame.U),
		"v_data_size":  len(frame.V),
	}).Debug("Encoding video frame")

	if frame.Width != e.width || frame.Height != e.height {
		logrus.WithFields(logrus.Fields{
			"function":       "SimpleVP8Encoder.Encode",
			"expected_width": e.width,
			"expected_height": e.height,
			"actual_width":   frame.Width,
			"actual_height":  frame.Height,
			"error":          "frame size mismatch",
		}).Error("Frame dimension validation failed")
		return nil, fmt.Errorf("frame size mismatch: expected %dx%d, got %dx%d",
			e.width, e.height, frame.Width, frame.Height)
	}

	// For now, pack YUV420 data into a simple format
	// Format: [width:2][height:2][y_data][u_data][v_data]
	ySize := len(frame.Y)
	uSize := len(frame.U)
	vSize := len(frame.V)

	data := make([]byte, 4+ySize+uSize+vSize)

	logrus.WithFields(logrus.Fields{
		"function":    "SimpleVP8Encoder.Encode",
		"y_size":      ySize,
		"u_size":      uSize,
		"v_size":      vSize,
		"total_size":  len(data),
	}).Debug("Packing YUV420 data")

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

	logrus.WithFields(logrus.Fields{
		"function":     "SimpleVP8Encoder.Encode",
		"output_size":  len(data),
		"frame_width":  frame.Width,
		"frame_height": frame.Height,
	}).Debug("Video frame encoding completed")

	return data, nil
}

// SetBitRate updates the target bit rate.
func (e *SimpleVP8Encoder) SetBitRate(bitRate uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":     "SimpleVP8Encoder.SetBitRate",
		"old_bit_rate": e.bitRate,
		"new_bit_rate": bitRate,
	}).Info("Updating VP8 encoder bit rate")

	e.bitRate = bitRate

	logrus.WithFields(logrus.Fields{
		"function": "SimpleVP8Encoder.SetBitRate",
		"bit_rate": bitRate,
	}).Info("VP8 encoder bit rate updated successfully")

	return nil
}

// Close releases encoder resources.
func (e *SimpleVP8Encoder) Close() error {
	logrus.WithFields(logrus.Fields{
		"function": "SimpleVP8Encoder.Close",
		"bit_rate": e.bitRate,
		"width":    e.width,
		"height":   e.height,
	}).Info("Closing VP8 encoder")

	// No resources to clean up for simple encoder
	logrus.WithFields(logrus.Fields{
		"function": "SimpleVP8Encoder.Close",
	}).Debug("VP8 encoder closed successfully (no resources to release)")

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

// Processor manages the complete video processing pipeline.
//
// Handles the full video processing flow:
//
//	YUV420 Input → Scaling → Effects → VP8 Encoding → RTP Packetization
//	YUV420 Output ← Scaling ← Effects ← VP8 Decoding ← RTP Depacketization
//
// Features:
//   - Encoding and decoding pipeline management
//   - Optional video scaling to different resolutions
//   - Effect chain processing (brightness, contrast, etc.)
//   - RTP packetization for network transmission
//   - Bitrate and quality management
type Processor struct {
	encoder      Encoder
	scaler       *Scaler
	effects      *EffectChain
	packetizer   *RTPPacketizer
	depacketizer *RTPDepacketizer
	bitRate      uint32
	width        uint16
	height       uint16
	ssrc         uint32 // RTP source identifier
	pictureID    uint16 // Current picture ID for VP8
}

// NewProcessor creates a new video processor instance.
//
// Initializes with standard settings suitable for video calling:
// - Default resolution: 640x480 (VGA)
// - Default bit rate: 512 kbps
// - SimpleVP8Encoder for basic functionality
// - Complete pipeline with scaling, effects, and RTP support
func NewProcessor() *Processor {
	logrus.WithFields(logrus.Fields{
		"function": "NewProcessor",
	}).Info("Creating new video processor with default settings")

	const (
		defaultWidth   = 640
		defaultHeight  = 480
		defaultBitRate = 512000 // 512 kbps
		defaultSSRC    = 1      // Default SSRC
	)

	processor := &Processor{
		encoder:      NewSimpleVP8Encoder(defaultWidth, defaultHeight, defaultBitRate),
		scaler:       NewScaler(),
		effects:      NewEffectChain(),
		packetizer:   NewRTPPacketizer(defaultSSRC),
		depacketizer: NewRTPDepacketizer(),
		bitRate:      defaultBitRate,
		width:        defaultWidth,
		height:       defaultHeight,
		ssrc:         defaultSSRC,
		pictureID:    1,
	}

	logrus.WithFields(logrus.Fields{
		"function":  "NewProcessor",
		"width":     defaultWidth,
		"height":    defaultHeight,
		"bit_rate":  defaultBitRate,
		"ssrc":      defaultSSRC,
	}).Info("Video processor created successfully")

	return processor
}

// NewProcessorWithSettings creates a processor with specific settings.
func NewProcessorWithSettings(width, height uint16, bitRate uint32) *Processor {
	logrus.WithFields(logrus.Fields{
		"function": "NewProcessorWithSettings",
		"width":    width,
		"height":   height,
		"bit_rate": bitRate,
	}).Info("Creating new video processor with custom settings")

	ssrc := uint32(1) // Default SSRC

	processor := &Processor{
		encoder:      NewSimpleVP8Encoder(width, height, bitRate),
		scaler:       NewScaler(),
		effects:      NewEffectChain(),
		packetizer:   NewRTPPacketizer(ssrc),
		depacketizer: NewRTPDepacketizer(),
		bitRate:      bitRate,
		width:        width,
		height:       height,
		ssrc:         ssrc,
		pictureID:    1,
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewProcessorWithSettings",
		"width":    width,
		"height":   height,
		"bit_rate": bitRate,
		"ssrc":     ssrc,
	}).Info("Video processor with custom settings created successfully")

	return processor
}

// ProcessOutgoing processes outgoing video data through the complete pipeline.
//
// Complete processing pipeline:
// 1. Input validation and frame checking
// 2. Optional scaling to target resolution
// 3. Apply effect chain (brightness, contrast, etc.)
// 4. VP8 encoding
// 5. RTP packetization for network transmission
//
// Parameters:
//   - frame: Video frame to process and transmit
//
// Returns:
//   - []RTPPacket: RTP packets ready for network transmission
//   - error: Any error that occurred during processing
func (p *Processor) ProcessOutgoing(frame *VideoFrame) ([]RTPPacket, error) {
	// Step 1: Validate frame and dimensions
	if err := p.validateFrame(frame); err != nil {
		return nil, err
	}

	// Step 2: Apply scaling if needed
	processedFrame, err := p.applyScaling(frame)
	if err != nil {
		return nil, err
	}

	// Step 3: Apply effects chain if configured
	processedFrame, err = p.applyEffects(processedFrame)
	if err != nil {
		return nil, err
	}

	// Step 4: Encode and packetize for network transmission
	packets, err := p.encodeAndPacketize(processedFrame)
	if err != nil {
		return nil, err
	}

	return packets, nil
}

// validateFrame validates that the video frame is properly formatted and contains valid data.
// It checks frame dimensions and YUV plane sizes according to YUV420 format requirements.
func (p *Processor) validateFrame(frame *VideoFrame) error {
	if frame == nil {
		return fmt.Errorf("video frame cannot be nil")
	}

	// Validate frame dimensions
	if frame.Width == 0 || frame.Height == 0 {
		return fmt.Errorf("invalid frame dimensions: %dx%d", frame.Width, frame.Height)
	}

	// Validate YUV data
	expectedYSize := int(frame.Width) * int(frame.Height)
	expectedUVSize := int(frame.Width/2) * int(frame.Height/2)

	if len(frame.Y) < expectedYSize {
		return fmt.Errorf("Y plane too small: got %d, expected %d", len(frame.Y), expectedYSize)
	}
	if len(frame.U) < expectedUVSize {
		return fmt.Errorf("U plane too small: got %d, expected %d", len(frame.U), expectedUVSize)
	}
	if len(frame.V) < expectedUVSize {
		return fmt.Errorf("V plane too small: got %d, expected %d", len(frame.V), expectedUVSize)
	}

	return nil
}

// applyScaling scales the frame to the target resolution if scaling is required.
// Returns the original frame if no scaling is needed, or the scaled frame if scaling was applied.
func (p *Processor) applyScaling(frame *VideoFrame) (*VideoFrame, error) {
	if !p.scaler.IsScalingRequired(frame.Width, frame.Height, p.width, p.height) {
		return frame, nil
	}

	scaledFrame, err := p.scaler.Scale(frame, p.width, p.height)
	if err != nil {
		return nil, fmt.Errorf("scaling failed: %v", err)
	}

	return scaledFrame, nil
}

// applyEffects applies the configured effects chain to the video frame.
// Returns the original frame if no effects are configured, or the processed frame with effects applied.
func (p *Processor) applyEffects(frame *VideoFrame) (*VideoFrame, error) {
	if p.effects.GetEffectCount() == 0 {
		return frame, nil
	}

	effectFrame, err := p.effects.Apply(frame)
	if err != nil {
		return nil, fmt.Errorf("effects processing failed: %v", err)
	}

	return effectFrame, nil
}

// encodeAndPacketize encodes the video frame with VP8 and packetizes it into RTP packets.
// This handles the final stage of video processing, creating network-ready packets with proper
// timestamps, sequence numbers, and VP8-specific headers.
func (p *Processor) encodeAndPacketize(frame *VideoFrame) ([]RTPPacket, error) {
	// Encode with VP8
	encodedData, err := p.encoder.Encode(frame)
	if err != nil {
		return nil, fmt.Errorf("encoding failed: %v", err)
	}

	// RTP packetization
	timestamp := p.generateTimestamp() // 90kHz timestamp for video
	packets, err := p.packetizer.PacketizeFrame(encodedData, timestamp, p.pictureID)
	if err != nil {
		return nil, fmt.Errorf("RTP packetization failed: %v", err)
	}

	// Increment picture ID for next frame
	p.pictureID++
	if p.pictureID == 0 { // Handle 16-bit wrap
		p.pictureID = 1
	}

	return packets, nil
}

// ProcessOutgoingLegacy provides backward compatibility with []byte return.
//
// This method bypasses the RTP pipeline and returns simple encoded data
// for compatibility with existing tests and legacy code.
func (p *Processor) ProcessOutgoingLegacy(frame *VideoFrame) ([]byte, error) {
	if frame == nil {
		return nil, fmt.Errorf("video frame cannot be nil")
	}

	// Validate frame dimensions
	if frame.Width == 0 || frame.Height == 0 {
		return nil, fmt.Errorf("invalid frame dimensions: %dx%d", frame.Width, frame.Height)
	}

	// Validate YUV data
	expectedYSize := int(frame.Width) * int(frame.Height)
	expectedUVSize := int(frame.Width/2) * int(frame.Height/2)

	if len(frame.Y) < expectedYSize {
		return nil, fmt.Errorf("Y plane too small: got %d, expected %d", len(frame.Y), expectedYSize)
	}
	if len(frame.U) < expectedUVSize {
		return nil, fmt.Errorf("U plane too small: got %d, expected %d", len(frame.U), expectedUVSize)
	}
	if len(frame.V) < expectedUVSize {
		return nil, fmt.Errorf("V plane too small: got %d, expected %d", len(frame.V), expectedUVSize)
	}

	// Step 1: Scale to target resolution if needed
	processedFrame := frame
	if p.scaler.IsScalingRequired(frame.Width, frame.Height, p.width, p.height) {
		scaledFrame, err := p.scaler.Scale(frame, p.width, p.height)
		if err != nil {
			return nil, fmt.Errorf("scaling failed: %v", err)
		}
		processedFrame = scaledFrame
	}

	// Step 2: Apply effects chain
	if p.effects.GetEffectCount() > 0 {
		effectFrame, err := p.effects.Apply(processedFrame)
		if err != nil {
			return nil, fmt.Errorf("effects processing failed: %v", err)
		}
		processedFrame = effectFrame
	}

	// Step 3: Encode with VP8 (no RTP packetization)
	return p.encoder.Encode(processedFrame)
}

// ProcessIncoming processes incoming RTP packets through the depacketization pipeline.
//
// Complete processing pipeline:
// 1. RTP depacketization and frame reassembly
// 2. VP8 decoding
// 3. Apply inverse effects (if any)
// 4. Optional scaling to output resolution
//
// Parameters:
//   - packet: RTP packet to process
//
// Returns:
//   - *VideoFrame: Decoded video frame (nil if frame not complete)
//   - error: Any error that occurred during processing
func (p *Processor) ProcessIncoming(packet RTPPacket) (*VideoFrame, error) {
	// Step 1: RTP depacketization
	frameData, pictureID, err := p.depacketizer.ProcessPacket(packet)
	if err != nil {
		return nil, fmt.Errorf("RTP depacketization failed: %v", err)
	}

	// Frame not complete yet
	if frameData == nil {
		return nil, nil
	}

	// Step 2: Decode frame data
	frame, err := p.decodeFrameData(frameData)
	if err != nil {
		return nil, fmt.Errorf("frame decoding failed (PictureID %d): %v", pictureID, err)
	}

	// Step 3: Apply inverse effects if needed
	if p.effects.GetEffectCount() > 0 {
		// For now, effects are not invertible, so we skip this step
		// Future enhancement: implement effect inversion
	}

	// Step 4: Scale to output resolution if needed
	// For now, return the decoded frame as-is
	// Future enhancement: implement output scaling

	return frame, nil
}

// ProcessIncomingLegacy provides backward compatibility with []byte input.
func (p *Processor) ProcessIncomingLegacy(data []byte) (*VideoFrame, error) {
	return p.decodeFrameData(data)
}

// decodeFrameData decodes SimpleVP8Encoder format back to VideoFrame.
func (p *Processor) decodeFrameData(data []byte) (*VideoFrame, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("data too short: %d bytes", len(data))
	}

	// Unpack dimensions (little-endian)
	width := uint16(data[0]) | uint16(data[1])<<8
	height := uint16(data[2]) | uint16(data[3])<<8

	// Calculate expected sizes
	ySize := int(width) * int(height)
	uvSize := ySize / 4 // U and V are quarter size

	expectedSize := 4 + ySize + uvSize + uvSize
	if len(data) != expectedSize {
		return nil, fmt.Errorf("invalid data size: expected %d, got %d", expectedSize, len(data))
	}

	// Create frame
	frame := &VideoFrame{
		Width:   width,
		Height:  height,
		YStride: int(width),
		UStride: int(width) / 2,
		VStride: int(width) / 2,
		Y:       make([]byte, ySize),
		U:       make([]byte, uvSize),
		V:       make([]byte, uvSize),
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

// generateTimestamp creates a 90kHz timestamp for video RTP.
func (p *Processor) generateTimestamp() uint32 {
	// Use current time in 90kHz units (standard for video RTP)
	return uint32(time.Now().UnixNano() / 1000 * 90 / 1000000)
}

// SetBitRate updates the target bit rate for encoding.
func (p *Processor) SetBitRate(bitRate uint32) error {
	if bitRate == 0 {
		return fmt.Errorf("bitrate cannot be zero")
	}
	p.bitRate = bitRate
	return p.encoder.SetBitRate(bitRate)
}

// Close releases all processor resources.
func (p *Processor) Close() error {
	return p.encoder.Close()
}

// GetBitRate returns the current bit rate setting.
func (p *Processor) GetBitRate() uint32 {
	return p.bitRate
}

// GetFrameSize returns the current frame dimensions.
func (p *Processor) GetFrameSize() (width, height uint16) {
	return p.width, p.height
}

// SetFrameSize updates the target frame dimensions.
func (p *Processor) SetFrameSize(width, height uint16) error {
	if width == 0 || height == 0 {
		return fmt.Errorf("invalid dimensions: %dx%d", width, height)
	}

	p.width = width
	p.height = height

	// Update encoder dimensions
	p.encoder = NewSimpleVP8Encoder(width, height, p.bitRate)

	return nil
}

// GetEffectChain returns the effect chain for modification.
func (p *Processor) GetEffectChain() *EffectChain {
	return p.effects
}

// GetScaler returns the scaler for configuration.
func (p *Processor) GetScaler() *Scaler {
	return p.scaler
}

// GetRTPStats returns RTP statistics.
func (p *Processor) GetRTPStats() (sequenceNumber uint16, timestamp uint32, bufferedFrames int) {
	seq, ts := p.packetizer.GetStats()
	buffered := p.depacketizer.GetBufferedFrameCount()
	return seq, ts, buffered
}

// SetRTPPacketSize configures the maximum RTP packet size.
func (p *Processor) SetRTPPacketSize(size int) error {
	return p.packetizer.SetMaxPacketSize(size)
}
