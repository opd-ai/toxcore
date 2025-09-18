# ToxAV Audio-Only Call Demo

This example demonstrates advanced audio-only calling capabilities using ToxAV with effects processing, musical tone generation, and real-time audio analysis.

## Overview

The audio call demo showcases:

- **Audio-Only Calling**: Optimized for voice communication
- **Musical Tone Generation**: C major scale melody generation
- **Audio Effects Chain**: Gain control and automatic gain control
- **Real-Time Audio Analysis**: Peak and RMS level monitoring
- **Effects Processing**: Comprehensive audio effects pipeline
- **Mono Audio Optimization**: Optimized for voice calls

## Features

### Audio Processing
- 48kHz mono audio for optimal voice quality
- 10ms frame size for low-latency communication
- Musical scale generation (C, D, E, F, G, A, B)
- Real-time audio effects processing

### Audio Effects Chain
- **Gain Effect**: Linear volume control with clipping protection
- **Auto Gain Control**: Automatic level adjustment with peak following
- **Effect Chaining**: Sequential effect processing pipeline
- **Real-Time Processing**: Sub-microsecond effect processing

### Audio Analysis
- Peak level detection for volume monitoring
- RMS level calculation for average volume
- Real-time audio quality assessment
- Frame statistics and processing metrics

### Call Management
- Audio-only call detection and handling
- Automatic call answering for audio calls
- Call state monitoring and management
- Graceful call termination

## Usage

### Running the Demo

```bash
cd examples/toxav_audio_call
go run main.go
```

### Expected Output

```
🎤 ToxAV Audio-Only Call Demo
=============================
🎵 ToxAV Audio-Only Call Demo - Initializing...
✅ Tox ID: 76518406F6A9F2217E8DC487CC783C25CC16A15EB36FF32E335364EC37B13349ABC123
🎤 Audio-only ToxAV ready (Mono, 48kHz, 64kbps)
🎬 Starting audio call demo for 1m0s
📋 Audio demo features:
   • Musical tone generation (C major scale)
   • Audio effects chain (gain control + auto gain)
   • Real-time audio analysis
   • Mono audio optimized for voice
   • Audio-only call handling
🌐 Connected to Tox network
▶️  Audio demo running - Press Ctrl+C to stop
🎼 Playing note: C (261.63 Hz)
🎼 Playing note: D (293.66 Hz)
📊 Audio Stats [5s]: Sent: 500, Received: 0, Active calls: 0, Effects: 500
🎼 Playing note: E (329.63 Hz)
```

### Receiving an Audio Call

When another ToxAV client initiates an audio call:

```
🎵 Incoming call from friend 0 (audio: true, video: false)
✅ Audio call answered with friend 0
📡 Audio call state changed for friend 0: State_3
🔊 Audio frame from friend 0: 480 samples @ 48000Hz, Peak: 15420, RMS: 8934
📊 Audio Stats [10s]: Sent: 1000, Received: 150, Active calls: 1, Effects: 1000
```

## Configuration

### Audio Settings

```go
const (
    audioSampleRate = 48000 // 48kHz for Opus compatibility
    audioChannels   = 1     // Mono for voice calls
    audioFrameSize  = 480   // 10ms frame size
    audioBitRate    = 64000 // 64 kbps - good quality for voice
)
```

### Musical Scale

```go
toneFreqs: []float64{
    261.63, // C4
    293.66, // D4
    329.63, // E4
    349.23, // F4
    392.00, // G4
    440.00, // A4
    493.88, // B4
}
```

## Key Components

### AudioCallDemo

The main demonstration class:

- **Audio Processing**: Musical tone generation with effects
- **Effects Chain**: Gain control and automatic gain control
- **Statistics**: Real-time audio performance monitoring
- **Call Management**: Audio-only call handling

### Musical Tone Generation

```go
func (d *AudioCallDemo) generateMelodyFrame() []int16 {
    // Generates musical tones cycling through C major scale
    // Each note plays for 2 seconds with tremolo effect
    // Returns mono PCM samples
}
```

### Audio Effects Processing

```go
func (d *AudioCallDemo) processAudioWithEffects(frame []int16) []int16 {
    // Applies effects chain to audio frame
    // Includes gain control and auto gain control
    // Returns processed audio with effects applied
}
```

### Real-Time Audio Analysis

```go
// Peak and RMS analysis in callback
peak := int16(0)
rms := int64(0)
for _, sample := range pcm {
    if sample > peak {
        peak = sample
    }
    rms += int64(sample) * int64(sample)
}
```

## Audio Effects

### Gain Effect

- **Purpose**: Linear volume control
- **Range**: 0.0 (silence) to 4.0 (400% volume)
- **Features**: Clipping protection, real-time adjustment
- **Performance**: ~356ns per 10ms frame

### Auto Gain Control (AGC)

- **Purpose**: Automatic level adjustment
- **Algorithm**: Peak-following with attack/release
- **Target**: 30% of maximum level
- **Performance**: ~903ns per 10ms frame

### Effect Chain

- **Processing**: Sequential effect application
- **Performance**: ~1.1μs for complete chain
- **Memory**: Zero allocations during processing
- **Thread Safety**: Safe for concurrent use

## Integration

### With ToxAV

The demo uses core ToxAV functionality:

- Audio-only call detection and handling
- Frame transmission and reception
- Bitrate management and negotiation
- Call state monitoring

### With Audio Effects

The demo demonstrates effects integration:

- Effects chain creation and management
- Real-time audio processing
- Performance monitoring
- Error handling

## Performance Metrics

### Audio Generation
- **Frame Rate**: 100 frames/second (10ms each)
- **CPU Usage**: Minimal CPU overhead for tone generation
- **Memory**: Pre-allocated buffers, no runtime allocations

### Effects Processing
- **Gain Effect**: 356ns per frame
- **Auto Gain Control**: 903ns per frame
- **Complete Chain**: ~1.1μs per frame
- **Total Overhead**: <0.1% of frame time

### Network Performance
- **Bitrate**: 64 kbps for high-quality voice
- **Latency**: 10ms frame processing
- **Bandwidth**: Optimized for voice communication

## Error Handling

The demo includes comprehensive error handling:

- ToxAV initialization failures
- Audio effects creation errors
- Frame processing failures
- Network connection issues
- Graceful shutdown on errors

## Extending the Demo

### Adding Real Audio Input

Replace tone generation with microphone input:

```go
// Replace generateMelodyFrame() with microphone capture
// Use audio capture library for real audio input
```

### Custom Audio Effects

Add additional effects to the chain:

```go
// Create custom effects implementing AudioEffect interface
// Add to effects chain for complex processing
```

### Call Initiation

Add ability to initiate calls:

```go
// Add friend management
// Initiate calls to specific friends
// Handle call negotiation
```

### Multiple Simultaneous Calls

Extend for multiple calls:

```go
// Track multiple friend connections
// Send audio to all active calls
// Manage per-call statistics
```

## Dependencies

- `github.com/opd-ai/toxcore` - Core Tox functionality
- `github.com/opd-ai/toxcore/av` - ToxAV types and constants
- `github.com/opd-ai/toxcore/av/audio` - Audio processing and effects

## Related Examples

- `toxav_basic_call/` - Complete audio/video calling
- `audio_effects_demo/` - Audio effects only
- `toxav_video_call/` - Video-focused calling
