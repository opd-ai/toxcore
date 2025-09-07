// Package transport provides versioned handshake support for Tox protocol negotiation.
// This file implements the integration between protocol version negotiation
// and the Noise-IK handshake process, enabling backward compatibility.
package transport

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/opd-ai/toxcore/noise"
)

var (
	// ErrVersionMismatch indicates incompatible protocol versions
	ErrVersionMismatch = errors.New("protocol version mismatch")
	// ErrHandshakeTimeout indicates handshake took too long
	ErrHandshakeTimeout = errors.New("handshake timeout")
	// ErrInvalidVersionData indicates malformed version negotiation data
	ErrInvalidVersionData = errors.New("invalid version negotiation data")
)

// VersionedHandshakeRequest represents the initial handshake message with version info.
// This extends the basic handshake to include protocol capabilities.
type VersionedHandshakeRequest struct {
	// ProtocolVersion is the preferred protocol version
	ProtocolVersion ProtocolVersion
	// SupportedVersions lists all protocol versions we support
	SupportedVersions []ProtocolVersion
	// NoiseMessage contains the Noise handshake data (nil for legacy)
	NoiseMessage []byte
	// LegacyData contains legacy handshake data for backward compatibility
	LegacyData []byte
}

// VersionedHandshakeResponse represents the handshake response with agreed version.
type VersionedHandshakeResponse struct {
	// AgreedVersion is the protocol version both peers will use
	AgreedVersion ProtocolVersion
	// NoiseMessage contains the Noise handshake response (nil for legacy)
	NoiseMessage []byte
	// LegacyData contains legacy handshake response for backward compatibility
	LegacyData []byte
}

// SerializeVersionedHandshakeRequest converts a versioned handshake request to bytes.
// Wire format: [version(1)][num_supported(1)][supported_versions][noise_len(2)][noise_data][legacy_data]
func SerializeVersionedHandshakeRequest(req *VersionedHandshakeRequest) ([]byte, error) {
	if req == nil {
		return nil, errors.New("handshake request cannot be nil")
	}

	if len(req.SupportedVersions) == 0 {
		return nil, errors.New("must support at least one protocol version")
	}

	if len(req.SupportedVersions) > 255 {
		return nil, errors.New("too many supported versions (max 255)")
	}

	if len(req.NoiseMessage) > 65535 {
		return nil, errors.New("noise message too large (max 65535 bytes)")
	}

	// Calculate total size
	size := 1 + 1 + len(req.SupportedVersions) + 2 + len(req.NoiseMessage) + len(req.LegacyData)
	data := make([]byte, size)

	offset := 0

	// Write protocol version
	data[offset] = byte(req.ProtocolVersion)
	offset++

	// Write number of supported versions
	data[offset] = byte(len(req.SupportedVersions))
	offset++

	// Write supported versions
	for _, version := range req.SupportedVersions {
		data[offset] = byte(version)
		offset++
	}

	// Write noise message length (big-endian)
	noiseLen := len(req.NoiseMessage)
	data[offset] = byte(noiseLen >> 8)
	data[offset+1] = byte(noiseLen & 0xFF)
	offset += 2

	// Write noise message
	copy(data[offset:], req.NoiseMessage)
	offset += len(req.NoiseMessage)

	// Write legacy data (remaining bytes)
	copy(data[offset:], req.LegacyData)

	return data, nil
}

// ParseVersionedHandshakeRequest converts bytes to a versioned handshake request.
func ParseVersionedHandshakeRequest(data []byte) (*VersionedHandshakeRequest, error) {
	if len(data) < 4 { // minimum: version + num_versions + noise_len
		return nil, ErrInvalidVersionData
	}

	offset := 0

	// Read protocol version
	protocolVersion := ProtocolVersion(data[offset])
	offset++

	// Read number of supported versions
	numVersions := int(data[offset])
	offset++

	if len(data) < offset+numVersions+2 {
		return nil, ErrInvalidVersionData
	}

	// Read supported versions
	supportedVersions := make([]ProtocolVersion, numVersions)
	for i := 0; i < numVersions; i++ {
		supportedVersions[i] = ProtocolVersion(data[offset])
		offset++
	}

	// Read noise message length
	if len(data) < offset+2 {
		return nil, ErrInvalidVersionData
	}
	noiseLen := int(data[offset])<<8 | int(data[offset+1])
	offset += 2

	if len(data) < offset+noiseLen {
		return nil, ErrInvalidVersionData
	}

	// Read noise message
	var noiseMessage []byte
	if noiseLen > 0 {
		noiseMessage = make([]byte, noiseLen)
		copy(noiseMessage, data[offset:offset+noiseLen])
	}
	offset += noiseLen

	// Read legacy data (remaining bytes)
	var legacyData []byte
	if offset < len(data) {
		legacyData = make([]byte, len(data)-offset)
		copy(legacyData, data[offset:])
	}

	return &VersionedHandshakeRequest{
		ProtocolVersion:   protocolVersion,
		SupportedVersions: supportedVersions,
		NoiseMessage:      noiseMessage,
		LegacyData:        legacyData,
	}, nil
}

// SerializeVersionedHandshakeResponse converts a versioned handshake response to bytes.
// Wire format: [agreed_version(1)][noise_len(2)][noise_data][legacy_data]
func SerializeVersionedHandshakeResponse(resp *VersionedHandshakeResponse) ([]byte, error) {
	if resp == nil {
		return nil, errors.New("handshake response cannot be nil")
	}

	if len(resp.NoiseMessage) > 65535 {
		return nil, errors.New("noise message too large (max 65535 bytes)")
	}

	// Calculate total size
	size := 1 + 2 + len(resp.NoiseMessage) + len(resp.LegacyData)
	data := make([]byte, size)

	offset := 0

	// Write agreed version
	data[offset] = byte(resp.AgreedVersion)
	offset++

	// Write noise message length
	noiseLen := len(resp.NoiseMessage)
	data[offset] = byte(noiseLen >> 8)
	data[offset+1] = byte(noiseLen & 0xFF)
	offset += 2

	// Write noise message
	copy(data[offset:], resp.NoiseMessage)
	offset += len(resp.NoiseMessage)

	// Write legacy data
	copy(data[offset:], resp.LegacyData)

	return data, nil
}

// ParseVersionedHandshakeResponse converts bytes to a versioned handshake response.
func ParseVersionedHandshakeResponse(data []byte) (*VersionedHandshakeResponse, error) {
	if len(data) < 3 { // minimum: version + noise_len
		return nil, ErrInvalidVersionData
	}

	offset := 0

	// Read agreed version
	agreedVersion := ProtocolVersion(data[offset])
	offset++

	// Read noise message length
	noiseLen := int(data[offset])<<8 | int(data[offset+1])
	offset += 2

	if len(data) < offset+noiseLen {
		return nil, ErrInvalidVersionData
	}

	// Read noise message
	var noiseMessage []byte
	if noiseLen > 0 {
		noiseMessage = make([]byte, noiseLen)
		copy(noiseMessage, data[offset:offset+noiseLen])
	}
	offset += noiseLen

	// Read legacy data
	var legacyData []byte
	if offset < len(data) {
		legacyData = make([]byte, len(data)-offset)
		copy(legacyData, data[offset:])
	}

	return &VersionedHandshakeResponse{
		AgreedVersion: agreedVersion,
		NoiseMessage:  noiseMessage,
		LegacyData:    legacyData,
	}, nil
}

// VersionedHandshakeManager manages versioned handshakes for different protocols.
// This integrates version negotiation with the actual cryptographic handshake.
type VersionedHandshakeManager struct {
	staticPrivKey     [32]byte
	supportedVersions []ProtocolVersion
	preferredVersion  ProtocolVersion
	handshakeTimeout  time.Duration
}

// NewVersionedHandshakeManager creates a new versioned handshake manager.
// staticPrivKey is our long-term private key used for Noise-IK handshakes.
func NewVersionedHandshakeManager(staticPrivKey [32]byte, supportedVersions []ProtocolVersion, preferredVersion ProtocolVersion) *VersionedHandshakeManager {
	return &VersionedHandshakeManager{
		staticPrivKey:     staticPrivKey,
		supportedVersions: supportedVersions,
		preferredVersion:  preferredVersion,
		handshakeTimeout:  10 * time.Second,
	}
}

// InitiateHandshake starts a versioned handshake as the initiator.
// peerPubKey is the peer's long-term public key (required for Noise-IK).
func (vhm *VersionedHandshakeManager) InitiateHandshake(peerPubKey [32]byte, transport Transport, peerAddr net.Addr) (*VersionedHandshakeResponse, error) {
	var noiseMessage []byte
	var legacyData []byte

	// If we support Noise-IK, prepare the noise handshake
	if vhm.isVersionSupported(ProtocolNoiseIK) {
		noiseHandshake, err := noise.NewIKHandshake(vhm.staticPrivKey[:], peerPubKey[:], noise.Initiator)
		if err != nil {
			return nil, fmt.Errorf("failed to create noise handshake: %w", err)
		}

		noiseMessage, err = noiseHandshake.WriteMessage(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create noise handshake message: %w", err)
		}
	}

	// If we support legacy, prepare legacy handshake data
	if vhm.isVersionSupported(ProtocolLegacy) {
		// For now, legacy data is empty - this would be filled with
		// actual legacy handshake data in a complete implementation
		legacyData = []byte{}
	}

	// Create versioned handshake request
	request := &VersionedHandshakeRequest{
		ProtocolVersion:   vhm.preferredVersion,
		SupportedVersions: vhm.supportedVersions,
		NoiseMessage:      noiseMessage,
		LegacyData:        legacyData,
	}

	// Serialize and send the request
	data, err := SerializeVersionedHandshakeRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize handshake request: %w", err)
	}

	packet := &Packet{
		PacketType: PacketVersionedHandshake,
		Data:       data,
	}

	err = transport.Send(packet, peerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to send handshake request: %w", err)
	}

	// TODO: In a complete implementation, this would wait for the response
	// For now, return a mock response indicating successful negotiation
	return &VersionedHandshakeResponse{
		AgreedVersion: vhm.selectBestVersion(vhm.supportedVersions),
		NoiseMessage:  []byte{}, // Would contain actual noise response
		LegacyData:    []byte{},
	}, nil
}

// HandleHandshakeRequest processes an incoming versioned handshake request.
func (vhm *VersionedHandshakeManager) HandleHandshakeRequest(request *VersionedHandshakeRequest, peerAddr net.Addr) (*VersionedHandshakeResponse, error) {
	// Select the best mutually supported version
	agreedVersion := vhm.selectBestVersion(request.SupportedVersions)

	var responseNoiseMessage []byte
	var responseLegacyData []byte

	switch agreedVersion {
	case ProtocolNoiseIK:
		if len(request.NoiseMessage) == 0 {
			return nil, errors.New("noise message required for Noise-IK handshake")
		}

		// Create responder handshake
		noiseHandshake, err := noise.NewIKHandshake(vhm.staticPrivKey[:], nil, noise.Responder)
		if err != nil {
			return nil, fmt.Errorf("failed to create noise responder handshake: %w", err)
		}

		// Process the initiator's message and generate response
		_, err = noiseHandshake.ReadMessage(request.NoiseMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to process noise handshake message: %w", err)
		}

		responseNoiseMessage, err = noiseHandshake.WriteMessage(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create noise response: %w", err)
		}

	case ProtocolLegacy:
		// Handle legacy handshake
		responseLegacyData = []byte{} // Would contain actual legacy response

	default:
		return nil, ErrVersionMismatch
	}

	return &VersionedHandshakeResponse{
		AgreedVersion: agreedVersion,
		NoiseMessage:  responseNoiseMessage,
		LegacyData:    responseLegacyData,
	}, nil
}

// isVersionSupported checks if we support a specific protocol version.
func (vhm *VersionedHandshakeManager) isVersionSupported(version ProtocolVersion) bool {
	for _, supported := range vhm.supportedVersions {
		if supported == version {
			return true
		}
	}
	return false
}

// selectBestVersion chooses the highest mutually supported protocol version.
func (vhm *VersionedHandshakeManager) selectBestVersion(peerVersions []ProtocolVersion) ProtocolVersion {
	// Create map of our supported versions for fast lookup
	ourVersions := make(map[ProtocolVersion]bool)
	for _, version := range vhm.supportedVersions {
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
