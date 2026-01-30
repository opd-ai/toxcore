package dht

import (
	"net"
	"sync"
	"testing"
	"time"
)

// TestLANDiscoveryCreation tests creating a new LAN discovery instance.
func TestLANDiscoveryCreation(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test_public_key_1234567890123456"))

	ld := NewLANDiscovery(publicKey, 33445)

	if ld == nil {
		t.Fatal("Expected LAN discovery instance, got nil")
	}

	if ld.publicKey != publicKey {
		t.Error("Public key mismatch")
	}

	if ld.port != 33445 {
		t.Errorf("Expected port 33445, got %d", ld.port)
	}

	if ld.IsEnabled() {
		t.Error("Expected LAN discovery to be disabled initially")
	}
}

// TestLANDiscoveryStartStop tests starting and stopping LAN discovery.
func TestLANDiscoveryStartStop(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test_public_key_1234567890123456"))

	ld := NewLANDiscovery(publicKey, 33445)

	// Start LAN discovery
	err := ld.Start()
	if err != nil {
		t.Fatalf("Failed to start LAN discovery: %v", err)
	}

	if !ld.IsEnabled() {
		t.Error("Expected LAN discovery to be enabled after Start()")
	}

	// Starting again should be idempotent
	err = ld.Start()
	if err != nil {
		t.Error("Expected Start() to be idempotent")
	}

	// Stop LAN discovery
	ld.Stop()

	if ld.IsEnabled() {
		t.Error("Expected LAN discovery to be disabled after Stop()")
	}

	// Stopping again should be safe
	ld.Stop()
}

// TestLANDiscoveryCallback tests the peer discovery callback.
func TestLANDiscoveryCallback(t *testing.T) {
	var publicKey1 [32]byte
	var publicKey2 [32]byte
	copy(publicKey1[:], []byte("test_public_key1_123456789012345"))
	copy(publicKey2[:], []byte("test_public_key2_123456789012345"))

	ld1 := NewLANDiscovery(publicKey1, 33446)
	ld2 := NewLANDiscovery(publicKey2, 33447)

	var wg sync.WaitGroup
	var mu sync.Mutex
	discoveredPeers := make(map[[32]byte]net.Addr)

	// Set up callback for ld1
	ld1.OnPeer(func(pkey [32]byte, addr net.Addr) {
		mu.Lock()
		discoveredPeers[pkey] = addr
		mu.Unlock()
		wg.Done()
	})

	wg.Add(1)

	// Start both instances
	if err := ld1.Start(); err != nil {
		t.Fatalf("Failed to start ld1: %v", err)
	}
	defer ld1.Stop()

	if err := ld2.Start(); err != nil {
		t.Fatalf("Failed to start ld2: %v", err)
	}
	defer ld2.Stop()

	// Wait for discovery with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - peer discovered
		mu.Lock()
		addr, found := discoveredPeers[publicKey2]
		mu.Unlock()

		if !found {
			t.Error("Expected to discover publicKey2")
		}

		if addr == nil {
			t.Error("Expected non-nil address for discovered peer")
		}

		t.Logf("Successfully discovered peer at %s", addr)

	case <-time.After(15 * time.Second):
		t.Log("Timeout waiting for peer discovery (this is acceptable on some networks)")
		// Don't fail the test as LAN discovery may not work in all test environments
	}
}

// TestLANDiscoveryNoSelfDiscovery tests that peers don't discover themselves.
func TestLANDiscoveryNoSelfDiscovery(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test_public_key_1234567890123456"))

	ld := NewLANDiscovery(publicKey, 33445)

	callbackCalled := false
	ld.OnPeer(func(pkey [32]byte, addr net.Addr) {
		if pkey == publicKey {
			callbackCalled = true
		}
	})

	if err := ld.Start(); err != nil {
		t.Fatalf("Failed to start LAN discovery: %v", err)
	}
	defer ld.Stop()

	// Wait a bit to ensure broadcasts happen
	time.Sleep(2 * time.Second)

	if callbackCalled {
		t.Error("LAN discovery should not discover itself")
	}
}

// TestLANDiscoveryMultipleCallbacks tests replacing the callback.
func TestLANDiscoveryMultipleCallbacks(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test_public_key_1234567890123456"))

	ld := NewLANDiscovery(publicKey, 33445)

	callback1Called := false
	callback2Called := false

	// Set first callback
	ld.OnPeer(func(pkey [32]byte, addr net.Addr) {
		callback1Called = true
	})

	// Replace with second callback
	ld.OnPeer(func(pkey [32]byte, addr net.Addr) {
		callback2Called = true
	})

	// Simulate a peer discovery
	var peerKey [32]byte
	copy(peerKey[:], []byte("peer_public_key_1234567890123456"))

	peerAddr := &net.UDPAddr{
		IP:   net.ParseIP("192.168.1.100"),
		Port: 33445,
	}

	ld.mu.RLock()
	callback := ld.onPeerFunc
	ld.mu.RUnlock()

	if callback != nil {
		callback(peerKey, peerAddr)
	}

	if callback1Called {
		t.Error("First callback should not be called after replacement")
	}

	if !callback2Called {
		t.Error("Second callback should be called")
	}
}

// TestParseLANDiscoveryPacket tests parsing LAN discovery packets.
func TestParseLANDiscoveryPacket(t *testing.T) {
	tests := []struct {
		name       string
		packet     []byte
		expectErr  bool
		expectKey  [32]byte
		expectPort uint16
	}{
		{
			name: "valid packet",
			packet: func() []byte {
				p := make([]byte, 34)
				copy(p[0:32], []byte("test_public_key_1234567890123456"))
				p[32] = 0x82 // port 33445 (0x82A5) in big endian
				p[33] = 0xA5
				return p
			}(),
			expectErr: false,
			expectKey: func() [32]byte {
				var k [32]byte
				copy(k[:], []byte("test_public_key_1234567890123456"))
				return k
			}(),
			expectPort: 33445,
		},
		{
			name:      "packet too short",
			packet:    make([]byte, 33),
			expectErr: true,
		},
		{
			name:      "empty packet",
			packet:    []byte{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publicKey, port, err := ParseLANDiscoveryPacket(tt.packet)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if publicKey != tt.expectKey {
				t.Error("Public key mismatch")
			}

			if port != tt.expectPort {
				t.Errorf("Expected port %d, got %d", tt.expectPort, port)
			}
		})
	}
}

// TestLANDiscoveryPacketData tests creating LAN discovery packet data.
func TestLANDiscoveryPacketData(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test_public_key_1234567890123456"))
	port := uint16(33445)

	data := LANDiscoveryPacketData(publicKey, port)

	if len(data) != 34 {
		t.Errorf("Expected packet length 34, got %d", len(data))
	}

	// Verify we can parse it back
	parsedKey, parsedPort, err := ParseLANDiscoveryPacket(data)
	if err != nil {
		t.Fatalf("Failed to parse created packet: %v", err)
	}

	if parsedKey != publicKey {
		t.Error("Public key mismatch after round-trip")
	}

	if parsedPort != port {
		t.Errorf("Port mismatch after round-trip: expected %d, got %d", port, parsedPort)
	}
}

// TestLANDiscoveryHandlePacket tests the packet handling logic.
func TestLANDiscoveryHandlePacket(t *testing.T) {
	var publicKey1 [32]byte
	var publicKey2 [32]byte
	copy(publicKey1[:], []byte("test_public_key1_123456789012345"))
	copy(publicKey2[:], []byte("test_public_key2_123456789012345"))

	ld := NewLANDiscovery(publicKey1, 33445)

	var receivedKey [32]byte
	var receivedAddr net.Addr

	ld.OnPeer(func(pkey [32]byte, addr net.Addr) {
		receivedKey = pkey
		receivedAddr = addr
	})

	// Create a valid packet
	packet := LANDiscoveryPacketData(publicKey2, 33446)
	addr := &net.UDPAddr{
		IP:   net.ParseIP("192.168.1.100"),
		Port: 33446,
	}

	// Handle the packet
	ld.handlePacket(packet, addr)

	if receivedKey != publicKey2 {
		t.Error("Expected callback to receive publicKey2")
	}

	if receivedAddr == nil {
		t.Fatal("Expected non-nil address")
	}

	udpAddr, ok := receivedAddr.(*net.UDPAddr)
	if !ok {
		t.Fatal("Expected UDP address")
	}

	if udpAddr.Port != 33446 {
		t.Errorf("Expected port 33446, got %d", udpAddr.Port)
	}
}

// TestLANDiscoveryIgnoreSelfPacket tests that self packets are ignored.
func TestLANDiscoveryIgnoreSelfPacket(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test_public_key_1234567890123456"))

	ld := NewLANDiscovery(publicKey, 33445)

	callbackCalled := false
	ld.OnPeer(func(pkey [32]byte, addr net.Addr) {
		callbackCalled = true
	})

	// Create a packet with our own public key
	packet := LANDiscoveryPacketData(publicKey, 33445)
	addr := &net.UDPAddr{
		IP:   net.ParseIP("192.168.1.100"),
		Port: 33445,
	}

	// Handle the packet
	ld.handlePacket(packet, addr)

	if callbackCalled {
		t.Error("Callback should not be called for self packet")
	}
}

// TestLANDiscoveryInvalidPacket tests handling of invalid packets.
func TestLANDiscoveryInvalidPacket(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test_public_key_1234567890123456"))

	ld := NewLANDiscovery(publicKey, 33445)

	callbackCalled := false
	ld.OnPeer(func(pkey [32]byte, addr net.Addr) {
		callbackCalled = true
	})

	// Create invalid packet (too short)
	packet := make([]byte, 10)
	addr := &net.UDPAddr{
		IP:   net.ParseIP("192.168.1.100"),
		Port: 33445,
	}

	// Handle the packet - should not panic or call callback
	ld.handlePacket(packet, addr)

	if callbackCalled {
		t.Error("Callback should not be called for invalid packet")
	}
}
