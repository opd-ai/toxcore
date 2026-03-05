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

	"github.com/opd-ai/toxcore/transport"
)

func main() {
	fmt.Println("=== NetworkAddress System Demo ===")
	fmt.Println()

	demonstrateIPv4Address()
	demonstratePublicIPv4Address()
	demonstrateMultiNetworkAddresses()
	demonstrateAddressTypeDetection()

	fmt.Println()
	fmt.Println("=== Demo Complete ===")
	fmt.Println("The NetworkAddress system provides a foundation for supporting")
	fmt.Println("multiple network types while maintaining backward compatibility")
	fmt.Println("with existing IPv4/IPv6 net.Addr interfaces.")
}

func demonstrateIPv4Address() {
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
}

func demonstratePublicIPv4Address() {
	fmt.Println("2. Public IPv4 Address Example:")
	publicAddr := &net.TCPAddr{
		IP:   net.IPv4(8, 8, 8, 8),
		Port: 53,
	}

	netAddr, err := transport.ConvertNetAddrToNetworkAddress(publicAddr)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("   Original: %s\n", publicAddr.String())
	fmt.Printf("   NetworkAddress: %s\n", netAddr.String())
	fmt.Printf("   IsPrivate: %t\n", netAddr.IsPrivate())
	fmt.Printf("   IsRoutable: %t\n", netAddr.IsRoutable())
	fmt.Println()
}

func demonstrateMultiNetworkAddresses() {
	fmt.Println("3. Multi-Network Address Examples:")

	printNetworkAddressInfo("Tor Onion Address",
		createNetworkAddress(transport.AddressTypeOnion, "exampleexampleexample.onion"))
	printNetworkAddressInfo("I2P Address",
		createNetworkAddress(transport.AddressTypeI2P, "example12345678901234567890123456.b32.i2p"))
	printNetworkAddressInfo("Nym Address",
		createNetworkAddress(transport.AddressTypeNym, "example.nym"))
	printNetworkAddressInfo("Lokinet Address",
		createNetworkAddress(transport.AddressTypeLoki, "example.loki"))
}

func createNetworkAddress(addrType transport.AddressType, data string) *transport.NetworkAddress {
	return &transport.NetworkAddress{
		Type:    addrType,
		Data:    []byte(data),
		Port:    8080,
		Network: "tcp",
	}
}

func printNetworkAddressInfo(name string, addr *transport.NetworkAddress) {
	fmt.Printf("   %s:\n", name)
	fmt.Printf("     NetworkAddress: %s\n", addr.String())
	fmt.Printf("     Type: %s\n", addr.Type.String())
	fmt.Printf("     IsPrivate: %t\n", addr.IsPrivate())
	fmt.Printf("     IsRoutable: %t\n", addr.IsRoutable())
	fmt.Printf("     net.Addr: %s\n", addr.ToNetAddr().String())
	fmt.Println()
}

func demonstrateAddressTypeDetection() {
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
}
