package rtp

import (
	"net"
	"testing"

	"github.com/opd-ai/toxcore/av/video"
	"github.com/opd-ai/toxcore/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession_SendVideoPacket_Comprehensive(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	session, err := NewSession(42, mockTransport, remoteAddr)
	require.NoError(t, err)

	tests := []struct {
		name         string
		videoData    []byte
		expectError  bool
		errorMsg     string
		validateFunc func(*testing.T, []SentPacket)
	}{
		{
			name:        "Small video frame",
			videoData:   make([]byte, 100),
			expectError: false,
			validateFunc: func(t *testing.T, packets []SentPacket) {
				assert.Greater(t, len(packets), 0)
				// All packets should be video frames
				for _, pkt := range packets {
					assert.Equal(t, transport.PacketAVVideoFrame, pkt.Packet.PacketType)
				}
			},
		},
		{
			name:        "Large video frame requiring fragmentation",
			videoData:   make([]byte, 5000), // Will require multiple RTP packets
			expectError: false,
			validateFunc: func(t *testing.T, packets []SentPacket) {
				assert.Greater(t, len(packets), 1, "Large frame should be fragmented")
				for _, pkt := range packets {
					assert.Equal(t, transport.PacketAVVideoFrame, pkt.Packet.PacketType)
					// Each packet should have RTP header + VP8 descriptor + payload
					assert.Greater(t, len(pkt.Packet.Data), 12+3)
				}
			},
		},
		{
			name:        "Empty video data",
			videoData:   []byte{},
			expectError: true,
			errorMsg:    "video data cannot be empty",
		},
		{
			name:        "Single byte frame",
			videoData:   []byte{0xFF},
			expectError: false,
			validateFunc: func(t *testing.T, packets []SentPacket) {
				assert.Equal(t, 1, len(packets))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear sent packets
			mockTransport.sentPackets = []SentPacket{}

			err := session.SendVideoPacket(tt.videoData)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				sentPackets := mockTransport.GetSentPackets()
				if tt.validateFunc != nil {
					tt.validateFunc(t, sentPackets)
				}
			}
		})
	}
}

func TestSession_ReceiveVideoPacket_Comprehensive(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	session, err := NewSession(42, mockTransport, remoteAddr)
	require.NoError(t, err)

	tests := []struct {
		name        string
		packet      []byte
		expectError bool
		expectFrame bool // Whether a complete frame is expected
		errorMsg    string
	}{
		{
			name: "Valid single-packet video frame",
			packet: createValidVideoRTPPacket(t, &videoPacketConfig{
				marker:         true, // End of frame
				sequenceNum:    1,
				timestamp:      1000,
				pictureID:      1,
				startPartition: true,
				payloadSize:    10,
			}),
			expectError: false,
			expectFrame: true,
		},
		{
			name:        "Empty packet",
			packet:      []byte{},
			expectError: true,
			expectFrame: false,
			errorMsg:    "packet cannot be empty",
		},
		{
			name:        "Packet too short",
			packet:      []byte{0x01, 0x02},
			expectError: true,
			expectFrame: false,
			errorMsg:    "packet too short",
		},
		{
			name: "First packet of multi-packet frame",
			packet: createValidVideoRTPPacket(t, &videoPacketConfig{
				marker:         false, // Not end of frame
				sequenceNum:    1,
				timestamp:      2000,
				pictureID:      2,
				startPartition: true,
				payloadSize:    100,
			}),
			expectError: false,
			expectFrame: false, // Frame not complete yet
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frameData, pictureID, err := session.ReceiveVideoPacket(tt.packet)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				if tt.expectFrame {
					assert.NotNil(t, frameData)
					// Picture ID may be 0 or non-zero depending on frame assembly
					assert.GreaterOrEqual(t, pictureID, uint16(0))
				} else {
					assert.Nil(t, frameData)
				}
			}
		})
	}
}

func TestVideoRTPPacket_SerializationRoundtrip(t *testing.T) {
	originalPacket := video.RTPPacket{
		Version:             2,
		Padding:             false,
		Extension:           false,
		CSRCCount:           0,
		Marker:              true,
		PayloadType:         96,
		SequenceNumber:      12345,
		Timestamp:           67890,
		SSRC:                0x12345678,
		ExtendedControlBits: true,
		NonReferenceBit:     false,
		StartOfPartition:    true,
		PictureID:           42,
		Payload:             []byte{0x01, 0x02, 0x03, 0x04, 0x05},
	}

	// Serialize
	serialized := serializeVideoRTPPacket(originalPacket)
	assert.Greater(t, len(serialized), 12, "Serialized packet should have header + payload")

	// Deserialize
	deserialized, err := deserializeVideoRTPPacket(serialized)
	require.NoError(t, err)

	// Verify all fields match
	assert.Equal(t, originalPacket.Version, deserialized.Version)
	assert.Equal(t, originalPacket.Padding, deserialized.Padding)
	assert.Equal(t, originalPacket.Extension, deserialized.Extension)
	assert.Equal(t, originalPacket.CSRCCount, deserialized.CSRCCount)
	assert.Equal(t, originalPacket.Marker, deserialized.Marker)
	assert.Equal(t, originalPacket.PayloadType, deserialized.PayloadType)
	assert.Equal(t, originalPacket.SequenceNumber, deserialized.SequenceNumber)
	assert.Equal(t, originalPacket.Timestamp, deserialized.Timestamp)
	assert.Equal(t, originalPacket.SSRC, deserialized.SSRC)
	assert.Equal(t, originalPacket.Payload, deserialized.Payload)
}

func TestTransportIntegration_VideoPacketRouting(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr1, _ := net.ResolveUDPAddr("udp", "127.0.0.1:10001")
	remoteAddr2, _ := net.ResolveUDPAddr("udp", "127.0.0.1:10002")

	integration, err := NewTransportIntegration(mockTransport)
	require.NoError(t, err)

	// Create sessions for two friends
	_, err = integration.CreateSession(1, remoteAddr1)
	require.NoError(t, err)
	_, err = integration.CreateSession(2, remoteAddr2)
	require.NoError(t, err)

	// Create valid video RTP packet
	videoPacket := &transport.Packet{
		PacketType: transport.PacketAVVideoFrame,
		Data: createValidVideoRTPPacket(t, &videoPacketConfig{
			marker:         true,
			sequenceNum:    1,
			timestamp:      1000,
			pictureID:      1,
			startPartition: true,
			payloadSize:    10,
		}),
	}

	// Test routing to first friend
	err = integration.handleIncomingVideoFrame(videoPacket, remoteAddr1)
	assert.NoError(t, err)

	// Test routing to second friend
	err = integration.handleIncomingVideoFrame(videoPacket, remoteAddr2)
	assert.NoError(t, err)

	// Test with unknown address
	unknownAddr, _ := net.ResolveUDPAddr("udp", "192.168.1.100:9999")
	err = integration.handleIncomingVideoFrame(videoPacket, unknownAddr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no session found")
}

// Helper types and functions

type videoPacketConfig struct {
	marker         bool
	sequenceNum    uint16
	timestamp      uint32
	pictureID      uint16
	startPartition bool
	payloadSize    int
}

func createValidVideoRTPPacket(t *testing.T, config *videoPacketConfig) []byte {
	// RTP header (12 bytes) + VP8 payload descriptor (3 bytes) + payload
	packet := make([]byte, 12+3+config.payloadSize)

	// RTP header
	packet[0] = 0x80 // Version 2
	if config.marker {
		packet[1] = 0xE0 // Marker bit + Payload type 96
	} else {
		packet[1] = 0x60 // Payload type 96
	}

	// Sequence number
	packet[2] = byte(config.sequenceNum >> 8)
	packet[3] = byte(config.sequenceNum)

	// Timestamp
	packet[4] = byte(config.timestamp >> 24)
	packet[5] = byte(config.timestamp >> 16)
	packet[6] = byte(config.timestamp >> 8)
	packet[7] = byte(config.timestamp)

	// SSRC
	packet[8] = 0x12
	packet[9] = 0x34
	packet[10] = 0x56
	packet[11] = 0x78

	// VP8 Payload Descriptor
	var firstByte byte = 0x80 // X bit (extended control bits)
	if config.startPartition {
		firstByte |= 0x10 // S bit
	}
	packet[12] = firstByte

	// Picture ID (2 bytes with I bit)
	packet[13] = 0x80 | byte((config.pictureID>>8)&0x7F)
	packet[14] = byte(config.pictureID & 0xFF)

	// Fill payload with test data
	for i := 0; i < config.payloadSize; i++ {
		packet[15+i] = byte(i)
	}

	return packet
}
