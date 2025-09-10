# RTP Package for ToxAV

This package provides RTP (Real-time Transport Protocol) functionality for ToxAV audio/video communication over the Tox network.

## Overview

The RTP package implements standards-compliant RTP packet handling using the pure Go `pion/rtp` library, providing:

- **Audio RTP Packetization**: Convert encoded audio data to RTP packets for transmission
- **Audio RTP Depacketization**: Extract audio data from received RTP packets  
- **Jitter Buffer**: Simple time-based buffering for smooth audio playback
- **Session Management**: Per-call RTP sessions with statistics tracking
- **Transport Integration**: Bridge between RTP and existing Tox transport infrastructure

## Key Features

- **Pure Go Implementation**: No CGo dependencies, uses `pion/rtp` library
- **Standards Compliant**: RFC-compliant RTP packet handling  
- **High Performance**: Sub-microsecond packetization/depacketization
- **Thread Safe**: Concurrent access protection with proper mutex usage
- **Opus Compatible**: Configured for Opus audio codec (payload type 96)
- **Test Coverage**: 91.9% test coverage with comprehensive error handling

## Architecture

```
Audio Data → AudioPacketizer → RTP Packets → Tox Transport
                                   ↓
Audio Data ← AudioDepacketizer ← RTP Packets ← Tox Transport
```

## Components

### AudioPacketizer (`packet.go`)
- Converts audio data to RTP packets
- Automatic SSRC generation and sequence numbering
- Timestamp management for audio streams

### AudioDepacketizer (`packet.go`)  
- Extracts audio data from RTP packets
- SSRC validation and sequence gap detection
- Basic jitter buffer integration

### Session (`session.go`)
- Per-call RTP session management
- Audio and video stream handling
- Statistics tracking (packets sent/received)

### TransportIntegration (`transport.go`)
- Bridge between RTP sessions and Tox transport
- Packet routing and session lifecycle management
- Handler registration for audio/video frames

## Usage Example

```go
// Create RTP session
transport := // ... existing Tox transport
remoteAddr := // ... remote peer address
session, err := rtp.NewSession(friendNumber, transport, remoteAddr)

// Send audio data
audioData := []byte{...} // Encoded Opus audio
sampleCount := uint32(960) // 20ms at 48kHz
err = session.SendAudioPacket(audioData, sampleCount)

// Receive RTP packet
rtpData := []byte{...} // Raw RTP packet
audioData, mediaType, err := session.ReceivePacket(rtpData)
```

## Performance

Benchmark results on AMD Ryzen 7 7735HS:

- **Packetization**: 245ns per operation, 2 allocations
- **Depacketization**: 438ns per operation, 1 allocation  
- **Session Send**: 245ns per operation, 2 allocations

## Future Enhancements

- Video RTP packetization (Phase 3)
- Advanced jitter buffer with timestamp ordering
- Adaptive buffer management based on network conditions
- RTCP support for quality feedback
- Packet loss detection and recovery

## Dependencies

- `github.com/pion/rtp`: Pure Go RTP implementation
- `github.com/opd-ai/toxcore/transport`: Tox transport layer

## Status

**Phase 2 - Audio Implementation**: ✅ **COMPLETED**
- RTP audio packetization fully implemented
- Comprehensive test coverage and documentation
- Ready for integration with ToxAV audio frame sending/receiving
