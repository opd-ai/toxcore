# Video Codec Implementation Documentation

## Overview

This document provides comprehensive documentation for the ToxAV video codec implementation, specifically the VP8 codec integration and video processing pipeline.

## Architecture

The video implementation follows the same patterns as the audio implementation for consistency:

```
VideoFrame Input → Validation → RealVP8Encoder (opd-ai/vp8) → RTP Packetization
VideoFrame Output ← Validation ← VP8 Decoder (x/image/vp8) ← RTP Depacketization
```

## Components

### 1. VP8Codec (`codec.go`)

The `VP8Codec` provides high-level VP8-specific functionality:

```go
// Create a new VP8 codec instance
codec := NewVP8Codec()

// Encode a video frame
data, err := codec.EncodeFrame(frame)

// Decode video data
frame, err := codec.DecodeFrame(data)

// Set encoding bitrate
err := codec.SetBitRate(1000000) // 1 Mbps
```

**Key Features:**
- VP8-specific frame validation
- Resolution and bitrate management
- Seamless integration with video processor
- Error handling and resource management

### 2. Video Processor (`processor.go`)

The core video processing engine:

```go
// Create processor with default settings (640x480, 512kbps)
processor := NewProcessor()

// Create processor with custom settings
processor := NewProcessorWithSettings(1280, 720, 2000000)

// Process outgoing video (returns RTP packets)
packets, err := processor.ProcessOutgoing(frame)

// Process incoming video (takes one RTPPacket at a time)
decodedFrame, err := processor.ProcessIncoming(packets[0])
```

**Key Features:**
- Comprehensive input validation
- YUV420 format handling
- Configurable frame dimensions and bitrates
- Thread-safe operations

### 3. RealVP8Encoder (Primary)

The primary encoder using `opd-ai/vp8` for actual VP8 compression:

```go
// Create encoder for specific dimensions
encoder := NewRealVP8Encoder(640, 480, 512000)

// Encode video frame → RFC 6386 VP8 bitstream (key or inter frame)
data, err := encoder.Encode(frame)

// Update bitrate
err := encoder.SetBitRate(1000000)

// Configure key frame interval (default: 30 frames)
encoder.SetKeyFrameInterval(30) // 1 key frame per second at 30fps

// Force a key frame on next encode
encoder.ForceKeyFrame()
```

**Output Format:** RFC 6386 VP8 bitstream (key frames and inter frames)
- Supports both I-frames (key frames) and P-frames (inter frames)
- Compatible with standard VP8 decoders and WebRTC stacks
- Actual lossy compression with motion estimation for P-frames
- Bitrate control via quantizer mapping
- Configurable key frame interval and loop filter

### 4. SimpleVP8Encoder (Fallback/Testing)

A minimal encoder that passes through YUV420 data with a dimension header.
Retained for testing and backward compatibility:

```go
encoder := NewSimpleVP8Encoder(640, 480, 512000)
```

## VideoFrame Structure

The `VideoFrame` type represents video data in YUV420 format:

```go
type VideoFrame struct {
    Width   uint16  // Frame width in pixels
    Height  uint16  // Frame height in pixels
    Y       []byte  // Luminance plane (full resolution)
    U       []byte  // Chrominance U plane (1/4 resolution)
    V       []byte  // Chrominance V plane (1/4 resolution)
    YStride int     // Stride for Y plane
    UStride int     // Stride for U plane  
    VStride int     // Stride for V plane
}
```

**YUV420 Format Details:**
- **Y Plane**: Width × Height bytes (luminance)
- **U Plane**: (Width × Height) / 4 bytes (blue chrominance)
- **V Plane**: (Width × Height) / 4 bytes (red chrominance)
- **Total Size**: Width × Height × 1.5 bytes per frame

## Supported Resolutions

Standard video calling resolutions supported:

| Name | Resolution | Pixels | Recommended Bitrate |
|------|------------|--------|-------------------|
| QQVGA | 160×120 | 19.2K | 64 kbps |
| QVGA | 320×240 | 76.8K | 128 kbps |
| VGA | 640×480 | 307.2K | 512 kbps |
| SVGA | 800×600 | 480K | 1 Mbps |
| XGA | 1024×768 | 786.4K | 1.5 Mbps |
| HD 720p | 1280×720 | 921.6K | 2 Mbps |
| HD 1080p | 1920×1080 | 2.07M | 4 Mbps |

## Performance Characteristics

Based on benchmark results (AMD Ryzen 7 7735HS):

| Operation | Latency | Memory Usage | Allocations |
|-----------|---------|--------------|-------------|
| Encode Frame (640×480) | ~35μs | ~467KB | 1 alloc |
| Decode Frame (640×480) | ~35μs | ~467KB | 1 alloc |
| Round Trip | ~82-100μs | ~942KB | 5 allocs |

**Real-Time Suitability:**
- 30 FPS: 33.3ms per frame budget
- Encoding latency: 35μs (0.1% of budget)
- Excellent performance for real-time video calls

## Usage Examples

### Basic Video Processing

```go
package main

import (
    "fmt"
    "github.com/opd-ai/toxcore/av/video"
)

func main() {
    // Create video processor
    processor := video.NewProcessor()
    
    // Create a test frame (640×480)
    frame := &video.VideoFrame{
        Width:   640,
        Height:  480,
        Y:       make([]byte, 640*480),
        U:       make([]byte, 640*480/4),
        V:       make([]byte, 640*480/4),
        YStride: 640,
        UStride: 320,
        VStride: 320,
    }
    
    // Fill with test pattern (gray frame)
    for i := range frame.Y {
        frame.Y[i] = 128 // Medium gray
    }
    for i := range frame.U {
        frame.U[i] = 128 // Neutral chrominance
    }
    for i := range frame.V {
        frame.V[i] = 128 // Neutral chrominance
    }
    
    // Encode the frame (returns RTP packets)
    packets, err := processor.ProcessOutgoing(frame)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Encoded %d RTP packets\n", len(packets))
    
    // Decode it back (takes one RTPPacket at a time)
    decodedFrame, err := processor.ProcessIncoming(packets[0])
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Decoded frame: %dx%d\n", decodedFrame.Width, decodedFrame.Height)
}
```

### VP8 Codec Usage

```go
package main

import (
    "fmt"
    "github.com/opd-ai/toxcore/av/video"
)

func main() {
    // Create VP8 codec
    codec := video.NewVP8Codec()
    defer codec.Close()
    
    // Set desired bitrate
    err := codec.SetBitRate(1000000) // 1 Mbps
    if err != nil {
        panic(err)
    }
    
    // Validate frame size
    err = codec.ValidateFrameSize(1280, 720)
    if err != nil {
        panic(err)
    }
    
    // Get appropriate bitrate for resolution
    resolution := video.Resolution{Width: 1280, Height: 720}
    recommendedBitrate := video.GetBitrateForResolution(resolution)
    fmt.Printf("Recommended bitrate for %s: %d bps\n", 
        resolution.String(), recommendedBitrate)
    
    // List supported resolutions
    resolutions := codec.GetSupportedResolutions()
    fmt.Printf("Supported resolutions: %d\n", len(resolutions))
    for _, res := range resolutions {
        fmt.Printf("  %s\n", res.String())
    }
}
```

### Custom Resolution Processing

```go
package main

import (
    "github.com/opd-ai/toxcore/av/video"
)

func main() {
    // Create processor with custom resolution
    processor := video.NewProcessorWithSettings(1280, 720, 2000000)
    defer processor.Close()
    
    // Get current settings
    width, height := processor.GetFrameSize()
    bitrate := processor.GetBitRate()
    
    fmt.Printf("Current settings: %dx%d @ %d bps\n", width, height, bitrate)
    
    // Change resolution during runtime
    err := processor.SetFrameSize(640, 480)
    if err != nil {
        panic(err)
    }
    
    // Update bitrate for new resolution
    err = processor.SetBitRate(512000)
    if err != nil {
        panic(err)
    }
}
```

## Error Handling

The video implementation provides comprehensive error handling:

```go
// Frame validation errors
func processFrame(processor *video.Processor, frame *video.VideoFrame) error {
    packets, err := processor.ProcessOutgoing(frame)
    if err != nil {
        switch {
        case strings.Contains(err.Error(), "frame cannot be nil"):
            return fmt.Errorf("invalid input: %w", err)
        case strings.Contains(err.Error(), "invalid frame dimensions"):
            return fmt.Errorf("dimension error: %w", err)
        case strings.Contains(err.Error(), "invalid Y plane size"):
            return fmt.Errorf("YUV format error: %w", err)
        default:
            return fmt.Errorf("processing error: %w", err)
        }
    }
    
    // Process data...
    return nil
}
```

## Design Decisions

### 1. Real VP8 Encoder via opd-ai/vp8

The `RealVP8Encoder` uses the pure Go `opd-ai/vp8` library:
- **Full VP8 Encoding**: Produces RFC 6386 compliant key frames and inter frames
- **Inter-frame Prediction**: P-frames with motion estimation for bandwidth efficiency
- **WebRTC Compatible**: Output works with standard VP8 decoders and pion/webrtc
- **Configurable Bitrate**: Maps bitrate to VP8 quantizer index for quality control
- **Loop Filter**: Reduces blocking artifacts in reference frames
- **Pure Go**: No CGo dependency required

The `SimpleVP8Encoder` is retained as a fallback/testing encoder.

### 2. VP8 Decoding via golang.org/x/image/vp8

Decoding uses the standard `golang.org/x/image/vp8` decoder:
- **Standard Library**: Well-maintained by the Go team
- **Key Frame Support**: Decodes VP8 key frames from the encoder
- **YCbCr Output**: Returns `image.YCbCr` with proper stride handling
- **Inter Frame Handling**: Inter frames (P-frames) are not decoded by `x/image/vp8`; the processor caches the last decoded key frame and returns it for P-frames, maintaining display continuity while benefiting from P-frame bandwidth savings on the wire

### 3. YUV420 Format Choice

- **Industry Standard**: Widely used in video processing
- **Efficient Storage**: 50% less data than RGB24
- **VP8 Compatible**: Native format for VP8 codec
- **Quality/Size Balance**: Good visual quality with reasonable bandwidth

### 4. Validation-First Approach

Comprehensive input validation to prevent runtime errors:
- **Frame dimensions** (non-zero, even numbers for VP8)
- **YUV plane sizes** (correct ratios for YUV420)
- **Data integrity** (valid VP8 bitstream format)
- **Resource bounds** (maximum frame sizes)

### 5. Thread-Safe Design

All components are designed for concurrent use:
- **Immutable configurations** where possible
- **Copy-based operations** for frame data
- **No shared mutable state** between operations
- **Proper resource cleanup** with Close methods

## Testing Strategy

Comprehensive test coverage includes:

### Unit Tests
- **Real VP8 encoder functionality** (multiple resolutions)
- **Simple encoder functionality** (passthrough verification)
- **VP8 round-trip** (encode → decode → verify dimensions)
- **Error conditions** (invalid inputs, edge cases)
- **Resource management** (proper cleanup)

### Integration Tests
- **Round-trip processing** (encode → decode → verify)
- **Codec integration** (VP8Codec → Processor → RealVP8Encoder)
- **Resolution changes** (runtime dimension updates)
- **Bitrate adjustments** (dynamic quality control)

### Performance Tests
- **Real VP8 encoding benchmarks** (latency and memory)
- **Simple encoder benchmarks** (baseline comparison)
- **Memory allocation** (allocation efficiency)
- **Real-time suitability** (30 FPS capability validation)

## Current Capabilities

### Inter-frame Prediction (P-frames) — Pure Go

P-frame support is now available natively through the `opd-ai/vp8` library:

**Default build (no CGo required):**
```bash
go build ./...  # Uses opd-ai/vp8, I-frames and P-frames
```

**Checking encoder capabilities at runtime:**
```go
encoder, _ := video.NewDefaultEncoder(640, 480, 512000)
if encoder.SupportsInterframe() {
    fmt.Println("P-frame support available")
}

// Configure key frame interval
encoder.SetKeyFrameInterval(30) // 1 key frame per second at 30fps

// Force a key frame (e.g., after scene change)
encoder.ForceKeyFrame()
```

**Optional CGo libvpx backend:**
```bash
# Install libvpx first:
# Ubuntu/Debian: apt-get install libvpx-dev
# macOS: brew install libvpx

go build -tags libvpx ./...  # Alternative VP8 via libvpx
```

## Future Enhancements

### Phase 3.1: P-frame Decoding
- **Native P-frame decoder**: The current `golang.org/x/image/vp8` decoder only supports key frames. Implementing a P-frame decoder would allow full utilization of inter-frame prediction on the receive side.
- **Reference frame management**: Track reference frames across decode calls for proper P-frame reconstruction.

### Phase 3.2: Advanced Features
- **Motion detection** (for bandwidth optimization)
- **Frame skipping** (adaptive quality)
- **Temporal scalability** for adaptive streaming
- **Quality presets** (low/medium/high with bitrate targets) — ✅ Implemented in `presets.go`

### Phase 3.3: Display Integration
- **Format conversion** (YUV420 to RGB for display)
- **Hardware acceleration** (where available)

## Library Dependencies

### Pure Go (default)
- **`github.com/opd-ai/vp8`** — VP8 encoding (key frames and inter frames with motion estimation, RFC 6386)
- **`golang.org/x/image/vp8`** — VP8 decoding (key frames; P-frames handled via frame caching)
- **No CGo requirements** for video codec functionality
- **Cross-platform compatibility** (Linux, macOS, Windows, WASM)

### Optional CGo (with `-tags libvpx`)
- **`github.com/xlab/libvpx-go`** — Full VP8 encoding with P-frames
- **Requires libvpx** native library installed
- **5-10x better bandwidth efficiency** than I-frame-only encoding
