//go:build cgo && (linux || darwin)

package crypto

// #include <sys/mman.h>
// #include <stdlib.h>
// #include <string.h>
import "C"
import "unsafe"

// mlockAvailable reports whether mlock(2) is available on this platform.
// It is always true on the cgo+linux/darwin build path.
const mlockAvailable = true

// secureAlloc allocates size bytes backed by C memory that is optionally
// locked into physical RAM via mlock(2), preventing the OS from paging the
// contents to swap.
//
// ⚠ Lifetime: The returned slice is backed by C heap memory.  It is NOT
// managed by the Go GC.  Callers MUST call SecureWipe on the slice when the
// key material is no longer needed, and MUST NOT use the slice after wiping.
// Failing to wipe creates a memory leak (the C memory is never freed), but
// the Go process will reclaim all C memory on exit.  This trade-off is
// acceptable for long-lived key buffers because the primary goal is preventing
// swap exposure, not preventing memory leaks.
//
// If malloc(3) fails or mlock(2) is unavailable, the function falls back to a
// normal zeroed Go allocation.
func secureAlloc(size int) []byte {
	ptr := C.malloc(C.size_t(size))
	if ptr == nil {
		// malloc failed; fall back to a regular Go allocation.
		return make([]byte, size)
	}

	// Attempt to lock the page(s) in physical RAM.  Ignore failure:
	// if mlock is unavailable we still benefit from C allocation stability
	// (Go's GC cannot copy C memory).
	C.mlock(ptr, C.size_t(size))

	// Zero the region before handing it to the caller.
	C.memset(ptr, 0, C.size_t(size))

	return unsafe.Slice((*byte)(ptr), size)
}
