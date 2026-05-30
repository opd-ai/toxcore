//go:build !cgo || (!linux && !darwin)

package crypto

// mlockAvailable reports whether mlock(2) is available on this platform.
// It is false on pure-Go builds and on platforms other than Linux/macOS.
const mlockAvailable = false

// secureAlloc falls back to a normal Go allocation when mlock is unavailable.
// The contents are zero-initialised (Go guarantees zeroed allocations).
func secureAlloc(size int) []byte {
	return make([]byte, size)
}
