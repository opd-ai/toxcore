//go:build linux
// +build linux

// Package transport provides network transport implementations for Tox.
//
// This file implements dynamic buffer sizing and enhanced UDP packet reading
// for Linux. While Go's x/sys/unix package doesn't expose recvmmsg directly,
// this provides dynamic buffer management and socket tuning for improved
// throughput.
package transport

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	// DefaultBatchSize is a placeholder for API compatibility.
	// Actual batching requires syscall-level recvmmsg which isn't in x/sys/unix.
	DefaultBatchSize = 16

	// MaxBatchSize is the maximum batch size allowed.
	MaxBatchSize = 64

	// MinReceiveBufferSize is the minimum per-packet buffer size.
	MinReceiveBufferSize = 512

	// DefaultReceiveBufferSize is the default per-packet buffer size.
	// Matches MTU minus IP/UDP headers: 1500 - 20 - 8 = 1472
	DefaultReceiveBufferSize = 1500

	// MaxReceiveBufferSize is the maximum per-packet buffer size.
	// Max UDP payload: 65535 - 20 (IP) - 8 (UDP) = 65507
	MaxReceiveBufferSize = 65507

	// BufferGrowThreshold is the fraction of buffer usage that triggers growth.
	// If a packet uses more than 90% of the buffer, consider growing.
	BufferGrowThreshold = 0.9

	// BufferShrinkThreshold is the fraction below which we consider shrinking.
	// If average usage is below 50% for a period, consider shrinking.
	BufferShrinkThreshold = 0.5

	// BufferAdjustmentInterval is how often to evaluate buffer sizing.
	BufferAdjustmentInterval = 30 * time.Second
)

// BatchReceiveConfig configures packet receiving with dynamic buffers.
type BatchReceiveConfig struct {
	// BatchSize is for API compatibility (batching not available via Go stdlib).
	BatchSize int

	// InitialBufferSize is the starting per-packet buffer size.
	InitialBufferSize int

	// EnableDynamicBuffers enables automatic buffer size adjustment.
	EnableDynamicBuffers bool

	// MaxBufferSize caps buffer growth for memory control.
	MaxBufferSize int

	// SocketReceiveBuffer sets SO_RCVBUF for kernel-level buffering (bytes).
	// 0 means use system default. Typical values: 256KB - 4MB.
	SocketReceiveBuffer int
}

// DefaultBatchReceiveConfig returns a sensible default configuration.
func DefaultBatchReceiveConfig() *BatchReceiveConfig {
	return &BatchReceiveConfig{
		BatchSize:            DefaultBatchSize,
		InitialBufferSize:    DefaultReceiveBufferSize,
		EnableDynamicBuffers: true,
		MaxBufferSize:        MaxReceiveBufferSize,
		SocketReceiveBuffer:  256 * 1024, // 256KB kernel buffer
	}
}

// BatchReceiveStats tracks receiving statistics.
type BatchReceiveStats struct {
	TotalBatches      uint64                   // Number of read calls
	TotalPackets      uint64                   // Total packets received
	TotalBytes        uint64                   // Total bytes received
	TruncatedPackets  uint64                   // Packets that were truncated (buffer too small)
	BatchSizeHist     [MaxBatchSize + 1]uint64 // Histogram of actual batch sizes (all 1 without recvmmsg)
	CurrentBufferSize int                      // Current per-packet buffer size
	BufferGrowCount   uint64                   // Number of times buffer was grown
	BufferShrinkCount uint64                   // Number of times buffer was shrunk
	MaxPacketSeen     int                      // Largest packet observed
	AvgPacketSize     uint64                   // Running average packet size
}

// ReceivedPacket holds a received UDP packet and its source.
type ReceivedPacket struct {
	Data []byte
	Addr net.Addr
}

// BatchReceiver reads UDP packets with dynamic buffer sizing.
// On Linux, it also tunes socket receive buffers for better performance.
type BatchReceiver struct {
	conn   net.PacketConn
	fd     int
	config *BatchReceiveConfig

	// Dynamic buffer
	buffer []byte

	// Dynamic buffer sizing state
	mu            sync.Mutex
	currentSize   int
	maxPacketSeen int
	packetSizeSum uint64
	packetCount   uint64
	lastAdjust    time.Time

	// Statistics
	stats BatchReceiveStats
}

// NewBatchReceiver creates a receiver with dynamic buffer sizing.
// The fd parameter is used to set socket options on Linux.
func NewBatchReceiver(fd int, config *BatchReceiveConfig) *BatchReceiver {
	config = applyBatchReceiverDefaults(config)

	br := &BatchReceiver{
		fd:          fd,
		config:      config,
		currentSize: config.InitialBufferSize,
		buffer:      make([]byte, config.InitialBufferSize),
		lastAdjust:  time.Now(),
	}

	br.stats.CurrentBufferSize = config.InitialBufferSize
	br.configureSocketBuffer(fd, config.SocketReceiveBuffer)

	return br
}

// applyBatchReceiverDefaults applies default values and clamps configuration.
func applyBatchReceiverDefaults(config *BatchReceiveConfig) *BatchReceiveConfig {
	if config == nil {
		config = DefaultBatchReceiveConfig()
	}

	config.BatchSize = clampInt(config.BatchSize, 1, MaxBatchSize)
	config.InitialBufferSize = clampInt(config.InitialBufferSize, MinReceiveBufferSize, MaxReceiveBufferSize)
	if config.MaxBufferSize < config.InitialBufferSize {
		config.MaxBufferSize = config.InitialBufferSize
	}
	config.MaxBufferSize = clampInt(config.MaxBufferSize, config.InitialBufferSize, MaxReceiveBufferSize)

	return config
}

// clampInt clamps an integer value to the given range.
func clampInt(value, minVal, maxVal int) int {
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}

// configureSocketBuffer sets the kernel-level socket receive buffer.
func (br *BatchReceiver) configureSocketBuffer(fd, bufferSize int) {
	if fd <= 0 || bufferSize <= 0 {
		return
	}

	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUF, bufferSize); err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "NewBatchReceiver",
			"buffer_size": bufferSize,
			"error":       err.Error(),
		}).Debug("Failed to set SO_RCVBUF")
	} else {
		logrus.WithFields(logrus.Fields{
			"function":    "NewBatchReceiver",
			"buffer_size": bufferSize,
		}).Debug("Set SO_RCVBUF for improved throughput")
	}
}

// NewBatchReceiverFromConn creates a receiver using a PacketConn.
func NewBatchReceiverFromConn(conn net.PacketConn, config *BatchReceiveConfig) *BatchReceiver {
	if config == nil {
		config = DefaultBatchReceiveConfig()
	}

	// Apply defaults
	if config.InitialBufferSize < MinReceiveBufferSize {
		config.InitialBufferSize = MinReceiveBufferSize
	}
	if config.MaxBufferSize < config.InitialBufferSize {
		config.MaxBufferSize = config.InitialBufferSize
	}

	br := &BatchReceiver{
		conn:        conn,
		config:      config,
		currentSize: config.InitialBufferSize,
		buffer:      make([]byte, config.InitialBufferSize),
		lastAdjust:  time.Now(),
	}

	br.stats.CurrentBufferSize = config.InitialBufferSize

	return br
}

// RecvBatch reads packets (one at a time, as Go stdlib doesn't expose recvmmsg).
// The "batch" name is for API compatibility; true batching requires syscall-level access.
func (br *BatchReceiver) RecvBatch(timeout time.Duration) ([]ReceivedPacket, error) {
	if br.conn == nil {
		return nil, nil
	}

	if err := br.setReadTimeout(timeout); err != nil {
		logrus.WithError(err).Debug("Failed to set read deadline")
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
		return nil // Timeout, no packets
	}
	return err
}

// updateReceiveStats updates statistics after receiving a packet.
func (br *BatchReceiver) updateReceiveStats(n int) {
	atomic.AddUint64(&br.stats.TotalBatches, 1)
	atomic.AddUint64(&br.stats.TotalPackets, 1)
	atomic.AddUint64(&br.stats.TotalBytes, uint64(n))
	atomic.AddUint64(&br.stats.BatchSizeHist[1], 1) // Always batch size 1

	br.mu.Lock()
	br.packetSizeSum += uint64(n)
	br.packetCount++
	if n > br.maxPacketSeen {
		br.maxPacketSeen = n
		br.stats.MaxPacketSeen = n
	}
	br.mu.Unlock()

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

// maybeAdjustBuffers evaluates and potentially adjusts buffer sizes.
func (br *BatchReceiver) maybeAdjustBuffers() {
	br.mu.Lock()
	defer br.mu.Unlock()

	if !br.shouldEvaluateBufferSize() {
		return
	}

	avgSize := br.packetSizeSum / br.packetCount
	br.stats.AvgPacketSize = avgSize

	br.adjustBufferIfNeeded(avgSize)
	br.resetBufferTracking()
}

// shouldEvaluateBufferSize checks if enough time has passed and data collected for evaluation.
func (br *BatchReceiver) shouldEvaluateBufferSize() bool {
	if time.Since(br.lastAdjust) < BufferAdjustmentInterval {
		return false
	}
	return br.packetCount > 0
}

// adjustBufferIfNeeded grows or shrinks the buffer based on packet size patterns.
func (br *BatchReceiver) adjustBufferIfNeeded(avgSize uint64) {
	if br.shouldGrowBuffer() {
		br.growBuffer()
	} else if br.shouldShrinkBuffer(avgSize) {
		br.shrinkBuffer(avgSize)
	}

	br.stats.CurrentBufferSize = br.currentSize
}

// shouldGrowBuffer checks if we've seen large packets that warrant buffer growth.
func (br *BatchReceiver) shouldGrowBuffer() bool {
	return br.maxPacketSeen > int(float64(br.currentSize)*BufferGrowThreshold)
}

// growBuffer increases the buffer size based on max packet seen.
func (br *BatchReceiver) growBuffer() {
	newSize := br.maxPacketSeen + 256 // Add headroom
	if newSize > br.config.MaxBufferSize {
		newSize = br.config.MaxBufferSize
	}
	if newSize <= br.currentSize {
		return
	}

	logrus.WithFields(logrus.Fields{
		"function":     "BatchReceiver.maybeAdjustBuffers",
		"old_size":     br.currentSize,
		"new_size":     newSize,
		"max_pkt_seen": br.maxPacketSeen,
		"packet_count": br.packetCount,
	}).Info("Growing receive buffers")

	br.currentSize = newSize
	br.buffer = make([]byte, newSize)
	br.stats.BufferGrowCount++
}

// shouldShrinkBuffer checks if packets are consistently small.
func (br *BatchReceiver) shouldShrinkBuffer(avgSize uint64) bool {
	return int(avgSize) < int(float64(br.currentSize)*BufferShrinkThreshold)
}

// shrinkBuffer decreases the buffer size based on average packet size.
func (br *BatchReceiver) shrinkBuffer(avgSize uint64) {
	newSize := int(avgSize*2) + 256
	if newSize < MinReceiveBufferSize {
		newSize = MinReceiveBufferSize
	}
	if newSize >= br.currentSize || newSize < br.config.InitialBufferSize {
		return
	}

	logrus.WithFields(logrus.Fields{
		"function":     "BatchReceiver.maybeAdjustBuffers",
		"old_size":     br.currentSize,
		"new_size":     newSize,
		"avg_pkt_size": avgSize,
		"packet_count": br.packetCount,
	}).Info("Shrinking receive buffers")

	br.currentSize = newSize
	br.buffer = make([]byte, newSize)
	br.stats.BufferShrinkCount++
}

// resetBufferTracking resets the tracking state for the next evaluation period.
func (br *BatchReceiver) resetBufferTracking() {
	br.maxPacketSeen = 0
	br.packetSizeSum = 0
	br.packetCount = 0
	br.lastAdjust = time.Now()
}

// Stats returns current statistics.
func (br *BatchReceiver) Stats() BatchReceiveStats {
	br.mu.Lock()
	defer br.mu.Unlock()
	stats := br.stats
	stats.CurrentBufferSize = br.currentSize
	if br.packetCount > 0 {
		stats.AvgPacketSize = br.packetSizeSum / br.packetCount
	}
	return stats
}

// SetBufferSize manually sets the receive buffer size.
func (br *BatchReceiver) SetBufferSize(size int) {
	br.mu.Lock()
	defer br.mu.Unlock()

	if size < MinReceiveBufferSize {
		size = MinReceiveBufferSize
	}
	if size > br.config.MaxBufferSize {
		size = br.config.MaxBufferSize
	}

	if size != br.currentSize {
		br.currentSize = size
		br.buffer = make([]byte, size)
		br.stats.CurrentBufferSize = size
	}
}

// BatchReceiverAdapter adapts BatchReceiver for use with existing transport code.
type BatchReceiverAdapter struct {
	receiver *BatchReceiver
	timeout  time.Duration
}

// NewBatchReceiverAdapter creates an adapter for the batch receiver.
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
		return nil, nil // Timeout
	}
	return &packets[0], nil
}

// Stats returns batch receiver statistics.
func (a *BatchReceiverAdapter) Stats() BatchReceiveStats {
	return a.receiver.Stats()
}

// SetSocketReceiveBuffer sets the kernel-level socket receive buffer.
// Larger buffers help with burst traffic. Typical values: 256KB - 4MB.
func SetSocketReceiveBuffer(conn net.PacketConn, size int) error {
	// Try to get the underlying file descriptor
	type filer interface {
		File() (*interface{}, error)
	}

	// Use raw control for UDP connections
	rawConn, err := conn.(interface{ SyscallConn() (interface{}, error) }).SyscallConn()
	if err != nil {
		return err
	}

	var setErr error
	controlFunc := rawConn.(interface {
		Control(func(fd uintptr)) error
	})
	err = controlFunc.Control(func(fd uintptr) {
		setErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUF, size)
	})
	if err != nil {
		return err
	}
	return setErr
}
