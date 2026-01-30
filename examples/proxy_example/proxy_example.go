// Example: Using toxcore-go with SOCKS5 proxy
//
// This example demonstrates how to configure a Tox instance to route
// all network traffic through a SOCKS5 proxy (e.g., Tor).
//
// Build: go run proxy_example.go
// Run with Tor: tor & go run proxy_example.go

package main

import (
	"fmt"
	"log"

	toxcore "github.com/opd-ai/toxcore"
)

func main() {
	// Create options with SOCKS5 proxy configuration
	options := toxcore.NewOptions()
	options.UDPEnabled = true

	// Configure SOCKS5 proxy (e.g., Tor SOCKS5 proxy)
	options.Proxy = &toxcore.ProxyOptions{
		Type:     toxcore.ProxyTypeSOCKS5,
		Host:     "127.0.0.1",
		Port:     9050, // Default Tor SOCKS5 port
		Username: "",   // Optional authentication
		Password: "",   // Optional authentication
	}

	// Create Tox instance - all traffic will route through the proxy
	tox, err := toxcore.New(options)
	if err != nil {
		log.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Get Tox ID
	address := tox.SelfGetAddress()
	fmt.Printf("Tox ID: %X\n", address)
	fmt.Println("All network traffic is now routed through SOCKS5 proxy at 127.0.0.1:9050")

	// Example: Bootstrap to the Tox network through proxy
	err = tox.Bootstrap(
		"node.tox.biribiri.org",
		33445,
		"F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67",
	)
	if err != nil {
		log.Printf("Bootstrap warning: %v (this is normal without a running proxy)", err)
	}

	fmt.Println("\nProxy Configuration:")
	fmt.Printf("  Type: SOCKS5\n")
	fmt.Printf("  Host: %s\n", options.Proxy.Host)
	fmt.Printf("  Port: %d\n", options.Proxy.Port)
	fmt.Println("\nNote: Ensure your SOCKS5 proxy (e.g., Tor) is running before using this example.")
	fmt.Println("      Start Tor with: tor")
}
