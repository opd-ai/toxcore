# ToxAV Basic Audio/Video Call Demo

This example demonstrates the complete workflow for using ToxAV to conduct audio/video calls using the toxcore-go library.

## Overview

The basic call demo showcases:

- **ToxAV Initialization**: Setting up ToxAV with an existing Tox instance
- **Call Management**: Handling incoming calls and call state changes
- **Audio Generation**: Creating synthetic audio frames (440Hz sine wave)
- **Video Generation**: Creating animated video frames with color patterns
- **Frame Transmission**: Sending audio and video frames during calls
- **Statistics Tracking**: Real-time monitoring of call performance
- **Network Integration**: Bootstrap connection to the Tox network

## Features

### Audio Processing
- 48kHz stereo audio generation
- 10ms frame size (480 samples) for low latency
- Sine wave generation at 440Hz (A4 musical note)
- Configurable volume and frequency

### Video Processing  
- 640×480 VGA resolution video
- 30 FPS frame rate
- YUV420 color format
- Animated diagonal stripe pattern
- Color cycling effects

### Call Management
- Automatic call answering
- Call state monitoring
- Audio/video bitrate management
- Graceful call termination

### Network Features
- Bootstrap to Tox network
- UDP transport integration
- Real-time frame transmission
- Network error handling

## Usage

### Running the Demo

```bash
cd examples/toxav_basic_call
go run main.go
```

### Expected Output

```
🎯 ToxAV Basic Audio/Video Call Demo
====================================
🚀 ToxAV Basic Call Demo - Initializing...
✅ Tox ID: 76518406F6A9F2217E8DC487CC783C25CC16A15EB36FF32E335364EC37B13349ABC123
📞 ToxAV ready for audio/video calls
🎬 Starting ToxAV demo for 30s
📋 Demo features:
   • Audio frame generation (440Hz sine wave)
   • Video frame generation (animated color pattern)
   • Automatic call answering
   • Real-time statistics
   • Bootstrap connection
🌐 Connected to Tox network
▶️  Demo running - Press Ctrl+C to stop
📊 Stats [5s]: Audio: 500 frames, Video: 150 frames, Calls: 0↗ 0↘ 0✓
📊 Stats [10s]: Audio: 1000 frames, Video: 300 frames, Calls: 0↗ 0↘ 0✓
⏰ Demo duration completed (30s)
📈 Final statistics:
   Audio frames sent: 3000
   Video frames sent: 900
   Calls initiated: 0
   Calls received: 0
   Calls completed: 0
✅ Cleanup completed
👋 Demo completed successfully
```

### Receiving a Call

If another ToxAV client calls this demo instance:

```
📞 Incoming call from friend 0 (audio: true, video: true)
✅ Call answered with friend 0
📡 Call state changed for friend 0: State_3
🔊 Received audio frame from friend 0: 480 samples, 2 channels, 48000 Hz
📹 Received video frame from friend 0: 640x480 (Y:307200 U:76800 V:76800 bytes)
```

## Configuration

### Audio Settings

```go
const (
    audioSampleRate = 48000 // 48kHz for Opus compatibility
    audioChannels   = 2     // Stereo audio
    audioFrameSize  = 480   // 10ms frame size
    audioBitRate    = 64000 // 64 kbps
)
```

### Video Settings

```go
const (
    videoWidth     = 640
    videoHeight    = 480
    videoFrameRate = 30
    videoBitRate   = 500000 // 500 kbps
)
```

### Demo Settings

```go
const (
    demoDuration = 30 * time.Second
)
```

## Key Components

### CallDemonstrator

The main class that manages the demonstration:

- **ToxAV Integration**: Creates and manages ToxAV instance
- **Media Generation**: Generates synthetic audio and video frames
- **Statistics**: Tracks performance metrics
- **Event Handling**: Manages callbacks for calls and state changes

### Audio Generation

```go
func (d *CallDemonstrator) generateAudioFrame() []int16 {
    // Generates 10ms of stereo audio (sine wave)
    // Returns PCM samples for both left and right channels
}
```

### Video Generation

```go
func (d *CallDemonstrator) generateVideoFrame() ([]byte, []byte, []byte) {
    // Generates YUV420 frame with animated pattern
    // Returns Y, U, V planes separately
}
```

## Integration Points

### With Existing Tox

The demo uses standard Tox functionality:

- Tox instance creation and configuration
- Profile management (name, status message)
- Network bootstrap connection
- Friend management (when calls are received)

### With ToxAV

The demo demonstrates all key ToxAV features:

- Instance creation from existing Tox
- Callback registration for events
- Audio and video frame transmission
- Call state management
- Bitrate control

## Error Handling

The demo includes comprehensive error handling:

- ToxAV initialization failures
- Network connection issues
- Frame transmission errors
- Graceful shutdown on signals

## Performance

The demo generates:

- **Audio**: 100 frames/second (10ms each)
- **Video**: 30 frames/second (33ms each)
- **Data Rate**: ~500 kbps total bandwidth

## Extending the Demo

### Adding Real Audio/Video

Replace the synthetic generation with real media:

```go
// Replace generateAudioFrame() with microphone input
// Replace generateVideoFrame() with camera input
```

### Multi-Friend Calls

Extend to handle multiple simultaneous calls:

```go
// Track multiple friend numbers
// Send frames to all active calls
// Manage per-call statistics
```

### Call Initiation

Add ability to initiate calls:

```go
// Add friend by Tox ID
// Initiate call to specific friend
// Handle call rejection/timeout
```

## Dependencies

- `github.com/opd-ai/toxcore` - Core Tox functionality
- `github.com/opd-ai/toxcore/av` - ToxAV types and constants

## Related Examples

- `audio_effects_demo/` - Audio effects processing
- `color_temperature_demo/` - Video effects processing
- `vp8_codec_demo/` - Video codec usage
