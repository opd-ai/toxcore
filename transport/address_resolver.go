// Package transport implements network transport for the Tox protocol.
//
// This file implements the public address resolution system that provides
// network-specific methods for discovering public addresses across different
// network types (IP, Tor, I2P, Nym, Loki).
package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// PublicAddressResolver defines the interface for resolving public addresses
// across different network types. Each network type requires different
// methods for public address discovery.
type PublicAddressResolver interface {
	// ResolvePublicAddress attempts to discover the public address for a given local address
	ResolvePublicAddress(ctx context.Context, localAddr net.Addr) (net.Addr, error)

	// SupportsNetwork indicates whether this resolver can handle the given network type
	SupportsNetwork(network string) bool

	// GetResolverName returns a human-readable name for this resolver
	GetResolverName() string
}

// MultiNetworkResolver aggregates multiple network-specific resolvers
// and routes resolution requests to the appropriate resolver based on
// network type detection.
type MultiNetworkResolver struct {
	resolvers []PublicAddressResolver
	// Default timeout for resolution operations
	defaultTimeout time.Duration
}

// NewMultiNetworkResolver creates a new multi-network resolver with
// default resolvers for common network types
func NewMultiNetworkResolver() *MultiNetworkResolver {
	return &MultiNetworkResolver{
		resolvers: []PublicAddressResolver{
			NewIPResolver(),
			&TorResolver{},
			&I2PResolver{},
			&NymResolver{},
			&LokiResolver{},
		},
		defaultTimeout: 30 * time.Second,
	}
}

// ResolvePublicAddress resolves the public address using the appropriate resolver
func (mnr *MultiNetworkResolver) ResolvePublicAddress(ctx context.Context, localAddr net.Addr) (net.Addr, error) {
	if localAddr == nil {
		return nil, errors.New("local address cannot be nil")
	}

	// Find appropriate resolver for this network type
	resolver := mnr.selectResolver(localAddr.Network())
	if resolver == nil {
		return nil, fmt.Errorf("no resolver available for network type: %s", localAddr.Network())
	}

	// Create context with timeout if none provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), mnr.defaultTimeout)
		defer cancel()
	}

	return resolver.ResolvePublicAddress(ctx, localAddr)
}

// selectResolver chooses the appropriate resolver for the given network type
func (mnr *MultiNetworkResolver) selectResolver(network string) PublicAddressResolver {
	for _, resolver := range mnr.resolvers {
		if resolver.SupportsNetwork(network) {
			return resolver
		}
	}
	return nil
}

// GetSupportedNetworks returns a list of all supported network types
func (mnr *MultiNetworkResolver) GetSupportedNetworks() []string {
	var networks []string
	for _, resolver := range mnr.resolvers {
		// Get the first supported network as representative
		// (resolvers may support multiple networks)
		if resolver.SupportsNetwork("tcp") && len(networks) == 0 {
			networks = append(networks, "tcp", "udp", "ip")
		} else if resolver.SupportsNetwork("tor") {
			networks = append(networks, "tor", "onion")
		} else if resolver.SupportsNetwork("i2p") {
			networks = append(networks, "i2p")
		} else if resolver.SupportsNetwork("nym") {
			networks = append(networks, "nym")
		} else if resolver.SupportsNetwork("loki") {
			networks = append(networks, "loki")
		}
	}
	return networks
}

// IPResolver handles public address resolution for IPv4 and IPv6 networks
// Uses multiple methods: interface enumeration, STUN servers, and UPnP
type IPResolver struct {
	stunClient *STUNClient
	upnpClient *UPnPClient
}

// NewIPResolver creates a new IP resolver with STUN and UPnP support
func NewIPResolver() *IPResolver {
	return &IPResolver{
		stunClient: NewSTUNClient(),
		upnpClient: NewUPnPClient(),
	}
}

// ResolvePublicAddress resolves public IP addresses using multiple methods
// Priority order: 1) Interface enumeration, 2) STUN, 3) UPnP
func (ipr *IPResolver) ResolvePublicAddress(ctx context.Context, localAddr net.Addr) (net.Addr, error) {
	localIP, err := ipr.extractIPFromAddress(localAddr)
	if err != nil {
		return nil, err
	}

	if !ipr.isPrivateIP(localIP) {
		return localAddr, nil
	}

	return ipr.resolveViaFallbackMethods(ctx, localAddr)
}

// extractIPFromAddress extracts the IP address from a net.Addr
func (ipr *IPResolver) extractIPFromAddress(localAddr net.Addr) (net.IP, error) {
	var localIP net.IP

	switch addr := localAddr.(type) {
	case *net.UDPAddr:
		localIP = addr.IP
	case *net.TCPAddr:
		localIP = addr.IP
	case *net.IPAddr:
		localIP = addr.IP
	default:
		return nil, fmt.Errorf("unsupported address type for IP resolution: %T", localAddr)
	}

	if localIP == nil {
		return nil, errors.New("invalid IP address")
	}

	return localIP, nil
}

// resolveViaFallbackMethods attempts resolution using interface enumeration, STUN, and UPnP
func (ipr *IPResolver) resolveViaFallbackMethods(ctx context.Context, localAddr net.Addr) (net.Addr, error) {
	// Method 1: Try to find a public IP through interface enumeration
	if publicAddr, err := ipr.findPublicIPFromInterfaces(); err == nil {
		return publicAddr, nil
	}

	// Method 2: Try STUN for accurate public IP detection
	if stunAddr, err := ipr.stunClient.DiscoverPublicAddress(ctx, localAddr); err == nil {
		return stunAddr, nil
	}

	// Method 3: Try UPnP as fallback
	if upnpAddr, err := ipr.resolveViaUPnP(ctx, localAddr); err == nil {
		return upnpAddr, nil
	}

	return nil, errors.New("failed to resolve public IP address using all available methods")
}

// resolveViaUPnP attempts to resolve public address using UPnP
func (ipr *IPResolver) resolveViaUPnP(ctx context.Context, localAddr net.Addr) (net.Addr, error) {
	if err := ipr.upnpClient.DiscoverGateway(ctx); err != nil {
		return nil, err
	}

	upnpIP, err := ipr.upnpClient.GetExternalIPAddress(ctx)
	if err != nil {
		return nil, err
	}

	return ipr.convertToSameAddressType(upnpIP, localAddr), nil
}

// convertToSameAddressType creates a new address of the same type as the input
func (ipr *IPResolver) convertToSameAddressType(ip net.IP, originalAddr net.Addr) net.Addr {
	switch originalAddr.(type) {
	case *net.UDPAddr:
		return &net.UDPAddr{IP: ip, Port: 0}
	case *net.TCPAddr:
		return &net.TCPAddr{IP: ip, Port: 0}
	case *net.IPAddr:
		return &net.IPAddr{IP: ip}
	default:
		// Fallback to UDP address
		return &net.UDPAddr{IP: ip, Port: 0}
	}
}

// SupportsNetwork indicates support for IP-based networks
func (ipr *IPResolver) SupportsNetwork(network string) bool {
	network = strings.ToLower(network)
	return network == "tcp" || network == "udp" || network == "ip" ||
		network == "tcp4" || network == "tcp6" ||
		network == "udp4" || network == "udp6"
}

// GetResolverName returns the resolver name
func (ipr *IPResolver) GetResolverName() string {
	return "IP Resolver"
}

// findPublicIPFromInterfaces attempts to find a public IP from network interfaces
func (ipr *IPResolver) findPublicIPFromInterfaces() (net.Addr, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if !ipr.isPrivateIP(ipnet.IP) {
					// Return as UDP address (default for Tox)
					return &net.UDPAddr{IP: ipnet.IP, Port: 0}, nil
				}
			}
		}
	}

	return nil, errors.New("no public IP address found")
}

// isPrivateIP checks if an IP address is in private address space
func (ipr *IPResolver) isPrivateIP(ip net.IP) bool {
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

// TorResolver handles public address resolution for Tor .onion addresses
type TorResolver struct{}

// ResolvePublicAddress for Tor returns the onion address as-is since
// onion addresses are inherently "public" within the Tor network
func (tr *TorResolver) ResolvePublicAddress(ctx context.Context, localAddr net.Addr) (net.Addr, error) {
	network := strings.ToLower(localAddr.Network())
	if !tr.SupportsNetwork(network) {
		return nil, fmt.Errorf("unsupported network type for Tor resolution: %s", network)
	}

	// For Tor addresses, the address is already the "public" address
	// within the Tor network context
	return localAddr, nil
}

// SupportsNetwork indicates support for Tor networks
func (tr *TorResolver) SupportsNetwork(network string) bool {
	network = strings.ToLower(network)
	return network == "tor" || network == "onion"
}

// GetResolverName returns the resolver name
func (tr *TorResolver) GetResolverName() string {
	return "Tor Resolver"
}

// I2PResolver handles public address resolution for I2P addresses
type I2PResolver struct{}

// ResolvePublicAddress for I2P returns the I2P address as-is since
// I2P addresses are inherently "public" within the I2P network
func (i2pr *I2PResolver) ResolvePublicAddress(ctx context.Context, localAddr net.Addr) (net.Addr, error) {
	network := strings.ToLower(localAddr.Network())
	if !i2pr.SupportsNetwork(network) {
		return nil, fmt.Errorf("unsupported network type for I2P resolution: %s", network)
	}

	// For I2P addresses, the address is already the "public" address
	// within the I2P network context
	return localAddr, nil
}

// SupportsNetwork indicates support for I2P networks
func (i2pr *I2PResolver) SupportsNetwork(network string) bool {
	network = strings.ToLower(network)
	return network == "i2p"
}

// GetResolverName returns the resolver name
func (i2pr *I2PResolver) GetResolverName() string {
	return "I2P Resolver"
}

// NymResolver handles public address resolution for Nym addresses
type NymResolver struct{}

// ResolvePublicAddress for Nym returns the Nym address as-is since
// Nym addresses are inherently "public" within the Nym mixnet
func (nr *NymResolver) ResolvePublicAddress(ctx context.Context, localAddr net.Addr) (net.Addr, error) {
	network := strings.ToLower(localAddr.Network())
	if !nr.SupportsNetwork(network) {
		return nil, fmt.Errorf("unsupported network type for Nym resolution: %s", network)
	}

	// For Nym addresses, the address is already the "public" address
	// within the Nym mixnet context
	return localAddr, nil
}

// SupportsNetwork indicates support for Nym networks
func (nr *NymResolver) SupportsNetwork(network string) bool {
	network = strings.ToLower(network)
	return network == "nym"
}

// GetResolverName returns the resolver name
func (nr *NymResolver) GetResolverName() string {
	return "Nym Resolver"
}

// LokiResolver handles public address resolution for Loki addresses
type LokiResolver struct{}

// ResolvePublicAddress for Loki returns the Loki address as-is since
// Loki addresses are inherently "public" within the Loki network
func (lr *LokiResolver) ResolvePublicAddress(ctx context.Context, localAddr net.Addr) (net.Addr, error) {
	network := strings.ToLower(localAddr.Network())
	if !lr.SupportsNetwork(network) {
		return nil, fmt.Errorf("unsupported network type for Loki resolution: %s", network)
	}

	// For Loki addresses, the address is already the "public" address
	// within the Loki network context
	return localAddr, nil
}

// SupportsNetwork indicates support for Loki networks
func (lr *LokiResolver) SupportsNetwork(network string) bool {
	network = strings.ToLower(network)
	return network == "loki"
}

// GetResolverName returns the resolver name
func (lr *LokiResolver) GetResolverName() string {
	return "Loki Resolver"
}
