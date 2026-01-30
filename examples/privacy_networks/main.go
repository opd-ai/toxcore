package main

import (
	"fmt"

	"github.com/opd-ai/toxcore/transport"
)

func main() {
	fmt.Println("Privacy Network Transport Examples")
	fmt.Println("====================================")
	fmt.Println()

	demonstrateTorTransport()
	fmt.Println()
	demonstrateI2PTransport()
	fmt.Println()
	demonstrateLokinetTransport()
}

func demonstrateTorTransport() {
	fmt.Println("1. Tor Transport (.onion addresses)")
	fmt.Println("-----------------------------------")

	tor := transport.NewTorTransport()
	defer tor.Close()

	fmt.Printf("Supported networks: %v\n", tor.SupportedNetworks())

	exampleOnion := "3g2upl4pq6kufc4m.onion:80"
	fmt.Printf("Attempting to connect to: %s\n", exampleOnion)

	conn, err := tor.Dial(exampleOnion)
	if err != nil {
		fmt.Printf("Connection failed (expected if Tor not running): %v\n", err)
	} else {
		fmt.Println("Successfully connected through Tor!")
		fmt.Printf("  Local address:  %s\n", conn.LocalAddr())
		fmt.Printf("  Remote address: %s\n", conn.RemoteAddr())
		conn.Close()
	}

	fmt.Printf("\nCustom Tor proxy can be configured via TOR_PROXY_ADDR environment variable\n")
}

func demonstrateI2PTransport() {
	fmt.Println("2. I2P Transport (.i2p addresses)")
	fmt.Println("----------------------------------")

	i2p := transport.NewI2PTransport()
	defer i2p.Close()

	fmt.Printf("Supported networks: %v\n", i2p.SupportedNetworks())

	exampleI2P := "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80"
	fmt.Printf("Attempting to connect to: %s\n", exampleI2P)

	conn, err := i2p.Dial(exampleI2P)
	if err != nil {
		fmt.Printf("Connection failed (expected if I2P not running): %v\n", err)
	} else {
		fmt.Println("Successfully connected through I2P!")
		fmt.Printf("  Local address:  %s\n", conn.LocalAddr())
		fmt.Printf("  Remote address: %s\n", conn.RemoteAddr())
		conn.Close()
	}

	fmt.Printf("\nCustom I2P SAM address can be configured via I2P_SAM_ADDR environment variable\n")
	fmt.Printf("Default SAM address: 127.0.0.1:7656\n")
}

func demonstrateLokinetTransport() {
	fmt.Println("3. Lokinet Transport (.loki addresses)")
	fmt.Println("--------------------------------------")

	lokinet := transport.NewLokinetTransport()
	defer lokinet.Close()

	fmt.Printf("Supported networks: %v\n", lokinet.SupportedNetworks())

	exampleLoki := "example.loki:80"
	fmt.Printf("Attempting to connect to: %s\n", exampleLoki)

	conn, err := lokinet.Dial(exampleLoki)
	if err != nil {
		fmt.Printf("Connection failed (expected if Lokinet not running): %v\n", err)
	} else {
		fmt.Println("Successfully connected through Lokinet!")
		fmt.Printf("  Local address:  %s\n", conn.LocalAddr())
		fmt.Printf("  Remote address: %s\n", conn.RemoteAddr())
		conn.Close()
	}

	fmt.Printf("\nCustom Lokinet proxy can be configured via LOKINET_PROXY_ADDR environment variable\n")
	fmt.Printf("Default proxy address: 127.0.0.1:9050\n")
}
