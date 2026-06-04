package transport

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"golang.org/x/sync/singleflight"
)

// ProtocolVersion represents the version of the Tox protocol being used
type ProtocolVersion uint8

const (
	// ProtocolLegacy represents the original Tox protocol without Noise-IK
	ProtocolLegacy ProtocolVersion = 0
	// ProtocolNoiseIK represents the Noise-IK enhanced protocol
	ProtocolNoiseIK ProtocolVersion = 1
)

// Capability represents optional cryptographic features negotiated between peers.
type Capability uint8

const (
	// CapX3DH indicates support for X3DH initial key agreement.
	// When both peers advertise this capability, the initial session secret
	// is derived via X3DH (4-DH + HKDF) instead of Noise-IK.
	// Never downgrade once mutually supported.
	CapX3DH Capability = 1 << 0

	// CapHeaderEncryption indicates support for Double Ratchet header encryption.
	// When both peers advertise this capability, ratchet headers are encrypted
	// with XChaCha20-Poly1305 under a separate header key.
	// Never downgrade once mutually supported.
	CapHeaderEncryption Capability = 1 << 1

	// CapPQXDH indicates support for the post-quantum hybrid initial key agreement
	// (PQXDH: X3DH + ML-KEM-768). When both peers advertise this capability the
	// session root is derived from the hybrid transcript
	// SK = HKDF-SHA256(F ‖ DH1 ‖ DH2 ‖ DH3 [‖ DH4] ‖ SS_pq_spk [‖ SS_pq_opk]).
	// Never downgrade once mutually supported; the choice is bound into the
	// version-commitment HMAC.
	CapPQXDH Capability = 1 << 2
)

// CapMaxSecurity is the bitmask of all known security capabilities.
// It represents the maximum security level: X3DH + Header Encryption + PQXDH.
// opd-ai/toxcore advertises this by default and downgrades only when the
// peer is incapable of the required features.
const CapMaxSecurity = uint8(CapX3DH | CapHeaderEncryption | CapPQXDH)

// NegotiateCapabilities returns the intersection of our advertised capabilities
// and the peer's advertised capabilities.  Only features supported by both
// ends should be activated for a session.
func NegotiateCapabilities(ours, peers uint8) uint8 {
	return ours & peers
}

// maxSupportedVersions is the protocol maximum for the version count field in a
// VersionNegotiation packet. numVersions is a wire uint8 (max 255) and bounded
// 1:1 by the packet length, so this is not an amplification risk; the named
// constant exists as an explicit protocol sanity limit.
const maxSupportedVersions = 16

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
// Capabilities are negotiated as a bitmask and are signed to prevent capability stripping.
type SignedVersionNegotiationPacket struct {
	VersionNegotiationPacket
	SenderPublicKey [32]byte         // Ed25519 public key for signature verification
	Signature       crypto.Signature // Ed25519 signature over the version data and capabilities
	Capabilities    uint8            // Bitmask of supported capabilities (CapX3DH, CapHeaderEncryption, etc.)
}

// serializeVersionData converts version negotiation fields to bytes.
// This is the common serialization logic used by both signed and unsigned packets.
// If capabilitiesByte is provided (non-nil), it's prepended to the version data.
func serializeVersionData(preferredVersion ProtocolVersion, supportedVersions []ProtocolVersion, capabilitiesByte *uint8) []byte {
	baseSize := 2 + len(supportedVersions)
	var data []byte

	if capabilitiesByte != nil {
		// Format with capabilities: [capabilities(1)][preferred_version(1)][num_versions(1)][versions...]
		data = make([]byte, 1+baseSize)
		data[0] = *capabilitiesByte
		data[1] = byte(preferredVersion)
		data[2] = byte(len(supportedVersions))
		for i, version := range supportedVersions {
			data[3+i] = byte(version)
		}
	} else {
		// Format without capabilities: [preferred_version(1)][num_versions(1)][versions...]
		data = make([]byte, baseSize)
		data[0] = byte(preferredVersion)
		data[1] = byte(len(supportedVersions))
		for i, version := range supportedVersions {
			data[2+i] = byte(version)
		}
	}
	return data
}

// validateVersionPacket validates common version negotiation packet requirements.
func validateVersionPacket(supportedVersions []ProtocolVersion) error {
	if len(supportedVersions) == 0 {
		return errors.New("must support at least one protocol version")
	}
	return nil
}

// SerializeVersionNegotiation converts a version negotiation packet to bytes
func SerializeVersionNegotiation(packet *VersionNegotiationPacket) ([]byte, error) {
	if packet == nil {
		return nil, errors.New("packet cannot be nil")
	}
	if err := validateVersionPacket(packet.SupportedVersions); err != nil {
		return nil, err
	}
	return serializeVersionData(packet.PreferredVersion, packet.SupportedVersions, nil), nil
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

	if numVersions > maxSupportedVersions {
		return nil, fmt.Errorf("version count %d exceeds protocol maximum %d", numVersions, maxSupportedVersions)
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
// Legacy format: [public_key(32)][signature(64)][preferred_version(1)][num_versions(1)][versions...]
// Extended format: [public_key(32)][signature(64)][capabilities(1)][preferred_version(1)][num_versions(1)][versions...]
// The signature covers the version data portion (including capabilities when present).
func SerializeSignedVersionNegotiation(packet *SignedVersionNegotiationPacket, privateKey [32]byte) ([]byte, error) {
	if packet == nil {
		return nil, errors.New("packet cannot be nil")
	}
	if err := validateVersionPacket(packet.SupportedVersions); err != nil {
		return nil, err
	}

	// Preserve wire compatibility: omit capabilities byte when it is zero.
	var capabilitiesByte *uint8
	if packet.Capabilities != 0 {
		capabilitiesByte = &packet.Capabilities
	}
	versionData := serializeVersionData(packet.PreferredVersion, packet.SupportedVersions, capabilitiesByte)

	// Get Ed25519 public key from private key
	signingPubKey := crypto.GetSignaturePublicKey(privateKey)

	// Sign the version data (including capabilities)
	signature, err := crypto.Sign(versionData, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign version negotiation: %w", err)
	}

	// Combine: [public_key(32)][signature(64)][version_data_with_capabilities]
	result := make([]byte, 32+crypto.SignatureSize+len(versionData))
	copy(result[0:32], signingPubKey[:])
	copy(result[32:32+crypto.SignatureSize], signature[:])
	copy(result[32+crypto.SignatureSize:], versionData)

	return result, nil
}

// ParseSignedVersionNegotiation validates and parses a signed version negotiation packet.
// Accepts both legacy signed packets (without a capabilities byte) and extended
// packets (with a leading capabilities byte in signed version data).
// Returns an error if signature validation fails.
func ParseSignedVersionNegotiation(data []byte) (*SignedVersionNegotiationPacket, error) {
	// Minimum length (legacy): public_key(32) + signature(64) + preferred_version(1) + num_versions(1)
	minLen := 32 + crypto.SignatureSize + 2
	if len(data) < minLen {
		return nil, fmt.Errorf("signed version negotiation packet too short: %d < %d", len(data), minLen)
	}

	// Extract components
	var senderPubKey [32]byte
	copy(senderPubKey[:], data[0:32])

	var signature crypto.Signature
	copy(signature[:], data[32:32+crypto.SignatureSize])

	versionData := data[32+crypto.SignatureSize:]

	// Verify signature over raw signed version data.
	valid, err := crypto.Verify(versionData, signature, senderPubKey)
	if err != nil {
		return nil, NewFatalSecurityError(
			"signature_verification_error",
			"version_negotiation",
			"signature verification encountered an error",
			fmt.Errorf("signature verification error: %w", err),
		)
	}
	if !valid {
		return nil, NewFatalSecurityError(
			"signature_verification_failed",
			"version_negotiation",
			"version negotiation packet signature is invalid",
			errors.New("invalid signature on version negotiation packet"),
		)
	}

	// Try parsing legacy format first.
	legacyPacket, legacyErr := ParseVersionNegotiation(versionData)

	// Try parsing extended format: [capabilities][preferred][count][versions...]
	if len(versionData) >= 3 {
		capabilities := versionData[0]
		if extendedPacket, parseErr := ParseVersionNegotiation(versionData[1:]); parseErr == nil && (capabilities != 0 || legacyErr != nil) {
			return &SignedVersionNegotiationPacket{
				VersionNegotiationPacket: *extendedPacket,
				SenderPublicKey:          senderPubKey,
				Signature:                signature,
				Capabilities:             capabilities,
			}, nil
		}
	}

	// Fall back to legacy format: [preferred][count][versions...]
	if legacyErr != nil {
		return nil, fmt.Errorf("failed to parse version data: %w", legacyErr)
	}
	return &SignedVersionNegotiationPacket{
		VersionNegotiationPacket: *legacyPacket,
		SenderPublicKey:          senderPubKey,
		Signature:                signature,
		Capabilities:             0,
	}, nil
}

// HasCapability returns true if a specific capability is advertised in the packet.
func (p *SignedVersionNegotiationPacket) HasCapability(cap Capability) bool {
	return (p.Capabilities & uint8(cap)) != 0
}

// SetCapability sets a specific capability in the packet.
func (p *SignedVersionNegotiationPacket) SetCapability(cap Capability) {
	p.Capabilities |= uint8(cap)
}

// ClearCapability clears a specific capability in the packet.
func (p *SignedVersionNegotiationPacket) ClearCapability(cap Capability) {
	p.Capabilities &= ^uint8(cap)
}

// VersionNegotiator handles protocol version negotiation between peers
type VersionNegotiator struct {
	supportedVersions      []ProtocolVersion
	preferredVersion       ProtocolVersion
	negotiationTimeout     time.Duration
	pendingMu              sync.Mutex
	pending                map[string]chan ProtocolVersion // addr.String() -> response channel
	staticPrivateKey       [32]byte                        // Static key for signing version packets
	requireSignatures      bool                            // Whether to require signed version packets
	negotiationGroup       singleflight.Group              // Prevents concurrent negotiations for the same peer
	advertisedCapabilities uint8                           // Bitmask of capabilities we advertise to peers
}

// NewVersionNegotiator creates a new version negotiator with specified capabilities.
// This version does not sign packets - use NewSignedVersionNegotiator for signed negotiations.
// If supported is empty, it defaults to []ProtocolVersion{ProtocolLegacy} and
// preferred defaults to ProtocolLegacy (L-2).
func NewVersionNegotiator(supported []ProtocolVersion, preferred ProtocolVersion, timeout time.Duration) *VersionNegotiator {
	// Guard against empty slice to prevent panic on supported[0] fallback (L-2).
	if len(supported) == 0 {
		supported = []ProtocolVersion{ProtocolLegacy}
		preferred = ProtocolLegacy
	}

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

// NewSignedVersionNegotiatorWithCapabilities creates a version negotiator that signs all
// packets, requires signature verification, and advertises the given capability bitmask.
// This is the preferred constructor when using opd-ai/toxcore extensions such as X3DH,
// header encryption, and post-quantum PQXDH.
func NewSignedVersionNegotiatorWithCapabilities(supported []ProtocolVersion, preferred ProtocolVersion, timeout time.Duration, staticKey [32]byte, capabilities uint8) *VersionNegotiator {
	vn := NewSignedVersionNegotiator(supported, preferred, timeout, staticKey)
	vn.advertisedCapabilities = capabilities
	return vn
}

// AdvertisedCapabilities returns the capability bitmask this negotiator includes in
// every outgoing signed version-negotiation packet.
func (vn *VersionNegotiator) AdvertisedCapabilities() uint8 {
	return vn.advertisedCapabilities
}

// NegotiateProtocol performs version negotiation with a peer
// Returns the agreed protocol version or error if negotiation fails
// Uses singleflight to prevent concurrent negotiations for the same peer address
func (vn *VersionNegotiator) NegotiateProtocol(transport Transport, peerAddr net.Addr) (ProtocolVersion, error) {
	addrKey := peerAddr.String()

	// Use singleflight to prevent concurrent negotiations for the same peer
	// If another goroutine is already negotiating with this peer, we'll get the same result.
	// The `shared` return indicates whether this result was shared with another waiting caller.
	result, err, _ /* shared */ := vn.negotiationGroup.Do(addrKey, func() (interface{}, error) {
		return vn.performNegotiation(transport, peerAddr)
	})

	if err != nil {
		return ProtocolLegacy, err
	}

	negotiatedVersion, ok := result.(ProtocolVersion)
	if !ok {
		return ProtocolLegacy, fmt.Errorf("internal error: invalid negotiation result type")
	}

	return negotiatedVersion, nil
}

// performNegotiation performs the actual version negotiation for a peer
func (vn *VersionNegotiator) performNegotiation(transport Transport, peerAddr net.Addr) (ProtocolVersion, error) {
	packet, err := vn.createNegotiationPacket()
	if err != nil {
		return ProtocolLegacy, err
	}

	responseChan := vn.registerPendingNegotiation(peerAddr)
	defer vn.cleanupPendingNegotiation(peerAddr)

	if err := transport.Send(packet, peerAddr); err != nil {
		return ProtocolLegacy, fmt.Errorf("failed to send version negotiation: %w", err)
	}

	return vn.awaitNegotiationResponse(responseChan)
}

// createNegotiationPacket creates a version negotiation packet (signed or unsigned).
func (vn *VersionNegotiator) createNegotiationPacket() (*Packet, error) {
	vnPacket := &VersionNegotiationPacket{
		SupportedVersions: vn.supportedVersions,
		PreferredVersion:  vn.preferredVersion,
	}

	data, err := vn.serializeNegotiationPacket(vnPacket)
	if err != nil {
		return nil, err
	}

	return &Packet{
		PacketType: PacketVersionNegotiation,
		Data:       data,
	}, nil
}

// serializeNegotiationPacket serializes a negotiation packet with or without signatures.
func (vn *VersionNegotiator) serializeNegotiationPacket(vnPacket *VersionNegotiationPacket) ([]byte, error) {
	if vn.requireSignatures {
		signedPacket := &SignedVersionNegotiationPacket{
			VersionNegotiationPacket: *vnPacket,
			Capabilities:             vn.advertisedCapabilities,
		}
		data, err := SerializeSignedVersionNegotiation(signedPacket, vn.staticPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize signed version packet: %w", err)
		}
		return data, nil
	}

	data, err := SerializeVersionNegotiation(vnPacket)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize version packet: %w", err)
	}
	return data, nil
}

// registerPendingNegotiation registers a response channel for a peer negotiation.
func (vn *VersionNegotiator) registerPendingNegotiation(peerAddr net.Addr) chan ProtocolVersion {
	responseChan := make(chan ProtocolVersion, 1)
	addrKey := peerAddr.String()

	vn.pendingMu.Lock()
	vn.pending[addrKey] = responseChan
	vn.pendingMu.Unlock()

	return responseChan
}

// cleanupPendingNegotiation removes the pending negotiation entry for a peer.
func (vn *VersionNegotiator) cleanupPendingNegotiation(peerAddr net.Addr) {
	addrKey := peerAddr.String()
	vn.pendingMu.Lock()
	delete(vn.pending, addrKey)
	vn.pendingMu.Unlock()
}

// awaitNegotiationResponse waits for a negotiation response with timeout.
func (vn *VersionNegotiator) awaitNegotiationResponse(responseChan chan ProtocolVersion) (ProtocolVersion, error) {
	select {
	case negotiatedVersion := <-responseChan:
		return negotiatedVersion, nil
	case <-time.After(vn.negotiationTimeout):
		return ProtocolLegacy, fmt.Errorf("version negotiation timeout after %v", vn.negotiationTimeout)
	}
}

// SelectBestVersion chooses the highest mutually supported protocol version
func (vn *VersionNegotiator) SelectBestVersion(peerVersions []ProtocolVersion) ProtocolVersion {
	return selectBestSupportedVersion(vn.supportedVersions, peerVersions)
}

// IsVersionSupported checks if we support a specific protocol version
func (vn *VersionNegotiator) IsVersionSupported(version ProtocolVersion) bool {
	return supportsProtocolVersion(vn.supportedVersions, version)
}

// supportsProtocolVersion reports whether a version appears in the supported list.
func supportsProtocolVersion(supportedVersions []ProtocolVersion, version ProtocolVersion) bool {
	for _, supported := range supportedVersions {
		if supported == version {
			return true
		}
	}
	return false
}

// selectBestSupportedVersion picks the highest mutually supported protocol version.
func selectBestSupportedVersion(ourVersions, peerVersions []ProtocolVersion) ProtocolVersion {
	supportedSet := make(map[ProtocolVersion]bool)
	for _, version := range ourVersions {
		supportedSet[version] = true
	}
	bestVersion := ProtocolLegacy
	for _, peerVersion := range peerVersions {
		if supportedSet[peerVersion] && peerVersion > bestVersion {
			bestVersion = peerVersion
		}
	}
	return bestVersion
}

// handleResponse processes a version negotiation response from a peer.
// Returns true if a pending negotiation was found and satisfied (the packet was
// a response to our own request); returns false when this is a fresh inbound
// request from a peer that we have not yet initiated negotiation with.
// This distinction is used by the transport to avoid ping-pong: only fresh
// requests should be answered with a response packet (M-07 fix).
func (vn *VersionNegotiator) handleResponse(peerAddr net.Addr, peerVersions []ProtocolVersion) bool {
	addrKey := peerAddr.String()

	vn.pendingMu.Lock()
	responseChan, exists := vn.pending[addrKey]
	vn.pendingMu.Unlock()

	if !exists {
		// No pending negotiation for this peer
		return false
	}

	// Select best mutually supported version
	negotiatedVersion := vn.SelectBestVersion(peerVersions)

	// Send response to waiting goroutine
	select {
	case responseChan <- negotiatedVersion:
	default:
		// Channel already closed or full, ignore
	}
	return true
}

// ParseVersionPacket parses a version negotiation packet, handling both signed and unsigned formats.
// If requireSignatures is true, only accepts signed packets and returns an error for unsigned packets.
// Returns the parsed packet, the sender's public key (if signed, otherwise nil), the peer's
// advertised capability bitmask (0 for unsigned/legacy peers), and any error.
func (vn *VersionNegotiator) ParseVersionPacket(data []byte) (*VersionNegotiationPacket, *[32]byte, uint8, error) {
	// Minimum size for signed packet: 32 (key) + 64 (sig) + 2 (min version data) = 98
	// Minimum size for unsigned packet: 2 (min version data)
	minSignedLen := 32 + crypto.SignatureSize + 2

	if len(data) >= minSignedLen {
		// Try to parse as signed packet first
		signedPacket, err := ParseSignedVersionNegotiation(data)
		if err == nil {
			return &signedPacket.VersionNegotiationPacket, &signedPacket.SenderPublicKey, signedPacket.Capabilities, nil
		}
		// If signature verification failed and we require signatures, error out
		if vn.requireSignatures {
			return nil, nil, 0, fmt.Errorf("signature verification failed and signatures required: %w", err)
		}
	}

	// If signatures are required and packet is too short for a signed packet, reject
	if vn.requireSignatures {
		return nil, nil, 0, errors.New("signed version negotiation packet required but received unsigned packet")
	}

	// Parse as unsigned packet
	vnPacket, err := ParseVersionNegotiation(data)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to parse version negotiation: %w", err)
	}

	return vnPacket, nil, 0, nil
}

// RequiresSignatures returns whether this negotiator requires signed packets.
func (vn *VersionNegotiator) RequiresSignatures() bool {
	return vn.requireSignatures
}
