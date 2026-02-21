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

// TestNewTorTransport verifies Tor transport creation with default and custom control addresses
func TestNewTorTransport(t *testing.T) {
	tests := []struct {
		name            string
		controlEnv      string
		expectedControl string
	}{
		{
			name:            "default control address",
			controlEnv:      "",
			expectedControl: "127.0.0.1:9051",
		},
		{
			name:            "custom control address from env",
			controlEnv:      "127.0.0.1:9151",
			expectedControl: "127.0.0.1:9151",
		},
		{
			name:            "custom control with hostname",
			controlEnv:      "localhost:9051",
			expectedControl: "localhost:9051",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable if specified
			if tt.controlEnv != "" {
				os.Setenv("TOR_CONTROL_ADDR", tt.controlEnv)
				defer os.Unsetenv("TOR_CONTROL_ADDR")
			}

			tor := NewTorTransport()
			require.NotNil(t, tor)
			assert.Equal(t, tt.expectedControl, tor.controlAddr)
		})
	}
}

// TestTorTransport_SupportedNetworks verifies Tor transport reports correct network types
func TestTorTransport_SupportedNetworks(t *testing.T) {
	tor := NewTorTransport()
	networks := tor.SupportedNetworks()

	assert.Equal(t, []string{"tor"}, networks)
}

// TestTorTransport_Listen verifies that Listen requires .onion address format
func TestTorTransport_Listen(t *testing.T) {
	tor := NewTorTransport()

	tests := []struct {
		name        string
		address     string
		expectError string
	}{
		{
			name:        "valid onion address format",
			address:     "test.onion:8080",
			expectError: "Tor onramp initialization failed", // Expected since Tor not running
		},
		{
			name:        "invalid address format without .onion",
			address:     "regular.com:8080",
			expectError: "invalid Tor address format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := tor.Listen(tt.address)
			if listener != nil {
				listener.Close()
			}
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

// TestTorTransport_Dial_NoControl tests behavior when Tor control port is not available
func TestTorTransport_Dial_NoControl(t *testing.T) {
	// Use a non-existent control address
	os.Setenv("TOR_CONTROL_ADDR", "127.0.0.1:19999")
	defer os.Unsetenv("TOR_CONTROL_ADDR")

	tor := NewTorTransport()

	// Try to dial through non-existent Tor control
	conn, err := tor.Dial("example.onion:8080")
	assert.Nil(t, conn)
	assert.Error(t, err)
	// Should contain error about Tor initialization or dial failure
	assert.True(t, strings.Contains(err.Error(), "Tor") || strings.Contains(err.Error(), "dial"))
}

// TestTorTransport_OnionInitialization tests lazy onion initialization
func TestTorTransport_OnionInitialization(t *testing.T) {
	// Create transport with invalid control address to force initialization failure
	os.Setenv("TOR_CONTROL_ADDR", "invalid:address:format")
	defer os.Unsetenv("TOR_CONTROL_ADDR")

	tor := NewTorTransport()
	// onion instance should be nil initially

	// Now try to dial - should attempt to create onion instance
	conn, err := tor.Dial("test.onion:8080")
	assert.Nil(t, conn)
	assert.Error(t, err)
	// Error should indicate Tor initialization or dial failure
	assert.True(t, strings.Contains(err.Error(), "Tor") || strings.Contains(err.Error(), "dial"))
}

// TestTorTransport_ConcurrentDials tests concurrent Dial operations for thread safety
func TestTorTransport_ConcurrentDials(t *testing.T) {
	os.Setenv("TOR_CONTROL_ADDR", "127.0.0.1:19998")
	defer os.Unsetenv("TOR_CONTROL_ADDR")

	tor := NewTorTransport()

	// Launch multiple concurrent dial attempts
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			conn, err := tor.Dial(fmt.Sprintf("test%d.onion:80", id))
			if conn != nil {
				conn.Close()
			}
			// We expect errors since Tor control port doesn't exist
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
	os.Setenv("TOR_CONTROL_ADDR", "127.0.0.1:19997")
	defer os.Unsetenv("TOR_CONTROL_ADDR")

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
			// All should fail due to non-existent Tor control port, but shouldn't panic
			assert.Error(t, err)
		})
	}
}

// TestTorTransport_CloseWithActiveOnion tests cleanup of onion instance
func TestTorTransport_CloseWithActiveOnion(t *testing.T) {
	// This test just verifies Close doesn't panic when onion is nil or set
	tor := NewTorTransport()

	// Close with nil onion
	err := tor.Close()
	assert.NoError(t, err)

	// Create another instance (would have onion after Dial/Listen if Tor was available)
	tor2 := NewTorTransport()
	err = tor2.Close()
	assert.NoError(t, err)
}
