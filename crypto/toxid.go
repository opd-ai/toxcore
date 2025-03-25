package crypto

import (
	"encoding/hex"
	"errors"
)

// ToxID represents a Tox identifier, consisting of a public key, nospam value, and checksum.
//
//export ToxID
type ToxID struct {
	PublicKey [32]byte
	Nospam    [4]byte
	Checksum  [2]byte
}

// NewToxID creates a ToxID from a public key and nospam value.
//
//export ToxIDNew
func NewToxID(publicKey [32]byte, nospam [4]byte) *ToxID {
	id := &ToxID{
		PublicKey: publicKey,
		Nospam:    nospam,
	}
	id.calculateChecksum()
	return id
}

// FromString parses a Tox ID from its hexadecimal string representation.
//
//export ToxIDFromString
func ToxIDFromString(s string) (*ToxID, error) {
	// ToxID is 38 bytes (76 hex chars): 32 for public key + 4 for nospam + 2 for checksum
	if len(s) != 76 {
		return nil, errors.New("invalid Tox ID length")
	}

	data, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}

	id := &ToxID{}
	copy(id.PublicKey[:], data[0:32])
	copy(id.Nospam[:], data[32:36])
	copy(id.Checksum[:], data[36:38])

	// Verify checksum
	expectedID := &ToxID{
		PublicKey: id.PublicKey,
		Nospam:    id.Nospam,
	}
	expectedID.calculateChecksum()

	if id.Checksum != expectedID.Checksum {
		return nil, errors.New("invalid checksum")
	}

	return id, nil
}

// String returns the hexadecimal string representation of the Tox ID.
//
//export ToxIDToString
func (id *ToxID) String() string {
	data := make([]byte, 38)
	copy(data[0:32], id.PublicKey[:])
	copy(data[32:36], id.Nospam[:])
	copy(data[36:38], id.Checksum[:])
	return hex.EncodeToString(data)
}

// calculateChecksum computes the checksum for this Tox ID.
func (id *ToxID) calculateChecksum() {
	// Implementation of Tox's checksum algorithm
	var checksum [2]byte
	for i := 0; i < 32; i++ {
		checksum[i%2] ^= id.PublicKey[i]
	}
	for i := 0; i < 4; i++ {
		checksum[i%2] ^= id.Nospam[i]
	}
	id.Checksum = checksum
}
