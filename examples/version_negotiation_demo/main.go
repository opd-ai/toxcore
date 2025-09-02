// Package main demonstrates version negotiation and backward compatibility
// for the Tox protocol using the NegotiatingTransport.
package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/toxcore/transport"
)

func main() {
	fmt.Println("=== Tox Protocol Version Negotiation Demo ===")
	
	// Create UDP transports for different nodes
	alice, err := transport.NewUDPTransport("127.0.0.1:8001")
	if err != nil {
		log.Fatalf("Failed to create Alice's UDP transport: %v", err)
	}
	defer alice.Close()
	
	bob, err := transport.NewUDPTransport("127.0.0.1:8002")
	if err != nil {
		log.Fatalf("Failed to create Bob's UDP transport: %v", err)
	}
	defer bob.Close()
	
	// Generate static keys for Noise-IK
	aliceKey := make([]byte, 32)
	bobKey := make([]byte, 32)
	rand.Read(aliceKey)
	rand.Read(bobKey)
	
	fmt.Println("\n1. Creating nodes with different protocol capabilities...")
	
	// Alice supports both Legacy and Noise-IK (modern node)
	aliceCaps := &transport.ProtocolCapabilities{
		SupportedVersions: []transport.ProtocolVersion{
			transport.ProtocolLegacy, 
			transport.ProtocolNoiseIK,
		},
		PreferredVersion:     transport.ProtocolNoiseIK,
		EnableLegacyFallback: true,
		NegotiationTimeout:   5 * time.Second,
	}
	
	aliceTransport, err := transport.NewNegotiatingTransport(alice, aliceCaps, aliceKey)
	if err != nil {
		log.Fatalf("Failed to create Alice's transport: %v", err)
	}
	defer aliceTransport.Close()
	
	// Bob only supports Legacy (older node)
	bobCaps := &transport.ProtocolCapabilities{
		SupportedVersions:    []transport.ProtocolVersion{transport.ProtocolLegacy},
		PreferredVersion:     transport.ProtocolLegacy,
		EnableLegacyFallback: true,
		NegotiationTimeout:   5 * time.Second,
	}
	
	bobTransport, err := transport.NewNegotiatingTransport(bob, bobCaps, bobKey)
	if err != nil {
		log.Fatalf("Failed to create Bob's transport: %v", err)
	}
	defer bobTransport.Close()
	
	fmt.Printf("✓ Alice supports: %v (prefers %s)\n", 
		formatVersions(aliceCaps.SupportedVersions), 
		aliceCaps.PreferredVersion.String())
	fmt.Printf("✓ Bob supports: %v (prefers %s)\n", 
		formatVersions(bobCaps.SupportedVersions), 
		bobCaps.PreferredVersion.String())
	
	fmt.Println("\n2. Demonstrating version negotiation concepts...")
	
	// Get peer addresses
	bobAddr := bob.LocalAddr()
	
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
	
	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("The NegotiatingTransport provides:")
	fmt.Println("• Automatic version negotiation between peers")
	fmt.Println("• Backward compatibility with legacy nodes")
	fmt.Println("• Configurable fallback behavior") 
	fmt.Println("• Seamless protocol upgrade path")
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
