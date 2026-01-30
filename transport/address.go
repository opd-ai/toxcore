// Package transport implements network address abstraction for multi-network support.
//
// This file provides the foundation for supporting multiple network types
// (.onion, .b32.i2p, .nym, .loki) by abstracting away IP-specific assumptions.
package transport

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// AddressType represents the type of network address.
// This enumeration allows the protocol to support different network types
// without breaking compatibility with existing IP-based implementations.
type AddressType uint8

const (
	// AddressTypeIPv4 represents IPv4 addresses (legacy compatibility)
	AddressTypeIPv4 AddressType = 0x01
	// AddressTypeIPv6 represents IPv6 addresses (legacy compatibility)
	AddressTypeIPv6 AddressType = 0x02
	// AddressTypeOnion represents Tor .onion addresses
	AddressTypeOnion AddressType = 0x03
	// AddressTypeI2P represents I2P .b32.i2p addresses
	AddressTypeI2P AddressType = 0x04
	// AddressTypeNym represents Nym .nym addresses
	AddressTypeNym AddressType = 0x05
	// AddressTypeLoki represents Lokinet .loki addresses
	AddressTypeLoki AddressType = 0x06
	// AddressTypeUnknown represents unknown or unsupported address types
	AddressTypeUnknown AddressType = 0xFF
)

// String returns a human-readable representation of the AddressType.
func (at AddressType) String() string {
	switch at {
	case AddressTypeIPv4:
		return "IPv4"
	case AddressTypeIPv6:
		return "IPv6"
	case AddressTypeOnion:
		return "Onion"
	case AddressTypeI2P:
		return "I2P"
	case AddressTypeNym:
		return "Nym"
	case AddressTypeLoki:
		return "Loki"
	case AddressTypeUnknown:
		return "Unknown"
	default:
		return fmt.Sprintf("AddressType(%d)", uint8(at))
	}
}

// NetworkAddress represents a network address that can be of various types.
// This abstraction allows the protocol to work with different network types
// without making assumptions about the underlying address format.
type NetworkAddress struct {
	// Type specifies the network address type
	Type AddressType
	// Data contains the variable-length address data
	Data []byte
	// Port contains the port number (0 if not applicable for the network type)
	Port uint16
	// Network contains the network identifier ("tcp", "udp", "tor", "i2p", etc.)
	Network string
}

// ToNetAddr converts the NetworkAddress to a net.Addr interface.
// This provides backward compatibility with existing code that expects net.Addr.
func (na *NetworkAddress) ToNetAddr() net.Addr {
	switch na.Type {
	case AddressTypeIPv4, AddressTypeIPv6:
		return na.toIPAddr()
	case AddressTypeOnion, AddressTypeI2P, AddressTypeNym, AddressTypeLoki:
		return na.toCustomAddr()
	default:
		return &customAddr{
			network: na.Network,
			address: string(na.Data),
		}
	}
}

// toIPAddr converts IPv4/IPv6 addresses to standard net.Addr types.
func (na *NetworkAddress) toIPAddr() net.Addr {
	if len(na.Data) == 0 {
		return nil
	}

	var ip net.IP
	if na.Type == AddressTypeIPv4 && len(na.Data) >= 4 {
		ip = net.IP(na.Data[:4])
	} else if na.Type == AddressTypeIPv6 && len(na.Data) >= 16 {
		ip = net.IP(na.Data[:16])
	} else {
		return nil
	}

	if na.Network == "tcp" {
		return &net.TCPAddr{IP: ip, Port: int(na.Port)}
	}
	return &net.UDPAddr{IP: ip, Port: int(na.Port)}
}

// toCustomAddr converts non-IP addresses to custom net.Addr implementation.
func (na *NetworkAddress) toCustomAddr() net.Addr {
	address := string(na.Data)
	if na.Port > 0 {
		address = net.JoinHostPort(address, strconv.Itoa(int(na.Port)))
	}
	return &customAddr{
		network: na.Network,
		address: address,
	}
}

// String returns a human-readable representation of the NetworkAddress.
func (na *NetworkAddress) String() string {
	var address string

	// Handle address string based on type
	switch na.Type {
	case AddressTypeIPv4:
		if len(na.Data) >= 4 {
			ip := net.IP(na.Data[:4])
			address = ip.String()
		} else {
			address = string(na.Data)
		}
	case AddressTypeIPv6:
		if len(na.Data) >= 16 {
			ip := net.IP(na.Data[:16])
			address = ip.String()
		} else {
			address = string(na.Data)
		}
	default:
		address = string(na.Data)
	}

	if na.Port > 0 && na.Type != AddressTypeOnion {
		// Onion addresses typically include port in the address string
		address = net.JoinHostPort(address, strconv.Itoa(int(na.Port)))
	}
	return fmt.Sprintf("%s://%s", na.Type.String(), address)
}

// IsPrivate determines if the address represents a private network.
// This method provides network-type-specific logic for privacy detection.
func (na *NetworkAddress) IsPrivate() bool {
	switch na.Type {
	case AddressTypeIPv4:
		return na.isPrivateIPv4()
	case AddressTypeIPv6:
		return na.isPrivateIPv6()
	case AddressTypeOnion, AddressTypeI2P, AddressTypeNym, AddressTypeLoki:
		// These are inherently private/anonymized networks
		return true
	default:
		// Unknown address types are assumed to be private for safety
		return true
	}
}

// isPrivateIPv4 checks if an IPv4 address is in a private range.
func (na *NetworkAddress) isPrivateIPv4() bool {
	if len(na.Data) < 4 {
		return true // Invalid address, assume private for safety
	}

	ip := na.Data
	// Check RFC 1918 private address ranges
	return ip[0] == 10 ||
		(ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31) ||
		(ip[0] == 192 && ip[1] == 168) ||
		(ip[0] == 127) // localhost
}

// isPrivateIPv6 checks if an IPv6 address is in a private range.
func (na *NetworkAddress) isPrivateIPv6() bool {
	if len(na.Data) < 16 {
		return true // Invalid address, assume private for safety
	}

	ip := net.IP(na.Data[:16])
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate()
}

// IsRoutable determines if the address is routable on the internet.
// This is the inverse of IsPrivate for most address types.
func (na *NetworkAddress) IsRoutable() bool {
	switch na.Type {
	case AddressTypeIPv4, AddressTypeIPv6:
		return !na.IsPrivate()
	case AddressTypeOnion, AddressTypeI2P, AddressTypeNym, AddressTypeLoki:
		// These are routable through their respective networks
		return true
	default:
		// Unknown address types are assumed to be non-routable for safety
		return false
	}
}

// customAddr implements net.Addr for non-IP address types.
type customAddr struct {
	network string
	address string
}

// Network returns the network type.
func (ca *customAddr) Network() string {
	return ca.network
}

// String returns the address string.
func (ca *customAddr) String() string {
	return ca.address
}

// ConvertNetAddrToNetworkAddress converts a net.Addr to NetworkAddress.
// This function provides the bridge from existing net.Addr usage to the new system.
func ConvertNetAddrToNetworkAddress(addr net.Addr) (*NetworkAddress, error) {
	if addr == nil {
		return nil, errors.New("address is nil")
	}

	network := addr.Network()
	addrStr := addr.String()

	var na *NetworkAddress
	var err error

	// Handle different address types based on network and string format
	switch {
	case network == "tcp" || network == "udp":
		na, err = parseIPAddress(addr, network)
	case strings.HasSuffix(addrStr, ".onion"):
		na, err = parseOnionAddress(addrStr, network)
	case strings.HasSuffix(addrStr, ".b32.i2p"):
		na, err = parseI2PAddress(addrStr, network)
	case strings.HasSuffix(addrStr, ".nym"):
		na, err = parseNymAddress(addrStr, network)
	case strings.HasSuffix(addrStr, ".loki"):
		na, err = parseLokiAddress(addrStr, network)
	default:
		// Try to parse as IP first, fall back to unknown
		if na, err = parseIPAddress(addr, network); err == nil {
			// Validation will be performed below
		} else {
			na = &NetworkAddress{
				Type:    AddressTypeUnknown,
				Data:    []byte(addrStr),
				Port:    0,
				Network: network,
			}
			err = nil
		}
	}

	if err != nil {
		return nil, err
	}

	// Validate the address for security issues
	if err := na.ValidateAddress(); err != nil {
		return nil, fmt.Errorf("address validation failed: %w", err)
	}

	return na, nil
}

// parseIPAddress parses IPv4/IPv6 addresses from net.Addr.
func parseIPAddress(addr net.Addr, network string) (*NetworkAddress, error) {
	var ip net.IP
	var port int

	switch a := addr.(type) {
	case *net.TCPAddr:
		ip = a.IP
		port = a.Port
	case *net.UDPAddr:
		ip = a.IP
		port = a.Port
	case *net.IPAddr:
		ip = a.IP
		port = 0
	default:
		// Try to parse from string representation
		host, portStr, err := net.SplitHostPort(addr.String())
		if err != nil {
			host = addr.String()
			portStr = "0"
		}

		ip = net.ParseIP(host)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address: %s", host)
		}

		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	if ip == nil {
		return nil, errors.New("no IP address found")
	}

	var addrType AddressType
	var data []byte

	if ipv4 := ip.To4(); ipv4 != nil {
		addrType = AddressTypeIPv4
		data = []byte(ipv4)
	} else {
		addrType = AddressTypeIPv6
		data = []byte(ip.To16())
	}

	return &NetworkAddress{
		Type:    addrType,
		Data:    data,
		Port:    uint16(port),
		Network: network,
	}, nil
}

// parseOnionAddress parses Tor .onion addresses.
func parseOnionAddress(addrStr, network string) (*NetworkAddress, error) {
	host, portStr, err := net.SplitHostPort(addrStr)
	if err != nil {
		// No port specified
		host = addrStr
		portStr = "0"
	}

	port := uint16(0)
	if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
		port = uint16(p)
	}

	return &NetworkAddress{
		Type:    AddressTypeOnion,
		Data:    []byte(host),
		Port:    port,
		Network: network,
	}, nil
}

// parseI2PAddress parses I2P .b32.i2p addresses.
func parseI2PAddress(addrStr, network string) (*NetworkAddress, error) {
	host, portStr, err := net.SplitHostPort(addrStr)
	if err != nil {
		host = addrStr
		portStr = "0"
	}

	port := uint16(0)
	if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
		port = uint16(p)
	}

	return &NetworkAddress{
		Type:    AddressTypeI2P,
		Data:    []byte(host),
		Port:    port,
		Network: network,
	}, nil
}

// parseNymAddress parses Nym .nym addresses.
func parseNymAddress(addrStr, network string) (*NetworkAddress, error) {
	host, portStr, err := net.SplitHostPort(addrStr)
	if err != nil {
		host = addrStr
		portStr = "0"
	}

	port := uint16(0)
	if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
		port = uint16(p)
	}

	return &NetworkAddress{
		Type:    AddressTypeNym,
		Data:    []byte(host),
		Port:    port,
		Network: network,
	}, nil
}

// parseLokiAddress parses Lokinet .loki addresses.
func parseLokiAddress(addrStr, network string) (*NetworkAddress, error) {
	host, portStr, err := net.SplitHostPort(addrStr)
	if err != nil {
		host = addrStr
		portStr = "0"
	}

	port := uint16(0)
	if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
		port = uint16(p)
	}

	return &NetworkAddress{
		Type:    AddressTypeLoki,
		Data:    []byte(host),
		Port:    port,
		Network: network,
	}, nil
}

// ValidateAddress validates a NetworkAddress for security issues.
// Returns an error if the address should not be accepted for security reasons.
func (na *NetworkAddress) ValidateAddress() error {
	if na == nil {
		return errors.New("address is nil")
	}

	switch na.Type {
	case AddressTypeIPv6:
		return na.validateIPv6()
	case AddressTypeIPv4:
		// IPv4 validation could be added here if needed
		return nil
	default:
		// Other address types don't need special validation
		return nil
	}
}

// validateIPv6 performs IPv6-specific security validation.
func (na *NetworkAddress) validateIPv6() error {
	if len(na.Data) < 16 {
		return fmt.Errorf("invalid IPv6 address length: %d", len(na.Data))
	}

	ip := net.IP(na.Data[:16])

	// Reject link-local addresses to prevent local network attacks
	if ip.IsLinkLocalUnicast() {
		return errors.New("link-local IPv6 addresses not allowed for security reasons")
	}

	// Optionally reject other special-use addresses
	if ip.IsMulticast() {
		return errors.New("multicast IPv6 addresses not allowed")
	}

	return nil
}

// ToBytes serializes the NetworkAddress to a byte representation.
// Format: For IPv4: 4 bytes IP + 2 bytes port (big-endian)
//         For IPv6: 16 bytes IP + 2 bytes port (big-endian)
// Returns an error for unsupported address types.
func (na *NetworkAddress) ToBytes() ([]byte, error) {
	switch na.Type {
	case AddressTypeIPv4:
		if len(na.Data) < 4 {
			return nil, fmt.Errorf("invalid IPv4 address length: %d", len(na.Data))
		}
		// IPv4: 4 bytes IP + 2 bytes port
		result := make([]byte, 6)
		copy(result[0:4], na.Data[0:4])
		result[4] = byte(na.Port >> 8)   // High byte of port
		result[5] = byte(na.Port & 0xFF) // Low byte of port
		return result, nil

	case AddressTypeIPv6:
		if len(na.Data) < 16 {
			return nil, fmt.Errorf("invalid IPv6 address length: %d", len(na.Data))
		}
		// IPv6: 16 bytes IP + 2 bytes port
		result := make([]byte, 18)
		copy(result[0:16], na.Data[0:16])
		result[16] = byte(na.Port >> 8)   // High byte of port
		result[17] = byte(na.Port & 0xFF) // Low byte of port
		return result, nil

	default:
		return nil, fmt.Errorf("unsupported address type for serialization: %s", na.Type.String())
	}
}

// SerializeNetAddrToBytes converts a net.Addr to bytes without type assertions.
// This follows the project's networking best practice of avoiding concrete type checks.
// Format: IPv4: 4 bytes IP + 2 bytes port, IPv6: 16 bytes IP + 2 bytes port (big-endian)
func SerializeNetAddrToBytes(addr net.Addr) ([]byte, error) {
	netAddr, err := ConvertNetAddrToNetworkAddress(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to convert address: %w", err)
	}
	return netAddr.ToBytes()
}
