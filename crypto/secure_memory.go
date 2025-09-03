package crypto

import (
	"crypto/subtle"
	"errors"
	"runtime"
)

// SecureWipe attempts to securely erase the contents of a byte slice 
// containing sensitive data. It returns an error if the byte slice is nil.
//
//export ToxSecureWipe
func SecureWipe(data []byte) error {
	if data == nil {
		return errors.New("cannot wipe nil data")
	}

	// Overwrite the data with zeros
	// Using subtle.ConstantTimeCompare's byteXor operation to avoid 
	// potential compiler optimizations that might remove the overwrite
	zeros := make([]byte, len(data))
	subtle.ConstantTimeCompare(data, zeros)
	copy(data, zeros)

	// Attempt to prevent the compiler from optimizing out the zeroing
	runtime.KeepAlive(data)
	runtime.KeepAlive(zeros)

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
