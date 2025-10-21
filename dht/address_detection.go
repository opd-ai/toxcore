// Package dht implements address type detection for multi-network support.
//
// This file provides address type detection and validation for DHT operations
// across different network types (.onion, .b32.i2p, .nym, .loki).
package dht

import (
	"fmt"
	"net"
	"strings"

	"github.com/opd-ai/toxforge/transport"
)

// AddressTypeDetector provides address type detection and validation
// for DHT operations across multiple network types.
type AddressTypeDetector struct {
	// supportedTypes tracks which address types this detector supports
	supportedTypes map[transport.AddressType]bool
}

// NewAddressTypeDetector creates a new address type detector with
// support for all known network types.
func NewAddressTypeDetector() *AddressTypeDetector {
	return &AddressTypeDetector{
		supportedTypes: map[transport.AddressType]bool{
			transport.AddressTypeIPv4:  true,
			transport.AddressTypeIPv6:  true,
			transport.AddressTypeOnion: true,
			transport.AddressTypeI2P:   true,
			transport.AddressTypeNym:   true,
			transport.AddressTypeLoki:  true,
		},
	}
}

// DetectAddressType analyzes a net.Addr and determines its network address type.
// This method replaces IP-specific assumptions with multi-network detection.
func (atd *AddressTypeDetector) DetectAddressType(addr net.Addr) (transport.AddressType, error) {
	if addr == nil {
		return transport.AddressTypeUnknown, fmt.Errorf("address is nil")
	}

	network := addr.Network()
	addrStr := addr.String()

	// Detect address type based on network and string format
	switch {
	case network == "tcp" || network == "udp" || network == "ip":
		return atd.detectIPAddressType(addr)
	case network == "tor" || strings.HasSuffix(addrStr, ".onion"):
		return transport.AddressTypeOnion, nil
	case network == "i2p" || strings.HasSuffix(addrStr, ".b32.i2p"):
		return transport.AddressTypeI2P, nil
	case network == "nym" || strings.HasSuffix(addrStr, ".nym"):
		return transport.AddressTypeNym, nil
	case network == "loki" || strings.HasSuffix(addrStr, ".loki"):
		return transport.AddressTypeLoki, nil
	default:
		// Try to detect IP addresses from string format
		if ipType, err := atd.detectIPAddressTypeFromString(addrStr); err == nil {
			return ipType, nil
		}
		return transport.AddressTypeUnknown, fmt.Errorf("unsupported address type: %s", network)
	}
}

// detectIPAddressType determines whether an IP address is IPv4 or IPv6.
func (atd *AddressTypeDetector) detectIPAddressType(addr net.Addr) (transport.AddressType, error) {
	var ip net.IP

	switch a := addr.(type) {
	case *net.TCPAddr:
		ip = a.IP
	case *net.UDPAddr:
		ip = a.IP
	case *net.IPAddr:
		ip = a.IP
	default:
		return atd.detectIPAddressTypeFromString(addr.String())
	}

	if ip == nil {
		return transport.AddressTypeUnknown, fmt.Errorf("no IP address found")
	}

	if ip.To4() != nil {
		return transport.AddressTypeIPv4, nil
	}
	return transport.AddressTypeIPv6, nil
}

// detectIPAddressTypeFromString attempts to detect IP address type from string representation.
func (atd *AddressTypeDetector) detectIPAddressTypeFromString(addrStr string) (transport.AddressType, error) {
	host, _, err := net.SplitHostPort(addrStr)
	if err != nil {
		host = addrStr // No port specified
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return transport.AddressTypeUnknown, fmt.Errorf("invalid IP address: %s", host)
	}

	if ip.To4() != nil {
		return transport.AddressTypeIPv4, nil
	}
	return transport.AddressTypeIPv6, nil
}

// ValidateAddressType checks if the given address type is supported by this detector.
func (atd *AddressTypeDetector) ValidateAddressType(addrType transport.AddressType) bool {
	return atd.supportedTypes[addrType]
}

// GetSupportedAddressTypes returns a list of all supported address types.
func (atd *AddressTypeDetector) GetSupportedAddressTypes() []transport.AddressType {
	var types []transport.AddressType
	for addrType, supported := range atd.supportedTypes {
		if supported {
			types = append(types, addrType)
		}
	}
	return types
}

// IsRoutableAddress determines if an address type is routable in the DHT.
// This provides policy control over which network types are allowed.
func (atd *AddressTypeDetector) IsRoutableAddress(addrType transport.AddressType) bool {
	switch addrType {
	case transport.AddressTypeIPv4, transport.AddressTypeIPv6:
		// Traditional IP addresses are always routable
		return true
	case transport.AddressTypeOnion, transport.AddressTypeI2P, transport.AddressTypeNym, transport.AddressTypeLoki:
		// Alternative networks are routable through their respective systems
		return true
	default:
		// Unknown address types are not routable for security
		return false
	}
}

// ConvertToNetworkAddress converts a net.Addr to a transport.NetworkAddress using detection.
// This provides the bridge from legacy net.Addr usage to the new multi-network system.
func (atd *AddressTypeDetector) ConvertToNetworkAddress(addr net.Addr) (*transport.NetworkAddress, error) {
	if addr == nil {
		return nil, fmt.Errorf("address is nil")
	}

	// Use the existing transport conversion function which has comprehensive address parsing
	return transport.ConvertNetAddrToNetworkAddress(addr)
}

// NetworkAddressToNetAddr converts a transport.NetworkAddress back to net.Addr for compatibility.
// This ensures backward compatibility with existing code that expects net.Addr.
func (atd *AddressTypeDetector) NetworkAddressToNetAddr(na *transport.NetworkAddress) net.Addr {
	if na == nil {
		return nil
	}
	return na.ToNetAddr()
}

// AddressTypeStats provides statistics about address types encountered during DHT operations.
type AddressTypeStats struct {
	IPv4Count  int `json:"ipv4_count"`
	IPv6Count  int `json:"ipv6_count"`
	OnionCount int `json:"onion_count"`
	I2PCount   int `json:"i2p_count"`
	NymCount   int `json:"nym_count"`
	LokiCount  int `json:"loki_count"`
	TotalCount int `json:"total_count"`
}

// IncrementCount increments the counter for the given address type.
func (ats *AddressTypeStats) IncrementCount(addrType transport.AddressType) {
	ats.TotalCount++
	switch addrType {
	case transport.AddressTypeIPv4:
		ats.IPv4Count++
	case transport.AddressTypeIPv6:
		ats.IPv6Count++
	case transport.AddressTypeOnion:
		ats.OnionCount++
	case transport.AddressTypeI2P:
		ats.I2PCount++
	case transport.AddressTypeNym:
		ats.NymCount++
	case transport.AddressTypeLoki:
		ats.LokiCount++
	}
}

// GetDominantAddressType returns the most frequently encountered address type.
func (ats *AddressTypeStats) GetDominantAddressType() transport.AddressType {
	max := 0
	var dominantType transport.AddressType = transport.AddressTypeUnknown

	if ats.IPv4Count > max {
		max = ats.IPv4Count
		dominantType = transport.AddressTypeIPv4
	}
	if ats.IPv6Count > max {
		max = ats.IPv6Count
		dominantType = transport.AddressTypeIPv6
	}
	if ats.OnionCount > max {
		max = ats.OnionCount
		dominantType = transport.AddressTypeOnion
	}
	if ats.I2PCount > max {
		max = ats.I2PCount
		dominantType = transport.AddressTypeI2P
	}
	if ats.NymCount > max {
		max = ats.NymCount
		dominantType = transport.AddressTypeNym
	}
	if ats.LokiCount > max {
		max = ats.LokiCount
		dominantType = transport.AddressTypeLoki
	}

	return dominantType
}
