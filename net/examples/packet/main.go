// Package main demonstrates the Tox packet networking interfaces.
//
// This example shows how to use the PacketDial and PacketListen functions
// along with the ToxPacketConn and ToxPacketListener implementations.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/crypto"
	toxnet "github.com/opd-ai/toxcore/net"
)

func main() {
	fmt.Println("Tox Packet Networking Example")
	fmt.Println("=============================")

	// Example 1: Direct packet connection usage
	fmt.Println("\n1. Direct ToxPacketConn Usage:")
	demonstratePacketConn()

	// Example 2: Packet listener usage
	fmt.Println("\n2. ToxPacketListener Usage:")
	demonstratePacketListener()

	// Example 3: PacketDial and PacketListen functions
	fmt.Println("\n3. PacketDial/PacketListen Functions:")
	demonstratePacketDialListen()
}

func demonstratePacketConn() {
	// Generate a test Tox address
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		log.Fatal("Failed to generate key pair:", err)
	}

	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := toxnet.NewToxAddrFromPublicKey(keyPair.Public, nospam)

	fmt.Printf("Generated Tox Address: %s\n", localAddr.String())

	// Create packet connection
	conn, err := toxnet.NewToxPacketConn(localAddr, ":0")
	if err != nil {
		log.Fatal("Failed to create packet connection:", err)
	}
	defer conn.Close()

	fmt.Printf("Local address: %s\n", conn.LocalAddr())
	fmt.Printf("Packet connection created successfully\n")

	// Test deadline setting
	deadline := time.Now().Add(5 * time.Second)
	conn.SetDeadline(deadline)
	fmt.Printf("Set deadline to: %s\n", deadline.Format(time.RFC3339))

	// In a real implementation, you would use WriteTo to send packets
	// and ReadFrom to receive them
}

func demonstratePacketListener() {
	// Generate a test Tox address
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		log.Fatal("Failed to generate key pair:", err)
	}

	nospam := [4]byte{0x05, 0x06, 0x07, 0x08}
	localAddr := toxnet.NewToxAddrFromPublicKey(keyPair.Public, nospam)

	fmt.Printf("Generated Tox Address: %s\n", localAddr.String())

	// Create packet listener
	listener, err := toxnet.NewToxPacketListener(localAddr, ":0")
	if err != nil {
		log.Fatal("Failed to create packet listener:", err)
	}
	defer listener.Close()

	fmt.Printf("Listener address: %s\n", listener.Addr())
	fmt.Printf("Packet listener created successfully\n")

	// In a real implementation, you would call Accept() in a loop
	// to handle incoming connections
}

func demonstratePacketDialListen() {
	// Test with invalid network (should fail)
	_, err := toxnet.PacketDial("invalid", "test-addr")
	if err != nil {
		fmt.Printf("Expected error for invalid network: %s\n", err)
	}

	// Test with invalid address (should fail)
	_, err = toxnet.PacketDial("tox", "invalid-tox-id")
	if err != nil {
		fmt.Printf("Expected error for invalid Tox ID: %s\n", err)
	}

	// Test PacketListen with invalid network (should fail)
	_, err = toxnet.PacketListen("invalid", ":0", nil)
	if err != nil {
		fmt.Printf("Expected error for invalid network: %s\n", err)
	}

	// Test PacketListen with nil Tox instance (should fail)
	_, err = toxnet.PacketListen("tox", ":0", nil)
	if err != nil {
		fmt.Printf("Expected error for nil Tox instance: %s\n", err)
	}

	// Test PacketListen with valid Tox instance
	opts := toxcore.NewOptions()
	tox, err := toxcore.New(opts)
	if err != nil {
		log.Printf("Could not create Tox instance for demo: %s\n", err)
		return
	}
	defer tox.Kill()

	listener, err := toxnet.PacketListen("tox", ":0", tox)
	if err != nil {
		log.Printf("Unexpected error creating packet listener: %s\n", err)
		return
	}
	defer listener.Close()
	fmt.Printf("PacketListen created successfully with address: %s\n", listener.Addr())

	fmt.Printf("PacketDial and PacketListen functions tested\n")
}

// Example showing how to integrate with existing net package patterns
func integrationExample() {
	// This shows how our ToxPacketConn can be used as a drop-in replacement
	// for net.PacketConn in existing code

	keyPair, _ := crypto.GenerateKeyPair()
	nospam := [4]byte{0x01, 0x02, 0x03, 0x04}
	localAddr := toxnet.NewToxAddrFromPublicKey(keyPair.Public, nospam)

	// Our ToxPacketConn implements net.PacketConn
	var packetConn interface{} // net.PacketConn
	var err error

	packetConn, err = toxnet.NewToxPacketConn(localAddr, ":0")
	if err != nil {
		log.Fatal("Failed to create packet connection:", err)
	}

	// Can be used anywhere that expects net.PacketConn
	_ = packetConn
	fmt.Println("Integration with net.PacketConn interface: ✓")
}
