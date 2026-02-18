package main

import (
	"testing"
	"unsafe"
)

// TestHexStringToBin tests the hex_string_to_bin function
func TestHexStringToBin(t *testing.T) {
	tests := []struct {
		name       string
		hexStr     string
		outputSize int
		wantLen    int
		wantError  bool
	}{
		{
			name:       "valid hex string",
			hexStr:     "48656c6c6f",
			outputSize: 10,
			wantLen:    5,
			wantError:  false,
		},
		{
			name:       "empty hex string",
			hexStr:     "",
			outputSize: 10,
			wantLen:    0,
			wantError:  false,
		},
		{
			name:       "short public key",
			hexStr:     "F404ABAA1C99A9D3",
			outputSize: 10,
			wantLen:    8,
			wantError:  false,
		},
		{
			name:       "invalid hex (odd length)",
			hexStr:     "ABC",
			outputSize: 10,
			wantLen:    -1,
			wantError:  true,
		},
		{
			name:       "invalid hex characters",
			hexStr:     "GHIJ",
			outputSize: 10,
			wantLen:    -1,
			wantError:  true,
		},
		{
			name:       "buffer too small",
			hexStr:     "48656c6c6f576f726c64",
			outputSize: 3,
			wantLen:    -1,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hexBytes := []byte(tt.hexStr)
			output := make([]byte, tt.outputSize)

			var hexPtr *byte
			if len(hexBytes) > 0 {
				hexPtr = &hexBytes[0]
			}
			var outPtr *byte
			if len(output) > 0 {
				outPtr = &output[0]
			}

			result := hex_string_to_bin(hexPtr, len(hexBytes), outPtr, tt.outputSize)

			if tt.wantError {
				if result != -1 {
					t.Errorf("Expected error (-1), got %d", result)
				}
			} else {
				if result != tt.wantLen {
					t.Errorf("Expected length %d, got %d", tt.wantLen, result)
				}
			}
		})
	}
}

// TestHexStringToBinContent verifies the output content is correct
func TestHexStringToBinContent(t *testing.T) {
	hexStr := "48656c6c6f" // "Hello" in hex
	hexBytes := []byte(hexStr)
	output := make([]byte, 10)

	result := hex_string_to_bin(&hexBytes[0], len(hexBytes), &output[0], len(output))
	if result != 5 {
		t.Fatalf("Expected 5 bytes, got %d", result)
	}

	expected := []byte("Hello")
	for i := 0; i < 5; i++ {
		if output[i] != expected[i] {
			t.Errorf("Byte %d: expected %d, got %d", i, expected[i], output[i])
		}
	}
}

// TestToxCoreBasicOperations tests basic toxcore_c.go operations
func TestToxCoreBasicOperations(t *testing.T) {
	// Test tox_new creates valid instance
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	// Test tox_iteration_interval returns reasonable value
	interval := tox_iteration_interval(tox)
	if interval <= 0 || interval > 1000 {
		t.Errorf("Unexpected iteration interval: %d", interval)
	}

	// Test tox_self_get_address_size returns correct size (hex string = 76 chars)
	addrSize := tox_self_get_address_size(tox)
	if addrSize != 76 { // Tox address is 38 bytes, but returned as hex string (76 chars)
		t.Errorf("Expected address size 76 (hex string), got %d", addrSize)
	}

	// Test tox_iterate doesn't crash
	tox_iterate(tox)
}

// TestToxCoreNilHandling tests nil pointer handling
func TestToxCoreNilHandling(t *testing.T) {
	// tox_kill with nil should not crash
	tox_kill(nil)

	// tox_iterate with nil should not crash
	tox_iterate(nil)

	// tox_iteration_interval with nil should return default
	interval := tox_iteration_interval(nil)
	if interval != 50 {
		t.Errorf("Expected default interval 50, got %d", interval)
	}

	// tox_self_get_address_size with nil should return 0
	size := tox_self_get_address_size(nil)
	if size != 0 {
		t.Errorf("Expected 0 for nil tox, got %d", size)
	}

	// tox_bootstrap_simple with nil should return -1
	result := tox_bootstrap_simple(nil)
	if result != -1 {
		t.Errorf("Expected -1 for nil tox bootstrap, got %d", result)
	}
}

// TestToxCoreInvalidPointer tests behavior with invalid pointers
func TestToxCoreInvalidPointer(t *testing.T) {
	// Create an invalid pointer (not pointing to real Tox instance)
	invalidID := 99999
	invalidPtr := unsafe.Pointer(&invalidID)

	// These should handle invalid pointers gracefully
	tox_iterate(invalidPtr)
	interval := tox_iteration_interval(invalidPtr)
	if interval != 50 { // Should return default
		t.Errorf("Expected default interval for invalid pointer, got %d", interval)
	}

	size := tox_self_get_address_size(invalidPtr)
	if size != 0 { // Should return 0 for invalid
		t.Errorf("Expected 0 for invalid pointer, got %d", size)
	}
}

// TestMultipleToxInstances tests creating and managing multiple instances
func TestMultipleToxInstances(t *testing.T) {
	instances := make([]unsafe.Pointer, 3)

	// Create multiple instances
	for i := 0; i < 3; i++ {
		instances[i] = tox_new()
		if instances[i] == nil {
			t.Fatalf("Failed to create Tox instance %d", i)
		}
	}

	// Verify each instance works independently
	for i, tox := range instances {
		interval := tox_iteration_interval(tox)
		if interval <= 0 {
			t.Errorf("Instance %d has invalid interval: %d", i, interval)
		}
	}

	// Clean up
	for _, tox := range instances {
		tox_kill(tox)
	}

	// Verify cleanup worked - operations on killed instances return defaults
	for i, tox := range instances {
		interval := tox_iteration_interval(tox)
		if interval != 50 { // Default value
			t.Errorf("Instance %d should return default after kill, got %d", i, interval)
		}
	}
}
