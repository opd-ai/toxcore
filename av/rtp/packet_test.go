package rtp

import (
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/transport"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTransport provides a test transport implementation.
type MockTransport struct {
	sentPackets []SentPacket
	localAddr   net.Addr
	handlers    map[transport.PacketType]transport.PacketHandler
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
		handlers:    make(map[transport.PacketType]transport.PacketHandler),
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
	mt.handlers[packetType] = handler
}

func (mt *MockTransport) IsConnectionOriented() bool {
	return false
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

	// Now add data - add in reverse order to test sorting
	jitterBuffer.Add(2000, testData2)
	jitterBuffer.Add(1000, testData1)

	// Should be able to get data immediately since buffer time has passed
	// With timestamp ordering, should return testData1 (timestamp 1000) first
	data, available := jitterBuffer.Get()
	assert.True(t, available)
	assert.NotNil(t, data)
	assert.Equal(t, testData1, data) // Oldest timestamp should be returned first

	// Immediately try to get another packet - should fail because buffer time hasn't passed since last get
	data2, available2 := jitterBuffer.Get()
	assert.False(t, available2)
	assert.Nil(t, data2)

	// Wait for buffer time and try again for second packet
	time.Sleep(bufferTime + 10*time.Millisecond)
	data2, available2 = jitterBuffer.Get()
	assert.True(t, available2)
	assert.NotNil(t, data2)
	assert.Equal(t, testData2, data2) // Second oldest timestamp should be returned

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

// MockTimeProvider allows deterministic time control for testing.
type MockTimeProvider struct {
	currentTime time.Time
}

func (m *MockTimeProvider) Now() time.Time {
	return m.currentTime
}

func (m *MockTimeProvider) Advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

// MockSSRCProvider provides deterministic SSRC values for testing.
type MockSSRCProvider struct {
	ssrcValues []uint32
	callCount  int
}

func (m *MockSSRCProvider) GenerateSSRC() (uint32, error) {
	if m.callCount >= len(m.ssrcValues) {
		return 12345678, nil // default fallback
	}
	ssrc := m.ssrcValues[m.callCount]
	m.callCount++
	return ssrc, nil
}

func TestDeterministicSSRCProvider(t *testing.T) {
	mockTransport := NewMockTransport()
	remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:54321")

	// Create deterministic SSRC provider
	ssrcProvider := &MockSSRCProvider{
		ssrcValues: []uint32{0xAABBCCDD},
	}

	packetizer, err := NewAudioPacketizerWithSSRCProvider(48000, mockTransport, remoteAddr, ssrcProvider)
	require.NoError(t, err)

	// Verify deterministic SSRC
	assert.Equal(t, uint32(0xAABBCCDD), packetizer.ssrc)
}

func TestDeterministicTimeProvider_JitterBuffer(t *testing.T) {
	mockTime := &MockTimeProvider{
		currentTime: time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC),
	}

	// Create jitter buffer with mock time
	jb := NewJitterBufferWithTimeProvider(50*time.Millisecond, mockTime)

	// Add a packet
	jb.Add(1000, []byte{0x01, 0x02, 0x03})

	// Initially, buffer time hasn't elapsed
	data, available := jb.Get()
	assert.False(t, available)
	assert.Nil(t, data)

	// Advance time past buffer duration
	mockTime.Advance(60 * time.Millisecond)

	// Now data should be available
	data, available = jb.Get()
	assert.True(t, available)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, data)
}

func TestDeterministicTimeProvider_JitterBuffer_Reset(t *testing.T) {
	mockTime := &MockTimeProvider{
		currentTime: time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC),
	}

	jb := NewJitterBufferWithTimeProvider(50*time.Millisecond, mockTime)
	jb.Add(1000, []byte{0x01})
	jb.Add(2000, []byte{0x02})

	// Reset should use the mock time
	jb.Reset()

	// Buffer should be empty
	data, available := jb.Get()
	assert.False(t, available)
	assert.Nil(t, data)
}

func TestSetTimeProvider_JitterBuffer(t *testing.T) {
	jb := NewJitterBuffer(50 * time.Millisecond)

	mockTime := &MockTimeProvider{
		currentTime: time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC),
	}

	// Set custom time provider
	jb.SetTimeProvider(mockTime)

	// Reset to sync lastDequeue with new time provider
	jb.Reset()

	// Add packet
	jb.Add(1000, []byte{0x01})

	// Advance mock time
	mockTime.Advance(100 * time.Millisecond)

	// Get should use the mock time
	data, available := jb.Get()
	assert.True(t, available)
	assert.Equal(t, []byte{0x01}, data)
}

func TestSetTimeProvider_JitterBuffer_NilFallback(t *testing.T) {
	jb := NewJitterBuffer(50 * time.Millisecond)

	// Setting nil should fall back to default
	jb.SetTimeProvider(nil)

	// Should still work with default time provider
	assert.NotNil(t, jb.timeProvider)
}

func TestDefaultTimeProvider(t *testing.T) {
	tp := DefaultTimeProvider{}

	before := time.Now()
	now := tp.Now()
	after := time.Now()

	// Time should be between before and after
	assert.True(t, !now.Before(before), "time should not be before test start")
	assert.True(t, !now.After(after), "time should not be after test end")
}

func TestDefaultSSRCProvider(t *testing.T) {
	sp := DefaultSSRCProvider{}

	// Generate multiple SSRCs
	ssrc1, err1 := sp.GenerateSSRC()
	ssrc2, err2 := sp.GenerateSSRC()

	require.NoError(t, err1)
	require.NoError(t, err2)

	// SSRCs should be different (with high probability)
	assert.NotEqual(t, ssrc1, ssrc2, "SSRCs should be unique")
}

func TestJitterBuffer_TimestampOrdering(t *testing.T) {
	// Test that packets are returned in timestamp order regardless of insertion order
	mockTime := &MockTimeProvider{
		currentTime: time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC),
	}

	jb := NewJitterBufferWithTimeProvider(50*time.Millisecond, mockTime)

	// Add packets out of order
	jb.Add(3000, []byte{0x03})
	jb.Add(1000, []byte{0x01})
	jb.Add(5000, []byte{0x05})
	jb.Add(2000, []byte{0x02})
	jb.Add(4000, []byte{0x04})

	// Verify they come out in timestamp order
	mockTime.Advance(60 * time.Millisecond)
	data, ok := jb.Get()
	assert.True(t, ok)
	assert.Equal(t, []byte{0x01}, data, "First packet should have timestamp 1000")

	mockTime.Advance(60 * time.Millisecond)
	data, ok = jb.Get()
	assert.True(t, ok)
	assert.Equal(t, []byte{0x02}, data, "Second packet should have timestamp 2000")

	mockTime.Advance(60 * time.Millisecond)
	data, ok = jb.Get()
	assert.True(t, ok)
	assert.Equal(t, []byte{0x03}, data, "Third packet should have timestamp 3000")

	mockTime.Advance(60 * time.Millisecond)
	data, ok = jb.Get()
	assert.True(t, ok)
	assert.Equal(t, []byte{0x04}, data, "Fourth packet should have timestamp 4000")

	mockTime.Advance(60 * time.Millisecond)
	data, ok = jb.Get()
	assert.True(t, ok)
	assert.Equal(t, []byte{0x05}, data, "Fifth packet should have timestamp 5000")

	// Buffer should be empty
	mockTime.Advance(60 * time.Millisecond)
	data, ok = jb.Get()
	assert.False(t, ok)
	assert.Nil(t, data)
}

func TestJitterBuffer_CapacityLimit(t *testing.T) {
	mockTime := &MockTimeProvider{
		currentTime: time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC),
	}

	// Create buffer with small capacity
	jb := NewJitterBufferWithOptions(50*time.Millisecond, 3, mockTime)

	// Add more packets than capacity
	jb.Add(1000, []byte{0x01})
	jb.Add(2000, []byte{0x02})
	jb.Add(3000, []byte{0x03})

	// Verify buffer is at capacity
	assert.Equal(t, 3, jb.Len())

	// Add one more - should evict oldest (timestamp 1000)
	jb.Add(4000, []byte{0x04})

	// Buffer should still be at capacity
	assert.Equal(t, 3, jb.Len())

	// First packet should now be timestamp 2000 (oldest was evicted)
	mockTime.Advance(60 * time.Millisecond)
	data, ok := jb.Get()
	assert.True(t, ok)
	assert.Equal(t, []byte{0x02}, data, "Oldest packet should be timestamp 2000 after eviction")
}

func TestJitterBuffer_CapacityEvictionOrder(t *testing.T) {
	mockTime := &MockTimeProvider{
		currentTime: time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC),
	}

	// Create buffer with small capacity
	jb := NewJitterBufferWithOptions(50*time.Millisecond, 2, mockTime)

	// Add packets - third should evict first
	jb.Add(1000, []byte{0x01})
	jb.Add(2000, []byte{0x02})
	jb.Add(3000, []byte{0x03}) // Should evict timestamp 1000

	assert.Equal(t, 2, jb.Len())

	// Verify remaining packets are 2000 and 3000
	mockTime.Advance(60 * time.Millisecond)
	data1, _ := jb.Get()
	mockTime.Advance(60 * time.Millisecond)
	data2, _ := jb.Get()

	assert.Equal(t, []byte{0x02}, data1)
	assert.Equal(t, []byte{0x03}, data2)
}

func TestJitterBuffer_SetMaxCapacity(t *testing.T) {
	mockTime := &MockTimeProvider{
		currentTime: time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC),
	}

	jb := NewJitterBufferWithOptions(50*time.Millisecond, 5, mockTime)

	// Add 5 packets
	for i := uint32(1); i <= 5; i++ {
		jb.Add(i*1000, []byte{byte(i)})
	}
	assert.Equal(t, 5, jb.Len())

	// Reduce capacity - should evict oldest packets
	jb.SetMaxCapacity(2)
	assert.Equal(t, 2, jb.Len())

	// Remaining packets should be the newest (timestamps 4000 and 5000)
	mockTime.Advance(60 * time.Millisecond)
	data1, _ := jb.Get()
	mockTime.Advance(60 * time.Millisecond)
	data2, _ := jb.Get()

	assert.Equal(t, []byte{0x04}, data1)
	assert.Equal(t, []byte{0x05}, data2)
}

func TestJitterBuffer_SetMaxCapacityZeroUsesDefault(t *testing.T) {
	jb := NewJitterBuffer(50 * time.Millisecond)

	// Setting 0 should use default
	jb.SetMaxCapacity(0)
	assert.Equal(t, DefaultMaxBufferCapacity, jb.maxCapacity)

	// Setting negative should use default
	jb.SetMaxCapacity(-1)
	assert.Equal(t, DefaultMaxBufferCapacity, jb.maxCapacity)
}

func TestJitterBuffer_OutOfOrderInsertion(t *testing.T) {
	mockTime := &MockTimeProvider{
		currentTime: time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC),
	}

	jb := NewJitterBufferWithTimeProvider(50*time.Millisecond, mockTime)

	// Insert packet with later timestamp first
	jb.Add(5000, []byte{0x05})
	// Then insert packet with earlier timestamp
	jb.Add(1000, []byte{0x01})

	// Earlier timestamp should come out first
	mockTime.Advance(60 * time.Millisecond)
	data, ok := jb.Get()
	assert.True(t, ok)
	assert.Equal(t, []byte{0x01}, data, "Earlier timestamp should be returned first")
}

func TestJitterBuffer_DuplicateTimestamp(t *testing.T) {
	mockTime := &MockTimeProvider{
		currentTime: time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC),
	}

	jb := NewJitterBufferWithTimeProvider(50*time.Millisecond, mockTime)

	// Add packets with same timestamp - both should be stored
	jb.Add(1000, []byte{0x01})
	jb.Add(1000, []byte{0x02})

	assert.Equal(t, 2, jb.Len())
}

func TestNewJitterBufferWithOptions_Defaults(t *testing.T) {
	// Test nil time provider defaults
	jb := NewJitterBufferWithOptions(50*time.Millisecond, 10, nil)
	assert.NotNil(t, jb.timeProvider)

	// Test zero capacity defaults
	jb2 := NewJitterBufferWithOptions(50*time.Millisecond, 0, nil)
	assert.Equal(t, DefaultMaxBufferCapacity, jb2.maxCapacity)

	// Test negative capacity defaults
	jb3 := NewJitterBufferWithOptions(50*time.Millisecond, -5, nil)
	assert.Equal(t, DefaultMaxBufferCapacity, jb3.maxCapacity)
}
