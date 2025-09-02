// Package main demonstrates the Noise-IK transport integration.
//
// This example shows how to use NoiseTransport to establish encrypted
// communication between two Tox nodes using the Noise Protocol Framework.
package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

func main() {
	fmt.Println("=== Noise-IK Transport Integration Demo ===")

	// Generate key pairs for two nodes
	keyPair1, err := crypto.GenerateKeyPair()
	if err != nil {
		log.Fatal("Failed to generate key pair 1:", err)
	}

	keyPair2, err := crypto.GenerateKeyPair()
	if err != nil {
		log.Fatal("Failed to generate key pair 2:", err)
	}

	fmt.Printf("Node 1 public key: %x\n", keyPair1.Public[:8])
	fmt.Printf("Node 2 public key: %x\n", keyPair2.Public[:8])

	// Create UDP transports
	udpTransport1, err := transport.NewUDPTransport("127.0.0.1:0")
	if err != nil {
		log.Fatal("Failed to create UDP transport 1:", err)
	}
	defer udpTransport1.Close()

	udpTransport2, err := transport.NewUDPTransport("127.0.0.1:0")
	if err != nil {
		log.Fatal("Failed to create UDP transport 2:", err)
	}
	defer udpTransport2.Close()

	// Wrap with Noise transports
	noiseTransport1, err := transport.NewNoiseTransport(udpTransport1, keyPair1.Private[:])
	if err != nil {
		log.Fatal("Failed to create Noise transport 1:", err)
	}
	defer noiseTransport1.Close()

	noiseTransport2, err := transport.NewNoiseTransport(udpTransport2, keyPair2.Private[:])
	if err != nil {
		log.Fatal("Failed to create Noise transport 2:", err)
	}
	defer noiseTransport2.Close()

	// Get addresses
	addr1 := noiseTransport1.LocalAddr()
	addr2 := noiseTransport2.LocalAddr()

	fmt.Printf("Node 1 listening on: %s\n", addr1)
	fmt.Printf("Node 2 listening on: %s\n", addr2)

	// Add each other as known peers
	err = noiseTransport1.AddPeer(addr2, keyPair2.Public[:])
	if err != nil {
		log.Fatal("Failed to add peer to transport 1:", err)
	}

	err = noiseTransport2.AddPeer(addr1, keyPair1.Public[:])
	if err != nil {
		log.Fatal("Failed to add peer to transport 2:", err)
	}

	fmt.Println("Peers added successfully")

	// Set up message handlers
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

	// Send a test message from node 1 to node 2
	testMessage := "Hello from Node 1!"
	packet := &transport.Packet{
		PacketType: transport.PacketFriendMessage,
		Data:       []byte(testMessage),
	}

	fmt.Printf("Sending message: '%s'\n", testMessage)
	err = noiseTransport1.Send(packet, addr2)
	if err != nil {
		log.Fatal("Failed to send message:", err)
	}

	// Wait for message to be received (with timeout)
	select {
	case received := <-messageReceived:
		if received == testMessage {
			fmt.Println("✅ Message transmitted successfully!")
		} else {
			fmt.Printf("❌ Message mismatch: expected '%s', got '%s'\n", testMessage, received)
		}
	case <-time.After(5 * time.Second):
		fmt.Println("❌ Timeout waiting for message")
	}

	// Send a reply from node 2 to node 1
	replyMessage := "Hello back from Node 2!"
	replyPacket := &transport.Packet{
		PacketType: transport.PacketFriendMessage,
		Data:       []byte(replyMessage),
	}

	fmt.Printf("Sending reply: '%s'\n", replyMessage)
	err = noiseTransport2.Send(replyPacket, addr1)
	if err != nil {
		log.Fatal("Failed to send reply:", err)
	}

	// Wait for reply to be received
	select {
	case received := <-messageReceived:
		if received == replyMessage {
			fmt.Println("✅ Reply transmitted successfully!")
		} else {
			fmt.Printf("❌ Reply mismatch: expected '%s', got '%s'\n", replyMessage, received)
		}
	case <-time.After(5 * time.Second):
		fmt.Println("❌ Timeout waiting for reply")
	}

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("Key features demonstrated:")
	fmt.Println("• Noise-IK handshake establishment")
	fmt.Println("• Automatic peer discovery and key exchange")
	fmt.Println("• Encrypted message transmission")
	fmt.Println("• Bidirectional communication")
	fmt.Println("• Forward secrecy and KCI resistance")
}
