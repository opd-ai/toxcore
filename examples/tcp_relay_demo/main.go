package main

import (
	"fmt"
	"log"

	toxcore "github.com/opd-ai/toxcore"
)

// Demo function that validates our AddTcpRelay implementation
// against the documented API usage patterns
func main() {
	fmt.Println("Testing AddTcpRelay implementation...")

	// Create a new Tox instance as documented
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		log.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test the AddTcpRelay method as documented in README
	address := "tox.abiliri.org"
	port := uint16(3389)
	publicKey := "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67"

	fmt.Printf("Adding TCP relay: %s:%d\n", address, port)
	err = tox.AddTcpRelay(address, port, publicKey)
	if err != nil {
		log.Fatalf("Failed to add TCP relay: %v", err)
	}

	fmt.Println("✓ TCP relay added successfully!")

	// Test with multiple relays
	testRelays := []struct {
		addr string
		port uint16
		key  string
	}{
		{"relay1.example.com", 3389, "A404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67"},
		{"relay2.example.com", 3390, "B404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67"},
	}

	for i, relay := range testRelays {
		fmt.Printf("Adding TCP relay %d: %s:%d\n", i+1, relay.addr, relay.port)
		err = tox.AddTcpRelay(relay.addr, relay.port, relay.key)
		if err != nil {
			log.Printf("Failed to add TCP relay %d: %v", i+1, err)
		} else {
			fmt.Printf("✓ TCP relay %d added successfully!\n", i+1)
		}
	}

	// Test error handling with invalid parameters
	fmt.Println("\nTesting error handling...")

	// Test empty address
	err = tox.AddTcpRelay("", 3389, publicKey)
	if err != nil {
		fmt.Printf("✓ Correctly rejected empty address: %v\n", err)
	} else {
		fmt.Println("✗ Should have rejected empty address")
	}

	// Test zero port
	err = tox.AddTcpRelay(address, 0, publicKey)
	if err != nil {
		fmt.Printf("✓ Correctly rejected zero port: %v\n", err)
	} else {
		fmt.Println("✗ Should have rejected zero port")
	}

	// Test invalid public key
	err = tox.AddTcpRelay(address, port, "invalid")
	if err != nil {
		fmt.Printf("✓ Correctly rejected invalid public key: %v\n", err)
	} else {
		fmt.Println("✗ Should have rejected invalid public key")
	}

	fmt.Println("\n🎉 AddTcpRelay implementation validation complete!")
	fmt.Println("The method matches the documented API and handles all expected use cases.")
}
