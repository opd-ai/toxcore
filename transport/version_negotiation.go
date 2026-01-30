package transport

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// ProtocolVersion represents the version of the Tox protocol being used
type ProtocolVersion uint8

const (
	// ProtocolLegacy represents the original Tox protocol without Noise-IK
	ProtocolLegacy ProtocolVersion = 0
	// ProtocolNoiseIK represents the Noise-IK enhanced protocol
	ProtocolNoiseIK ProtocolVersion = 1
)

// String returns the human-readable name of the protocol version
func (v ProtocolVersion) String() string {
	switch v {
	case ProtocolLegacy:
		return "Legacy"
	case ProtocolNoiseIK:
		return "Noise-IK"
	default:
		return fmt.Sprintf("Unknown(%d)", v)
	}
}

// VersionNegotiationPacket represents a protocol version negotiation packet
type VersionNegotiationPacket struct {
	SupportedVersions []ProtocolVersion
	PreferredVersion  ProtocolVersion
}

// SerializeVersionNegotiation converts a version negotiation packet to bytes
func SerializeVersionNegotiation(packet *VersionNegotiationPacket) ([]byte, error) {
	if packet == nil {
		return nil, errors.New("packet cannot be nil")
	}

	if len(packet.SupportedVersions) == 0 {
		return nil, errors.New("must support at least one protocol version")
	}

	// Format: [preferred_version(1)][num_versions(1)][version1][version2]...
	data := make([]byte, 2+len(packet.SupportedVersions))
	data[0] = byte(packet.PreferredVersion)
	data[1] = byte(len(packet.SupportedVersions))

	for i, version := range packet.SupportedVersions {
		data[2+i] = byte(version)
	}

	return data, nil
}

// ParseVersionNegotiation converts bytes to a version negotiation packet
func ParseVersionNegotiation(data []byte) (*VersionNegotiationPacket, error) {
	if len(data) < 2 {
		return nil, errors.New("version negotiation packet too short")
	}

	preferredVersion := ProtocolVersion(data[0])
	numVersions := int(data[1])

	if len(data) != 2+numVersions {
		return nil, fmt.Errorf("expected %d bytes, got %d", 2+numVersions, len(data))
	}

	supportedVersions := make([]ProtocolVersion, numVersions)
	for i := 0; i < numVersions; i++ {
		supportedVersions[i] = ProtocolVersion(data[2+i])
	}

	return &VersionNegotiationPacket{
		SupportedVersions: supportedVersions,
		PreferredVersion:  preferredVersion,
	}, nil
}

// VersionNegotiator handles protocol version negotiation between peers
type VersionNegotiator struct {
	supportedVersions  []ProtocolVersion
	preferredVersion   ProtocolVersion
	negotiationTimeout time.Duration
	pendingMu          sync.Mutex
	pending            map[string]chan ProtocolVersion // addr.String() -> response channel
}

// NewVersionNegotiator creates a new version negotiator with specified capabilities
func NewVersionNegotiator(supported []ProtocolVersion, preferred ProtocolVersion, timeout time.Duration) *VersionNegotiator {
	// Validate that preferred version is in supported list
	preferredSupported := false
	for _, version := range supported {
		if version == preferred {
			preferredSupported = true
			break
		}
	}

	if !preferredSupported {
		// Fallback to first supported version
		preferred = supported[0]
	}

	// Use default timeout if zero value provided
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	return &VersionNegotiator{
		supportedVersions:  supported,
		preferredVersion:   preferred,
		negotiationTimeout: timeout,
		pending:            make(map[string]chan ProtocolVersion),
	}
}

// NegotiateProtocol performs version negotiation with a peer
// Returns the agreed protocol version or error if negotiation fails
func (vn *VersionNegotiator) NegotiateProtocol(transport Transport, peerAddr net.Addr) (ProtocolVersion, error) {
	// Create version negotiation packet
	vnPacket := &VersionNegotiationPacket{
		SupportedVersions: vn.supportedVersions,
		PreferredVersion:  vn.preferredVersion,
	}

	// Serialize the packet
	data, err := SerializeVersionNegotiation(vnPacket)
	if err != nil {
		return ProtocolLegacy, fmt.Errorf("failed to serialize version packet: %w", err)
	}

	// Create transport packet with new version negotiation type
	packet := &Packet{
		PacketType: PacketVersionNegotiation,
		Data:       data,
	}

	// Create response channel for this peer
	responseChan := make(chan ProtocolVersion, 1)
	addrKey := peerAddr.String()

	vn.pendingMu.Lock()
	vn.pending[addrKey] = responseChan
	vn.pendingMu.Unlock()

	// Clean up pending entry when done
	defer func() {
		vn.pendingMu.Lock()
		delete(vn.pending, addrKey)
		vn.pendingMu.Unlock()
	}()

	// Send version negotiation request
	err = transport.Send(packet, peerAddr)
	if err != nil {
		return ProtocolLegacy, fmt.Errorf("failed to send version negotiation: %w", err)
	}

	// Wait for peer response with timeout
	select {
	case negotiatedVersion := <-responseChan:
		return negotiatedVersion, nil
	case <-time.After(vn.negotiationTimeout):
		return ProtocolLegacy, fmt.Errorf("version negotiation timeout after %v", vn.negotiationTimeout)
	}
}

// SelectBestVersion chooses the highest mutually supported protocol version
func (vn *VersionNegotiator) SelectBestVersion(peerVersions []ProtocolVersion) ProtocolVersion {
	// Create map of our supported versions for fast lookup
	ourVersions := make(map[ProtocolVersion]bool)
	for _, version := range vn.supportedVersions {
		ourVersions[version] = true
	}

	// Find highest mutually supported version
	var bestVersion ProtocolVersion = ProtocolLegacy
	for _, peerVersion := range peerVersions {
		if ourVersions[peerVersion] && peerVersion > bestVersion {
			bestVersion = peerVersion
		}
	}

	return bestVersion
}

// IsVersionSupported checks if we support a specific protocol version
func (vn *VersionNegotiator) IsVersionSupported(version ProtocolVersion) bool {
	for _, supported := range vn.supportedVersions {
		if supported == version {
			return true
		}
	}
	return false
}

// handleResponse processes a version negotiation response from a peer
// This should be called by the transport layer when a response is received
func (vn *VersionNegotiator) handleResponse(peerAddr net.Addr, peerVersions []ProtocolVersion) {
	addrKey := peerAddr.String()

	vn.pendingMu.Lock()
	responseChan, exists := vn.pending[addrKey]
	vn.pendingMu.Unlock()

	if !exists {
		// No pending negotiation for this peer
		return
	}

	// Select best mutually supported version
	negotiatedVersion := vn.SelectBestVersion(peerVersions)

	// Send response to waiting goroutine
	select {
	case responseChan <- negotiatedVersion:
	default:
		// Channel already closed or full, ignore
	}
}
