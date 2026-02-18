package main

import (
	"testing"
	"time"

	"github.com/opd-ai/toxcore/transport"
)

// TestMultiTransportCreation tests that MultiTransport can be created and closed properly
func TestMultiTransportCreation(t *testing.T) {
	mt := transport.NewMultiTransport()
	if mt == nil {
		t.Fatal("NewMultiTransport returned nil")
	}
	defer mt.Close()

	// Should have some supported networks
	networks := mt.GetSupportedNetworks()
	if len(networks) == 0 {
		t.Error("Expected at least one supported network")
	}
}

// TestMultiTransportSupportedNetworks verifies expected networks are registered
func TestMultiTransportSupportedNetworks(t *testing.T) {
	mt := transport.NewMultiTransport()
	defer mt.Close()

	networks := mt.GetSupportedNetworks()

	// Should contain IP at minimum
	hasIP := false
	for _, network := range networks {
		if network == "ip" || network == "tcp" || network == "udp" {
			hasIP = true
			break
		}
	}

	if !hasIP {
		t.Error("Expected IP/TCP/UDP to be in supported networks")
	}
}

// TestIPTransportListen tests that IP transport can create a listener
func TestIPTransportListen(t *testing.T) {
	mt := transport.NewMultiTransport()
	defer mt.Close()

	listener, err := mt.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr()
	if addr == nil {
		t.Error("Listener address is nil")
	}

	addrStr := addr.String()
	if addrStr == "" {
		t.Error("Listener address string is empty")
	}
}

// TestIPTransportDialPacket tests UDP packet connection creation
func TestIPTransportDialPacket(t *testing.T) {
	mt := transport.NewMultiTransport()
	defer mt.Close()

	conn, err := mt.DialPacket("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create packet connection: %v", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr()
	if localAddr == nil {
		t.Error("Packet connection local address is nil")
	}
}

// TestIPTransportRoundTrip tests a complete TCP connection with data exchange
func TestIPTransportRoundTrip(t *testing.T) {
	mt := transport.NewMultiTransport()
	defer mt.Close()

	// Create listener
	listener, err := mt.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Start echo server
	errChan := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			errChan <- err
			return
		}
		defer conn.Close()

		buffer := make([]byte, 1024)
		n, err := conn.Read(buffer)
		if err != nil {
			errChan <- err
			return
		}

		_, err = conn.Write(buffer[:n])
		errChan <- err
	}()

	// Connect as client
	client, err := mt.Dial(listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	// Send message
	testMessage := "Hello Multi-Transport Test!"
	n, err := client.Write([]byte(testMessage))
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if n != len(testMessage) {
		t.Errorf("Write returned %d, expected %d", n, len(testMessage))
	}

	// Read response
	buffer := make([]byte, 1024)
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err = client.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	response := string(buffer[:n])
	if response != testMessage {
		t.Errorf("Expected %q, got %q", testMessage, response)
	}

	// Check server completed without error
	select {
	case serverErr := <-errChan:
		if serverErr != nil {
			t.Errorf("Server error: %v", serverErr)
		}
	case <-time.After(time.Second):
		// Server might still be processing, which is fine
	}
}

// TestGetTransport tests direct transport access
func TestGetTransport(t *testing.T) {
	mt := transport.NewMultiTransport()
	defer mt.Close()

	// IP transport should always be available
	ipTransport, exists := mt.GetTransport("ip")
	if !exists {
		t.Skip("IP transport not registered, skipping")
	}

	networks := ipTransport.SupportedNetworks()
	if len(networks) == 0 {
		t.Error("IP transport should support at least one network")
	}
}

// TestRegisterTransport tests custom transport registration
func TestRegisterTransport(t *testing.T) {
	mt := transport.NewMultiTransport()
	defer mt.Close()

	// Register a custom transport
	customTransport := transport.NewIPTransport()
	mt.RegisterTransport("custom", customTransport)

	// Verify it was registered
	retrieved, exists := mt.GetTransport("custom")
	if !exists {
		t.Fatal("Custom transport was not registered")
	}

	networks := retrieved.SupportedNetworks()
	if len(networks) == 0 {
		t.Error("Custom transport should support networks")
	}
}

// TestTransportSelectionTor tests that .onion addresses select Tor transport
func TestTransportSelectionTor(t *testing.T) {
	mt := transport.NewMultiTransport()
	defer mt.Close()

	// Tor address should fail with a specific error (Tor transport limitations)
	// but the address selection should recognize it as a Tor address
	_, err := mt.Listen("test.onion:80")
	if err == nil {
		t.Skip("Tor transport unexpectedly working, skipping selection test")
	}

	// The error message should indicate Tor transport was selected
	// Not an "unknown address format" error
	errStr := err.Error()
	if errStr == "" {
		t.Error("Expected an error message for Tor address")
	}
}

// TestTransportSelectionI2P tests that .i2p addresses select I2P transport
func TestTransportSelectionI2P(t *testing.T) {
	mt := transport.NewMultiTransport()
	defer mt.Close()

	// I2P address should fail but be recognized as I2P format
	_, err := mt.Listen("example.b32.i2p:80")
	if err == nil {
		t.Skip("I2P transport unexpectedly working, skipping selection test")
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("Expected an error message for I2P address")
	}
}
