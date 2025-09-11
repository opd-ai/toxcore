package main

import (
	"encoding/hex"
	"unsafe"
	"github.com/opd-ai/toxcore"
)

// This is the main package required for building as c-shared
// It provides C-compatible wrappers for the Go toxcore implementation

func main() {} // Required for c-shared build mode

// Global variable to store Tox instances by ID
var (
	toxInstances   = make(map[int]*toxcore.Tox)
	nextInstanceID = 1
)

//export tox_new
func tox_new() int {
	// Create new Tox instance with default options
	goOptions := toxcore.NewOptions()

	// Create new Tox instance
	tox, err := toxcore.New(goOptions)
	if err != nil {
		return -1 // Error
	}

	// Store instance and return ID
	instanceID := nextInstanceID
	nextInstanceID++
	toxInstances[instanceID] = tox

	return instanceID
}

//export tox_kill
func tox_kill(toxID int) {
	if tox, exists := toxInstances[toxID]; exists {
		tox.Kill()
		delete(toxInstances, toxID)
	}
}

//export tox_bootstrap_simple
func tox_bootstrap_simple(toxID int) int {
	tox, exists := toxInstances[toxID]
	if !exists {
		return -1
	}

	// Use known working bootstrap node for testing
	err := tox.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		return -1
	}

	return 0 // Success
}

//export tox_iterate
func tox_iterate(toxID int) {
	if tox, exists := toxInstances[toxID]; exists {
		tox.Iterate()
	}
}

//export tox_iteration_interval
func tox_iteration_interval(toxID int) int {
	if tox, exists := toxInstances[toxID]; exists {
		return int(tox.IterationInterval().Milliseconds())
	}
	return 50 // Default 50ms
}

//export tox_self_get_address_size
func tox_self_get_address_size(toxID int) int {
	if tox, exists := toxInstances[toxID]; exists {
		addr := tox.SelfGetAddress()
		return len(addr)
	}
	return 0
}

//export hex_string_to_bin
func hex_string_to_bin(hexStr *byte, hexLen int, output *byte, outputLen int) int {
	// Convert C string to Go string
	hexBytes := make([]byte, hexLen)
	for i := 0; i < hexLen; i++ {
		hexBytes[i] = *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(hexStr)) + uintptr(i)))
	}
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
	
	// Copy to output buffer
	for i, b := range decoded {
		*(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(output)) + uintptr(i))) = b
	}
	
	return len(decoded) // Return number of bytes written
}
