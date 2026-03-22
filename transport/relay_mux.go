// Package transport implements network transport for the Tox protocol.
//
// This file implements connection multiplexing for TCP relay mode,
// allowing multiple concurrent peer streams over a single relay connection.
package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// StreamID identifies a multiplexed stream within a relay connection.
type StreamID uint32

// StreamState represents the current state of a multiplexed stream.
type StreamState uint8

const (
	// StreamStateOpen indicates the stream is open and ready.
	StreamStateOpen StreamState = iota
	// StreamStateClosed indicates the stream has been closed.
	StreamStateClosed
	// StreamStateError indicates the stream encountered an error.
	StreamStateError
)

// MuxPacketType identifies multiplexed packet types within the relay protocol.
type MuxPacketType uint8

const (
	// MuxPacketStreamOpen requests opening a new stream.
	MuxPacketStreamOpen MuxPacketType = 0x10
	// MuxPacketStreamOpenAck acknowledges stream opening.
	MuxPacketStreamOpenAck MuxPacketType = 0x11
	// MuxPacketStreamData carries stream data.
	MuxPacketStreamData MuxPacketType = 0x12
	// MuxPacketStreamClose requests closing a stream.
	MuxPacketStreamClose MuxPacketType = 0x13
	// MuxPacketStreamCloseAck acknowledges stream closure.
	MuxPacketStreamCloseAck MuxPacketType = 0x14
	// MuxPacketStreamReset forcibly resets a stream.
	MuxPacketStreamReset MuxPacketType = 0x15
)

// DefaultMuxConfig provides default multiplexer configuration values.
var DefaultMuxConfig = MuxConfig{
	MaxStreams:       1024,
	StreamBufferSize: 32 * 1024, // 32KB per stream
	IdleTimeout:      5 * time.Minute,
	WriteTimeout:     10 * time.Second,
	MaxFrameSize:     16 * 1024, // 16KB max frame
}

// MuxConfig holds configuration for the connection multiplexer.
type MuxConfig struct {
	// MaxStreams is the maximum number of concurrent streams.
	MaxStreams int
	// StreamBufferSize is the read buffer size per stream in bytes.
	StreamBufferSize int
	// IdleTimeout is how long an idle stream survives before cleanup.
	IdleTimeout time.Duration
	// WriteTimeout is the timeout for write operations.
	WriteTimeout time.Duration
	// MaxFrameSize is the maximum size of a single frame.
	MaxFrameSize int
}

// MuxStream represents a single multiplexed stream over the relay connection.
type MuxStream struct {
	id          StreamID
	mux         *RelayMux
	state       atomic.Int32
	readBuf     chan []byte
	readPartial []byte
	peerKey     [32]byte
	localKey    [32]byte
	mu          sync.Mutex
	lastActive  atomic.Int64
	closedCh    chan struct{}
	closeOnce   sync.Once
}

// RelayMux implements connection multiplexing over a single TCP relay connection.
// It allows multiple logical streams to share one physical connection.
//
//export ToxRelayMux
type RelayMux struct {
	conn         net.Conn
	streams      map[StreamID]*MuxStream
	streamsByKey map[[32]byte]*MuxStream
	nextStreamID atomic.Uint32
	mu           sync.RWMutex
	config       MuxConfig
	localKey     [32]byte
	ctx          context.Context
	cancel       context.CancelFunc
	closed       atomic.Bool
	stats        MuxStats
}

// MuxStats tracks multiplexer statistics.
type MuxStats struct {
	StreamsOpened  atomic.Uint64
	StreamsClosed  atomic.Uint64
	BytesSent      atomic.Uint64
	BytesReceived  atomic.Uint64
	FramesSent     atomic.Uint64
	FramesReceived atomic.Uint64
	Errors         atomic.Uint64
}

// NewRelayMux creates a new connection multiplexer over the given connection.
//
//export ToxNewRelayMux
func NewRelayMux(conn net.Conn, localKey [32]byte, config *MuxConfig) *RelayMux {
	if config == nil {
		config = &DefaultMuxConfig
	}

	ctx, cancel := context.WithCancel(context.Background())

	mux := &RelayMux{
		conn:         conn,
		streams:      make(map[StreamID]*MuxStream),
		streamsByKey: make(map[[32]byte]*MuxStream),
		config:       *config,
		localKey:     localKey,
		ctx:          ctx,
		cancel:       cancel,
	}

	logrus.WithFields(logrus.Fields{
		"function":    "NewRelayMux",
		"local_addr":  conn.LocalAddr().String(),
		"max_streams": config.MaxStreams,
	}).Info("Created relay multiplexer")

	go mux.readLoop()
	go mux.idleCleanupLoop()

	return mux
}

// OpenStream opens a new multiplexed stream to the specified peer.
//
//export ToxMuxOpenStream
func (m *RelayMux) OpenStream(ctx context.Context, peerKey [32]byte) (*MuxStream, error) {
	if m.closed.Load() {
		return nil, errors.New("multiplexer is closed")
	}

	m.mu.Lock()
	// Check if we already have a stream to this peer
	if existing, ok := m.streamsByKey[peerKey]; ok {
		m.mu.Unlock()
		return existing, nil
	}

	// Check stream limit
	if len(m.streams) >= m.config.MaxStreams {
		m.mu.Unlock()
		return nil, errors.New("maximum stream limit reached")
	}

	streamID := StreamID(m.nextStreamID.Add(1))
	stream := m.createStream(streamID, peerKey)
	m.streams[streamID] = stream
	m.streamsByKey[peerKey] = stream
	m.mu.Unlock()

	// Send stream open request
	if err := m.sendStreamOpen(streamID, peerKey); err != nil {
		m.removeStream(streamID, peerKey)
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}

	m.stats.StreamsOpened.Add(1)

	logrus.WithFields(logrus.Fields{
		"function":  "OpenStream",
		"stream_id": streamID,
		"peer_key":  fmt.Sprintf("%x", peerKey[:8]),
	}).Debug("Opened multiplexed stream")

	return stream, nil
}

// createStream initializes a new MuxStream.
func (m *RelayMux) createStream(id StreamID, peerKey [32]byte) *MuxStream {
	stream := &MuxStream{
		id:       id,
		mux:      m,
		readBuf:  make(chan []byte, m.config.StreamBufferSize/m.config.MaxFrameSize+1),
		peerKey:  peerKey,
		localKey: m.localKey,
		closedCh: make(chan struct{}),
	}
	stream.state.Store(int32(StreamStateOpen))
	stream.lastActive.Store(time.Now().UnixNano())
	return stream
}

// sendStreamOpen sends a stream open request to the relay.
func (m *RelayMux) sendStreamOpen(id StreamID, peerKey [32]byte) error {
	// Frame format: [MuxPacketStreamOpen:1][StreamID:4][PeerKey:32][LocalKey:32]
	frame := make([]byte, 1+4+32+32)
	frame[0] = byte(MuxPacketStreamOpen)
	frame[1] = byte(id >> 24)
	frame[2] = byte(id >> 16)
	frame[3] = byte(id >> 8)
	frame[4] = byte(id)
	copy(frame[5:37], peerKey[:])
	copy(frame[37:69], m.localKey[:])

	return m.writeFrame(frame)
}

// writeFrame writes a complete frame to the connection with length prefix.
func (m *RelayMux) writeFrame(data []byte) error {
	if m.closed.Load() {
		return errors.New("multiplexer is closed")
	}

	if len(data) > m.config.MaxFrameSize {
		return fmt.Errorf("frame size %d exceeds maximum %d", len(data), m.config.MaxFrameSize)
	}

	// Length-prefix the frame: [Length:4][Data:...]
	frame := make([]byte, 4+len(data))
	frame[0] = byte(len(data) >> 24)
	frame[1] = byte(len(data) >> 16)
	frame[2] = byte(len(data) >> 8)
	frame[3] = byte(len(data))
	copy(frame[4:], data)

	if err := m.conn.SetWriteDeadline(time.Now().Add(m.config.WriteTimeout)); err != nil {
		return err
	}
	defer m.conn.SetWriteDeadline(time.Time{})

	_, err := m.conn.Write(frame)
	if err != nil {
		m.stats.Errors.Add(1)
		return err
	}

	m.stats.FramesSent.Add(1)
	m.stats.BytesSent.Add(uint64(len(frame)))
	return nil
}

// readLoop continuously reads frames from the connection.
func (m *RelayMux) readLoop() {
	header := make([]byte, 4)
	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		// Read frame length
		_, err := io.ReadFull(m.conn, header)
		if err != nil {
			if !errors.Is(err, io.EOF) && !m.closed.Load() {
				logrus.WithFields(logrus.Fields{
					"function": "readLoop",
					"error":    err.Error(),
				}).Warn("Mux read error")
			}
			m.closeAllStreams()
			return
		}

		length := (uint32(header[0]) << 24) |
			(uint32(header[1]) << 16) |
			(uint32(header[2]) << 8) |
			uint32(header[3])

		if length > uint32(m.config.MaxFrameSize) {
			logrus.WithFields(logrus.Fields{
				"function": "readLoop",
				"length":   length,
				"max":      m.config.MaxFrameSize,
			}).Warn("Frame too large")
			continue
		}

		// Read frame data
		data := make([]byte, length)
		if _, err := io.ReadFull(m.conn, data); err != nil {
			if !errors.Is(err, io.EOF) && !m.closed.Load() {
				logrus.WithFields(logrus.Fields{
					"function": "readLoop",
					"error":    err.Error(),
				}).Warn("Mux frame read error")
			}
			m.closeAllStreams()
			return
		}

		m.stats.FramesReceived.Add(1)
		m.stats.BytesReceived.Add(uint64(4 + length))

		m.processFrame(data)
	}
}

// processFrame handles an incoming multiplexed frame.
func (m *RelayMux) processFrame(data []byte) {
	if len(data) < 1 {
		return
	}

	packetType := MuxPacketType(data[0])

	switch packetType {
	case MuxPacketStreamOpen:
		m.handleStreamOpen(data[1:])
	case MuxPacketStreamOpenAck:
		m.handleStreamOpenAck(data[1:])
	case MuxPacketStreamData:
		m.handleStreamData(data[1:])
	case MuxPacketStreamClose:
		m.handleStreamClose(data[1:])
	case MuxPacketStreamCloseAck:
		m.handleStreamCloseAck(data[1:])
	case MuxPacketStreamReset:
		m.handleStreamReset(data[1:])
	default:
		logrus.WithFields(logrus.Fields{
			"function":    "processFrame",
			"packet_type": packetType,
		}).Debug("Unknown mux packet type")
	}
}

// handleStreamOpen processes an incoming stream open request.
func (m *RelayMux) handleStreamOpen(data []byte) {
	if len(data) < 4+32+32 {
		return
	}

	streamID := StreamID(
		(uint32(data[0]) << 24) |
			(uint32(data[1]) << 16) |
			(uint32(data[2]) << 8) |
			uint32(data[3]))

	var peerKey [32]byte
	copy(peerKey[:], data[4:36])

	m.mu.Lock()
	if len(m.streams) >= m.config.MaxStreams {
		m.mu.Unlock()
		// Send reset
		m.sendStreamReset(streamID)
		return
	}

	stream := m.createStream(streamID, peerKey)
	m.streams[streamID] = stream
	m.streamsByKey[peerKey] = stream
	m.mu.Unlock()

	// Send acknowledgment
	m.sendStreamOpenAck(streamID)
	m.stats.StreamsOpened.Add(1)

	logrus.WithFields(logrus.Fields{
		"function":  "handleStreamOpen",
		"stream_id": streamID,
		"peer_key":  fmt.Sprintf("%x", peerKey[:8]),
	}).Debug("Accepted incoming stream")
}

// handleStreamOpenAck processes a stream open acknowledgment.
func (m *RelayMux) handleStreamOpenAck(data []byte) {
	if len(data) < 4 {
		return
	}

	streamID := StreamID(
		(uint32(data[0]) << 24) |
			(uint32(data[1]) << 16) |
			(uint32(data[2]) << 8) |
			uint32(data[3]))

	m.mu.RLock()
	stream, ok := m.streams[streamID]
	m.mu.RUnlock()

	if ok {
		stream.lastActive.Store(time.Now().UnixNano())
		logrus.WithFields(logrus.Fields{
			"function":  "handleStreamOpenAck",
			"stream_id": streamID,
		}).Debug("Stream open acknowledged")
	}
}

// handleStreamData processes incoming stream data.
func (m *RelayMux) handleStreamData(data []byte) {
	if len(data) < 4 {
		return
	}

	streamID := StreamID(
		(uint32(data[0]) << 24) |
			(uint32(data[1]) << 16) |
			(uint32(data[2]) << 8) |
			uint32(data[3]))

	m.mu.RLock()
	stream, ok := m.streams[streamID]
	m.mu.RUnlock()

	if !ok || stream.State() != StreamStateOpen {
		return
	}

	payload := data[4:]
	stream.lastActive.Store(time.Now().UnixNano())

	select {
	case stream.readBuf <- payload:
	default:
		// Buffer full, drop packet
		logrus.WithFields(logrus.Fields{
			"function":  "handleStreamData",
			"stream_id": streamID,
		}).Debug("Stream buffer full, dropping data")
	}
}

// handleStreamClose processes a stream close request.
func (m *RelayMux) handleStreamClose(data []byte) {
	if len(data) < 4 {
		return
	}

	streamID := StreamID(
		(uint32(data[0]) << 24) |
			(uint32(data[1]) << 16) |
			(uint32(data[2]) << 8) |
			uint32(data[3]))

	m.mu.RLock()
	stream, ok := m.streams[streamID]
	m.mu.RUnlock()

	if ok {
		stream.closeInternal()
		m.sendStreamCloseAck(streamID)

		m.mu.Lock()
		delete(m.streams, streamID)
		delete(m.streamsByKey, stream.peerKey)
		m.mu.Unlock()

		m.stats.StreamsClosed.Add(1)
	}
}

// handleStreamCloseAck processes a stream close acknowledgment.
func (m *RelayMux) handleStreamCloseAck(data []byte) {
	if len(data) < 4 {
		return
	}

	streamID := StreamID(
		(uint32(data[0]) << 24) |
			(uint32(data[1]) << 16) |
			(uint32(data[2]) << 8) |
			uint32(data[3]))

	m.mu.Lock()
	if stream, ok := m.streams[streamID]; ok {
		stream.closeInternal()
		delete(m.streams, streamID)
		delete(m.streamsByKey, stream.peerKey)
	}
	m.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"function":  "handleStreamCloseAck",
		"stream_id": streamID,
	}).Debug("Stream close acknowledged")
}

// handleStreamReset processes a stream reset.
func (m *RelayMux) handleStreamReset(data []byte) {
	if len(data) < 4 {
		return
	}

	streamID := StreamID(
		(uint32(data[0]) << 24) |
			(uint32(data[1]) << 16) |
			(uint32(data[2]) << 8) |
			uint32(data[3]))

	m.mu.Lock()
	if stream, ok := m.streams[streamID]; ok {
		stream.state.Store(int32(StreamStateError))
		stream.closeInternal()
		delete(m.streams, streamID)
		delete(m.streamsByKey, stream.peerKey)
	}
	m.mu.Unlock()

	m.stats.Errors.Add(1)

	logrus.WithFields(logrus.Fields{
		"function":  "handleStreamReset",
		"stream_id": streamID,
	}).Debug("Stream reset received")
}

// sendStreamOpenAck sends a stream open acknowledgment.
func (m *RelayMux) sendStreamOpenAck(id StreamID) error {
	frame := make([]byte, 5)
	frame[0] = byte(MuxPacketStreamOpenAck)
	frame[1] = byte(id >> 24)
	frame[2] = byte(id >> 16)
	frame[3] = byte(id >> 8)
	frame[4] = byte(id)
	return m.writeFrame(frame)
}

// sendStreamClose sends a stream close request.
func (m *RelayMux) sendStreamClose(id StreamID) error {
	frame := make([]byte, 5)
	frame[0] = byte(MuxPacketStreamClose)
	frame[1] = byte(id >> 24)
	frame[2] = byte(id >> 16)
	frame[3] = byte(id >> 8)
	frame[4] = byte(id)
	return m.writeFrame(frame)
}

// sendStreamCloseAck sends a stream close acknowledgment.
func (m *RelayMux) sendStreamCloseAck(id StreamID) error {
	frame := make([]byte, 5)
	frame[0] = byte(MuxPacketStreamCloseAck)
	frame[1] = byte(id >> 24)
	frame[2] = byte(id >> 16)
	frame[3] = byte(id >> 8)
	frame[4] = byte(id)
	return m.writeFrame(frame)
}

// sendStreamReset sends a stream reset.
func (m *RelayMux) sendStreamReset(id StreamID) error {
	frame := make([]byte, 5)
	frame[0] = byte(MuxPacketStreamReset)
	frame[1] = byte(id >> 24)
	frame[2] = byte(id >> 16)
	frame[3] = byte(id >> 8)
	frame[4] = byte(id)
	return m.writeFrame(frame)
}

// removeStream removes a stream from the multiplexer.
func (m *RelayMux) removeStream(id StreamID, peerKey [32]byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.streams, id)
	delete(m.streamsByKey, peerKey)
}

// idleCleanupLoop periodically cleans up idle streams.
func (m *RelayMux) idleCleanupLoop() {
	ticker := time.NewTicker(m.config.IdleTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanupIdleStreams()
		}
	}
}

// cleanupIdleStreams removes streams that have been idle too long.
func (m *RelayMux) cleanupIdleStreams() {
	now := time.Now().UnixNano()
	idleThreshold := m.config.IdleTimeout.Nanoseconds()

	m.mu.Lock()
	var toRemove []StreamID
	for id, stream := range m.streams {
		lastActive := stream.lastActive.Load()
		if now-lastActive > idleThreshold {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		if stream, ok := m.streams[id]; ok {
			stream.closeInternal()
			delete(m.streams, id)
			delete(m.streamsByKey, stream.peerKey)
			m.stats.StreamsClosed.Add(1)
		}
	}
	m.mu.Unlock()

	if len(toRemove) > 0 {
		logrus.WithFields(logrus.Fields{
			"function": "cleanupIdleStreams",
			"count":    len(toRemove),
		}).Debug("Cleaned up idle streams")
	}
}

// closeAllStreams closes all active streams.
func (m *RelayMux) closeAllStreams() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, stream := range m.streams {
		stream.closeInternal()
	}
	m.streams = make(map[StreamID]*MuxStream)
	m.streamsByKey = make(map[[32]byte]*MuxStream)
}

// GetStream returns the stream for the given peer key, if it exists.
//
//export ToxMuxGetStream
func (m *RelayMux) GetStream(peerKey [32]byte) *MuxStream {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.streamsByKey[peerKey]
}

// StreamCount returns the current number of active streams.
//
//export ToxMuxStreamCount
func (m *RelayMux) StreamCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.streams)
}

// Stats returns a copy of the current multiplexer statistics.
//
//export ToxMuxStats
func (m *RelayMux) Stats() MuxStats {
	return MuxStats{
		StreamsOpened:  atomic.Uint64{},
		StreamsClosed:  atomic.Uint64{},
		BytesSent:      atomic.Uint64{},
		BytesReceived:  atomic.Uint64{},
		FramesSent:     atomic.Uint64{},
		FramesReceived: atomic.Uint64{},
		Errors:         atomic.Uint64{},
	}
}

// GetStatsSnapshot returns a snapshot of the current statistics.
func (m *RelayMux) GetStatsSnapshot() (opened, closed, bytesSent, bytesRecv, framesSent, framesRecv, errs uint64) {
	return m.stats.StreamsOpened.Load(),
		m.stats.StreamsClosed.Load(),
		m.stats.BytesSent.Load(),
		m.stats.BytesReceived.Load(),
		m.stats.FramesSent.Load(),
		m.stats.FramesReceived.Load(),
		m.stats.Errors.Load()
}

// Close closes the multiplexer and all its streams.
//
//export ToxMuxClose
func (m *RelayMux) Close() error {
	if m.closed.Swap(true) {
		return nil // Already closed
	}

	m.cancel()

	// Send close to all streams
	m.mu.Lock()
	for id := range m.streams {
		m.sendStreamClose(id)
	}
	m.mu.Unlock()

	m.closeAllStreams()

	logrus.WithField("function", "Close").Info("Relay multiplexer closed")

	return m.conn.Close()
}

// --- MuxStream methods ---

// ID returns the stream's unique identifier.
func (s *MuxStream) ID() StreamID {
	return s.id
}

// State returns the current stream state.
func (s *MuxStream) State() StreamState {
	return StreamState(s.state.Load())
}

// PeerKey returns the peer's public key for this stream.
func (s *MuxStream) PeerKey() [32]byte {
	return s.peerKey
}

// Write sends data on the stream.
func (s *MuxStream) Write(data []byte) (int, error) {
	if s.State() != StreamStateOpen {
		return 0, errors.New("stream not open")
	}

	s.lastActive.Store(time.Now().UnixNano())

	// Build data frame: [MuxPacketStreamData:1][StreamID:4][Data:...]
	frame := make([]byte, 5+len(data))
	frame[0] = byte(MuxPacketStreamData)
	frame[1] = byte(s.id >> 24)
	frame[2] = byte(s.id >> 16)
	frame[3] = byte(s.id >> 8)
	frame[4] = byte(s.id)
	copy(frame[5:], data)

	if err := s.mux.writeFrame(frame); err != nil {
		return 0, err
	}

	return len(data), nil
}

// Read reads data from the stream.
func (s *MuxStream) Read(buf []byte) (int, error) {
	// First, drain any partial data from previous read
	if len(s.readPartial) > 0 {
		n := copy(buf, s.readPartial)
		s.readPartial = s.readPartial[n:]
		return n, nil
	}

	select {
	case <-s.closedCh:
		return 0, io.EOF
	case data := <-s.readBuf:
		n := copy(buf, data)
		if n < len(data) {
			s.readPartial = data[n:]
		}
		s.lastActive.Store(time.Now().UnixNano())
		return n, nil
	}
}

// ReadWithTimeout reads data with a timeout.
func (s *MuxStream) ReadWithTimeout(buf []byte, timeout time.Duration) (int, error) {
	// First, drain any partial data from previous read
	if len(s.readPartial) > 0 {
		n := copy(buf, s.readPartial)
		s.readPartial = s.readPartial[n:]
		return n, nil
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-s.closedCh:
		return 0, io.EOF
	case <-timer.C:
		return 0, context.DeadlineExceeded
	case data := <-s.readBuf:
		n := copy(buf, data)
		if n < len(data) {
			s.readPartial = data[n:]
		}
		s.lastActive.Store(time.Now().UnixNano())
		return n, nil
	}
}

// Close closes the stream.
func (s *MuxStream) Close() error {
	if s.State() != StreamStateOpen {
		return nil
	}

	s.state.Store(int32(StreamStateClosed))

	// Send close request
	if err := s.mux.sendStreamClose(s.id); err != nil {
		return err
	}

	// Remove from mux
	s.mux.removeStream(s.id, s.peerKey)
	s.closeInternal()

	return nil
}

// closeInternal closes the stream without sending a close message.
func (s *MuxStream) closeInternal() {
	s.closeOnce.Do(func() {
		s.state.Store(int32(StreamStateClosed))
		close(s.closedCh)
	})
}

// LocalAddr returns a local address representation for the stream.
func (s *MuxStream) LocalAddr() net.Addr {
	return &MuxStreamAddr{
		StreamID: s.id,
		Key:      s.localKey,
	}
}

// RemoteAddr returns the remote address representation for the stream.
func (s *MuxStream) RemoteAddr() net.Addr {
	return &MuxStreamAddr{
		StreamID: s.id,
		Key:      s.peerKey,
	}
}

// MuxStreamAddr implements net.Addr for multiplexed streams.
type MuxStreamAddr struct {
	StreamID StreamID
	Key      [32]byte
}

// Network returns the network type.
func (a *MuxStreamAddr) Network() string {
	return "mux"
}

// String returns a string representation.
func (a *MuxStreamAddr) String() string {
	return fmt.Sprintf("mux://%x/%d", a.Key[:8], a.StreamID)
}
