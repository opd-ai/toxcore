# ToxAV Video Call Demo

This example demonstrates advanced video calling capabilities using ToxAV with multiple video patterns, real-time generation, and comprehensive video analysis.

## Overview

The video call demo showcases:

- **Advanced Video Generation**: Multiple animated video patterns
- **Real-Time Processing**: 30 FPS video generation and transmission
- **Pattern Cycling**: Automatic switching between different video patterns
- **Video Analysis**: Frame analysis and quality metrics
- **High-Quality Video**: 640×480 @ 30 FPS with 500 kbps bitrate
- **Performance Monitoring**: Real-time processing time tracking

## Features

### Video Patterns
- **Color Bars**: Classic TV color bar test pattern
- **Moving Gradient**: Animated color gradients with wave effects
- **Checkerboard**: Animated checkerboard with varying size
- **Plasma Effect**: Retro plasma animation with mathematical formulas
- **Test Pattern**: Technical test pattern with frame information

### Video Processing
- YUV420 color format for optimal compression
- 640×480 VGA resolution for clear video
- 30 FPS for smooth motion
- Real-time pattern generation and animation
- Frame-by-frame processing time measurement

### Audio Integration
- Minimal audio processing (focus on video)
- 48kHz mono audio with simple tone generation
- Lower bitrate audio to prioritize video bandwidth
- Synchronized audio/video transmission

### Analysis Features
- Y, U, V color plane analysis
- Average brightness and color level monitoring
- Frame reception statistics
- Processing time measurement
- Performance optimization tracking

## Usage

### Running the Demo

```bash
cd examples/toxav_video_call
go run main.go
```

### Expected Output

```
📹 ToxAV Video Call Demo
========================
📹 ToxAV Video Call Demo - Initializing...
✅ Tox ID: 76518406F6A9F2217E8DC487CC783C25CC16A15EB36FF32E335364EC37B13349ABC123
📹 Video ToxAV ready (640x480 @ 30 fps, YUV420)
🎬 Starting video call demo for 1m30s
📋 Video demo features:
   • Multiple video patterns (color bars, gradients, checkerboard, plasma, test)
   • Real-time video generation and processing
   • Video frame analysis and statistics
   • High-quality video calling (500 kbps)
   • Animated video effects and patterns
🎨 Current pattern: Color Bars - Classic TV color bar pattern
🌐 Connected to Tox network
▶️  Video demo running - Press Ctrl+C to stop
🎨 Switched to pattern: Moving Gradient - Animated color gradient
📊 Video Stats [10s]: Video: 300 frames (avg: 125μs), Audio: 1000, Received: 0, Active: 0
🎨 Switched to pattern: Checkerboard - Animated checkerboard pattern
📊 Video Stats [20s]: Video: 600 frames (avg: 118μs), Audio: 2000, Received: 0, Active: 0
```

### Receiving a Video Call

When another ToxAV client initiates a video call:

```
📹 Incoming call from friend 0 (audio: true, video: true)
✅ Video call answered with friend 0
📡 Video call state changed for friend 0: State_3
📹 Video frame from friend 0: 640x480, Y:128 U:100 V:156 (avg levels)
🔊 Audio frame from friend 0: 480 samples @ 48000Hz
📊 Video Stats [30s]: Video: 900 frames (avg: 112μs), Audio: 3000, Received: 45, Active: 1
```

## Configuration

### Video Settings

```go
const (
    videoWidth     = 640
    videoHeight    = 480
    videoFrameRate = 30
    videoBitRate   = 500000 // 500 kbps
    videoFormat    = "YUV420"
)
```

### Audio Settings

```go
const (
    audioSampleRate = 48000
    audioChannels   = 1
    audioFrameSize  = 480
    audioBitRate    = 32000 // Lower bitrate, focus on video
)
```

### Demo Settings

```go
const (
    demoDuration = 90 * time.Second
)
```

## Video Patterns

### 1. Color Bars
Classic television color bar test pattern:
- Standard broadcast colors (white, yellow, cyan, green, magenta, red, blue, black)
- Precise color reproduction for testing
- Static pattern for baseline comparison

### 2. Moving Gradient
Animated color gradient with wave effects:
- Mathematical wave functions for smooth animation
- Color phase rotation over time
- Gradient transitions across the frame

### 3. Checkerboard
Animated checkerboard pattern:
- Variable checker size based on animation phase
- Moving offset for dynamic animation
- High contrast pattern for compression testing

### 4. Plasma Effect
Retro plasma animation:
- Mathematical plasma formula with sine waves
- Complex animated patterns
- Retro aesthetic with smooth color transitions

### 5. Test Pattern
Technical test pattern:
- Frame borders and center crosshair
- Frame counter integration
- Neutral color balance for calibration

## Key Components

### VideoCallDemo

The main demonstration class:

- **Pattern Management**: Cycling through different video patterns
- **Video Generation**: Real-time frame generation at 30 FPS
- **Performance Tracking**: Processing time measurement
- **Statistics**: Comprehensive metrics collection

### Video Pattern System

```go
type VideoPattern struct {
    Name        string
    Description string
    Generator   func(demo *VideoCallDemo) ([]byte, []byte, []byte)
}
```

Each pattern implements:
- YUV420 format generation
- Real-time animation
- Mathematical pattern formulas
- Color space management

### Video Analysis

```go
// Frame analysis in callback
yAvg := uint64(0)
for _, pixel := range y {
    yAvg += uint64(pixel)
}
yAvg /= uint64(len(y))
```

Analysis includes:
- Y plane (luminance) average
- U plane (blue chrominance) average  
- V plane (red chrominance) average
- Frame quality assessment

## Performance Metrics

### Video Generation
- **Frame Rate**: 30 FPS (33.3ms per frame)
- **Processing Time**: ~125μs average per frame
- **CPU Usage**: <1% on modern hardware
- **Memory**: Efficient pre-allocated buffers

### Network Performance
- **Video Bitrate**: 500 kbps for high quality
- **Audio Bitrate**: 32 kbps (minimal overhead)
- **Total Bandwidth**: ~532 kbps combined
- **Latency**: <100ms for local network

### Pattern Complexity
- **Color Bars**: Simplest pattern, minimal CPU
- **Gradients**: Mathematical calculations, moderate CPU
- **Checkerboard**: Variable complexity, efficient
- **Plasma**: Most complex, highest CPU usage
- **Test Pattern**: Minimal CPU, static elements

## Integration Points

### With ToxAV
- High-level video frame transmission API
- Bitrate management and negotiation
- Call state management
- Error handling and recovery

### With Video Processor
- YUV420 format handling
- Frame validation and processing
- Performance optimization
- Memory management

## Error Handling

The demo includes comprehensive error handling:

### Video Processing Errors
- Frame generation failures
- Invalid format detection
- Memory allocation errors
- Performance degradation handling

### Network Errors
- Video transmission failures
- Bitrate adaptation
- Connection quality issues
- Call termination handling

### Recovery Mechanisms
- Graceful pattern switching
- Performance optimization
- Memory cleanup
- Resource management

## Extending the Demo

### Adding Real Camera Input

Replace pattern generation with camera input:

```go
// Replace pattern generators with camera capture
// Use video capture library for real video input
// Implement format conversion from camera
```

### Custom Video Effects

Add video effects processing:

```go
// Implement video effects like color temperature
// Add brightness/contrast adjustments
// Implement video filters and transformations
```

### Multiple Resolution Support

Add resolution switching:

```go
// Support for different video resolutions
// Dynamic resolution adaptation
// Quality-based resolution selection
```

### Advanced Analysis

Enhance frame analysis:

```go
// Add histogram analysis
// Implement motion detection
// Add quality metrics (PSNR, SSIM)
// Frame difference analysis
```

## Mathematical Formulas

### Plasma Effect
```go
plasma := math.Sin(x + time) +
          math.Sin(y + time) +
          math.Sin((x + y + time)/2) +
          math.Sin(math.Sqrt(x*x + y*y) + time)
```

### Moving Gradient
```go
wave := math.Sin(x*4*math.Pi + phase*0.1) * 
        math.Cos(y*2*math.Pi + phase*0.05)
```

### Color Phase Rotation
```go
u = 128 + 64*sin(colorPhase)
v = 128 + 64*cos(colorPhase)
```

## Dependencies

- `github.com/opd-ai/toxcore` - Core Tox functionality
- `github.com/opd-ai/toxcore/av` - ToxAV types and constants
- `github.com/opd-ai/toxcore/av/video` - Video processing

## Related Examples

- `toxav_basic_call/` - Basic audio/video calling
- `toxav_audio_call/` - Audio-only calling
- `toxav_integration/` - Complete Tox+ToxAV integration
- `color_temperature_demo/` - Video effects only
