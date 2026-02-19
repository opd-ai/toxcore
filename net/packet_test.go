package net

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToxPacketConn(t *testing.T) {
	// Generate a test key pair for the Tox address
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	// Create a packet connection
	conn, err := NewToxPacketConn(localAddr, ":0")
	require.NoError(t, err)
	defer conn.Close()

	// Test LocalAddr
	assert.Equal(t, localAddr, conn.LocalAddr())

	// Test that we can set deadlines without error
	deadline := time.Now().Add(5 * time.Second)
	err = conn.SetDeadline(deadline)
	assert.NoError(t, err)

	err = conn.SetReadDeadline(deadline)
	assert.NoError(t, err)

	err = conn.SetWriteDeadline(deadline)
	assert.NoError(t, err)

	// Test Close
	err = conn.Close()
	assert.NoError(t, err)

	// Test that operations after close return errors
	buffer := make([]byte, 1024)
	_, _, err = conn.ReadFrom(buffer)
	assert.Error(t, err)

	_, err = conn.WriteTo([]byte("test"), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080})
	assert.Error(t, err)
}

func TestToxPacketListener(t *testing.T) {
	// Generate a test key pair for the Tox address
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	// Create a packet listener
	listener, err := NewToxPacketListener(localAddr, ":0")
	require.NoError(t, err)
	defer listener.Close()

	// Test Addr
	assert.Equal(t, localAddr, listener.Addr())

	// Test Close
	err = listener.Close()
	assert.NoError(t, err)

	// Test that Accept after close returns error
	_, err = listener.Accept()
	assert.Error(t, err)
}

func TestPacketDialAndListen(t *testing.T) {
	// Test PacketListen with invalid network
	_, err := PacketListen("invalid", ":0", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")

	// Test PacketListen with nil Tox instance
	_, err = PacketListen("tox", ":0", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")

	// Test PacketDial with invalid network
	_, err = PacketDial("invalid", "test-addr")
	assert.Error(t, err)

	// Test PacketDial with invalid address
	_, err = PacketDial("tox", "invalid-tox-id")
	assert.Error(t, err)
}

// TestPacketListenWithToxInstance verifies PacketListen creates a valid listener
// when provided with a real Tox instance.
func TestPacketListenWithToxInstance(t *testing.T) {
	// Create Tox instance
	opts := toxcore.NewOptions()
	tox, err := toxcore.New(opts)
	require.NoError(t, err)
	defer tox.Kill()

	// Create packet listener with valid Tox instance
	listener, err := PacketListen("tox", ":0", tox)
	require.NoError(t, err)
	defer listener.Close()

	// Verify the listener has a valid address
	addr := listener.Addr()
	assert.NotNil(t, addr)

	toxAddr, ok := addr.(*ToxAddr)
	assert.True(t, ok, "Address should be a *ToxAddr")
	assert.NotNil(t, toxAddr.ToxID(), "ToxAddr should have a valid ToxID")

	// Verify the public key matches the Tox instance
	expectedPubKey := tox.SelfGetPublicKey()
	actualPubKey := toxAddr.PublicKey()
	assert.Equal(t, expectedPubKey, actualPubKey, "Listener public key should match Tox instance")
}

// Integration test demonstrating basic packet communication
func TestPacketCommunication(t *testing.T) {
	// Generate test addresses
	keyPair1, err := crypto.GenerateKeyPair()
	require.NoError(t, err)
	keyPair2, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	addr1 := NewToxAddrFromPublicKey(keyPair1.Public, nospam)
	addr2 := NewToxAddrFromPublicKey(keyPair2.Public, nospam)

	// Create two packet connections
	conn1, err := NewToxPacketConn(addr1, ":0")
	require.NoError(t, err)
	defer conn1.Close()

	conn2, err := NewToxPacketConn(addr2, ":0")
	require.NoError(t, err)
	defer conn2.Close()

	// Get actual UDP addresses for communication
	udpAddr1 := conn1.udpConn.LocalAddr()
	udpAddr2 := conn2.udpConn.LocalAddr()

	// Test message sending
	testMessage := []byte("Hello, Tox!")

	// Send from conn1 to conn2
	n, err := conn1.WriteTo(testMessage, udpAddr2)
	require.NoError(t, err)
	assert.Equal(t, len(testMessage), n)

	// Read on conn2
	buffer := make([]byte, 1024)
	conn2.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, addr, err := conn2.ReadFrom(buffer)
	require.NoError(t, err)
	assert.Equal(t, len(testMessage), n)
	assert.Equal(t, testMessage, buffer[:n])

	// Verify the source address matches, accounting for IPv6 address normalization
	// When binding to [::]:port and communicating locally, the source appears as [::1]:port
	expectedAddr := udpAddr1.String()
	actualAddr := addr.String()

	// Handle IPv6 address normalization: [::]:port becomes [::1]:port in local communication
	if strings.HasPrefix(expectedAddr, "[::]:") && strings.HasPrefix(actualAddr, "[::1]:") {
		// Extract port from both addresses and compare
		expectedPort := strings.Split(expectedAddr, "]:")[1]
		actualPort := strings.Split(actualAddr, "]:")[1]
		assert.Equal(t, expectedPort, actualPort, "Port should match even with IPv6 address normalization")
	} else {
		// For non-IPv6 or non-local cases, addresses should match exactly
		assert.Equal(t, expectedAddr, actualAddr)
	}

	fmt.Printf("Successfully sent and received message: %s\n", string(buffer[:n]))
}

// Test ToxPacketListener close is idempotent
func TestToxPacketListenerCloseTwice(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	listener, err := NewToxPacketListener(localAddr, ":0")
	require.NoError(t, err)

	// First close should succeed
	err = listener.Close()
	assert.NoError(t, err)

	// Second close should also succeed (idempotent)
	err = listener.Close()
	assert.NoError(t, err)
}

// TestToxPacketConnCloseReturnsWrappedError verifies that ToxPacketConn.Close()
// returns errors wrapped in ToxNetError for consistency with other net package errors
func TestToxPacketConnCloseReturnsWrappedError(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	// Create a packet connection
	conn, err := NewToxPacketConn(localAddr, ":0")
	require.NoError(t, err)

	// Normal close should succeed with nil error
	err = conn.Close()
	assert.NoError(t, err, "First close should succeed without error")

	// Second close should also succeed (idempotent)
	err = conn.Close()
	assert.NoError(t, err, "Second close should be idempotent")

	// Verify error type consistency: ReadFrom after close returns ToxNetError
	buffer := make([]byte, 1024)
	_, _, err = conn.ReadFrom(buffer)
	require.Error(t, err)
	var toxErr *ToxNetError
	assert.ErrorAs(t, err, &toxErr, "ReadFrom error should be ToxNetError")
}

// Benchmark tests
func BenchmarkToxPacketConn_WriteTo(b *testing.B) {
	keyPair, _ := crypto.GenerateKeyPair()
	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	conn, err := NewToxPacketConn(localAddr, ":0")
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()

	data := make([]byte, 1024)
	destAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.WriteTo(data, destAddr)
	}
}

func BenchmarkToxPacketConn_ReadFrom(b *testing.B) {
	keyPair, _ := crypto.GenerateKeyPair()
	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	conn, err := NewToxPacketConn(localAddr, ":0")
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()

	// Pre-fill the read buffer
	for i := 0; i < 100; i++ {
		packet := packetWithAddr{
			data: make([]byte, 1024),
			addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080},
		}
		select {
		case conn.readBuffer <- packet:
		default:
			goto done
		}
	}
done:

	buffer := make([]byte, 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.ReadFrom(buffer)
	}
}

// TestToxPacketConnEncryption tests the encryption functionality
func TestToxPacketConnEncryption(t *testing.T) {
	// Generate key pairs for two peers
	keyPair1, err := crypto.GenerateKeyPair()
	require.NoError(t, err)
	keyPair2, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	addr1 := NewToxAddrFromPublicKey(keyPair1.Public, nospam)
	addr2 := NewToxAddrFromPublicKey(keyPair2.Public, nospam)

	// Create two packet connections
	conn1, err := NewToxPacketConn(addr1, ":0")
	require.NoError(t, err)
	defer conn1.Close()

	conn2, err := NewToxPacketConn(addr2, ":0")
	require.NoError(t, err)
	defer conn2.Close()

	// Enable encryption on both connections
	err = conn1.EnableEncryption(keyPair1)
	require.NoError(t, err)
	err = conn2.EnableEncryption(keyPair2)
	require.NoError(t, err)

	// Verify encryption is enabled
	assert.True(t, conn1.IsEncryptionEnabled())
	assert.True(t, conn2.IsEncryptionEnabled())

	// Register peer keys (each connection needs to know the other's public key)
	udpAddr1 := conn1.udpConn.LocalAddr()
	udpAddr2 := conn2.udpConn.LocalAddr()

	conn1.AddPeerKey(udpAddr2, keyPair2.Public)
	conn2.AddPeerKey(udpAddr1, keyPair1.Public)

	// Test encrypted message sending
	testMessage := []byte("Encrypted Hello, Tox!")

	// Send encrypted from conn1 to conn2
	n, err := conn1.WriteTo(testMessage, udpAddr2)
	require.NoError(t, err)
	assert.Equal(t, len(testMessage), n)

	// Read and decrypt on conn2
	buffer := make([]byte, 1024)
	conn2.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, _, err = conn2.ReadFrom(buffer)
	require.NoError(t, err)
	assert.Equal(t, testMessage, buffer[:n])
}

// TestToxPacketConnEnableEncryptionNilKey tests that EnableEncryption rejects nil keys
func TestToxPacketConnEnableEncryptionNilKey(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	conn, err := NewToxPacketConn(localAddr, ":0")
	require.NoError(t, err)
	defer conn.Close()

	// Enabling encryption with nil key should fail
	err = conn.EnableEncryption(nil)
	assert.Error(t, err)
	assert.False(t, conn.IsEncryptionEnabled())
}

// TestToxPacketConnWriteToNoPeerKey tests that encrypted write fails without peer key
func TestToxPacketConnWriteToNoPeerKey(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	conn, err := NewToxPacketConn(localAddr, ":0")
	require.NoError(t, err)
	defer conn.Close()

	// Enable encryption without adding peer key
	err = conn.EnableEncryption(keyPair)
	require.NoError(t, err)

	// Write to unknown peer should fail
	destAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
	_, err = conn.WriteTo([]byte("test"), destAddr)
	assert.Error(t, err)

	var toxErr *ToxNetError
	assert.ErrorAs(t, err, &toxErr)
	assert.Equal(t, "encrypt", toxErr.Op)
}

// TestToxPacketConnRemovePeerKey tests peer key removal
func TestToxPacketConnRemovePeerKey(t *testing.T) {
	keyPair1, err := crypto.GenerateKeyPair()
	require.NoError(t, err)
	keyPair2, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair1.Public, nospam)

	conn, err := NewToxPacketConn(localAddr, ":0")
	require.NoError(t, err)
	defer conn.Close()

	err = conn.EnableEncryption(keyPair1)
	require.NoError(t, err)

	destAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}

	// Add peer key
	conn.AddPeerKey(destAddr, keyPair2.Public)

	// Should be able to encrypt (even if send fails due to no listener)
	_, _ = conn.WriteTo([]byte("test"), destAddr)

	// Remove peer key
	conn.RemovePeerKey(destAddr)

	// Now write should fail due to missing peer key
	_, err = conn.WriteTo([]byte("test"), destAddr)
	assert.Error(t, err)
}

// TestToxPacketConnEncryptionDisabled tests that non-encrypted connections work
func TestToxPacketConnEncryptionDisabled(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := NewToxAddrFromPublicKey(keyPair.Public, nospam)

	conn, err := NewToxPacketConn(localAddr, ":0")
	require.NoError(t, err)
	defer conn.Close()

	// Encryption should be disabled by default
	assert.False(t, conn.IsEncryptionEnabled())

	// WriteTo should work without encryption (sending raw data)
	destAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
	n, err := conn.WriteTo([]byte("test"), destAddr)
	assert.NoError(t, err)
	assert.Equal(t, 4, n)
}

// TestToxPacketConnBidirectionalEncryption tests both send and receive encryption
func TestToxPacketConnBidirectionalEncryption(t *testing.T) {
	// Generate key pairs for two peers
	keyPair1, err := crypto.GenerateKeyPair()
	require.NoError(t, err)
	keyPair2, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	addr1 := NewToxAddrFromPublicKey(keyPair1.Public, nospam)
	addr2 := NewToxAddrFromPublicKey(keyPair2.Public, nospam)

	// Create two packet connections
	conn1, err := NewToxPacketConn(addr1, ":0")
	require.NoError(t, err)
	defer conn1.Close()

	conn2, err := NewToxPacketConn(addr2, ":0")
	require.NoError(t, err)
	defer conn2.Close()

	// Enable encryption on both
	require.NoError(t, conn1.EnableEncryption(keyPair1))
	require.NoError(t, conn2.EnableEncryption(keyPair2))

	udpAddr1 := conn1.udpConn.LocalAddr()
	udpAddr2 := conn2.udpConn.LocalAddr()

	conn1.AddPeerKey(udpAddr2, keyPair2.Public)
	conn2.AddPeerKey(udpAddr1, keyPair1.Public)

	// Test both directions
	// Direction 1: conn1 -> conn2
	msg1 := []byte("Message from peer 1")
	_, err = conn1.WriteTo(msg1, udpAddr2)
	require.NoError(t, err)

	buffer := make([]byte, 1024)
	conn2.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, _, err := conn2.ReadFrom(buffer)
	require.NoError(t, err)
	assert.Equal(t, msg1, buffer[:n])

	// Direction 2: conn2 -> conn1
	msg2 := []byte("Response from peer 2")
	_, err = conn2.WriteTo(msg2, udpAddr1)
	require.NoError(t, err)

	conn1.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, _, err = conn1.ReadFrom(buffer)
	require.NoError(t, err)
	assert.Equal(t, msg2, buffer[:n])
}
