package toxcore

import (
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/factory"
	"github.com/opd-ai/toxcore/interfaces"
	testsim "github.com/opd-ai/toxcore/testing"
)

func TestPacketDeliveryMigration(t *testing.T) {
	// Create Tox instance
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test 1: Verify initial state (may be real or simulation depending on environment)
	isSimulation := tox.IsPacketDeliverySimulation()
	t.Logf("Initial packet delivery mode: simulation=%v", isSimulation)

	// Test 2: Verify packet delivery stats are available
	stats := tox.GetPacketDeliveryStats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}
	t.Logf("Initial stats: %+v", stats)

	// Test 3: Test switching to real implementation
	err = tox.SetPacketDeliveryMode(false)
	if err != nil {
		t.Errorf("Failed to switch to real mode: %v", err)
	}

	// Note: May still be simulation if no transport available
	// This is expected behavior for test environment

	// Test 4: Test switching back to simulation
	err = tox.SetPacketDeliveryMode(true)
	if err != nil {
		t.Errorf("Failed to switch to simulation mode: %v", err)
	}
	if !tox.IsPacketDeliverySimulation() {
		t.Error("Should be using simulation after switch")
	}

	// Test 5: Test friend address management
	friendID := uint32(1)
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")

	err = tox.AddFriendAddress(friendID, addr)
	if err != nil {
		t.Errorf("Failed to add friend address: %v", err)
	}

	err = tox.RemoveFriendAddress(friendID)
	if err != nil {
		t.Errorf("Failed to remove friend address: %v", err)
	}
}

func TestPacketDeliveryInterface(t *testing.T) {
	// Test packet delivery interface directly
	config := &interfaces.PacketDeliveryConfig{
		UseSimulation:   true,
		NetworkTimeout:  1000,
		RetryAttempts:   1,
		EnableBroadcast: true,
	}

	// Create simulation implementation
	simDelivery := testsim.NewSimulatedPacketDelivery(config)
	if simDelivery == nil {
		t.Fatal("Failed to create simulation delivery")
	}
	if !simDelivery.IsSimulation() {
		t.Error("Should be simulation implementation")
	}

	// Test adding a friend
	friendID := uint32(1)
	simDelivery.AddFriend(friendID, nil)

	// Test packet delivery
	packet := []byte("test message")
	err := simDelivery.DeliverPacket(friendID, packet)
	if err != nil {
		t.Errorf("Failed to deliver packet: %v", err)
	}

	// Verify delivery log
	log := simDelivery.GetDeliveryLog()
	if len(log) != 1 {
		t.Errorf("Expected 1 delivery, got %d", len(log))
	}
	if log[0].FriendID != friendID {
		t.Errorf("Expected friend ID %d, got %d", friendID, log[0].FriendID)
	}
	if log[0].PacketSize != len(packet) {
		t.Errorf("Expected packet size %d, got %d", len(packet), log[0].PacketSize)
	}
	if !log[0].Success {
		t.Error("Delivery should have been successful")
	}

	// Test broadcast
	err = simDelivery.BroadcastPacket(packet, nil)
	if err != nil {
		t.Errorf("Failed to broadcast packet: %v", err)
	}

	// Verify broadcast delivery
	log = simDelivery.GetDeliveryLog()
	if len(log) != 2 {
		t.Errorf("Expected 2 deliveries after broadcast, got %d", len(log))
	}

	// Test stats
	stats := simDelivery.GetStats()
	if stats["total_friends"] != 1 {
		t.Errorf("Expected 1 friend, got %v", stats["total_friends"])
	}
	if stats["total_deliveries"] != 2 {
		t.Errorf("Expected 2 deliveries, got %v", stats["total_deliveries"])
	}
	if stats["successful_deliveries"] != 2 {
		t.Errorf("Expected 2 successful deliveries, got %v", stats["successful_deliveries"])
	}
	if stats["failed_deliveries"] != 0 {
		t.Errorf("Expected 0 failed deliveries, got %v", stats["failed_deliveries"])
	}
}

func TestDeprecatedSimulatePacketDelivery(t *testing.T) {
	// Test that the deprecated function still works but uses new interface
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Set up simulation mode
	err = tox.SetPacketDeliveryMode(true)
	if err != nil {
		t.Fatalf("Failed to set simulation mode: %v", err)
	}

	// Add a friend to the simulation
	friendID := uint32(1)
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")
	err = tox.AddFriendAddress(friendID, addr)
	if err != nil {
		t.Fatalf("Failed to add friend address: %v", err)
	}

	// Test deprecated function
	packet := []byte("test packet")
	tox.simulatePacketDelivery(friendID, packet)

	// Verify packet was processed through new interface
	stats := tox.GetPacketDeliveryStats()
	if stats["is_simulation"] != true {
		t.Error("Should be using simulation")
	}

	// In simulation mode, delivery should be successful
	deliveries := stats["total_deliveries"]
	if deliveries == nil || deliveries.(int) <= 0 {
		t.Error("Should have at least one delivery recorded")
	}
}

func TestPacketDeliveryFactoryMigration(t *testing.T) {
	// Test factory creation and configuration
	factoryInstance := factory.NewPacketDeliveryFactory()
	if factoryInstance == nil {
		t.Fatal("Failed to create factory")
	}

	// Test default configuration
	config := factoryInstance.GetCurrentConfig()
	t.Logf("Factory default config: UseSimulation=%v, Timeout=%d, Retries=%d, Broadcast=%v",
		config.UseSimulation, config.NetworkTimeout, config.RetryAttempts, config.EnableBroadcast)

	if config.NetworkTimeout != 5000 {
		t.Errorf("Expected timeout 5000, got %d", config.NetworkTimeout)
	}
	if config.RetryAttempts != 3 {
		t.Errorf("Expected 3 retry attempts, got %d", config.RetryAttempts)
	}
	if !config.EnableBroadcast {
		t.Error("Broadcast should be enabled by default")
	}

	// Test switching modes
	factoryInstance.SwitchToSimulation()
	if !factoryInstance.IsUsingSimulation() {
		t.Error("Should be using simulation after switch")
	}

	factoryInstance.SwitchToReal()
	if factoryInstance.IsUsingSimulation() {
		t.Error("Should be using real implementation after switch")
	}

	// Test creating simulation for testing
	simDelivery := factoryInstance.CreateSimulationForTesting()
	if !simDelivery.IsSimulation() {
		t.Error("Should create simulation implementation")
	}

	// Test custom configuration
	customConfig := &interfaces.PacketDeliveryConfig{
		UseSimulation:   true,
		NetworkTimeout:  2000,
		RetryAttempts:   5,
		EnableBroadcast: false,
	}

	err := factoryInstance.UpdateConfig(customConfig)
	if err != nil {
		t.Errorf("Failed to update config: %v", err)
	}

	updatedConfig := factoryInstance.GetCurrentConfig()
	if !updatedConfig.UseSimulation {
		t.Error("Should be using simulation after config update")
	}
	if updatedConfig.NetworkTimeout != 2000 {
		t.Errorf("Expected timeout 2000, got %d", updatedConfig.NetworkTimeout)
	}
	if updatedConfig.RetryAttempts != 5 {
		t.Errorf("Expected 5 retry attempts, got %d", updatedConfig.RetryAttempts)
	}
	if updatedConfig.EnableBroadcast {
		t.Error("Broadcast should be disabled after config update")
	}
}

func TestMigrationBackwardCompatibility(t *testing.T) {
	// Ensure existing tests still pass with new system
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test that basic functionality still works
	publicKey := tox.GetSelfPublicKey()
	if publicKey == [32]byte{} {
		t.Error("Public key should not be empty")
	}

	// Test that iteration still works
	start := time.Now()
	tox.Iterate()
	duration := time.Since(start)
	if duration > time.Second {
		t.Error("Iteration should be fast")
	}

	// Test that the deprecated simulate function doesn't break things
	friendID := uint32(999)
	packet := []byte("compatibility test")

	// This should not panic or error even if friend doesn't exist
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("simulatePacketDelivery panicked: %v", r)
			}
		}()
		tox.simulatePacketDelivery(friendID, packet)
	}()

	// Test that we can still access other functionality
	_ = tox.GetAsyncStorageStats()
	// stats may be nil if async is disabled, that's okay
	// Just verify no panic occurs
}
