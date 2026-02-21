// Package rtp provides RTP (Real-time Transport Protocol) transport functionality
// for ToxAV audio and video streaming.
//
// This package handles RTP packet creation, parsing, jitter buffering, and transport
// over the existing Tox infrastructure. It uses the pion/rtp library for
// standards-compliant RTP packet handling.
//
// # Architecture Overview
//
// The RTP transport layer consists of several key components:
//
//   - AudioPacketizer: Converts raw audio frames into RTP packets
//   - AudioDepacketizer: Reconstructs audio frames from RTP packets
//   - JitterBuffer: Smooths out network jitter for consistent playback
//   - Session: Manages RTP sessions with statistics tracking
//   - RTPTransport: Integrates RTP with Tox transport infrastructure
//
// # Audio Packetization
//
// Audio frames are packetized using the AudioPacketizer:
//
//	packetizer, err := rtp.NewAudioPacketizer(clockRate, payloadType)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	packets, err := packetizer.Packetize(audioData, 20 * time.Millisecond)
//
// The packetizer generates unique SSRC (Synchronization Source) identifiers
// and handles sequence numbering and timestamping automatically.
//
// # Audio Depacketization
//
// Incoming RTP packets are processed by the AudioDepacketizer:
//
//	depacketizer := rtp.NewAudioDepacketizer(clockRate, 50 * time.Millisecond)
//	audioData, timestamp, err := depacketizer.ProcessPacket(rtpData)
//	if err != nil {
//	    log.Printf("Error processing packet: %v", err)
//	}
//
// The depacketizer includes a jitter buffer to smooth out network delays
// and provide consistent audio playback.
//
// # Jitter Buffer
//
// The JitterBuffer provides basic packet buffering to handle network jitter:
//
//	buffer := rtp.NewJitterBuffer(50 * time.Millisecond)
//	buffer.Add(timestamp, audioData)
//	data, available := buffer.Get()
//
// The buffer uses a simple time-based approach where packets are held for
// a configurable duration before being released for playback.
//
// # Session Management
//
// RTP sessions track statistics and manage packet flow:
//
//	session, err := rtp.NewSession(friendID, callID, transport)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer session.Close()
//
//	err = session.SendAudioFrame(audioData)
//	stats := session.GetStatistics()
//
// Sessions support both audio and video streams with separate SSRC identifiers.
//
// # Deterministic Testing
//
// All time-dependent and random operations support injectable providers
// for deterministic testing:
//
//	// Custom time provider for testing
//	type MockTimeProvider struct {
//	    currentTime time.Time
//	}
//	func (m *MockTimeProvider) Now() time.Time { return m.currentTime }
//
//	// Custom SSRC provider for testing
//	type MockSSRCProvider struct {
//	    nextSSRC uint32
//	}
//	func (m *MockSSRCProvider) GenerateSSRC() (uint32, error) { return m.nextSSRC, nil }
//
//	// Use with constructors
//	packetizer, _ := rtp.NewAudioPacketizerWithSSRCProvider(clockRate, pt, &MockSSRCProvider{})
//	buffer := rtp.NewJitterBufferWithTimeProvider(duration, &MockTimeProvider{})
//
// # Packet Type Registration
//
// The RTP transport registers handlers for AV packet types:
//
//   - PacketAVAudioFrame (0x15): Audio RTP frames
//   - PacketAVVideoFrame (0x16): Video RTP frames
//
// These handlers are automatically registered when creating an RTPTransport
// instance with the Tox transport layer.
//
// # Thread Safety
//
// All exported types are safe for concurrent use from multiple goroutines.
// Internal synchronization uses sync.RWMutex for efficient read-heavy workloads.
//
// # Integration with ToxAV
//
// This package integrates with the parent av package through:
//
//   - Call.SetupMedia() creates RTP sessions for calls
//   - Manager routes packets through RTP transport handlers
//   - Video frames use the av/video package for codec operations
//
// # Limitations
//
//   - Video handler is placeholder pending Phase 3 implementation
//
// Note: The jitter buffer now uses a sorted slice with binary search insertion
// for timestamp-ordered packet delivery, and includes configurable capacity
// limits with automatic pruning to prevent unbounded memory growth.
//
// For more detailed integration documentation, see INTEGRATION.md.
package rtp
