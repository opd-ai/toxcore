package main

import (
	"fmt"
	"time"

	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// demonstrateMultiTransport shows how to use the new multi-transport system
// to handle different network types through a unified interface.
func main() {
	fmt.Println("=== Multi-Transport Demo ===")
	fmt.Println("Demonstrating Phase 4.1: Multi-Protocol Transport Layer")
	fmt.Println()

	// Create a new multi-transport instance
	mt := transport.NewMultiTransport()
	defer mt.Close()

	// Display supported networks
	fmt.Println("Supported Networks:")
	networks := mt.GetSupportedNetworks()
	for _, network := range networks {
		fmt.Printf("  - %s\n", network)
	}
	fmt.Println()

	// Demonstrate transport selection and capabilities
	addresses := []string{
		"127.0.0.1:8080",     // IP address -> IPTransport
		"localhost:9000",     // Hostname -> IPTransport
		"test.onion:80",      // Tor address -> TorTransport
		"example.b32.i2p:80", // I2P address -> I2PTransport
		"service.nym:80",     // Nym address -> NymTransport
	}

	fmt.Println("Transport Selection Examples:")
	for _, addr := range addresses {
		demonstrateTransportSelection(mt, addr)
	}
	fmt.Println()

	// Demonstrate working IP transport functionality
	fmt.Println("Working IP Transport Demonstration:")
	demonstrateIPTransport(mt)
	fmt.Println()

	// Show individual transport access
	fmt.Println("Direct Transport Access:")
	demonstrateDirectTransportAccess(mt)
}

// demonstrateTransportSelection shows how MultiTransport selects appropriate
// transports based on address format. It attempts to create a listener at
// the given address and reports the result, demonstrating automatic transport
// routing for different network types.
func demonstrateTransportSelection(mt *transport.MultiTransport, address string) {
	fmt.Printf("Address: %s\n", address)

	// Try to create a listener (this will show transport selection)
	listener, err := mt.Listen(address)
	if err != nil {
		fmt.Printf("  Result: %s (expected for unimplemented transports)\n", err.Error())
	} else {
		fmt.Printf("  Result: Listener created at %s\n", listener.Addr().String())
		listener.Close()
	}
	fmt.Println()
}

// demonstrateIPTransport shows the fully functional IP transport with a
// complete TCP echo server/client example. It demonstrates listener creation,
// connection establishment, and bidirectional data transfer using the
// MultiTransport unified interface.
func demonstrateIPTransport(mt *transport.MultiTransport) {
	// Create a TCP listener
	fmt.Println("Creating TCP listener on localhost...")
	listener, err := mt.Listen("127.0.0.1:0")
	if err != nil {
		logrus.WithError(err).Error("Failed to create listener")
		return
	}
	defer listener.Close()

	fmt.Printf("TCP listener created: %s\n", listener.Addr().String())

	// Create a UDP packet connection
	fmt.Println("Creating UDP packet connection...")
	conn, err := mt.DialPacket("127.0.0.1:0")
	if err != nil {
		logrus.WithError(err).Error("Failed to create packet connection")
		return
	}
	defer conn.Close()

	fmt.Printf("UDP connection created: %s\n", conn.LocalAddr().String())

	// Demonstrate a simple TCP connection
	go func() {
		// Accept one connection for demo
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read and echo back
		buffer := make([]byte, 1024)
		n, err := conn.Read(buffer)
		if err != nil {
			return
		}
		// Explicitly ignore write error in echo server (demo code)
		_, _ = conn.Write(buffer[:n])
	}()

	// Connect to the listener
	fmt.Println("Testing TCP connection...")
	client, err := mt.Dial(listener.Addr().String())
	if err != nil {
		logrus.WithError(err).Error("Failed to dial")
		return
	}
	defer client.Close()

	// Send test message
	message := "Hello Multi-Transport!"
	if _, err := client.Write([]byte(message)); err != nil {
		logrus.WithError(err).Error("Failed to write message")
		return
	}

	// Read response
	buffer := make([]byte, 1024)
	// Note: time.Now() used for deadline is acceptable for demo code showing timeout patterns
	client.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, err := client.Read(buffer)
	if err != nil {
		logrus.WithError(err).Error("Failed to read response")
		return
	}

	fmt.Printf("Sent: %s\n", message)
	fmt.Printf("Received: %s\n", string(buffer[:n]))
}

// demonstrateDirectTransportAccess shows how to access specific transport
// implementations directly and register custom transports. This is useful
// when transport-specific configuration or capabilities are needed beyond
// the unified MultiTransport interface.
func demonstrateDirectTransportAccess(mt *transport.MultiTransport) {
	transportTypes := []string{"ip", "tor", "i2p", "nym"}

	for _, transportType := range transportTypes {
		transport, exists := mt.GetTransport(transportType)
		if !exists {
			fmt.Printf("%s transport: Not registered\n", transportType)
			continue
		}

		networks := transport.SupportedNetworks()
		fmt.Printf("%s transport: Supports %v\n", transportType, networks)
	}
	fmt.Println()

	// Demonstrate registering a custom transport
	fmt.Println("Custom Transport Registration:")
	customTransport := transport.NewIPTransport()
	mt.RegisterTransport("custom", customTransport)

	if transport, exists := mt.GetTransport("custom"); exists {
		fmt.Printf("Custom transport registered: Supports %v\n", transport.SupportedNetworks())
	}
}
