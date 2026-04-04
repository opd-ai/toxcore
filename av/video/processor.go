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
	"bytes"
	"fmt"
	"time"

	vp8enc "github.com/opd-ai/vp8"
	"github.com/sirupsen/logrus"
	vp8dec "golang.org/x/image/vp8"
)

// Encoder provides a video encoder interface for VP8 encoding.
//
// Implementations include RealVP8Encoder (using opd-ai/vp8 for actual
// VP8 compression with I-frame and P-frame support), SimpleVP8Encoder
// (YUV420 passthrough for testing), and optionally LibVPXEncoder (full
// VP8 via CGo libvpx when built with the 'libvpx' build tag).
type Encoder interface {
	// Encode converts YUV420 frame to encoded video data.
	// Depending on configuration, the output may be a key frame (I-frame)
	// or an inter frame (P-frame).
	Encode(frame *VideoFrame) ([]byte, error)
	// SetBitRate updates the target encoding bit rate
	SetBitRate(bitRate uint32) error
	// SupportsInterframe returns true if the encoder supports P-frames
	// (inter-frame prediction). I-frame-only encoders return false.
	SupportsInterframe() bool
	// SetKeyFrameInterval configures the maximum number of inter frames
	// between key frames. A value of 0 means every frame is a key frame.
	SetKeyFrameInterval(interval int)
	// ForceKeyFrame causes the next Encode call to produce a key frame,
	// resetting the inter-frame prediction chain.
	ForceKeyFrame()
	// Close releases encoder resources
	Close() error
}

// defaultFPS is the default frames-per-second used for the VP8 encoder.
const defaultFPS = 30

// defaultKeyFrameInterval is the default number of frames between key frames.
// At 30fps this produces one key frame per second.
const defaultKeyFrameInterval = 30

// defaultLoopFilterLevel is the loop filter strength for reconstructed
// reference frames. A moderate level reduces blocking artifacts between
// inter-frame and key-frame boundaries.
const defaultLoopFilterLevel = 20

// RealVP8Encoder wraps the opd-ai/vp8 encoder to produce actual VP8 bitstreams.
//
// This encoder produces RFC 6386 compliant VP8 bitstreams with both key frames
// (I-frames) and inter frames (P-frames) using motion estimation. It is
// compatible with standard VP8 decoders and WebRTC stacks.
type RealVP8Encoder struct {
	enc     *vp8enc.Encoder
	bitRate uint32
	width   uint16
	height  uint16
}

// NewRealVP8Encoder creates a new VP8 encoder using the opd-ai/vp8 library.
//
// The encoder is configured for inter-frame encoding by default:
// key frames are emitted every 30 frames (1 second at 30fps) with
// inter frames (P-frames) in between. The loop filter is enabled at
// a moderate level to reduce blocking artifacts in reference frames.
//
// Parameters:
//   - width, height: Frame dimensions (must be positive, even integers)
//   - bitRate: Target encoding bit rate in bits per second
//
// Returns an error if the underlying VP8 encoder cannot be created
// (e.g. invalid dimensions).
func NewRealVP8Encoder(width, height uint16, bitRate uint32) (*RealVP8Encoder, error) {
	logrus.WithFields(logrus.Fields{
		"function": "NewRealVP8Encoder",
		"width":    width,
		"height":   height,
		"bit_rate": bitRate,
	}).Info("Creating new real VP8 encoder")

	enc, err := vp8enc.NewEncoder(int(width), int(height), defaultFPS)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewRealVP8Encoder",
			"width":    width,
			"height":   height,
			"error":    err.Error(),
		}).Error("Failed to create VP8 encoder, dimensions may be invalid")
		return nil, fmt.Errorf("failed to create VP8 encoder for %dx%d: %w", width, height, err)
	}

	enc.SetBitrate(int(bitRate))
	enc.SetKeyFrameInterval(defaultKeyFrameInterval)
	enc.SetLoopFilterLevel(defaultLoopFilterLevel)

	logrus.WithFields(logrus.Fields{
		"function":           "NewRealVP8Encoder",
		"width":              width,
		"height":             height,
		"bit_rate":           bitRate,
		"key_frame_interval": defaultKeyFrameInterval,
		"loop_filter_level":  defaultLoopFilterLevel,
	}).Info("Real VP8 encoder created with inter-frame support")

	return &RealVP8Encoder{
		enc:     enc,
		bitRate: bitRate,
		width:   width,
		height:  height,
	}, nil
}

// Encode encodes a YUV420 video frame into a VP8 bitstream.
//
// The input frame must have dimensions matching the encoder configuration.
// The output is an RFC 6386 compliant VP8 bitstream — either a key frame
// (I-frame) or an inter frame (P-frame) depending on the configured key
// frame interval and encoder state. Plane strides are respected: if a
// plane has a stride larger than its row width, only the active pixels
// are copied into the I420 buffer sent to the encoder.
func (e *RealVP8Encoder) Encode(frame *VideoFrame) ([]byte, error) {
	if frame == nil {
		return nil, fmt.Errorf("video frame cannot be nil")
	}

	logrus.WithFields(logrus.Fields{
		"function":       "RealVP8Encoder.Encode",
		"frame_width":    frame.Width,
		"frame_height":   frame.Height,
		"encoder_width":  e.width,
		"encoder_height": e.height,
	}).Debug("Encoding video frame with VP8")

	if frame.Width != e.width || frame.Height != e.height {
		logrus.WithFields(logrus.Fields{
			"function":        "RealVP8Encoder.Encode",
			"expected_width":  e.width,
			"expected_height": e.height,
			"actual_width":    frame.Width,
			"actual_height":   frame.Height,
		}).Error("Frame dimension validation failed")
		return nil, fmt.Errorf("frame size mismatch: expected %dx%d, got %dx%d",
			e.width, e.height, frame.Width, frame.Height)
	}

	// Build raw I420 buffer respecting plane strides.
	w := int(frame.Width)
	h := int(frame.Height)
	uvW := w / 2
	uvH := h / 2
	ySize := w * h
	uvSize := uvW * uvH
	yuv := make([]byte, ySize+uvSize+uvSize)

	packPlane(yuv[:ySize], frame.Y, frame.YStride, w, h)
	packPlane(yuv[ySize:ySize+uvSize], frame.U, frame.UStride, uvW, uvH)
	packPlane(yuv[ySize+uvSize:], frame.V, frame.VStride, uvW, uvH)

	vp8Data, err := e.enc.Encode(yuv)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "RealVP8Encoder.Encode",
			"error":    err.Error(),
		}).Error("VP8 encoding failed")
		return nil, fmt.Errorf("VP8 encoding failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":     "RealVP8Encoder.Encode",
		"input_size":   len(yuv),
		"output_size":  len(vp8Data),
		"frame_width":  frame.Width,
		"frame_height": frame.Height,
	}).Debug("VP8 frame encoding completed")

	return vp8Data, nil
}

// packPlane copies pixel rows from a source plane (which may have a
// stride larger than the row width) into a tightly packed destination buffer.
func packPlane(dst, src []byte, stride, width, height int) {
	if stride == width || stride == 0 {
		copy(dst, src[:width*height])
		return
	}
	for y := 0; y < height; y++ {
		copy(dst[y*width:], src[y*stride:y*stride+width])
	}
}

// SetBitRate updates the target bit rate for the VP8 encoder.
func (e *RealVP8Encoder) SetBitRate(bitRate uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":     "RealVP8Encoder.SetBitRate",
		"old_bit_rate": e.bitRate,
		"new_bit_rate": bitRate,
	}).Info("Updating VP8 encoder bit rate")

	e.bitRate = bitRate
	e.enc.SetBitrate(int(bitRate))

	logrus.WithFields(logrus.Fields{
		"function": "RealVP8Encoder.SetBitRate",
		"bit_rate": bitRate,
	}).Info("VP8 encoder bit rate updated successfully")

	return nil
}

// Close releases encoder resources.
func (e *RealVP8Encoder) Close() error {
	logrus.WithFields(logrus.Fields{
		"function": "RealVP8Encoder.Close",
		"width":    e.width,
		"height":   e.height,
	}).Info("Closing VP8 encoder")

	return nil
}

// SupportsInterframe returns true because the opd-ai/vp8 encoder supports
// inter-frame prediction (P-frames) with motion estimation.
func (e *RealVP8Encoder) SupportsInterframe() bool {
	return true
}

// SetKeyFrameInterval configures the maximum number of inter frames between
// key frames. A value of 0 means every frame is a key frame (I-frame only).
// Negative values are clamped to 0.
func (e *RealVP8Encoder) SetKeyFrameInterval(interval int) {
	if interval < 0 {
		logrus.WithFields(logrus.Fields{
			"function":           "RealVP8Encoder.SetKeyFrameInterval",
			"requested_interval": interval,
			"applied_interval":   0,
		}).Warn("Negative key frame interval requested; clamping to 0")
		interval = 0
	}
	e.enc.SetKeyFrameInterval(interval)
}

// ForceKeyFrame causes the next Encode call to produce a key frame,
// resetting the inter-frame prediction chain.
func (e *RealVP8Encoder) ForceKeyFrame() {
	e.enc.ForceKeyFrame()
}

// SimpleVP8Encoder is a basic encoder that passes through YUV420 data
// with a 4-byte dimension header. It is retained only for testing and
// does not produce a real VP8 bitstream or provide runtime fallback.
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
	}).Info("Creating new simple VP8 encoder")

	return &SimpleVP8Encoder{
		bitRate: bitRate,
		width:   width,
		height:  height,
	}
}

// Encode passes through YUV420 data with a 4-byte dimension header.
func (e *SimpleVP8Encoder) Encode(frame *VideoFrame) ([]byte, error) {
	if frame.Width != e.width || frame.Height != e.height {
		return nil, fmt.Errorf("frame size mismatch: expected %dx%d, got %dx%d",
			e.width, e.height, frame.Width, frame.Height)
	}

	ySize := len(frame.Y)
	uSize := len(frame.U)
	vSize := len(frame.V)
	data := make([]byte, 4+ySize+uSize+vSize)

	// Write dimensions (little-endian)
	data[0] = byte(frame.Width)
	data[1] = byte(frame.Width >> 8)
	data[2] = byte(frame.Height)
	data[3] = byte(frame.Height >> 8)

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

// SupportsInterframe returns false because SimpleVP8Encoder is a
// passthrough encoder that does not perform any inter-frame prediction.
func (e *SimpleVP8Encoder) SupportsInterframe() bool {
	return false
}

// SetKeyFrameInterval is a no-op for the simple encoder.
func (e *SimpleVP8Encoder) SetKeyFrameInterval(_ int) {}

// ForceKeyFrame is a no-op for the simple encoder.
func (e *SimpleVP8Encoder) ForceKeyFrame() {}

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

// TimeProvider abstracts time operations for deterministic testing and
// prevents timing side-channel attacks by allowing controlled time injection.
type TimeProvider interface {
	Now() time.Time
}

// DefaultTimeProvider uses the standard library time functions.
type DefaultTimeProvider struct{}

// Now returns the current time.
func (DefaultTimeProvider) Now() time.Time { return time.Now() }

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
	encoder        Encoder
	scaler         *Scaler
	effects        *EffectChain
	packetizer     *RTPPacketizer
	depacketizer   *RTPDepacketizer
	bitRate        uint32
	width          uint16
	height         uint16
	ssrc           uint32 // RTP source identifier
	pictureID      uint16 // Current picture ID for VP8
	timeProvider   TimeProvider
	lastDecodedKey *VideoFrame // Cache of last successfully decoded key frame
}

// NewProcessor creates a new video processor instance.
//
// Initializes with standard settings suitable for video calling:
// - Default resolution: 640x480 (VGA)
// - Default bit rate: 512 kbps
// - RealVP8Encoder for actual VP8 compression
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

	enc, err := NewRealVP8Encoder(defaultWidth, defaultHeight, defaultBitRate)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewProcessor",
			"error":    err.Error(),
		}).Warn("Failed to create real VP8 encoder, falling back to simple encoder")
		enc = nil
	}

	var encoder Encoder
	if enc != nil {
		encoder = enc
	} else {
		encoder = NewSimpleVP8Encoder(defaultWidth, defaultHeight, defaultBitRate)
	}

	processor := &Processor{
		encoder:      encoder,
		scaler:       NewScaler(),
		effects:      NewEffectChain(),
		packetizer:   NewRTPPacketizer(defaultSSRC),
		depacketizer: NewRTPDepacketizer(),
		bitRate:      defaultBitRate,
		width:        defaultWidth,
		height:       defaultHeight,
		ssrc:         defaultSSRC,
		pictureID:    1,
		timeProvider: DefaultTimeProvider{},
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewProcessor",
		"width":    defaultWidth,
		"height":   defaultHeight,
		"bit_rate": defaultBitRate,
		"ssrc":     defaultSSRC,
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

	enc, err := NewRealVP8Encoder(width, height, bitRate)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewProcessorWithSettings",
			"error":    err.Error(),
		}).Warn("Failed to create real VP8 encoder, falling back to simple encoder")
		enc = nil
	}

	var encoder Encoder
	if enc != nil {
		encoder = enc
	} else {
		encoder = NewSimpleVP8Encoder(width, height, bitRate)
	}

	processor := &Processor{
		encoder:      encoder,
		scaler:       NewScaler(),
		effects:      NewEffectChain(),
		packetizer:   NewRTPPacketizer(ssrc),
		depacketizer: NewRTPDepacketizer(),
		bitRate:      bitRate,
		width:        width,
		height:       height,
		ssrc:         ssrc,
		pictureID:    1,
		timeProvider: DefaultTimeProvider{},
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
	if err := p.validateBasicFrameInput(frame); err != nil {
		return err
	}
	return p.validateYUVPlaneData(frame)
}

// applyScaling scales the frame to the target resolution if scaling is required.
// Returns the original frame if no scaling is needed, or the scaled frame if scaling was applied.
func (p *Processor) applyScaling(frame *VideoFrame) (*VideoFrame, error) {
	if !p.scaler.IsScalingRequired(frame.Width, frame.Height, p.width, p.height) {
		return frame, nil
	}

	scaledFrame, err := p.scaler.Scale(frame, p.width, p.height)
	if err != nil {
		return nil, fmt.Errorf("scaling failed: %w", err)
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
		return nil, fmt.Errorf("effects processing failed: %w", err)
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
		return nil, fmt.Errorf("encoding failed: %w", err)
	}

	// RTP packetization
	timestamp := p.generateTimestamp() // 90kHz timestamp for video
	packets, err := p.packetizer.PacketizeFrame(encodedData, timestamp, p.pictureID)
	if err != nil {
		return nil, fmt.Errorf("RTP packetization failed: %w", err)
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
	// Step 1: Validate input frame
	if err := p.validateBasicFrameInput(frame); err != nil {
		return nil, err
	}

	// Step 2: Validate YUV plane data integrity
	if err := p.validateYUVPlaneData(frame); err != nil {
		return nil, err
	}

	// Step 3: Apply scaling if required
	processedFrame, err := p.applyConditionalScaling(frame)
	if err != nil {
		return nil, err
	}

	// Step 4: Apply effects if configured
	processedFrame, err = p.applyConditionalEffects(processedFrame)
	if err != nil {
		return nil, err
	}

	// Step 5: Encode with VP8 (no RTP packetization)
	return p.encoder.Encode(processedFrame)
}

// validateBasicFrameInput validates basic frame input constraints for processing.
// It checks for nil frames and ensures frame dimensions are valid for video processing.
func (p *Processor) validateBasicFrameInput(frame *VideoFrame) error {
	if frame == nil {
		return fmt.Errorf("video frame cannot be nil")
	}

	// Validate frame dimensions
	if frame.Width == 0 || frame.Height == 0 {
		return fmt.Errorf("invalid frame dimensions: %dx%d", frame.Width, frame.Height)
	}

	return nil
}

// validateYUVPlaneData validates YUV420 plane data sizes according to format requirements.
// It ensures Y, U, and V planes contain sufficient data for the specified frame dimensions.
func (p *Processor) validateYUVPlaneData(frame *VideoFrame) error {
	// Calculate expected sizes for YUV420 format
	expectedYSize := int(frame.Width) * int(frame.Height)
	expectedUVSize := int(frame.Width/2) * int(frame.Height/2)

	// Validate Y plane size
	if len(frame.Y) < expectedYSize {
		return fmt.Errorf("y plane too small: got %d, expected %d", len(frame.Y), expectedYSize)
	}

	// Validate U plane size
	if len(frame.U) < expectedUVSize {
		return fmt.Errorf("u plane too small: got %d, expected %d", len(frame.U), expectedUVSize)
	}

	// Validate V plane size
	if len(frame.V) < expectedUVSize {
		return fmt.Errorf("v plane too small: got %d, expected %d", len(frame.V), expectedUVSize)
	}

	return nil
}

// applyConditionalScaling applies video scaling only when required based on target dimensions.
// Returns the original frame if no scaling is needed, or the scaled frame if scaling was applied.
func (p *Processor) applyConditionalScaling(frame *VideoFrame) (*VideoFrame, error) {
	processedFrame := frame
	if p.scaler.IsScalingRequired(frame.Width, frame.Height, p.width, p.height) {
		scaledFrame, err := p.scaler.Scale(frame, p.width, p.height)
		if err != nil {
			return nil, fmt.Errorf("scaling failed: %w", err)
		}
		processedFrame = scaledFrame
	}
	return processedFrame, nil
}

// applyConditionalEffects applies the configured effects chain only when effects are present.
// Returns the original frame if no effects are configured, or the processed frame with effects applied.
func (p *Processor) applyConditionalEffects(frame *VideoFrame) (*VideoFrame, error) {
	processedFrame := frame
	if p.effects.GetEffectCount() > 0 {
		effectFrame, err := p.effects.Apply(processedFrame)
		if err != nil {
			return nil, fmt.Errorf("effects processing failed: %w", err)
		}
		processedFrame = effectFrame
	}
	return processedFrame, nil
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
		return nil, fmt.Errorf("RTP depacketization failed: %w", err)
	}

	// Frame not complete yet
	if frameData == nil {
		return nil, nil
	}

	// Step 2: Decode frame data
	frame, err := p.decodeFrameData(frameData)
	if err != nil {
		return nil, fmt.Errorf("frame decoding failed (PictureID %d): %w", pictureID, err)
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

// isVP8KeyFrame returns true if the given VP8 bitstream starts with a key frame.
// Per RFC 6386 §9.1, the 3-byte frame tag encodes:
//   - bit  0:      frame type (0 = key, 1 = inter)
//   - bits 1-3:    version
//   - bit  4:      show_frame
//   - bits 5-23:   first partition size (19 bits)
//
// For key frames, bytes 3-5 must contain the VP8 start code 0x9D 0x01 0x2A.
// Returns false for data that is too short or has an invalid frame tag.
func isVP8KeyFrame(data []byte) bool {
	if len(data) < 3 {
		return false
	}
	isKey := (data[0] & 1) == 0
	// Validate that the first-partition size fits within the data.
	firstPartSize := (uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16) >> 5
	headerSize := uint32(3)
	if isKey {
		headerSize = 10 // 3-byte tag + 7-byte key-frame header
	}
	if uint32(len(data)) < headerSize+firstPartSize {
		return false
	}
	// For key frames, validate the VP8 start code at bytes 3-5.
	if isKey {
		if data[3] != 0x9D || data[4] != 0x01 || data[5] != 0x2A {
			return false
		}
	}
	return isKey
}

// isVP8InterFrame returns true if the given VP8 bitstream starts with a valid
// inter frame (P-frame). Validates the frame tag and first-partition size.
func isVP8InterFrame(data []byte) bool {
	if len(data) < 3 {
		return false
	}
	isKey := (data[0] & 1) == 0
	if isKey {
		return false
	}
	firstPartSize := (uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16) >> 5
	headerSize := uint32(3)
	return uint32(len(data)) >= headerSize+firstPartSize
}

// decodeFrameData decodes VP8-encoded data back to a VideoFrame.
//
// Uses golang.org/x/image/vp8 to decode key frames. Inter frames (P-frames)
// cannot be decoded by the standard library decoder and instead return the
// last successfully decoded key frame. This approach leverages the bandwidth
// savings of P-frame encoding on the wire while ensuring the receiver always
// has a displayable frame.
func (p *Processor) decodeFrameData(data []byte) (*VideoFrame, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("data too short for VP8: %d bytes", len(data))
	}

	if isVP8InterFrame(data) {
		// Valid inter frame (P-frame): the golang.org/x/image/vp8 decoder does
		// not support inter-frame decoding. Return a copy of the cached last key
		// frame so callers cannot mutate the processor's cached state.
		if p.lastDecodedKey != nil {
			logrus.WithFields(logrus.Fields{
				"function":  "Processor.decodeFrameData",
				"data_size": len(data),
			}).Debug("Received inter frame, returning cached key frame")
			return copyFrame(p.lastDecodedKey), nil
		}
		return nil, fmt.Errorf("inter frame received but no cached key frame available")
	}

	if !isVP8KeyFrame(data) {
		return nil, fmt.Errorf("invalid VP8 frame tag: not a valid key frame or inter frame")
	}

	frame, err := p.decodeKeyFrame(data)
	if err != nil {
		// If key frame decode fails, fall back to a copy of the cached frame.
		if p.lastDecodedKey != nil {
			logrus.WithFields(logrus.Fields{
				"function":  "Processor.decodeFrameData",
				"error":     err.Error(),
				"data_size": len(data),
			}).Warn("Key frame decode failed, returning cached frame")
			return copyFrame(p.lastDecodedKey), nil
		}
		return nil, err
	}

	// Cache a deep copy of the successfully decoded key frame so later caller
	// mutations to the returned frame do not affect the processor's fallback
	// state.
	p.lastDecodedKey = copyFrame(frame)
	return frame, nil
}

// decodeKeyFrame decodes a VP8 key frame bitstream into a VideoFrame.
func (p *Processor) decodeKeyFrame(data []byte) (*VideoFrame, error) {
	decoder := vp8dec.NewDecoder()
	decoder.Init(bytes.NewReader(data), len(data))

	fh, err := decoder.DecodeFrameHeader()
	if err != nil {
		return nil, fmt.Errorf("VP8 frame header decode failed: %w", err)
	}

	img, err := decoder.DecodeFrame()
	if err != nil {
		return nil, fmt.Errorf("VP8 frame decode failed: %w", err)
	}

	width := fh.Width
	height := fh.Height

	frame := &VideoFrame{
		Width:   uint16(width),
		Height:  uint16(height),
		YStride: width,
		UStride: width / 2,
		VStride: width / 2,
	}

	// Extract Y plane (handle stride differences from decoder)
	frame.Y = extractPlane(img.Y, img.YStride, width, height)
	// Extract Cb/Cr planes (half resolution for YUV420)
	frame.U = extractPlane(img.Cb, img.CStride, width/2, height/2)
	frame.V = extractPlane(img.Cr, img.CStride, width/2, height/2)

	return frame, nil
}

// extractPlane copies pixel data from a plane that may have a stride larger
// than the row width, producing a tightly packed output buffer.
// A copy is always made because the source data is owned by the decoder
// and may be reused on subsequent decode calls.
func extractPlane(data []byte, stride, width, height int) []byte {
	out := make([]byte, width*height)
	if stride == width {
		copy(out, data[:width*height])
		return out
	}
	for y := 0; y < height; y++ {
		copy(out[y*width:], data[y*stride:y*stride+width])
	}
	return out
}

// generateTimestamp creates a 90kHz timestamp for video RTP.
// Uses the injected time provider for deterministic testing.
func (p *Processor) generateTimestamp() uint32 {
	// Use current time in 90kHz units (standard for video RTP)
	return uint32(p.timeProvider.Now().UnixNano() / 1000 * 90 / 1000000)
}

// SetTimeProvider sets the time provider for deterministic testing.
func (p *Processor) SetTimeProvider(tp TimeProvider) {
	p.timeProvider = tp
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
//
// Creates a new VP8 encoder for the requested dimensions. If the encoder
// cannot be created, the previous encoder and dimensions are preserved
// and an error is returned.
func (p *Processor) SetFrameSize(width, height uint16) error {
	if width == 0 || height == 0 {
		return fmt.Errorf("invalid dimensions: %dx%d", width, height)
	}

	// Create new encoder; keep old encoder intact on failure
	newEncoder, err := NewRealVP8Encoder(width, height, p.bitRate)
	if err != nil {
		return fmt.Errorf("failed to resize encoder to %dx%d: %w", width, height, err)
	}

	p.width = width
	p.height = height
	p.encoder = newEncoder

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
