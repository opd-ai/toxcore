package transport

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultBatchReceiveConfig(t *testing.T) {
	config := DefaultBatchReceiveConfig()
	require.NotNil(t, config)

	assert.Greater(t, config.BatchSize, 0)
	assert.Greater(t, config.InitialBufferSize, 0)
	assert.GreaterOrEqual(t, config.MaxBufferSize, config.InitialBufferSize)
	assert.True(t, config.EnableDynamicBuffers)
}

func TestBatchReceiverFromConn(t *testing.T) {
	// Create a UDP socket for testing
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer conn.Close()

	config := &BatchReceiveConfig{
		InitialBufferSize:    1024,
		MaxBufferSize:        4096,
		EnableDynamicBuffers: true,
	}

	receiver := NewBatchReceiverFromConn(conn, config)
	require.NotNil(t, receiver)

	// Verify initial state
	stats := receiver.Stats()
	assert.Equal(t, 1024, stats.CurrentBufferSize)
	assert.Equal(t, uint64(0), stats.TotalPackets)
}

func TestBatchReceiverSetBufferSize(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer conn.Close()

	config := &BatchReceiveConfig{
		InitialBufferSize:    1024,
		MaxBufferSize:        4096,
		EnableDynamicBuffers: true,
	}

	receiver := NewBatchReceiverFromConn(conn, config)

	// Test setting valid buffer size
	receiver.SetBufferSize(2048)
	stats := receiver.Stats()
	assert.Equal(t, 2048, stats.CurrentBufferSize)

	// Test clamping to minimum
	receiver.SetBufferSize(100)
	stats = receiver.Stats()
	assert.GreaterOrEqual(t, stats.CurrentBufferSize, MinReceiveBufferSize)

	// Test clamping to maximum
	receiver.SetBufferSize(100000)
	stats = receiver.Stats()
	assert.LessOrEqual(t, stats.CurrentBufferSize, config.MaxBufferSize)
}

func TestBatchReceiverReceivePackets(t *testing.T) {
	// Create server socket
	serverConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer serverConn.Close()

	serverAddr := serverConn.LocalAddr()

	// Create client socket
	clientConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer clientConn.Close()

	// Create receiver for server
	config := &BatchReceiveConfig{
		InitialBufferSize:    2048,
		MaxBufferSize:        4096,
		EnableDynamicBuffers: true,
	}
	receiver := NewBatchReceiverFromConn(serverConn, config)

	// Send some test packets
	testData := []byte("Hello, BatchReceiver!")
	_, err = clientConn.WriteTo(testData, serverAddr)
	require.NoError(t, err)

	// Receive with timeout
	packets, err := receiver.RecvBatch(500 * time.Millisecond)
	require.NoError(t, err)
	require.Len(t, packets, 1)

	assert.Equal(t, testData, packets[0].Data)
	assert.NotNil(t, packets[0].Addr)

	// Check statistics
	stats := receiver.Stats()
	assert.Equal(t, uint64(1), stats.TotalPackets)
	assert.Equal(t, uint64(len(testData)), stats.TotalBytes)
}

func TestBatchReceiverTimeout(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer conn.Close()

	receiver := NewBatchReceiverFromConn(conn, nil)

	start := time.Now()
	packets, err := receiver.RecvBatch(100 * time.Millisecond)
	elapsed := time.Since(start)

	// Should timeout with no error and no packets
	assert.NoError(t, err)
	assert.Nil(t, packets)
	assert.GreaterOrEqual(t, elapsed, 90*time.Millisecond) // Allow some timing slack
}

func TestBatchReceiverAdapter(t *testing.T) {
	// Create server socket
	serverConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer serverConn.Close()

	serverAddr := serverConn.LocalAddr()

	// Create client socket
	clientConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer clientConn.Close()

	// Create adapter
	config := &BatchReceiveConfig{
		InitialBufferSize: 2048,
	}
	adapter := NewBatchReceiverAdapterFromConn(serverConn, config, 500*time.Millisecond)

	// Send test packet
	testData := []byte("Adapter test packet")
	_, err = clientConn.WriteTo(testData, serverAddr)
	require.NoError(t, err)

	// Read via adapter
	pkt, err := adapter.ReadPacket()
	require.NoError(t, err)
	require.NotNil(t, pkt)

	assert.Equal(t, testData, pkt.Data)
}

func TestBatchReceiveStatsTracking(t *testing.T) {
	// Create server and client sockets
	serverConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer serverConn.Close()

	serverAddr := serverConn.LocalAddr()

	clientConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer clientConn.Close()

	receiver := NewBatchReceiverFromConn(serverConn, nil)

	// Send multiple packets
	numPackets := 5
	packetSize := 100

	for i := 0; i < numPackets; i++ {
		data := make([]byte, packetSize)
		for j := range data {
			data[j] = byte(i + j)
		}
		_, err = clientConn.WriteTo(data, serverAddr)
		require.NoError(t, err)
	}

	// Give some time for packets to arrive
	time.Sleep(50 * time.Millisecond)

	// Receive all packets
	received := 0
	for received < numPackets {
		packets, err := receiver.RecvBatch(100 * time.Millisecond)
		if err != nil {
			break
		}
		if len(packets) == 0 {
			break
		}
		received += len(packets)
	}

	// Check statistics
	stats := receiver.Stats()
	assert.Equal(t, uint64(received), stats.TotalPackets)
	assert.Equal(t, uint64(received*packetSize), stats.TotalBytes)
	assert.Equal(t, uint64(received), stats.TotalBatches) // One batch per ReadFrom (no recvmmsg)
}

func TestBatchReceiveStatsBatchSizeHistogram(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer conn.Close()

	receiver := NewBatchReceiverFromConn(conn, nil)

	// Send and receive a packet
	clientConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer clientConn.Close()

	_, err = clientConn.WriteTo([]byte("test"), conn.LocalAddr())
	require.NoError(t, err)

	_, err = receiver.RecvBatch(100 * time.Millisecond)
	require.NoError(t, err)

	// Without recvmmsg, batch size is always 1
	stats := receiver.Stats()
	assert.Equal(t, uint64(1), stats.BatchSizeHist[1])
}

func TestBatchReceiverConcurrentAccess(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer conn.Close()

	receiver := NewBatchReceiverFromConn(conn, nil)

	// Concurrent stats access should be safe
	var statsCount uint64
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = receiver.Stats()
				atomic.AddUint64(&statsCount, 1)
			}
			done <- true
		}()
	}

	// Also do some buffer size changes
	go func() {
		for j := 0; j < 50; j++ {
			receiver.SetBufferSize(1024 + j*10)
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 11; i++ {
		<-done
	}

	assert.Equal(t, uint64(1000), atomic.LoadUint64(&statsCount))
}

func BenchmarkBatchReceiverRecvBatch(b *testing.B) {
	// Create server and client
	serverConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(b, err)
	defer serverConn.Close()

	serverAddr := serverConn.LocalAddr()

	clientConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(b, err)
	defer clientConn.Close()

	receiver := NewBatchReceiverFromConn(serverConn, nil)
	testData := make([]byte, 512)

	// Pre-send packets to avoid startup overhead
	for i := 0; i < 100; i++ {
		_, _ = clientConn.WriteTo(testData, serverAddr)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Send packet
		_, _ = clientConn.WriteTo(testData, serverAddr)
		// Receive packet
		_, _ = receiver.RecvBatch(10 * time.Millisecond)
	}
}

func BenchmarkBatchReceiverStats(b *testing.B) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(b, err)
	defer conn.Close()

	receiver := NewBatchReceiverFromConn(conn, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = receiver.Stats()
	}
}
