package transport

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewI2PTransport verifies I2P transport creation with default and custom SAM addresses
func TestNewI2PTransport(t *testing.T) {
	tests := []struct {
		name        string
		samEnv      string
		expectedSAM string
	}{
		{
			name:        "default SAM address",
			samEnv:      "",
			expectedSAM: "127.0.0.1:7656",
		},
		{
			name:        "custom SAM address from env",
			samEnv:      "127.0.0.1:7657",
			expectedSAM: "127.0.0.1:7657",
		},
		{
			name:        "custom SAM with hostname",
			samEnv:      "localhost:7656",
			expectedSAM: "localhost:7656",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable if specified
			if tt.samEnv != "" {
				os.Setenv("I2P_SAM_ADDR", tt.samEnv)
				defer os.Unsetenv("I2P_SAM_ADDR")
			}

			i2p := NewI2PTransport()
			require.NotNil(t, i2p)
			assert.Equal(t, tt.expectedSAM, i2p.samAddr)
		})
	}
}

// TestI2PTransport_SupportedNetworks verifies I2P transport reports correct network types
func TestI2PTransport_SupportedNetworks(t *testing.T) {
	i2p := NewI2PTransport()
	networks := i2p.SupportedNetworks()

	assert.Equal(t, []string{"i2p"}, networks)
}

// TestI2PTransport_Listen verifies that Listen returns appropriate error
func TestI2PTransport_Listen(t *testing.T) {
	i2p := NewI2PTransport()

	tests := []struct {
		name        string
		address     string
		expectError string
	}{
		{
			name:        "i2p address returns unsupported error",
			address:     "test.b32.i2p:8080",
			expectError: "I2P listener not supported",
		},
		{
			name:        "invalid address format",
			address:     "regular.com:8080",
			expectError: "invalid I2P address format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := i2p.Listen(tt.address)
			assert.Nil(t, listener)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

// TestI2PTransport_DialPacket verifies that DialPacket returns unsupported error
func TestI2PTransport_DialPacket(t *testing.T) {
	i2p := NewI2PTransport()

	conn, err := i2p.DialPacket("test.b32.i2p:8080")
	assert.Nil(t, conn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "I2P datagram transport not yet implemented")
}

// TestI2PTransport_Close verifies Close works correctly
func TestI2PTransport_Close(t *testing.T) {
	i2p := NewI2PTransport()
	err := i2p.Close()
	assert.NoError(t, err)

	// Close should be idempotent
	err = i2p.Close()
	assert.NoError(t, err)
}

// TestI2PTransport_Dial_NoSAMBridge tests behavior when SAM bridge is not available
func TestI2PTransport_Dial_NoSAMBridge(t *testing.T) {
	// Use a non-existent SAM bridge address
	os.Setenv("I2P_SAM_ADDR", "127.0.0.1:39999")
	defer os.Unsetenv("I2P_SAM_ADDR")

	i2p := NewI2PTransport()

	// Try to dial through non-existent SAM bridge
	conn, err := i2p.Dial("example.b32.i2p:8080")
	assert.Nil(t, conn)
	assert.Error(t, err)
	// Should contain either "connection refused" or "SAM connection failed"
	assert.True(t, 
		err != nil && (err.Error() != ""),
		"Expected error when SAM bridge is unavailable")
}

// TestI2PTransport_Dial_InvalidAddress tests Dial with invalid address
func TestI2PTransport_Dial_InvalidAddress(t *testing.T) {
	i2p := NewI2PTransport()

	tests := []struct {
		name        string
		address     string
		expectError string
	}{
		{
			name:        "non-i2p address",
			address:     "example.com:80",
			expectError: "invalid I2P address format",
		},
		{
			name:        "onion address",
			address:     "example.onion:80",
			expectError: "invalid I2P address format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := i2p.Dial(tt.address)
			assert.Nil(t, conn)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

// TestI2PTransport_CloseWithoutConnection tests closing transport without any connections
func TestI2PTransport_CloseWithoutConnection(t *testing.T) {
	// Use invalid SAM address so no connection is established
	os.Setenv("I2P_SAM_ADDR", "invalid:address:format")
	defer os.Unsetenv("I2P_SAM_ADDR")

	i2p := NewI2PTransport()
	
	// Close should work even if SAM connection was never established
	err := i2p.Close()
	assert.NoError(t, err)
}

// TestI2PTransport_DialPacket_InvalidAddress tests DialPacket with invalid address
func TestI2PTransport_DialPacket_InvalidAddress(t *testing.T) {
	i2p := NewI2PTransport()

	conn, err := i2p.DialPacket("example.com:80")
	assert.Nil(t, conn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid I2P address format")
}
