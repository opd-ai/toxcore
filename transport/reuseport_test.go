package transport

import (
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReusePortTransport_Basic(t *testing.T) {
	transport, err := NewReusePortTransport(":0", nil)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	if transport.LocalAddr() == nil {
		t.Error("LocalAddr should not be nil")
	}

	// Should have at least 1 socket
	stats := transport.Stats()
	if stats.NumSockets < 1 {
		t.Errorf("Expected at least 1 socket, got %d", stats.NumSockets)
	}
}

func TestReusePortTransport_Config(t *testing.T) {
	config := &ReusePortConfig{
		NumSockets: 4,
		WorkerPool: DefaultWorkerPoolConfig(),
	}

	transport, err := NewReusePortTransport(":0", config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	stats := transport.Stats()
	// On platforms supporting SO_REUSEPORT, we should have 4 sockets
	// On other platforms, we fall back to 1
	if stats.NumSockets < 1 {
		t.Errorf("Expected at least 1 socket, got %d", stats.NumSockets)
	}

	// Worker pool stats should be available
	if stats.WorkerPoolStats == nil {
		t.Error("Expected worker pool stats to be available")
	}
}

func TestReusePortTransport_DefaultConfig(t *testing.T) {
	config := DefaultReusePortConfig()

	if config.NumSockets != runtime.NumCPU() {
		t.Errorf("Expected NumSockets=%d, got %d", runtime.NumCPU(), config.NumSockets)
	}

	if config.WorkerPool == nil {
		t.Error("Expected WorkerPool config to be set")
	}
}

func TestReusePortTransport_RegisterHandler(t *testing.T) {
	transport, err := NewReusePortTransport(":0", nil)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	var called int32
	handler := func(p *Packet, a net.Addr) error {
		atomic.AddInt32(&called, 1)
		return nil
	}

	transport.RegisterHandler(PacketPingRequest, handler)

	// Handler should be registered (can't easily test internal state)
}

func TestReusePortTransport_SendReceive(t *testing.T) {
	config := &ReusePortConfig{
		NumSockets: 2,
		WorkerPool: &WorkerPoolConfig{
			NumWorkers: MinWorkerPoolSize,
			QueueSize:  MinQueueSize,
			DropOnFull: false,
		},
	}

	transport, err := NewReusePortTransport("127.0.0.1:0", config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	var received int32
	var wg sync.WaitGroup
	numPackets := 10
	wg.Add(numPackets)

	handler := func(p *Packet, a net.Addr) error {
		atomic.AddInt32(&received, 1)
		wg.Done()
		return nil
	}

	transport.RegisterHandler(PacketPingRequest, handler)

	// Create a sender on IPv4
	sender, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create sender: %v", err)
	}
	defer sender.Close()

	// Send packets
	packet := &Packet{PacketType: PacketPingRequest, Data: []byte("test")}
	data, err := packet.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize packet: %v", err)
	}

	for i := 0; i < numPackets; i++ {
		_, err := sender.WriteTo(data, transport.LocalAddr())
		if err != nil {
			t.Fatalf("Failed to send packet: %v", err)
		}
	}

	// Wait for packets to be received
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatalf("Timeout waiting for packets, received %d", atomic.LoadInt32(&received))
	}

	if atomic.LoadInt32(&received) != int32(numPackets) {
		t.Errorf("Expected %d received, got %d", numPackets, received)
	}

	// Check stats
	stats := transport.Stats()
	if stats.PacketsReceived < uint64(numPackets) {
		t.Errorf("Expected at least %d packets received in stats, got %d", numPackets, stats.PacketsReceived)
	}
}

func TestReusePortTransport_Send(t *testing.T) {
	transport, err := NewReusePortTransport("127.0.0.1:0", nil)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	// Create a receiver on IPv4
	receiver, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create receiver: %v", err)
	}
	defer receiver.Close()

	// Send a packet
	packet := &Packet{PacketType: PacketPingRequest, Data: []byte("hello")}
	err = transport.Send(packet, receiver.LocalAddr())
	if err != nil {
		t.Errorf("Failed to send packet: %v", err)
	}

	// Verify stats
	stats := transport.Stats()
	if stats.PacketsSent != 1 {
		t.Errorf("Expected 1 packet sent, got %d", stats.PacketsSent)
	}
}

func TestReusePortTransport_Close(t *testing.T) {
	transport, err := NewReusePortTransport(":0", nil)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	// First close should succeed
	err = transport.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Send after close should fail
	packet := &Packet{PacketType: PacketPingRequest}
	err = transport.Send(packet, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345})
	if err == nil {
		t.Error("Send after close should fail")
	}
}

func TestReusePortTransport_IsConnectionOriented(t *testing.T) {
	transport, err := NewReusePortTransport(":0", nil)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	if transport.IsConnectionOriented() {
		t.Error("UDP transport should not be connection-oriented")
	}
}

func TestReusePortTransport_SupportedNetworks(t *testing.T) {
	transport, err := NewReusePortTransport(":0", nil)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	networks := transport.SupportedNetworks()
	if len(networks) == 0 {
		t.Error("Should support at least one network type")
	}

	hasUDP := false
	for _, n := range networks {
		if n == "udp" {
			hasUDP = true
			break
		}
	}
	if !hasUDP {
		t.Error("Should support UDP network")
	}
}

func TestReusePortTransport_NoWorkerPool(t *testing.T) {
	config := &ReusePortConfig{
		NumSockets: 1,
		WorkerPool: nil, // No worker pool
	}

	transport, err := NewReusePortTransport(":0", config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	stats := transport.Stats()
	if stats.WorkerPoolStats != nil {
		t.Error("Worker pool stats should be nil when no pool configured")
	}
}

func TestReusePortStats_Fields(t *testing.T) {
	stats := ReusePortStats{
		NumSockets:      4,
		PacketsReceived: 1000,
		PacketsSent:     900,
		BytesReceived:   100000,
		BytesSent:       90000,
		SendErrors:      5,
		ReceiveErrors:   3,
	}

	if stats.NumSockets != 4 {
		t.Errorf("Expected 4 sockets, got %d", stats.NumSockets)
	}
	if stats.PacketsReceived != 1000 {
		t.Errorf("Expected 1000 packets received, got %d", stats.PacketsReceived)
	}
}

func BenchmarkReusePortTransport_Send(b *testing.B) {
	transport, err := NewReusePortTransport(":0", nil)
	if err != nil {
		b.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	receiver, err := net.ListenPacket("udp", ":0")
	if err != nil {
		b.Fatalf("Failed to create receiver: %v", err)
	}
	defer receiver.Close()

	packet := &Packet{PacketType: PacketPingRequest, Data: make([]byte, 100)}
	addr := receiver.LocalAddr()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transport.Send(packet, addr)
	}
}

func BenchmarkReusePortTransport_SendReceive(b *testing.B) {
	config := &ReusePortConfig{
		NumSockets: runtime.NumCPU(),
		WorkerPool: &WorkerPoolConfig{
			NumWorkers: 100,
			QueueSize:  10000,
			DropOnFull: true,
		},
	}

	transport, err := NewReusePortTransport(":0", config)
	if err != nil {
		b.Fatalf("Failed to create transport: %v", err)
	}
	defer transport.Close()

	var received int64
	handler := func(p *Packet, a net.Addr) error {
		atomic.AddInt64(&received, 1)
		return nil
	}
	transport.RegisterHandler(PacketPingRequest, handler)

	sender, err := net.ListenPacket("udp", ":0")
	if err != nil {
		b.Fatalf("Failed to create sender: %v", err)
	}
	defer sender.Close()

	packet := &Packet{PacketType: PacketPingRequest, Data: make([]byte, 100)}
	data, _ := packet.Serialize()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sender.WriteTo(data, transport.LocalAddr())
	}

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)
}
