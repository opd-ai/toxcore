package main

import (
	"fmt"
	"log"

	"github.com/opd-ai/toxcore/transport"
)

func main() {
	fmt.Println("=== Tor Transport Demo ===")
	tor := transport.NewTorTransport()
	defer tor.Close()

	networks := tor.SupportedNetworks()
	fmt.Printf("Supported networks: %v\n", networks)

	onionAddr := "exampleonion.onion:80"
	fmt.Printf("Attempting to connect to %s...\n", onionAddr)

	conn, err := tor.Dial(onionAddr)
	if err != nil {
		log.Printf("Connection failed: %v\n", err)
		fmt.Println("To use Tor transport:")
		fmt.Println("1. Install Tor: sudo apt-get install tor")
		fmt.Println("2. Start Tor: sudo systemctl start tor")
		return
	}
	defer conn.Close()

	fmt.Println("Connected successfully!")
	fmt.Printf("Local: %s Remote: %s\n", conn.LocalAddr(), conn.RemoteAddr())
}
