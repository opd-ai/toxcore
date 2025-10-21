// Package main demonstrates the new NetworkAddress system for multi-network support.
//
// This example shows how to use the new address type system to support
// different network types including IPv4, IPv6, Tor .onion, I2P .b32.i2p,
// Nym .nym, and Lokinet .loki addresses.
package main

import (
	"fmt"
	"log"
	"net"

	"github.com/opd-ai/toxforge/transport"
)

func main() {
	fmt.Println("=== NetworkAddress System Demo ===")
	fmt.Println()

	// Example 1: Working with IPv4 addresses
	fmt.Println("1. IPv4 Address Example:")
	ipv4Addr := &net.UDPAddr{
		IP:   net.IPv4(192, 168, 1, 100),
		Port: 8080,
	}

	netAddr, err := transport.ConvertNetAddrToNetworkAddress(ipv4Addr)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("   Original: %s\n", ipv4Addr.String())
	fmt.Printf("   NetworkAddress: %s\n", netAddr.String())
	fmt.Printf("   Type: %s\n", netAddr.Type.String())
	fmt.Printf("   IsPrivate: %t\n", netAddr.IsPrivate())
	fmt.Printf("   IsRoutable: %t\n", netAddr.IsRoutable())
	fmt.Printf("   Back to net.Addr: %s\n", netAddr.ToNetAddr().String())
	fmt.Println()

	// Example 2: Working with public IPv4 addresses
	fmt.Println("2. Public IPv4 Address Example:")
	publicAddr := &net.TCPAddr{
		IP:   net.IPv4(8, 8, 8, 8),
		Port: 53,
	}

	netAddr, err = transport.ConvertNetAddrToNetworkAddress(publicAddr)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("   Original: %s\n", publicAddr.String())
	fmt.Printf("   NetworkAddress: %s\n", netAddr.String())
	fmt.Printf("   IsPrivate: %t\n", netAddr.IsPrivate())
	fmt.Printf("   IsRoutable: %t\n", netAddr.IsRoutable())
	fmt.Println()

	// Example 3: Working with different network types
	fmt.Println("3. Multi-Network Address Examples:")

	examples := []struct {
		name string
		addr *transport.NetworkAddress
	}{
		{
			name: "Tor Onion Address",
			addr: &transport.NetworkAddress{
				Type:    transport.AddressTypeOnion,
				Data:    []byte("exampleexampleexample.onion"),
				Port:    8080,
				Network: "tcp",
			},
		},
		{
			name: "I2P Address",
			addr: &transport.NetworkAddress{
				Type:    transport.AddressTypeI2P,
				Data:    []byte("example12345678901234567890123456.b32.i2p"),
				Port:    8080,
				Network: "tcp",
			},
		},
		{
			name: "Nym Address",
			addr: &transport.NetworkAddress{
				Type:    transport.AddressTypeNym,
				Data:    []byte("example.nym"),
				Port:    8080,
				Network: "tcp",
			},
		},
		{
			name: "Lokinet Address",
			addr: &transport.NetworkAddress{
				Type:    transport.AddressTypeLoki,
				Data:    []byte("example.loki"),
				Port:    8080,
				Network: "tcp",
			},
		},
	}

	for _, example := range examples {
		fmt.Printf("   %s:\n", example.name)
		fmt.Printf("     NetworkAddress: %s\n", example.addr.String())
		fmt.Printf("     Type: %s\n", example.addr.Type.String())
		fmt.Printf("     IsPrivate: %t\n", example.addr.IsPrivate())
		fmt.Printf("     IsRoutable: %t\n", example.addr.IsRoutable())
		fmt.Printf("     net.Addr: %s\n", example.addr.ToNetAddr().String())
		fmt.Println()
	}

	// Example 4: Address Type Detection
	fmt.Println("4. Address Type Detection:")
	addressTypes := []transport.AddressType{
		transport.AddressTypeIPv4,
		transport.AddressTypeIPv6,
		transport.AddressTypeOnion,
		transport.AddressTypeI2P,
		transport.AddressTypeNym,
		transport.AddressTypeLoki,
		transport.AddressTypeUnknown,
	}

	for _, addrType := range addressTypes {
		fmt.Printf("   %s (0x%02X)\n", addrType.String(), uint8(addrType))
	}

	fmt.Println()
	fmt.Println("=== Demo Complete ===")
	fmt.Println("The NetworkAddress system provides a foundation for supporting")
	fmt.Println("multiple network types while maintaining backward compatibility")
	fmt.Println("with existing IPv4/IPv6 net.Addr interfaces.")
}
