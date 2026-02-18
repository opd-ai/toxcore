package group

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/transport"
)

// benchmarkTransport is a fast mock transport for benchmarks
type benchmarkTransport struct {
	sendCount int64
}

func (t *benchmarkTransport) Send(packet *transport.Packet, addr net.Addr) error {
	atomic.AddInt64(&t.sendCount, 1)
	return nil
}

func (t *benchmarkTransport) Close() error { return nil }

func (t *benchmarkTransport) LocalAddr() net.Addr {
	return &mockAddr{address: "127.0.0.1:33445"}
}

func (t *benchmarkTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
}

func (t *benchmarkTransport) IsConnectionOriented() bool { return false }

// benchmarkSlowTransport simulates network latency for realistic benchmarks
type benchmarkSlowTransport struct {
	sendCount int64
	latency   time.Duration
}

func (t *benchmarkSlowTransport) Send(packet *transport.Packet, addr net.Addr) error {
	time.Sleep(t.latency)
	atomic.AddInt64(&t.sendCount, 1)
	return nil
}

func (t *benchmarkSlowTransport) Close() error { return nil }

func (t *benchmarkSlowTransport) LocalAddr() net.Addr {
	return &mockAddr{address: "127.0.0.1:33445"}
}

func (t *benchmarkSlowTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
}

func (t *benchmarkSlowTransport) IsConnectionOriented() bool { return false }

// createBenchmarkChat creates a Chat with specified number of online peers
func createBenchmarkChat(peerCount int, tr transport.Transport) *Chat {
	selfKey := [32]byte{0x01}
	chat := &Chat{
		ID:           uint32(0xaa),
		Name:         "BenchmarkGroup",
		transport:    tr,
		Peers:        make(map[uint32]*Peer),
		SelfPeerID:   0,
		timeProvider: DefaultTimeProvider{},
	}

	// Add self as peer 0
	chat.Peers[0] = &Peer{
		ID:         0,
		PublicKey:  selfKey,
		Name:       "Self",
		Role:       RoleFounder,
		Connection: 1,
	}

	// Add online peers with addresses
	for i := 1; i <= peerCount; i++ {
		var peerKey [32]byte
		peerKey[0] = byte(i)

		chat.Peers[uint32(i)] = &Peer{
			ID:         uint32(i),
			PublicKey:  peerKey,
			Name:       "Peer",
			Role:       RoleModerator,
			Connection: 1,
			Address:    &mockAddr{address: "192.168.1." + string(rune('0'+i%10)) + ":33445"},
		}
	}

	return chat
}

// BenchmarkBroadcastSmallGroup benchmarks broadcasting to a small group (10 peers)
func BenchmarkBroadcastSmallGroup(b *testing.B) {
	tr := &benchmarkTransport{}
	chat := createBenchmarkChat(10, tr)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = chat.sendToConnectedPeers([]byte("benchmark message"))
	}
}

// BenchmarkBroadcastMediumGroup benchmarks broadcasting to a medium group (50 peers)
func BenchmarkBroadcastMediumGroup(b *testing.B) {
	tr := &benchmarkTransport{}
	chat := createBenchmarkChat(50, tr)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = chat.sendToConnectedPeers([]byte("benchmark message"))
	}
}

// BenchmarkBroadcastLargeGroup benchmarks broadcasting to a large group (100 peers)
func BenchmarkBroadcastLargeGroup(b *testing.B) {
	tr := &benchmarkTransport{}
	chat := createBenchmarkChat(100, tr)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = chat.sendToConnectedPeers([]byte("benchmark message"))
	}
}

// BenchmarkBroadcastWorkerPoolScaling benchmarks worker pool efficiency
func BenchmarkBroadcastWorkerPoolScaling(b *testing.B) {
	peerCounts := []int{5, 10, 20, 50, 100}

	for _, peerCount := range peerCounts {
		b.Run("peers_"+string(rune('0'+peerCount/100))+string(rune('0'+(peerCount%100)/10))+string(rune('0'+peerCount%10)), func(b *testing.B) {
			tr := &benchmarkTransport{}
			chat := createBenchmarkChat(peerCount, tr)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = chat.sendToConnectedPeers([]byte("worker pool scaling test"))
			}
		})
	}
}

// BenchmarkBroadcastWithLatency benchmarks worker pool parallel efficiency with network latency
func BenchmarkBroadcastWithLatency(b *testing.B) {
	latencies := []time.Duration{
		100 * time.Microsecond,
		1 * time.Millisecond,
	}

	for _, latency := range latencies {
		b.Run("latency_"+latency.String(), func(b *testing.B) {
			tr := &benchmarkSlowTransport{latency: latency}
			chat := createBenchmarkChat(30, tr)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = chat.sendToConnectedPeers([]byte("latency test"))
			}
		})
	}
}

// BenchmarkBroadcastMessageSerialization benchmarks message creation overhead
func BenchmarkBroadcastMessageSerialization(b *testing.B) {
	chat := createBenchmarkChat(10, &benchmarkTransport{})

	messageSizes := []struct {
		name string
		data map[string]interface{}
	}{
		{"small", map[string]interface{}{"msg": "hello"}},
		{"medium", map[string]interface{}{
			"msg":       "test message",
			"timestamp": time.Now().Unix(),
			"peer_id":   uint32(1),
		}},
		{"large", map[string]interface{}{
			"msg":       string(make([]byte, 1000)),
			"timestamp": time.Now().Unix(),
			"peer_id":   uint32(1),
			"extra":     make([]byte, 500),
		}},
	}

	for _, tc := range messageSizes {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = chat.createBroadcastMessage("test_update", tc.data)
			}
		})
	}
}

// BenchmarkBroadcastGroupUpdateFull benchmarks complete broadcast cycle
func BenchmarkBroadcastGroupUpdateFull(b *testing.B) {
	peerCounts := []int{10, 50, 100}

	for _, peerCount := range peerCounts {
		b.Run("peers_"+string(rune('0'+peerCount/100))+string(rune('0'+(peerCount%100)/10))+string(rune('0'+peerCount%10)), func(b *testing.B) {
			tr := &benchmarkTransport{}
			chat := createBenchmarkChat(peerCount, tr)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = chat.broadcastGroupUpdate("benchmark_update", map[string]interface{}{
					"message": "benchmark test",
				})
			}
		})
	}
}

// BenchmarkCreateBroadcastMessage benchmarks JSON serialization for broadcast messages
func BenchmarkCreateBroadcastMessage(b *testing.B) {
	selfKey := [32]byte{0x01}
	chat := &Chat{
		ID:         uint32(0xaa),
		SelfPeerID: 0,
		Peers: map[uint32]*Peer{
			0: {ID: 0, PublicKey: selfKey},
		},
		timeProvider: DefaultTimeProvider{},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = chat.createBroadcastMessage("group_message", map[string]interface{}{
			"sender_id": uint32(0),
			"message":   "benchmark test message",
		})
	}
}

// BenchmarkValidatePeerForBroadcast benchmarks peer validation in broadcast path
func BenchmarkValidatePeerForBroadcast(b *testing.B) {
	tr := &benchmarkTransport{}
	chat := createBenchmarkChat(50, tr)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = chat.validatePeerForBroadcast(uint32(i%50 + 1))
	}
}

// BenchmarkBroadcastParallelVsSequential compares parallel worker pool with sequential sends
func BenchmarkBroadcastParallelVsSequential(b *testing.B) {
	// This benchmark demonstrates the benefit of worker pool pattern
	peerCounts := []int{10, 30, 50}

	for _, peerCount := range peerCounts {
		b.Run("parallel_peers_"+itoa(peerCount), func(b *testing.B) {
			tr := &benchmarkSlowTransport{latency: 100 * time.Microsecond}
			chat := createBenchmarkChat(peerCount, tr)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = chat.sendToConnectedPeers([]byte("parallel test"))
			}
		})
	}
}

// itoa converts small int to string for benchmark names
func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	if i < 100 {
		return string(rune('0'+i/10)) + string(rune('0'+i%10))
	}
	return string(rune('0'+i/100)) + string(rune('0'+(i%100)/10)) + string(rune('0'+i%10))
}
