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

// TestNewTorTransport verifies Tor transport creation with default and custom proxy addresses
func TestNewTorTransport(t *testing.T) {
	tests := []struct {
		name          string
		proxyEnv      string
		expectedProxy string
	}{
		{
			name:          "default proxy address",
			proxyEnv:      "",
			expectedProxy: "127.0.0.1:9050",
		},
		{
			name:          "custom proxy address from env",
			proxyEnv:      "127.0.0.1:9150",
			expectedProxy: "127.0.0.1:9150",
		},
		{
			name:          "custom proxy with hostname",
			proxyEnv:      "localhost:9050",
			expectedProxy: "localhost:9050",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable if specified
			if tt.proxyEnv != "" {
				os.Setenv("TOR_PROXY_ADDR", tt.proxyEnv)
				defer os.Unsetenv("TOR_PROXY_ADDR")
			}

			tor := NewTorTransport()
			require.NotNil(t, tor)
			assert.Equal(t, tt.expectedProxy, tor.proxyAddr)
		})
	}
}

// TestTorTransport_SupportedNetworks verifies Tor transport reports correct network types
func TestTorTransport_SupportedNetworks(t *testing.T) {
	tor := NewTorTransport()
	networks := tor.SupportedNetworks()

	assert.Equal(t, []string{"tor"}, networks)
}

// TestTorTransport_Listen verifies that Listen returns appropriate error
func TestTorTransport_Listen(t *testing.T) {
	tor := NewTorTransport()

	tests := []struct {
		name        string
		address     string
		expectError string
	}{
		{
			name:        "onion address returns unsupported error",
			address:     "test.onion:8080",
			expectError: "Tor onion service hosting not supported via SOCKS5",
		},
		{
			name:        "invalid address format",
			address:     "regular.com:8080",
			expectError: "invalid Tor address format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := tor.Listen(tt.address)
			assert.Nil(t, listener)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

// TestTorTransport_DialPacket verifies that DialPacket returns unsupported error
func TestTorTransport_DialPacket(t *testing.T) {
	tor := NewTorTransport()

	conn, err := tor.DialPacket("test.onion:8080")
	assert.Nil(t, conn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Tor UDP transport not supported")
}

// TestTorTransport_Close verifies Close doesn't return errors
func TestTorTransport_Close(t *testing.T) {
	tor := NewTorTransport()
	err := tor.Close()
	assert.NoError(t, err)
}

// TestTorTransport_Dial_NoProxy tests behavior when proxy is not available
func TestTorTransport_Dial_NoProxy(t *testing.T) {
	// Use a non-existent proxy address
	os.Setenv("TOR_PROXY_ADDR", "127.0.0.1:19999")
	defer os.Unsetenv("TOR_PROXY_ADDR")

	tor := NewTorTransport()

	// Try to dial through non-existent proxy
	conn, err := tor.Dial("example.onion:8080")
	assert.Nil(t, conn)
	assert.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "dial")
}

// TestTorTransport_DialerInitialization tests lazy dialer initialization
func TestTorTransport_DialerInitialization(t *testing.T) {
	// Create transport with invalid proxy to force dialer failure
	os.Setenv("TOR_PROXY_ADDR", "invalid:address:format")
	defer os.Unsetenv("TOR_PROXY_ADDR")

	tor := NewTorTransport()
	// socksDialer should be nil if creation failed

	// Now try to dial - should attempt to recreate dialer
	conn, err := tor.Dial("test.onion:8080")
	assert.Nil(t, conn)
	assert.Error(t, err)
}

// TestTorTransport_Integration_MockSOCKS5 tests Tor transport with a mock SOCKS5 server
func TestTorTransport_Integration_MockSOCKS5(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start a mock SOCKS5 server
	mockServer, err := startMockSOCKS5Server()
	if err != nil {
		t.Skip("Could not start mock SOCKS5 server:", err)
		return
	}
	defer mockServer.Close()

	// Configure Tor transport to use mock server
	os.Setenv("TOR_PROXY_ADDR", mockServer.Addr().String())
	defer os.Unsetenv("TOR_PROXY_ADDR")

	tor := NewTorTransport()

	// Attempt to dial through mock SOCKS5
	// Note: This will fail auth/handshake but tests that dialing is attempted
	conn, err := tor.Dial("example.onion:80")
	if conn != nil {
		conn.Close()
	}
	// We expect an error since we're not implementing full SOCKS5 protocol in mock
	// but we verify the connection attempt was made
	assert.Error(t, err)
}

// startMockSOCKS5Server creates a simple TCP listener to simulate SOCKS5 proxy availability
func startMockSOCKS5Server() (net.Listener, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	// Accept connections in background but don't process SOCKS5 protocol
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Just close immediately - enough to test dialing behavior
			conn.Close()
		}
	}()

	return listener, nil
}

// TestTorTransport_ConcurrentDials tests concurrent Dial operations for thread safety
func TestTorTransport_ConcurrentDials(t *testing.T) {
	os.Setenv("TOR_PROXY_ADDR", "127.0.0.1:19998")
	defer os.Unsetenv("TOR_PROXY_ADDR")

	tor := NewTorTransport()

	// Launch multiple concurrent dial attempts
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			conn, err := tor.Dial(fmt.Sprintf("test%d.onion:80", id))
			if conn != nil {
				conn.Close()
			}
			// We expect errors since proxy doesn't exist
			assert.Error(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines with timeout
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent dials")
		}
	}
}

// TestTorTransport_AddressFormats tests various address formats
func TestTorTransport_AddressFormats(t *testing.T) {
	os.Setenv("TOR_PROXY_ADDR", "127.0.0.1:19997")
	defer os.Unsetenv("TOR_PROXY_ADDR")

	tor := NewTorTransport()

	addresses := []struct {
		addr        string
		description string
	}{
		{"example.onion:80", "standard onion v2"},
		{"longexamplev3addressxxxxxxxxxxxxxxxxxxxxxxxxxx.onion:443", "onion v3 address"},
		{"regular.example.com:80", "regular domain through Tor"},
		{"192.168.1.1:8080", "IP address through Tor"},
	}

	for _, test := range addresses {
		t.Run(test.description, func(t *testing.T) {
			conn, err := tor.Dial(test.addr)
			if conn != nil {
				conn.Close()
			}
			// All should fail due to non-existent proxy, but shouldn't panic
			assert.Error(t, err)
		})
	}
}
