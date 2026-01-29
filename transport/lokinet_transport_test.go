package transport

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewLokinetTransport verifies Lokinet transport creation with default and custom proxy addresses
func TestNewLokinetTransport(t *testing.T) {
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
				os.Setenv("LOKINET_PROXY_ADDR", tt.proxyEnv)
				defer os.Unsetenv("LOKINET_PROXY_ADDR")
			}

			lokinet := NewLokinetTransport()
			require.NotNil(t, lokinet)
			assert.Equal(t, tt.expectedProxy, lokinet.proxyAddr)
		})
	}
}

// TestLokinetTransport_SupportedNetworks verifies Lokinet transport reports correct network types
func TestLokinetTransport_SupportedNetworks(t *testing.T) {
	lokinet := NewLokinetTransport()
	networks := lokinet.SupportedNetworks()

	assert.Equal(t, []string{"loki", "lokinet"}, networks)
}

// TestLokinetTransport_Listen verifies that Listen returns appropriate error
func TestLokinetTransport_Listen(t *testing.T) {
	lokinet := NewLokinetTransport()

	tests := []struct {
		name        string
		address     string
		expectError string
	}{
		{
			name:        "loki address returns unsupported error",
			address:     "test.loki:8080",
			expectError: "Lokinet SNApp hosting not supported via SOCKS5",
		},
		{
			name:        "invalid address format",
			address:     "regular.com:8080",
			expectError: "invalid Lokinet address format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := lokinet.Listen(tt.address)
			assert.Nil(t, listener)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

// TestLokinetTransport_DialPacket verifies that DialPacket returns unsupported error
func TestLokinetTransport_DialPacket(t *testing.T) {
	lokinet := NewLokinetTransport()

	conn, err := lokinet.DialPacket("test.loki:8080")
	assert.Nil(t, conn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Lokinet UDP transport not supported")
}

// TestLokinetTransport_Close verifies Close doesn't return errors
func TestLokinetTransport_Close(t *testing.T) {
	lokinet := NewLokinetTransport()
	err := lokinet.Close()
	assert.NoError(t, err)
}

// TestLokinetTransport_Dial_NoProxy tests behavior when proxy is not available
func TestLokinetTransport_Dial_NoProxy(t *testing.T) {
	// Use a non-existent proxy address
	os.Setenv("LOKINET_PROXY_ADDR", "127.0.0.1:29999")
	defer os.Unsetenv("LOKINET_PROXY_ADDR")

	lokinet := NewLokinetTransport()

	// Try to dial through non-existent proxy
	conn, err := lokinet.Dial("example.loki:8080")
	assert.Nil(t, conn)
	assert.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "dial")
}

// TestLokinetTransport_DialerInitialization tests lazy dialer initialization
func TestLokinetTransport_DialerInitialization(t *testing.T) {
	// Create transport with invalid proxy to force dialer failure
	os.Setenv("LOKINET_PROXY_ADDR", "invalid:address:format")
	defer os.Unsetenv("LOKINET_PROXY_ADDR")

	lokinet := NewLokinetTransport()

	// Now try to dial - should attempt to recreate dialer
	conn, err := lokinet.Dial("test.loki:8080")
	assert.Nil(t, conn)
	assert.Error(t, err)
}

// TestLokinetTransport_Integration_MockSOCKS5 tests Lokinet transport with a mock SOCKS5 server
func TestLokinetTransport_Integration_MockSOCKS5(t *testing.T) {
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

	// Configure Lokinet transport to use mock server
	os.Setenv("LOKINET_PROXY_ADDR", mockServer.Addr().String())
	defer os.Unsetenv("LOKINET_PROXY_ADDR")

	lokinet := NewLokinetTransport()

	// Attempt to dial through mock SOCKS5
	conn, err := lokinet.Dial("example.loki:80")
	if conn != nil {
		conn.Close()
	}
	// We expect an error since mock server doesn't implement full SOCKS5 protocol
	assert.Error(t, err)
}

// TestLokinetTransport_ConcurrentDials tests concurrent Dial operations for thread safety
func TestLokinetTransport_ConcurrentDials(t *testing.T) {
	os.Setenv("LOKINET_PROXY_ADDR", "127.0.0.1:29998")
	defer os.Unsetenv("LOKINET_PROXY_ADDR")

	lokinet := NewLokinetTransport()

	// Launch multiple concurrent dial attempts
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			conn, err := lokinet.Dial(fmt.Sprintf("test%d.loki:80", id))
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

// TestLokinetTransport_AddressFormats tests various address formats
func TestLokinetTransport_AddressFormats(t *testing.T) {
	os.Setenv("LOKINET_PROXY_ADDR", "127.0.0.1:29997")
	defer os.Unsetenv("LOKINET_PROXY_ADDR")

	lokinet := NewLokinetTransport()

	addresses := []struct {
		addr        string
		description string
	}{
		{"example.loki:80", "standard loki address"},
		{"longexamplelokiaddressxxxxxxxxxxxxxxxxxx.loki:443", "long loki address"},
		{"regular.example.com:80", "regular domain through Lokinet"},
		{"192.168.1.1:8080", "IP address through Lokinet"},
	}

	for _, test := range addresses {
		t.Run(test.description, func(t *testing.T) {
			conn, err := lokinet.Dial(test.addr)
			if conn != nil {
				conn.Close()
			}
			// All should fail due to non-existent proxy, but shouldn't panic
			assert.Error(t, err)
		})
	}
}
