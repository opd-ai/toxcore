// Package dht implements the Distributed Hash Table for the Tox protocol.
package dht

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMDNSDiscovery(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test-public-key-1234567890123456"))

	md := NewMDNSDiscovery(publicKey, 33445)
	require.NotNil(t, md)

	assert.Equal(t, publicKey, md.publicKey)
	assert.Equal(t, uint16(33445), md.port)
	assert.False(t, md.IsEnabled())
	assert.NotNil(t, md.knownPeers)
}

func TestMDNSDiscovery_BuildQuery(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test-public-key-1234567890123456"))

	md := NewMDNSDiscovery(publicKey, 33445)
	query := md.buildMDNSQuery()

	// Verify packet structure
	require.Len(t, query, 38)

	// Check magic number
	magic := uint16(query[0])<<8 | uint16(query[1])
	assert.Equal(t, uint16(0xF0F0), magic)

	// Check packet type (query)
	assert.Equal(t, byte(0x01), query[2])

	// Check public key
	var extractedKey [32]byte
	copy(extractedKey[:], query[4:36])
	assert.Equal(t, publicKey, extractedKey)

	// Check port
	port := uint16(query[36])<<8 | uint16(query[37])
	assert.Equal(t, uint16(33445), port)
}

func TestMDNSDiscovery_BuildResponse(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test-public-key-1234567890123456"))

	md := NewMDNSDiscovery(publicKey, 33445)
	response := md.buildMDNSResponse(publicKey, 33445)

	// Verify packet structure
	require.Len(t, response, 38)

	// Check magic number
	magic := uint16(response[0])<<8 | uint16(response[1])
	assert.Equal(t, uint16(0xF0F0), magic)

	// Check packet type (response)
	assert.Equal(t, byte(0x02), response[2])
}

func TestMDNSDiscovery_HandlePacket_IgnoresOwnKey(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test-public-key-1234567890123456"))

	md := NewMDNSDiscovery(publicKey, 33445)

	callbackCalled := false
	md.OnPeer(func(pk [32]byte, addr net.Addr) {
		callbackCalled = true
	})

	// Build a response packet with our own key
	packet := md.buildMDNSResponse(publicKey, 33445)

	// Should not trigger callback since it's our own key
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 5353}
	md.handlePacket(packet, addr, "test")

	assert.False(t, callbackCalled, "Callback should not be called for own key")
}

func TestMDNSDiscovery_HandlePacket_NotifiesPeer(t *testing.T) {
	var ownKey [32]byte
	copy(ownKey[:], []byte("own-public-key-12345678901234567"))

	var peerKey [32]byte
	copy(peerKey[:], []byte("peer-public-key-1234567890123456"))

	md := NewMDNSDiscovery(ownKey, 33445)

	var receivedKey [32]byte
	var receivedAddr net.Addr
	callbackCalled := make(chan struct{}, 1)

	md.OnPeer(func(pk [32]byte, addr net.Addr) {
		receivedKey = pk
		receivedAddr = addr
		select {
		case callbackCalled <- struct{}{}:
		default:
		}
	})

	// Build a response packet with a different key
	packet := md.buildMDNSResponse(peerKey, 44556)

	// Should trigger callback
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 5353}
	md.handlePacket(packet, addr, "test")

	select {
	case <-callbackCalled:
		assert.Equal(t, peerKey, receivedKey)
		require.NotNil(t, receivedAddr)
		// Verify the port is from the packet, not the source
		udpAddr, ok := receivedAddr.(*net.UDPAddr)
		require.True(t, ok)
		assert.Equal(t, 44556, udpAddr.Port)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Callback was not called")
	}
}

func TestMDNSDiscovery_HandlePacket_InvalidPacket(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test-public-key-1234567890123456"))

	md := NewMDNSDiscovery(publicKey, 33445)

	callbackCalled := false
	md.OnPeer(func(pk [32]byte, addr net.Addr) {
		callbackCalled = true
	})

	testCases := []struct {
		name   string
		packet []byte
	}{
		{"too short", []byte{0xF0, 0xF0, 0x02}},
		{"wrong magic", append([]byte{0x00, 0x00, 0x02, 0x00}, make([]byte, 34)...)},
		{"empty", []byte{}},
	}

	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 5353}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			callbackCalled = false
			md.handlePacket(tc.packet, addr, "test")
			assert.False(t, callbackCalled, "Callback should not be called for invalid packet")
		})
	}
}

func TestMDNSDiscovery_KnownPeerTracking(t *testing.T) {
	var ownKey [32]byte
	copy(ownKey[:], []byte("own-public-key-12345678901234567"))

	md := NewMDNSDiscovery(ownKey, 33445)

	// Initially no peers
	assert.Equal(t, 0, md.KnownPeerCount())

	// Discover a peer
	var peerKey1 [32]byte
	copy(peerKey1[:], []byte("peer-public-key-1234567890123456"))

	packet1 := md.buildMDNSResponse(peerKey1, 44556)
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 5353}

	md.handlePacket(packet1, addr, "test")
	assert.Equal(t, 1, md.KnownPeerCount())

	// Discover another peer
	var peerKey2 [32]byte
	copy(peerKey2[:], []byte("peer-public-key-2222222222222222"))

	packet2 := md.buildMDNSResponse(peerKey2, 44557)
	md.handlePacket(packet2, addr, "test")
	assert.Equal(t, 2, md.KnownPeerCount())

	// Same peer again doesn't increase count
	md.handlePacket(packet1, addr, "test")
	assert.Equal(t, 2, md.KnownPeerCount())
}

func TestMDNSDiscovery_CleanupStale(t *testing.T) {
	var ownKey [32]byte
	copy(ownKey[:], []byte("own-public-key-12345678901234567"))

	md := NewMDNSDiscovery(ownKey, 33445)

	// Manually add a stale peer
	staleKey := "stale1234567890123456789012345678901234567890123456789012345678"
	md.mu.Lock()
	md.knownPeers[staleKey] = time.Now().Add(-10 * time.Minute)
	md.mu.Unlock()

	// Add a fresh peer
	var freshKey [32]byte
	copy(freshKey[:], []byte("fresh-public-key-123456789012345"))
	packet := md.buildMDNSResponse(freshKey, 44556)
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 5353}
	md.handlePacket(packet, addr, "test")

	assert.Equal(t, 2, md.KnownPeerCount())

	// Cleanup stale peers (older than 5 minutes)
	removed := md.CleanupStale(5 * time.Minute)
	assert.Equal(t, 1, removed)
	assert.Equal(t, 1, md.KnownPeerCount())
}

func TestMDNSDiscovery_HandleQuery_RespondsWithAnnouncement(t *testing.T) {
	var ownKey [32]byte
	copy(ownKey[:], []byte("own-public-key-12345678901234567"))

	md := NewMDNSDiscovery(ownKey, 33445)

	// Build a query packet from another peer
	var peerKey [32]byte
	copy(peerKey[:], []byte("peer-public-key-1234567890123456"))

	packet := make([]byte, 38)
	packet[0] = 0xF0
	packet[1] = 0xF0
	packet[2] = 0x01 // Query type
	packet[3] = 0x00
	copy(packet[4:36], peerKey[:])
	packet[36] = 0x00
	packet[37] = 0x01

	// This should trigger an announcement response
	// We can't easily verify the response without network mocking,
	// but we can verify the packet type is recognized
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 5353}
	md.handlePacket(packet, addr, "test")

	// Queries shouldn't add the peer to known peers
	assert.Equal(t, 0, md.KnownPeerCount())
}

func TestMDNSDiscovery_ConcurrentAccess(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test-public-key-1234567890123456"))

	md := NewMDNSDiscovery(publicKey, 33445)

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent reads of IsEnabled
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = md.IsEnabled()
		}
	}()

	// Concurrent reads of KnownPeerCount
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = md.KnownPeerCount()
		}
	}()

	// Concurrent packet handling
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			var peerKey [32]byte
			copy(peerKey[:], []byte("peer-public-key-1234567890123456"))
			peerKey[0] = byte(i % 256)
			packet := md.buildMDNSResponse(peerKey, uint16(44000+i))
			addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 5353}
			md.handlePacket(packet, addr, "test")
		}
	}()

	// Concurrent cleanup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations/10; i++ {
			md.CleanupStale(time.Hour)
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()
}

func TestMDNSDiscovery_StartStop(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test-public-key-1234567890123456"))

	md := NewMDNSDiscovery(publicKey, 33445)

	// Stop before start should be safe
	md.Stop()

	// Multiple stops should be safe
	md.Stop()
	md.Stop()

	// Note: We don't test Start() here because it requires actual network access
	// and may fail in CI environments. Integration tests would cover that.
}
