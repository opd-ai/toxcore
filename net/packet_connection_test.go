package net

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToxPacketConnectionReadWrite tests the Read and Write methods of ToxPacketConnection
func TestToxPacketConnectionReadWrite(t *testing.T) {
	// Create a listener and establish a connection
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	listener, err := NewToxPacketListener(localAddr, ":0")
	require.NoError(t, err)
	defer listener.Close()

	// Create a UDP conn to send data to the listener
	udpConn, err := net.ListenPacket("udp", ":0")
	require.NoError(t, err)
	defer udpConn.Close()

	// Get the listener's UDP address
	listenerUDPAddr := listener.packetConn.LocalAddr()

	// Send data to trigger connection creation
	testData := []byte("Hello, ToxPacketConnection!")
	_, err = udpConn.WriteTo(testData, listenerUDPAddr)
	require.NoError(t, err)

	// Accept the connection
	acceptDone := make(chan struct{})
	var conn net.Conn
	go func() {
		conn, err = listener.Accept()
		close(acceptDone)
	}()

	select {
	case <-acceptDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Accept timed out")
	}
	defer conn.Close()

	// Read the data
	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, err := conn.Read(buffer)
	require.NoError(t, err)
	assert.Equal(t, testData, buffer[:n])
}

// TestToxPacketConnectionReadClosed tests Read on a closed connection
func TestToxPacketConnectionReadClosed(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	ctx, cancel := context.WithCancel(context.Background())
	conn := &ToxPacketConnection{
		localAddr:   localAddr,
		remoteAddr:  &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
		readBuffer:  make(chan []byte, 100),
		writeBuffer: make(chan packetToSend, 100),
		ctx:         ctx,
		cancel:      cancel,
		closed:      true, // Mark as closed
	}

	buffer := make([]byte, 1024)
	_, err = conn.Read(buffer)
	assert.Error(t, err)

	var toxErr *ToxNetError
	assert.ErrorAs(t, err, &toxErr)
	assert.Equal(t, "read", toxErr.Op)
}

// TestToxPacketConnectionWriteClosed tests Write on a closed connection
func TestToxPacketConnectionWriteClosed(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	ctx, cancel := context.WithCancel(context.Background())
	conn := &ToxPacketConnection{
		localAddr:   localAddr,
		remoteAddr:  &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
		readBuffer:  make(chan []byte, 100),
		writeBuffer: make(chan packetToSend, 100),
		ctx:         ctx,
		cancel:      cancel,
		closed:      true, // Mark as closed
	}

	_, err = conn.Write([]byte("test"))
	assert.Error(t, err)

	var toxErr *ToxNetError
	assert.ErrorAs(t, err, &toxErr)
	assert.Equal(t, "write", toxErr.Op)
}

// TestToxPacketConnectionReadTimeout tests Read with a timeout
func TestToxPacketConnectionReadTimeout(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := &ToxPacketConnection{
		localAddr:   localAddr,
		remoteAddr:  &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
		readBuffer:  make(chan []byte, 100), // Empty buffer
		writeBuffer: make(chan packetToSend, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Set a short deadline
	conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))

	buffer := make([]byte, 1024)
	start := time.Now()
	_, err = conn.Read(buffer)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.GreaterOrEqual(t, elapsed, 40*time.Millisecond, "Timeout should wait for deadline")
	assert.Less(t, elapsed, 200*time.Millisecond, "Timeout should not take too long")
}

// TestToxPacketConnectionWriteBufferFull tests Write when buffer is full
func TestToxPacketConnectionWriteBufferFull(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create connection with tiny buffer
	conn := &ToxPacketConnection{
		localAddr:   localAddr,
		remoteAddr:  &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
		readBuffer:  make(chan []byte, 1),
		writeBuffer: make(chan packetToSend, 1), // Tiny buffer
		ctx:         ctx,
		cancel:      cancel,
	}

	// Fill the buffer
	_, err = conn.Write([]byte("first"))
	require.NoError(t, err)

	// Second write should fail with buffer full
	_, err = conn.Write([]byte("second"))
	assert.Error(t, err)

	var toxErr *ToxNetError
	assert.ErrorAs(t, err, &toxErr)
	assert.Equal(t, "write", toxErr.Op)
}

// TestToxPacketConnectionWriteDeadlineExpired tests Write after deadline has passed
func TestToxPacketConnectionWriteDeadlineExpired(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := &ToxPacketConnection{
		localAddr:   localAddr,
		remoteAddr:  &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
		readBuffer:  make(chan []byte, 100),
		writeBuffer: make(chan packetToSend, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Set deadline in the past
	conn.SetWriteDeadline(time.Now().Add(-1 * time.Second))

	_, err = conn.Write([]byte("test"))
	assert.Error(t, err)

	var toxErr *ToxNetError
	assert.ErrorAs(t, err, &toxErr)
	assert.Equal(t, "write", toxErr.Op)
}

// TestToxPacketConnectionLocalRemoteAddr tests LocalAddr and RemoteAddr
func TestToxPacketConnectionLocalRemoteAddr(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)
	remoteAddr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 100), Port: 12345}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := &ToxPacketConnection{
		localAddr:   localAddr,
		remoteAddr:  remoteAddr,
		readBuffer:  make(chan []byte, 100),
		writeBuffer: make(chan packetToSend, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	assert.Equal(t, localAddr, conn.LocalAddr())
	assert.Equal(t, remoteAddr, conn.RemoteAddr())
}

// TestToxPacketConnectionDeadlineMethods tests all deadline setting methods
func TestToxPacketConnectionDeadlineMethods(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := &ToxPacketConnection{
		localAddr:   localAddr,
		remoteAddr:  &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
		readBuffer:  make(chan []byte, 100),
		writeBuffer: make(chan packetToSend, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	futureTime := time.Now().Add(10 * time.Second)

	// Test SetDeadline
	err = conn.SetDeadline(futureTime)
	assert.NoError(t, err)
	assert.Equal(t, futureTime, conn.readDeadline)
	assert.Equal(t, futureTime, conn.writeDeadline)

	// Test SetReadDeadline
	newReadDeadline := time.Now().Add(5 * time.Second)
	err = conn.SetReadDeadline(newReadDeadline)
	assert.NoError(t, err)
	assert.Equal(t, newReadDeadline, conn.readDeadline)
	assert.Equal(t, futureTime, conn.writeDeadline) // Write deadline unchanged

	// Test SetWriteDeadline
	newWriteDeadline := time.Now().Add(15 * time.Second)
	err = conn.SetWriteDeadline(newWriteDeadline)
	assert.NoError(t, err)
	assert.Equal(t, newReadDeadline, conn.readDeadline) // Read deadline unchanged
	assert.Equal(t, newWriteDeadline, conn.writeDeadline)
}

// TestToxPacketConnectionCloseIdempotent tests that Close is idempotent
func TestToxPacketConnectionCloseIdempotent(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	// Create a minimal listener for the connection
	listener, err := NewToxPacketListener(localAddr, ":0")
	require.NoError(t, err)
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}

	conn := &ToxPacketConnection{
		listener:    listener,
		localAddr:   localAddr,
		remoteAddr:  remoteAddr,
		readBuffer:  make(chan []byte, 100),
		writeBuffer: make(chan packetToSend, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Add to listener's connection map
	listener.connMu.Lock()
	listener.connections[remoteAddr.String()] = conn
	listener.connMu.Unlock()

	// First close should succeed
	err = conn.Close()
	assert.NoError(t, err)

	// Second close should also succeed (idempotent)
	err = conn.Close()
	assert.NoError(t, err)
}

// TestToxPacketListenerInternalHelpers tests the internal helper functions
func TestToxPacketListenerInternalHelpers(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	listener, err := NewToxPacketListener(localAddr, ":0")
	require.NoError(t, err)
	defer listener.Close()

	// Test isListenerClosed when open
	assert.False(t, listener.isListenerClosed())

	// Test isListenerClosed when closed
	listener.Close()
	assert.True(t, listener.isListenerClosed())
}

// TestToxPacketListenerIsTimeoutError tests timeout error detection
func TestToxPacketListenerIsTimeoutError(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	listener, err := NewToxPacketListener(localAddr, ":0")
	require.NoError(t, err)
	defer listener.Close()

	// Create a mock timeout error
	mockTimeoutErr := &mockNetError{isTimeout: true}
	assert.True(t, listener.isTimeoutError(mockTimeoutErr))

	// Non-timeout error should return false
	normalErr := &mockNetError{isTimeout: false}
	assert.False(t, listener.isTimeoutError(normalErr))
}

// mockNetError implements net.Error for testing
type mockNetError struct {
	isTimeout   bool
	isTemporary bool
}

func (e *mockNetError) Error() string   { return "mock error" }
func (e *mockNetError) Timeout() bool   { return e.isTimeout }
func (e *mockNetError) Temporary() bool { return e.isTemporary }

// TestToxPacketListenerHandlePacket tests packet handling for new and existing connections
func TestToxPacketListenerHandlePacket(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	listener, err := NewToxPacketListener(localAddr, ":0")
	require.NoError(t, err)
	defer listener.Close()

	remoteAddr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 100), Port: 5555}
	testData := []byte("test packet data")

	// First packet should create a new connection
	listener.handlePacket(testData, remoteAddr)

	// Wait for connection to be created
	var conn net.Conn
	select {
	case conn = <-listener.acceptCh:
		defer conn.Close()
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected new connection to be created")
	}

	// Verify connection exists in map
	listener.connMu.RLock()
	_, exists := listener.connections[remoteAddr.String()]
	listener.connMu.RUnlock()
	assert.True(t, exists, "Connection should be in map")

	// Second packet to same address should use existing connection
	testData2 := []byte("second packet")
	listener.handlePacket(testData2, remoteAddr)

	// Read the data from the connection
	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	n, err := conn.Read(buffer)
	require.NoError(t, err)
	assert.Equal(t, testData, buffer[:n])

	// Read the second packet
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	n, err = conn.Read(buffer)
	require.NoError(t, err)
	assert.Equal(t, testData2, buffer[:n])
}

// TestToxPacketConnectionProcessWrites tests the write processing goroutine
func TestToxPacketConnectionProcessWrites(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	// Create two listeners for bidirectional communication
	listener, err := NewToxPacketListener(localAddr, ":0")
	require.NoError(t, err)
	defer listener.Close()

	// Create a UDP conn to receive writes
	udpConn, err := net.ListenPacket("udp", ":0")
	require.NoError(t, err)
	defer udpConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	destAddr := udpConn.LocalAddr()
	conn := &ToxPacketConnection{
		listener:    listener,
		localAddr:   localAddr,
		remoteAddr:  destAddr,
		readBuffer:  make(chan []byte, 100),
		writeBuffer: make(chan packetToSend, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start write processing
	go conn.processWrites()

	// Write data
	testData := []byte("write test data")
	_, err = conn.Write(testData)
	require.NoError(t, err)

	// Read from UDP conn to verify data was sent
	buffer := make([]byte, 1024)
	udpConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	n, _, err := udpConn.ReadFrom(buffer)
	require.NoError(t, err)
	assert.Equal(t, testData, buffer[:n])
}

// TestToxPacketListenerHandleReadError tests error handling during read
func TestToxPacketListenerHandleReadError(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	listener, err := NewToxPacketListener(localAddr, ":0")
	require.NoError(t, err)

	// Test timeout error handling (should continue)
	timeoutErr := &mockNetError{isTimeout: true}
	shouldStop := listener.handleReadError(timeoutErr)
	assert.False(t, shouldStop, "Should continue on timeout error")

	// Test non-timeout error when listener is open (should continue)
	normalErr := &mockNetError{isTimeout: false}
	shouldStop = listener.handleReadError(normalErr)
	assert.False(t, shouldStop, "Should continue on regular error when open")

	// Close listener and test error handling
	listener.Close()
	shouldStop = listener.handleReadError(normalErr)
	assert.True(t, shouldStop, "Should stop when listener is closed")
}

// TestToxPacketConnectionReadContextCancelled tests Read when context is cancelled
func TestToxPacketConnectionReadContextCancelled(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	ctx, cancel := context.WithCancel(context.Background())
	conn := &ToxPacketConnection{
		localAddr:   localAddr,
		remoteAddr:  &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
		readBuffer:  make(chan []byte, 100),
		writeBuffer: make(chan packetToSend, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Cancel context immediately
	cancel()

	buffer := make([]byte, 1024)
	_, err = conn.Read(buffer)
	assert.Error(t, err)

	var toxErr *ToxNetError
	assert.ErrorAs(t, err, &toxErr)
	assert.Equal(t, "read", toxErr.Op)
}

// TestToxPacketConnectionConcurrentAccess tests concurrent Read/Write operations
func TestToxPacketConnectionConcurrentAccess(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	ctx, cancel := context.WithCancel(context.Background())

	// Create a standalone connection without listener to avoid mutex issues
	conn := &ToxPacketConnection{
		localAddr:   localAddr,
		remoteAddr:  &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
		readBuffer:  make(chan []byte, 100),
		writeBuffer: make(chan packetToSend, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Pre-fill read buffer
	for i := 0; i < 10; i++ {
		conn.readBuffer <- []byte("test data")
	}

	// Concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buffer := make([]byte, 1024)
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			conn.Read(buffer)
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn.Write([]byte("concurrent write"))
		}()
	}

	// Concurrent deadline setting
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn.SetDeadline(time.Now().Add(1 * time.Second))
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		}()
	}

	// Wait for all goroutines to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - all concurrent operations completed without race conditions
		cancel()
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("Concurrent operations timed out")
	}
}
