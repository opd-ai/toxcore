package toxcore

import (
	"sync"
	"testing"

	"github.com/opd-ai/toxcore/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToxAVTransportAdapter_AudioVideoFramePackets tests that audio and video frame
// packets (0x33 and 0x34) are properly handled by the transport adapter.
// This test verifies the fix for AUDIT.md issue:
// "ToxAV Transport Adapter Does Not Handle Audio/Video Frame Packets"
func TestToxAVTransportAdapter_AudioVideoFramePackets(t *testing.T) {
	// Create a mock UDP transport
	mockTransport := newMockUDPTransport()

	// Create the ToxAV transport adapter
	adapter := newToxAVTransportAdapter(mockTransport)
	require.NotNil(t, adapter)

	// Test address (192.168.1.100:5555)
	testAddr := []byte{192, 168, 1, 100, 21, 179} // Port 5555 = 0x15B3
	testData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	tests := []struct {
		name              string
		packetType        byte
		expectedTransport transport.PacketType
		description       string
	}{
		{
			name:              "CallRequest",
			packetType:        0x30,
			expectedTransport: transport.PacketAVCallRequest,
			description:       "Call request packet",
		},
		{
			name:              "CallResponse",
			packetType:        0x31,
			expectedTransport: transport.PacketAVCallResponse,
			description:       "Call response packet",
		},
		{
			name:              "CallControl",
			packetType:        0x32,
			expectedTransport: transport.PacketAVCallControl,
			description:       "Call control packet",
		},
		{
			name:              "AudioFrame",
			packetType:        0x33,
			expectedTransport: transport.PacketAVAudioFrame,
			description:       "Audio frame packet (new)",
		},
		{
			name:              "VideoFrame",
			packetType:        0x34,
			expectedTransport: transport.PacketAVVideoFrame,
			description:       "Video frame packet (new)",
		},
		{
			name:              "BitrateControl",
			packetType:        0x35,
			expectedTransport: transport.PacketAVBitrateControl,
			description:       "Bitrate control packet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous packets
			mockTransport.sentPackets = nil

			// Send packet via adapter
			err := adapter.Send(tt.packetType, testData, testAddr)
			require.NoError(t, err, "Send should succeed for packet type 0x%02x", tt.packetType)

			// Verify packet was sent
			require.Len(t, mockTransport.sentPackets, 1, "Should have sent exactly one packet")

			sentPacket := mockTransport.sentPackets[0]
			assert.Equal(t, tt.expectedTransport, sentPacket.packet.PacketType,
				"Should convert 0x%02x to transport.%v", tt.packetType, tt.expectedTransport)
			assert.Equal(t, testData, sentPacket.packet.Data, "Packet data should match")
			assert.Equal(t, "192.168.1.100:5555", sentPacket.addr.String(), "Destination address should match")
		})
	}
}

// TestToxAVTransportAdapter_RegisterHandlers tests that all AV packet handlers
// including audio and video frames can be registered correctly.
func TestToxAVTransportAdapter_RegisterHandlers(t *testing.T) {
	mockTransport := newMockUDPTransport()
	adapter := newToxAVTransportAdapter(mockTransport)
	require.NotNil(t, adapter)

	handlerCalls := make(map[byte]int)
	var mu sync.Mutex

	// Create a handler that tracks which packet type it was called for
	createHandler := func(packetType byte) func([]byte, []byte) error {
		return func(data, addr []byte) error {
			mu.Lock()
			handlerCalls[packetType]++
			mu.Unlock()
			return nil
		}
	}

	// Register handlers for all packet types
	packetTypes := []byte{0x30, 0x31, 0x32, 0x33, 0x34, 0x35}
	for _, pt := range packetTypes {
		adapter.RegisterHandler(pt, createHandler(pt))
	}

	// Verify all handlers were registered in the mock transport
	assert.Len(t, mockTransport.handlers, 6, "Should have registered 6 packet handlers")

	// Verify specific packet types are registered
	expectedTransportTypes := []transport.PacketType{
		transport.PacketAVCallRequest,
		transport.PacketAVCallResponse,
		transport.PacketAVCallControl,
		transport.PacketAVAudioFrame,
		transport.PacketAVVideoFrame,
		transport.PacketAVBitrateControl,
	}

	for _, pt := range expectedTransportTypes {
		_, exists := mockTransport.handlers[pt]
		assert.True(t, exists, "Handler for %v should be registered", pt)
	}
}

// TestToxAVTransportAdapter_UnknownPacketType tests that unknown packet types
// are properly rejected.
func TestToxAVTransportAdapter_UnknownPacketType(t *testing.T) {
	mockTransport := newMockUDPTransport()
	adapter := newToxAVTransportAdapter(mockTransport)
	require.NotNil(t, adapter)

	testAddr := []byte{192, 168, 1, 100, 0, 80}
	testData := []byte{0x01, 0x02, 0x03}

	// Test unknown packet types
	unknownTypes := []byte{0x00, 0x29, 0x36, 0xFF}
	for _, pt := range unknownTypes {
		err := adapter.Send(pt, testData, testAddr)
		assert.Error(t, err, "Unknown packet type 0x%02x should return error", pt)
		assert.Contains(t, err.Error(), "unknown AV packet type", "Error should mention unknown packet type")
	}
}
