package transport

import (
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewNymTransport verifies Nym transport creation with default and custom proxy addresses.
func TestNewNymTransport(t *testing.T) {
	tests := []struct {
		name          string
		proxyEnv      string
		expectedProxy string
	}{
		{
			name:          "default proxy address",
			proxyEnv:      "",
			expectedProxy: "127.0.0.1:1080",
		},
		{
			name:          "custom proxy address from env",
			proxyEnv:      "127.0.0.1:1180",
			expectedProxy: "127.0.0.1:1180",
		},
		{
			name:          "custom proxy with hostname",
			proxyEnv:      "localhost:1080",
			expectedProxy: "localhost:1080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.proxyEnv != "" {
				os.Setenv("NYM_CLIENT_ADDR", tt.proxyEnv)
				defer os.Unsetenv("NYM_CLIENT_ADDR")
			}

			nym := NewNymTransport()
			require.NotNil(t, nym)
			assert.Equal(t, tt.expectedProxy, nym.proxyAddr)
		})
	}
}

// TestNymTransport_SupportedNetworks verifies Nym transport reports correct network types.
func TestNymTransport_SupportedNetworks(t *testing.T) {
	nym := NewNymTransport()
	networks := nym.SupportedNetworks()

	assert.Equal(t, []string{"nym"}, networks)
}

// TestNymTransport_Listen verifies that Listen returns appropriate errors.
func TestNymTransport_Listen(t *testing.T) {
	nym := NewNymTransport()

	tests := []struct {
		name        string
		address     string
		expectError string
	}{
		{
			name:        "nym address returns unsupported error",
			address:     "test.nym:8080",
			expectError: "Nym service hosting not supported via SOCKS5",
		},
		{
			name:        "invalid address format without .nym",
			address:     "regular.com:8080",
			expectError: "invalid Nym address format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := nym.Listen(tt.address)
			assert.Nil(t, listener)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

// TestNymTransport_Dial_InvalidAddress tests Dial with invalid address formats.
func TestNymTransport_Dial_InvalidAddress(t *testing.T) {
	nym := NewNymTransport()

	tests := []struct {
		name        string
		address     string
		expectError string
	}{
		{
			name:        "non-nym address",
			address:     "example.com:80",
			expectError: "invalid Nym address format",
		},
		{
			name:        "onion address",
			address:     "example.onion:80",
			expectError: "invalid Nym address format",
		},
		{
			name:        "i2p address",
			address:     "example.b32.i2p:80",
			expectError: "invalid Nym address format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := nym.Dial(tt.address)
			assert.Nil(t, conn)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

// TestNymTransport_Dial_NoProxy tests behavior when the Nym client proxy is not available.
func TestNymTransport_Dial_NoProxy(t *testing.T) {
	os.Setenv("NYM_CLIENT_ADDR", "127.0.0.1:49999")
	defer os.Unsetenv("NYM_CLIENT_ADDR")

	nym := NewNymTransport()

	conn, err := nym.Dial("example.nym:80")
	assert.Nil(t, conn)
	assert.Error(t, err)
	// Error should mention dial failure or Nym client availability
	assert.True(t,
		strings.Contains(strings.ToLower(err.Error()), "dial") ||
			strings.Contains(err.Error(), "Nym"),
		"Expected actionable error, got: %v", err)
}

// TestNymTransport_DialPacket_InvalidAddress tests DialPacket with invalid address.
func TestNymTransport_DialPacket_InvalidAddress(t *testing.T) {
	nym := NewNymTransport()

	conn, err := nym.DialPacket("example.com:80")
	assert.Nil(t, conn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Nym address format")
}

// TestNymTransport_DialPacket_NoProxy tests DialPacket when Nym client is not available.
func TestNymTransport_DialPacket_NoProxy(t *testing.T) {
	os.Setenv("NYM_CLIENT_ADDR", "127.0.0.1:49998")
	defer os.Unsetenv("NYM_CLIENT_ADDR")

	nym := NewNymTransport()

	conn, err := nym.DialPacket("test.nym:8080")
	assert.Nil(t, conn)
	assert.Error(t, err)
	// Error should mention failure to connect
	assert.True(t,
		strings.Contains(strings.ToLower(err.Error()), "dial") ||
			strings.Contains(err.Error(), "Nym"),
		"Expected actionable error, got: %v", err)
}

// TestNymTransport_Close verifies Close doesn't return errors.
func TestNymTransport_Close(t *testing.T) {
	nym := NewNymTransport()
	err := nym.Close()
	assert.NoError(t, err)

	// Close should be idempotent
	err = nym.Close()
	assert.NoError(t, err)
}

// TestNymTransport_ConcurrentDials tests concurrent Dial operations for thread safety.
func TestNymTransport_ConcurrentDials(t *testing.T) {
	os.Setenv("NYM_CLIENT_ADDR", "127.0.0.1:49997")
	defer os.Unsetenv("NYM_CLIENT_ADDR")

	nym := NewNymTransport()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			conn, err := nym.Dial(fmt.Sprintf("test%d.nym:80", id))
			if conn != nil {
				conn.Close()
			}
			// We expect errors since proxy doesn't exist
			assert.Error(t, err)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent dials")
		}
	}
}

// TestNymTransport_AddressFormats tests various address formats with Nym transport.
func TestNymTransport_AddressFormats(t *testing.T) {
	os.Setenv("NYM_CLIENT_ADDR", "127.0.0.1:49996")
	defer os.Unsetenv("NYM_CLIENT_ADDR")

	nym := NewNymTransport()

	validAddresses := []struct {
		addr        string
		description string
	}{
		{"example.nym:80", "standard nym address"},
		{"longexamplenymaddress.nym:443", "long nym address"},
		{"sub.example.nym:8080", "subdomain nym address"},
	}

	for _, test := range validAddresses {
		t.Run(test.description, func(t *testing.T) {
			conn, err := nym.Dial(test.addr)
			if conn != nil {
				conn.Close()
			}
			// All should fail due to non-existent proxy, but shouldn't panic
			assert.Error(t, err)
			assert.NotContains(t, err.Error(), "invalid Nym address format")
		})
	}

	invalidAddresses := []struct {
		addr        string
		description string
	}{
		{"example.onion:80", "onion address"},
		{"example.loki:80", "loki address"},
		{"127.0.0.1:8080", "IP address"},
		{"example.com:80", "regular domain"},
	}

	for _, test := range invalidAddresses {
		t.Run("invalid_"+test.description, func(t *testing.T) {
			conn, err := nym.Dial(test.addr)
			if conn != nil {
				conn.Close()
			}
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid Nym address format")
		})
	}
}

// TestNymPacketConn_FramingEncoding tests packet framing encode/decode round-trip.
func TestNymPacketConn_FramingEncoding(t *testing.T) {
	// Create an in-process pipe to simulate a stream connection
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	clientPC := newNymPacketConn(clientConn)
	serverPC := newNymPacketConn(serverConn)

	payload := []byte("hello nym packet framing test")

	// Write from client
	done := make(chan error, 1)
	go func() {
		n, err := clientPC.WriteTo(payload, clientConn.RemoteAddr())
		if err != nil {
			done <- err
			return
		}
		if n != len(payload) {
			done <- fmt.Errorf("wrote %d bytes, expected %d", n, len(payload))
			return
		}
		done <- nil
	}()

	// Read on server
	buf := make([]byte, 1024)
	n, addr, err := serverPC.ReadFrom(buf)
	require.NoError(t, err)
	assert.Equal(t, len(payload), n)
	assert.Equal(t, payload, buf[:n])
	assert.NotNil(t, addr)

	writeErr := <-done
	assert.NoError(t, writeErr)
}

// TestNymPacketConn_MultiplePackets tests sending and receiving multiple packets.
func TestNymPacketConn_MultiplePackets(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	clientPC := newNymPacketConn(clientConn)
	serverPC := newNymPacketConn(serverConn)

	packets := [][]byte{
		[]byte("packet one"),
		[]byte("packet two with more data"),
		{0x00, 0x01, 0x02, 0x03}, // binary data
		[]byte("final packet"),
	}

	done := make(chan error, 1)
	go func() {
		for _, pkt := range packets {
			if _, err := clientPC.WriteTo(pkt, nil); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()

	buf := make([]byte, 4096)
	for i, expected := range packets {
		n, _, err := serverPC.ReadFrom(buf)
		require.NoError(t, err, "reading packet %d", i)
		assert.Equal(t, expected, buf[:n], "packet %d mismatch", i)
	}

	assert.NoError(t, <-done)
}

// TestNymPacketConn_LocalAddr verifies LocalAddr returns the underlying connection's addr.
func TestNymPacketConn_LocalAddr(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	pc := newNymPacketConn(clientConn)
	assert.NotNil(t, pc.LocalAddr())
}

// TestNymPacketConn_Close verifies Close closes the underlying connection.
func TestNymPacketConn_Close(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()

	pc := newNymPacketConn(clientConn)
	err := pc.Close()
	assert.NoError(t, err)

	// Subsequent operations should fail after close
	_, err = pc.WriteTo([]byte("data"), nil)
	assert.Error(t, err)
}

// TestNymPacketConn_BufferTooSmall verifies behavior when read buffer is smaller than packet.
func TestNymPacketConn_BufferTooSmall(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	clientPC := newNymPacketConn(clientConn)
	serverPC := newNymPacketConn(serverConn)

	payload := []byte("this is a large payload that will not fit in the small buffer")

	go func() {
		clientPC.WriteTo(payload, nil)
	}()

	// Use a buffer too small to hold the packet
	smallBuf := make([]byte, 5)
	_, _, err := serverPC.ReadFrom(smallBuf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "buffer too small")
}

// TestNymTransport_Integration only runs when a local Nym client is available.
// Set NYM_INTEGRATION_TEST=1 and ensure the Nym client is running on NYM_CLIENT_ADDR.
func TestNymTransport_Integration(t *testing.T) {
	if os.Getenv("NYM_INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test: set NYM_INTEGRATION_TEST=1 with a running Nym client")
	}

	nym := NewNymTransport()
	defer nym.Close()

	// Attempt a dial - the address may not exist but the SOCKS5 handshake should succeed
	conn, err := nym.Dial("test.nym:80")
	if err != nil {
		t.Logf("Integration dial error (expected if address doesn't exist): %v", err)
		assert.NotContains(t, err.Error(), "is Nym client running",
			"Should not get 'client not running' error during integration test")
		return
	}
	if conn != nil {
		conn.Close()
	}
}
