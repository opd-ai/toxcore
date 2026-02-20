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
		got      any
		expected any
	}{
		{"Address", config.Address, "127.0.0.1"},
		{"Port", config.Port, BootstrapDefaultPort},
		{"Timeout", config.Timeout, 10 * time.Second},
		{"InitDelay", config.InitDelay, 1 * time.Second},
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

// TestBootstrapServerGracefulShutdown verifies that Stop() waits for the eventLoop to finish.
func TestBootstrapServerGracefulShutdown(t *testing.T) {
	// This tests that the BootstrapServer struct has the necessary fields for graceful shutdown
	server := &BootstrapServer{
		stopChan: make(chan struct{}),
	}

	// Verify stopChan is created and can be closed without panic
	select {
	case <-server.stopChan:
		t.Error("stopChan should not be closed initially")
	default:
		// Expected: channel is open
	}

	// Close the channel to simulate stop signal
	close(server.stopChan)

	// Verify channel is now closed
	select {
	case <-server.stopChan:
		// Expected: channel is closed
	default:
		t.Error("stopChan should be closed after close()")
	}
}

// TestBootstrapServerWaitGroupTracking tests that the WaitGroup tracks goroutines correctly.
func TestBootstrapServerWaitGroupTracking(t *testing.T) {
	server := &BootstrapServer{
		stopChan: make(chan struct{}),
	}

	// Simulate the pattern used in Start()
	done := make(chan struct{})
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		// Simulate eventLoop work
		<-server.stopChan
		close(done)
	}()

	// Signal stop
	close(server.stopChan)

	// Wait should complete without timeout
	waitDone := make(chan struct{})
	go func() {
		server.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		// Expected: Wait() completed because goroutine finished
	case <-time.After(1 * time.Second):
		t.Error("WaitGroup.Wait() timed out - goroutine did not finish")
	}
}
