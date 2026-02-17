package dht

import (
	"net"
	"testing"
)

// customAddr is a custom net.Addr implementation for testing
// This ensures the code works with any net.Addr, not just *net.UDPAddr
type customAddr struct {
	network string
	address string
}

func (c *customAddr) Network() string {
	return c.network
}

func (c *customAddr) String() string {
	return c.address
}

// TestLANDiscoveryHandlePacketWithCustomAddr verifies that handlePacket
// works with any net.Addr implementation, not just *net.UDPAddr.
// This test ensures compliance with the project's networking best practices.
func TestLANDiscoveryHandlePacketWithCustomAddr(t *testing.T) {
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

	// Use a custom address implementation (not *net.UDPAddr)
	addr := &customAddr{
		network: "custom",
		address: "192.168.1.100:12345",
	}

	// Handle the packet - should work with any net.Addr
	ld.handlePacket(packet, addr)

	// Verify callback was called with correct data
	if receivedKey != publicKey2 {
		t.Error("Expected callback to receive publicKey2")
	}

	if receivedAddr == nil {
		t.Fatal("Expected non-nil address")
	}

	// The returned address should be a UDP address with the IP from the custom addr
	// and the port from the packet
	udpAddr, ok := receivedAddr.(*net.UDPAddr)
	if !ok {
		t.Fatal("Expected UDP address to be returned")
	}

	if udpAddr.IP.String() != "192.168.1.100" {
		t.Errorf("Expected IP 192.168.1.100, got %s", udpAddr.IP.String())
	}

	if udpAddr.Port != 33446 {
		t.Errorf("Expected port 33446 (from packet), got %d", udpAddr.Port)
	}
}

// TestLANDiscoveryHandlePacketWithIPv6 tests handling of IPv6 addresses
func TestLANDiscoveryHandlePacketWithIPv6(t *testing.T) {
	var publicKey1 [32]byte
	var publicKey2 [32]byte
	copy(publicKey1[:], []byte("test_public_key1_123456789012345"))
	copy(publicKey2[:], []byte("test_public_key2_123456789012345"))

	ld := NewLANDiscovery(publicKey1, 33445)

	var receivedAddr net.Addr

	ld.OnPeer(func(pkey [32]byte, addr net.Addr) {
		receivedAddr = addr
	})

	// Create a valid packet
	packet := LANDiscoveryPacketData(publicKey2, 33447)

	// Use an IPv6 address
	addr := &net.UDPAddr{
		IP:   net.ParseIP("fe80::1"),
		Port: 12345,
	}

	// Handle the packet
	ld.handlePacket(packet, addr)

	if receivedAddr == nil {
		t.Fatal("Expected non-nil address")
	}

	udpAddr, ok := receivedAddr.(*net.UDPAddr)
	if !ok {
		t.Fatal("Expected UDP address")
	}

	if !udpAddr.IP.Equal(net.ParseIP("fe80::1")) {
		t.Errorf("Expected IPv6 address fe80::1, got %s", udpAddr.IP.String())
	}

	if udpAddr.Port != 33447 {
		t.Errorf("Expected port 33447 (from packet), got %d", udpAddr.Port)
	}
}

// TestLANDiscoveryHandlePacketWithInvalidAddr tests handling of addresses
// that cannot be parsed as IP:port
func TestLANDiscoveryHandlePacketWithInvalidAddr(t *testing.T) {
	var publicKey1 [32]byte
	var publicKey2 [32]byte
	copy(publicKey1[:], []byte("test_public_key1_123456789012345"))
	copy(publicKey2[:], []byte("test_public_key2_123456789012345"))

	ld := NewLANDiscovery(publicKey1, 33445)

	callbackCalled := false
	ld.OnPeer(func(pkey [32]byte, addr net.Addr) {
		callbackCalled = true
	})

	// Create a valid packet
	packet := LANDiscoveryPacketData(publicKey2, 33446)

	// Use an address that can't be parsed as IP
	addr := &customAddr{
		network: "custom",
		address: "not-a-valid-ip",
	}

	// Handle the packet - should not call callback for invalid IP
	ld.handlePacket(packet, addr)

	if callbackCalled {
		t.Error("Callback should not be called for unparseable address")
	}
}

// TestLANDiscoveryHandlePacketWithAddressNoPort tests handling of addresses
// without a port (edge case)
func TestLANDiscoveryHandlePacketWithAddressNoPort(t *testing.T) {
	var publicKey1 [32]byte
	var publicKey2 [32]byte
	copy(publicKey1[:], []byte("test_public_key1_123456789012345"))
	copy(publicKey2[:], []byte("test_public_key2_123456789012345"))

	ld := NewLANDiscovery(publicKey1, 33445)

	var receivedAddr net.Addr

	ld.OnPeer(func(pkey [32]byte, addr net.Addr) {
		receivedAddr = addr
	})

	// Create a valid packet
	packet := LANDiscoveryPacketData(publicKey2, 33448)

	// Use an address without a port
	addr := &customAddr{
		network: "custom",
		address: "192.168.1.200",
	}

	// Handle the packet - should use IP and port from packet
	ld.handlePacket(packet, addr)

	if receivedAddr == nil {
		t.Fatal("Expected non-nil address")
	}

	udpAddr, ok := receivedAddr.(*net.UDPAddr)
	if !ok {
		t.Fatal("Expected UDP address")
	}

	if udpAddr.IP.String() != "192.168.1.200" {
		t.Errorf("Expected IP 192.168.1.200, got %s", udpAddr.IP.String())
	}

	if udpAddr.Port != 33448 {
		t.Errorf("Expected port 33448 (from packet), got %d", udpAddr.Port)
	}
}
