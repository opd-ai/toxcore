// Package audio provides comprehensive audio processing capabilities for ToxAV.
//
// This package implements the complete audio pipeline for Tox audio/video calls,
// including Opus codec integration, audio effects processing, sample rate conversion,
// and real-time audio manipulation.
//
// # Architecture Overview
//
// The audio processing pipeline follows this flow:
//
//	Capture:  PCM Input → Resampling → Effects → Opus Encoding → RTP Packetization
//	Playback: PCM Output ← Resampling ← Effects ← Opus Decoding ← RTP Depacketization
//
// # Core Components
//
// The package provides these main components:
//
// ## OpusCodec
//
// The primary codec wrapper providing Opus-specific audio processing:
//
//	codec := audio.NewOpusCodec()
//	encoded, err := codec.EncodeFrame(pcmSamples, 48000)
//	decoded, err := codec.DecodeFrame(opusData, 48000)
//
// ## Processor
//
// The main audio processing pipeline combining encoding, decoding, resampling,
// and effects processing:
//
//	processor := audio.NewProcessor()
//	defer processor.Close()
//	
//	// Configure effects
//	processor.AddEffect(audio.NewGainEffect(1.5))
//	
//	// Process audio
//	output, err := processor.ProcessAudio(input, 48000)
//
// ## Resampler
//
// Sample rate conversion with linear interpolation:
//
//	config := audio.ResamplerConfig{
//	    InputRate:  44100,
//	    OutputRate: 48000,
//	    Channels:   1,
//	    Quality:    4,
//	}
//	resampler, err := audio.NewResampler(config)
//	resampled, err := resampler.Resample(samples)
//
// ## Effects
//
// The package provides a flexible effects framework with built-in effects:
//
//   - GainEffect: Volume adjustment with clipping protection
//   - AutoGainEffect: Automatic gain control (AGC) for consistent volume
//   - NoiseSuppressionEffect: Spectral subtraction-based noise reduction
//   - EffectChain: Sequential effect processing pipeline
//
// Example of building an effects chain:
//
//	chain := audio.NewEffectChain()
//	chain.AddEffect(audio.NewNoiseSuppressionEffect(0.5))
//	chain.AddEffect(audio.NewAutoGainEffect(0.7, -18.0))
//	chain.AddEffect(audio.NewGainEffect(1.2))
//	
//	processed, err := chain.Process(samples)
//
// # Thread Safety
//
// All components in this package are designed for concurrent use:
//
//   - Processor instances are thread-safe
//   - Effect chains can be shared across goroutines
//   - Resampler maintains internal state and should be used per-stream
//
// # Dependencies
//
// The package uses minimal external dependencies:
//
//   - github.com/pion/opus: Pure Go Opus decoder (no CGO)
//   - github.com/sirupsen/logrus: Structured logging
//
// # Performance Considerations
//
//   - Linear interpolation resampling: O(n) complexity, suitable for real-time
//   - Noise suppression FFT: O(n log n), configurable frame sizes (64-4096)
//   - Pre-allocated buffers minimize garbage collection pressure
//   - Same-rate resampling optimization bypasses processing
//
// # Integration
//
// The audio package integrates with the ToxAV ecosystem:
//
//   - Used by av/types.go for call audio processing
//   - Supports the ToxAV callback system for audio frame handling
//   - Compatible with RTP packetization for network transmission
package audio
