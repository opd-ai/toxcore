package main

import (
	"fmt"
	"log"

	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

func main() {
	// Set up logging
	logrus.SetLevel(logrus.InfoLevel)

	fmt.Println("=== Address Parser Demo ===")
	fmt.Println("Demonstrating Phase 4.2: Address Resolution Service")
	fmt.Println()

	// Create multi-network address parser
	parser := transport.NewMultiNetworkParser()
	defer parser.Close()

	// Display supported networks
	networks := parser.GetSupportedNetworks()
	fmt.Printf("Supported Networks: %v\n\n", networks)

	// Test addresses for each network type
	testAddresses := []string{
		"127.0.0.1:8080",            // IPv4
		"[::1]:9000",                // IPv6
		"localhost:3000",            // Hostname
		"3g2upl4pq6kufc4m.onion:80", // Tor v2
		"duckduckgogg42ts72uq75htqjyqmp6u2mtpd6d7lw6wpfrdz0ycxhhfakid.onion:443", // Tor v3
		"example.b32.i2p:80", // I2P
		"service.nym:80",     // Nym
	}

	fmt.Println("Address Parsing Examples:")
	for _, address := range testAddresses {
		fmt.Printf("Address: %s\n", address)

		results, err := parser.Parse(address)
		if err != nil {
			fmt.Printf("  Result: parsing failed: %v\n", err)
		} else {
			for i, result := range results {
				fmt.Printf("  Result[%d]: %s network, type: %s, port: %d\n",
					i, result.Network, result.Type, result.Port)
			}
		}
		fmt.Println()
	}

	// Demonstrate individual parser access
	fmt.Println("Direct Parser Access:")

	// Get IP parser
	if ipParser, exists := parser.GetParser("ip"); exists {
		fmt.Printf("IP parser: %s network\n", ipParser.GetNetworkType())

		// Test hostname resolution
		if ipParser.CanParse("google.com:80") {
			result, err := ipParser.ParseAddress("google.com:80")
			if err != nil {
				fmt.Printf("  Hostname resolution failed: %v\n", err)
			} else {
				fmt.Printf("  google.com:80 → %s network, type: %s\n", result.Network, result.Type)
			}
		}
	}

	// Get Tor parser
	if torParser, exists := parser.GetParser("tor"); exists {
		fmt.Printf("Tor parser: %s network\n", torParser.GetNetworkType())

		// Test onion address validation
		testOnion := "invalid.onion:80"
		if torParser.CanParse(testOnion) {
			result, err := torParser.ParseAddress(testOnion)
			if err != nil {
				fmt.Printf("  %s parsing failed: %v\n", testOnion, err)
			} else {
				// Try validation
				if err := torParser.ValidateAddress(result); err != nil {
					fmt.Printf("  %s validation failed: %v\n", testOnion, err)
				} else {
					fmt.Printf("  %s validated successfully\n", testOnion)
				}
			}
		}
	}

	// Get I2P parser
	if i2pParser, exists := parser.GetParser("i2p"); exists {
		fmt.Printf("I2P parser: %s network\n", i2pParser.GetNetworkType())
	}

	// Get Nym parser
	if nymParser, exists := parser.GetParser("nym"); exists {
		fmt.Printf("Nym parser: %s network\n", nymParser.GetNetworkType())
	}

	fmt.Println()

	// Demonstrate custom parser registration
	fmt.Println("Custom Parser Registration:")

	// Create custom parser (reusing IP parser for demo)
	customParser := transport.NewIPAddressParser()
	parser.RegisterNetwork("custom", customParser)

	// Show updated network list
	updatedNetworks := parser.GetSupportedNetworks()
	fmt.Printf("Networks after custom registration: %v\n", updatedNetworks)

	// Test custom parser retrieval
	if retrieved, exists := parser.GetParser("custom"); exists {
		fmt.Printf("Custom parser retrieved: %s network\n", retrieved.GetNetworkType())
	}

	fmt.Println()

	// Demonstrate error handling
	fmt.Println("Error Handling Examples:")

	errorCases := []string{
		"",                   // Empty
		"no-port",            // Missing port
		"256.256.256.256:80", // Invalid IP
		"bad.format",         // No port, unknown domain
	}

	for _, errorCase := range errorCases {
		_, err := parser.Parse(errorCase)
		if err != nil {
			fmt.Printf("  \"%s\" → Error: %v\n", errorCase, err)
		} else {
			fmt.Printf("  \"%s\" → Unexpectedly succeeded\n", errorCase)
		}
	}

	fmt.Println()

	// Performance demonstration
	fmt.Println("Performance Test:")

	// Parse the same address multiple times to show performance
	testAddr := "127.0.0.1:8080"
	const iterations = 1000

	for i := 0; i < iterations; i++ {
		_, err := parser.Parse(testAddr)
		if err != nil {
			log.Printf("Performance test failed: %v", err)
			break
		}
	}

	fmt.Printf("Successfully parsed %s %d times\n", testAddr, iterations)

	fmt.Println()
	fmt.Println("=== Address Parser Demo Complete ===")
	fmt.Println("Phase 4.2: Address Resolution Service implementation ready!")
}
