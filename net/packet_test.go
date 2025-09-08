package net

import (
	"fmt"
	"net"
	"testing"
	"time"

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
	_, err := PacketListen("invalid", ":0")
	assert.Error(t, err)

	// Test PacketDial with invalid network
	_, err = PacketDial("invalid", "test-addr")
	assert.Error(t, err)

	// Test PacketDial with invalid address
	_, err = PacketDial("tox", "invalid-tox-id")
	assert.Error(t, err)
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
	assert.Equal(t, udpAddr1.String(), addr.String())

	fmt.Printf("Successfully sent and received message: %s\n", string(buffer[:n]))
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
