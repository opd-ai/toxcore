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
// - Deterministic testing support via injectable providers
//
// Implementation completed for Phase 2: Audio RTP packetization
// Video functionality will be added in Phase 3.
package rtp

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/av/video"
	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
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
	videoPacketizer   *video.RTPPacketizer
	videoDepacketizer *video.RTPDepacketizer
	transport         transport.Transport
	remoteAddr        net.Addr
	videoPictureID    uint16 // Current video picture ID

	// Providers for deterministic testing
	timeProvider TimeProvider
	ssrcProvider SSRCProvider

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
	return NewSessionWithProviders(friendNumber, transport, remoteAddr, DefaultTimeProvider{}, DefaultSSRCProvider{})
}

// NewSessionWithProviders creates a new RTP session with injectable providers.
//
// This constructor allows for deterministic testing by injecting custom
// TimeProvider and SSRCProvider implementations.
//
// Parameters:
//   - friendNumber: The friend number for this session
//   - transport: Tox transport for packet transmission
//   - remoteAddr: Remote peer address for packet transmission
//   - timeProvider: Provider for time operations
//   - ssrcProvider: Provider for SSRC generation
//
// Returns:
//   - *Session: The new RTP session
//   - error: Any error that occurred during setup
func NewSessionWithProviders(friendNumber uint32, transport transport.Transport, remoteAddr net.Addr, timeProvider TimeProvider, ssrcProvider SSRCProvider) (*Session, error) {
	if transport == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewSession",
			"error":    "transport cannot be nil",
		}).Error("Invalid transport")
		return nil, fmt.Errorf("transport cannot be nil")
	}
	if remoteAddr == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewSession",
			"error":    "remote address cannot be nil",
		}).Error("Invalid remote address")
		return nil, fmt.Errorf("remote address cannot be nil")
	}
	if timeProvider == nil {
		timeProvider = DefaultTimeProvider{}
	}
	if ssrcProvider == nil {
		ssrcProvider = DefaultSSRCProvider{}
	}

	logrus.WithFields(logrus.Fields{
		"function":      "NewSession",
		"friend_number": friendNumber,
		"remote_addr":   remoteAddr.String(),
	}).Info("Creating new RTP session")

	// Create audio packetizer with standard Opus clock rate
	audioPacketizer, err := NewAudioPacketizerWithSSRCProvider(48000, transport, remoteAddr, ssrcProvider)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewSession",
			"error":    err.Error(),
		}).Error("Failed to create audio packetizer")
		return nil, fmt.Errorf("failed to create audio packetizer: %w", err)
	}

	// Create audio depacketizer
	audioDepacketizer := NewAudioDepacketizer()

	// Generate video SSRC using provider (deterministic in tests, random in production)
	videoSSRC, err := ssrcProvider.GenerateSSRC()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewSession",
			"error":    err.Error(),
		}).Error("Failed to generate video SSRC")
		return nil, fmt.Errorf("failed to generate video SSRC: %w", err)
	}

	// Create video packetizer and depacketizer
	videoPacketizer := video.NewRTPPacketizer(videoSSRC)
	videoDepacketizer := video.NewRTPDepacketizer()

	now := timeProvider.Now()
	session := &Session{
		friendNumber:      friendNumber,
		videoSSRC:         videoSSRC,
		created:           now,
		audioPacketizer:   audioPacketizer,
		audioDepacketizer: audioDepacketizer,
		videoPacketizer:   videoPacketizer,
		videoDepacketizer: videoDepacketizer,
		transport:         transport,
		remoteAddr:        remoteAddr,
		videoPictureID:    1, // Start from 1
		timeProvider:      timeProvider,
		ssrcProvider:      ssrcProvider,
		stats: Statistics{
			StartTime: now,
		},
	}

	logrus.WithFields(logrus.Fields{
		"function":        "NewSession",
		"friend_number":   friendNumber,
		"session_created": session.created,
	}).Info("RTP session created successfully")

	return session, nil
}

// SetTimeProvider sets the time provider for the session.
// This allows for deterministic testing by injecting a mock time provider.
func (s *Session) SetTimeProvider(tp TimeProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tp == nil {
		tp = DefaultTimeProvider{}
	}
	s.timeProvider = tp
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
	s.stats.BytesSent += uint64(len(data))

	return nil
}

// SendVideoPacket sends an RTP video packet.
//
// This method takes encoded video data, wraps it in RTP packets
// using the session's video packetizer, and sends them over the
// Tox transport with proper VP8 payload formatting.
//
// Parameters:
//   - data: Encoded video data (VP8 frames)
//
// Returns:
//   - error: Any error that occurred during sending
func (s *Session) SendVideoPacket(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.videoPacketizer == nil {
		return fmt.Errorf("video packetizer not initialized")
	}

	if len(data) == 0 {
		return fmt.Errorf("video data cannot be empty")
	}

	// Calculate timestamp (90kHz clock for video) using time provider
	elapsed := s.timeProvider.Now().Sub(s.created)
	timestamp := uint32(elapsed.Milliseconds() * 90)

	// Packetize the video frame
	rtpPackets, err := s.videoPacketizer.PacketizeFrame(data, timestamp, s.videoPictureID)
	if err != nil {
		return fmt.Errorf("failed to packetize video frame: %w", err)
	}

	// Send each RTP packet over transport
	for _, rtpPacket := range rtpPackets {
		// Serialize RTP packet
		packetData := serializeVideoRTPPacket(rtpPacket)

		// Send via Tox transport
		toxPacket := &transport.Packet{
			PacketType: transport.PacketAVVideoFrame,
			Data:       packetData,
		}

		if err := s.transport.Send(toxPacket, s.remoteAddr); err != nil {
			return fmt.Errorf("failed to send video packet: %w", err)
		}
	}

	// Update statistics
	s.stats.PacketsSent += uint64(len(rtpPackets))
	s.stats.BytesSent += uint64(len(data))

	// Increment picture ID for next frame
	s.videoPictureID++
	if s.videoPictureID == 0 { // Handle overflow
		s.videoPictureID = 1
	}

	return nil
}

// ReceivePacket processes an incoming RTP packet.
//
// This method parses RTP packets and extracts audio/video
// data for decoding and playback using the session's
// depacketizers. Currently supports audio packets only;
// use ReceiveVideoPacket for video packets.
//
// Parameters:
//   - packet: Raw RTP packet data
//
// Returns:
//   - []byte: Extracted media data
//   - string: Media type ("audio")
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

	// Process as audio packet
	audioData, timestamp, err := s.audioDepacketizer.ProcessPacket(packet)
	if err != nil {
		return nil, "", fmt.Errorf("failed to process audio packet: %w", err)
	}

	// Update statistics
	s.stats.PacketsReceived++

	_ = timestamp // Will be used for jitter calculation
	return audioData, "audio", nil
}

// ReceiveVideoPacket processes an incoming video RTP packet.
//
// This method parses video RTP packets and extracts VP8 frame
// data using the session's video depacketizer, reassembling
// fragmented frames as needed.
//
// Parameters:
//   - packet: Raw RTP packet data
//
// Returns:
//   - []byte: Complete VP8 frame data (nil if frame incomplete)
//   - uint16: Picture ID of complete frame
//   - error: Any error that occurred during processing
func (s *Session) ReceiveVideoPacket(packet []byte) ([]byte, uint16, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(packet) == 0 {
		return nil, 0, fmt.Errorf("packet cannot be empty")
	}

	if s.videoDepacketizer == nil {
		return nil, 0, fmt.Errorf("video depacketizer not initialized")
	}

	// Deserialize the RTP packet
	rtpPacket, err := deserializeVideoRTPPacket(packet)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to deserialize video packet: %w", err)
	}

	// Process the packet and attempt frame reassembly
	frameData, pictureID, err := s.videoDepacketizer.ProcessPacket(rtpPacket)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to process video packet: %w", err)
	}

	// Update statistics
	s.stats.PacketsReceived++

	// Return complete frame if available, otherwise nil
	return frameData, pictureID, nil
}

// GetStatistics returns RTP session statistics.
//
// This method provides quality monitoring information
// including packet loss, jitter, and bandwidth usage.
type Statistics struct {
	PacketsSent     uint64
	PacketsReceived uint64
	PacketsLost     uint64
	BytesSent       uint64
	Jitter          time.Duration
	Bandwidth       uint64 // bits per second
	StartTime       time.Time
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

// serializeVideoRTPPacket converts a video RTP packet to wire format.
//
// This serializes the RTP header and VP8 payload according to RFC 3550
// and RFC 7741 specifications for transmission.
//
// Parameters:
//   - packet: The RTP packet to serialize
//
// Returns:
//   - []byte: Serialized packet data
func serializeVideoRTPPacket(packet video.RTPPacket) []byte {
	// RTP header is 12 bytes
	header := make([]byte, 12)

	// Byte 0: V(2)|P(1)|X(1)|CC(4)
	header[0] = (packet.Version << 6) | (packet.CSRCCount & 0x0F)
	if packet.Padding {
		header[0] |= 0x20
	}
	if packet.Extension {
		header[0] |= 0x10
	}

	// Byte 1: M(1)|PT(7)
	header[1] = packet.PayloadType & 0x7F
	if packet.Marker {
		header[1] |= 0x80
	}

	// Bytes 2-3: Sequence number
	header[2] = byte(packet.SequenceNumber >> 8)
	header[3] = byte(packet.SequenceNumber)

	// Bytes 4-7: Timestamp
	header[4] = byte(packet.Timestamp >> 24)
	header[5] = byte(packet.Timestamp >> 16)
	header[6] = byte(packet.Timestamp >> 8)
	header[7] = byte(packet.Timestamp)

	// Bytes 8-11: SSRC
	header[8] = byte(packet.SSRC >> 24)
	header[9] = byte(packet.SSRC >> 16)
	header[10] = byte(packet.SSRC >> 8)
	header[11] = byte(packet.SSRC)

	// Combine header and payload (payload already includes VP8 descriptor)
	result := make([]byte, len(header)+len(packet.Payload))
	copy(result, header)
	copy(result[len(header):], packet.Payload)

	return result
}

// deserializeVideoRTPPacket parses wire format data into a video RTP packet.
//
// This deserializes the RTP header and extracts the VP8 payload according
// to RFC 3550 and RFC 7741 specifications, parsing the VP8 payload descriptor
// to extract key frame information.
//
// Parameters:
//   - data: Serialized packet data
//
// Returns:
//   - video.RTPPacket: Parsed RTP packet
//   - error: Any error that occurred during parsing
func deserializeVideoRTPPacket(data []byte) (video.RTPPacket, error) {
	if len(data) < 12 {
		return video.RTPPacket{}, fmt.Errorf("packet too short: %d bytes (minimum 12)", len(data))
	}

	packet := video.RTPPacket{}

	// Parse RTP header
	packet.Version = (data[0] >> 6) & 0x03
	packet.Padding = (data[0] & 0x20) != 0
	packet.Extension = (data[0] & 0x10) != 0
	packet.CSRCCount = data[0] & 0x0F

	packet.Marker = (data[1] & 0x80) != 0
	packet.PayloadType = data[1] & 0x7F

	packet.SequenceNumber = uint16(data[2])<<8 | uint16(data[3])
	packet.Timestamp = uint32(data[4])<<24 | uint32(data[5])<<16 | uint32(data[6])<<8 | uint32(data[7])
	packet.SSRC = uint32(data[8])<<24 | uint32(data[9])<<16 | uint32(data[10])<<8 | uint32(data[11])

	// Payload includes VP8 descriptor
	packet.Payload = make([]byte, len(data)-12)
	copy(packet.Payload, data[12:])

	// Parse VP8 payload descriptor to extract key frame information
	if len(packet.Payload) >= 3 {
		firstByte := packet.Payload[0]
		packet.ExtendedControlBits = (firstByte & 0x80) != 0 // X bit
		packet.NonReferenceBit = (firstByte & 0x20) != 0     // N bit
		packet.StartOfPartition = (firstByte & 0x10) != 0    // S bit

		// Extract Picture ID if extended control bits are present
		if packet.ExtendedControlBits && len(packet.Payload) >= 3 {
			packet.PictureID = uint16(packet.Payload[1]&0x7F)<<8 | uint16(packet.Payload[2])
		}
	}

	return packet, nil
}
