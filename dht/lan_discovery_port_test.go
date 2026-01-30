package dht

import (
	"testing"
)

// TestLANDiscoveryPortOffset verifies that LAN discovery uses port+1 to avoid conflicts.
func TestLANDiscoveryPortOffset(t *testing.T) {
	tests := []struct {
		name         string
		port         uint16
		expectedPort uint16
	}{
		{
			name:         "standard port",
			port:         33445,
			expectedPort: 33446,
		},
		{
			name:         "custom port",
			port:         12345,
			expectedPort: 12346,
		},
		{
			name:         "high port",
			port:         65534,
			expectedPort: 65535,
		},
		{
			name:         "edge case max port",
			port:         65535,
			expectedPort: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var publicKey [32]byte
			ld := NewLANDiscovery(publicKey, tt.port)

			if ld.port != tt.port {
				t.Errorf("Expected port %d, got %d", tt.port, ld.port)
			}

			if ld.discoveryPort != tt.expectedPort {
				t.Errorf("Expected discovery port %d, got %d", tt.expectedPort, ld.discoveryPort)
			}
		})
	}
}
