// Package rtp provides RTP transport functionality for ToxAV.
//
// This package handles RTP packet creation, parsing, and transport
// over the existing Tox infrastructure. It uses the pion/rtp library
// for pure Go RTP packet handling.
//
// Design principles:
// - Leverage existing Tox transport infrastructure
// - Use pion/rtp for standards-compliant RTP implementation
// - Provide simple interface for audio frame packetization
// - Support jitter buffer for smooth audio playback
package rtp

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/transport"
	"github.com/pion/rtp"
)

// AudioPacketizer handles RTP packetization for audio frames.
//
// This provides a simple interface to convert encoded audio data
// into RTP packets for transmission over Tox transport.
type AudioPacketizer struct {
	mu             sync.RWMutex
	ssrc           uint32
	sequenceNumber uint16
	timestamp      uint32
	clockRate      uint32 // Audio clock rate (e.g., 48000 for Opus)
	transport      transport.Transport
	remoteAddr     net.Addr
}

// NewAudioPacketizer creates a new audio RTP packetizer.
//
// Parameters:
//   - clockRate: Audio clock rate in Hz (typically 48000 for Opus)
//   - transport: Tox transport for packet transmission
//   - remoteAddr: Remote peer address for packet transmission
//
// Returns:
//   - *AudioPacketizer: New packetizer instance
//   - error: Any error that occurred during setup
func NewAudioPacketizer(clockRate uint32, transport transport.Transport, remoteAddr net.Addr) (*AudioPacketizer, error) {
	if clockRate == 0 {
		return nil, fmt.Errorf("clock rate cannot be zero")
	}
	if transport == nil {
		return nil, fmt.Errorf("transport cannot be nil")
	}
	if remoteAddr == nil {
		return nil, fmt.Errorf("remote address cannot be nil")
	}

	// Generate random SSRC for this stream
	ssrcBytes := make([]byte, 4)
	if _, err := rand.Read(ssrcBytes); err != nil {
		return nil, fmt.Errorf("failed to generate SSRC: %w", err)
	}
	ssrc := binary.BigEndian.Uint32(ssrcBytes)

	return &AudioPacketizer{
		ssrc:           ssrc,
		sequenceNumber: 0,
		timestamp:      0,
		clockRate:      clockRate,
		transport:      transport,
		remoteAddr:     remoteAddr,
	}, nil
}

// PacketizeAndSend converts audio data to RTP packets and sends them.
//
// This method takes encoded audio data, wraps it in RTP packets,
// and sends them over the configured Tox transport.
//
// Parameters:
//   - audioData: Encoded audio data (e.g., Opus frames)
//   - sampleCount: Number of audio samples in this frame
//
// Returns:
//   - error: Any error that occurred during packetization or sending
func (ap *AudioPacketizer) PacketizeAndSend(audioData []byte, sampleCount uint32) error {
	if len(audioData) == 0 {
		return fmt.Errorf("audio data cannot be empty")
	}

	ap.mu.Lock()
	defer ap.mu.Unlock()

	// Create RTP packet
	packet := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Padding:        false,
			Extension:      false,
			Marker:         false, // Set to true for end of talkspurt if needed
			PayloadType:    96,    // Dynamic payload type for Opus (RFC 7587)
			SequenceNumber: ap.sequenceNumber,
			Timestamp:      ap.timestamp,
			SSRC:           ap.ssrc,
		},
		Payload: audioData,
	}

	// Serialize RTP packet
	rtpData, err := packet.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal RTP packet: %w", err)
	}

	// Create Tox transport packet
	toxPacket := &transport.Packet{
		PacketType: transport.PacketAVAudioFrame,
		Data:       rtpData,
	}

	// Send over Tox transport
	if err := ap.transport.Send(toxPacket, ap.remoteAddr); err != nil {
		return fmt.Errorf("failed to send audio RTP packet: %w", err)
	}

	// Update sequence number and timestamp
	ap.sequenceNumber++
	ap.timestamp += sampleCount

	return nil
}

// AudioDepacketizer handles RTP depacketization for incoming audio frames.
//
// This extracts audio data from received RTP packets and provides
// basic jitter buffer functionality for smooth playback.
type AudioDepacketizer struct {
	mu           sync.RWMutex
	expectedSSRC uint32
	hasSSRC      bool
	lastSeq      uint16
	hasLastSeq   bool
	jitterBuffer *JitterBuffer
}

// NewAudioDepacketizer creates a new audio RTP depacketizer.
//
// Returns:
//   - *AudioDepacketizer: New depacketizer instance
func NewAudioDepacketizer() *AudioDepacketizer {
	return &AudioDepacketizer{
		jitterBuffer: NewJitterBuffer(50 * time.Millisecond), // 50ms jitter buffer
	}
}

// ProcessPacket processes an incoming RTP audio packet.
//
// This method extracts audio data from RTP packets and handles
// basic packet validation and jitter buffering.
//
// Parameters:
//   - rtpData: Raw RTP packet data
//
// Returns:
//   - []byte: Extracted audio data
//   - uint32: Timestamp from RTP header
//   - error: Any error that occurred during processing
func (ad *AudioDepacketizer) ProcessPacket(rtpData []byte) ([]byte, uint32, error) {
	if len(rtpData) == 0 {
		return nil, 0, fmt.Errorf("RTP data cannot be empty")
	}

	// Parse RTP packet
	packet := &rtp.Packet{}
	if err := packet.Unmarshal(rtpData); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal RTP packet: %w", err)
	}

	ad.mu.Lock()
	defer ad.mu.Unlock()

	// Validate SSRC (accept first seen SSRC)
	if !ad.hasSSRC {
		ad.expectedSSRC = packet.SSRC
		ad.hasSSRC = true
	} else if packet.SSRC != ad.expectedSSRC {
		return nil, 0, fmt.Errorf("unexpected SSRC: expected %d, got %d", ad.expectedSSRC, packet.SSRC)
	}

	// Basic sequence number validation
	if ad.hasLastSeq {
		expectedSeq := ad.lastSeq + 1
		if packet.SequenceNumber != expectedSeq {
			// Simple gap detection - in production, implement proper jitter buffer
			fmt.Printf("Sequence gap detected: expected %d, got %d\n", expectedSeq, packet.SequenceNumber)
		}
	}
	ad.lastSeq = packet.SequenceNumber
	ad.hasLastSeq = true

	// Add to jitter buffer for smooth playback
	ad.jitterBuffer.Add(packet.Timestamp, packet.Payload)

	return packet.Payload, packet.Timestamp, nil
}

// GetBufferedAudio retrieves audio data from the jitter buffer.
//
// This provides smooth audio playback by buffering incoming packets
// and releasing them at appropriate times.
//
// Returns:
//   - []byte: Buffered audio data (nil if no data available)
//   - bool: Whether data was available
func (ad *AudioDepacketizer) GetBufferedAudio() ([]byte, bool) {
	return ad.jitterBuffer.Get()
}

// JitterBuffer provides basic jitter buffering for audio packets.
//
// This simple implementation buffers packets for a fixed duration
// to smooth out network jitter and provide consistent audio playback.
type JitterBuffer struct {
	mu          sync.RWMutex
	bufferTime  time.Duration
	packets     map[uint32][]byte // timestamp -> audio data
	lastDequeue time.Time
}

// NewJitterBuffer creates a new jitter buffer.
//
// Parameters:
//   - bufferTime: Duration to buffer packets
//
// Returns:
//   - *JitterBuffer: New jitter buffer instance
func NewJitterBuffer(bufferTime time.Duration) *JitterBuffer {
	return &JitterBuffer{
		bufferTime:  bufferTime,
		packets:     make(map[uint32][]byte),
		lastDequeue: time.Now(),
	}
}

// Add adds a packet to the jitter buffer.
//
// Parameters:
//   - timestamp: RTP timestamp
//   - data: Audio data
func (jb *JitterBuffer) Add(timestamp uint32, data []byte) {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	// Store packet with timestamp as key
	jb.packets[timestamp] = data
}

// Get retrieves the next packet from the jitter buffer.
//
// This implements a simple time-based release mechanism.
// In production, this should be enhanced with proper timestamp
// ordering and adaptive buffer management.
//
// Returns:
//   - []byte: Audio data (nil if no data ready)
//   - bool: Whether data was available
func (jb *JitterBuffer) Get() ([]byte, bool) {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	// Simple time-based release: wait for buffer time to pass
	if time.Since(jb.lastDequeue) < jb.bufferTime {
		return nil, false
	}

	// Get any available packet (simplified - should order by timestamp)
	for timestamp, data := range jb.packets {
		delete(jb.packets, timestamp)
		jb.lastDequeue = time.Now()
		return data, true
	}

	return nil, false
}

// Reset clears the jitter buffer.
func (jb *JitterBuffer) Reset() {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	jb.packets = make(map[uint32][]byte)
	jb.lastDequeue = time.Now()
}
