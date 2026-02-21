package transport

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

// VersionCommitment provides cryptographic binding between protocol version
// selection and the Noise handshake. This prevents version rollback attacks
// where an attacker could cause peers to disagree about the negotiated version.
//
// After Noise-IK handshake completes, both peers exchange VersionCommitment
// messages encrypted under the session keys. Each commitment binds the
// selected version to the handshake by including an HMAC over the version
// and handshake transcript hash.
type VersionCommitment struct {
	// Version is the agreed protocol version
	Version ProtocolVersion
	// Timestamp is when the commitment was created (Unix seconds)
	Timestamp int64
	// HMAC is the commitment MAC binding version to handshake
	// Computed as HMAC-SHA256(handshake_hash, version || timestamp)
	HMAC [32]byte
}

// CommitmentMaxAge is the maximum age for a valid version commitment (5 minutes)
const CommitmentMaxAge = 5 * time.Minute

// CommitmentMaxFutureDrift is the maximum future drift allowed (1 minute)
const CommitmentMaxFutureDrift = 1 * time.Minute

var (
	// ErrCommitmentVersionMismatch indicates committed version doesn't match local selection
	ErrCommitmentVersionMismatch = errors.New("version commitment mismatch: versions differ")
	// ErrInvalidCommitmentMAC indicates HMAC verification failed
	ErrInvalidCommitmentMAC = errors.New("version commitment MAC verification failed")
	// ErrCommitmentTooOld indicates commitment timestamp is too old
	ErrCommitmentTooOld = errors.New("version commitment timestamp too old")
	// ErrCommitmentFromFuture indicates commitment timestamp is from the future
	ErrCommitmentFromFuture = errors.New("version commitment timestamp from future")
)

// CreateVersionCommitment creates a new version commitment binding the
// selected protocol version to the Noise handshake via HMAC.
func CreateVersionCommitment(version ProtocolVersion, handshakeHash []byte) (*VersionCommitment, error) {
	if len(handshakeHash) == 0 {
		return nil, errors.New("handshake hash cannot be empty")
	}

	commitment := &VersionCommitment{
		Version:   version,
		Timestamp: time.Now().Unix(),
	}

	// Compute HMAC binding version to handshake
	mac := computeCommitmentMAC(commitment.Version, commitment.Timestamp, handshakeHash)
	copy(commitment.HMAC[:], mac)

	return commitment, nil
}

// computeCommitmentMAC computes HMAC-SHA256(handshakeHash, version || timestamp)
func computeCommitmentMAC(version ProtocolVersion, timestamp int64, handshakeHash []byte) []byte {
	h := hmac.New(sha256.New, handshakeHash)

	// Write version byte
	h.Write([]byte{byte(version)})

	// Write timestamp (8 bytes big-endian)
	tsBytes := make([]byte, 8)
	tsBytes[0] = byte(timestamp >> 56)
	tsBytes[1] = byte(timestamp >> 48)
	tsBytes[2] = byte(timestamp >> 40)
	tsBytes[3] = byte(timestamp >> 32)
	tsBytes[4] = byte(timestamp >> 24)
	tsBytes[5] = byte(timestamp >> 16)
	tsBytes[6] = byte(timestamp >> 8)
	tsBytes[7] = byte(timestamp)
	h.Write(tsBytes)

	return h.Sum(nil)
}

// SerializeVersionCommitment converts a commitment to bytes for transmission.
// Format: [version(1)][timestamp(8)][hmac(32)] = 41 bytes total
func SerializeVersionCommitment(c *VersionCommitment) ([]byte, error) {
	if c == nil {
		return nil, errors.New("commitment cannot be nil")
	}

	data := make([]byte, 41)

	// Version (1 byte)
	data[0] = byte(c.Version)

	// Timestamp (8 bytes big-endian)
	data[1] = byte(c.Timestamp >> 56)
	data[2] = byte(c.Timestamp >> 48)
	data[3] = byte(c.Timestamp >> 40)
	data[4] = byte(c.Timestamp >> 32)
	data[5] = byte(c.Timestamp >> 24)
	data[6] = byte(c.Timestamp >> 16)
	data[7] = byte(c.Timestamp >> 8)
	data[8] = byte(c.Timestamp)

	// HMAC (32 bytes)
	copy(data[9:41], c.HMAC[:])

	return data, nil
}

// ParseVersionCommitment parses bytes into a VersionCommitment.
func ParseVersionCommitment(data []byte) (*VersionCommitment, error) {
	if len(data) != 41 {
		return nil, fmt.Errorf("version commitment must be 41 bytes, got %d", len(data))
	}

	c := &VersionCommitment{
		Version: ProtocolVersion(data[0]),
	}

	// Parse timestamp (8 bytes big-endian)
	c.Timestamp = int64(data[1])<<56 |
		int64(data[2])<<48 |
		int64(data[3])<<40 |
		int64(data[4])<<32 |
		int64(data[5])<<24 |
		int64(data[6])<<16 |
		int64(data[7])<<8 |
		int64(data[8])

	// Copy HMAC
	copy(c.HMAC[:], data[9:41])

	return c, nil
}

// VerifyVersionCommitment validates a received commitment against local state.
// Returns nil if the commitment is valid, or an error describing the failure.
func VerifyVersionCommitment(commitment *VersionCommitment, expectedVersion ProtocolVersion, handshakeHash []byte) error {
	if commitment == nil {
		return errors.New("commitment cannot be nil")
	}

	// Check version matches our selection
	if commitment.Version != expectedVersion {
		return fmt.Errorf("%w: expected %s, got %s",
			ErrCommitmentVersionMismatch, expectedVersion, commitment.Version)
	}

	// Check timestamp freshness
	now := time.Now().Unix()

	age := time.Duration(now-commitment.Timestamp) * time.Second
	if age > CommitmentMaxAge {
		return fmt.Errorf("%w: age %v exceeds %v",
			ErrCommitmentTooOld, age, CommitmentMaxAge)
	}

	futureTime := time.Duration(commitment.Timestamp-now) * time.Second
	if futureTime > CommitmentMaxFutureDrift {
		return fmt.Errorf("%w: %v in future exceeds drift %v",
			ErrCommitmentFromFuture, futureTime, CommitmentMaxFutureDrift)
	}

	// Verify HMAC
	expectedMAC := computeCommitmentMAC(commitment.Version, commitment.Timestamp, handshakeHash)
	if !hmac.Equal(commitment.HMAC[:], expectedMAC) {
		return ErrInvalidCommitmentMAC
	}

	return nil
}

// VersionCommitmentExchange handles the full commitment exchange protocol.
// This is used by NoiseTransport to verify version agreement after handshake.
type VersionCommitmentExchange struct {
	localVersion    ProtocolVersion
	handshakeHash   []byte
	localCommitment *VersionCommitment
	peerCommitment  *VersionCommitment
	verified        bool
}

// NewVersionCommitmentExchange creates a new exchange handler.
func NewVersionCommitmentExchange(version ProtocolVersion, handshakeHash []byte) (*VersionCommitmentExchange, error) {
	commitment, err := CreateVersionCommitment(version, handshakeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to create local commitment: %w", err)
	}

	return &VersionCommitmentExchange{
		localVersion:    version,
		handshakeHash:   handshakeHash,
		localCommitment: commitment,
		verified:        false,
	}, nil
}

// GetLocalCommitment returns the serialized local commitment to send to peer.
func (vce *VersionCommitmentExchange) GetLocalCommitment() ([]byte, error) {
	return SerializeVersionCommitment(vce.localCommitment)
}

// ProcessPeerCommitment validates the peer's commitment and marks exchange as complete.
func (vce *VersionCommitmentExchange) ProcessPeerCommitment(data []byte) error {
	commitment, err := ParseVersionCommitment(data)
	if err != nil {
		return fmt.Errorf("failed to parse peer commitment: %w", err)
	}

	if err := VerifyVersionCommitment(commitment, vce.localVersion, vce.handshakeHash); err != nil {
		return fmt.Errorf("peer commitment verification failed: %w", err)
	}

	vce.peerCommitment = commitment
	vce.verified = true
	return nil
}

// IsVerified returns true if the peer's commitment has been verified.
func (vce *VersionCommitmentExchange) IsVerified() bool {
	return vce.verified
}

// GetAgreedVersion returns the verified protocol version after successful exchange.
func (vce *VersionCommitmentExchange) GetAgreedVersion() (ProtocolVersion, error) {
	if !vce.verified {
		return ProtocolLegacy, errors.New("version commitment exchange not complete")
	}
	return vce.localVersion, nil
}
