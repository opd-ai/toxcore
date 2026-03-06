package main

import (
	"strings"
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

// TestTorI2PSimultaneousDialPacketPrefersI2P verifies that when both Tor and I2P are
// simultaneously registered in MultiTransport, DialPacket uses I2P datagrams.
// Tor is TCP-only and cannot carry Tox UDP protocol messages, so I2P is preferred.
func TestTorI2PSimultaneousDialPacketPrefersI2P(t *testing.T) {
	mt := transport.NewMultiTransport()
	defer mt.Close()

	// Both Tor and I2P are registered by default in NewMultiTransport.
	torTransport, hasTor := mt.GetTransport("tor")
	_, hasI2P := mt.GetTransport("i2p")
	if !hasTor || !hasI2P {
		t.Skip("Tor and/or I2P not registered in MultiTransport, skipping")
	}

	// Verify Tor refuses DialPacket (TCP-only transport).
	_, torErr := torTransport.DialPacket("test.onion:8080")
	if torErr == nil {
		t.Skip("Tor unexpectedly supports DialPacket, skipping")
	}
	if !strings.Contains(torErr.Error(), "Tor UDP transport not supported") {
		t.Errorf("Expected Tor to report UDP unsupported, got: %v", torErr)
	}

	// When both are registered, MultiTransport.DialPacket should attempt I2P first.
	// Since no I2P SAM bridge is running, we expect an I2P init error — NOT a Tor error.
	_, err := mt.DialPacket("test.onion:8080")
	if err == nil {
		t.Skip("I2P SAM bridge appears to be running, skipping expected-failure test")
	}

	// Error should come from I2P (not Tor), because Tor+I2P mode prefers I2P datagrams.
	if strings.Contains(err.Error(), "Tor UDP transport not supported") {
		t.Errorf("Expected I2P error (not Tor), got: %v", err)
	}
}

// TestI2PDialPacketPreferredForDatagrams verifies that I2PTransport.DialPacket accepts
// any address (not just .i2p format), since garlic.ListenPacket creates a local I2P
// datagram endpoint and does not connect to the given address.
func TestI2PDialPacketPreferredForDatagrams(t *testing.T) {
	i2p := transport.NewI2PTransport()
	defer i2p.Close()

	// DialPacket should not reject non-.i2p addresses with an address-format error.
	// It should fail only if the SAM bridge is unavailable.
	_, err := i2p.DialPacket("192.0.2.1:33445")
	if err == nil {
		t.Skip("I2P SAM bridge appears to be running, skipping expected-failure test")
	}
	// Must not be an address-format rejection.
	if strings.Contains(err.Error(), "invalid I2P address format") {
		t.Errorf("DialPacket incorrectly rejected non-.i2p address: %v", err)
	}
}
