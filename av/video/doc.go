// Package video provides video processing capabilities for ToxAV.
//
// This package implements the complete video processing pipeline for
// audio/video calls in the Tox protocol, including VP8 codec support,
// RTP packetization, frame scaling, and visual effects processing.
//
// # Architecture Overview
//
// The video processing pipeline handles both encoding and decoding:
//
//	Encoding: YUV420 Input → Scaling → Effects → VP8 Encoding → RTP Packetization
//	Decoding: YUV420 Output ← Scaling ← Effects ← VP8 Decoding ← RTP Depacketization
//
// Each stage is implemented as a composable component that can be used
// independently or as part of the full pipeline.
//
// # Video Frames
//
// Video data is represented using the YUV420 format, which is efficient
// for video compression and widely supported by codecs:
//
//	frame := &video.VideoFrame{
//	    Width:  640,
//	    Height: 480,
//	    Y:      yPlane,  // Luminance plane (full resolution)
//	    U:      uPlane,  // Chrominance U (half resolution)
//	    V:      vPlane,  // Chrominance V (half resolution)
//	}
//
// # VP8 Codec
//
// The VP8Codec provides video encoding and decoding using the VP8 format,
// which is optimized for real-time video streaming:
//
//	codec := video.NewVP8Codec()
//	defer codec.Close()
//
//	// Encode a frame
//	encoded, err := codec.EncodeFrame(frame)
//	if err != nil {
//	    return fmt.Errorf("encoding failed: %w", err)
//	}
//
//	// Decode a frame
//	decoded, err := codec.DecodeFrame(encoded)
//	if err != nil {
//	    return fmt.Errorf("decoding failed: %w", err)
//	}
//
// # RTP Packetization
//
// RTP packetization breaks encoded video frames into network-friendly
// packets according to RFC 7741 for VP8:
//
//	packetizer := video.NewRTPPacketizer(ssrc)
//
//	// Packetize an encoded frame
//	packets := packetizer.Packetize(encodedFrame, timestamp)
//
//	// Send packets over network
//	for _, packet := range packets {
//	    transport.Send(packet.Serialize())
//	}
//
// The RTPDepacketizer reassembles packets into complete frames:
//
//	depacketizer := video.NewRTPDepacketizer()
//
//	// Process incoming packet
//	frame, complete := depacketizer.ProcessPacket(packet)
//	if complete {
//	    // Frame is ready for decoding
//	}
//
// # Video Scaling
//
// The Scaler resizes video frames using bilinear interpolation:
//
//	scaler := video.NewScaler()
//
//	// Scale to target resolution
//	scaled, err := scaler.Scale(frame, 1280, 720)
//	if err != nil {
//	    return fmt.Errorf("scaling failed: %w", err)
//	}
//
// # Visual Effects
//
// Effects can be applied to video frames individually or in chains:
//
//	// Apply individual effects
//	brightness := video.NewBrightnessEffect(20)
//	frame, err := brightness.Apply(frame)
//
//	// Use effect chain for multiple effects
//	chain := video.NewEffectChain()
//	chain.AddEffect(video.NewBrightnessEffect(10))
//	chain.AddEffect(video.NewContrastEffect(1.2))
//	chain.AddEffect(video.NewGrayscaleEffect())
//
//	processed, err := chain.Apply(frame)
//
// Available effects include:
//   - BrightnessEffect: Adjust image brightness
//   - ContrastEffect: Modify image contrast
//   - GrayscaleEffect: Convert to grayscale
//   - BlurEffect: Apply Gaussian blur
//   - SharpenEffect: Sharpen image details
//   - ColorTemperatureEffect: Adjust warm/cool tones
//
// # Video Processor
//
// The Processor combines encoding and effects into a complete pipeline:
//
//	processor := video.NewProcessor()
//
//	// Configure with effects
//	processor.SetEffectChain(effectChain)
//
//	// Process and encode frame
//	encoded, err := processor.ProcessFrame(frame)
//
// # Deterministic Testing
//
// For reproducible tests, inject a custom TimeProvider:
//
//	// Create with custom time provider
//	depacketizer := video.NewRTPDepacketizerWithTimeProvider(mockTime)
//
//	// Or set after creation
//	processor := video.NewProcessor()
//	processor.SetTimeProvider(mockTime)
//
// This allows deterministic control over timestamp generation and
// timeout calculations in tests.
//
// # Thread Safety
//
// Video processing types in this package are NOT thread-safe by default.
// External synchronization is required when sharing instances between
// goroutines. The recommended pattern is to process frames in a single
// goroutine and use channels for inter-goroutine communication:
//
//	frames := make(chan *video.VideoFrame, 10)
//	go func() {
//	    processor := video.NewProcessor()
//	    for frame := range frames {
//	        encoded, err := processor.ProcessFrame(frame)
//	        // Handle result
//	    }
//	}()
//
// # ToxAV Integration
//
// This package integrates with the parent av package for ToxAV calls:
//
//	// In av/types.go, video components are used for call sessions:
//	call.videoProcessor = video.NewProcessor()
//	call.rtpPacketizer = video.NewRTPPacketizer(ssrc)
//
// Video frames are transmitted via the rtp package using the
// transport.PacketAVVideoFrame packet type.
//
// # Known Limitations
//
//   - VP8 encoding uses a simplified passthrough encoder; full VP8
//     encoding requires external codec integration
//   - Jitter buffer in depacketizer uses simple map iteration instead
//     of timestamp-ordered retrieval
//   - No hardware acceleration support; all processing is CPU-based
package video
