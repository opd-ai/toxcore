package main

import (
	"encoding/hex"
	"sync"
	"unsafe"

	"github.com/opd-ai/toxcore"
	"github.com/sirupsen/logrus"
)

// This is the main package required for building as c-shared.
// It provides C-compatible wrappers for the Go toxcore implementation.

// main is required by Go for c-shared build mode but intentionally empty.
// When building with -buildmode=c-shared, Go requires a main package with a main
// function, but the function body is never executed. The shared library's entry
// point is the C runtime initialization, not main().
func main() {}

// Global variable to store Tox instances by ID
var (
	toxInstances   = make(map[int]*toxcore.Tox)
	nextInstanceID = 1
	toxMutex       sync.RWMutex
)

// GetToxInstanceByID retrieves a Tox instance by ID with proper mutex protection.
// This is the authorized accessor for cross-file access within the capi package.
// Returns nil if the instance doesn't exist.
func GetToxInstanceByID(toxID int) *toxcore.Tox {
	toxMutex.RLock()
	defer toxMutex.RUnlock()
	if tox, exists := toxInstances[toxID]; exists {
		return tox
	}
	return nil
}

// safeGetToxID safely extracts the Tox instance ID from an opaque C pointer.
// This function uses panic recovery to prevent crashes from invalid pointers
// passed from C code, which is essential for C API safety.
// Returns (id, valid) where valid indicates if the pointer was successfully dereferenced.
func safeGetToxID(ptr unsafe.Pointer) (int, bool) {
	if ptr == nil {
		return 0, false
	}

	var toxID int
	var validDeref bool

	func() {
		defer func() {
			if r := recover(); r != nil {
				validDeref = false
				logrus.WithFields(logrus.Fields{
					"function": "safeGetToxID",
					"error":    r,
				}).Warn("Invalid pointer dereference caught in C API")
			}
		}()

		handle := (*int)(ptr)
		toxID = *handle
		validDeref = true
	}()

	if !validDeref {
		return 0, false
	}

	// Sanity check: ID should be positive
	if toxID <= 0 {
		return 0, false
	}

	return toxID, true
}

//export tox_new
func tox_new() unsafe.Pointer {
	// Create new Tox instance with default options
	goOptions := toxcore.NewOptions()

	// Create new Tox instance
	tox, err := toxcore.New(goOptions)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "tox_new",
			"error":    err.Error(),
		}).Error("Failed to create new Tox instance")
		return nil
	}

	toxMutex.Lock()
	defer toxMutex.Unlock()

	// Store instance and return ID as pointer
	instanceID := nextInstanceID
	nextInstanceID++
	toxInstances[instanceID] = tox

	// Create an opaque pointer handle
	handle := new(int)
	*handle = instanceID
	return unsafe.Pointer(handle)
}

//export tox_kill
func tox_kill(tox unsafe.Pointer) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	toxMutex.Lock()
	defer toxMutex.Unlock()

	if toxInstance, exists := toxInstances[toxID]; exists {
		toxInstance.Kill()
		delete(toxInstances, toxID)
	}
}

//export tox_bootstrap_simple
func tox_bootstrap_simple(tox unsafe.Pointer) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return -1
	}

	toxMutex.RLock()
	toxInstance, exists := toxInstances[toxID]
	toxMutex.RUnlock()

	if !exists {
		return -1
	}

	// Use known working bootstrap node for testing
	err := toxInstance.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		return -1
	}

	return 0 // Success
}

//export tox_iterate
func tox_iterate(tox unsafe.Pointer) {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return
	}

	toxMutex.RLock()
	toxInstance, exists := toxInstances[toxID]
	toxMutex.RUnlock()

	if exists {
		toxInstance.Iterate()
	}
}

//export tox_iteration_interval
func tox_iteration_interval(tox unsafe.Pointer) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 50 // Default 50ms
	}

	toxMutex.RLock()
	toxInstance, exists := toxInstances[toxID]
	toxMutex.RUnlock()

	if exists {
		return int(toxInstance.IterationInterval().Milliseconds())
	}
	return 50 // Default 50ms
}

//export tox_self_get_address_size
func tox_self_get_address_size(tox unsafe.Pointer) int {
	toxID, ok := safeGetToxID(tox)
	if !ok {
		return 0
	}

	toxMutex.RLock()
	toxInstance, exists := toxInstances[toxID]
	toxMutex.RUnlock()

	if exists {
		addr := toxInstance.SelfGetAddress()
		return len(addr)
	}
	return 0
}

//export hex_string_to_bin
func hex_string_to_bin(hexStr *byte, hexLen int, output *byte, outputLen int) int {
	// Convert C buffer to Go slice using unsafe.Slice (clearer than manual iteration)
	hexBytes := unsafe.Slice(hexStr, hexLen)
	hexString := string(hexBytes)

	// Decode hex string
	decoded, err := hex.DecodeString(hexString)
	if err != nil {
		return -1 // Error
	}

	// Check output buffer size
	if len(decoded) > outputLen {
		return -1 // Buffer too small
	}

	// Copy to output buffer using copy builtin (clearer and potentially faster)
	outputSlice := unsafe.Slice(output, outputLen)
	copy(outputSlice, decoded)

	return len(decoded) // Return number of bytes written
}
