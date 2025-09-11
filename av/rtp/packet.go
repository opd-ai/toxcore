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
	"github.com/sirupsen/logrus"
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
	logrus.WithFields(logrus.Fields{
		"function": "NewAudioPacketizer",
		"clock_rate": clockRate,
		"remote_addr": remoteAddr.String(),
	}).Info("Creating new audio packetizer")

	if clockRate == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "NewAudioPacketizer",
			"error": "clock rate cannot be zero",
		}).Error("Invalid clock rate")
		return nil, fmt.Errorf("clock rate cannot be zero")
	}
	if transport == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewAudioPacketizer",
			"error": "transport cannot be nil",
		}).Error("Invalid transport")
		return nil, fmt.Errorf("transport cannot be nil")
	}
	if remoteAddr == nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewAudioPacketizer",
			"error": "remote address cannot be nil",
		}).Error("Invalid remote address")
		return nil, fmt.Errorf("remote address cannot be nil")
	}

	// Generate random SSRC for this stream
	ssrcBytes := make([]byte, 4)
	if _, err := rand.Read(ssrcBytes); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "NewAudioPacketizer",
			"error": err.Error(),
		}).Error("Failed to generate SSRC")
		return nil, fmt.Errorf("failed to generate SSRC: %w", err)
	}
	ssrc := binary.BigEndian.Uint32(ssrcBytes)

	packetizer := &AudioPacketizer{
		ssrc:           ssrc,
		sequenceNumber: 0,
		timestamp:      0,
		clockRate:      clockRate,
		transport:      transport,
		remoteAddr:     remoteAddr,
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewAudioPacketizer",
		"ssrc": ssrc,
		"clock_rate": clockRate,
	}).Info("Audio packetizer created successfully")

	return packetizer, nil
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
		"function": "AudioPacketizer.PacketizeAndSend",
		"data_size": len(audioData),
		"sample_count": sampleCount,
	}).Debug("Starting audio packetization")

	if len(audioData) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "AudioPacketizer.PacketizeAndSend",
			"error": "audio data cannot be empty",
		}).Error("Invalid audio data")
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

	logrus.WithFields(logrus.Fields{
		"function": "AudioPacketizer.PacketizeAndSend",
		"sequence_number": ap.sequenceNumber,
		"timestamp": ap.timestamp,
		"ssrc": ap.ssrc,
	}).Debug("Created RTP packet")

	// Serialize RTP packet
	rtpData, err := packet.Marshal()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "AudioPacketizer.PacketizeAndSend",
			"error": err.Error(),
		}).Error("Failed to marshal RTP packet")
		return fmt.Errorf("failed to marshal RTP packet: %w", err)
	}

	// Create Tox transport packet
	toxPacket := &transport.Packet{
		PacketType: transport.PacketAVAudioFrame,
		Data:       rtpData,
	}

	logrus.WithFields(logrus.Fields{
		"function": "AudioPacketizer.PacketizeAndSend",
		"rtp_size": len(rtpData),
		"packet_type": toxPacket.PacketType,
	}).Debug("Created Tox transport packet")

	// Send over Tox transport
	if err := ap.transport.Send(toxPacket, ap.remoteAddr); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "AudioPacketizer.PacketizeAndSend",
			"error": err.Error(),
		}).Error("Failed to send audio RTP packet")
		return fmt.Errorf("failed to send audio RTP packet: %w", err)
	}

	// Update sequence number and timestamp
	ap.sequenceNumber++
	ap.timestamp += sampleCount

	logrus.WithFields(logrus.Fields{
		"function": "AudioPacketizer.PacketizeAndSend",
		"new_sequence": ap.sequenceNumber,
		"new_timestamp": ap.timestamp,
	}).Debug("Audio packet sent successfully")

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
	logrus.WithFields(logrus.Fields{
		"function": "NewAudioDepacketizer",
	}).Info("Creating new audio depacketizer")

	depacketizer := &AudioDepacketizer{
		jitterBuffer: NewJitterBuffer(50 * time.Millisecond), // 50ms jitter buffer
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewAudioDepacketizer",
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
		"function": "AudioDepacketizer.ProcessPacket",
		"data_size": len(rtpData),
	}).Debug("Processing incoming RTP packet")

	if len(rtpData) == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "AudioDepacketizer.ProcessPacket",
			"error": "RTP data cannot be empty",
		}).Error("Invalid RTP data")
		return nil, 0, fmt.Errorf("RTP data cannot be empty")
	}

	// Parse RTP packet
	packet := &rtp.Packet{}
	if err := packet.Unmarshal(rtpData); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "AudioDepacketizer.ProcessPacket",
			"error": err.Error(),
		}).Error("Failed to unmarshal RTP packet")
		return nil, 0, fmt.Errorf("failed to unmarshal RTP packet: %w", err)
	}

	ad.mu.Lock()
	defer ad.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function": "AudioDepacketizer.ProcessPacket",
		"ssrc": packet.SSRC,
		"sequence": packet.SequenceNumber,
		"timestamp": packet.Timestamp,
		"payload_size": len(packet.Payload),
	}).Debug("Parsed RTP packet")

	// Validate SSRC (accept first seen SSRC)
	if !ad.hasSSRC {
		ad.expectedSSRC = packet.SSRC
		ad.hasSSRC = true
		logrus.WithFields(logrus.Fields{
			"function": "AudioDepacketizer.ProcessPacket",
			"ssrc": packet.SSRC,
		}).Info("Accepted new SSRC for stream")
	} else if packet.SSRC != ad.expectedSSRC {
		logrus.WithFields(logrus.Fields{
			"function": "AudioDepacketizer.ProcessPacket",
			"expected_ssrc": ad.expectedSSRC,
			"received_ssrc": packet.SSRC,
		}).Warn("Unexpected SSRC in RTP packet")
		return nil, 0, fmt.Errorf("unexpected SSRC: expected %d, got %d", ad.expectedSSRC, packet.SSRC)
	}

	// Basic sequence number validation
	if ad.hasLastSeq {
		expectedSeq := ad.lastSeq + 1
		if packet.SequenceNumber != expectedSeq {
			// Simple gap detection - in production, implement proper jitter buffer
			logrus.WithFields(logrus.Fields{
				"function": "AudioDepacketizer.ProcessPacket",
				"expected_sequence": expectedSeq,
				"received_sequence": packet.SequenceNumber,
			}).Warn("Sequence gap detected in RTP stream")
			fmt.Printf("Sequence gap detected: expected %d, got %d\n", expectedSeq, packet.SequenceNumber)
		}
	}
	ad.lastSeq = packet.SequenceNumber
	ad.hasLastSeq = true

	// Add to jitter buffer for smooth playback
	ad.jitterBuffer.Add(packet.Timestamp, packet.Payload)

	logrus.WithFields(logrus.Fields{
		"function": "AudioDepacketizer.ProcessPacket",
		"timestamp": packet.Timestamp,
		"payload_size": len(packet.Payload),
	}).Debug("RTP packet processed successfully")

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
	logrus.WithFields(logrus.Fields{
		"function": "AudioDepacketizer.GetBufferedAudio",
	}).Debug("Retrieving buffered audio data")

	data, available := ad.jitterBuffer.Get()
	
	logrus.WithFields(logrus.Fields{
		"function": "AudioDepacketizer.GetBufferedAudio",
		"data_available": available,
		"data_size": len(data),
	}).Debug("Retrieved buffered audio data")

	return data, available
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
	logrus.WithFields(logrus.Fields{
		"function": "NewJitterBuffer",
		"buffer_time": bufferTime.String(),
	}).Info("Creating new jitter buffer")

	buffer := &JitterBuffer{
		bufferTime:  bufferTime,
		packets:     make(map[uint32][]byte),
		lastDequeue: time.Now(),
	}

	logrus.WithFields(logrus.Fields{
		"function": "NewJitterBuffer",
		"buffer_time": bufferTime.String(),
	}).Info("Jitter buffer created successfully")

	return buffer
}

// Add adds a packet to the jitter buffer.
//
// Parameters:
//   - timestamp: RTP timestamp
//   - data: Audio data
func (jb *JitterBuffer) Add(timestamp uint32, data []byte) {
	logrus.WithFields(logrus.Fields{
		"function": "JitterBuffer.Add",
		"timestamp": timestamp,
		"data_size": len(data),
	}).Debug("Adding packet to jitter buffer")

	jb.mu.Lock()
	defer jb.mu.Unlock()

	// Store packet with timestamp as key
	jb.packets[timestamp] = data

	logrus.WithFields(logrus.Fields{
		"function": "JitterBuffer.Add",
		"timestamp": timestamp,
		"buffer_size": len(jb.packets),
	}).Debug("Packet added to jitter buffer")
}

// Get retrieves the next packet from the jitter buffer.
//
// This implements a simple time-based release mechanism.
// Packets are only released after the buffer time has elapsed
// since the buffer was created or last reset.
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
	timeSinceLastDequeue := time.Since(jb.lastDequeue)
	if timeSinceLastDequeue < jb.bufferTime {
		logrus.WithFields(logrus.Fields{
			"function": "JitterBuffer.Get",
			"time_since_last": timeSinceLastDequeue.String(),
			"buffer_time": jb.bufferTime.String(),
		}).Debug("Buffer time not elapsed, no packet ready")
		return nil, false
	}

	// Get any available packet (simplified - should order by timestamp)
	for timestamp, data := range jb.packets {
		delete(jb.packets, timestamp)
		jb.lastDequeue = time.Now()
		
		logrus.WithFields(logrus.Fields{
			"function": "JitterBuffer.Get",
			"timestamp": timestamp,
			"data_size": len(data),
			"remaining_packets": len(jb.packets),
		}).Debug("Retrieved packet from jitter buffer")
		
		return data, true
	}

	logrus.WithFields(logrus.Fields{
		"function": "JitterBuffer.Get",
	}).Debug("No packets available in jitter buffer")

	return nil, false
}

// Reset clears the jitter buffer.
func (jb *JitterBuffer) Reset() {
	logrus.WithFields(logrus.Fields{
		"function": "JitterBuffer.Reset",
	}).Info("Resetting jitter buffer")

	jb.mu.Lock()
	defer jb.mu.Unlock()

	packetCount := len(jb.packets)
	jb.packets = make(map[uint32][]byte)
	jb.lastDequeue = time.Now()

	logrus.WithFields(logrus.Fields{
		"function": "JitterBuffer.Reset",
		"cleared_packets": packetCount,
	}).Info("Jitter buffer reset successfully")
}
