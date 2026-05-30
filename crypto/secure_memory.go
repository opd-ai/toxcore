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
// This is a convenience function for cases where the caller cannot or does
// not need to handle errors (e.g., defer statements, cleanup code).
//
// If data is nil, this function is a no-op and returns silently.
// For cases where error handling is important, use SecureWipe directly.
//
//export ToxZeroBytes
func ZeroBytes(data []byte) {
	if data == nil {
		return
	}
	if err := SecureWipe(data); err != nil {
		// A failure here means the runtime's security invariant cannot be upheld.
		// Panic rather than silently allowing key material to persist in memory.
		panic("crypto: SecureWipe failed: " + err.Error())
	}
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

// SecureAllocate allocates a zeroed byte slice of the given size intended to
// hold sensitive key material.
//
// On Linux and macOS when the binary is built with CGo enabled, the backing
// memory is allocated via malloc(3) and locked into physical RAM with
// mlock(2), preventing the OS from paging the contents to swap.  When mlock
// fails (e.g. the process has exhausted its RLIMIT_MEMLOCK budget), or when
// CGo is disabled, the function falls back to a regular zeroed Go allocation.
//
// Callers MUST call SecureWipe on the returned slice when the key material is
// no longer needed.  The allocator registers a best-effort GC finalizer, but
// explicit wiping is required to minimise the window during which key material
// is live in memory.
//
// MlockAvailable reports the build-time capability; check it if the
// application requires a hard guarantee rather than best-effort protection.
//
//export ToxSecureAllocate
func SecureAllocate(size int) []byte {
	if size <= 0 {
		return nil
	}
	return secureAlloc(size)
}

// MlockAvailable reports whether the mlock(2) system call is available and
// will be used by SecureAllocate on this platform and build configuration.
// When false, SecureAllocate falls back to a normal Go allocation and key
// material may appear in swap.
func MlockAvailable() bool {
	return mlockAvailable
}
