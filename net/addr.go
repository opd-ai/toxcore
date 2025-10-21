package net

import (
	"encoding/hex"
	"strings"

	"github.com/opd-ai/toxforge/crypto"
)

// ToxAddr implements net.Addr for Tox addresses.
// It represents a Tox ID (public key + nospam + checksum).
type ToxAddr struct {
	toxID *crypto.ToxID
}

// NewToxAddr creates a new ToxAddr from a Tox ID string.
// The Tox ID should be a 76-character hexadecimal string.
func NewToxAddr(toxIDString string) (*ToxAddr, error) {
	// Remove any whitespace and convert to uppercase for consistency
	toxIDString = strings.ToUpper(strings.TrimSpace(toxIDString))

	toxID, err := crypto.ToxIDFromString(toxIDString)
	if err != nil {
		return nil, &ToxNetError{
			Op:   "parse",
			Addr: toxIDString,
			Err:  ErrInvalidToxID,
		}
	}

	return &ToxAddr{toxID: toxID}, nil
}

// NewToxAddrFromPublicKey creates a ToxAddr from a public key and nospam.
func NewToxAddrFromPublicKey(publicKey [32]byte, nospam [4]byte) *ToxAddr {
	toxID := crypto.NewToxID(publicKey, nospam)
	return &ToxAddr{toxID: toxID}
}

// Network returns the network name for Tox addresses.
// This implements net.Addr.Network().
func (a *ToxAddr) Network() string {
	return "tox"
}

// String returns the string representation of the Tox address.
// This implements net.Addr.String().
func (a *ToxAddr) String() string {
	if a.toxID == nil {
		return "<invalid>"
	}
	return a.toxID.String()
}

// PublicKey returns the public key component of the Tox ID.
func (a *ToxAddr) PublicKey() [32]byte {
	if a.toxID == nil {
		return [32]byte{}
	}
	return a.toxID.PublicKey
}

// Nospam returns the nospam component of the Tox ID.
func (a *ToxAddr) Nospam() [4]byte {
	if a.toxID == nil {
		return [4]byte{}
	}
	return a.toxID.Nospam
}

// ToxID returns the underlying ToxID object.
func (a *ToxAddr) ToxID() *crypto.ToxID {
	return a.toxID
}

// Equal returns true if two ToxAddr instances represent the same address.
func (a *ToxAddr) Equal(other *ToxAddr) bool {
	if a == nil || other == nil {
		return a == other
	}
	if a.toxID == nil || other.toxID == nil {
		return a.toxID == other.toxID
	}
	return a.toxID.PublicKey == other.toxID.PublicKey &&
		a.toxID.Nospam == other.toxID.Nospam
}

// ParseToxAddr is a convenience function that parses a Tox address string.
// It's equivalent to NewToxAddr but follows Go naming conventions.
func ParseToxAddr(address string) (*ToxAddr, error) {
	return NewToxAddr(address)
}

// ResolveToxAddr resolves a Tox address. Since Tox addresses are direct
// identifiers, this just validates and returns the parsed address.
func ResolveToxAddr(address string) (*ToxAddr, error) {
	return NewToxAddr(address)
}

// IsToxAddr checks if an address string represents a valid Tox address.
func IsToxAddr(address string) bool {
	// Clean the address
	address = strings.ToUpper(strings.TrimSpace(address))

	// Check length (76 hex characters)
	if len(address) != 76 {
		return false
	}

	// Check if it's valid hex
	_, err := hex.DecodeString(address)
	if err != nil {
		return false
	}

	// Try to parse as ToxID to validate checksum
	_, err = crypto.ToxIDFromString(address)
	return err == nil
}
