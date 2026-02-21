package main

import (
	"fmt"
	"log"

	"github.com/opd-ai/toxcore/transport"
)

func main() {
	fmt.Println("=== Tor Transport Demo ===")
	fmt.Println("This demo showcases Tor onion services via the onramp library.")
	fmt.Println()

	tor := transport.NewTorTransport()
	defer tor.Close()

	networks := tor.SupportedNetworks()
	fmt.Printf("Supported networks: %v\n", networks)
	fmt.Println()

	// Demonstrate listener capability (new with onramp)
	fmt.Println("--- Tor Listener (Onion Service) ---")
	fmt.Println("Attempting to create a Tor hidden service...")
	listener, err := tor.Listen("myservice.onion:80")
	if err != nil {
		log.Printf("Listener creation failed: %v\n", err)
		fmt.Println("\nTo host Tor onion services:")
		fmt.Println("1. Install Tor: sudo apt-get install tor")
		fmt.Println("2. Start Tor: sudo systemctl start tor")
		fmt.Println("3. Ensure Tor control port is enabled (default: 9051)")
		fmt.Println("4. Configure via TOR_CONTROL_ADDR environment variable if needed")
		fmt.Println()
	} else {
		defer listener.Close()
		fmt.Println("Listener created successfully!")
		fmt.Printf("Listening on: %s\n", listener.Addr())
		fmt.Println("Keys stored in: onionkeys/toxcore-tor.onion.private")
		fmt.Println()
	}

	// Demonstrate dial capability
	fmt.Println("--- Tor Dialer ---")
	onionAddr := "duckduckgogg42xjoc72x3sjasowoarfbgcmvfimaftt6twagswzczad.onion:80"
	fmt.Printf("Attempting to connect to %s...\n", onionAddr)

	conn, err := tor.Dial(onionAddr)
	if err != nil {
		log.Printf("Connection failed: %v\n", err)
		fmt.Println("\nTo dial through Tor:")
		fmt.Println("1. Install Tor: sudo apt-get install tor")
		fmt.Println("2. Start Tor: sudo systemctl start tor")
		fmt.Println("3. Ensure Tor control port is enabled")
		return
	}
	defer conn.Close()

	fmt.Println("Connected successfully!")
	fmt.Printf("Local: %s Remote: %s\n", conn.LocalAddr(), conn.RemoteAddr())
}
