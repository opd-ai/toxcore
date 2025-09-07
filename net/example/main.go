package main

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/opd-ai/toxcore"
	toxnet "github.com/opd-ai/toxcore/net"
)

func main() {
	// Example 1: Basic Echo Server
	fmt.Println("=== Tox Networking Example ===")

	// Create a Tox instance for the server
	serverOptions := toxcore.NewOptions()
	serverTox, err := toxcore.New(serverOptions)
	if err != nil {
		log.Fatalf("Failed to create server Tox instance: %v", err)
	}
	defer serverTox.Kill()

	// Start the server
	fmt.Printf("Server Tox ID: %s\n", serverTox.SelfGetAddress())

	// Create a listener
	listener, err := toxnet.Listen(serverTox)
	if err != nil {
		log.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	fmt.Printf("Server listening on: %s\n", listener.Addr())

	// Start server goroutine
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Accept error: %v", err)
				return
			}
			go handleConnection(conn)
		}
	}()

	// Example 2: Client Connection
	fmt.Println("\n=== Client Example ===")

	// Create a Tox instance for the client
	clientOptions := toxcore.NewOptions()
	clientTox, err := toxcore.New(clientOptions)
	if err != nil {
		log.Fatalf("Failed to create client Tox instance: %v", err)
	}
	defer clientTox.Kill()

	fmt.Printf("Client Tox ID: %s\n", clientTox.SelfGetAddress())

	// In a real scenario, you would connect to a different Tox ID
	// For this example, we'll demonstrate the API usage
	serverToxID := serverTox.SelfGetAddress()

	fmt.Printf("Attempting to connect to: %s\n", serverToxID)

	// Connect with timeout
	conn, err := toxnet.DialTimeout(serverToxID, clientTox, 10*time.Second)
	if err != nil {
		// This is expected to fail in this example since we can't connect to ourselves
		// In a real scenario, you'd connect to a different Tox instance
		fmt.Printf("Connection failed (expected in this demo): %v\n", err)
	} else {
		defer conn.Close()

		// Send some data
		_, err = conn.Write([]byte("Hello from Tox networking!"))
		if err != nil {
			log.Printf("Write error: %v", err)
		}

		// Read response
		buffer := make([]byte, 1024)
		n, err := conn.Read(buffer)
		if err != nil {
			log.Printf("Read error: %v", err)
		} else {
			fmt.Printf("Received: %s\n", string(buffer[:n]))
		}
	}

	// Example 3: Address Operations
	fmt.Println("\n=== Address Operations ===")

	// Parse a Tox address
	toxID := serverTox.SelfGetAddress()
	addr, err := toxnet.NewToxAddr(toxID)
	if err != nil {
		log.Printf("Address parse error: %v", err)
	} else {
		fmt.Printf("Network: %s\n", addr.Network())
		fmt.Printf("Address: %s\n", addr.String())
		fmt.Printf("Public Key: %x\n", addr.PublicKey())
		fmt.Printf("Nospam: %x\n", addr.Nospam())
	}

	// Validate addresses
	validAddresses := []string{
		toxID,
	}

	invalidAddresses := []string{
		"invalid",
		"too_short",
		"GGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGG",
	}

	fmt.Println("\nValid addresses:")
	for _, addr := range validAddresses {
		if toxnet.IsToxAddr(addr) {
			fmt.Printf("✓ %s\n", addr)
		} else {
			fmt.Printf("✗ %s\n", addr)
		}
	}

	fmt.Println("\nInvalid addresses:")
	for _, addr := range invalidAddresses {
		if toxnet.IsToxAddr(addr) {
			fmt.Printf("✓ %s (unexpected!)\n", addr)
		} else {
			fmt.Printf("✗ %s (expected)\n", addr)
		}
	}
}

// handleConnection demonstrates how to handle incoming connections
func handleConnection(conn io.ReadWriteCloser) {
	defer conn.Close()

	fmt.Printf("New connection from: %s\n", conn.(*toxnet.ToxConn).RemoteAddr())

	// Echo server - copy all received data back
	buffer := make([]byte, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("Read error: %v", err)
			}
			break
		}

		fmt.Printf("Received: %s\n", string(buffer[:n]))

		// Echo back
		_, err = conn.Write(buffer[:n])
		if err != nil {
			log.Printf("Write error: %v", err)
			break
		}
	}
}
