//go:build integration
// +build integration

package internal

import (
	"context"
	"testing"
	"time"
)

// Integration tests that require actual Tox instances.
// Run with: go test -tags=integration ./internal/...
// These tests may take longer and require network resources.

// TestNewBootstrapServerCreation tests creating a bootstrap server with real Tox instance.
func TestNewBootstrapServerCreation(t *testing.T) {
	t.Skip("Skipping integration test - requires isolated network port")

	config := DefaultBootstrapConfig()
	config.Port = 44001 // Use a different port to avoid conflicts
	config.InitDelay = 100 * time.Millisecond

	server, err := NewBootstrapServer(config)
	if err != nil {
		t.Fatalf("NewBootstrapServer failed: %v", err)
	}

	// Verify server was created
	if server == nil {
		t.Fatal("Server should not be nil")
	}

	// Clean up
	if server.tox != nil {
		server.tox.Kill()
	}
}

// TestNewTestClientCreation tests creating a test client with real Tox instance.
func TestNewTestClientCreation(t *testing.T) {
	t.Skip("Skipping integration test - requires isolated network port")

	config := DefaultClientConfig("IntegrationTest")

	client, err := NewTestClient(config)
	if err != nil {
		t.Fatalf("NewTestClient failed: %v", err)
	}

	// Verify client was created
	if client == nil {
		t.Fatal("Client should not be nil")
	}

	// Clean up
	if client.tox != nil {
		client.tox.Kill()
	}
}

// TestBootstrapServerGetStatusWithTox tests GetStatus with a real Tox instance.
func TestBootstrapServerGetStatusWithTox(t *testing.T) {
	t.Skip("Skipping integration test - requires isolated network port")

	config := DefaultBootstrapConfig()
	config.Port = 44002
	config.InitDelay = 100 * time.Millisecond

	server, err := NewBootstrapServer(config)
	if err != nil {
		t.Fatalf("NewBootstrapServer failed: %v", err)
	}
	defer func() {
		if server.tox != nil {
			server.tox.Kill()
		}
	}()

	// Test GetStatus
	status := server.GetStatus()
	if status == nil {
		t.Fatal("GetStatus should not return nil")
	}

	// Verify expected keys exist
	if _, ok := status["address"]; !ok {
		t.Error("Status should contain 'address' key")
	}
	if _, ok := status["port"]; !ok {
		t.Error("Status should contain 'port' key")
	}
	if _, ok := status["running"]; !ok {
		t.Error("Status should contain 'running' key")
	}
}

// TestBootstrapServerGetStatusTypedWithTox tests GetStatusTyped with a real Tox instance.
func TestBootstrapServerGetStatusTypedWithTox(t *testing.T) {
	t.Skip("Skipping integration test - requires isolated network port")

	config := DefaultBootstrapConfig()
	config.Port = 44003
	config.InitDelay = 100 * time.Millisecond

	server, err := NewBootstrapServer(config)
	if err != nil {
		t.Fatalf("NewBootstrapServer failed: %v", err)
	}
	defer func() {
		if server.tox != nil {
			server.tox.Kill()
		}
	}()

	// Test GetStatusTyped
	status := server.GetStatusTyped()

	if status.Address != config.Address {
		t.Errorf("Address = %q, want %q", status.Address, config.Address)
	}
	if status.Port != config.Port {
		t.Errorf("Port = %d, want %d", status.Port, config.Port)
	}
}

// TestClientGetStatusWithTox tests client GetStatus with a real Tox instance.
func TestClientGetStatusWithTox(t *testing.T) {
	t.Skip("Skipping integration test - requires isolated network port")

	config := DefaultClientConfig("StatusTest")

	client, err := NewTestClient(config)
	if err != nil {
		t.Fatalf("NewTestClient failed: %v", err)
	}
	defer func() {
		if client.tox != nil {
			client.tox.Kill()
		}
	}()

	// Test GetStatus
	status := client.GetStatus()
	if status == nil {
		t.Fatal("GetStatus should not return nil")
	}

	// Verify expected keys exist
	if _, ok := status["name"]; !ok {
		t.Error("Status should contain 'name' key")
	}
	if _, ok := status["connected"]; !ok {
		t.Error("Status should contain 'connected' key")
	}
}

// TestClientGetPublicKeyWithTox tests client GetPublicKey with a real Tox instance.
func TestClientGetPublicKeyWithTox(t *testing.T) {
	t.Skip("Skipping integration test - requires isolated network port")

	config := DefaultClientConfig("PubKeyTest")

	client, err := NewTestClient(config)
	if err != nil {
		t.Fatalf("NewTestClient failed: %v", err)
	}
	defer func() {
		if client.tox != nil {
			client.tox.Kill()
		}
	}()

	// Test GetPublicKey
	pubKey := client.GetPublicKey()

	// Public key should be 32 bytes and non-zero
	allZero := true
	for _, b := range pubKey {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("Public key should not be all zeros")
	}
}

// TestBootstrapServerStartAndStop tests the full start/stop lifecycle.
func TestBootstrapServerStartAndStop(t *testing.T) {
	t.Skip("Skipping integration test - requires isolated network port")

	config := DefaultBootstrapConfig()
	config.Port = 44004
	config.InitDelay = 100 * time.Millisecond

	server, err := NewBootstrapServer(config)
	if err != nil {
		t.Fatalf("NewBootstrapServer failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start the server
	err = server.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify running
	if !server.IsRunning() {
		t.Error("Server should be running after Start")
	}

	// Stop the server
	err = server.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify stopped
	if server.IsRunning() {
		t.Error("Server should not be running after Stop")
	}
}
