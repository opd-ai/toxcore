package transport

import (
	"net"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

const (
	// DefaultWorkerPoolSize is the default number of workers in the pool.
	DefaultWorkerPoolSize = 100

	// DefaultQueueSize is the default size of the work queue.
	DefaultQueueSize = 10000

	// MinWorkerPoolSize is the minimum allowed worker pool size.
	MinWorkerPoolSize = 10

	// MinQueueSize is the minimum allowed queue size.
	MinQueueSize = 100
)

// WorkerPoolConfig configures the packet handler worker pool.
type WorkerPoolConfig struct {
	// NumWorkers is the number of goroutines in the pool.
	NumWorkers int

	// QueueSize is the maximum number of pending work items.
	QueueSize int

	// DropPolicy determines what happens when the queue is full.
	// If true, new items are dropped. If false, Submit blocks.
	DropOnFull bool
}

// DefaultWorkerPoolConfig returns the default worker pool configuration.
func DefaultWorkerPoolConfig() *WorkerPoolConfig {
	return &WorkerPoolConfig{
		NumWorkers: DefaultWorkerPoolSize,
		QueueSize:  DefaultQueueSize,
		DropOnFull: true, // Prefer dropping packets over blocking under high load
	}
}

// packetWork represents a packet handling work item.
type packetWork struct {
	packet  *Packet
	addr    net.Addr
	handler PacketHandler
}

// WorkerPool manages a fixed-size pool of goroutines for packet handling.
// This replaces unbounded goroutine creation with bounded concurrency.
type WorkerPool struct {
	workChan chan *packetWork
	config   *WorkerPoolConfig
	wg       sync.WaitGroup
	stopChan chan struct{}
	stopped  int32 // Atomic flag

	// Statistics
	submitted   uint64 // Total work items submitted
	processed   uint64 // Total work items processed
	dropped     uint64 // Work items dropped due to full queue
	queueHighWM uint64 // High watermark for queue length
}

// NewWorkerPool creates a new worker pool with the specified configuration.
func NewWorkerPool(config *WorkerPoolConfig) *WorkerPool {
	if config == nil {
		config = DefaultWorkerPoolConfig()
	}
	if config.NumWorkers < MinWorkerPoolSize {
		config.NumWorkers = MinWorkerPoolSize
	}
	if config.QueueSize < MinQueueSize {
		config.QueueSize = MinQueueSize
	}

	wp := &WorkerPool{
		workChan: make(chan *packetWork, config.QueueSize),
		config:   config,
		stopChan: make(chan struct{}),
	}

	// Start worker goroutines
	wp.wg.Add(config.NumWorkers)
	for i := 0; i < config.NumWorkers; i++ {
		go wp.worker(i)
	}

	logrus.WithFields(logrus.Fields{
		"function":    "NewWorkerPool",
		"num_workers": config.NumWorkers,
		"queue_size":  config.QueueSize,
		"drop_policy": config.DropOnFull,
	}).Info("Worker pool created")

	return wp
}

// worker is the main loop for a worker goroutine.
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	logrus.WithField("worker_id", id).Debug("Worker started")

	for {
		select {
		case work, ok := <-wp.workChan:
			if !ok {
				// Channel closed
				logrus.WithField("worker_id", id).Debug("Worker stopping (channel closed)")
				return
			}
			wp.processWork(work)
		case <-wp.stopChan:
			logrus.WithField("worker_id", id).Debug("Worker stopping (stop signal)")
			return
		}
	}
}

// processWork handles a single work item.
func (wp *WorkerPool) processWork(work *packetWork) {
	defer func() {
		if r := recover(); r != nil {
			logrus.WithFields(logrus.Fields{
				"function":    "WorkerPool.processWork",
				"panic":       r,
				"packet_type": work.packet.PacketType,
			}).Error("Handler panicked, recovered")
		}
	}()

	work.handler(work.packet, work.addr)
	atomic.AddUint64(&wp.processed, 1)
}

// Submit queues a packet for handling by the worker pool.
// Returns true if the work was accepted, false if dropped.
func (wp *WorkerPool) Submit(packet *Packet, addr net.Addr, handler PacketHandler) bool {
	if atomic.LoadInt32(&wp.stopped) == 1 {
		return false
	}

	work := &packetWork{
		packet:  packet,
		addr:    addr,
		handler: handler,
	}

	atomic.AddUint64(&wp.submitted, 1)

	// Track high watermark
	queueLen := uint64(len(wp.workChan))
	for {
		hwm := atomic.LoadUint64(&wp.queueHighWM)
		if queueLen <= hwm || atomic.CompareAndSwapUint64(&wp.queueHighWM, hwm, queueLen) {
			break
		}
	}

	if wp.config.DropOnFull {
		select {
		case wp.workChan <- work:
			return true
		default:
			// Queue is full, drop the packet
			atomic.AddUint64(&wp.dropped, 1)
			logrus.WithFields(logrus.Fields{
				"function":    "WorkerPool.Submit",
				"packet_type": packet.PacketType,
				"queue_len":   len(wp.workChan),
			}).Warn("Work queue full, dropping packet")
			return false
		}
	} else {
		// Blocking mode
		wp.workChan <- work
		return true
	}
}

// Stop shuts down the worker pool gracefully.
// It waits for all workers to finish their current work.
func (wp *WorkerPool) Stop() {
	if !atomic.CompareAndSwapInt32(&wp.stopped, 0, 1) {
		return // Already stopped
	}

	logrus.WithField("function", "WorkerPool.Stop").Info("Stopping worker pool")

	close(wp.stopChan)
	wp.wg.Wait()

	// Drain remaining work (optional: could process remaining items)
	close(wp.workChan)
	drained := 0
	for range wp.workChan {
		drained++
	}

	logrus.WithFields(logrus.Fields{
		"function": "WorkerPool.Stop",
		"drained":  drained,
	}).Info("Worker pool stopped")
}

// Stats returns statistics about the worker pool.
func (wp *WorkerPool) Stats() WorkerPoolStats {
	return WorkerPoolStats{
		NumWorkers:  wp.config.NumWorkers,
		QueueSize:   wp.config.QueueSize,
		QueueLength: len(wp.workChan),
		Submitted:   atomic.LoadUint64(&wp.submitted),
		Processed:   atomic.LoadUint64(&wp.processed),
		Dropped:     atomic.LoadUint64(&wp.dropped),
		QueueHighWM: atomic.LoadUint64(&wp.queueHighWM),
	}
}

// WorkerPoolStats contains statistics about the worker pool.
type WorkerPoolStats struct {
	NumWorkers  int
	QueueSize   int
	QueueLength int
	Submitted   uint64
	Processed   uint64
	Dropped     uint64
	QueueHighWM uint64
}

// Pending returns the number of unprocessed work items.
func (s WorkerPoolStats) Pending() uint64 {
	if s.Submitted > s.Processed+s.Dropped {
		return s.Submitted - s.Processed - s.Dropped
	}
	return 0
}

// DropRate returns the percentage of items dropped (0-100).
func (s WorkerPoolStats) DropRate() float64 {
	if s.Submitted == 0 {
		return 0
	}
	return float64(s.Dropped) / float64(s.Submitted) * 100
}

// Utilization returns the queue utilization as a percentage (0-100).
func (s WorkerPoolStats) Utilization() float64 {
	if s.QueueSize == 0 {
		return 0
	}
	return float64(s.QueueLength) / float64(s.QueueSize) * 100
}
