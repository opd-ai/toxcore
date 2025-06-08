package toxcore

import (
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

func TestAddTcpRelay(t *testing.T) {
	// Create a new Tox instance
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test valid TCP relay addition
	address := "tox.abiliri.org"
	port := uint16(3389)
	publicKeyHex := "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67"

	err = tox.AddTcpRelay(address, port, publicKeyHex)
	if err != nil {
		t.Fatalf("Failed to add TCP relay: %v", err)
	}

	// Verify the relay was added
	tox.tcpRelayMutex.RLock()
	relayKey := "tox.abiliri.org:3389"
	relay, exists := tox.tcpRelays[relayKey]
	tox.tcpRelayMutex.RUnlock()

	if !exists {
		t.Fatal("TCP relay was not added to the relay map")
	}

	if relay.Address != address {
		t.Errorf("Expected address %s, got %s", address, relay.Address)
	}

	if relay.Port != port {
		t.Errorf("Expected port %d, got %d", port, relay.Port)
	}

	// Verify public key was decoded correctly
	expectedKeyBytes, _ := hex.DecodeString(publicKeyHex)
	for i, b := range expectedKeyBytes {
		if relay.PublicKey[i] != b {
			t.Errorf("Public key mismatch at byte %d: expected %02x, got %02x", i, b, relay.PublicKey[i])
		}
	}

	// Verify timestamps
	if relay.Added.IsZero() {
		t.Error("Added timestamp was not set")
	}

	if relay.Connected {
		t.Error("Relay should not be connected immediately")
	}
}

func TestAddTcpRelayValidation(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	tests := []struct {
		name      string
		address   string
		port      uint16
		publicKey string
		wantError bool
	}{
		{
			name:      "valid relay",
			address:   "relay.example.com",
			port:      3389,
			publicKey: "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67",
			wantError: false,
		},
		{
			name:      "empty address",
			address:   "",
			port:      3389,
			publicKey: "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67",
			wantError: true,
		},
		{
			name:      "zero port",
			address:   "relay.example.com",
			port:      0,
			publicKey: "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67",
			wantError: true,
		},
		{
			name:      "invalid public key length",
			address:   "relay.example.com",
			port:      3389,
			publicKey: "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB",
			wantError: true,
		},
		{
			name:      "invalid hex characters",
			address:   "relay.example.com",
			port:      3389,
			publicKey: "G404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tox.AddTcpRelay(tt.address, tt.port, tt.publicKey)
			if (err != nil) != tt.wantError {
				t.Errorf("AddTcpRelay() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestTcpRelayInitialization(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify TCP transport is nil initially
	if tox.tcpTransport != nil {
		t.Error("TCP transport should be nil initially")
	}

	// Add a TCP relay
	err = tox.AddTcpRelay("relay.example.com", 3389, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		t.Fatalf("Failed to add TCP relay: %v", err)
	}

	// Verify TCP transport was initialized
	if tox.tcpTransport == nil {
		t.Error("TCP transport should be initialized after adding relay")
	}
}

func TestTcpRelayMultipleAdditions(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add multiple TCP relays
	relays := []struct {
		address string
		port    uint16
		key     string
	}{
		{"relay1.example.com", 3389, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67"},
		{"relay2.example.com", 3390, "A404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67"},
		{"relay3.example.com", 3391, "B404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67"},
	}

	for _, relay := range relays {
		err := tox.AddTcpRelay(relay.address, relay.port, relay.key)
		if err != nil {
			t.Fatalf("Failed to add TCP relay %s:%d: %v", relay.address, relay.port, err)
		}
	}

	// Verify all relays were added
	tox.tcpRelayMutex.RLock()
	defer tox.tcpRelayMutex.RUnlock()

	if len(tox.tcpRelays) != len(relays) {
		t.Errorf("Expected %d relays, got %d", len(relays), len(tox.tcpRelays))
	}

	for _, relay := range relays {
		relayKey := relay.address + ":" + strconv.Itoa(int(relay.port))
		if _, exists := tox.tcpRelays[relayKey]; !exists {
			t.Errorf("Relay %s was not found in relay map", relayKey)
		}
	}
}

func TestTcpRelayConnectionAttempt(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add a TCP relay
	err = tox.AddTcpRelay("127.0.0.1", 3389, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		t.Fatalf("Failed to add TCP relay: %v", err)
	}

	// Wait a short time for connection attempt
	time.Sleep(100 * time.Millisecond)

	// Verify the relay entry exists (connection may fail, but entry should exist)
	tox.tcpRelayMutex.RLock()
	relay, exists := tox.tcpRelays["127.0.0.1:3389"]
	tox.tcpRelayMutex.RUnlock()

	if !exists {
		t.Fatal("TCP relay entry not found")
	}

	// The connection likely failed to localhost (no server), but the relay should exist
	if relay.Address != "127.0.0.1" || relay.Port != 3389 {
		t.Error("Relay properties don't match expected values")
	}
}
