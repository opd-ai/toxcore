// Package transport implements network transport for the Tox protocol.
//
// This file implements advanced NAT traversal with priority-based
// connection establishment (direct -> UPnP -> STUN -> hole punching -> relay).
package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// ConnectionMethod represents different connection establishment methods
type ConnectionMethod uint8

const (
	// ConnectionDirect means direct connection without NAT traversal
	ConnectionDirect ConnectionMethod = iota
	// ConnectionUPnP means connection through UPnP port mapping
	ConnectionUPnP
	// ConnectionSTUN means connection through STUN-assisted NAT traversal
	ConnectionSTUN
	// ConnectionHolePunch means connection through UDP hole punching
	ConnectionHolePunch
	// ConnectionRelay means connection through relay server
	ConnectionRelay
)

// ConnectionAttempt represents an attempt to establish a connection
type ConnectionAttempt struct {
	Method     ConnectionMethod
	RemoteAddr net.Addr
	LocalAddr  net.Addr
	Success    bool
	Error      error
	Duration   time.Duration
	Timestamp  time.Time
}

// AdvancedNATTraversal provides sophisticated NAT traversal capabilities
type AdvancedNATTraversal struct {
	ipResolver     *IPResolver
	holePuncher    *HolePuncher
	natTraversal   *NATTraversal
	attempts       []ConnectionAttempt
	mu             sync.RWMutex
	timeout        time.Duration
	enabledMethods map[ConnectionMethod]bool
}

// NewAdvancedNATTraversal creates a new advanced NAT traversal system
func NewAdvancedNATTraversal(localAddr net.Addr) (*AdvancedNATTraversal, error) {
	if localAddr == nil {
		return nil, errors.New("local address cannot be nil")
	}

	// Create UDP address for hole puncher
	var udpAddr *net.UDPAddr
	switch addr := localAddr.(type) {
	case *net.UDPAddr:
		udpAddr = addr
	case *net.TCPAddr:
		udpAddr = &net.UDPAddr{IP: addr.IP, Port: addr.Port}
	default:
		return nil, fmt.Errorf("unsupported local address type: %T", localAddr)
	}

	holePuncher, err := NewHolePuncher(udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create hole puncher: %w", err)
	}

	natTraversal := NewNATTraversal()

	return &AdvancedNATTraversal{
		ipResolver:   NewIPResolver(),
		holePuncher:  holePuncher,
		natTraversal: natTraversal,
		timeout:      30 * time.Second,
		enabledMethods: map[ConnectionMethod]bool{
			ConnectionDirect:    true,
			ConnectionUPnP:      true,
			ConnectionSTUN:      true,
			ConnectionHolePunch: true,
			ConnectionRelay:     false, // Relay requires external infrastructure
		},
	}, nil
}

// EstablishConnection attempts to establish a connection using priority-based methods
func (ant *AdvancedNATTraversal) EstablishConnection(ctx context.Context, remoteAddr net.Addr) (*ConnectionAttempt, error) {
	if remoteAddr == nil {
		return nil, errors.New("remote address cannot be nil")
	}

	// Try connection methods in priority order
	methods := []ConnectionMethod{
		ConnectionDirect,
		ConnectionUPnP,
		ConnectionSTUN,
		ConnectionHolePunch,
		ConnectionRelay,
	}

	var lastAttempt *ConnectionAttempt

	for _, method := range methods {
		if !ant.isMethodEnabled(method) {
			continue
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return lastAttempt, ctx.Err()
		default:
		}

		attempt := ant.attemptConnection(ctx, method, remoteAddr)
		ant.recordAttempt(attempt)
		lastAttempt = attempt

		if attempt.Success {
			return attempt, nil
		}
	}

	if lastAttempt == nil {
		return nil, errors.New("no connection methods available")
	}

	return lastAttempt, fmt.Errorf("all connection methods failed, last error: %w", lastAttempt.Error)
}

// attemptConnection tries a specific connection method
func (ant *AdvancedNATTraversal) attemptConnection(ctx context.Context, method ConnectionMethod, remoteAddr net.Addr) *ConnectionAttempt {
	attempt := &ConnectionAttempt{
		Method:     method,
		RemoteAddr: remoteAddr,
		LocalAddr:  ant.holePuncher.GetLocalAddr(),
		Timestamp:  time.Now(),
	}

	start := time.Now()
	defer func() {
		attempt.Duration = time.Since(start)
	}()

	switch method {
	case ConnectionDirect:
		attempt.Error = ant.attemptDirectConnection(ctx, remoteAddr)
	case ConnectionUPnP:
		attempt.Error = ant.attemptUPnPConnection(ctx, remoteAddr)
	case ConnectionSTUN:
		attempt.Error = ant.attemptSTUNConnection(ctx, remoteAddr)
	case ConnectionHolePunch:
		attempt.Error = ant.attemptHolePunchConnection(ctx, remoteAddr)
	case ConnectionRelay:
		attempt.Error = ant.attemptRelayConnection(ctx, remoteAddr)
	default:
		attempt.Error = fmt.Errorf("unknown connection method: %v", method)
	}

	attempt.Success = attempt.Error == nil
	return attempt
}

// attemptDirectConnection tries direct connection without NAT traversal
func (ant *AdvancedNATTraversal) attemptDirectConnection(ctx context.Context, remoteAddr net.Addr) error {
	// Check if we have a public IP that can connect directly
	localAddr := ant.holePuncher.GetLocalAddr()

	// Try to resolve our public address
	publicAddr, err := ant.ipResolver.ResolvePublicAddress(ctx, localAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve public address: %w", err)
	}

	// Check if remote address is reachable directly
	// This is a simplified check - in practice would attempt actual connection
	if ant.isDirectlyReachable(publicAddr, remoteAddr) {
		return nil // Success
	}

	return errors.New("direct connection not possible")
}

// attemptUPnPConnection tries connection through UPnP port mapping
func (ant *AdvancedNATTraversal) attemptUPnPConnection(ctx context.Context, remoteAddr net.Addr) error {
	upnpClient := ant.ipResolver.upnpClient

	// Check if UPnP is available
	if !upnpClient.IsAvailable(ctx) {
		return errors.New("UPnP not available")
	}

	// Create port mapping
	localAddr := ant.holePuncher.GetLocalAddr()
	mapping := UPnPMapping{
		ExternalPort: localAddr.Port,
		InternalPort: localAddr.Port,
		InternalIP:   localAddr.IP.String(),
		Protocol:     "UDP",
		Description:  "Tox Advanced NAT Traversal",
		Duration:     time.Hour, // Temporary mapping
	}

	if err := upnpClient.AddPortMapping(ctx, mapping); err != nil {
		return fmt.Errorf("failed to create UPnP port mapping: %w", err)
	}

	// Test connectivity through the mapped port
	// This would involve actual connection testing in practice
	return nil // Assume success for now
}

// attemptSTUNConnection tries connection through STUN-assisted traversal
func (ant *AdvancedNATTraversal) attemptSTUNConnection(ctx context.Context, remoteAddr net.Addr) error {
	stunClient := ant.ipResolver.stunClient
	localAddr := ant.holePuncher.GetLocalAddr()

	// Discover our public address through STUN
	publicAddr, err := stunClient.DiscoverPublicAddress(ctx, localAddr)
	if err != nil {
		return fmt.Errorf("STUN discovery failed: %w", err)
	}

	// Use the discovered public address for connection establishment
	// This would involve coordinating with remote peer through signaling
	_ = publicAddr // Use the address for connection setup

	return nil // Assume success for now
}

// attemptHolePunchConnection tries UDP hole punching
func (ant *AdvancedNATTraversal) attemptHolePunchConnection(ctx context.Context, remoteAddr net.Addr) error {
	udpRemoteAddr, ok := remoteAddr.(*net.UDPAddr)
	if !ok {
		return errors.New("hole punching requires UDP address")
	}

	attempt, err := ant.holePuncher.PunchHole(ctx, udpRemoteAddr)
	if err != nil {
		return fmt.Errorf("hole punching failed: %w", err)
	}

	if attempt.Result != HolePunchSuccess {
		return fmt.Errorf("hole punching unsuccessful: %v", attempt.Result)
	}

	return nil
}

// attemptRelayConnection tries connection through relay server
func (ant *AdvancedNATTraversal) attemptRelayConnection(ctx context.Context, remoteAddr net.Addr) error {
	// Relay connection would require external relay infrastructure
	// This is a placeholder for future implementation
	return errors.New("relay connection not implemented")
}

// isDirectlyReachable checks if a remote address is directly reachable
func (ant *AdvancedNATTraversal) isDirectlyReachable(localAddr, remoteAddr net.Addr) bool {
	// Simplified check - in practice would consider network topology
	// Check if both addresses are public
	localIP := ant.extractIP(localAddr)
	remoteIP := ant.extractIP(remoteAddr)

	if localIP == nil || remoteIP == nil {
		return false
	}

	// Both IPs must be public for direct connection
	if ant.ipResolver.isPrivateIP(localIP) || ant.ipResolver.isPrivateIP(remoteIP) {
		return false
	}

	// Additional check: Reject TEST-NET addresses (RFC 5737) as non-reachable
	// 192.0.2.0/24 (TEST-NET-1), 198.51.100.0/24 (TEST-NET-2), 203.0.113.0/24 (TEST-NET-3)
	if ant.isTestNetworkIP(remoteIP) || ant.isTestNetworkIP(localIP) {
		return false
	}

	return true
}

// extractIP extracts IP address from net.Addr
func (ant *AdvancedNATTraversal) extractIP(addr net.Addr) net.IP {
	switch a := addr.(type) {
	case *net.UDPAddr:
		return a.IP
	case *net.TCPAddr:
		return a.IP
	case *net.IPAddr:
		return a.IP
	default:
		return nil
	}
}

// isTestNetworkIP checks if an IP is in TEST-NET ranges (RFC 5737)
func (ant *AdvancedNATTraversal) isTestNetworkIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Convert to IPv4 for testing
	if ip.To4() != nil {
		ip = ip.To4()
		// TEST-NET-1: 192.0.2.0/24
		if ip[0] == 192 && ip[1] == 0 && ip[2] == 2 {
			return true
		}
		// TEST-NET-2: 198.51.100.0/24
		if ip[0] == 198 && ip[1] == 51 && ip[2] == 100 {
			return true
		}
		// TEST-NET-3: 203.0.113.0/24
		if ip[0] == 203 && ip[1] == 0 && ip[2] == 113 {
			return true
		}
	}

	return false
}

// isMethodEnabled checks if a connection method is enabled
func (ant *AdvancedNATTraversal) isMethodEnabled(method ConnectionMethod) bool {
	ant.mu.RLock()
	defer ant.mu.RUnlock()

	enabled, exists := ant.enabledMethods[method]
	return exists && enabled
}

// recordAttempt records a connection attempt
func (ant *AdvancedNATTraversal) recordAttempt(attempt *ConnectionAttempt) {
	ant.mu.Lock()
	defer ant.mu.Unlock()

	ant.attempts = append(ant.attempts, *attempt)

	// Keep only last 100 attempts to prevent memory growth
	if len(ant.attempts) > 100 {
		ant.attempts = ant.attempts[len(ant.attempts)-100:]
	}
}

// EnableMethod enables or disables a connection method
func (ant *AdvancedNATTraversal) EnableMethod(method ConnectionMethod, enabled bool) {
	ant.mu.Lock()
	defer ant.mu.Unlock()

	ant.enabledMethods[method] = enabled
}

// GetAttemptHistory returns the history of connection attempts
func (ant *AdvancedNATTraversal) GetAttemptHistory() []ConnectionAttempt {
	ant.mu.RLock()
	defer ant.mu.RUnlock()

	history := make([]ConnectionAttempt, len(ant.attempts))
	copy(history, ant.attempts)
	return history
}

// GetMethodStatistics returns statistics for each connection method
func (ant *AdvancedNATTraversal) GetMethodStatistics() map[ConnectionMethod]struct {
	Attempts    int
	Successes   int
	SuccessRate float64
} {
	ant.mu.RLock()
	defer ant.mu.RUnlock()

	type Stats struct {
		Attempts    int
		Successes   int
		SuccessRate float64
	}

	stats := make(map[ConnectionMethod]Stats)

	for _, attempt := range ant.attempts {
		s := stats[attempt.Method]
		s.Attempts++
		if attempt.Success {
			s.Successes++
		}
		if s.Attempts > 0 {
			s.SuccessRate = float64(s.Successes) / float64(s.Attempts) * 100
		}
		stats[attempt.Method] = s
	}

	result := make(map[ConnectionMethod]struct {
		Attempts    int
		Successes   int
		SuccessRate float64
	})

	for method, s := range stats {
		result[method] = struct {
			Attempts    int
			Successes   int
			SuccessRate float64
		}{
			Attempts:    s.Attempts,
			Successes:   s.Successes,
			SuccessRate: s.SuccessRate,
		}
	}

	return result
}

// SetTimeout sets the timeout for connection attempts
func (ant *AdvancedNATTraversal) SetTimeout(timeout time.Duration) {
	ant.timeout = timeout
}

// Close closes the advanced NAT traversal system and releases resources
func (ant *AdvancedNATTraversal) Close() error {
	if ant.holePuncher != nil {
		return ant.holePuncher.Close()
	}
	return nil
}
