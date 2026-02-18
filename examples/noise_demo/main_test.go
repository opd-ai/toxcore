package main

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// TestGenerateNodeKeyPairs verifies key pair generation produces valid keys.
func TestGenerateNodeKeyPairs(t *testing.T) {
	keyPair1, keyPair2, err := generateNodeKeyPairs()
	if err != nil {
		t.Fatalf("generateNodeKeyPairs() error: %v", err)
	}

	// Verify keys are not nil
	if keyPair1 == nil || keyPair2 == nil {
		t.Fatal("generateNodeKeyPairs() returned nil key pair")
	}

	// Verify keys are different
	if keyPair1.Public == keyPair2.Public {
		t.Error("generateNodeKeyPairs() returned identical public keys")
	}

	// Verify keys have expected length (32 bytes for Curve25519)
	if len(keyPair1.Public) != 32 || len(keyPair2.Public) != 32 {
		t.Errorf("unexpected public key length: got %d and %d, want 32", len(keyPair1.Public), len(keyPair2.Public))
	}
}

// TestSetupUDPTransports verifies UDP transport creation.
func TestSetupUDPTransports(t *testing.T) {
	udp1, udp2, err := setupUDPTransports()
	if err != nil {
		t.Fatalf("setupUDPTransports() error: %v", err)
	}
	defer udp1.Close()
	defer udp2.Close()

	// Verify transports have different addresses
	addr1 := udp1.LocalAddr()
	addr2 := udp2.LocalAddr()

	if addr1 == nil || addr2 == nil {
		t.Fatal("setupUDPTransports() returned transport with nil address")
	}

	if addr1.String() == addr2.String() {
		t.Error("setupUDPTransports() returned transports with same address")
	}
}

// TestSetupNoiseTransports verifies Noise transport wrapping and address validation.
func TestSetupNoiseTransports(t *testing.T) {
	keyPair1, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}
	keyPair2, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	udp1, udp2, err := setupUDPTransports()
	if err != nil {
		t.Fatalf("setupUDPTransports() error: %v", err)
	}
	defer udp1.Close()
	defer udp2.Close()

	noise1, noise2, addr1, addr2, err := setupNoiseTransports(udp1, udp2, keyPair1, keyPair2)
	if err != nil {
		t.Fatalf("setupNoiseTransports() error: %v", err)
	}
	defer noise1.Close()
	defer noise2.Close()

	// Verify addresses are non-nil (validated by function)
	if addr1 == nil || addr2 == nil {
		t.Fatal("setupNoiseTransports() returned nil addresses")
	}

	// Verify transports are functional
	if noise1.LocalAddr() == nil || noise2.LocalAddr() == nil {
		t.Error("Noise transports returned nil local addresses")
	}
}

// TestConfigurePeers verifies peer configuration between two Noise transports.
func TestConfigurePeers(t *testing.T) {
	keyPair1, _ := crypto.GenerateKeyPair()
	keyPair2, _ := crypto.GenerateKeyPair()
	udp1, udp2, _ := setupUDPTransports()
	defer udp1.Close()
	defer udp2.Close()

	noise1, noise2, addr1, addr2, _ := setupNoiseTransports(udp1, udp2, keyPair1, keyPair2)
	defer noise1.Close()
	defer noise2.Close()

	err := configurePeers(noise1, noise2, addr1, addr2, keyPair1, keyPair2)
	if err != nil {
		t.Fatalf("configurePeers() error: %v", err)
	}
}

// TestNoiseMessageExchange performs an integration test of the Noise-IK handshake
// and encrypted message exchange between two nodes.
// Note: The first message exchange triggers the handshake, so a retry pattern is used.
func TestNoiseMessageExchange(t *testing.T) {
	// Setup key pairs
	keyPair1, keyPair2, err := generateNodeKeyPairs()
	if err != nil {
		t.Fatalf("generateNodeKeyPairs() error: %v", err)
	}

	// Setup UDP transports
	udp1, udp2, err := setupUDPTransports()
	if err != nil {
		t.Fatalf("setupUDPTransports() error: %v", err)
	}
	defer udp1.Close()
	defer udp2.Close()

	// Setup Noise transports
	noise1, noise2, addr1, addr2, err := setupNoiseTransports(udp1, udp2, keyPair1, keyPair2)
	if err != nil {
		t.Fatalf("setupNoiseTransports() error: %v", err)
	}
	defer noise1.Close()
	defer noise2.Close()

	// Configure peers
	err = configurePeers(noise1, noise2, addr1, addr2, keyPair1, keyPair2)
	if err != nil {
		t.Fatalf("configurePeers() error: %v", err)
	}

	// Setup message handlers
	messageReceived := setupMessageHandlers(noise1, noise2)

	// The Noise handshake requires initial packet exchange before the channel is ready.
	// Send initial messages to trigger handshake, then verify communication works.
	// The first message may be lost during handshake establishment - this is expected.

	// Send a preliminary message to trigger handshake (may fail)
	_ = sendAndVerifyMessageWithTimeout(noise1, addr2, "handshake-trigger", messageReceived, 500*time.Millisecond)

	// Allow time for handshake to complete
	time.Sleep(200 * time.Millisecond)

	// Now the channel should be established. Test bidirectional communication.
	// Message from node 2 to node 1 (reverse direction after handshake)
	replyMessage := "Reply from Node 2"
	err = sendAndVerifyMessageWithTimeout(noise2, addr1, replyMessage, messageReceived, 2*time.Second)
	if err != nil {
		t.Errorf("sendAndVerifyMessage() from node 2: %v", err)
	}

	// Message from node 1 to node 2 (should work now)
	testMessage := "Test message from Node 1"
	err = sendAndVerifyMessageWithTimeout(noise1, addr2, testMessage, messageReceived, 2*time.Second)
	if err != nil {
		t.Errorf("sendAndVerifyMessage() from node 1: %v", err)
	}
}

// TestSendAndVerifyMessageTimeout verifies timeout behavior.
func TestSendAndVerifyMessageTimeout(t *testing.T) {
	keyPair1, _ := crypto.GenerateKeyPair()
	keyPair2, _ := crypto.GenerateKeyPair()
	udp1, udp2, _ := setupUDPTransports()
	defer udp1.Close()
	defer udp2.Close()

	noise1, noise2, _, addr2, _ := setupNoiseTransports(udp1, udp2, keyPair1, keyPair2)
	defer noise1.Close()
	defer noise2.Close()

	// Don't set up message handlers - messages won't be received
	// Use a non-receiving channel
	messageReceived := make(chan string, 1)

	// Short timeout for test speed
	err := sendAndVerifyMessageWithTimeout(noise1, addr2, "Test", messageReceived, 100*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
	if !errors.Is(err, ErrMessageTimeout) {
		t.Errorf("expected ErrMessageTimeout, got: %v", err)
	}
}

// TestSendAndVerifyMessageMismatch verifies message mismatch detection.
func TestSendAndVerifyMessageMismatch(t *testing.T) {
	keyPair1, _ := crypto.GenerateKeyPair()
	keyPair2, _ := crypto.GenerateKeyPair()
	udp1, udp2, _ := setupUDPTransports()
	defer udp1.Close()
	defer udp2.Close()

	noise1, noise2, _, addr2, _ := setupNoiseTransports(udp1, udp2, keyPair1, keyPair2)
	defer noise1.Close()
	defer noise2.Close()

	// Pre-fill channel with wrong message
	messageReceived := make(chan string, 1)
	messageReceived <- "wrong message"

	err := sendAndVerifyMessageWithTimeout(noise1, addr2, "expected message", messageReceived, 100*time.Millisecond)
	if err == nil {
		t.Error("expected mismatch error, got nil")
	}
	if !errors.Is(err, ErrMessageMismatch) {
		t.Errorf("expected ErrMessageMismatch, got: %v", err)
	}
}

// mockTransport is a minimal mock for testing nil address handling.
type mockTransport struct {
	localAddr net.Addr
}

func (m *mockTransport) LocalAddr() net.Addr                                           { return m.localAddr }
func (m *mockTransport) Send(*transport.Packet, net.Addr) error                        { return nil }
func (m *mockTransport) Close() error                                                  { return nil }
func (m *mockTransport) RegisterHandler(transport.PacketType, transport.PacketHandler) {}

// TestSetupMessageHandlers verifies message handler setup returns a valid channel.
func TestSetupMessageHandlers(t *testing.T) {
	keyPair1, _ := crypto.GenerateKeyPair()
	keyPair2, _ := crypto.GenerateKeyPair()
	udp1, udp2, _ := setupUDPTransports()
	defer udp1.Close()
	defer udp2.Close()

	noise1, noise2, _, _, _ := setupNoiseTransports(udp1, udp2, keyPair1, keyPair2)
	defer noise1.Close()
	defer noise2.Close()

	messageReceived := setupMessageHandlers(noise1, noise2)
	if messageReceived == nil {
		t.Fatal("setupMessageHandlers() returned nil channel")
	}

	// Verify channel is buffered
	select {
	case messageReceived <- "test":
		// Expected - channel has capacity
	default:
		t.Error("setupMessageHandlers() channel should have buffer capacity")
	}
}

// TestPrintDemoSummary verifies the demo summary prints without error.
// This test primarily exists to improve coverage.
func TestPrintDemoSummary(t *testing.T) {
	// Just call it to verify no panic
	printDemoSummary()
}

// TestSendAndVerifyMessageDefaultTimeout verifies the default timeout wrapper works.
func TestSendAndVerifyMessageDefaultTimeout(t *testing.T) {
	keyPair1, _ := crypto.GenerateKeyPair()
	keyPair2, _ := crypto.GenerateKeyPair()
	udp1, udp2, _ := setupUDPTransports()
	defer udp1.Close()
	defer udp2.Close()

	noise1, noise2, _, addr2, _ := setupNoiseTransports(udp1, udp2, keyPair1, keyPair2)
	defer noise1.Close()
	defer noise2.Close()

	// Pre-fill channel with expected message
	messageReceived := make(chan string, 1)
	messageReceived <- "test message"

	// sendAndVerifyMessage uses DefaultMessageTimeout
	err := sendAndVerifyMessage(noise1, addr2, "test message", messageReceived)
	if err != nil {
		t.Errorf("sendAndVerifyMessage() error: %v", err)
	}
}
