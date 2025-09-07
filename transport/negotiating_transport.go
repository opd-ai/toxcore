package transport

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	// ErrVersionNegotiationFailed indicates protocol version negotiation failed
	ErrVersionNegotiationFailed = errors.New("version negotiation failed")
	// ErrUnsupportedProtocolVersion indicates peer uses unsupported protocol version
	ErrUnsupportedProtocolVersion = errors.New("unsupported protocol version")
)

// ProtocolCapabilities defines what protocol versions and features are supported
type ProtocolCapabilities struct {
	SupportedVersions    []ProtocolVersion
	PreferredVersion     ProtocolVersion
	EnableLegacyFallback bool
	NegotiationTimeout   time.Duration
}

// DefaultProtocolCapabilities returns sensible defaults for protocol capabilities
func DefaultProtocolCapabilities() *ProtocolCapabilities {
	return &ProtocolCapabilities{
		SupportedVersions:    []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
		PreferredVersion:     ProtocolNoiseIK,
		EnableLegacyFallback: true,
		NegotiationTimeout:   5 * time.Second,
	}
}

// NegotiatingTransport wraps a transport with automatic version negotiation
// and fallback capabilities for backward compatibility with legacy peers.
type NegotiatingTransport struct {
	underlying      Transport
	capabilities    *ProtocolCapabilities
	negotiator      *VersionNegotiator
	noiseTransport  *NoiseTransport
	peerVersions    map[string]ProtocolVersion // addr.String() -> version
	versionsMu      sync.RWMutex
	fallbackEnabled bool
}

// NewNegotiatingTransport creates a transport that handles version negotiation
// and automatic fallback to legacy protocols when needed.
func NewNegotiatingTransport(underlying Transport, capabilities *ProtocolCapabilities, staticPrivKey []byte) (*NegotiatingTransport, error) {
	if capabilities == nil {
		capabilities = DefaultProtocolCapabilities()
	}

	if len(capabilities.SupportedVersions) == 0 {
		return nil, errors.New("must support at least one protocol version")
	}

	negotiator := NewVersionNegotiator(capabilities.SupportedVersions, capabilities.PreferredVersion)

	var noiseTransport *NoiseTransport
	var err error

	// Only create noise transport if we support Noise-IK
	if negotiator.IsVersionSupported(ProtocolNoiseIK) {
		if len(staticPrivKey) != 32 {
			return nil, fmt.Errorf("static private key must be 32 bytes for Noise-IK, got %d", len(staticPrivKey))
		}
		noiseTransport, err = NewNoiseTransport(underlying, staticPrivKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create noise transport: %w", err)
		}
	}

	nt := &NegotiatingTransport{
		underlying:      underlying,
		capabilities:    capabilities,
		negotiator:      negotiator,
		noiseTransport:  noiseTransport,
		peerVersions:    make(map[string]ProtocolVersion),
		fallbackEnabled: capabilities.EnableLegacyFallback,
	}

	// Register handler for version negotiation packets
	underlying.RegisterHandler(PacketVersionNegotiation, nt.handleVersionNegotiation)

	return nt, nil
}

// Send sends a packet using the appropriate protocol version for the peer
func (nt *NegotiatingTransport) Send(packet *Packet, addr net.Addr) error {
	peerVersion := nt.getPeerVersion(addr)

	switch peerVersion {
	case ProtocolNoiseIK:
		if nt.noiseTransport == nil {
			return ErrUnsupportedProtocolVersion
		}
		return nt.noiseTransport.Send(packet, addr)

	case ProtocolLegacy:
		return nt.underlying.Send(packet, addr)

	default:
		// Unknown peer - attempt version negotiation
		negotiatedVersion, err := nt.negotiateWithPeer(addr)
		if err != nil {
			if nt.fallbackEnabled {
				// Log cryptographic downgrade for security monitoring
				logrus.WithFields(logrus.Fields{
					"peer":        addr.String(),
					"reason":      "negotiation_failed",
					"error":       err.Error(),
					"fallback_to": "legacy_encryption",
				}).Warn("Cryptographic downgrade: Using legacy encryption - peer does not support Noise-IK")
				
				// Fallback to legacy if negotiation fails
				nt.setPeerVersion(addr, ProtocolLegacy)
				return nt.underlying.Send(packet, addr)
			}
			return fmt.Errorf("version negotiation failed: %w", err)
		}

		// Log successful negotiation for security visibility
		logrus.WithFields(logrus.Fields{
			"peer":                addr.String(),
			"negotiated_version":  negotiatedVersion.String(),
			"security_level":      getSecurityLevel(negotiatedVersion),
		}).Info("Protocol negotiation successful")

		nt.setPeerVersion(addr, negotiatedVersion)
		return nt.Send(packet, addr) // Retry with negotiated version
	}
}

// Close shuts down the negotiating transport and all underlying transports
func (nt *NegotiatingTransport) Close() error {
	var err error

	// Close noise transport if it exists
	if nt.noiseTransport != nil {
		if closeErr := nt.noiseTransport.Close(); closeErr != nil {
			err = closeErr
		}
	}

	// Close underlying transport
	if closeErr := nt.underlying.Close(); closeErr != nil {
		err = closeErr
	}

	return err
}

// LocalAddr returns the local address from the underlying transport
func (nt *NegotiatingTransport) LocalAddr() net.Addr {
	return nt.underlying.LocalAddr()
}

// RegisterHandler registers a packet handler with the underlying transport
func (nt *NegotiatingTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
	nt.underlying.RegisterHandler(packetType, handler)
}

// SetPeerVersion manually sets the protocol version for a specific peer
// This is useful for cases where version is known through other means
func (nt *NegotiatingTransport) SetPeerVersion(addr net.Addr, version ProtocolVersion) {
	nt.setPeerVersion(addr, version)
}

// GetPeerVersion returns the known protocol version for a peer
func (nt *NegotiatingTransport) GetPeerVersion(addr net.Addr) ProtocolVersion {
	return nt.getPeerVersion(addr)
}

// AddNoiseKeyForPeer adds a known public key for Noise-IK handshakes
func (nt *NegotiatingTransport) AddNoiseKeyForPeer(addr net.Addr, publicKey []byte) error {
	if nt.noiseTransport == nil {
		return ErrUnsupportedProtocolVersion
	}
	return nt.noiseTransport.AddPeer(addr, publicKey)
}

// getPeerVersion retrieves the protocol version for a peer (thread-safe)
func (nt *NegotiatingTransport) getPeerVersion(addr net.Addr) ProtocolVersion {
	nt.versionsMu.RLock()
	defer nt.versionsMu.RUnlock()

	version, exists := nt.peerVersions[addr.String()]
	if !exists {
		// Return a sentinel value indicating unknown version
		return ProtocolVersion(255) // Use 255 as "unknown"
	}
	return version
}

// setPeerVersion stores the protocol version for a peer (thread-safe)
func (nt *NegotiatingTransport) setPeerVersion(addr net.Addr, version ProtocolVersion) {
	nt.versionsMu.Lock()
	defer nt.versionsMu.Unlock()
	nt.peerVersions[addr.String()] = version
}

// negotiateWithPeer performs version negotiation with a specific peer
func (nt *NegotiatingTransport) negotiateWithPeer(addr net.Addr) (ProtocolVersion, error) {
	return nt.negotiator.NegotiateProtocol(nt.underlying, addr)
}

// handleVersionNegotiation processes incoming version negotiation packets
func (nt *NegotiatingTransport) handleVersionNegotiation(packet *Packet, senderAddr net.Addr) error {
	vnPacket, err := ParseVersionNegotiation(packet.Data)
	if err != nil {
		return fmt.Errorf("failed to parse version negotiation packet: %w", err)
	}

	// Select best mutually supported version
	selectedVersion := nt.negotiator.SelectBestVersion(vnPacket.SupportedVersions)

	// Store the negotiated version for this peer
	nt.setPeerVersion(senderAddr, selectedVersion)

	// Send our version capabilities back
	responsePacket := &VersionNegotiationPacket{
		SupportedVersions: nt.capabilities.SupportedVersions,
		PreferredVersion:  selectedVersion, // Echo selected version
	}

	responseData, err := SerializeVersionNegotiation(responsePacket)
	if err != nil {
		return fmt.Errorf("failed to serialize version response: %w", err)
	}

	response := &Packet{
		PacketType: PacketVersionNegotiation,
		Data:       responseData,
	}

	return nt.underlying.Send(response, senderAddr)
}

// GetUnderlying returns the underlying transport for compatibility with
// components that need access to the concrete transport type.
// This method is provided for backward compatibility during the transition
// to interface-based transport handling.
func (nt *NegotiatingTransport) GetUnderlying() Transport {
	return nt.underlying
}

// getSecurityLevel returns a human-readable security level description
func getSecurityLevel(version ProtocolVersion) string {
	switch version {
	case ProtocolNoiseIK:
		return "high_forward_secrecy"
	case ProtocolLegacy:
		return "basic_nacl_encryption"
	default:
		return "unknown"
	}
}
