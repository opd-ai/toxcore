package video

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRTPDebugRoundTrip(t *testing.T) {
	// Create packetizer and depacketizer
	packetizer := NewRTPPacketizer(12345)
	depacketizer := NewRTPDepacketizer()

	// Large test data to force fragmentation
	originalData := make([]byte, 2500)
	for i := range originalData {
		originalData[i] = byte(i % 256)
	}

	fmt.Printf("Original data length: %d\n", len(originalData))
	fmt.Printf("Original data[0:10]: %v\n", originalData[:10])

	// Packetize
	packets, err := packetizer.PacketizeFrame(originalData, 90000, 456)
	require.NoError(t, err)

	fmt.Printf("Number of packets: %d\n", len(packets))
	for i, packet := range packets {
		fmt.Printf("Packet %d: payload length %d, marker %v\n", i, len(packet.Payload), packet.Marker)
		if len(packet.Payload) > 3 {
			fmt.Printf("  Payload[0:3]: %v (header)\n", packet.Payload[:3])
			fmt.Printf("  Payload[3:min(13,%d)]: %v (data)\n", len(packet.Payload), packet.Payload[3:min(13, len(packet.Payload))])
		}
	}

	// Depacketize in reverse order (like the failing test)
	var reconstructedData []byte
	var finalPictureID uint16

	fmt.Printf("Processing packets in reverse order:\n")
	for i := len(packets) - 1; i >= 0; i-- {
		fmt.Printf("Processing packet %d\n", i)
		frameData, pictureID, err := depacketizer.ProcessPacket(packets[i])
		require.NoError(t, err)

		if frameData != nil {
			fmt.Printf("  Got complete frame: length %d\n", len(frameData))
			if len(frameData) > 0 {
				fmt.Printf("  Frame[0:min(10,%d)]: %v\n", len(frameData), frameData[:min(10, len(frameData))])
			}
			reconstructedData = frameData
			finalPictureID = pictureID
		} else {
			fmt.Printf("  Frame not complete yet\n")
		}
	}

	fmt.Printf("Reconstructed data length: %d\n", len(reconstructedData))
	if len(reconstructedData) > 0 {
		fmt.Printf("Reconstructed[0:min(10,%d)]: %v\n", len(reconstructedData), reconstructedData[:min(10, len(reconstructedData))])
	}

	// Verify reconstruction
	assert.NotNil(t, reconstructedData)
	assert.Equal(t, originalData, reconstructedData)
	assert.Equal(t, uint16(456), finalPictureID)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
