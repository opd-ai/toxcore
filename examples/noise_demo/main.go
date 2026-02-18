// Package main demonstrates the Noise-IK transport integration.
//
// This example shows how to use NoiseTransport to establish encrypted
// communication between two Tox nodes using the Noise Protocol Framework.
package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// DefaultMessageTimeout is the default timeout for message delivery verification.
// This can be overridden for testing by calling sendAndVerifyMessageWithTimeout.
const DefaultMessageTimeout = 5 * time.Second

// ErrMessageTimeout is returned when a message is not received within the timeout period.
var ErrMessageTimeout = errors.New("timeout waiting for message")

// ErrMessageMismatch is returned when the received message doesn't match the sent message.
var ErrMessageMismatch = errors.New("message content mismatch")

// generateNodeKeyPairs creates key pairs for both demo nodes and displays their public keys.
func generateNodeKeyPairs() (*crypto.KeyPair, *crypto.KeyPair, error) {
	keyPair1, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair 1: %w", err)
	}

	keyPair2, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair 2: %w", err)
	}

	fmt.Printf("Node 1 public key: %x\n", keyPair1.Public[:8])
	fmt.Printf("Node 2 public key: %x\n", keyPair2.Public[:8])

	return keyPair1, keyPair2, nil
}

// setupUDPTransports creates UDP transport layers for both nodes.
func setupUDPTransports() (transport.Transport, transport.Transport, error) {
	udpTransport1, err := transport.NewUDPTransport("127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create UDP transport 1: %w", err)
	}

	udpTransport2, err := transport.NewUDPTransport("127.0.0.1:0")
	if err != nil {
		udpTransport1.Close()
		return nil, nil, fmt.Errorf("failed to create UDP transport 2: %w", err)
	}

	return udpTransport1, udpTransport2, nil
}

// setupNoiseTransports wraps UDP transports with Noise encryption and displays addresses.
// Returns an error if either transport fails to create or if local addresses are nil.
func setupNoiseTransports(udpTransport1, udpTransport2 transport.Transport, keyPair1, keyPair2 *crypto.KeyPair) (*transport.NoiseTransport, *transport.NoiseTransport, net.Addr, net.Addr, error) {
	noiseTransport1, err := transport.NewNoiseTransport(udpTransport1, keyPair1.Private[:])
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create Noise transport 1: %w", err)
	}

	noiseTransport2, err := transport.NewNoiseTransport(udpTransport2, keyPair2.Private[:])
	if err != nil {
		noiseTransport1.Close()
		return nil, nil, nil, nil, fmt.Errorf("failed to create Noise transport 2: %w", err)
	}

	addr1 := noiseTransport1.LocalAddr()
	addr2 := noiseTransport2.LocalAddr()

	// Validate addresses are available before proceeding
	if addr1 == nil {
		noiseTransport1.Close()
		noiseTransport2.Close()
		return nil, nil, nil, nil, fmt.Errorf("transport 1 returned nil local address")
	}
	if addr2 == nil {
		noiseTransport1.Close()
		noiseTransport2.Close()
		return nil, nil, nil, nil, fmt.Errorf("transport 2 returned nil local address")
	}

	fmt.Printf("Node 1 listening on: %s\n", addr1)
	fmt.Printf("Node 2 listening on: %s\n", addr2)

	return noiseTransport1, noiseTransport2, addr1, addr2, nil
}

// configurePeers adds each node as a peer to the other.
func configurePeers(noiseTransport1, noiseTransport2 *transport.NoiseTransport, addr1, addr2 net.Addr, keyPair1, keyPair2 *crypto.KeyPair) error {
	err := noiseTransport1.AddPeer(addr2, keyPair2.Public[:])
	if err != nil {
		return fmt.Errorf("failed to add peer to transport 1: %w", err)
	}

	err = noiseTransport2.AddPeer(addr1, keyPair1.Public[:])
	if err != nil {
		return fmt.Errorf("failed to add peer to transport 2: %w", err)
	}

	fmt.Println("Peers added successfully")
	return nil
}

// setupMessageHandlers configures message handlers for both transports.
func setupMessageHandlers(noiseTransport1, noiseTransport2 *transport.NoiseTransport) chan string {
	messageReceived := make(chan string, 2)

	noiseTransport1.RegisterHandler(transport.PacketFriendMessage, func(packet *transport.Packet, addr net.Addr) error {
		message := string(packet.Data)
		fmt.Printf("Node 1 received: '%s' from %s\n", message, addr)
		messageReceived <- message
		return nil
	})

	noiseTransport2.RegisterHandler(transport.PacketFriendMessage, func(packet *transport.Packet, addr net.Addr) error {
		message := string(packet.Data)
		fmt.Printf("Node 2 received: '%s' from %s\n", message, addr)
		messageReceived <- message
		return nil
	})

	return messageReceived
}

// sendAndVerifyMessage sends a message and waits for confirmation of receipt using default timeout.
func sendAndVerifyMessage(sender *transport.NoiseTransport, targetAddr net.Addr, message string, messageReceived chan string) error {
	return sendAndVerifyMessageWithTimeout(sender, targetAddr, message, messageReceived, DefaultMessageTimeout)
}

// sendAndVerifyMessageWithTimeout sends a message and waits for confirmation of receipt.
// Returns ErrMessageTimeout if no message is received within the timeout period,
// or ErrMessageMismatch if the received message doesn't match the sent message.
func sendAndVerifyMessageWithTimeout(sender *transport.NoiseTransport, targetAddr net.Addr, message string, messageReceived chan string, timeout time.Duration) error {
	packet := &transport.Packet{
		PacketType: transport.PacketFriendMessage,
		Data:       []byte(message),
	}

	fmt.Printf("Sending message: '%s'\n", message)
	err := sender.Send(packet, targetAddr)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	select {
	case received := <-messageReceived:
		if received == message {
			fmt.Println("✅ Message transmitted successfully!")
			return nil
		}
		fmt.Printf("❌ Message mismatch: expected '%s', got '%s'\n", message, received)
		return fmt.Errorf("%w: expected '%s', got '%s'", ErrMessageMismatch, message, received)
	case <-time.After(timeout):
		fmt.Println("❌ Timeout waiting for message")
		return ErrMessageTimeout
	}
}

// printDemoSummary displays the completion message and feature summary.
func printDemoSummary() {
	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("Key features demonstrated:")
	fmt.Println("• Noise-IK handshake establishment")
	fmt.Println("• Automatic peer discovery and key exchange")
	fmt.Println("• Encrypted message transmission")
	fmt.Println("• Bidirectional communication")
	fmt.Println("• Forward secrecy and KCI resistance")
}

func main() {
	fmt.Println("=== Noise-IK Transport Integration Demo ===")

	// Generate key pairs for both nodes
	keyPair1, keyPair2, err := generateNodeKeyPairs()
	if err != nil {
		log.Fatal(err)
	}

	// Create UDP transports
	udpTransport1, udpTransport2, err := setupUDPTransports()
	if err != nil {
		log.Fatal(err)
	}
	defer udpTransport1.Close()
	defer udpTransport2.Close()

	// Wrap with Noise transports
	noiseTransport1, noiseTransport2, addr1, addr2, err := setupNoiseTransports(udpTransport1, udpTransport2, keyPair1, keyPair2)
	if err != nil {
		log.Fatal(err)
	}
	defer noiseTransport1.Close()
	defer noiseTransport2.Close()

	// Configure peers
	err = configurePeers(noiseTransport1, noiseTransport2, addr1, addr2, keyPair1, keyPair2)
	if err != nil {
		log.Fatal(err)
	}

	// Set up message handlers
	messageReceived := setupMessageHandlers(noiseTransport1, noiseTransport2)

	// Send test message from node 1 to node 2
	err = sendAndVerifyMessage(noiseTransport1, addr2, "Hello from Node 1!", messageReceived)
	if err != nil {
		log.Fatal(err)
	}

	// Send reply from node 2 to node 1
	err = sendAndVerifyMessage(noiseTransport2, addr1, "Hello back from Node 2!", messageReceived)
	if err != nil {
		log.Fatal(err)
	}

	printDemoSummary()
}
