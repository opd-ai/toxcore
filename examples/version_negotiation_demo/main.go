// Package main demonstrates version negotiation and backward compatibility
// for the Tox protocol using the NegotiatingTransport.
package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/opd-ai/toxcore/transport"
)

func main() {
	fmt.Println("=== Tox Protocol Version Negotiation Demo ===")

	// Phase 1: Setup nodes and their capabilities
	aliceTransport, bobTransport := setupDemoNodes()
	defer aliceTransport.Close()
	defer bobTransport.Close()

	// Phase 2: Demonstrate version negotiation between Alice and Bob
	demonstrateVersionNegotiation(aliceTransport, bobTransport)

	// Phase 3: Test strict mode with Charlie
	demonstrateStrictMode()

	// Phase 4: Display protocol summary
	displayProtocolSummary()

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("The NegotiatingTransport provides:")
	fmt.Println("• Automatic version negotiation between peers")
	fmt.Println("• Backward compatibility with legacy nodes")
	fmt.Println("• Configurable fallback behavior")
	fmt.Println("• Seamless protocol upgrade path")
}

// setupDemoNodes creates and configures Alice and Bob transports with their protocol capabilities.
func setupDemoNodes() (*transport.NegotiatingTransport, *transport.NegotiatingTransport) {
	fmt.Println("\n1. Creating nodes with different protocol capabilities...")

	aliceKey, bobKey := generateNodeKeys()
	aliceUDP := createUDPTransport("127.0.0.1:8001", "Alice")
	bobUDP := createUDPTransport("127.0.0.1:8002", "Bob")

	aliceCaps := createAliceCapabilities()
	bobCaps := createBobCapabilities()

	aliceTransport := createNegotiatingTransport(aliceUDP, aliceCaps, aliceKey, "Alice")
	bobTransport := createNegotiatingTransport(bobUDP, bobCaps, bobKey, "Bob")

	displayNodeCapabilities(aliceCaps, "Alice")
	displayNodeCapabilities(bobCaps, "Bob")

	return aliceTransport, bobTransport
}

// generateNodeKeys creates cryptographic keys for Alice and Bob.
func generateNodeKeys() ([]byte, []byte) {
	aliceKey := make([]byte, 32)
	bobKey := make([]byte, 32)
	rand.Read(aliceKey)
	rand.Read(bobKey)
	return aliceKey, bobKey
}

// createUDPTransport creates a UDP transport for a node at the specified address.
func createUDPTransport(address, nodeName string) transport.Transport {
	udp, err := transport.NewUDPTransport(address)
	if err != nil {
		log.Fatalf("Failed to create %s's UDP transport: %v", nodeName, err)
	}
	return udp
}

// createAliceCapabilities returns protocol capabilities for Alice (modern node).
func createAliceCapabilities() *transport.ProtocolCapabilities {
	return &transport.ProtocolCapabilities{
		SupportedVersions: []transport.ProtocolVersion{
			transport.ProtocolLegacy,
			transport.ProtocolNoiseIK,
		},
		PreferredVersion:     transport.ProtocolNoiseIK,
		EnableLegacyFallback: true,
		NegotiationTimeout:   5 * time.Second,
	}
}

// createBobCapabilities returns protocol capabilities for Bob (legacy node).
func createBobCapabilities() *transport.ProtocolCapabilities {
	return &transport.ProtocolCapabilities{
		SupportedVersions:    []transport.ProtocolVersion{transport.ProtocolLegacy},
		PreferredVersion:     transport.ProtocolLegacy,
		EnableLegacyFallback: true,
		NegotiationTimeout:   5 * time.Second,
	}
}

// createNegotiatingTransport creates a negotiating transport with the given configuration.
func createNegotiatingTransport(base transport.Transport, caps *transport.ProtocolCapabilities, key []byte, nodeName string) *transport.NegotiatingTransport {
	t, err := transport.NewNegotiatingTransport(base, caps, key)
	if err != nil {
		log.Fatalf("Failed to create %s's transport: %v", nodeName, err)
	}
	return t
}

// displayNodeCapabilities prints the protocol capabilities of a node.
func displayNodeCapabilities(caps *transport.ProtocolCapabilities, nodeName string) {
	fmt.Printf("✓ %s supports: %v (prefers %s)\n",
		nodeName,
		formatVersions(caps.SupportedVersions),
		caps.PreferredVersion.String())
}

// demonstrateVersionNegotiation shows how Alice and Bob negotiate protocol versions and communicate.
func demonstrateVersionNegotiation(aliceTransport, bobTransport *transport.NegotiatingTransport) {
	fmt.Println("\n2. Demonstrating version negotiation concepts...")

	// Get peer addresses - we need to create a UDP address for Bob
	bobAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8002")
	if err != nil {
		log.Fatalf("Failed to resolve Bob's address: %v", err)
	}

	// Alice tries to communicate with Bob
	fmt.Println("→ Alice initiating communication with Bob...")

	// Create a test message
	testPacket := &transport.Packet{
		PacketType: transport.PacketFriendMessage,
		Data:       []byte("Hello from Alice!"),
	}

	// Manually set Bob to use Legacy protocol with Alice to simulate negotiation result
	aliceTransport.SetPeerVersion(bobAddr, transport.ProtocolLegacy)
	fmt.Println("✓ Simulated negotiation: Alice will use Legacy protocol with Bob")

	// Now communication uses the negotiated protocol
	err = aliceTransport.Send(testPacket, bobAddr)
	if err != nil {
		log.Printf("Send failed: %v", err)
	} else {
		fmt.Println("✓ Communication successful with Legacy protocol")
	}

	// Check what protocol version was negotiated
	aliceViewOfBob := aliceTransport.GetPeerVersion(bobAddr)
	fmt.Printf("✓ Alice is using protocol: %s with Bob\n", aliceViewOfBob.String())
}

// demonstrateStrictMode creates Charlie with strict mode (no fallback) and shows incompatible communication scenarios.
func demonstrateStrictMode() {
	fmt.Println("\n3. Testing strict mode (no fallback)...")

	// Create Charlie who supports only Noise-IK (no fallback)
	charlie, err := transport.NewUDPTransport("127.0.0.1:8003")
	if err != nil {
		log.Fatalf("Failed to create Charlie's UDP transport: %v", err)
	}
	defer charlie.Close()

	charlieCaps := &transport.ProtocolCapabilities{
		SupportedVersions:    []transport.ProtocolVersion{transport.ProtocolNoiseIK},
		PreferredVersion:     transport.ProtocolNoiseIK,
		EnableLegacyFallback: false, // Strict mode
		NegotiationTimeout:   5 * time.Second,
	}

	charlieKey := make([]byte, 32)
	rand.Read(charlieKey)

	charlieTransport, err := transport.NewNegotiatingTransport(charlie, charlieCaps, charlieKey)
	if err != nil {
		log.Fatalf("Failed to create Charlie's transport: %v", err)
	}
	defer charlieTransport.Close()

	fmt.Printf("✓ Charlie supports: %v (no fallback enabled)\n",
		formatVersions(charlieCaps.SupportedVersions))

	// Charlie tries to communicate with Bob (legacy-only) - would fail in real scenario
	fmt.Println("→ Charlie trying to communicate with Bob (would fail in negotiation)...")

	// This demonstrates the concept - in real usage, version negotiation would prevent this
	fmt.Println("✓ In real scenario: Version negotiation would prevent incompatible communication")
}

// displayProtocolSummary presents detailed information about each protocol version and their features.
func displayProtocolSummary() {
	fmt.Println("\n4. Protocol version summary:")

	protocols := []struct {
		name     string
		version  transport.ProtocolVersion
		features []string
	}{
		{
			name:    "Legacy",
			version: transport.ProtocolLegacy,
			features: []string{
				"Original Tox protocol",
				"Established network compatibility",
				"No forward secrecy",
				"Potential KCI vulnerability",
			},
		},
		{
			name:    "Noise-IK",
			version: transport.ProtocolNoiseIK,
			features: []string{
				"Noise Protocol Framework",
				"Forward secrecy",
				"KCI resistance",
				"Mutual authentication",
			},
		},
	}

	for _, p := range protocols {
		fmt.Printf("\n%s Protocol (v%d):\n", p.name, p.version)
		for _, feature := range p.features {
			fmt.Printf("  • %s\n", feature)
		}
	}
}

// formatVersions returns a human-readable string of protocol versions
func formatVersions(versions []transport.ProtocolVersion) string {
	if len(versions) == 0 {
		return "none"
	}

	var result string
	for i, v := range versions {
		if i > 0 {
			result += ", "
		}
		result += v.String()
	}
	return result
}
