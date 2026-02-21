package transport

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
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

// SignedVersionNegotiationPacket extends VersionNegotiationPacket with authentication.
// The signature prevents MITM downgrade attacks by cryptographically binding
// the version negotiation to the sender's static key.
type SignedVersionNegotiationPacket struct {
	VersionNegotiationPacket
	SenderPublicKey [32]byte         // Ed25519 public key for signature verification
	Signature       crypto.Signature // Ed25519 signature over the version data
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

// SerializeSignedVersionNegotiation creates a signed version negotiation packet.
// Format: [public_key(32)][signature(64)][preferred_version(1)][num_versions(1)][versions...]
// The signature covers only the version data portion.
func SerializeSignedVersionNegotiation(packet *SignedVersionNegotiationPacket, privateKey [32]byte) ([]byte, error) {
	if packet == nil {
		return nil, errors.New("packet cannot be nil")
	}

	if len(packet.SupportedVersions) == 0 {
		return nil, errors.New("must support at least one protocol version")
	}

	// Serialize the unsigned version data
	versionData := make([]byte, 2+len(packet.SupportedVersions))
	versionData[0] = byte(packet.PreferredVersion)
	versionData[1] = byte(len(packet.SupportedVersions))
	for i, version := range packet.SupportedVersions {
		versionData[2+i] = byte(version)
	}

	// Get Ed25519 public key from private key
	signingPubKey := crypto.GetSignaturePublicKey(privateKey)

	// Sign the version data
	signature, err := crypto.Sign(versionData, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign version negotiation: %w", err)
	}

	// Combine: [public_key(32)][signature(64)][version_data]
	result := make([]byte, 32+crypto.SignatureSize+len(versionData))
	copy(result[0:32], signingPubKey[:])
	copy(result[32:32+crypto.SignatureSize], signature[:])
	copy(result[32+crypto.SignatureSize:], versionData)

	return result, nil
}

// ParseSignedVersionNegotiation validates and parses a signed version negotiation packet.
// Returns an error if the signature is invalid.
func ParseSignedVersionNegotiation(data []byte) (*SignedVersionNegotiationPacket, error) {
	minLen := 32 + crypto.SignatureSize + 2 // public_key + signature + min version data
	if len(data) < minLen {
		return nil, fmt.Errorf("signed version negotiation packet too short: %d < %d", len(data), minLen)
	}

	// Extract components
	var senderPubKey [32]byte
	copy(senderPubKey[:], data[0:32])

	var signature crypto.Signature
	copy(signature[:], data[32:32+crypto.SignatureSize])

	versionData := data[32+crypto.SignatureSize:]

	// Verify signature before parsing
	valid, err := crypto.Verify(versionData, signature, senderPubKey)
	if err != nil {
		return nil, fmt.Errorf("signature verification error: %w", err)
	}
	if !valid {
		return nil, errors.New("invalid signature on version negotiation packet")
	}

	// Parse the version data
	vnPacket, err := ParseVersionNegotiation(versionData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version data: %w", err)
	}

	return &SignedVersionNegotiationPacket{
		VersionNegotiationPacket: *vnPacket,
		SenderPublicKey:          senderPubKey,
		Signature:                signature,
	}, nil
}

// VersionNegotiator handles protocol version negotiation between peers
type VersionNegotiator struct {
	supportedVersions  []ProtocolVersion
	preferredVersion   ProtocolVersion
	negotiationTimeout time.Duration
	pendingMu          sync.Mutex
	pending            map[string]chan ProtocolVersion // addr.String() -> response channel
	staticPrivateKey   [32]byte                        // Static key for signing version packets
	requireSignatures  bool                            // Whether to require signed version packets
}

// NewVersionNegotiator creates a new version negotiator with specified capabilities.
// This version does not sign packets - use NewSignedVersionNegotiator for signed negotiations.
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
		requireSignatures:  false,
	}
}

// NewSignedVersionNegotiator creates a version negotiator that signs all packets
// and requires signature verification on incoming packets.
func NewSignedVersionNegotiator(supported []ProtocolVersion, preferred ProtocolVersion, timeout time.Duration, staticKey [32]byte) *VersionNegotiator {
	vn := NewVersionNegotiator(supported, preferred, timeout)
	vn.staticPrivateKey = staticKey
	vn.requireSignatures = true
	return vn
}

// NegotiateProtocol performs version negotiation with a peer
// Returns the agreed protocol version or error if negotiation fails
func (vn *VersionNegotiator) NegotiateProtocol(transport Transport, peerAddr net.Addr) (ProtocolVersion, error) {
	// Create version negotiation packet
	vnPacket := &VersionNegotiationPacket{
		SupportedVersions: vn.supportedVersions,
		PreferredVersion:  vn.preferredVersion,
	}

	var data []byte
	var err error

	// Use signed packets if signatures are required
	if vn.requireSignatures {
		signedPacket := &SignedVersionNegotiationPacket{
			VersionNegotiationPacket: *vnPacket,
		}
		data, err = SerializeSignedVersionNegotiation(signedPacket, vn.staticPrivateKey)
		if err != nil {
			return ProtocolLegacy, fmt.Errorf("failed to serialize signed version packet: %w", err)
		}
	} else {
		data, err = SerializeVersionNegotiation(vnPacket)
		if err != nil {
			return ProtocolLegacy, fmt.Errorf("failed to serialize version packet: %w", err)
		}
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

// ParseVersionPacket parses a version negotiation packet, handling both signed and unsigned formats.
// If requireSignatures is true, only accepts signed packets and returns an error for unsigned packets.
// Returns the parsed packet and the sender's public key (if signed, otherwise nil).
func (vn *VersionNegotiator) ParseVersionPacket(data []byte) (*VersionNegotiationPacket, *[32]byte, error) {
	// Minimum size for signed packet: 32 (key) + 64 (sig) + 2 (min version data) = 98
	// Minimum size for unsigned packet: 2 (min version data)
	minSignedLen := 32 + crypto.SignatureSize + 2

	if len(data) >= minSignedLen {
		// Try to parse as signed packet first
		signedPacket, err := ParseSignedVersionNegotiation(data)
		if err == nil {
			return &signedPacket.VersionNegotiationPacket, &signedPacket.SenderPublicKey, nil
		}
		// If signature verification failed and we require signatures, error out
		if vn.requireSignatures {
			return nil, nil, fmt.Errorf("signature verification failed and signatures required: %w", err)
		}
	}

	// If signatures are required and packet is too short for a signed packet, reject
	if vn.requireSignatures {
		return nil, nil, errors.New("signed version negotiation packet required but received unsigned packet")
	}

	// Parse as unsigned packet
	vnPacket, err := ParseVersionNegotiation(data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse version negotiation: %w", err)
	}

	return vnPacket, nil, nil
}

// RequiresSignatures returns whether this negotiator requires signed packets.
func (vn *VersionNegotiator) RequiresSignatures() bool {
	return vn.requireSignatures
}
