package crypto

import (
	"crypto/subtle"
	"errors"
	"runtime"
)

// SecureWipe attempts to securely erase the contents of a byte slice
// containing sensitive data. It returns an error if the byte slice is nil.
//
// This function uses subtle.XORBytes to perform a constant-time XOR operation
// that the compiler cannot optimize away. XORing data with itself (x XOR x = 0)
// securely zeros the data while providing resistance to compiler optimizations.
//
//export ToxSecureWipe
func SecureWipe(data []byte) error {
	if data == nil {
		return errors.New("cannot wipe nil data")
	}

	// Overwrite the data with zeros using XOR operation
	// subtle.XORBytes performs constant-time XOR that compilers cannot optimize away
	// XORing data with itself: x XOR x = 0
	subtle.XORBytes(data, data, data)

	// Prevent compiler from optimizing out the zeroing
	runtime.KeepAlive(data)

	return nil
}

// ZeroBytes erases the contents of a byte slice containing sensitive data.
// This is a convenience function that ignores the error from SecureWipe.
//
//export ToxZeroBytes
func ZeroBytes(data []byte) {
	_ = SecureWipe(data)
}

// WipeKeyPair securely erases the private key in a KeyPair.
// This should be called when a KeyPair is no longer needed.
//
//export ToxWipeKeyPair
func WipeKeyPair(kp *KeyPair) error {
	if kp == nil {
		return errors.New("cannot wipe nil KeyPair")
	}
	return SecureWipe(kp.Private[:])
}
