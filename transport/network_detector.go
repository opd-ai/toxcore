// Package transport implements network transport for the Tox protocol.
//
// This file implements the network capability detection system that replaces
// IP-specific logic with capability-based detection for multi-network support.
package transport

import (
	"net"
	"strings"
)

// RoutingMethod defines how packets are routed through different network types
type RoutingMethod int

const (
	// RoutingDirect indicates direct routing without intermediaries
	RoutingDirect RoutingMethod = iota
	// RoutingNAT indicates routing through NAT traversal
	RoutingNAT
	// RoutingProxy indicates routing through proxy networks (Tor, I2P)
	RoutingProxy
	// RoutingMixed indicates multiple routing methods available
	RoutingMixed
)

// String returns a string representation of the routing method
func (rm RoutingMethod) String() string {
	switch rm {
	case RoutingDirect:
		return "Direct"
	case RoutingNAT:
		return "NAT"
	case RoutingProxy:
		return "Proxy"
	case RoutingMixed:
		return "Mixed"
	default:
		return "Unknown"
	}
}

// NetworkCapabilities describes the capabilities of a network address
type NetworkCapabilities struct {
	// SupportsNAT indicates if the network supports NAT traversal
	SupportsNAT bool
	// SupportsUPnP indicates if the network supports UPnP port mapping
	SupportsUPnP bool
	// IsPrivateSpace indicates if the address is in private address space
	IsPrivateSpace bool
	// RoutingMethod describes how packets are routed
	RoutingMethod RoutingMethod
	// SupportsDirectConnection indicates if direct connections are possible
	SupportsDirectConnection bool
	// RequiresProxy indicates if connections require proxy networks
	RequiresProxy bool
}

// NetworkDetector interface defines capability detection for different network types
type NetworkDetector interface {
	// DetectCapabilities analyzes an address and returns its network capabilities
	DetectCapabilities(addr net.Addr) NetworkCapabilities
	// SupportedNetworks returns the list of network types this detector supports
	SupportedNetworks() []string
	// CanDetect determines if this detector can analyze the given address
	CanDetect(addr net.Addr) bool
}

// MultiNetworkDetector aggregates multiple network detectors
type MultiNetworkDetector struct {
	detectors []NetworkDetector
}

// NewMultiNetworkDetector creates a new multi-network detector with default detectors
func NewMultiNetworkDetector() *MultiNetworkDetector {
	return &MultiNetworkDetector{
		detectors: []NetworkDetector{
			&IPNetworkDetector{},
			&TorNetworkDetector{},
			&I2PNetworkDetector{},
			&NymNetworkDetector{},
			&LokiNetworkDetector{},
		},
	}
}

// DetectCapabilities detects network capabilities using the appropriate detector
func (mnd *MultiNetworkDetector) DetectCapabilities(addr net.Addr) NetworkCapabilities {
	for _, detector := range mnd.detectors {
		if detector.CanDetect(addr) {
			return detector.DetectCapabilities(addr)
		}
	}

	// Fallback: assume unknown network with conservative capabilities
	return NetworkCapabilities{
		SupportsNAT:              false,
		SupportsUPnP:             false,
		IsPrivateSpace:           true, // Conservative assumption
		RoutingMethod:            RoutingDirect,
		SupportsDirectConnection: false,
		RequiresProxy:            false,
	}
}

// SupportedNetworks returns all supported network types
func (mnd *MultiNetworkDetector) SupportedNetworks() []string {
	var networks []string
	for _, detector := range mnd.detectors {
		networks = append(networks, detector.SupportedNetworks()...)
	}
	return networks
}

// IPNetworkDetector handles IPv4 and IPv6 network detection
type IPNetworkDetector struct{}

// DetectCapabilities analyzes IPv4/IPv6 addresses for NAT and routing capabilities
func (ind *IPNetworkDetector) DetectCapabilities(addr net.Addr) NetworkCapabilities {
	// Extract IP address from the net.Addr
	var ip net.IP

	switch a := addr.(type) {
	case *net.UDPAddr:
		ip = a.IP
	case *net.TCPAddr:
		ip = a.IP
	case *net.IPAddr:
		ip = a.IP
	default:
		// Try to parse from string representation
		host, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			host = addr.String()
		}
		ip = net.ParseIP(host)
	}

	if ip == nil {
		// Cannot parse as IP, assume public with conservative settings
		return NetworkCapabilities{
			SupportsNAT:              false,
			SupportsUPnP:             false,
			IsPrivateSpace:           false,
			RoutingMethod:            RoutingDirect,
			SupportsDirectConnection: true,
			RequiresProxy:            false,
		}
	}

	isPrivate := ind.isPrivateIP(ip)

	return NetworkCapabilities{
		SupportsNAT:              isPrivate, // Private IPs typically need NAT
		SupportsUPnP:             isPrivate, // UPnP is relevant for private networks
		IsPrivateSpace:           isPrivate,
		RoutingMethod:            ind.getRoutingMethod(isPrivate),
		SupportsDirectConnection: !isPrivate, // Public IPs support direct connections
		RequiresProxy:            false,
	}
}

// SupportedNetworks returns the network types this detector supports
func (ind *IPNetworkDetector) SupportedNetworks() []string {
	return []string{"tcp", "udp", "ip", "tcp4", "tcp6", "udp4", "udp6"}
}

// CanDetect determines if this detector can analyze the given address
func (ind *IPNetworkDetector) CanDetect(addr net.Addr) bool {
	network := strings.ToLower(addr.Network())
	for _, supported := range ind.SupportedNetworks() {
		if network == supported {
			return true
		}
	}
	return false
}

// isPrivateIP checks if an IP address is in private address space
func (ind *IPNetworkDetector) isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Check for IPv4 private ranges (RFC 1918)
	if ip.To4() != nil {
		ip = ip.To4()
		return ip[0] == 10 ||
			(ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31) ||
			(ip[0] == 192 && ip[1] == 168) ||
			ip[0] == 127 // Include localhost as private
	}

	// Check for IPv6 private ranges
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

// getRoutingMethod determines the routing method for IP addresses
func (ind *IPNetworkDetector) getRoutingMethod(isPrivate bool) RoutingMethod {
	if isPrivate {
		return RoutingNAT
	}
	return RoutingDirect
}

// TorNetworkDetector handles Tor .onion address detection
type TorNetworkDetector struct{}

// DetectCapabilities analyzes Tor .onion addresses
func (tnd *TorNetworkDetector) DetectCapabilities(addr net.Addr) NetworkCapabilities {
	return NetworkCapabilities{
		SupportsNAT:              false, // Tor doesn't use traditional NAT
		SupportsUPnP:             false, // UPnP not applicable to Tor
		IsPrivateSpace:           true,  // Tor addresses are not directly routable
		RoutingMethod:            RoutingProxy,
		SupportsDirectConnection: false, // Requires Tor proxy
		RequiresProxy:            true,
	}
}

// SupportedNetworks returns the network types this detector supports
func (tnd *TorNetworkDetector) SupportedNetworks() []string {
	return []string{"tor", "onion"}
}

// CanDetect determines if this detector can analyze the given address
func (tnd *TorNetworkDetector) CanDetect(addr net.Addr) bool {
	addrStr := strings.ToLower(addr.String())
	network := strings.ToLower(addr.Network())

	// Check network type or address pattern
	for _, supported := range tnd.SupportedNetworks() {
		if network == supported {
			return true
		}
	}

	// Check for .onion suffix
	return strings.Contains(addrStr, ".onion")
}

// I2PNetworkDetector handles I2P .b32.i2p address detection
type I2PNetworkDetector struct{}

// DetectCapabilities analyzes I2P addresses
func (i2pnd *I2PNetworkDetector) DetectCapabilities(addr net.Addr) NetworkCapabilities {
	return NetworkCapabilities{
		SupportsNAT:              false, // I2P doesn't use traditional NAT
		SupportsUPnP:             false, // UPnP not applicable to I2P
		IsPrivateSpace:           true,  // I2P addresses are not directly routable
		RoutingMethod:            RoutingProxy,
		SupportsDirectConnection: false, // Requires I2P proxy
		RequiresProxy:            true,
	}
}

// SupportedNetworks returns the network types this detector supports
func (i2pnd *I2PNetworkDetector) SupportedNetworks() []string {
	return []string{"i2p"}
}

// CanDetect determines if this detector can analyze the given address
func (i2pnd *I2PNetworkDetector) CanDetect(addr net.Addr) bool {
	addrStr := strings.ToLower(addr.String())
	network := strings.ToLower(addr.Network())

	// Check network type or address pattern
	for _, supported := range i2pnd.SupportedNetworks() {
		if network == supported {
			return true
		}
	}

	// Check for .i2p suffix
	return strings.Contains(addrStr, ".i2p")
}

// NymNetworkDetector handles Nym address detection
type NymNetworkDetector struct{}

// DetectCapabilities analyzes Nym addresses
func (nnd *NymNetworkDetector) DetectCapabilities(addr net.Addr) NetworkCapabilities {
	return NetworkCapabilities{
		SupportsNAT:              false, // Nym doesn't use traditional NAT
		SupportsUPnP:             false, // UPnP not applicable to Nym
		IsPrivateSpace:           true,  // Nym addresses are not directly routable
		RoutingMethod:            RoutingProxy,
		SupportsDirectConnection: false, // Requires Nym mixnet
		RequiresProxy:            true,
	}
}

// SupportedNetworks returns the network types this detector supports
func (nnd *NymNetworkDetector) SupportedNetworks() []string {
	return []string{"nym"}
}

// CanDetect determines if this detector can analyze the given address
func (nnd *NymNetworkDetector) CanDetect(addr net.Addr) bool {
	addrStr := strings.ToLower(addr.String())
	network := strings.ToLower(addr.Network())

	// Check network type or address pattern
	for _, supported := range nnd.SupportedNetworks() {
		if network == supported {
			return true
		}
	}

	// Check for .nym suffix
	return strings.Contains(addrStr, ".nym")
}

// LokiNetworkDetector handles Loki .loki address detection
type LokiNetworkDetector struct{}

// DetectCapabilities analyzes Loki addresses
func (lnd *LokiNetworkDetector) DetectCapabilities(addr net.Addr) NetworkCapabilities {
	return NetworkCapabilities{
		SupportsNAT:              false, // Loki doesn't use traditional NAT
		SupportsUPnP:             false, // UPnP not applicable to Loki
		IsPrivateSpace:           true,  // Loki addresses are not directly routable
		RoutingMethod:            RoutingProxy,
		SupportsDirectConnection: false, // Requires Loki network
		RequiresProxy:            true,
	}
}

// SupportedNetworks returns the network types this detector supports
func (lnd *LokiNetworkDetector) SupportedNetworks() []string {
	return []string{"loki"}
}

// CanDetect determines if this detector can analyze the given address
func (lnd *LokiNetworkDetector) CanDetect(addr net.Addr) bool {
	addrStr := strings.ToLower(addr.String())
	network := strings.ToLower(addr.Network())

	// Check network type or address pattern
	for _, supported := range lnd.SupportedNetworks() {
		if network == supported {
			return true
		}
	}

	// Check for .loki suffix
	return strings.Contains(addrStr, ".loki")
}
