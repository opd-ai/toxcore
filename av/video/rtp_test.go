package video

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRTPPacketizer(t *testing.T) {
	ssrc := uint32(12345)
	packetizer := NewRTPPacketizer(ssrc)

	assert.NotNil(t, packetizer)
	assert.Equal(t, ssrc, packetizer.ssrc)
	assert.Equal(t, uint16(1), packetizer.sequenceNumber)
	assert.Equal(t, uint8(96), packetizer.payloadType)
	assert.Equal(t, 1200, packetizer.maxPacketSize)
}

func TestRTPPacketizer_PacketizeFrame_SinglePacket(t *testing.T) {
	packetizer := NewRTPPacketizer(12345)
	frameData := make([]byte, 500) // Small frame fits in single packet
	timestamp := uint32(90000)
	pictureID := uint16(123)

	packets, err := packetizer.PacketizeFrame(frameData, timestamp, pictureID)

	require.NoError(t, err)
	assert.Len(t, packets, 1)

	packet := packets[0]
	assert.Equal(t, uint8(2), packet.Version)
	assert.Equal(t, uint8(96), packet.PayloadType)
	assert.Equal(t, uint16(1), packet.SequenceNumber)
	assert.Equal(t, timestamp, packet.Timestamp)
	assert.Equal(t, uint32(12345), packet.SSRC)
	assert.True(t, packet.Marker) // Single packet should have marker
	assert.True(t, packet.ExtendedControlBits)
	assert.True(t, packet.StartOfPartition)
	assert.Equal(t, pictureID, packet.PictureID)
}

func TestRTPPacketizer_PacketizeFrame_MultiplePackets(t *testing.T) {
	packetizer := NewRTPPacketizer(12345)
	frameData := make([]byte, 3000) // Large frame requires multiple packets
	timestamp := uint32(90000)
	pictureID := uint16(456)

	packets, err := packetizer.PacketizeFrame(frameData, timestamp, pictureID)

	require.NoError(t, err)
	assert.Greater(t, len(packets), 1) // Should be multiple packets

	// Check first packet
	assert.False(t, packets[0].Marker)          // Not last packet
	assert.True(t, packets[0].StartOfPartition) // First packet

	// Check middle packets (if any)
	for i := 1; i < len(packets)-1; i++ {
		assert.False(t, packets[i].Marker)           // Not last packet
		assert.False(t, packets[i].StartOfPartition) // Not first packet
		assert.Equal(t, uint16(i+1), packets[i].SequenceNumber)
	}

	// Check last packet
	lastIdx := len(packets) - 1
	assert.True(t, packets[lastIdx].Marker)            // Last packet
	assert.False(t, packets[lastIdx].StartOfPartition) // Not first packet

	// All packets should have same timestamp and picture ID
	for _, packet := range packets {
		assert.Equal(t, timestamp, packet.Timestamp)
		assert.Equal(t, pictureID, packet.PictureID)
		assert.Equal(t, uint32(12345), packet.SSRC)
	}
}

func TestRTPPacketizer_PacketizeFrame_ErrorCases(t *testing.T) {
	packetizer := NewRTPPacketizer(12345)

	tests := []struct {
		name        string
		frameData   []byte
		expectedErr string
	}{
		{
			name:        "empty frame",
			frameData:   []byte{},
			expectedErr: "frame data cannot be empty",
		},
		{
			name:        "frame too large",
			frameData:   make([]byte, 3000000), // 3MB > 2MB limit
			expectedErr: "frame data too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packets, err := packetizer.PacketizeFrame(tt.frameData, 90000, 123)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
			assert.Nil(t, packets)
		})
	}
}

func TestRTPPacketizer_SequenceNumberIncrement(t *testing.T) {
	packetizer := NewRTPPacketizer(12345)
	frameData := make([]byte, 3000) // Multiple packets

	packets1, err := packetizer.PacketizeFrame(frameData, 90000, 1)
	require.NoError(t, err)

	packets2, err := packetizer.PacketizeFrame(frameData, 180000, 2)
	require.NoError(t, err)

	// Second frame should start where first frame ended
	expectedSeq := packets1[len(packets1)-1].SequenceNumber + 1
	assert.Equal(t, expectedSeq, packets2[0].SequenceNumber)
}

func TestRTPPacketizer_SequenceNumberWrap(t *testing.T) {
	packetizer := NewRTPPacketizer(12345)
	packetizer.sequenceNumber = 65535 // Near max

	frameData := make([]byte, 100)
	packets, err := packetizer.PacketizeFrame(frameData, 90000, 1)
	require.NoError(t, err)

	assert.Equal(t, uint16(65535), packets[0].SequenceNumber)

	// Next frame should wrap to 1 (skip 0)
	packets2, err := packetizer.PacketizeFrame(frameData, 180000, 2)
	require.NoError(t, err)
	assert.Equal(t, uint16(1), packets2[0].SequenceNumber)
}

func TestNewRTPDepacketizer(t *testing.T) {
	depacketizer := NewRTPDepacketizer()

	assert.NotNil(t, depacketizer)
	assert.NotNil(t, depacketizer.frameBuffer)
	assert.Equal(t, 10, depacketizer.maxFrames)
	assert.Equal(t, 0, len(depacketizer.frameBuffer))
}

func TestRTPDepacketizer_ProcessPacket_SinglePacket(t *testing.T) {
	depacketizer := NewRTPDepacketizer()

	// Create a single-packet frame
	packet := RTPPacket{
		Version:             2,
		Marker:              true, // Single packet
		PayloadType:         96,
		SequenceNumber:      1,
		Timestamp:           90000,
		SSRC:                12345,
		ExtendedControlBits: true,
		StartOfPartition:    true,
		PictureID:           123,
		Payload:             []byte{0x80, 0x00, 0x7B, 0x01, 0x02, 0x03}, // VP8 descriptor + data
	}

	frameData, pictureID, err := depacketizer.ProcessPacket(packet)

	require.NoError(t, err)
	assert.NotNil(t, frameData)
	assert.Equal(t, uint16(123), pictureID)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, frameData)     // Just the payload data
	assert.Equal(t, 0, depacketizer.GetBufferedFrameCount()) // Should be cleaned up
}

func TestRTPDepacketizer_ProcessPacket_MultiplePackets(t *testing.T) {
	depacketizer := NewRTPDepacketizer()

	// Create multi-packet frame
	packets := []RTPPacket{
		{
			Version:             2,
			Marker:              false, // Not last
			SequenceNumber:      1,
			Timestamp:           90000,
			SSRC:                12345,
			ExtendedControlBits: true,
			StartOfPartition:    true, // First packet
			PictureID:           123,
			Payload:             []byte{0x90, 0x00, 0x7B, 0x01, 0x02}, // VP8 descriptor + data
		},
		{
			Version:             2,
			Marker:              true, // Last packet
			SequenceNumber:      2,
			Timestamp:           90000, // Same timestamp
			SSRC:                12345,
			ExtendedControlBits: true,
			StartOfPartition:    false, // Not first
			PictureID:           123,
			Payload:             []byte{0x80, 0x00, 0x7B, 0x03, 0x04}, // VP8 descriptor + data
		},
	}

	// Process first packet (incomplete)
	frameData, pictureID, err := depacketizer.ProcessPacket(packets[0])
	require.NoError(t, err)
	assert.Nil(t, frameData) // Frame not complete yet
	assert.Equal(t, uint16(0), pictureID)
	assert.Equal(t, 1, depacketizer.GetBufferedFrameCount())

	// Process second packet (complete)
	frameData, pictureID, err = depacketizer.ProcessPacket(packets[1])
	require.NoError(t, err)
	assert.NotNil(t, frameData)
	assert.Equal(t, uint16(123), pictureID)
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, frameData) // Combined payload
	assert.Equal(t, 0, depacketizer.GetBufferedFrameCount())   // Should be cleaned up
}

func TestRTPDepacketizer_ProcessPacket_OutOfOrder(t *testing.T) {
	depacketizer := NewRTPDepacketizer()

	// Create packets for a 2-packet frame
	packet1 := RTPPacket{
		Version:             2,
		Marker:              false, // First packet
		SequenceNumber:      1,
		Timestamp:           90000,
		SSRC:                12345,
		ExtendedControlBits: true,
		StartOfPartition:    true, // Start of frame
		PictureID:           123,
		Payload:             []byte{0x90, 0x00, 0x7B, 0x01, 0x02}, // VP8 descriptor + data
	}

	packet2 := RTPPacket{
		Version:             2,
		Marker:              true, // Last packet
		SequenceNumber:      2,
		Timestamp:           90000,
		SSRC:                12345,
		ExtendedControlBits: true,
		StartOfPartition:    false,
		PictureID:           123,
		Payload:             []byte{0x80, 0x00, 0x7B, 0x03, 0x04}, // VP8 descriptor + data
	}

	// Process packets in reverse order
	// First process packet 2 (marker packet) - should not complete frame yet
	frameData, pictureID, err := depacketizer.ProcessPacket(packet2)
	require.NoError(t, err)
	assert.Nil(t, frameData) // Should not complete frame without start packet
	assert.Equal(t, uint16(0), pictureID)

	// Then process packet 1 (start packet) - should complete frame
	frameData, pictureID, err = depacketizer.ProcessPacket(packet1)
	require.NoError(t, err)
	assert.NotNil(t, frameData) // Should complete frame now
	assert.Equal(t, uint16(123), pictureID)

	// Frame should contain both packets' data
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, frameData)
}

func TestRTPDepacketizer_ProcessPacket_ErrorCases(t *testing.T) {
	depacketizer := NewRTPDepacketizer()

	tests := []struct {
		name        string
		payload     []byte
		expectedErr string
	}{
		{
			name:        "payload too short",
			payload:     []byte{0x80}, // Less than 3 bytes
			expectedErr: "VP8 payload too short",
		},
		{
			name:        "no extended control bits",
			payload:     []byte{0x00, 0x00, 0x7B}, // X bit not set
			expectedErr: "expected extended control bits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet := RTPPacket{
				Version:   2,
				Marker:    true,
				Timestamp: 90000,
				Payload:   tt.payload,
			}

			frameData, pictureID, err := depacketizer.ProcessPacket(packet)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
			assert.Nil(t, frameData)
			assert.Equal(t, uint16(0), pictureID)
		})
	}
}

func TestRTPPacketizer_SetMaxPacketSize(t *testing.T) {
	packetizer := NewRTPPacketizer(12345)

	// Valid sizes
	err := packetizer.SetMaxPacketSize(500)
	assert.NoError(t, err)
	assert.Equal(t, 500, packetizer.maxPacketSize)

	err = packetizer.SetMaxPacketSize(1500)
	assert.NoError(t, err)
	assert.Equal(t, 1500, packetizer.maxPacketSize)

	// Invalid sizes
	err = packetizer.SetMaxPacketSize(50)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid packet size")

	err = packetizer.SetMaxPacketSize(10000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid packet size")
}

func TestRTPPacketizer_GetStats(t *testing.T) {
	packetizer := NewRTPPacketizer(12345)

	// Initial stats
	seq, ts := packetizer.GetStats()
	assert.Equal(t, uint16(1), seq)
	assert.Equal(t, uint32(0), ts)

	// After packetizing
	frameData := make([]byte, 100)
	_, err := packetizer.PacketizeFrame(frameData, 90000, 1)
	require.NoError(t, err)

	seq, ts = packetizer.GetStats()
	assert.Equal(t, uint16(2), seq) // Incremented
	assert.Equal(t, uint32(0), ts)  // Timestamp not stored in packetizer
}

func TestRTPDepacketizer_FrameTimeout(t *testing.T) {
	depacketizer := NewRTPDepacketizer()

	// Create incomplete frame
	packet := RTPPacket{
		Version:             2,
		Marker:              false, // Incomplete
		SequenceNumber:      1,
		Timestamp:           90000,
		SSRC:                12345,
		ExtendedControlBits: true,
		StartOfPartition:    true,
		PictureID:           123,
		Payload:             []byte{0x90, 0x00, 0x7B, 0x01, 0x02},
	}

	frameData, _, err := depacketizer.ProcessPacket(packet)
	require.NoError(t, err)
	assert.Nil(t, frameData) // Incomplete
	assert.Equal(t, 1, depacketizer.GetBufferedFrameCount())

	// Manually set old timestamp to trigger cleanup
	for _, assembly := range depacketizer.frameBuffer {
		assembly.lastActivity = time.Now().Add(-10 * time.Second)
	}

	// Fill buffer to trigger cleanup
	for i := 0; i < 15; i++ {
		newPacket := packet
		newPacket.Timestamp = uint32(90000 + i*3000)
		depacketizer.ProcessPacket(newPacket)
	}

	// Buffer should be at the maximum limit after cleanup
	assert.LessOrEqual(t, depacketizer.GetBufferedFrameCount(), 10)
}

func TestRTPRoundTrip(t *testing.T) {
	// Create packetizer and depacketizer
	packetizer := NewRTPPacketizer(12345)
	depacketizer := NewRTPDepacketizer()

	// Original frame data
	originalData := make([]byte, 2500) // Multiple packets
	for i := range originalData {
		originalData[i] = byte(i % 256) // Test pattern
	}

	// Packetize
	packets, err := packetizer.PacketizeFrame(originalData, 90000, 456)
	require.NoError(t, err)
	assert.Greater(t, len(packets), 1) // Should be multiple packets

	// Depacketize (simulate out-of-order arrival)
	var reconstructedData []byte
	var finalPictureID uint16

	// Process packets in reverse order to test sorting
	for i := len(packets) - 1; i >= 0; i-- {
		frameData, pictureID, err := depacketizer.ProcessPacket(packets[i])
		require.NoError(t, err)

		if frameData != nil {
			reconstructedData = frameData
			finalPictureID = pictureID
		}
	}

	// Verify reconstruction
	assert.NotNil(t, reconstructedData)
	assert.Equal(t, originalData, reconstructedData)
	assert.Equal(t, uint16(456), finalPictureID)
}

// Benchmark RTP operations
func BenchmarkRTPPacketizer_PacketizeFrame(b *testing.B) {
	packetizer := NewRTPPacketizer(12345)
	frameData := make([]byte, 2000) // Typical frame size

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := packetizer.PacketizeFrame(frameData, uint32(i*3000), uint16(i))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRTPDepacketizer_ProcessPacket(b *testing.B) {
	depacketizer := NewRTPDepacketizer()
	packet := RTPPacket{
		Version:             2,
		Marker:              true,
		SequenceNumber:      1,
		Timestamp:           90000,
		SSRC:                12345,
		ExtendedControlBits: true,
		StartOfPartition:    true,
		PictureID:           123,
		Payload:             []byte{0x80, 0x00, 0x7B, 0x01, 0x02, 0x03},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		packet.Timestamp = uint32(90000 + i*3000) // Unique timestamp
		_, _, err := depacketizer.ProcessPacket(packet)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// mockTimeProvider implements TimeProvider for deterministic testing.
type mockTimeProvider struct {
	currentTime time.Time
}

func (m *mockTimeProvider) Now() time.Time {
	return m.currentTime
}

func (m *mockTimeProvider) Advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

func TestRTPDepacketizer_WithTimeProvider(t *testing.T) {
	// Create a mock time provider with a fixed starting time
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mockTime := &mockTimeProvider{currentTime: startTime}

	// Create depacketizer with custom time provider
	depacketizer := NewRTPDepacketizerWithTimeProvider(mockTime)
	assert.NotNil(t, depacketizer)

	// Create a test packet
	packet := RTPPacket{
		Version:             2,
		Marker:              false, // Not complete
		SequenceNumber:      1,
		Timestamp:           90000,
		SSRC:                12345,
		ExtendedControlBits: true,
		StartOfPartition:    true,
		PictureID:           123,
		Payload:             []byte{0x80, 0x00, 0x7B, 0x01, 0x02, 0x03},
	}

	// Process packet - should create assembly with mocked time
	_, _, err := depacketizer.ProcessPacket(packet)
	require.NoError(t, err)

	// Verify the lastActivity time is from our mock
	assembly := depacketizer.frameBuffer[90000]
	require.NotNil(t, assembly)
	assert.Equal(t, startTime, assembly.lastActivity)

	// Advance time and process another packet to same assembly
	mockTime.Advance(1 * time.Second)
	packet.SequenceNumber = 2
	_, _, err = depacketizer.ProcessPacket(packet)
	require.NoError(t, err)

	// Verify lastActivity was updated with new time
	assert.Equal(t, startTime.Add(1*time.Second), assembly.lastActivity)
}

func TestRTPDepacketizer_SetTimeProvider(t *testing.T) {
	depacketizer := NewRTPDepacketizer()

	// Create a mock time provider
	mockTime := &mockTimeProvider{currentTime: time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)}

	// Set the time provider
	depacketizer.SetTimeProvider(mockTime)

	// Create a test packet
	packet := RTPPacket{
		Version:             2,
		Marker:              false,
		SequenceNumber:      1,
		Timestamp:           90000,
		SSRC:                12345,
		ExtendedControlBits: true,
		StartOfPartition:    true,
		PictureID:           456,
		Payload:             []byte{0x80, 0x01, 0xC8, 0x01, 0x02, 0x03},
	}

	// Process packet
	_, _, err := depacketizer.ProcessPacket(packet)
	require.NoError(t, err)

	// Verify the lastActivity time uses our mock
	assembly := depacketizer.frameBuffer[90000]
	require.NotNil(t, assembly)
	assert.Equal(t, mockTime.currentTime, assembly.lastActivity)
}

func TestRTPDepacketizer_CleanupWithTimeProvider(t *testing.T) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mockTime := &mockTimeProvider{currentTime: startTime}

	depacketizer := NewRTPDepacketizerWithTimeProvider(mockTime)

	// Create incomplete frame assemblies
	for i := 0; i < 5; i++ {
		packet := RTPPacket{
			Version:             2,
			Marker:              false,
			SequenceNumber:      uint16(i + 1),
			Timestamp:           uint32(90000 + i*3000),
			SSRC:                12345,
			ExtendedControlBits: true,
			StartOfPartition:    true,
			PictureID:           uint16(i),
			Payload:             []byte{0x80, 0x00, byte(i), 0x01, 0x02, 0x03},
		}
		_, _, err := depacketizer.ProcessPacket(packet)
		require.NoError(t, err)
	}

	assert.Equal(t, 5, depacketizer.GetBufferedFrameCount())

	// Advance time past the 5-second timeout
	mockTime.Advance(6 * time.Second)

	// Fill buffer to trigger cleanup
	for i := 0; i < 10; i++ {
		packet := RTPPacket{
			Version:             2,
			Marker:              false,
			SequenceNumber:      uint16(100 + i),
			Timestamp:           uint32(200000 + i*3000),
			SSRC:                12345,
			ExtendedControlBits: true,
			StartOfPartition:    true,
			PictureID:           uint16(100 + i),
			Payload:             []byte{0x80, 0x00, byte(100 + i), 0x01, 0x02, 0x03},
		}
		_, _, err := depacketizer.ProcessPacket(packet)
		require.NoError(t, err)
	}

	// Old frames should have been cleaned up, only new ones remain
	// We should have exactly 10 frames (the new ones)
	assert.Equal(t, 10, depacketizer.GetBufferedFrameCount())
}
