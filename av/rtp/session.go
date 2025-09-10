// Package rtp provides RTP transport functionality for ToxAV.
//
// This package handles RTP session management, packet handling,
// jitter buffer management, and RTP transport over the existing
// Tox transport infrastructure.
//
// The RTP integration provides:
// - RTP packet encapsulation for audio/video data
// - Jitter buffer management for smooth playback
// - Integration with existing Tox transport security
// - Quality monitoring and adaptation
//
// Implementation completed for Phase 2: Audio RTP packetization
// Video functionality will be added in Phase 3.
package rtp

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/transport"
)

// Session represents an RTP session for a specific call.
//
// Each call will have its own RTP session that handles
// audio and video streams with the remote peer using
// the RTP packetization implemented in packet.go.
type Session struct {
	mu           sync.RWMutex
	friendNumber uint32
	audioSSRC    uint32
	videoSSRC    uint32
	created      time.Time

	// RTP packetization components
	audioPacketizer   *AudioPacketizer
	audioDepacketizer *AudioDepacketizer
	transport         transport.Transport
	remoteAddr        net.Addr

	// Statistics tracking
	stats Statistics
}

// NewSession creates a new RTP session for a friend.
//
// This function initializes RTP session state including
// packetizers, depacketizers, and transport configuration.
//
// Parameters:
//   - friendNumber: The friend number for this session
//   - transport: Tox transport for packet transmission
//   - remoteAddr: Remote peer address for packet transmission
//
// Returns:
//   - *Session: The new RTP session
//   - error: Any error that occurred during setup
func NewSession(friendNumber uint32, transport transport.Transport, remoteAddr net.Addr) (*Session, error) {
	if transport == nil {
		return nil, fmt.Errorf("transport cannot be nil")
	}
	if remoteAddr == nil {
		return nil, fmt.Errorf("remote address cannot be nil")
	}

	// Create audio packetizer with standard Opus clock rate
	audioPacketizer, err := NewAudioPacketizer(48000, transport, remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create audio packetizer: %w", err)
	}

	// Create audio depacketizer
	audioDepacketizer := NewAudioDepacketizer()

	return &Session{
		friendNumber:      friendNumber,
		created:           time.Now(),
		audioPacketizer:   audioPacketizer,
		audioDepacketizer: audioDepacketizer,
		transport:         transport,
		remoteAddr:        remoteAddr,
		stats:             Statistics{},
	}, nil
}

// SendAudioPacket sends an RTP audio packet.
//
// This method takes encoded audio data, wraps it in RTP packets
// using the session's audio packetizer, and sends them over the
// Tox transport.
//
// Parameters:
//   - data: Encoded audio data (e.g., Opus frames)
//   - sampleCount: Number of audio samples in this frame
//
// Returns:
//   - error: Any error that occurred during sending
func (s *Session) SendAudioPacket(data []byte, sampleCount uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.audioPacketizer == nil {
		return fmt.Errorf("audio packetizer not initialized")
	}

	err := s.audioPacketizer.PacketizeAndSend(data, sampleCount)
	if err != nil {
		return fmt.Errorf("failed to send audio packet: %w", err)
	}

	// Update statistics
	s.stats.PacketsSent++

	return nil
}

// SendVideoPacket sends an RTP video packet.
//
// This method will encapsulate video data in RTP packets
// and send them over the Tox transport.
//
// TODO: Implement in Phase 3: Video Implementation
//
// Parameters:
//   - data: Encoded video data
//
// Returns:
//   - error: Any error that occurred during sending
func (s *Session) SendVideoPacket(data []byte) error {
	// TODO: Implement in Phase 3: Video Implementation
	return fmt.Errorf("video RTP packetization not yet implemented")
}

// ReceivePacket processes an incoming RTP packet.
//
// This method parses RTP packets and extracts audio/video
// data for decoding and playback using the session's
// depacketizers.
//
// Parameters:
//   - packet: Raw RTP packet data
//
// Returns:
//   - []byte: Extracted media data
//   - string: Media type ("audio" or "video")
//   - error: Any error that occurred during processing
func (s *Session) ReceivePacket(packet []byte) ([]byte, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(packet) == 0 {
		return nil, "", fmt.Errorf("packet cannot be empty")
	}

	if s.audioDepacketizer == nil {
		return nil, "", fmt.Errorf("audio depacketizer not initialized")
	}

	// Try to process as audio packet
	audioData, timestamp, err := s.audioDepacketizer.ProcessPacket(packet)
	if err != nil {
		return nil, "", fmt.Errorf("failed to process audio packet: %w", err)
	}

	// Update statistics
	s.stats.PacketsReceived++

	// For now, all packets are treated as audio
	// Video support will be added in Phase 3
	_ = timestamp // Will be used for jitter calculation
	return audioData, "audio", nil
}

// GetStatistics returns RTP session statistics.
//
// This method provides quality monitoring information
// including packet loss, jitter, and bandwidth usage.
type Statistics struct {
	PacketsSent     uint64
	PacketsReceived uint64
	PacketsLost     uint64
	Jitter          time.Duration
	Bandwidth       uint64 // bits per second
}

// GetStatistics returns current session statistics.
func (s *Session) GetStatistics() Statistics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.stats
}

// Close gracefully closes the RTP session.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up resources
	s.audioPacketizer = nil
	s.audioDepacketizer = nil

	return nil
}

// GetBufferedAudio retrieves buffered audio data from the jitter buffer.
//
// This provides access to the audio depacketizer's jitter buffer
// for smooth audio playback.
//
// Returns:
//   - []byte: Buffered audio data (nil if no data available)
//   - bool: Whether data was available
func (s *Session) GetBufferedAudio() ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.audioDepacketizer == nil {
		return nil, false
	}

	return s.audioDepacketizer.GetBufferedAudio()
}
