package factory

import (
	"net"
	"os"
	"testing"

	"github.com/opd-ai/toxcore/interfaces"
)

// mockTransport implements interfaces.INetworkTransport for testing
type mockTransport struct {
	connected bool
	friends   map[uint32]net.Addr
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		connected: true,
		friends:   make(map[uint32]net.Addr),
	}
}

func (m *mockTransport) Send(packet []byte, addr net.Addr) error {
	return nil
}

func (m *mockTransport) SendToFriend(friendID uint32, packet []byte) error {
	return nil
}

func (m *mockTransport) GetFriendAddress(friendID uint32) (net.Addr, error) {
	if addr, ok := m.friends[friendID]; ok {
		return addr, nil
	}
	return nil, nil
}

func (m *mockTransport) RegisterFriend(friendID uint32, addr net.Addr) error {
	m.friends[friendID] = addr
	return nil
}

func (m *mockTransport) Close() error {
	m.connected = false
	return nil
}

func (m *mockTransport) IsConnected() bool {
	return m.connected
}

// TestNewPacketDeliveryFactory verifies default factory creation
func TestNewPacketDeliveryFactory(t *testing.T) {
	factory := NewPacketDeliveryFactory()

	if factory == nil {
		t.Fatal("NewPacketDeliveryFactory returned nil")
	}

	config := factory.GetCurrentConfig()
	if config == nil {
		t.Fatal("GetCurrentConfig returned nil")
	}

	// Verify default values (without environment overrides)
	if config.NetworkTimeout != 5000 && os.Getenv("TOX_NETWORK_TIMEOUT") == "" {
		t.Errorf("expected default NetworkTimeout 5000, got %d", config.NetworkTimeout)
	}
	if config.RetryAttempts != 3 && os.Getenv("TOX_RETRY_ATTEMPTS") == "" {
		t.Errorf("expected default RetryAttempts 3, got %d", config.RetryAttempts)
	}
	if config.EnableBroadcast != true && os.Getenv("TOX_ENABLE_BROADCAST") == "" {
		t.Errorf("expected default EnableBroadcast true, got %v", config.EnableBroadcast)
	}
}

// TestEnvironmentVariableParsing verifies environment variable handling
func TestEnvironmentVariableParsing(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		checkFunc    func(*interfaces.PacketDeliveryConfig) bool
		description  string
		restoreValue string
	}{
		{
			name:        "valid_simulation_true",
			envKey:      "TOX_USE_SIMULATION",
			envValue:    "true",
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.UseSimulation == true },
			description: "UseSimulation should be true",
		},
		{
			name:        "valid_simulation_false",
			envKey:      "TOX_USE_SIMULATION",
			envValue:    "false",
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.UseSimulation == false },
			description: "UseSimulation should be false",
		},
		{
			name:        "valid_timeout",
			envKey:      "TOX_NETWORK_TIMEOUT",
			envValue:    "10000",
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.NetworkTimeout == 10000 },
			description: "NetworkTimeout should be 10000",
		},
		{
			name:        "valid_retries",
			envKey:      "TOX_RETRY_ATTEMPTS",
			envValue:    "5",
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.RetryAttempts == 5 },
			description: "RetryAttempts should be 5",
		},
		{
			name:        "valid_broadcast_false",
			envKey:      "TOX_ENABLE_BROADCAST",
			envValue:    "false",
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.EnableBroadcast == false },
			description: "EnableBroadcast should be false",
		},
		{
			name:        "invalid_simulation_value",
			envKey:      "TOX_USE_SIMULATION",
			envValue:    "invalid",
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.UseSimulation == false }, // Falls back to default
			description: "UseSimulation should fall back to default (false) on invalid value",
		},
		{
			name:        "invalid_timeout_value",
			envKey:      "TOX_NETWORK_TIMEOUT",
			envValue:    "not_a_number",
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.NetworkTimeout == 5000 }, // Falls back to default
			description: "NetworkTimeout should fall back to default (5000) on invalid value",
		},
		{
			name:        "invalid_retries_value",
			envKey:      "TOX_RETRY_ATTEMPTS",
			envValue:    "abc",
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.RetryAttempts == 3 }, // Falls back to default
			description: "RetryAttempts should fall back to default (3) on invalid value",
		},
		// Bounds checking tests
		{
			name:        "timeout_below_minimum",
			envKey:      "TOX_NETWORK_TIMEOUT",
			envValue:    "50",                                                                              // Below MinNetworkTimeout (100)
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.NetworkTimeout == 5000 }, // Falls back to default
			description: "NetworkTimeout should fall back to default (5000) when below minimum",
		},
		{
			name:        "timeout_negative",
			envKey:      "TOX_NETWORK_TIMEOUT",
			envValue:    "-1000",
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.NetworkTimeout == 5000 }, // Falls back to default
			description: "NetworkTimeout should fall back to default (5000) when negative",
		},
		{
			name:        "timeout_above_maximum",
			envKey:      "TOX_NETWORK_TIMEOUT",
			envValue:    "700000",                                                                          // Above MaxNetworkTimeout (600000)
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.NetworkTimeout == 5000 }, // Falls back to default
			description: "NetworkTimeout should fall back to default (5000) when above maximum",
		},
		{
			name:        "timeout_at_minimum",
			envKey:      "TOX_NETWORK_TIMEOUT",
			envValue:    "100", // Exactly MinNetworkTimeout
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.NetworkTimeout == 100 },
			description: "NetworkTimeout should accept value at minimum boundary",
		},
		{
			name:        "timeout_at_maximum",
			envKey:      "TOX_NETWORK_TIMEOUT",
			envValue:    "600000", // Exactly MaxNetworkTimeout
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.NetworkTimeout == 600000 },
			description: "NetworkTimeout should accept value at maximum boundary",
		},
		{
			name:        "retries_negative",
			envKey:      "TOX_RETRY_ATTEMPTS",
			envValue:    "-5",
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.RetryAttempts == 3 }, // Falls back to default
			description: "RetryAttempts should fall back to default (3) when negative",
		},
		{
			name:        "retries_above_maximum",
			envKey:      "TOX_RETRY_ATTEMPTS",
			envValue:    "150",                                                                         // Above MaxRetryAttempts (100)
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.RetryAttempts == 3 }, // Falls back to default
			description: "RetryAttempts should fall back to default (3) when above maximum",
		},
		{
			name:        "retries_at_zero",
			envKey:      "TOX_RETRY_ATTEMPTS",
			envValue:    "0", // Exactly MinRetryAttempts
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.RetryAttempts == 0 },
			description: "RetryAttempts should accept zero (no retries)",
		},
		{
			name:        "retries_at_maximum",
			envKey:      "TOX_RETRY_ATTEMPTS",
			envValue:    "100", // Exactly MaxRetryAttempts
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.RetryAttempts == 100 },
			description: "RetryAttempts should accept value at maximum boundary",
		},
		{
			name:        "invalid_broadcast_value",
			envKey:      "TOX_ENABLE_BROADCAST",
			envValue:    "invalid_bool",
			checkFunc:   func(c *interfaces.PacketDeliveryConfig) bool { return c.EnableBroadcast == true }, // Falls back to default
			description: "EnableBroadcast should fall back to default (true) on invalid value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore environment variable
			originalValue := os.Getenv(tt.envKey)
			defer os.Setenv(tt.envKey, originalValue)

			// Set test environment variable
			os.Setenv(tt.envKey, tt.envValue)

			// Create factory (which applies environment overrides)
			factory := NewPacketDeliveryFactory()
			config := factory.GetCurrentConfig()

			if !tt.checkFunc(config) {
				t.Errorf("%s failed: %s", tt.name, tt.description)
			}
		})
	}
}

// TestCreatePacketDeliverySimulation verifies simulation mode creation
func TestCreatePacketDeliverySimulation(t *testing.T) {
	factory := NewPacketDeliveryFactory()
	factory.SwitchToSimulation()

	delivery, err := factory.CreatePacketDelivery(nil) // No transport needed for simulation
	if err != nil {
		t.Fatalf("CreatePacketDelivery failed: %v", err)
	}
	if delivery == nil {
		t.Fatal("CreatePacketDelivery returned nil")
	}
	if !delivery.IsSimulation() {
		t.Error("expected simulation implementation")
	}
}

// TestCreatePacketDeliveryReal verifies real mode creation with transport
func TestCreatePacketDeliveryReal(t *testing.T) {
	factory := NewPacketDeliveryFactory()
	factory.SwitchToReal()

	transport := newMockTransport()
	delivery, err := factory.CreatePacketDelivery(transport)
	if err != nil {
		t.Fatalf("CreatePacketDelivery failed: %v", err)
	}
	if delivery == nil {
		t.Fatal("CreatePacketDelivery returned nil")
	}
	if delivery.IsSimulation() {
		t.Error("expected real implementation, got simulation")
	}
}

// TestCreatePacketDeliveryRealNilTransport verifies error on nil transport for real mode
func TestCreatePacketDeliveryRealNilTransport(t *testing.T) {
	factory := NewPacketDeliveryFactory()
	factory.SwitchToReal()

	_, err := factory.CreatePacketDelivery(nil)
	if err == nil {
		t.Error("expected error when creating real implementation without transport")
	}
}

// TestCreateSimulationForTesting verifies test-optimized simulation creation
func TestCreateSimulationForTesting(t *testing.T) {
	factory := NewPacketDeliveryFactory()
	delivery := factory.CreateSimulationForTesting()

	if delivery == nil {
		t.Fatal("CreateSimulationForTesting returned nil")
	}
	if !delivery.IsSimulation() {
		t.Error("expected simulation implementation")
	}
}

// TestSwitchToSimulation verifies mode switching to simulation
func TestSwitchToSimulation(t *testing.T) {
	factory := NewPacketDeliveryFactory()
	factory.SwitchToReal() // Ensure we start in real mode

	if factory.IsUsingSimulation() {
		t.Error("expected real mode before switch")
	}

	factory.SwitchToSimulation()

	if !factory.IsUsingSimulation() {
		t.Error("expected simulation mode after switch")
	}
}

// TestSwitchToReal verifies mode switching to real
func TestSwitchToReal(t *testing.T) {
	factory := NewPacketDeliveryFactory()
	factory.SwitchToSimulation() // Ensure we start in simulation mode

	if !factory.IsUsingSimulation() {
		t.Error("expected simulation mode before switch")
	}

	factory.SwitchToReal()

	if factory.IsUsingSimulation() {
		t.Error("expected real mode after switch")
	}
}

// TestGetCurrentConfigReturnsCopy verifies config is a copy, not reference
func TestGetCurrentConfigReturnsCopy(t *testing.T) {
	factory := NewPacketDeliveryFactory()
	config1 := factory.GetCurrentConfig()
	config2 := factory.GetCurrentConfig()

	// Modify config1 and verify config2 is unchanged
	config1.NetworkTimeout = 99999
	if config2.NetworkTimeout == 99999 {
		t.Error("GetCurrentConfig should return a copy, not reference")
	}
}

// TestUpdateConfigNil verifies error on nil config
func TestUpdateConfigNil(t *testing.T) {
	factory := NewPacketDeliveryFactory()
	err := factory.UpdateConfig(nil)
	if err == nil {
		t.Error("expected error when updating with nil config")
	}
}

// TestUpdateConfigValid verifies successful config update
func TestUpdateConfigValid(t *testing.T) {
	factory := NewPacketDeliveryFactory()

	newConfig := &interfaces.PacketDeliveryConfig{
		UseSimulation:   true,
		NetworkTimeout:  15000,
		RetryAttempts:   7,
		EnableBroadcast: false,
	}

	err := factory.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	config := factory.GetCurrentConfig()
	if config.UseSimulation != true {
		t.Errorf("expected UseSimulation true, got %v", config.UseSimulation)
	}
	if config.NetworkTimeout != 15000 {
		t.Errorf("expected NetworkTimeout 15000, got %d", config.NetworkTimeout)
	}
	if config.RetryAttempts != 7 {
		t.Errorf("expected RetryAttempts 7, got %d", config.RetryAttempts)
	}
	if config.EnableBroadcast != false {
		t.Errorf("expected EnableBroadcast false, got %v", config.EnableBroadcast)
	}
}

// TestIsUsingSimulation verifies simulation mode query
func TestIsUsingSimulation(t *testing.T) {
	factory := NewPacketDeliveryFactory()

	// Test when set to simulation
	factory.SwitchToSimulation()
	if !factory.IsUsingSimulation() {
		t.Error("IsUsingSimulation should return true in simulation mode")
	}

	// Test when set to real
	factory.SwitchToReal()
	if factory.IsUsingSimulation() {
		t.Error("IsUsingSimulation should return false in real mode")
	}
}

// TestCreatePacketDeliveryWithConfigNilFallback verifies nil config uses default
func TestCreatePacketDeliveryWithConfigNilFallback(t *testing.T) {
	factory := NewPacketDeliveryFactory()
	factory.SwitchToSimulation() // Set default to simulation

	// Pass nil config - should fall back to default (simulation)
	delivery, err := factory.CreatePacketDeliveryWithConfig(nil, nil)
	if err != nil {
		t.Fatalf("CreatePacketDeliveryWithConfig failed: %v", err)
	}
	if !delivery.IsSimulation() {
		t.Error("expected simulation implementation when using nil config fallback")
	}
}

// TestCreatePacketDeliveryWithConfigCustom verifies custom config override
func TestCreatePacketDeliveryWithConfigCustom(t *testing.T) {
	factory := NewPacketDeliveryFactory()
	factory.SwitchToReal() // Default is real

	// Pass custom config requesting simulation
	customConfig := &interfaces.PacketDeliveryConfig{
		UseSimulation:   true,
		NetworkTimeout:  2000,
		RetryAttempts:   2,
		EnableBroadcast: true,
	}

	delivery, err := factory.CreatePacketDeliveryWithConfig(nil, customConfig)
	if err != nil {
		t.Fatalf("CreatePacketDeliveryWithConfig failed: %v", err)
	}
	if !delivery.IsSimulation() {
		t.Error("expected simulation implementation with custom config")
	}
}

// TestCreatePacketDeliveryWithConfigRealRequiresTransport verifies transport requirement
func TestCreatePacketDeliveryWithConfigRealRequiresTransport(t *testing.T) {
	factory := NewPacketDeliveryFactory()

	realConfig := &interfaces.PacketDeliveryConfig{
		UseSimulation:   false,
		NetworkTimeout:  5000,
		RetryAttempts:   3,
		EnableBroadcast: true,
	}

	_, err := factory.CreatePacketDeliveryWithConfig(nil, realConfig)
	if err == nil {
		t.Error("expected error when creating real implementation without transport")
	}
}

// TestUpdateConfigIndependentCopy verifies updated config is independent
func TestUpdateConfigIndependentCopy(t *testing.T) {
	factory := NewPacketDeliveryFactory()

	originalConfig := &interfaces.PacketDeliveryConfig{
		UseSimulation:   true,
		NetworkTimeout:  8000,
		RetryAttempts:   4,
		EnableBroadcast: false,
	}

	err := factory.UpdateConfig(originalConfig)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Modify original after update
	originalConfig.NetworkTimeout = 99999

	// Factory config should be unchanged
	factoryConfig := factory.GetCurrentConfig()
	if factoryConfig.NetworkTimeout == 99999 {
		t.Error("UpdateConfig should store a copy, not reference to original")
	}
}
