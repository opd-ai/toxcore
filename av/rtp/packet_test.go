package rtp

import (
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxforge/transport"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTransport provides a test transport implementation.
type MockTransport struct {
	sentPackets []SentPacket
	localAddr   net.Addr
}

type SentPacket struct {
	Packet *transport.Packet
	Addr   net.Addr
}

func NewMockTransport() *MockTransport {
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")
	return &MockTransport{
		sentPackets: make([]SentPacket, 0),
		localAddr:   addr,
	}
}

func (mt *MockTransport) Send(packet *transport.Packet, addr net.Addr) error {
	mt.sentPackets = append(mt.sentPackets, SentPacket{
		Packet: packet,
		Addr:   addr,
	})
	return nil
}

func (mt *MockTransport) Close() error {
	return nil
}

func (mt *MockTransport) LocalAddr() net.Addr {
	return mt.localAddr
}

func (mt *MockTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
	// Mock implementation
}

func (mt *MockTransport) GetSentPackets() []SentPacket {
	return mt.sentPackets
}

func TestNewAudioPacketizer(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	tests := []struct {
		name        string
		clockRate   uint32
		transport   transport.Transport
		remoteAddr  net.Addr
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid parameters",
			clockRate:   48000,
			transport:   mockTransport,
			remoteAddr:  remoteAddr,
			expectError: false,
		},
		{
			name:        "Zero clock rate",
			clockRate:   0,
			transport:   mockTransport,
			remoteAddr:  remoteAddr,
			expectError: true,
			errorMsg:    "clock rate cannot be zero",
		},
		{
			name:        "Nil transport",
			clockRate:   48000,
			transport:   nil,
			remoteAddr:  remoteAddr,
			expectError: true,
			errorMsg:    "transport cannot be nil",
		},
		{
			name:        "Nil remote address",
			clockRate:   48000,
			transport:   mockTransport,
			remoteAddr:  nil,
			expectError: true,
			errorMsg:    "remote address cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packetizer, err := NewAudioPacketizer(tt.clockRate, tt.transport, tt.remoteAddr)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, packetizer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, packetizer)
				assert.Equal(t, tt.clockRate, packetizer.clockRate)
				assert.Equal(t, tt.transport, packetizer.transport)
				assert.Equal(t, tt.remoteAddr, packetizer.remoteAddr)
				assert.NotZero(t, packetizer.ssrc) // Should have random SSRC
			}
		})
	}
}

func TestAudioPacketizer_PacketizeAndSend(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	packetizer, err := NewAudioPacketizer(48000, mockTransport, remoteAddr)
	require.NoError(t, err)

	tests := []struct {
		name        string
		audioData   []byte
		sampleCount uint32
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid audio data",
			audioData:   []byte{0x01, 0x02, 0x03, 0x04},
			sampleCount: 960, // 20ms at 48kHz
			expectError: false,
		},
		{
			name:        "Empty audio data",
			audioData:   []byte{},
			sampleCount: 960,
			expectError: true,
			errorMsg:    "audio data cannot be empty",
		},
		{
			name:        "Large audio frame",
			audioData:   make([]byte, 1372), // Max Tox message size
			sampleCount: 2880,               // 60ms at 48kHz
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialPacketCount := len(mockTransport.GetSentPackets())
			initialSeq := packetizer.sequenceNumber
			initialTimestamp := packetizer.timestamp

			err := packetizer.PacketizeAndSend(tt.audioData, tt.sampleCount)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				// Should not send packet on error
				assert.Equal(t, initialPacketCount, len(mockTransport.GetSentPackets()))
			} else {
				assert.NoError(t, err)

				// Should send exactly one packet
				sentPackets := mockTransport.GetSentPackets()
				assert.Equal(t, initialPacketCount+1, len(sentPackets))

				// Verify packet details
				sentPacket := sentPackets[len(sentPackets)-1]
				assert.Equal(t, transport.PacketAVAudioFrame, sentPacket.Packet.PacketType)
				assert.Equal(t, remoteAddr, sentPacket.Addr)

				// Parse RTP packet
				rtpPacket := &rtp.Packet{}
				err = rtpPacket.Unmarshal(sentPacket.Packet.Data)
				assert.NoError(t, err)

				// Verify RTP header
				assert.Equal(t, uint8(2), rtpPacket.Version)
				assert.Equal(t, uint8(96), rtpPacket.PayloadType) // Opus payload type
				assert.Equal(t, initialSeq, rtpPacket.SequenceNumber)
				assert.Equal(t, initialTimestamp, rtpPacket.Timestamp)
				assert.Equal(t, packetizer.ssrc, rtpPacket.SSRC)
				assert.Equal(t, tt.audioData, rtpPacket.Payload)

				// Verify sequence and timestamp increment
				assert.Equal(t, initialSeq+1, packetizer.sequenceNumber)
				assert.Equal(t, initialTimestamp+tt.sampleCount, packetizer.timestamp)
			}
		})
	}
}

func TestAudioPacketizer_SequenceNumberWraparound(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	packetizer, err := NewAudioPacketizer(48000, mockTransport, remoteAddr)
	require.NoError(t, err)

	// Set sequence number near wraparound
	packetizer.sequenceNumber = 65534

	audioData := []byte{0x01, 0x02, 0x03, 0x04}
	sampleCount := uint32(960)

	// Send first packet
	err = packetizer.PacketizeAndSend(audioData, sampleCount)
	assert.NoError(t, err)
	assert.Equal(t, uint16(65535), packetizer.sequenceNumber)

	// Send second packet (should wraparound)
	err = packetizer.PacketizeAndSend(audioData, sampleCount)
	assert.NoError(t, err)
	assert.Equal(t, uint16(0), packetizer.sequenceNumber)

	// Send third packet
	err = packetizer.PacketizeAndSend(audioData, sampleCount)
	assert.NoError(t, err)
	assert.Equal(t, uint16(1), packetizer.sequenceNumber)
}

func TestNewAudioDepacketizer(t *testing.T) {
	depacketizer := NewAudioDepacketizer()

	assert.NotNil(t, depacketizer)
	assert.NotNil(t, depacketizer.jitterBuffer)
	assert.False(t, depacketizer.hasSSRC)
	assert.False(t, depacketizer.hasLastSeq)
}

func TestAudioDepacketizer_ProcessPacket(t *testing.T) {
	depacketizer := NewAudioDepacketizer()

	// Create valid RTP packet
	audioData := []byte{0x01, 0x02, 0x03, 0x04}
	rtpPacket := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: 1000,
			Timestamp:      48000,
			SSRC:           0x12345678,
		},
		Payload: audioData,
	}

	rtpData, err := rtpPacket.Marshal()
	require.NoError(t, err)

	tests := []struct {
		name        string
		rtpData     []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid RTP packet",
			rtpData:     rtpData,
			expectError: false,
		},
		{
			name:        "Empty RTP data",
			rtpData:     []byte{},
			expectError: true,
			errorMsg:    "RTP data cannot be empty",
		},
		{
			name:        "Invalid RTP data",
			rtpData:     []byte{0x01, 0x02}, // Too short for RTP header
			expectError: true,
			errorMsg:    "failed to unmarshal RTP packet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractedData, timestamp, err := depacketizer.ProcessPacket(tt.rtpData)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, extractedData)
				assert.Zero(t, timestamp)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, audioData, extractedData)
				assert.Equal(t, uint32(48000), timestamp)

				// SSRC should be remembered
				assert.True(t, depacketizer.hasSSRC)
				assert.Equal(t, uint32(0x12345678), depacketizer.expectedSSRC)
			}
		})
	}
}

func TestAudioDepacketizer_SSRCValidation(t *testing.T) {
	depacketizer := NewAudioDepacketizer()

	// First packet with SSRC1
	packet1 := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: 1000,
			Timestamp:      48000,
			SSRC:           0x12345678,
		},
		Payload: []byte{0x01, 0x02},
	}

	rtpData1, err := packet1.Marshal()
	require.NoError(t, err)

	// Process first packet - should succeed
	_, _, err = depacketizer.ProcessPacket(rtpData1)
	assert.NoError(t, err)

	// Second packet with different SSRC
	packet2 := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: 1001,
			Timestamp:      48960,
			SSRC:           0x87654321, // Different SSRC
		},
		Payload: []byte{0x03, 0x04},
	}

	rtpData2, err := packet2.Marshal()
	require.NoError(t, err)

	// Process second packet - should fail
	_, _, err = depacketizer.ProcessPacket(rtpData2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected SSRC")
}

func TestJitterBuffer(t *testing.T) {
	bufferTime := 50 * time.Millisecond
	jitterBuffer := NewJitterBuffer(bufferTime)

	// Add some test data
	testData1 := []byte{0x01, 0x02, 0x03}
	testData2 := []byte{0x04, 0x05, 0x06}

	// Initially, buffer time should have passed since creation (lastDequeue is set to time.Now())
	// So we need to wait for the buffer time first
	time.Sleep(bufferTime + 10*time.Millisecond)

	// Now add data
	jitterBuffer.Add(1000, testData1)
	jitterBuffer.Add(2000, testData2)

	// Should be able to get data immediately since buffer time has passed
	data, available := jitterBuffer.Get()
	assert.True(t, available)
	assert.NotNil(t, data)
	assert.Contains(t, [][]byte{testData1, testData2}, data) // Either packet could be returned

	// Immediately try to get another packet - should fail because buffer time hasn't passed since last get
	data2, available2 := jitterBuffer.Get()
	assert.False(t, available2)
	assert.Nil(t, data2)

	// Wait for buffer time and try again for second packet
	time.Sleep(bufferTime + 10*time.Millisecond)
	data2, available2 = jitterBuffer.Get()
	if len(jitterBuffer.packets) > 0 { // Only test if there's still data
		assert.True(t, available2)
		assert.NotNil(t, data2)
		assert.Contains(t, [][]byte{testData1, testData2}, data2)
		assert.NotEqual(t, data, data2) // Should be different packets
	}

	// Buffer should be empty now
	time.Sleep(bufferTime + 10*time.Millisecond)
	data, available = jitterBuffer.Get()
	assert.False(t, available)
	assert.Nil(t, data)
}

func TestJitterBuffer_Reset(t *testing.T) {
	jitterBuffer := NewJitterBuffer(50 * time.Millisecond)

	// Add some data
	jitterBuffer.Add(1000, []byte{0x01, 0x02})
	jitterBuffer.Add(2000, []byte{0x03, 0x04})

	// Reset buffer
	jitterBuffer.Reset()

	// Should have no data even after buffer time
	time.Sleep(60 * time.Millisecond)
	data, available := jitterBuffer.Get()
	assert.False(t, available)
	assert.Nil(t, data)
}

// Benchmark tests for performance validation
func BenchmarkAudioPacketizer_PacketizeAndSend(b *testing.B) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	packetizer, err := NewAudioPacketizer(48000, mockTransport, remoteAddr)
	require.NoError(b, err)

	audioData := make([]byte, 160) // Typical Opus frame size
	sampleCount := uint32(960)     // 20ms at 48kHz

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := packetizer.PacketizeAndSend(audioData, sampleCount)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAudioDepacketizer_ProcessPacket(b *testing.B) {
	depacketizer := NewAudioDepacketizer()

	// Create test RTP packet
	audioData := make([]byte, 160)
	rtpPacket := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: 1000,
			Timestamp:      48000,
			SSRC:           0x12345678,
		},
		Payload: audioData,
	}

	rtpData, err := rtpPacket.Marshal()
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Update sequence number for each packet
		rtpPacket.SequenceNumber = uint16(1000 + i)
		rtpPacket.Timestamp = uint32(48000 + i*960)
		rtpData, _ = rtpPacket.Marshal()

		_, _, err := depacketizer.ProcessPacket(rtpData)
		if err != nil {
			b.Fatal(err)
		}
	}
}
