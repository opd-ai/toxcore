// Package transport provides versioned handshake support for Tox protocol negotiation.
// This file implements the integration between protocol version negotiation
// and the Noise-IK handshake process, enabling backward compatibility.
package transport

import (
	"errors"
	"fmt"
	"net"
	"sync"
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
	if err := validateHandshakeMinLength(data); err != nil {
		return nil, err
	}

	offset := 0
	protocolVersion, offset := readProtocolVersion(data, offset)
	supportedVersions, offset, err := readSupportedVersions(data, offset)
	if err != nil {
		return nil, err
	}

	noiseMessage, offset, err := readNoiseMessage(data, offset)
	if err != nil {
		return nil, err
	}

	legacyData := readLegacyData(data, offset)

	return &VersionedHandshakeRequest{
		ProtocolVersion:   protocolVersion,
		SupportedVersions: supportedVersions,
		NoiseMessage:      noiseMessage,
		LegacyData:        legacyData,
	}, nil
}

// validateHandshakeMinLength checks if data has minimum required length.
func validateHandshakeMinLength(data []byte) error {
	if len(data) < 4 {
		return ErrInvalidVersionData
	}
	return nil
}

// readProtocolVersion extracts the protocol version from data.
func readProtocolVersion(data []byte, offset int) (ProtocolVersion, int) {
	protocolVersion := ProtocolVersion(data[offset])
	return protocolVersion, offset + 1
}

// readSupportedVersions extracts the supported versions list from data.
func readSupportedVersions(data []byte, offset int) ([]ProtocolVersion, int, error) {
	numVersions := int(data[offset])
	offset++

	if len(data) < offset+numVersions+2 {
		return nil, offset, ErrInvalidVersionData
	}

	supportedVersions := make([]ProtocolVersion, numVersions)
	for i := 0; i < numVersions; i++ {
		supportedVersions[i] = ProtocolVersion(data[offset])
		offset++
	}

	return supportedVersions, offset, nil
}

// readNoiseMessage extracts the noise message from data.
func readNoiseMessage(data []byte, offset int) ([]byte, int, error) {
	if len(data) < offset+2 {
		return nil, offset, ErrInvalidVersionData
	}

	noiseLen := int(data[offset])<<8 | int(data[offset+1])
	offset += 2

	if len(data) < offset+noiseLen {
		return nil, offset, ErrInvalidVersionData
	}

	var noiseMessage []byte
	if noiseLen > 0 {
		noiseMessage = make([]byte, noiseLen)
		copy(noiseMessage, data[offset:offset+noiseLen])
	}
	offset += noiseLen

	return noiseMessage, offset, nil
}

// readLegacyData extracts any remaining legacy data from the packet.
func readLegacyData(data []byte, offset int) []byte {
	if offset < len(data) {
		legacyData := make([]byte, len(data)-offset)
		copy(legacyData, data[offset:])
		return legacyData
	}
	return nil
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

// pendingHandshake tracks an in-flight handshake waiting for response
type pendingHandshake struct {
	responseChan chan *VersionedHandshakeResponse
	errChan      chan error
}

// VersionedHandshakeManager manages versioned handshakes for different protocols.
// This integrates version negotiation with the actual cryptographic handshake.
type VersionedHandshakeManager struct {
	staticPrivKey     [32]byte
	supportedVersions []ProtocolVersion
	preferredVersion  ProtocolVersion
	handshakeTimeout  time.Duration
	pendingMu         sync.Mutex
	pending           map[string]*pendingHandshake
}

// NewVersionedHandshakeManager creates a new versioned handshake manager.
// staticPrivKey is our long-term private key used for Noise-IK handshakes.
func NewVersionedHandshakeManager(staticPrivKey [32]byte, supportedVersions []ProtocolVersion, preferredVersion ProtocolVersion) *VersionedHandshakeManager {
	return &VersionedHandshakeManager{
		staticPrivKey:     staticPrivKey,
		supportedVersions: supportedVersions,
		preferredVersion:  preferredVersion,
		handshakeTimeout:  10 * time.Second,
		pending:           make(map[string]*pendingHandshake),
	}
}

// InitiateHandshake starts a versioned handshake as the initiator.
// peerPubKey is the peer's long-term public key (required for Noise-IK).
// prepareNoiseHandshake creates a Noise-IK handshake message for the initiator.
// It returns the serialized message or nil if Noise-IK is not supported.
func (vhm *VersionedHandshakeManager) prepareNoiseHandshake(peerPubKey [32]byte) ([]byte, error) {
	if !vhm.isVersionSupported(ProtocolNoiseIK) {
		return nil, nil
	}

	noiseHandshake, err := noise.NewIKHandshake(vhm.staticPrivKey[:], peerPubKey[:], noise.Initiator)
	if err != nil {
		return nil, fmt.Errorf("failed to create noise handshake: %w", err)
	}

	message, _, err := noiseHandshake.WriteMessage(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create noise handshake message: %w", err)
	}

	return message, nil
}

// prepareLegacyHandshake creates legacy handshake data.
// It returns empty data if legacy protocol is not supported.
func (vhm *VersionedHandshakeManager) prepareLegacyHandshake() []byte {
	if !vhm.isVersionSupported(ProtocolLegacy) {
		return nil
	}
	return []byte{}
}

// createHandshakeRequest constructs a versioned handshake request with Noise and legacy data.
func (vhm *VersionedHandshakeManager) createHandshakeRequest(noiseMessage, legacyData []byte) *VersionedHandshakeRequest {
	return &VersionedHandshakeRequest{
		ProtocolVersion:   vhm.preferredVersion,
		SupportedVersions: vhm.supportedVersions,
		NoiseMessage:      noiseMessage,
		LegacyData:        legacyData,
	}
}

// registerPendingHandshake registers a pending handshake and returns cleanup function.
// The cleanup function must be called when the handshake completes or times out.
func (vhm *VersionedHandshakeManager) registerPendingHandshake(peerAddr net.Addr) (*pendingHandshake, func()) {
	addrKey := peerAddr.String()
	pending := &pendingHandshake{
		responseChan: make(chan *VersionedHandshakeResponse, 1),
		errChan:      make(chan error, 1),
	}

	vhm.pendingMu.Lock()
	vhm.pending[addrKey] = pending
	vhm.pendingMu.Unlock()

	cleanup := func() {
		vhm.pendingMu.Lock()
		delete(vhm.pending, addrKey)
		vhm.pendingMu.Unlock()
	}

	return pending, cleanup
}

// awaitHandshakeResponse waits for a handshake response or timeout.
// It returns the response or an error if the handshake fails or times out.
func (vhm *VersionedHandshakeManager) awaitHandshakeResponse(pending *pendingHandshake) (*VersionedHandshakeResponse, error) {
	select {
	case response := <-pending.responseChan:
		return response, nil
	case err := <-pending.errChan:
		return nil, err
	case <-time.After(vhm.handshakeTimeout):
		return nil, ErrHandshakeTimeout
	}
}

// InitiateHandshake starts a versioned protocol handshake with a peer.
// It prepares the Noise protocol message, creates a handshake request with
// legacy compatibility data, and sends it to the peer via the transport.
// The function blocks until a response is received or the handshake times out.
func (vhm *VersionedHandshakeManager) InitiateHandshake(peerPubKey [32]byte, transport Transport, peerAddr net.Addr) (*VersionedHandshakeResponse, error) {
	noiseMessage, err := vhm.prepareNoiseHandshake(peerPubKey)
	if err != nil {
		return nil, err
	}

	legacyData := vhm.prepareLegacyHandshake()
	request := vhm.createHandshakeRequest(noiseMessage, legacyData)

	data, err := SerializeVersionedHandshakeRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize handshake request: %w", err)
	}

	packet := &Packet{
		PacketType: PacketNoiseHandshake,
		Data:       data,
	}

	pending, cleanup := vhm.registerPendingHandshake(peerAddr)
	defer cleanup()

	transport.RegisterHandler(PacketNoiseHandshake, vhm.handleHandshakeResponse)

	err = transport.Send(packet, peerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to send handshake request: %w", err)
	}

	return vhm.awaitHandshakeResponse(pending)
}

// handleHandshakeResponse processes incoming handshake response packets
func (vhm *VersionedHandshakeManager) handleHandshakeResponse(packet *Packet, addr net.Addr) error {
	// Parse the response
	response, err := ParseVersionedHandshakeResponse(packet.Data)
	if err != nil {
		return fmt.Errorf("failed to parse handshake response: %w", err)
	}

	// Find the pending handshake for this address
	addrKey := addr.String()
	vhm.pendingMu.Lock()
	pending, exists := vhm.pending[addrKey]
	vhm.pendingMu.Unlock()

	if !exists {
		// No pending handshake for this address - might be unsolicited or expired
		return nil
	}

	// Send the response to the waiting goroutine
	select {
	case pending.responseChan <- response:
	default:
		// Channel already has a response or is closed
	}

	return nil
}

// HandleHandshakeRequest processes an incoming versioned handshake request.
// This should be called by the responder when receiving a handshake request.
// It generates the appropriate response and sends it back to the initiator.
func (vhm *VersionedHandshakeManager) HandleHandshakeRequest(request *VersionedHandshakeRequest, transport Transport, peerAddr net.Addr) (*VersionedHandshakeResponse, error) {
	agreedVersion := vhm.selectBestVersion(request.SupportedVersions)

	responseNoiseMessage, responseLegacyData, err := vhm.processHandshakeByVersion(agreedVersion, request)
	if err != nil {
		return nil, err
	}

	response := &VersionedHandshakeResponse{
		AgreedVersion: agreedVersion,
		NoiseMessage:  responseNoiseMessage,
		LegacyData:    responseLegacyData,
	}

	if err := vhm.sendHandshakeResponse(response, transport, peerAddr); err != nil {
		return nil, err
	}

	return response, nil
}

// processHandshakeByVersion processes handshake based on the agreed version.
func (vhm *VersionedHandshakeManager) processHandshakeByVersion(version ProtocolVersion, request *VersionedHandshakeRequest) ([]byte, []byte, error) {
	switch version {
	case ProtocolNoiseIK:
		return vhm.processNoiseHandshake(request)
	case ProtocolLegacy:
		return nil, []byte{}, nil
	default:
		return nil, nil, ErrVersionMismatch
	}
}

// processNoiseHandshake handles Noise-IK protocol handshake.
func (vhm *VersionedHandshakeManager) processNoiseHandshake(request *VersionedHandshakeRequest) ([]byte, []byte, error) {
	if len(request.NoiseMessage) == 0 {
		return nil, nil, errors.New("noise message required for Noise-IK handshake")
	}

	noiseHandshake, err := noise.NewIKHandshake(vhm.staticPrivKey[:], nil, noise.Responder)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create noise responder handshake: %w", err)
	}

	message, _, err := noiseHandshake.WriteMessage(nil, request.NoiseMessage)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to process noise handshake message: %w", err)
	}

	return message, nil, nil
}

// sendHandshakeResponse serializes and sends the handshake response.
func (vhm *VersionedHandshakeManager) sendHandshakeResponse(response *VersionedHandshakeResponse, transport Transport, peerAddr net.Addr) error {
	responseData, err := SerializeVersionedHandshakeResponse(response)
	if err != nil {
		return fmt.Errorf("failed to serialize handshake response: %w", err)
	}

	packet := &Packet{
		PacketType: PacketNoiseHandshake,
		Data:       responseData,
	}

	if err := transport.Send(packet, peerAddr); err != nil {
		return fmt.Errorf("failed to send handshake response: %w", err)
	}

	return nil
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

// GetSupportedVersions returns a copy of the supported protocol versions.
// This allows external code to inspect which protocol versions are supported.
func (vhm *VersionedHandshakeManager) GetSupportedVersions() []ProtocolVersion {
	versions := make([]ProtocolVersion, len(vhm.supportedVersions))
	copy(versions, vhm.supportedVersions)
	return versions
}
