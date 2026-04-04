//go:build !linux
// +build !linux

// Package transport provides network transport implementations for Tox.
//
// This file provides a fallback implementation for platforms without recvmmsg.
// On these platforms, packets are received one at a time using standard ReadFrom.
package transport

import (
	"net"
	"sync/atomic"
	"time"
)

// BatchReceiveConfig configures batch packet receiving.
// On non-Linux platforms, batching is not available but the config
// is still accepted for API compatibility.
type BatchReceiveConfig struct {
	// BatchSize is ignored on non-Linux platforms.
	BatchSize int

	// InitialBufferSize is the per-packet buffer size.
	InitialBufferSize int

	// EnableDynamicBuffers enables automatic buffer size adjustment.
	EnableDynamicBuffers bool

	// MaxBufferSize caps buffer growth for memory control.
	MaxBufferSize int
}

// DefaultBatchReceiveConfig returns a sensible default configuration.
func DefaultBatchReceiveConfig() *BatchReceiveConfig {
	return &BatchReceiveConfig{
		BatchSize:            1, // No batching on non-Linux
		InitialBufferSize:    1500,
		EnableDynamicBuffers: true,
		MaxBufferSize:        65507,
	}
}

// BatchReceiveStats tracks receiving statistics.
type BatchReceiveStats struct {
	TotalBatches      uint64
	TotalPackets      uint64
	TotalBytes        uint64
	TruncatedPackets  uint64
	BatchSizeHist     [65]uint64 // Matches MaxBatchSize+1 from Linux
	CurrentBufferSize int
	BufferGrowCount   uint64
	BufferShrinkCount uint64
	MaxPacketSeen     int
	AvgPacketSize     uint64
}

// ReceivedPacket holds a received UDP packet and its source.
type ReceivedPacket struct {
	Data []byte
	Addr net.Addr
}

// BatchReceiver provides a compatible API on non-Linux platforms.
// It reads packets one at a time using standard net.PacketConn.
type BatchReceiver struct {
	conn          net.PacketConn
	config        *BatchReceiveConfig
	buffer        []byte
	currentSize   int
	stats         BatchReceiveStats
	maxPacketSeen int
	packetSizeSum uint64
	packetCount   uint64
	lastAdjust    time.Time
}

// NewBatchReceiver creates a receiver for the given PacketConn.
// On non-Linux, this uses standard ReadFrom instead of recvmmsg.
func NewBatchReceiver(fd int, config *BatchReceiveConfig) *BatchReceiver {
	// Note: On non-Linux we don't use the fd directly
	// Caller should use NewBatchReceiverFromConn instead
	if config == nil {
		config = DefaultBatchReceiveConfig()
	}
	return &BatchReceiver{
		config:      config,
		currentSize: config.InitialBufferSize,
		buffer:      make([]byte, config.InitialBufferSize),
		lastAdjust:  time.Now(),
	}
}

// NewBatchReceiverFromConn creates a receiver using a PacketConn.
// This is the preferred constructor on non-Linux platforms.
func NewBatchReceiverFromConn(conn net.PacketConn, config *BatchReceiveConfig) *BatchReceiver {
	if config == nil {
		config = DefaultBatchReceiveConfig()
	}

	// Apply defaults
	if config.InitialBufferSize < 512 {
		config.InitialBufferSize = 512
	}
	if config.MaxBufferSize < config.InitialBufferSize {
		config.MaxBufferSize = config.InitialBufferSize
	}

	return &BatchReceiver{
		conn:        conn,
		config:      config,
		currentSize: config.InitialBufferSize,
		buffer:      make([]byte, config.InitialBufferSize),
		lastAdjust:  time.Now(),
	}
}

// RecvBatch reads packets (one at a time on non-Linux).
// Returns a slice with at most one packet.
func (br *BatchReceiver) RecvBatch(timeout time.Duration) ([]ReceivedPacket, error) {
	if br.conn == nil {
		return nil, nil
	}

	if err := br.setReadTimeout(timeout); err != nil {
		// Log but continue
	}

	n, addr, err := br.conn.ReadFrom(br.buffer)
	if err != nil {
		return nil, br.handleReadError(err)
	}

	br.updateReceiveStats(n)

	if br.config.EnableDynamicBuffers {
		br.maybeAdjustBuffers()
	}

	return br.buildPacketResult(n, addr), nil
}

// setReadTimeout sets the read deadline if timeout is positive.
func (br *BatchReceiver) setReadTimeout(timeout time.Duration) error {
	if timeout > 0 {
		return br.conn.SetReadDeadline(time.Now().Add(timeout))
	}
	return nil
}

// handleReadError processes read errors, returning nil for timeouts.
func (br *BatchReceiver) handleReadError(err error) error {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return nil // Timeout
	}
	return err
}

// updateReceiveStats updates statistics after receiving a packet.
func (br *BatchReceiver) updateReceiveStats(n int) {
	atomic.AddUint64(&br.stats.TotalBatches, 1)
	atomic.AddUint64(&br.stats.TotalPackets, 1)
	atomic.AddUint64(&br.stats.TotalBytes, uint64(n))
	atomic.AddUint64(&br.stats.BatchSizeHist[1], 1) // Always batch size 1

	br.packetSizeSum += uint64(n)
	br.packetCount++
	if n > br.maxPacketSeen {
		br.maxPacketSeen = n
		br.stats.MaxPacketSeen = n
	}

	if n >= br.currentSize {
		atomic.AddUint64(&br.stats.TruncatedPackets, 1)
	}
}

// buildPacketResult creates the result slice with a copy of received data.
func (br *BatchReceiver) buildPacketResult(n int, addr net.Addr) []ReceivedPacket {
	data := make([]byte, n)
	copy(data, br.buffer[:n])
	return []ReceivedPacket{{Data: data, Addr: addr}}
}

// maybeAdjustBuffers evaluates and potentially adjusts buffer size.
func (br *BatchReceiver) maybeAdjustBuffers() {
	if !br.shouldAdjustBuffers() {
		return
	}

	br.tryGrowBuffer()
	br.updateStats()
	br.resetAdjustmentPeriod()
}

// shouldAdjustBuffers checks if it's time to consider buffer adjustment.
func (br *BatchReceiver) shouldAdjustBuffers() bool {
	if time.Since(br.lastAdjust) < 30*time.Second {
		return false
	}
	return br.packetCount > 0
}

// tryGrowBuffer grows the buffer if large packets have been observed.
func (br *BatchReceiver) tryGrowBuffer() {
	if br.maxPacketSeen <= int(float64(br.currentSize)*0.9) {
		return
	}

	newSize := br.maxPacketSeen + 256
	if newSize > br.config.MaxBufferSize {
		newSize = br.config.MaxBufferSize
	}
	if newSize > br.currentSize {
		br.currentSize = newSize
		br.buffer = make([]byte, newSize)
		br.stats.BufferGrowCount++
	}
}

// updateStats updates buffer statistics.
func (br *BatchReceiver) updateStats() {
	br.stats.CurrentBufferSize = br.currentSize
	if br.packetCount > 0 {
		br.stats.AvgPacketSize = br.packetSizeSum / br.packetCount
	}
}

// resetAdjustmentPeriod resets counters for the next adjustment period.
func (br *BatchReceiver) resetAdjustmentPeriod() {
	br.maxPacketSeen = 0
	br.packetSizeSum = 0
	br.packetCount = 0
	br.lastAdjust = time.Now()
}

// Stats returns current statistics.
func (br *BatchReceiver) Stats() BatchReceiveStats {
	stats := br.stats
	stats.CurrentBufferSize = br.currentSize
	if br.packetCount > 0 {
		stats.AvgPacketSize = br.packetSizeSum / br.packetCount
	}
	return stats
}

// SetBufferSize manually sets the receive buffer size.
func (br *BatchReceiver) SetBufferSize(size int) {
	if size < 512 {
		size = 512
	}
	if size > br.config.MaxBufferSize {
		size = br.config.MaxBufferSize
	}
	if size != br.currentSize {
		br.currentSize = size
		br.buffer = make([]byte, size)
	}
}

// BatchReceiverAdapter adapts BatchReceiver for use with existing transport code.
type BatchReceiverAdapter struct {
	receiver *BatchReceiver
	timeout  time.Duration
}

// NewBatchReceiverAdapter creates an adapter.
// On non-Linux, the fd parameter is ignored; use NewBatchReceiverAdapterFromConn.
func NewBatchReceiverAdapter(fd int, config *BatchReceiveConfig, timeout time.Duration) *BatchReceiverAdapter {
	return &BatchReceiverAdapter{
		receiver: NewBatchReceiver(fd, config),
		timeout:  timeout,
	}
}

// NewBatchReceiverAdapterFromConn creates an adapter using a PacketConn.
func NewBatchReceiverAdapterFromConn(conn net.PacketConn, config *BatchReceiveConfig, timeout time.Duration) *BatchReceiverAdapter {
	return &BatchReceiverAdapter{
		receiver: NewBatchReceiverFromConn(conn, config),
		timeout:  timeout,
	}
}

// ReadPacket returns the next received packet.
func (a *BatchReceiverAdapter) ReadPacket() (*ReceivedPacket, error) {
	packets, err := a.receiver.RecvBatch(a.timeout)
	if err != nil {
		return nil, err
	}
	if len(packets) == 0 {
		return nil, nil
	}
	return &packets[0], nil
}

// Stats returns receiver statistics.
func (a *BatchReceiverAdapter) Stats() BatchReceiveStats {
	return a.receiver.Stats()
}
