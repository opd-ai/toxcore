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
// This package will be implemented across multiple phases as
// audio and video functionality is added.
package rtp

import (
	"time"
)

// Session represents an RTP session for a specific call.
//
// Each call will have its own RTP session that handles
// audio and video streams with the remote peer.
type Session struct {
	// TODO: Implement RTP session management
	// - SSRC management
	// - Sequence number tracking
	// - Timestamp management
	// - Jitter buffer integration
	friendNumber uint32
	audioSSRC    uint32
	videoSSRC    uint32
	created      time.Time
}

// NewSession creates a new RTP session for a friend.
//
// This function will initialize RTP session state including
// SSRC values, sequence numbers, and jitter buffers.
//
// Parameters:
//   - friendNumber: The friend number for this session
//
// Returns:
//   - *Session: The new RTP session
//   - error: Any error that occurred during setup
func NewSession(friendNumber uint32) (*Session, error) {
	// TODO: Implement in audio/video phases
	return &Session{
		friendNumber: friendNumber,
		created:      time.Now(),
	}, nil
}

// SendAudioPacket sends an RTP audio packet.
//
// This method will encapsulate audio data in RTP packets
// and send them over the Tox transport.
//
// Parameters:
//   - data: Encoded audio data
//
// Returns:
//   - error: Any error that occurred during sending
func (s *Session) SendAudioPacket(data []byte) error {
	// TODO: Implement in Phase 2: Audio Implementation
	return nil
}

// SendVideoPacket sends an RTP video packet.
//
// This method will encapsulate video data in RTP packets
// and send them over the Tox transport.
//
// Parameters:
//   - data: Encoded video data
//
// Returns:
//   - error: Any error that occurred during sending
func (s *Session) SendVideoPacket(data []byte) error {
	// TODO: Implement in Phase 3: Video Implementation
	return nil
}

// ReceivePacket processes an incoming RTP packet.
//
// This method will parse RTP packets and extract audio/video
// data for decoding and playback.
//
// Parameters:
//   - packet: Raw RTP packet data
//
// Returns:
//   - []byte: Extracted media data
//   - string: Media type ("audio" or "video")
//   - error: Any error that occurred during processing
func (s *Session) ReceivePacket(packet []byte) ([]byte, string, error) {
	// TODO: Implement in audio/video phases
	return nil, "", nil
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
	// TODO: Implement statistics collection
	return Statistics{}
}

// Close gracefully closes the RTP session.
func (s *Session) Close() error {
	// TODO: Implement session cleanup
	return nil
}
