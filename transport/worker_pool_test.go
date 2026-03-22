package transport

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type workerPoolTestAddr struct{}

func (workerPoolTestAddr) Network() string { return "mock" }
func (workerPoolTestAddr) String() string  { return "mock:1234" }

func TestWorkerPool_Basic(t *testing.T) {
	config := DefaultWorkerPoolConfig()
	pool := NewWorkerPool(config)
	defer pool.Stop()

	if pool.config.NumWorkers != DefaultWorkerPoolSize {
		t.Errorf("Expected %d workers, got %d", DefaultWorkerPoolSize, pool.config.NumWorkers)
	}
}

func TestWorkerPool_MinConfig(t *testing.T) {
	config := &WorkerPoolConfig{
		NumWorkers: 1,  // Below minimum
		QueueSize:  10, // Below minimum
	}
	pool := NewWorkerPool(config)
	defer pool.Stop()

	if pool.config.NumWorkers != MinWorkerPoolSize {
		t.Errorf("Expected minimum %d workers, got %d", MinWorkerPoolSize, pool.config.NumWorkers)
	}
	if pool.config.QueueSize != MinQueueSize {
		t.Errorf("Expected minimum %d queue size, got %d", MinQueueSize, pool.config.QueueSize)
	}
}

func TestWorkerPool_SubmitAndProcess(t *testing.T) {
	config := &WorkerPoolConfig{
		NumWorkers: MinWorkerPoolSize,
		QueueSize:  MinQueueSize,
	}
	pool := NewWorkerPool(config)
	defer pool.Stop()

	var processed int32
	var wg sync.WaitGroup

	numPackets := 100
	wg.Add(numPackets)

	handler := func(p *Packet, a net.Addr) error {
		atomic.AddInt32(&processed, 1)
		wg.Done()
		return nil
	}

	for i := 0; i < numPackets; i++ {
		packet := &Packet{PacketType: PacketPingRequest}
		if !pool.Submit(packet, workerPoolTestAddr{}, handler) {
			t.Error("Submit should succeed when queue is not full")
		}
	}

	// Wait for all to be processed
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatalf("Timeout waiting for packets to be processed, only got %d", atomic.LoadInt32(&processed))
	}

	if atomic.LoadInt32(&processed) != int32(numPackets) {
		t.Errorf("Expected %d processed, got %d", numPackets, processed)
	}
}

func TestWorkerPool_DropOnFull(t *testing.T) {
	config := &WorkerPoolConfig{
		NumWorkers: MinWorkerPoolSize,
		QueueSize:  MinQueueSize,
		DropOnFull: true,
	}
	pool := NewWorkerPool(config)
	defer pool.Stop()

	// Handler that blocks forever
	blockChan := make(chan struct{})
	blockingHandler := func(p *Packet, a net.Addr) error {
		<-blockChan
		return nil
	}

	// Fill up the queue and workers
	dropped := 0
	for i := 0; i < MinQueueSize+MinWorkerPoolSize+50; i++ {
		packet := &Packet{PacketType: PacketPingRequest}
		if !pool.Submit(packet, workerPoolTestAddr{}, blockingHandler) {
			dropped++
		}
	}

	// Should have dropped some packets
	if dropped == 0 {
		t.Error("Expected some packets to be dropped when queue is full")
	}

	stats := pool.Stats()
	if stats.Dropped == 0 {
		t.Error("Stats should show dropped packets")
	}

	// Clean up
	close(blockChan)
}

func TestWorkerPool_BlockOnFull(t *testing.T) {
	config := &WorkerPoolConfig{
		NumWorkers: MinWorkerPoolSize,
		QueueSize:  MinQueueSize,
		DropOnFull: false,
	}
	pool := NewWorkerPool(config)
	defer pool.Stop()

	// Handler that releases quickly
	quickHandler := func(p *Packet, a net.Addr) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	// All submits should succeed (blocking mode)
	for i := 0; i < 50; i++ {
		packet := &Packet{PacketType: PacketPingRequest}
		if !pool.Submit(packet, workerPoolTestAddr{}, quickHandler) {
			t.Error("Submit should always succeed in blocking mode")
		}
	}

	stats := pool.Stats()
	if stats.Dropped != 0 {
		t.Error("No packets should be dropped in blocking mode")
	}
}

func TestWorkerPool_Stop(t *testing.T) {
	config := &WorkerPoolConfig{
		NumWorkers: MinWorkerPoolSize,
		QueueSize:  MinQueueSize,
	}
	pool := NewWorkerPool(config)

	// Submit some work
	handler := func(p *Packet, a net.Addr) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	for i := 0; i < 10; i++ {
		packet := &Packet{PacketType: PacketPingRequest}
		pool.Submit(packet, workerPoolTestAddr{}, handler)
	}

	// Stop should wait for workers
	done := make(chan struct{})
	go func() {
		pool.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Stop timed out")
	}

	// Submit after stop should fail
	packet := &Packet{PacketType: PacketPingRequest}
	if pool.Submit(packet, workerPoolTestAddr{}, handler) {
		t.Error("Submit should fail after stop")
	}
}

func TestWorkerPool_PanicRecovery(t *testing.T) {
	config := &WorkerPoolConfig{
		NumWorkers: MinWorkerPoolSize,
		QueueSize:  MinQueueSize,
	}
	pool := NewWorkerPool(config)
	defer pool.Stop()

	var processedBefore, processedAfter int32

	// Handler that panics
	panicHandler := func(p *Packet, a net.Addr) error {
		atomic.AddInt32(&processedBefore, 1)
		panic("test panic")
	}

	// Handler that doesn't panic
	normalHandler := func(p *Packet, a net.Addr) error {
		atomic.AddInt32(&processedAfter, 1)
		return nil
	}

	// Submit panic-inducing work
	packet := &Packet{PacketType: PacketPingRequest}
	pool.Submit(packet, workerPoolTestAddr{}, panicHandler)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Submit normal work - pool should still work
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		packet := &Packet{PacketType: PacketPingRequest}
		pool.Submit(packet, workerPoolTestAddr{}, func(p *Packet, a net.Addr) error {
			normalHandler(p, a)
			wg.Done()
			return nil
		})
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - pool recovered from panic
	case <-time.After(5 * time.Second):
		t.Fatal("Pool failed to recover from panic")
	}

	if atomic.LoadInt32(&processedAfter) != 10 {
		t.Errorf("Expected 10 processed after panic, got %d", processedAfter)
	}
}

func TestWorkerPoolStats(t *testing.T) {
	config := &WorkerPoolConfig{
		NumWorkers: MinWorkerPoolSize,
		QueueSize:  MinQueueSize,
	}
	pool := NewWorkerPool(config)
	defer pool.Stop()

	var wg sync.WaitGroup
	wg.Add(50)

	handler := func(p *Packet, a net.Addr) error {
		wg.Done()
		return nil
	}

	for i := 0; i < 50; i++ {
		packet := &Packet{PacketType: PacketPingRequest}
		pool.Submit(packet, workerPoolTestAddr{}, handler)
	}

	wg.Wait()

	stats := pool.Stats()
	if stats.Submitted != 50 {
		t.Errorf("Expected 50 submitted, got %d", stats.Submitted)
	}
	if stats.Processed < 50 {
		t.Errorf("Expected at least 50 processed, got %d", stats.Processed)
	}
}

func TestWorkerPoolStats_Calculations(t *testing.T) {
	stats := WorkerPoolStats{
		NumWorkers:  10,
		QueueSize:   100,
		QueueLength: 50,
		Submitted:   1000,
		Processed:   900,
		Dropped:     50,
	}

	if stats.Pending() != 50 {
		t.Errorf("Expected pending 50, got %d", stats.Pending())
	}

	if stats.DropRate() != 5.0 {
		t.Errorf("Expected drop rate 5.0, got %f", stats.DropRate())
	}

	if stats.Utilization() != 50.0 {
		t.Errorf("Expected utilization 50.0, got %f", stats.Utilization())
	}
}

func TestWorkerPoolStats_ZeroCase(t *testing.T) {
	stats := WorkerPoolStats{}

	if stats.Pending() != 0 {
		t.Error("Expected 0 pending for empty stats")
	}
	if stats.DropRate() != 0 {
		t.Error("Expected 0 drop rate for empty stats")
	}
	if stats.Utilization() != 0 {
		t.Error("Expected 0 utilization for empty stats")
	}
}

func BenchmarkWorkerPool_Submit(b *testing.B) {
	config := &WorkerPoolConfig{
		NumWorkers: 100,
		QueueSize:  10000,
		DropOnFull: true,
	}
	pool := NewWorkerPool(config)
	defer pool.Stop()

	handler := func(p *Packet, a net.Addr) error {
		return nil
	}

	packet := &Packet{PacketType: PacketPingRequest}
	addr := workerPoolTestAddr{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Submit(packet, addr, handler)
	}
}

func BenchmarkWorkerPool_SubmitProcess(b *testing.B) {
	config := &WorkerPoolConfig{
		NumWorkers: 100,
		QueueSize:  10000,
		DropOnFull: true,
	}
	pool := NewWorkerPool(config)
	defer pool.Stop()

	var processed int64
	handler := func(p *Packet, a net.Addr) error {
		atomic.AddInt64(&processed, 1)
		return nil
	}

	packet := &Packet{PacketType: PacketPingRequest}
	addr := workerPoolTestAddr{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Submit(packet, addr, handler)
	}

	// Wait for processing
	for atomic.LoadInt64(&processed) < int64(b.N-int(pool.Stats().Dropped)) {
		time.Sleep(time.Millisecond)
	}
}
