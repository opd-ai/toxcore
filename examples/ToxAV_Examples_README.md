# ToxAV Examples Collection

This directory contains comprehensive examples demonstrating ToxAV (audio/video calling) functionality in toxcore-go. These examples showcase the complete range of ToxAV capabilities from basic calling to advanced integration.

## Overview

The ToxAV examples demonstrate:

- **Complete API Coverage**: All major ToxAV functions and features
- **Real-World Usage**: Practical examples for common use cases
- **Best Practices**: Recommended patterns and implementations
- **Performance Optimization**: Efficient audio/video processing
- **Integration Patterns**: How to combine ToxAV with Tox messaging
- **Advanced Features**: Effects processing, pattern generation, analysis

## Available Examples

### 1. Basic Audio/Video Call (`toxav_basic_call/`)

**Purpose**: Comprehensive introduction to ToxAV calling

**Features**:
- Complete call lifecycle (initiate, answer, end)
- Audio generation (440Hz sine wave)
- Video generation (animated color patterns)
- Automatic call handling
- Real-time statistics
- Network bootstrap integration

**Best For**: Learning ToxAV basics, understanding call flow

**Key Concepts**:
```go
// Basic ToxAV setup
toxav, err := toxcore.NewToxAV(tox)
toxav.Call(friendNumber, audioBitRate, videoBitRate)
toxav.AudioSendFrame(friendNumber, pcm, sampleCount, channels, samplingRate)
toxav.VideoSendFrame(friendNumber, width, height, y, u, v)
```

**Output Example**:
```
🎯 ToxAV Basic Audio/Video Call Demo
====================================
✅ Tox ID: 76518406F6A9F221...
📞 ToxAV ready for audio/video calls
📊 Stats [30s]: Audio: 3000 frames, Video: 900 frames
```

### 2. Audio-Only Call (`toxav_audio_call/`)

**Purpose**: Advanced audio-only calling with effects

**Features**:
- Musical tone generation (C major scale)
- Audio effects chain (gain control, auto gain)
- Real-time audio analysis (peak, RMS)
- Mono audio optimization
- Audio-only call detection
- Effects performance monitoring

**Best For**: VoIP applications, audio processing, voice chat

**Key Concepts**:
```go
// Audio effects chain
effectsChain := audio.NewEffectChain()
gainEffect, _ := audio.NewGainEffect(1.2)
autoGainEffect := audio.NewAutoGainEffect()
effectsChain.AddEffect(gainEffect)
effectsChain.AddEffect(autoGainEffect)
```

**Output Example**:
```
🎤 ToxAV Audio-Only Call Demo
=============================
🎼 Playing note: C (261.63 Hz)
🔊 Audio frame: Peak: 15420, RMS: 8934
📊 Audio Stats: Sent: 1000, Effects: 1000
```

### 3. Integration Demo (`toxav_integration/`)

**Purpose**: Complete Tox client with messaging and calling

**Features**:
- Full Tox client functionality
- Interactive command-line interface
- Friend management and messaging
- Profile persistence and loading
- Message command processing
- Call coordination with messaging
- Statistics and monitoring

**Best For**: Building complete Tox applications, client development

**Key Concepts**:
```go
// Integrated message and call handling
tox.OnFriendMessage(func(friendNumber uint32, message string) {
    if message == "call" {
        toxav.Call(friendNumber, audioBitRate, 0) // Audio-only
    }
})
```

**Output Example**:
```
🎯 ToxAV Integration Demo
========================
> friends
👥 Friends (2):
  [0] Alice Cooper (last seen: 14:32:15)
> call 0
📞 Calling Alice Cooper (0) - Audio: true, Video: false
```

### 4. Video Call Demo (`toxav_video_call/`)

**Purpose**: Advanced video calling with pattern generation

**Features**:
- Multiple video patterns (5 different types)
- Real-time video generation (30 FPS)
- Mathematical pattern algorithms
- Video frame analysis
- Pattern cycling automation
- Performance measurement
- YUV420 format handling

**Best For**: Video applications, pattern generation, video processing

**Key Concepts**:
```go
// Video pattern generation
patterns := []VideoPattern{
    {Name: "Color Bars", Generator: generateColorBars},
    {Name: "Plasma Effect", Generator: generatePlasmaEffect},
    // ...
}
```

**Output Example**:
```
📹 ToxAV Video Call Demo
========================
🎨 Current pattern: Color Bars - Classic TV color bar pattern
📊 Video Stats: 600 frames (avg: 118μs), Active: 1
🎨 Switched to pattern: Plasma Effect
```

### 5. Effects Processing Demo (`toxav_effects_processing/`)

**Purpose**: Advanced audio/video effects processing with real-time parameter adjustment

**Features**:
- Advanced audio effects (noise suppression, gain control, AGC)
- Video effects (color temperature adjustment)
- Real-time effects parameter modification
- Interactive console interface
- Performance monitoring and benchmarking
- Effect chain demonstration

**Best For**: Learning effects processing, VoIP applications, content creation

**Key Concepts**:
```go
// Audio effects chain
gainEffect, _ := audio.NewGainEffect(1.0)
noiseEffect, _ := audio.NewNoiseSuppressionEffect(0.5, 480)
agcEffect := audio.NewAutoGainEffect()

chain := audio.NewEffectChain()
chain.AddEffect(gainEffect)
chain.AddEffect(noiseEffect)
chain.AddEffect(agcEffect)

// Video effects
tempEffect := video.NewColorTemperatureEffect(6500) // Daylight
```

**Output Example**:
```
🎯 ToxAV Effects Processing Demo
===============================
🎧 Audio Effects: Gain(1.0), NoiseSuppress(0.5), AGC(0.7)
🎨 Video Effects: ColorTemp(6500K)
📊 Performance: Audio(156μs), Video(89μs)

> audio gain 1.5
🔊 Audio gain set to 1.5

> video temp 3000
🌅 Color temperature set to 3000K (warm)
```

## Quick Start Guide

### 1. Choose Your Example

- **New to ToxAV?** Start with `toxav_basic_call/`
- **Audio focus?** Use `toxav_audio_call/`
- **Building an app?** See `toxav_integration/`
- **Video processing?** Try `toxav_video_call/`

### 2. Run the Example

```bash
cd examples/toxav_basic_call
go run main.go
```

### 3. Test with Another Client

To test calling functionality:

1. Run the example to get the Tox ID
2. Use another Tox client to add the friend
3. Initiate a call to see the demo in action

## Common Usage Patterns

### Basic ToxAV Setup

```go
// Standard initialization pattern
options := toxcore.NewOptions()
options.UDPEnabled = true

tox, err := toxcore.New(options)
if err != nil {
    log.Fatal(err)
}

toxav, err := toxcore.NewToxAV(tox)
if err != nil {
    log.Fatal(err)
}

// Set up callbacks
toxav.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
    // Handle incoming calls
})
```

### Audio Frame Processing

```go
// Generate and send audio frames
func sendAudioFrame(toxav *toxcore.ToxAV, friendNumber uint32) {
    // Generate PCM audio data
    pcm := generateAudioFrame() // []int16
    
    err := toxav.AudioSendFrame(friendNumber, pcm, 
        audioFrameSize, audioChannels, audioSampleRate)
    if err != nil {
        log.Printf("Audio send error: %v", err)
    }
}
```

### Video Frame Processing

```go
// Generate and send video frames
func sendVideoFrame(toxav *toxcore.ToxAV, friendNumber uint32) {
    // Generate YUV420 video data
    y, u, v := generateVideoFrame() // []byte for each plane
    
    err := toxav.VideoSendFrame(friendNumber, 
        videoWidth, videoHeight, y, u, v)
    if err != nil {
        log.Printf("Video send error: %v", err)
    }
}
```

### Call Management

```go
// Complete call handling pattern
func handleCall(toxav *toxcore.ToxAV) {
    // Set up call callback
    toxav.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
        // Determine bitrates
        audioBR := uint32(0)
        videoBR := uint32(0)
        if audioEnabled {
            audioBR = 64000 // 64 kbps
        }
        if videoEnabled {
            videoBR = 500000 // 500 kbps
        }
        
        // Answer the call
        err := toxav.Answer(friendNumber, audioBR, videoBR)
        if err != nil {
            log.Printf("Failed to answer call: %v", err)
        }
    })
    
    // Set up state callback
    toxav.CallbackCallState(func(friendNumber uint32, state av.CallState) {
        if state == av.CallStateFinished {
            log.Printf("Call ended with friend %d", friendNumber)
        }
    })
}
```

## Configuration Reference

### Audio Configuration

```go
const (
    audioSampleRate = 48000 // 48kHz for Opus compatibility
    audioChannels   = 2     // Stereo (or 1 for mono)
    audioFrameSize  = 480   // 10ms frame size
    audioBitRate    = 64000 // 64 kbps (adjust for quality)
)
```

### Video Configuration

```go
const (
    videoWidth     = 640    // VGA width
    videoHeight    = 480    // VGA height  
    videoFrameRate = 30     // 30 FPS
    videoBitRate   = 500000 // 500 kbps (adjust for quality)
)
```

### Performance Settings

```go
const (
    toxIterationInterval = 50 * time.Millisecond // Tox iteration
    audioFrameInterval   = 10 * time.Millisecond // Audio frame timing
    videoFrameInterval   = 33 * time.Millisecond // Video frame timing (30 FPS)
)
```

## Testing and Development

### Local Testing

For testing without network:
1. Run two instances of the same example
2. Copy Tox IDs and add as friends
3. Initiate calls between instances

### Network Testing

For real network testing:
1. Run examples on different machines
2. Ensure UDP connectivity
3. Use public bootstrap nodes
4. Monitor network performance

### Performance Testing

To measure performance:
1. Use the benchmark tools in each example
2. Monitor CPU and memory usage
3. Measure frame processing times
4. Test under different network conditions

## Troubleshooting

### Common Issues

**ToxAV Creation Fails**:
```go
// Ensure Tox transport is initialized
if tox.udpTransport == nil {
    return errors.New("tox transport not initialized")
}
```

**Frame Send Errors**:
```go
// Check for active calls before sending
if err.Error() == "no call found for friend" {
    // This is expected when no calls are active
    return
}
```

**Audio/Video Not Working**:
- Verify frame format (PCM for audio, YUV420 for video)
- Check frame size calculations
- Ensure proper bitrate settings
- Verify callback registration

### Debug Output

Enable debug logging:
```go
import "github.com/sirupsen/logrus"

logrus.SetLevel(logrus.DebugLevel)
```

### Performance Issues

If experiencing performance problems:
- Reduce frame rates or bitrates
- Optimize frame generation algorithms
- Use simpler video patterns
- Monitor memory allocation patterns

## Advanced Topics

### Custom Audio Effects

Implement custom audio effects:
```go
type CustomEffect struct{}

func (e *CustomEffect) Process(samples []int16) ([]int16, error) {
    // Custom audio processing
    return processedSamples, nil
}

func (e *CustomEffect) GetName() string { return "Custom Effect" }
func (e *CustomEffect) Close() error { return nil }
```

### Custom Video Patterns

Create custom video patterns:
```go
func customVideoPattern(demo *VideoCallDemo) ([]byte, []byte, []byte) {
    // Generate Y, U, V planes for YUV420 format
    y := make([]byte, videoWidth*videoHeight)
    u := make([]byte, (videoWidth/2)*(videoHeight/2))
    v := make([]byte, (videoWidth/2)*(videoHeight/2))
    
    // Fill with custom pattern
    // ...
    
    return y, u, v
}
```

### Integration with Real Media

Replace synthetic generation with real media:
- Use audio capture libraries for microphone input
- Integrate camera libraries for video input
- Implement format conversion as needed
- Handle device enumeration and selection

## Dependencies

All examples require:
- `github.com/opd-ai/toxcore` - Core Tox functionality
- `github.com/opd-ai/toxcore/av` - ToxAV implementation

Some examples additionally use:
- `github.com/opd-ai/toxcore/av/audio` - Audio processing
- `github.com/opd-ai/toxcore/av/video` - Video processing

## Contributing

When adding new ToxAV examples:

1. **Follow Naming Convention**: `toxav_<feature>/`
2. **Include Complete README**: Document features and usage
3. **Add Error Handling**: Comprehensive error handling
4. **Performance Considerations**: Optimize for real-time use
5. **Documentation**: Clear code comments and examples
6. **Testing**: Verify functionality and performance

### Example Template

```go
// ToxAV <Feature> Example
//
// This example demonstrates <feature> using ToxAV with
// <specific capabilities and use cases>.

package main

import (
    "github.com/opd-ai/toxcore"
    "github.com/opd-ai/toxcore/av"
)

func main() {
    fmt.Println("🎯 ToxAV <Feature> Demo")
    
    // Implementation with proper error handling
    // Clear documentation and comments
    // Performance considerations
}
```

## Future Examples

Planned additional examples:
- **File Transfer Integration**: Combining A/V calls with file transfer
- **Group Calling**: Multi-party audio/video calls
- **Recording**: Call recording and playback
- **Streaming**: One-to-many video streaming
- **Effects Processing**: Advanced audio/video effects
- **Quality Adaptation**: Dynamic quality adjustment based on network

---

These examples provide a comprehensive foundation for understanding and implementing ToxAV functionality in your own applications. Each example builds on the previous ones, demonstrating increasingly sophisticated features and integration patterns.
