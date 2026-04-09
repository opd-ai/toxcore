# ToxAV Examples

Examples demonstrating ToxAV audio/video calling in toxcore-go.

## Examples

### 1. Basic Audio/Video Call (`toxav_basic_call/`)
Complete call lifecycle: initiate, answer, end. Audio (440Hz sine), video (animated patterns), real-time stats.

### 2. Audio-Only Call (`toxav_audio_call/`)
Musical tone generation (C major scale), audio effects chain (gain, AGC), real-time analysis (peak, RMS).

### 3. Integration Demo (`toxav_integration/`)
Full Tox client with interactive CLI, friend management, messaging, profile persistence, and call coordination.

### 4. Video Call (`toxav_video_call/`)
5 video patterns at 30 FPS, YUV420 format, pattern cycling, performance measurement.

### 5. Effects Processing (`toxav_effects_processing/`)
Audio effects (noise suppression, gain, AGC), video effects (color temperature), real-time parameter adjustment.

## Quick Start

```bash
cd examples/toxav_basic_call
go run main.go
```

To test: run example, get Tox ID, add as friend from another Tox client, initiate call.

## Configuration

| Setting | Audio | Video |
|---------|-------|-------|
| Sample/Frame Rate | 48kHz | 30 FPS |
| Frame Size | 480 (10ms) | 640×480 |
| Bit Rate | 64 kbps | 500 kbps |
| Channels/Format | 1-2 (PCM) | YUV420 |

## Key API Patterns

```go
// Setup
tox, _ := toxcore.New(toxcore.NewOptions())
toxav, _ := toxcore.NewToxAV(tox)

// Callbacks
toxav.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
    toxav.Answer(friendNumber, 64000, 500000)
})

// Send frames
toxav.AudioSendFrame(friendNumber, pcm, sampleCount, channels, sampleRate)
toxav.VideoSendFrame(friendNumber, width, height, y, u, v)
```

## Troubleshooting

- **ToxAV creation fails**: Ensure Tox transport is initialized first
- **Frame send errors**: Check for active calls before sending
- **Format issues**: Audio must be PCM int16, video must be YUV420
- **Debug**: `logrus.SetLevel(logrus.DebugLevel)`

## Dependencies

- `github.com/opd-ai/toxcore` — Core Tox
- `github.com/opd-ai/toxcore/av` — ToxAV
- `github.com/opd-ai/toxcore/av/audio` — Audio processing (optional)
- `github.com/opd-ai/toxcore/av/video` — Video processing (optional)

