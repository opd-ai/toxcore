package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// TestTorTransportCreation verifies Tor transport can be created with default and custom proxy addresses
func TestTorTransportCreation(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		wantAddr string
	}{
		{
			name:     "default proxy address",
			envValue: "",
			wantAddr: "127.0.0.1:9050",
		},
		{
			name:     "custom proxy address",
			envValue: "192.168.1.100:9150",
			wantAddr: "192.168.1.100:9150",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("TOR_PROXY_ADDR", tt.envValue)
				defer os.Unsetenv("TOR_PROXY_ADDR")
			}

			tor := transport.NewTorTransport()
			if tor == nil {
				t.Fatal("NewTorTransport returned nil")
			}
			defer tor.Close()

			networks := tor.SupportedNetworks()
			if len(networks) != 1 || networks[0] != "tor" {
				t.Errorf("SupportedNetworks() = %v, want [tor]", networks)
			}
		})
	}
}

// TestI2PTransportCreation verifies I2P transport can be created with default and custom SAM addresses
func TestI2PTransportCreation(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		wantAddr string
	}{
		{
			name:     "default SAM address",
			envValue: "",
			wantAddr: "127.0.0.1:7656",
		},
		{
			name:     "custom SAM address",
			envValue: "192.168.1.100:7657",
			wantAddr: "192.168.1.100:7657",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("I2P_SAM_ADDR", tt.envValue)
				defer os.Unsetenv("I2P_SAM_ADDR")
			}

			i2p := transport.NewI2PTransport()
			if i2p == nil {
				t.Fatal("NewI2PTransport returned nil")
			}
			defer i2p.Close()

			networks := i2p.SupportedNetworks()
			if len(networks) != 1 || networks[0] != "i2p" {
				t.Errorf("SupportedNetworks() = %v, want [i2p]", networks)
			}
		})
	}
}

// TestLokinetTransportCreation verifies Lokinet transport can be created with default and custom proxy addresses
func TestLokinetTransportCreation(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		wantAddr string
	}{
		{
			name:     "default proxy address",
			envValue: "",
			wantAddr: "127.0.0.1:9050",
		},
		{
			name:     "custom proxy address",
			envValue: "192.168.1.100:9060",
			wantAddr: "192.168.1.100:9060",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("LOKINET_PROXY_ADDR", tt.envValue)
				defer os.Unsetenv("LOKINET_PROXY_ADDR")
			}

			lokinet := transport.NewLokinetTransport()
			if lokinet == nil {
				t.Fatal("NewLokinetTransport returned nil")
			}
			defer lokinet.Close()

			networks := lokinet.SupportedNetworks()
			if len(networks) != 2 || networks[0] != "loki" || networks[1] != "lokinet" {
				t.Errorf("SupportedNetworks() = %v, want [loki lokinet]", networks)
			}
		})
	}
}

// TestTorTransportDialExpectedFailure verifies Tor transport dial fails gracefully when Tor is not running
func TestTorTransportDialExpectedFailure(t *testing.T) {
	tor := transport.NewTorTransport()
	defer tor.Close()

	// Dial should fail since Tor proxy is not running
	conn, err := tor.Dial("3g2upl4pq6kufc4m.onion:80")
	if err == nil {
		conn.Close()
		t.Skip("Tor proxy is running; skipping expected failure test")
	}

	// Expected: connection refused or timeout error
	if conn != nil {
		t.Error("Expected nil connection when Tor is not running")
	}
}

// TestI2PTransportDialExpectedFailure verifies I2P transport dial fails gracefully when I2P is not running
func TestI2PTransportDialExpectedFailure(t *testing.T) {
	i2p := transport.NewI2PTransport()
	defer i2p.Close()

	// Dial should fail since I2P SAM bridge is not running
	conn, err := i2p.Dial("ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80")
	if err == nil {
		conn.Close()
		t.Skip("I2P SAM bridge is running; skipping expected failure test")
	}

	// Expected: connection refused or timeout error
	if conn != nil {
		t.Error("Expected nil connection when I2P is not running")
	}
}

// TestLokinetTransportDialExpectedFailure verifies Lokinet transport dial fails gracefully when Lokinet is not running
func TestLokinetTransportDialExpectedFailure(t *testing.T) {
	lokinet := transport.NewLokinetTransport()
	defer lokinet.Close()

	// Dial should fail since Lokinet daemon is not running
	conn, err := lokinet.Dial("example.loki:80")
	if err == nil {
		conn.Close()
		t.Skip("Lokinet daemon is running; skipping expected failure test")
	}

	// Expected: connection refused or timeout error
	if conn != nil {
		t.Error("Expected nil connection when Lokinet is not running")
	}
}

// TestDemonstrateTorTransport verifies the Tor demonstration function runs without panicking
func TestDemonstrateTorTransport(t *testing.T) {
	// Capture log output to avoid polluting test output
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(os.Stdout)

	// Should not panic
	demonstrateTorTransport()

	// Verify log output contains expected messages
	output := buf.String()
	if len(output) == 0 {
		t.Error("Expected log output from demonstrateTorTransport")
	}
}

// TestDemonstrateI2PTransport verifies the I2P demonstration function runs without panicking
func TestDemonstrateI2PTransport(t *testing.T) {
	// Capture log output to avoid polluting test output
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(os.Stdout)

	// Should not panic
	demonstrateI2PTransport()

	// Verify log output contains expected messages
	output := buf.String()
	if len(output) == 0 {
		t.Error("Expected log output from demonstrateI2PTransport")
	}
}

// TestDemonstrateLokinetTransport verifies the Lokinet demonstration function runs without panicking
func TestDemonstrateLokinetTransport(t *testing.T) {
	// Capture log output to avoid polluting test output
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(os.Stdout)

	// Should not panic
	demonstrateLokinetTransport()

	// Verify log output contains expected messages
	output := buf.String()
	if len(output) == 0 {
		t.Error("Expected log output from demonstrateLokinetTransport")
	}
}

// TestTransportResourceCleanup verifies transports release resources correctly
func TestTransportResourceCleanup(t *testing.T) {
	// Create and close multiple times to check for resource leaks
	for i := 0; i < 5; i++ {
		tor := transport.NewTorTransport()
		tor.Close()

		i2p := transport.NewI2PTransport()
		i2p.Close()

		lokinet := transport.NewLokinetTransport()
		lokinet.Close()
	}

	// No panic or hang means resources are cleaned up correctly
}

// TestSupportedNetworksConsistency verifies each transport returns consistent network names
func TestSupportedNetworksConsistency(t *testing.T) {
	tests := []struct {
		name     string
		factory  func() interface{ SupportedNetworks() []string }
		expected []string
	}{
		{
			name:     "TorTransport",
			factory:  func() interface{ SupportedNetworks() []string } { return transport.NewTorTransport() },
			expected: []string{"tor"},
		},
		{
			name:     "I2PTransport",
			factory:  func() interface{ SupportedNetworks() []string } { return transport.NewI2PTransport() },
			expected: []string{"i2p"},
		},
		{
			name:     "LokinetTransport",
			factory:  func() interface{ SupportedNetworks() []string } { return transport.NewLokinetTransport() },
			expected: []string{"loki", "lokinet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := tt.factory()

			// Call SupportedNetworks multiple times to verify consistency
			for i := 0; i < 3; i++ {
				networks := transport.SupportedNetworks()
				if len(networks) != len(tt.expected) {
					t.Errorf("SupportedNetworks() returned %d networks, want %d", len(networks), len(tt.expected))
					continue
				}
				for j, network := range networks {
					if network != tt.expected[j] {
						t.Errorf("SupportedNetworks()[%d] = %q, want %q", j, network, tt.expected[j])
					}
				}
			}
		})
	}
}
