package main

import (
	"testing"
	"time"

	"github.com/opd-ai/toxcore"
	toxnet "github.com/opd-ai/toxcore/net"
)

// MockTimeProvider is a mock time provider for deterministic testing.
type MockTimeProvider struct {
	fixedTime time.Time
}

// Now returns the fixed time set in the mock provider.
func (m MockTimeProvider) Now() time.Time {
	return m.fixedTime
}

// TestTimeProvider verifies the time provider interface is correctly implemented.
func TestTimeProvider(t *testing.T) {
	// Test RealTimeProvider
	realProvider := RealTimeProvider{}
	now := realProvider.Now()
	if now.IsZero() {
		t.Error("RealTimeProvider.Now() returned zero time")
	}

	// Test MockTimeProvider
	fixedTime := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)
	mockProvider := MockTimeProvider{fixedTime: fixedTime}
	if !mockProvider.Now().Equal(fixedTime) {
		t.Errorf("MockTimeProvider.Now() = %v, want %v", mockProvider.Now(), fixedTime)
	}
}

// TestTimeProviderSwapping verifies that we can swap time providers for testing.
func TestTimeProviderSwapping(t *testing.T) {
	// Save original provider
	originalProvider := timeProvider
	defer func() { timeProvider = originalProvider }()

	// Set mock provider
	fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	timeProvider = MockTimeProvider{fixedTime: fixedTime}

	// Verify time provider returns fixed time
	if !timeProvider.Now().Equal(fixedTime) {
		t.Errorf("timeProvider.Now() = %v, want %v", timeProvider.Now(), fixedTime)
	}
}

// TestPacketDialInvalidNetwork verifies PacketDial returns error for invalid network.
func TestPacketDialInvalidNetwork(t *testing.T) {
	_, err := toxnet.PacketDial("invalid", "test-addr")
	if err == nil {
		t.Error("PacketDial with invalid network should return error")
	}
}

// TestPacketDialInvalidAddress verifies PacketDial returns error for invalid Tox ID.
func TestPacketDialInvalidAddress(t *testing.T) {
	_, err := toxnet.PacketDial("tox", "invalid-tox-id")
	if err == nil {
		t.Error("PacketDial with invalid address should return error")
	}
}

// TestPacketListenInvalidNetwork verifies PacketListen returns error for invalid network.
func TestPacketListenInvalidNetwork(t *testing.T) {
	_, err := toxnet.PacketListen("invalid", ":0", nil)
	if err == nil {
		t.Error("PacketListen with invalid network should return error")
	}
}

// TestPacketListenNilTox verifies PacketListen returns error for nil Tox instance.
func TestPacketListenNilTox(t *testing.T) {
	_, err := toxnet.PacketListen("tox", ":0", nil)
	if err == nil {
		t.Error("PacketListen with nil Tox should return error")
	}
}

// TestDemonstratePacketConn tests the packet connection demonstration function.
func TestDemonstratePacketConn(t *testing.T) {
	// Save original provider
	originalProvider := timeProvider
	defer func() { timeProvider = originalProvider }()

	// Set mock provider for deterministic deadline
	fixedTime := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)
	timeProvider = MockTimeProvider{fixedTime: fixedTime}

	// Run demonstration function
	err := demonstratePacketConn()
	if err != nil {
		t.Errorf("demonstratePacketConn() returned error: %v", err)
	}
}

// TestDemonstratePacketListener tests the packet listener demonstration function.
func TestDemonstratePacketListener(t *testing.T) {
	err := demonstratePacketListener()
	if err != nil {
		t.Errorf("demonstratePacketListener() returned error: %v", err)
	}
}

// TestDemonstratePacketDialListen tests the dial/listen demonstration function.
func TestDemonstratePacketDialListen(t *testing.T) {
	err := demonstratePacketDialListen()
	if err != nil {
		t.Errorf("demonstratePacketDialListen() returned error: %v", err)
	}
}

// TestIntegrationExample tests the integration example function.
func TestIntegrationExample(t *testing.T) {
	err := integrationExample()
	if err != nil {
		t.Errorf("integrationExample() returned error: %v", err)
	}
}

// TestNewToxAddrFromPublicKey verifies ToxAddr creation from public key.
func TestNewToxAddrFromPublicKey(t *testing.T) {
	testCases := []struct {
		name   string
		nospam [4]byte
	}{
		{"zero_nospam", [4]byte{0x00, 0x00, 0x00, 0x00}},
		{"sequential_nospam", [4]byte{0x01, 0x02, 0x03, 0x04}},
		{"max_nospam", [4]byte{0xFF, 0xFF, 0xFF, 0xFF}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keyPair, err := generateTestKeyPair()
			if err != nil {
				t.Fatalf("Failed to generate key pair: %v", err)
			}

			addr := toxnet.NewToxAddrFromPublicKey(keyPair.Public, tc.nospam)
			if addr.String() == "" {
				t.Error("ToxAddr should have non-empty string representation")
			}
		})
	}
}

// TestDeadlineCalculation verifies deadline is calculated using time provider.
func TestDeadlineCalculation(t *testing.T) {
	// Save original provider
	originalProvider := timeProvider
	defer func() { timeProvider = originalProvider }()

	// Set mock provider with fixed time
	fixedTime := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)
	timeProvider = MockTimeProvider{fixedTime: fixedTime}

	// Calculate deadline
	duration := 5 * time.Second
	deadline := timeProvider.Now().Add(duration)

	// Verify deadline is exactly 5 seconds after fixed time
	expectedDeadline := time.Date(2026, 2, 18, 12, 0, 5, 0, time.UTC)
	if !deadline.Equal(expectedDeadline) {
		t.Errorf("deadline = %v, want %v", deadline, expectedDeadline)
	}
}

// generateTestKeyPair is a helper to generate test key pairs using the crypto package.
func generateTestKeyPair() (*testKeyPair, error) {
	// Create a fixed 32-byte public key for testing
	var public [32]byte
	var private [32]byte
	// Fill with deterministic test data
	for i := 0; i < 32; i++ {
		public[i] = byte(i)
		private[i] = byte(32 + i)
	}
	return &testKeyPair{
		Public:  public,
		Private: private,
	}, nil
}

type testKeyPair struct {
	Public  [32]byte
	Private [32]byte
}

// TestPacketConnInterface verifies ToxPacketConn implements net.PacketConn.
func TestPacketConnInterface(t *testing.T) {
	keyPair, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := toxnet.NewToxAddrFromPublicKey(keyPair.Public, nospam)

	// Create packet connection
	conn, err := toxnet.NewToxPacketConn(localAddr, ":0")
	if err != nil {
		t.Fatalf("Failed to create packet connection: %v", err)
	}
	defer conn.Close()

	// Verify it has a local address
	if conn.LocalAddr() == nil {
		t.Error("LocalAddr() should not return nil")
	}
}

// TestPacketListenerInterface verifies ToxPacketListener methods.
func TestPacketListenerInterface(t *testing.T) {
	keyPair, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	nospam := [4]byte{0x05, 0x06, 0x07, 0x08}
	localAddr := toxnet.NewToxAddrFromPublicKey(keyPair.Public, nospam)

	// Create packet listener
	listener, err := toxnet.NewToxPacketListener(localAddr, ":0")
	if err != nil {
		t.Fatalf("Failed to create packet listener: %v", err)
	}
	defer listener.Close()

	// Verify it has an address
	if listener.Addr() == nil {
		t.Error("Addr() should not return nil")
	}
}

// TestPacketListenWithValidTox verifies PacketListen works with a valid Tox instance.
func TestPacketListenWithValidTox(t *testing.T) {
	opts := toxcore.NewOptions()
	tox, err := toxcore.New(opts)
	if err != nil {
		t.Skipf("Could not create Tox instance: %v", err)
	}
	defer tox.Kill()

	listener, err := toxnet.PacketListen("tox", ":0", tox)
	if err != nil {
		t.Fatalf("PacketListen with valid Tox should succeed: %v", err)
	}
	defer listener.Close()

	// Verify listener address is not nil
	if listener.Addr() == nil {
		t.Error("Listener.Addr() should not return nil")
	}
}
