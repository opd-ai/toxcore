package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

// ToxID represents a Tox identifier, consisting of a public key, nospam value, and checksum.
//
//export ToxID
type ToxID struct {
	PublicKey [KeySize]byte
	Nospam    [ToxIDNospamSize]byte
	Checksum  [ToxIDChecksumSize]byte
}

// NewToxID creates a ToxID from a public key and nospam value.
//
//export ToxIDNew
func NewToxID(publicKey [KeySize]byte, nospam [ToxIDNospamSize]byte) *ToxID {
	id := &ToxID{
		PublicKey: publicKey,
		Nospam:    nospam,
	}
	id.calculateChecksum()
	return id
}

// ToxIDFromString parses a Tox ID from its hexadecimal string representation.
//
//export ToxIDFromString
func ToxIDFromString(s string) (*ToxID, error) {
	// ToxID is 38 bytes (76 hex chars): 32 for public key + 4 for nospam + 2 for checksum
	if len(s) != ToxIDHexLength {
		return nil, errors.New("invalid Tox ID length")
	}

	data, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}

	id := &ToxID{}
	copy(id.PublicKey[:], data[0:KeySize])
	copy(id.Nospam[:], data[KeySize:KeySize+ToxIDNospamSize])
	copy(id.Checksum[:], data[KeySize+ToxIDNospamSize:ToxIDSize])

	// Verify checksum
	expectedID := &ToxID{
		PublicKey: id.PublicKey,
		Nospam:    id.Nospam,
	}
	expectedID.calculateChecksum()

	if !ConstantTimeEqual2(id.Checksum, expectedID.Checksum) {
		return nil, errors.New("invalid checksum")
	}

	return id, nil
}

// String returns the hexadecimal string representation of the Tox ID.
//
//export ToxIDToString
func (id *ToxID) String() string {
	data := make([]byte, ToxIDSize)
	copy(data[0:KeySize], id.PublicKey[:])
	copy(data[KeySize:KeySize+ToxIDNospamSize], id.Nospam[:])
	copy(data[KeySize+ToxIDNospamSize:ToxIDSize], id.Checksum[:])
	return hex.EncodeToString(data)
}

// SetNospam changes the nospam value for the Tox ID.
//
//export ToxIDSetNospam
func (id *ToxID) SetNospam(nospam [ToxIDNospamSize]byte) {
	id.Nospam = nospam
	id.calculateChecksum()
}

// GenerateNospam creates a random nospam value.
//
//export ToxIDGenerateNospam
func GenerateNospam() ([ToxIDNospamSize]byte, error) {
	var nospam [ToxIDNospamSize]byte
	_, err := rand.Read(nospam[:])
	if err != nil {
		return [ToxIDNospamSize]byte{}, err
	}
	return nospam, nil
}

// GetNospam returns the current nospam value.
//
//export ToxIDGetNospam
func (id *ToxID) GetNospam() [ToxIDNospamSize]byte {
	return id.Nospam
}

// Equal reports whether two ToxIDs are identical, including public key, nospam, and checksum.
// This is more efficient than string comparison as it avoids hex encoding allocations.
// Uses constant-time comparison for all fields to avoid timing side-channel vulnerabilities.
func (id ToxID) Equal(other ToxID) bool {
	return ConstantTimeEqual32(id.PublicKey, other.PublicKey) &&
		ConstantTimeEqual4(id.Nospam, other.Nospam) &&
		ConstantTimeEqual2(id.Checksum, other.Checksum)
}

// PublicKeyEqual reports whether two ToxIDs have the same public key.
// This helper preserves the original semantics of Equal for callers that only care about key identity.
func (id ToxID) PublicKeyEqual(other ToxID) bool {
	return ConstantTimeEqual32(id.PublicKey, other.PublicKey)
}

// calculateChecksum computes the checksum for this Tox ID.
func (id *ToxID) calculateChecksum() {
	// Implementation of Tox's checksum algorithm
	var checksum [ToxIDChecksumSize]byte

	// Combine public key and nospam bytes for checksum calculation
	checksumDataSize := KeySize + ToxIDNospamSize // 36 bytes
	data := make([]byte, checksumDataSize)
	copy(data[0:KeySize], id.PublicKey[:])
	copy(data[KeySize:checksumDataSize], id.Nospam[:])

	// Calculate checksum using XOR operations on each byte
	for i := 0; i < checksumDataSize; i++ {
		checksum[i%ToxIDChecksumSize] ^= data[i]
	}

	id.Checksum = checksum
}
