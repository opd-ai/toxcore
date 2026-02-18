package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/opd-ai/toxcore"
	toxnet "github.com/opd-ai/toxcore/net"
)

func main() {
	fmt.Println("=== Tox Networking Example ===")

	// Example 1: Basic Echo Server
	serverTox := setupEchoServer()
	defer serverTox.Kill()

	// Example 2: Client Connection
	demonstrateClientConnection(serverTox)

	// Example 3: Address Operations
	demonstrateAddressOperations(serverTox)
}

// setupEchoServer creates and starts a Tox echo server, returning the server instance.
func setupEchoServer() *toxcore.Tox {
	// Create a Tox instance for the server
	serverOptions := toxcore.NewOptions()
	serverTox, err := toxcore.New(serverOptions)
	if err != nil {
		log.Fatalf("Failed to create server Tox instance: %v", err)
	}

	// Start the server
	fmt.Printf("Server Tox ID: %s\n", serverTox.SelfGetAddress())

	// Create a listener
	listener, err := toxnet.Listen(serverTox)
	if err != nil {
		log.Fatalf("Failed to create listener: %v", err)
	}

	fmt.Printf("Server listening on: %s\n", listener.Addr())

	// Start server goroutine
	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Accept error: %v", err)
				return
			}
			go handleConnection(conn)
		}
	}()

	return serverTox
}

// demonstrateClientConnection shows how to create a client and attempt to connect to a server.
func demonstrateClientConnection(serverTox *toxcore.Tox) {
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
		performClientCommunication(conn)
	}
}

// performClientCommunication handles sending data and reading responses from the server.
func performClientCommunication(conn io.ReadWriteCloser) {
	// Send some data
	_, err := conn.Write([]byte("Hello from Tox networking!"))
	if err != nil {
		log.Printf("Write error: %v", err)
		return
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

// demonstrateAddressOperations shows Tox address parsing and validation capabilities.
func demonstrateAddressOperations(serverTox *toxcore.Tox) {
	fmt.Println("\n=== Address Operations ===")

	// Parse a Tox address
	toxID := serverTox.SelfGetAddress()
	parseAndDisplayAddress(toxID)

	// Validate addresses
	validateSampleAddresses(toxID)
}

// parseAndDisplayAddress parses a Tox ID and displays its components.
func parseAndDisplayAddress(toxID string) {
	addr, err := toxnet.NewToxAddr(toxID)
	if err != nil {
		log.Printf("Address parse error: %v", err)
		return
	}

	fmt.Printf("Network: %s\n", addr.Network())
	fmt.Printf("Address: %s\n", addr.String())
	fmt.Printf("Public Key: %x\n", addr.PublicKey())
	fmt.Printf("Nospam: %x\n", addr.Nospam())
}

// validateSampleAddresses demonstrates address validation with valid and invalid examples.
func validateSampleAddresses(validToxID string) {
	validAddresses := []string{
		validToxID,
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
// Uses net.Conn interface to access RemoteAddr without type assertions
func handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Printf("New connection from: %s\n", conn.RemoteAddr())

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
