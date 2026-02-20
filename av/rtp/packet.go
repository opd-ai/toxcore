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
// - Support deterministic testing via injectable TimeProvider and SSRCProvider
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
	"github.com/sirupsen/logrus"
)

// TimeProvider abstracts time operations for deterministic testing.
// Production code uses DefaultTimeProvider; tests can inject mock implementations.
type TimeProvider interface {
	Now() time.Time
}

// DefaultTimeProvider uses the standard time package.
type DefaultTimeProvider struct{}

// Now returns the current time.
func (d DefaultTimeProvider) Now() time.Time {
	return time.Now()
}

// SSRCProvider abstracts SSRC generation for deterministic testing.
// Production code uses DefaultSSRCProvider; tests can inject mock implementations.
type SSRCProvider interface {
	GenerateSSRC() (uint32, error)
}

// DefaultSSRCProvider uses crypto/rand for secure SSRC generation.
type DefaultSSRCProvider struct{}

// GenerateSSRC generates a cryptographically random SSRC.
func (d DefaultSSRCProvider) GenerateSSRC() (uint32, error) {
	ssrcBytes := make([]byte, 4)
	if _, err := rand.Read(ssrcBytes); err != nil {
		return 0, fmt.Errorf("failed to generate SSRC: %w", err)
	}
	return binary.BigEndian.Uint32(ssrcBytes), nil
}

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
	ssrcProvider   SSRCProvider
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
	return NewAudioPacketizerWithSSRCProvider(clockRate, transport, remoteAddr, DefaultSSRCProvider{})
}

// NewAudioPacketizerWithSSRCProvider creates a new audio RTP packetizer with an injectable SSRC provider.
//
// This constructor allows for deterministic testing by injecting a custom SSRCProvider.
//
// Parameters:
//   - clockRate: Audio clock rate in Hz (typically 48000 for Opus)
//   - transport: Tox transport for packet transmission
//   - remoteAddr: Remote peer address for packet transmission
//   - ssrcProvider: Provider for SSRC generation
//
// Returns:
//   - *AudioPacketizer: New packetizer instance
//   - error: Any error that occurred during setup
func NewAudioPacketizerWithSSRCProvider(clockRate uint32, transport transport.Transport, remoteAddr net.Addr, ssrcProvider SSRCProvider) (*AudioPacketizer, error) {
	if err := validatePacketizerInputs(clockRate, transport, remoteAddr); err != nil {
		return nil, err
	}

	ssrcProvider = ensurePacketizerSSRCProvider(ssrcProvider)
	logPacketizerCreation(clockRate, remoteAddr)

	ssrc, err := generatePacketizerSSRC(ssrcProvider)
	if err != nil {
		return nil, err
	}

	packetizer := buildAudioPacketizer(ssrc, clockRate, transport, remoteAddr, ssrcProvider)
	logPacketizerSuccess(ssrc, clockRate)

	return packetizer, nil
}

// validatePacketizerInputs validates the required parameters for audio packetizer creation.
func validatePacketizerInputs(clockRate uint32, transport transport.Transport, remoteAddr net.Addr) error {
	if clockRate == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "NewAudioPacketizer",
			"error":    "clock rate cannot be zero",
		}).Error("Invalid clock rate")
		return fmt.Errorf("clock rate cannot be zero")
	}
	if transport == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewAudioPacketizer",
			"error":    "transport cannot be nil",
		}).Error("Invalid transport")
		return fmt.Errorf("transport cannot be nil")
	}
	if remoteAddr == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewAudioPacketizer",
			"error":    "remote address cannot be nil",
		}).Error("Invalid remote address")
		return fmt.Errorf("remote address cannot be nil")
	}
	return nil
}

// ensurePacketizerSSRCProvider returns the provided SSRC provider or a default if nil.
func ensurePacketizerSSRCProvider(ssrcProvider SSRCProvider) SSRCProvider {
	if ssrcProvider == nil {
		return DefaultSSRCProvider{}
	}
	return ssrcProvider
}

// logPacketizerCreation logs the start of audio packetizer creation.
func logPacketizerCreation(clockRate uint32, remoteAddr net.Addr) {
	logrus.WithFields(logrus.Fields{
		"function":    "NewAudioPacketizer",
		"clock_rate":  clockRate,
		"remote_addr": remoteAddr.String(),
	}).Info("Creating new audio packetizer")
}

// generatePacketizerSSRC generates an SSRC using the provided generator.
func generatePacketizerSSRC(ssrcProvider SSRCProvider) (uint32, error) {
	ssrc, err := ssrcProvider.GenerateSSRC()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewAudioPacketizer",
			"error":    err.Error(),
		}).Error("Failed to generate SSRC")
		return 0, fmt.Errorf("failed to generate SSRC: %w", err)
	}
	return ssrc, nil
}

// buildAudioPacketizer constructs an AudioPacketizer with the given parameters.
func buildAudioPacketizer(ssrc, clockRate uint32, transport transport.Transport, remoteAddr net.Addr, ssrcProvider SSRCProvider) *AudioPacketizer {
	return &AudioPacketizer{
		ssrc:           ssrc,
		sequenceNumber: 0,
		timestamp:      0,
		clockRate:      clockRate,
		transport:      transport,
		remoteAddr:     remoteAddr,
		ssrcProvider:   ssrcProvider,
	}
}

// logPacketizerSuccess logs successful audio packetizer creation.
func logPacketizerSuccess(ssrc, clockRate uint32) {
	logrus.WithFields(logrus.Fields{
		"function":   "NewAudioPacketizer",
		"ssrc":       ssrc,
		"clock_rate": clockRate,
	}).Info("Audio packetizer created successfully")
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
	logrus.WithFields(logrus.Fields{
		"function":     "AudioPacketizer.PacketizeAndSend",
		"data_size":    len(audioData),
		"sample_count": sampleCount,
	}).Debug("Starting audio packetization")

	if err := validateAudioData(audioData); err != nil {
		return err
	}

	ap.mu.Lock()
	defer ap.mu.Unlock()

	rtpData, err := ap.createAndMarshalRTPPacket(audioData)
	if err != nil {
		return err
	}

	if err := ap.sendToxPacket(rtpData); err != nil {
		return err
	}

	ap.updateRTPCounters(sampleCount)

	logrus.WithFields(logrus.Fields{
		"function":      "AudioPacketizer.PacketizeAndSend",
		"new_sequence":  ap.sequenceNumber,
		"new_timestamp": ap.timestamp,
	}).Debug("Audio packet sent successfully")

	return nil
}

// validateAudioData validates that audio data is non-empty.
func validateAudioData(audioData []byte) error {
	if len(audioData) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "AudioPacketizer.PacketizeAndSend",
			"error":    "audio data cannot be empty",
		}).Error("Invalid audio data")
		return fmt.Errorf("audio data cannot be empty")
	}
	return nil
}

// createAndMarshalRTPPacket creates and marshals an RTP packet from audio data.
func (ap *AudioPacketizer) createAndMarshalRTPPacket(audioData []byte) ([]byte, error) {
	packet := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Padding:        false,
			Extension:      false,
			Marker:         false,
			PayloadType:    96,
			SequenceNumber: ap.sequenceNumber,
			Timestamp:      ap.timestamp,
			SSRC:           ap.ssrc,
		},
		Payload: audioData,
	}

	logrus.WithFields(logrus.Fields{
		"function":        "AudioPacketizer.PacketizeAndSend",
		"sequence_number": ap.sequenceNumber,
		"timestamp":       ap.timestamp,
		"ssrc":            ap.ssrc,
	}).Debug("Created RTP packet")

	rtpData, err := packet.Marshal()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "AudioPacketizer.PacketizeAndSend",
			"error":    err.Error(),
		}).Error("Failed to marshal RTP packet")
		return nil, fmt.Errorf("failed to marshal RTP packet: %w", err)
	}

	return rtpData, nil
}

// sendToxPacket sends RTP data over Tox transport.
func (ap *AudioPacketizer) sendToxPacket(rtpData []byte) error {
	toxPacket := &transport.Packet{
		PacketType: transport.PacketAVAudioFrame,
		Data:       rtpData,
	}

	logrus.WithFields(logrus.Fields{
		"function":    "AudioPacketizer.PacketizeAndSend",
		"rtp_size":    len(rtpData),
		"packet_type": toxPacket.PacketType,
	}).Debug("Created Tox transport packet")

	if err := ap.transport.Send(toxPacket, ap.remoteAddr); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "AudioPacketizer.PacketizeAndSend",
			"error":    err.Error(),
		}).Error("Failed to send audio RTP packet")
		return fmt.Errorf("failed to send audio RTP packet: %w", err)
	}

	return nil
}

// updateRTPCounters increments sequence number and timestamp.
func (ap *AudioPacketizer) updateRTPCounters(sampleCount uint32) {
	ap.sequenceNumber++
	ap.timestamp += sampleCount
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
	logrus.WithFields(logrus.Fields{
		"function": "NewAudioDepacketizer",
	}).Info("Creating new audio depacketizer")

	depacketizer := &AudioDepacketizer{
		jitterBuffer: NewJitterBuffer(50 * time.Millisecond), // 50ms jitter buffer
	}

	logrus.WithFields(logrus.Fields{
		"function":           "NewAudioDepacketizer",
		"jitter_buffer_size": "50ms",
	}).Info("Audio depacketizer created successfully")

	return depacketizer
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
	logrus.WithFields(logrus.Fields{
		"function":  "AudioDepacketizer.ProcessPacket",
		"data_size": len(rtpData),
	}).Debug("Processing incoming RTP packet")

	if len(rtpData) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "AudioDepacketizer.ProcessPacket",
			"error":    "RTP data cannot be empty",
		}).Error("Invalid RTP data")
		return nil, 0, fmt.Errorf("RTP data cannot be empty")
	}

	packet, err := ad.parseRTPPacket(rtpData)
	if err != nil {
		return nil, 0, err
	}

	ad.mu.Lock()
	defer ad.mu.Unlock()

	if err := ad.validateAndUpdateSSRC(packet); err != nil {
		return nil, 0, err
	}

	ad.checkSequenceGap(packet)
	ad.jitterBuffer.Add(packet.Timestamp, packet.Payload)

	logrus.WithFields(logrus.Fields{
		"function":     "AudioDepacketizer.ProcessPacket",
		"timestamp":    packet.Timestamp,
		"payload_size": len(packet.Payload),
	}).Debug("RTP packet processed successfully")

	return packet.Payload, packet.Timestamp, nil
}

// parseRTPPacket unmarshals RTP data into a packet structure.
func (ad *AudioDepacketizer) parseRTPPacket(rtpData []byte) (*rtp.Packet, error) {
	packet := &rtp.Packet{}
	if err := packet.Unmarshal(rtpData); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "AudioDepacketizer.ProcessPacket",
			"error":    err.Error(),
		}).Error("Failed to unmarshal RTP packet")
		return nil, fmt.Errorf("failed to unmarshal RTP packet: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"function":     "AudioDepacketizer.ProcessPacket",
		"ssrc":         packet.SSRC,
		"sequence":     packet.SequenceNumber,
		"timestamp":    packet.Timestamp,
		"payload_size": len(packet.Payload),
	}).Debug("Parsed RTP packet")

	return packet, nil
}

// validateAndUpdateSSRC validates the packet SSRC and updates expected SSRC if needed.
func (ad *AudioDepacketizer) validateAndUpdateSSRC(packet *rtp.Packet) error {
	if !ad.hasSSRC {
		ad.expectedSSRC = packet.SSRC
		ad.hasSSRC = true
		logrus.WithFields(logrus.Fields{
			"function": "AudioDepacketizer.ProcessPacket",
			"ssrc":     packet.SSRC,
		}).Info("Accepted new SSRC for stream")
		return nil
	}

	if packet.SSRC != ad.expectedSSRC {
		logrus.WithFields(logrus.Fields{
			"function":      "AudioDepacketizer.ProcessPacket",
			"expected_ssrc": ad.expectedSSRC,
			"received_ssrc": packet.SSRC,
		}).Warn("Unexpected SSRC in RTP packet")
		return fmt.Errorf("unexpected SSRC: expected %d, got %d", ad.expectedSSRC, packet.SSRC)
	}

	return nil
}

// checkSequenceGap detects and logs gaps in the RTP sequence numbers.
func (ad *AudioDepacketizer) checkSequenceGap(packet *rtp.Packet) {
	if ad.hasLastSeq {
		expectedSeq := ad.lastSeq + 1
		if packet.SequenceNumber != expectedSeq {
			logrus.WithFields(logrus.Fields{
				"function":          "AudioDepacketizer.ProcessPacket",
				"expected_sequence": expectedSeq,
				"received_sequence": packet.SequenceNumber,
			}).Warn("Sequence gap detected in RTP stream")
		}
	}
	ad.lastSeq = packet.SequenceNumber
	ad.hasLastSeq = true
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
	logrus.WithFields(logrus.Fields{
		"function": "AudioDepacketizer.GetBufferedAudio",
	}).Debug("Retrieving buffered audio data")

	data, available := ad.jitterBuffer.Get()

	logrus.WithFields(logrus.Fields{
		"function":       "AudioDepacketizer.GetBufferedAudio",
		"data_available": available,
		"data_size":      len(data),
	}).Debug("Retrieved buffered audio data")

	return data, available
}

// DefaultMaxBufferCapacity is the default maximum number of packets in the jitter buffer.
// This prevents unbounded memory growth from network issues or attacks.
const DefaultMaxBufferCapacity = 100

// jitterBufferEntry represents a single packet stored in the jitter buffer.
// Packets are stored sorted by RTP timestamp for proper playback ordering.
// The timestamp field holds the RTP timestamp from the packet header,
// while data contains the actual audio payload bytes.
type jitterBufferEntry struct {
	timestamp uint32
	data      []byte
}

// JitterBuffer provides basic jitter buffering for audio packets.
//
// This implementation buffers packets for a fixed duration to smooth out
// network jitter and provides consistent audio playback. Packets are
// returned in timestamp order for proper audio sequencing.
//
// The buffer has a configurable maximum capacity (default 100 packets)
// to prevent unbounded memory growth. When capacity is exceeded, the
// oldest packets are evicted.
type JitterBuffer struct {
	mu           sync.RWMutex
	bufferTime   time.Duration
	packets      []jitterBufferEntry // sorted by timestamp
	maxCapacity  int                 // maximum number of packets to buffer
	lastDequeue  time.Time
	timeProvider TimeProvider
}

// NewJitterBuffer creates a new jitter buffer.
//
// Parameters:
//   - bufferTime: Duration to buffer packets
//
// Returns:
//   - *JitterBuffer: New jitter buffer instance
func NewJitterBuffer(bufferTime time.Duration) *JitterBuffer {
	return NewJitterBufferWithTimeProvider(bufferTime, DefaultTimeProvider{})
}

// NewJitterBufferWithTimeProvider creates a new jitter buffer with an injectable time provider.
//
// This constructor allows for deterministic testing by injecting a custom TimeProvider.
//
// Parameters:
//   - bufferTime: Duration to buffer packets
//   - timeProvider: Provider for time operations
//
// Returns:
//   - *JitterBuffer: New jitter buffer instance
func NewJitterBufferWithTimeProvider(bufferTime time.Duration, timeProvider TimeProvider) *JitterBuffer {
	return NewJitterBufferWithOptions(bufferTime, DefaultMaxBufferCapacity, timeProvider)
}

// NewJitterBufferWithOptions creates a new jitter buffer with full configuration.
//
// Parameters:
//   - bufferTime: Duration to buffer packets
//   - maxCapacity: Maximum number of packets to buffer (0 uses default)
//   - timeProvider: Provider for time operations (nil uses default)
//
// Returns:
//   - *JitterBuffer: New jitter buffer instance
func NewJitterBufferWithOptions(bufferTime time.Duration, maxCapacity int, timeProvider TimeProvider) *JitterBuffer {
	logrus.WithFields(logrus.Fields{
		"function":     "NewJitterBuffer",
		"buffer_time":  bufferTime.String(),
		"max_capacity": maxCapacity,
	}).Info("Creating new jitter buffer")

	if timeProvider == nil {
		timeProvider = DefaultTimeProvider{}
	}
	if maxCapacity <= 0 {
		maxCapacity = DefaultMaxBufferCapacity
	}

	buffer := &JitterBuffer{
		bufferTime:   bufferTime,
		packets:      make([]jitterBufferEntry, 0, maxCapacity),
		maxCapacity:  maxCapacity,
		lastDequeue:  timeProvider.Now(),
		timeProvider: timeProvider,
	}

	logrus.WithFields(logrus.Fields{
		"function":     "NewJitterBuffer",
		"buffer_time":  bufferTime.String(),
		"max_capacity": maxCapacity,
	}).Info("Jitter buffer created successfully")

	return buffer
}

// SetTimeProvider sets the time provider for the jitter buffer.
// This allows for deterministic testing by injecting a mock time provider.
func (jb *JitterBuffer) SetTimeProvider(tp TimeProvider) {
	jb.mu.Lock()
	defer jb.mu.Unlock()
	if tp == nil {
		tp = DefaultTimeProvider{}
	}
	jb.timeProvider = tp
}

// SetMaxCapacity sets the maximum number of packets in the jitter buffer.
// When capacity is exceeded, oldest packets are evicted.
// A value of 0 or negative uses the default capacity.
func (jb *JitterBuffer) SetMaxCapacity(capacity int) {
	jb.mu.Lock()
	defer jb.mu.Unlock()
	if capacity <= 0 {
		capacity = DefaultMaxBufferCapacity
	}
	jb.maxCapacity = capacity
	// Evict excess packets if necessary
	if len(jb.packets) > jb.maxCapacity {
		evicted := len(jb.packets) - jb.maxCapacity
		jb.packets = jb.packets[evicted:]
		logrus.WithFields(logrus.Fields{
			"function":      "JitterBuffer.SetMaxCapacity",
			"evicted_count": evicted,
			"new_capacity":  capacity,
			"current_size":  len(jb.packets),
		}).Debug("Evicted excess packets after capacity change")
	}
}

// Len returns the current number of packets in the buffer.
func (jb *JitterBuffer) Len() int {
	jb.mu.RLock()
	defer jb.mu.RUnlock()
	return len(jb.packets)
}

// Add adds a packet to the jitter buffer.
//
// Packets are inserted in timestamp order. If the buffer is at capacity,
// the oldest packet is evicted to make room.
//
// Parameters:
//   - timestamp: RTP timestamp
//   - data: Audio data
func (jb *JitterBuffer) Add(timestamp uint32, data []byte) {
	logrus.WithFields(logrus.Fields{
		"function":  "JitterBuffer.Add",
		"timestamp": timestamp,
		"data_size": len(data),
	}).Debug("Adding packet to jitter buffer")

	jb.mu.Lock()
	defer jb.mu.Unlock()

	entry := jitterBufferEntry{timestamp: timestamp, data: data}

	// Find insertion point using binary search for sorted order
	insertIdx := jb.findInsertIndex(timestamp)

	// If at capacity, evict oldest packet first
	if len(jb.packets) >= jb.maxCapacity {
		evicted := jb.packets[0]
		jb.packets = jb.packets[1:]
		// Adjust insert index after eviction
		if insertIdx > 0 {
			insertIdx--
		}
		logrus.WithFields(logrus.Fields{
			"function":          "JitterBuffer.Add",
			"evicted_timestamp": evicted.timestamp,
			"new_timestamp":     timestamp,
		}).Debug("Evicted oldest packet due to capacity limit")
	}

	// Insert at sorted position
	jb.packets = append(jb.packets, jitterBufferEntry{})
	copy(jb.packets[insertIdx+1:], jb.packets[insertIdx:])
	jb.packets[insertIdx] = entry

	logrus.WithFields(logrus.Fields{
		"function":    "JitterBuffer.Add",
		"timestamp":   timestamp,
		"buffer_size": len(jb.packets),
	}).Debug("Packet added to jitter buffer")
}

// findInsertIndex returns the index where a packet with the given timestamp
// should be inserted to maintain sorted order.
func (jb *JitterBuffer) findInsertIndex(timestamp uint32) int {
	// Binary search for insertion point
	left, right := 0, len(jb.packets)
	for left < right {
		mid := (left + right) / 2
		if jb.packets[mid].timestamp < timestamp {
			left = mid + 1
		} else {
			right = mid
		}
	}
	return left
}

// Get retrieves the next packet from the jitter buffer.
//
// This implements a simple time-based release mechanism.
// Packets are returned in timestamp order (oldest first) after the
// buffer time has elapsed since the last dequeue.
//
// Returns:
//   - []byte: Audio data (nil if no data ready)
//   - bool: Whether data was available
func (jb *JitterBuffer) Get() ([]byte, bool) {
	logrus.WithFields(logrus.Fields{
		"function": "JitterBuffer.Get",
	}).Debug("Retrieving packet from jitter buffer")

	jb.mu.Lock()
	defer jb.mu.Unlock()

	// Simple time-based release: wait for buffer time to pass since last dequeue
	timeSinceLastDequeue := jb.timeProvider.Now().Sub(jb.lastDequeue)
	if timeSinceLastDequeue < jb.bufferTime {
		logrus.WithFields(logrus.Fields{
			"function":        "JitterBuffer.Get",
			"time_since_last": timeSinceLastDequeue.String(),
			"buffer_time":     jb.bufferTime.String(),
		}).Debug("Buffer time not elapsed, no packet ready")
		return nil, false
	}

	// Return oldest packet (first in sorted slice) for proper ordering
	if len(jb.packets) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "JitterBuffer.Get",
		}).Debug("No packets available in jitter buffer")
		return nil, false
	}

	// Get oldest packet (lowest timestamp, first in sorted slice)
	entry := jb.packets[0]
	jb.packets = jb.packets[1:]
	jb.lastDequeue = jb.timeProvider.Now()

	logrus.WithFields(logrus.Fields{
		"function":          "JitterBuffer.Get",
		"timestamp":         entry.timestamp,
		"data_size":         len(entry.data),
		"remaining_packets": len(jb.packets),
	}).Debug("Retrieved packet from jitter buffer")

	return entry.data, true
}

// Reset clears the jitter buffer.
func (jb *JitterBuffer) Reset() {
	logrus.WithFields(logrus.Fields{
		"function": "JitterBuffer.Reset",
	}).Info("Resetting jitter buffer")

	jb.mu.Lock()
	defer jb.mu.Unlock()

	packetCount := len(jb.packets)
	jb.packets = make([]jitterBufferEntry, 0, jb.maxCapacity)
	jb.lastDequeue = jb.timeProvider.Now()

	logrus.WithFields(logrus.Fields{
		"function":        "JitterBuffer.Reset",
		"cleared_packets": packetCount,
	}).Info("Jitter buffer reset successfully")
}
