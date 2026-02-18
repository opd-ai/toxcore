package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/opd-ai/toxcore"
	toxnet "github.com/opd-ai/toxcore/net"
	"github.com/sirupsen/logrus"
)

func main() {
	fmt.Println("=== Tox Networking Example ===")

	// Example 1: Basic Echo Server
	serverTox, err := setupEchoServer()
	if err != nil {
		logrus.WithError(err).Error("Failed to setup echo server")
		os.Exit(1)
	}
	defer serverTox.Kill()

	// Example 2: Client Connection
	demonstrateClientConnection(serverTox)

	// Example 3: Address Operations
	demonstrateAddressOperations(serverTox)
}

// setupEchoServer creates and starts a Tox echo server, returning the server instance.
// Returns an error instead of exiting on failure to demonstrate proper error propagation.
func setupEchoServer() (*toxcore.Tox, error) {
	// Create a Tox instance for the server
	serverOptions := toxcore.NewOptions()
	serverTox, err := toxcore.New(serverOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create server Tox instance: %w", err)
	}

	// Start the server
	fmt.Printf("Server Tox ID: %s\n", serverTox.SelfGetAddress())

	// Create a listener
	listener, err := toxnet.Listen(serverTox)
	if err != nil {
		serverTox.Kill() // Cleanup on failure
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	fmt.Printf("Server listening on: %s\n", listener.Addr())

	// Start server goroutine
	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				// Check if error is temporary and we should retry
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Temporary() {
					logrus.WithError(err).Warn("Temporary accept error, retrying")
					continue
				}
				// Permanent error or listener closed - stop accepting
				logrus.WithError(err).Debug("Listener closed or permanent error")
				return
			}
			go handleConnection(conn)
		}
	}()

	return serverTox, nil
}

// demonstrateClientConnection shows how to create a client and attempt to connect to a server.
// Demonstrates proper error propagation instead of using log.Fatal.
func demonstrateClientConnection(serverTox *toxcore.Tox) {
	fmt.Println("\n=== Client Example ===")

	// Create a Tox instance for the client
	clientOptions := toxcore.NewOptions()
	clientTox, err := toxcore.New(clientOptions)
	if err != nil {
		logrus.WithError(err).Error("Failed to create client Tox instance")
		return
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
		logrus.WithError(err).Error("Write error")
		return
	}

	// Read response
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		logrus.WithError(err).Error("Read error")
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
		logrus.WithError(err).Error("Address parse error")
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
				logrus.WithError(err).Error("Read error")
			}
			break
		}

		fmt.Printf("Received: %s\n", string(buffer[:n]))

		// Echo back
		_, err = conn.Write(buffer[:n])
		if err != nil {
			logrus.WithError(err).Error("Write error")
			break
		}
	}
}
