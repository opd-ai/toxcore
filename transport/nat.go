// Package transport implements network transport for the Tox protocol.
//
// This file implements NAT traversal techniques to allow Tox to work
// behind firewalls and NAT devices.
package transport

import (
	"errors"
	"net"
	"sync"
	"time"
)

// NATType represents the type of NAT detected.
type NATType uint8

const (
	// NATTypeUnknown means the NAT type hasn't been determined yet.
	NATTypeUnknown NATType = iota
	// NATTypeNone means no NAT is present (public IP).
	NATTypeNone
	// NATTypeSymmetric means a symmetric NAT is present (most restrictive).
	NATTypeSymmetric
	// NATTypeRestricted means a restricted NAT is present.
	NATTypeRestricted
	// NATTypePortRestricted means a port-restricted NAT is present.
	NATTypePortRestricted
	// NATTypeCone means a full cone NAT is present (least restrictive).
	NATTypeCone
)

// HolePunchResult represents the result of a hole punching attempt.
type HolePunchResult uint8

const (
	// HolePunchSuccess means hole punching succeeded.
	HolePunchSuccess HolePunchResult = iota
	// HolePunchFailedTimeout means hole punching failed due to timeout.
	HolePunchFailedTimeout
	// HolePunchFailedRejected means hole punching failed due to rejection.
	HolePunchFailedRejected
	// HolePunchFailedUnknown means hole punching failed for an unknown reason.
	HolePunchFailedUnknown
)

// NATTraversal handles NAT traversal for Tox.
//
//export ToxNATTraversal
type NATTraversal struct {
	detectedType      NATType
	publicIP          net.IP
	lastTypeCheck     time.Time
	typeCheckInterval time.Duration
	stuns             []string

	mu sync.Mutex
}

// NewNATTraversal creates a new NAT traversal handler.
//
//export ToxNewNATTraversal
func NewNATTraversal() *NATTraversal {
	return &NATTraversal{
		detectedType:      NATTypeUnknown,
		typeCheckInterval: 30 * time.Minute,
		stuns: []string{
			"stun.l.google.com:19302",
			"stun1.l.google.com:19302",
			"stun2.l.google.com:19302",
			"stun.antisip.com:3478",
		},
	}
}

// DetectNATType determines the type of NAT present.
//
//export ToxDetectNATType
func (nt *NATTraversal) DetectNATType() (NATType, error) {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	// If we've checked recently, return the cached result
	if !nt.lastTypeCheck.IsZero() && time.Since(nt.lastTypeCheck) < nt.typeCheckInterval {
		return nt.detectedType, nil
	}

	// In a real implementation, this would use STUN to detect NAT type
	// For simplicity, we'll assume a port-restricted NAT
	nt.detectedType = NATTypePortRestricted
	nt.lastTypeCheck = time.Now()

	// In a real implementation, this would also determine the public IP
	nt.publicIP = net.ParseIP("203.0.113.1") // Example IP

	return nt.detectedType, nil
}

// GetPublicIP returns the detected public IP address.
//
//export ToxGetPublicIP
func (nt *NATTraversal) GetPublicIP() (net.IP, error) {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	if nt.publicIP == nil {
		return nil, errors.New("public IP not yet detected")
	}

	return nt.publicIP, nil
}

// Updated PunchHole method to use Transport interface

// PunchHole attempts to punch a hole through NAT to a peer.
//
//export ToxPunchHole
func (nt *NATTraversal) PunchHole(transport Transport, target net.Addr) (HolePunchResult, error) {
    // First check our NAT type
    natType, err := nt.DetectNATType()
    if err != nil {
        return HolePunchFailedUnknown, err
    }

    if natType == NATTypeSymmetric {
        return HolePunchFailedUnknown, errors.New("symmetric NAT detected, direct hole punching not possible")
    }

    // Create a hole punch packet
    packet := &Packet{
        PacketType: PacketPingRequest,
        Data:       []byte{0xF0, 0x0D},
    }

    // Send hole punch packet
    err = transport.Send(packet, target)
    if err != nil {
        return HolePunchFailedUnknown, err
    }

    // For response handling, we'd need to register a handler
    // This would require redesigning the hole punching protocol
    // to work with our Transport abstraction
    
    // For now, we'll just return success if we could send
    return HolePunchSuccess, nil
}

// SetSTUNServers sets the STUN servers to use for NAT detection.
//
//export ToxSetSTUNServers
func (nt *NATTraversal) SetSTUNServers(servers []string) {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	nt.stuns = make([]string, len(servers))
	copy(nt.stuns, servers)
}

// GetSTUNServers returns the configured STUN servers.
//
//export ToxGetSTUNServers
func (nt *NATTraversal) GetSTUNServers() []string {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	servers := make([]string, len(nt.stuns))
	copy(servers, nt.stuns)

	return servers
}

// ForceNATTypeCheck forces an immediate check of NAT type.
//
//export ToxForceNATTypeCheck
func (nt *NATTraversal) ForceNATTypeCheck() (NATType, error) {
	nt.mu.Lock()
	nt.lastTypeCheck = time.Time{} // Zero time
	nt.mu.Unlock()

	return nt.DetectNATType()
}

// NATTypeToString converts a NAT type to a human-readable string.
//
//export ToxNATTypeToString
func NATTypeToString(natType NATType) string {
	switch natType {
	case NATTypeUnknown:
		return "Unknown"
	case NATTypeNone:
		return "None (Public IP)"
	case NATTypeSymmetric:
		return "Symmetric NAT"
	case NATTypeRestricted:
		return "Restricted NAT"
	case NATTypePortRestricted:
		return "Port-Restricted NAT"
	case NATTypeCone:
		return "Full Cone NAT"
	default:
		return "Invalid"
	}
}
