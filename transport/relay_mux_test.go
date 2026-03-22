package transport

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConn implements net.Conn for testing RelayMux.
type mockMuxConn struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	mu       sync.Mutex
	closed   bool
	readCh   chan []byte
}

func newMockMuxConn() *mockMuxConn {
	return &mockMuxConn{
		readBuf:  bytes.NewBuffer(nil),
		writeBuf: bytes.NewBuffer(nil),
		readCh:   make(chan []byte, 100),
	}
}

func (m *mockMuxConn) Read(b []byte) (int, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return 0, io.EOF
	}
	m.mu.Unlock()

	// Try reading from channel first (simulated incoming data)
	select {
	case data := <-m.readCh:
		return copy(b, data), nil
	default:
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readBuf.Len() > 0 {
		return m.readBuf.Read(b)
	}

	// Block until data is available or closed
	m.mu.Unlock()
	time.Sleep(10 * time.Millisecond)
	m.mu.Lock()

	if m.closed {
		return 0, io.EOF
	}
	return 0, nil
}

func (m *mockMuxConn) Write(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.writeBuf.Write(b)
}

func (m *mockMuxConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockMuxConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445}
}

func (m *mockMuxConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33446}
}

func (m *mockMuxConn) SetDeadline(_ time.Time) error      { return nil }
func (m *mockMuxConn) SetReadDeadline(_ time.Time) error  { return nil }
func (m *mockMuxConn) SetWriteDeadline(_ time.Time) error { return nil }

func (m *mockMuxConn) InjectData(data []byte) {
	m.readCh <- data
}

func (m *mockMuxConn) GetWritten() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeBuf.Bytes()
}

func TestNewRelayMux(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte
	copy(localKey[:], []byte("test_local_key_123456789012345"))

	mux := NewRelayMux(conn, localKey, nil)
	require.NotNil(t, mux)

	assert.Equal(t, 0, mux.StreamCount())
	assert.Equal(t, DefaultMuxConfig.MaxStreams, mux.config.MaxStreams)

	err := mux.Close()
	assert.NoError(t, err)
}

func TestRelayMuxCustomConfig(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	config := &MuxConfig{
		MaxStreams:       512,
		StreamBufferSize: 64 * 1024,
		IdleTimeout:      10 * time.Minute,
		WriteTimeout:     5 * time.Second,
		MaxFrameSize:     8 * 1024,
	}

	mux := NewRelayMux(conn, localKey, config)
	require.NotNil(t, mux)

	assert.Equal(t, 512, mux.config.MaxStreams)
	assert.Equal(t, 64*1024, mux.config.StreamBufferSize)
	assert.Equal(t, 10*time.Minute, mux.config.IdleTimeout)

	err := mux.Close()
	assert.NoError(t, err)
}

func TestMuxStreamState(t *testing.T) {
	tests := []struct {
		state    StreamState
		expected string
	}{
		{StreamStateOpen, "open"},
		{StreamStateClosed, "closed"},
		{StreamStateError, "error"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			assert.True(t, tc.state >= StreamStateOpen && tc.state <= StreamStateError)
		})
	}
}

func TestMuxPacketTypes(t *testing.T) {
	// Verify packet types don't conflict with relay packet types
	assert.True(t, byte(MuxPacketStreamOpen) >= 0x10)
	assert.True(t, byte(MuxPacketStreamOpenAck) >= 0x10)
	assert.True(t, byte(MuxPacketStreamData) >= 0x10)
	assert.True(t, byte(MuxPacketStreamClose) >= 0x10)
	assert.True(t, byte(MuxPacketStreamCloseAck) >= 0x10)
	assert.True(t, byte(MuxPacketStreamReset) >= 0x10)
}

func TestMuxStreamAddr(t *testing.T) {
	var key [32]byte
	copy(key[:], []byte("test_key_1234567890123456789012"))

	addr := &MuxStreamAddr{
		StreamID: 42,
		Key:      key,
	}

	assert.Equal(t, "mux", addr.Network())
	assert.Contains(t, addr.String(), "mux://")
	assert.Contains(t, addr.String(), "/42")
}

func TestRelayMuxOpenStreamLimitReached(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	config := &MuxConfig{
		MaxStreams:       2,
		StreamBufferSize: 1024,
		IdleTimeout:      5 * time.Minute,
		WriteTimeout:     5 * time.Second,
		MaxFrameSize:     1024,
	}

	mux := NewRelayMux(conn, localKey, config)
	defer mux.Close()

	// Manually add streams to reach limit
	mux.mu.Lock()
	var key1, key2 [32]byte
	key1[0] = 1
	key2[0] = 2
	mux.streams[1] = mux.createStream(1, key1)
	mux.streams[2] = mux.createStream(2, key2)
	mux.mu.Unlock()

	// Try to open another stream
	var key3 [32]byte
	key3[0] = 3
	_, err := mux.OpenStream(context.Background(), key3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum stream limit reached")
}

func TestRelayMuxGetStream(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	var peerKey [32]byte
	copy(peerKey[:], []byte("peer_key_12345678901234567890"))

	// Initially no stream
	stream := mux.GetStream(peerKey)
	assert.Nil(t, stream)

	// Add a stream manually
	mux.mu.Lock()
	newStream := mux.createStream(1, peerKey)
	mux.streams[1] = newStream
	mux.streamsByKey[peerKey] = newStream
	mux.mu.Unlock()

	// Now should find it
	stream = mux.GetStream(peerKey)
	assert.NotNil(t, stream)
	assert.Equal(t, StreamID(1), stream.ID())
}

func TestMuxStreamWrite(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	var peerKey [32]byte
	peerKey[0] = 0xAB

	// Create a stream manually
	stream := mux.createStream(1, peerKey)
	mux.mu.Lock()
	mux.streams[1] = stream
	mux.streamsByKey[peerKey] = stream
	mux.mu.Unlock()

	// Write data
	testData := []byte("hello, multiplexed world!")
	n, err := stream.Write(testData)
	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)

	// Verify frame was written
	written := conn.GetWritten()
	assert.True(t, len(written) > 0)
}

func TestMuxStreamWriteClosed(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	var peerKey [32]byte

	stream := mux.createStream(1, peerKey)
	stream.state.Store(int32(StreamStateClosed))

	_, err := stream.Write([]byte("test"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stream not open")
}

func TestMuxStreamAddresses(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte
	copy(localKey[:], []byte("local_key_123456789012345678"))

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	var peerKey [32]byte
	copy(peerKey[:], []byte("peer_key_1234567890123456789"))

	stream := mux.createStream(42, peerKey)

	localAddr := stream.LocalAddr()
	assert.Equal(t, "mux", localAddr.Network())
	assert.Contains(t, localAddr.String(), "/42")

	remoteAddr := stream.RemoteAddr()
	assert.Equal(t, "mux", remoteAddr.Network())
	assert.Contains(t, remoteAddr.String(), "/42")
}

func TestMuxStreamPeerKey(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	var peerKey [32]byte
	copy(peerKey[:], []byte("unique_peer_key_12345678901234"))

	stream := mux.createStream(1, peerKey)
	assert.Equal(t, peerKey, stream.PeerKey())
}

func TestRelayMuxCloseMultipleTimes(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)

	// Close multiple times should not panic
	err1 := mux.Close()
	assert.NoError(t, err1)

	err2 := mux.Close()
	assert.NoError(t, err2)
}

func TestRelayMuxStreamCount(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	assert.Equal(t, 0, mux.StreamCount())

	// Add streams
	var key1, key2, key3 [32]byte
	key1[0] = 1
	key2[0] = 2
	key3[0] = 3

	mux.mu.Lock()
	mux.streams[1] = mux.createStream(1, key1)
	mux.streams[2] = mux.createStream(2, key2)
	mux.streams[3] = mux.createStream(3, key3)
	mux.mu.Unlock()

	assert.Equal(t, 3, mux.StreamCount())
}

func TestRelayMuxStats(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	// Write some frames
	var peerKey [32]byte
	stream := mux.createStream(1, peerKey)
	mux.mu.Lock()
	mux.streams[1] = stream
	mux.mu.Unlock()

	stream.Write([]byte("test data 1"))
	stream.Write([]byte("test data 2"))

	opened, closed, bytesSent, bytesRecv, framesSent, framesRecv, errs := mux.GetStatsSnapshot()
	assert.True(t, framesSent >= 2)
	assert.True(t, bytesSent > 0)
	assert.Equal(t, uint64(0), bytesRecv) // No receives in this test
	assert.Equal(t, uint64(0), framesRecv)
	assert.Equal(t, uint64(0), errs)
	_ = opened
	_ = closed
}

func TestDefaultMuxConfig(t *testing.T) {
	assert.Equal(t, 1024, DefaultMuxConfig.MaxStreams)
	assert.Equal(t, 32*1024, DefaultMuxConfig.StreamBufferSize)
	assert.Equal(t, 5*time.Minute, DefaultMuxConfig.IdleTimeout)
	assert.Equal(t, 10*time.Second, DefaultMuxConfig.WriteTimeout)
	assert.Equal(t, 16*1024, DefaultMuxConfig.MaxFrameSize)
}

func TestMuxStreamCloseInternal(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	var peerKey [32]byte
	stream := mux.createStream(1, peerKey)

	assert.Equal(t, StreamStateOpen, stream.State())

	// Close internally
	stream.closeInternal()

	assert.Equal(t, StreamStateClosed, stream.State())

	// Closing again should not panic
	stream.closeInternal()
	assert.Equal(t, StreamStateClosed, stream.State())
}

func TestMuxStreamReadWithPartialData(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	var peerKey [32]byte
	stream := mux.createStream(1, peerKey)

	// Inject data into read buffer
	testData := []byte("hello, this is a longer message for testing")
	stream.readBuf <- testData

	// Read with small buffer
	buf := make([]byte, 10)
	n, err := stream.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 10, n)
	assert.Equal(t, "hello, thi", string(buf))

	// Read remaining partial data
	buf2 := make([]byte, 100)
	n2, err := stream.Read(buf2)
	assert.NoError(t, err)
	assert.Equal(t, len(testData)-10, n2)
	assert.Equal(t, "s is a longer message for testing", string(buf2[:n2]))
}

func TestMuxStreamReadFromClosedStream(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	var peerKey [32]byte
	stream := mux.createStream(1, peerKey)

	// Close the stream
	stream.closeInternal()

	// Read should return EOF
	buf := make([]byte, 100)
	_, err := stream.Read(buf)
	assert.Equal(t, io.EOF, err)
}

func TestMuxStreamReadWithTimeout(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	var peerKey [32]byte
	stream := mux.createStream(1, peerKey)

	// Try to read with short timeout - should timeout
	buf := make([]byte, 100)
	_, err := stream.ReadWithTimeout(buf, 50*time.Millisecond)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestMuxStreamReadWithTimeoutSuccess(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	var peerKey [32]byte
	stream := mux.createStream(1, peerKey)

	// Inject data
	testData := []byte("timeout test data")
	stream.readBuf <- testData

	// Read with timeout - should succeed
	buf := make([]byte, 100)
	n, err := stream.ReadWithTimeout(buf, 1*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)
	assert.Equal(t, testData, buf[:n])
}

func TestProcessFrameUnknownType(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	// Process frame with unknown type - should not panic
	unknownFrame := []byte{0xFF, 0x01, 0x02, 0x03}
	mux.processFrame(unknownFrame)
}

func TestProcessFrameEmpty(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	// Process empty frame - should not panic
	mux.processFrame([]byte{})
}

func TestHandleStreamDataNoStream(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	// Build a stream data frame for non-existent stream
	frame := []byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x42, 0x43} // StreamID 1, data "ABC"
	mux.handleStreamData(frame)
	// Should not panic, just ignore
}

func TestHandleStreamOpenMaxStreams(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	config := &MuxConfig{
		MaxStreams:       1,
		StreamBufferSize: 1024,
		IdleTimeout:      5 * time.Minute,
		WriteTimeout:     5 * time.Second,
		MaxFrameSize:     1024,
	}

	mux := NewRelayMux(conn, localKey, config)
	defer mux.Close()

	// Add one stream to reach limit
	var key1 [32]byte
	key1[0] = 1
	mux.mu.Lock()
	mux.streams[1] = mux.createStream(1, key1)
	mux.mu.Unlock()

	// Try to open another via handleStreamOpen
	var peerKey [32]byte
	peerKey[0] = 2
	frame := make([]byte, 4+32+32)
	frame[3] = 2 // StreamID 2
	copy(frame[4:36], peerKey[:])
	copy(frame[36:68], localKey[:])

	mux.handleStreamOpen(frame)

	// Should have sent a reset, still only 1 stream
	assert.Equal(t, 1, mux.StreamCount())
}

func TestWriteFrameTooLarge(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	config := &MuxConfig{
		MaxStreams:       100,
		StreamBufferSize: 1024,
		IdleTimeout:      5 * time.Minute,
		WriteTimeout:     5 * time.Second,
		MaxFrameSize:     100, // Small max
	}

	mux := NewRelayMux(conn, localKey, config)
	defer mux.Close()

	// Try to write oversized frame
	largeData := make([]byte, 200)
	err := mux.writeFrame(largeData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestWriteFrameClosedMux(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	mux.Close()

	err := mux.writeFrame([]byte{0x01, 0x02, 0x03})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestOpenStreamOnClosedMux(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	mux.Close()

	var peerKey [32]byte
	_, err := mux.OpenStream(context.Background(), peerKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestOpenStreamExistingStream(t *testing.T) {
	conn := newMockMuxConn()
	var localKey [32]byte

	mux := NewRelayMux(conn, localKey, nil)
	defer mux.Close()

	var peerKey [32]byte
	peerKey[0] = 0xAB

	// Pre-add stream
	existingStream := mux.createStream(1, peerKey)
	mux.mu.Lock()
	mux.streams[1] = existingStream
	mux.streamsByKey[peerKey] = existingStream
	mux.mu.Unlock()

	// Try to open again - should return existing
	stream, err := mux.OpenStream(context.Background(), peerKey)
	assert.NoError(t, err)
	assert.Equal(t, existingStream, stream)
}
