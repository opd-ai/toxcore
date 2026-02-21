package testing

import (
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/interfaces"
)

func newTestConfig() *interfaces.PacketDeliveryConfig {
	return &interfaces.PacketDeliveryConfig{
		UseSimulation:   true,
		NetworkTimeout:  5000,
		RetryAttempts:   3,
		EnableBroadcast: true,
	}
}

func TestNewSimulatedPacketDelivery(t *testing.T) {
	config := newTestConfig()
	sim := NewSimulatedPacketDelivery(config)

	if sim == nil {
		t.Fatal("expected non-nil SimulatedPacketDelivery")
	}
	if !sim.IsSimulation() {
		t.Error("IsSimulation should return true")
	}
	if len(sim.GetDeliveryLog()) != 0 {
		t.Error("new simulation should have empty delivery log")
	}
}

func TestAddFriend(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())

	sim.AddFriend(1, nil) // addr is ignored in simulation
	sim.AddFriend(2, nil)

	stats := sim.GetTypedStats()
	if stats.FriendCount != 2 {
		t.Errorf("expected 2 friends, got %d", stats.FriendCount)
	}
}

func TestAddFriendIdempotent(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())

	sim.AddFriend(1, nil)
	sim.AddFriend(1, nil) // Adding same friend twice

	stats := sim.GetTypedStats()
	if stats.FriendCount != 1 {
		t.Errorf("expected 1 friend after duplicate add, got %d", stats.FriendCount)
	}
}

func TestRemoveFriend(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())

	sim.AddFriend(1, nil)
	sim.AddFriend(2, nil)
	sim.RemoveFriend(1)

	stats := sim.GetTypedStats()
	if stats.FriendCount != 1 {
		t.Errorf("expected 1 friend after removal, got %d", stats.FriendCount)
	}
}

func TestRemoveNonexistentFriend(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())

	// Should not panic when removing non-existent friend
	sim.RemoveFriend(999)

	stats := sim.GetTypedStats()
	if stats.FriendCount != 0 {
		t.Errorf("expected 0 friends, got %d", stats.FriendCount)
	}
}

func TestDeliverPacketSuccess(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())
	sim.AddFriend(1, nil)

	packet := []byte("hello world")
	beforeDelivery := time.Now().UnixNano()

	err := sim.DeliverPacket(1, packet)

	afterDelivery := time.Now().UnixNano()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log := sim.GetDeliveryLog()
	if len(log) != 1 {
		t.Fatalf("expected 1 delivery record, got %d", len(log))
	}

	record := log[0]
	if record.FriendID != 1 {
		t.Errorf("expected FriendID 1, got %d", record.FriendID)
	}
	if record.PacketSize != len(packet) {
		t.Errorf("expected PacketSize %d, got %d", len(packet), record.PacketSize)
	}
	if !record.Success {
		t.Error("expected successful delivery")
	}
	if record.Error != nil {
		t.Errorf("expected nil error, got %v", record.Error)
	}
	if record.Timestamp < beforeDelivery || record.Timestamp > afterDelivery {
		t.Errorf("timestamp %d not in expected range [%d, %d]",
			record.Timestamp, beforeDelivery, afterDelivery)
	}
}

func TestDeliverPacketFriendNotFound(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())

	packet := []byte("hello")
	err := sim.DeliverPacket(999, packet)

	if err == nil {
		t.Fatal("expected error for non-existent friend")
	}

	log := sim.GetDeliveryLog()
	if len(log) != 1 {
		t.Fatalf("expected 1 delivery record, got %d", len(log))
	}

	record := log[0]
	if record.FriendID != 999 {
		t.Errorf("expected FriendID 999, got %d", record.FriendID)
	}
	if record.Success {
		t.Error("expected failed delivery")
	}
	if record.Error == nil {
		t.Error("expected non-nil error in record")
	}
	if record.Timestamp == 0 {
		t.Error("expected non-zero timestamp")
	}
}

func TestDeliverPacketEmptyPacket(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())
	sim.AddFriend(1, nil)

	err := sim.DeliverPacket(1, []byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log := sim.GetDeliveryLog()
	if len(log) != 1 || log[0].PacketSize != 0 {
		t.Error("expected delivery of empty packet")
	}
}

func TestBroadcastPacket(t *testing.T) {
	config := newTestConfig()
	sim := NewSimulatedPacketDelivery(config)

	sim.AddFriend(1, nil)
	sim.AddFriend(2, nil)
	sim.AddFriend(3, nil)

	packet := []byte("broadcast message")
	err := sim.BroadcastPacket(packet, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log := sim.GetDeliveryLog()
	if len(log) != 3 {
		t.Errorf("expected 3 delivery records, got %d", len(log))
	}

	// Verify all deliveries were successful
	for _, record := range log {
		if !record.Success {
			t.Errorf("expected successful delivery for friend %d", record.FriendID)
		}
		if record.PacketSize != len(packet) {
			t.Errorf("expected packet size %d, got %d", len(packet), record.PacketSize)
		}
		if record.Timestamp == 0 {
			t.Error("expected non-zero timestamp")
		}
	}
}

func TestBroadcastPacketWithExclusions(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())

	sim.AddFriend(1, nil)
	sim.AddFriend(2, nil)
	sim.AddFriend(3, nil)

	packet := []byte("selective broadcast")
	excludeFriends := []uint32{2}

	err := sim.BroadcastPacket(packet, excludeFriends)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log := sim.GetDeliveryLog()
	if len(log) != 2 {
		t.Errorf("expected 2 delivery records (excluding friend 2), got %d", len(log))
	}

	// Verify friend 2 was excluded
	for _, record := range log {
		if record.FriendID == 2 {
			t.Error("friend 2 should have been excluded from broadcast")
		}
	}
}

func TestBroadcastPacketDisabled(t *testing.T) {
	config := newTestConfig()
	config.EnableBroadcast = false
	sim := NewSimulatedPacketDelivery(config)

	sim.AddFriend(1, nil)

	err := sim.BroadcastPacket([]byte("test"), nil)

	if err == nil {
		t.Error("expected error when broadcast is disabled")
	}
}

func TestBroadcastToNoFriends(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())

	err := sim.BroadcastPacket([]byte("no recipients"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log := sim.GetDeliveryLog()
	if len(log) != 0 {
		t.Errorf("expected 0 delivery records, got %d", len(log))
	}
}

func TestSetNetworkTransport(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())

	// SetNetworkTransport is a no-op for simulation
	err := sim.SetNetworkTransport(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetDeliveryLogIsCopy(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())
	sim.AddFriend(1, nil)

	sim.DeliverPacket(1, []byte("test"))

	log1 := sim.GetDeliveryLog()
	log2 := sim.GetDeliveryLog()

	// Modify the returned log
	log1[0].Success = false

	// Original should be unchanged
	if !log2[0].Success {
		t.Error("GetDeliveryLog should return a copy, not the original")
	}
}

func TestClearDeliveryLog(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())
	sim.AddFriend(1, nil)

	sim.DeliverPacket(1, []byte("test1"))
	sim.DeliverPacket(1, []byte("test2"))

	if len(sim.GetDeliveryLog()) != 2 {
		t.Fatal("expected 2 delivery records before clear")
	}

	sim.ClearDeliveryLog()

	if len(sim.GetDeliveryLog()) != 0 {
		t.Error("expected empty delivery log after clear")
	}

	// Friends should still be registered
	stats := sim.GetTypedStats()
	if stats.FriendCount != 1 {
		t.Error("friends should remain after clearing log")
	}
}

// TestGetStats verifies the deprecated GetStats method still works for backward compatibility.
// New code should use GetTypedStats instead.
func TestGetStats(t *testing.T) {
	config := newTestConfig()
	sim := NewSimulatedPacketDelivery(config)

	sim.AddFriend(1, nil)
	sim.AddFriend(2, nil)
	sim.DeliverPacket(1, []byte("success"))
	sim.DeliverPacket(999, []byte("fail")) // Non-existent friend

	stats := sim.GetStats()

	tests := []struct {
		key      string
		expected interface{}
	}{
		{"total_friends", 2},
		{"total_deliveries", 2},
		{"successful_deliveries", 1},
		{"failed_deliveries", 1},
		{"is_simulation", true},
		{"broadcast_enabled", true},
		{"retry_attempts", config.RetryAttempts},
		{"network_timeout", config.NetworkTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if stats[tt.key] != tt.expected {
				t.Errorf("stats[%q] = %v, want %v", tt.key, stats[tt.key], tt.expected)
			}
		})
	}
}

// TestGetTypedStats verifies the type-safe statistics method.
func TestGetTypedStats(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())

	sim.AddFriend(1, nil)
	sim.AddFriend(2, nil)
	sim.DeliverPacket(1, []byte("success packet"))
	sim.DeliverPacket(999, []byte("fail")) // Non-existent friend

	stats := sim.GetTypedStats()

	if !stats.IsSimulation {
		t.Error("expected IsSimulation to be true")
	}
	if stats.FriendCount != 2 {
		t.Errorf("expected FriendCount=2, got %d", stats.FriendCount)
	}
	if stats.PacketsSent != 2 {
		t.Errorf("expected PacketsSent=2, got %d", stats.PacketsSent)
	}
	if stats.PacketsDelivered != 1 {
		t.Errorf("expected PacketsDelivered=1, got %d", stats.PacketsDelivered)
	}
	if stats.PacketsFailed != 1 {
		t.Errorf("expected PacketsFailed=1, got %d", stats.PacketsFailed)
	}
	// BytesSent = len("success packet") + len("fail") = 14 + 4 = 18
	expectedBytes := int64(len("success packet") + len("fail"))
	if stats.BytesSent != expectedBytes {
		t.Errorf("expected BytesSent=%d, got %d", expectedBytes, stats.BytesSent)
	}
	// AverageLatencyMs is always 0 for simulation
	if stats.AverageLatencyMs != 0 {
		t.Errorf("expected AverageLatencyMs=0, got %f", stats.AverageLatencyMs)
	}
}

func TestConcurrentDelivery(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())

	// Add friends
	for i := uint32(1); i <= 10; i++ {
		sim.AddFriend(i, nil)
	}

	var wg sync.WaitGroup
	deliveriesPerGoroutine := 10
	numGoroutines := 10

	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < deliveriesPerGoroutine; i++ {
				friendID := uint32((goroutineID % 10) + 1)
				sim.DeliverPacket(friendID, []byte("concurrent test"))
			}
		}(g)
	}

	wg.Wait()

	log := sim.GetDeliveryLog()
	expected := numGoroutines * deliveriesPerGoroutine
	if len(log) != expected {
		t.Errorf("expected %d delivery records, got %d", expected, len(log))
	}

	stats := sim.GetTypedStats()
	if stats.PacketsDelivered != int64(expected) {
		t.Errorf("expected %d successful deliveries, got %d",
			expected, stats.PacketsDelivered)
	}
}

func TestConcurrentFriendManagement(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrently add and remove friends
	wg.Add(numGoroutines * 2)
	for g := 0; g < numGoroutines; g++ {
		friendID := uint32(g + 1)

		go func(id uint32) {
			defer wg.Done()
			sim.AddFriend(id, nil)
		}(friendID)

		go func(id uint32) {
			defer wg.Done()
			// Small delay to increase chance of concurrent access
			time.Sleep(time.Microsecond)
			sim.RemoveFriend(id)
		}(friendID)
	}

	wg.Wait()

	// Just verify no panic occurred and we can still get stats
	stats := sim.GetTypedStats()
	if !stats.IsSimulation {
		t.Error("simulation flag should remain true")
	}
}

func TestDeliveryRecordTimestampOrdering(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())
	sim.AddFriend(1, nil)

	// Send multiple packets with small delays
	for i := 0; i < 5; i++ {
		sim.DeliverPacket(1, []byte("ordered test"))
		time.Sleep(time.Microsecond)
	}

	log := sim.GetDeliveryLog()
	for i := 1; i < len(log); i++ {
		if log[i].Timestamp < log[i-1].Timestamp {
			t.Error("timestamps should be monotonically increasing")
		}
	}
}

// TestConcurrentLogClearing tests race conditions when clearing the delivery log
// while other operations are in progress. This covers edge cases for concurrent
// access to the delivery log during clear operations.
func TestConcurrentLogClearing(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())

	// Add friends for delivery
	for i := uint32(1); i <= 5; i++ {
		sim.AddFriend(i, nil)
	}

	var wg sync.WaitGroup
	numOperations := 50

	// Edge case 1: Concurrent delivery during log clearing
	wg.Add(3)

	// Goroutine 1: Continuously deliver packets
	go func() {
		defer wg.Done()
		for i := 0; i < numOperations; i++ {
			sim.DeliverPacket(uint32((i%5)+1), []byte("clear test"))
		}
	}()

	// Goroutine 2: Continuously clear log
	go func() {
		defer wg.Done()
		for i := 0; i < numOperations/5; i++ {
			sim.ClearDeliveryLog()
			time.Sleep(time.Microsecond)
		}
	}()

	// Goroutine 3: Continuously read log
	go func() {
		defer wg.Done()
		for i := 0; i < numOperations; i++ {
			_ = sim.GetDeliveryLog()
		}
	}()

	wg.Wait()

	// Verify simulation is still in valid state
	stats := sim.GetTypedStats()
	if !stats.IsSimulation {
		t.Error("simulation flag should remain true after concurrent operations")
	}

	// Edge case 2: Clear during stats access
	sim.ClearDeliveryLog()
	for i := 0; i < 10; i++ {
		sim.DeliverPacket(1, []byte("after clear"))
	}

	wg.Add(2)

	// Concurrent stats reading and clearing
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = sim.GetTypedStats()
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			sim.ClearDeliveryLog()
			time.Sleep(time.Microsecond)
		}
	}()

	wg.Wait()

	// Edge case 3: Rapid clear-deliver-clear cycle
	for i := 0; i < 10; i++ {
		sim.ClearDeliveryLog()
		sim.DeliverPacket(1, []byte("rapid"))
		stats := sim.GetTypedStats()
		// Stats should reflect at most 1 packet (may be 0 if cleared after)
		if stats.PacketsSent < 0 || stats.PacketsSent > 1 {
			t.Errorf("unexpected PacketsSent after rapid cycle: %d", stats.PacketsSent)
		}
		sim.ClearDeliveryLog()
	}
}

func TestIsSimulation(t *testing.T) {
	sim := NewSimulatedPacketDelivery(newTestConfig())

	if !sim.IsSimulation() {
		t.Error("IsSimulation should return true")
	}
}

// Benchmark tests

func BenchmarkDeliverPacket(b *testing.B) {
	sim := NewSimulatedPacketDelivery(newTestConfig())
	sim.AddFriend(1, nil)
	packet := []byte("benchmark packet data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sim.DeliverPacket(1, packet)
	}
}

func BenchmarkBroadcastPacket(b *testing.B) {
	sim := NewSimulatedPacketDelivery(newTestConfig())
	for i := uint32(1); i <= 100; i++ {
		sim.AddFriend(i, nil)
	}
	packet := []byte("broadcast benchmark data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sim.BroadcastPacket(packet, nil)
	}
}

func BenchmarkConcurrentDelivery(b *testing.B) {
	sim := NewSimulatedPacketDelivery(newTestConfig())
	for i := uint32(1); i <= 10; i++ {
		sim.AddFriend(i, nil)
	}
	packet := []byte("concurrent benchmark data")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		friendID := uint32(1)
		for pb.Next() {
			sim.DeliverPacket(friendID, packet)
			friendID = (friendID % 10) + 1
		}
	})
}
