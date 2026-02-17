package internal

import (
	"testing"
	"time"
)

// TestDefaultBootstrapConfig tests the default bootstrap configuration.
func TestDefaultBootstrapConfig(t *testing.T) {
	config := DefaultBootstrapConfig()

	if config == nil {
		t.Fatal("DefaultBootstrapConfig() returned nil")
	}

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"Address", config.Address, "127.0.0.1"},
		{"Port", config.Port, uint16(33445)},
		{"Timeout", config.Timeout, 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}

	if config.Logger == nil {
		t.Error("Logger should not be nil")
	}
}

// TestServerMetricsStruct tests the ServerMetrics struct fields.
func TestServerMetricsStruct(t *testing.T) {
	now := time.Now()
	metrics := &ServerMetrics{
		StartTime:         now,
		ConnectionsServed: 100,
		PacketsProcessed:  5000,
		ActiveClients:     10,
	}

	if metrics.StartTime != now {
		t.Errorf("StartTime = %v, want %v", metrics.StartTime, now)
	}

	if metrics.ConnectionsServed != 100 {
		t.Errorf("ConnectionsServed = %d, want %d", metrics.ConnectionsServed, 100)
	}

	if metrics.PacketsProcessed != 5000 {
		t.Errorf("PacketsProcessed = %d, want %d", metrics.PacketsProcessed, 5000)
	}

	if metrics.ActiveClients != 10 {
		t.Errorf("ActiveClients = %d, want %d", metrics.ActiveClients, 10)
	}
}

// TestBootstrapConfigStruct tests the BootstrapConfig struct fields.
func TestBootstrapConfigStruct(t *testing.T) {
	config := &BootstrapConfig{
		Address: "192.168.1.100",
		Port:    44555,
		Timeout: 30 * time.Second,
	}

	if config.Address != "192.168.1.100" {
		t.Errorf("Address = %q, want %q", config.Address, "192.168.1.100")
	}

	if config.Port != 44555 {
		t.Errorf("Port = %d, want %d", config.Port, 44555)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", config.Timeout, 30*time.Second)
	}
}
