package crypto

import "crypto/subtle"

// ConstantTimeEqual32 compares two 32-byte arrays in constant time.
// Use this for public keys and other fixed-size cryptographic values
// to avoid timing side-channel vulnerabilities.
func ConstantTimeEqual32(a, b [32]byte) bool {
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

// ConstantTimeEqual4 compares two 4-byte arrays in constant time.
func ConstantTimeEqual4(a, b [4]byte) bool {
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

// ConstantTimeEqual2 compares two 2-byte arrays in constant time.
func ConstantTimeEqual2(a, b [2]byte) bool {
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}
