package main

import (
	"bytes"
	"fmt"
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

	// Test tox_self_get_address_size returns correct size (binary = 38 bytes)
	addrSize := tox_self_get_address_size(tox)
	if addrSize != 38 { // Tox address is 38 bytes (32 pubkey + 4 nospam + 2 checksum)
		t.Errorf("Expected address size 38 (binary), got %d", addrSize)
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

func TestToxSelfGetSafetyNumber(t *testing.T) {
	tox := tox_new()
	if tox == nil {
		t.Fatal("Failed to create Tox instance")
	}
	defer tox_kill(tox)

	peer := make([]byte, 32)
	for i := range peer {
		peer[i] = byte(i + 1)
	}

	// Query required length first.
	needed := tox_self_get_safety_number(tox, &peer[0], nil, 0)
	if needed <= 0 {
		t.Fatalf("expected positive safety number length, got %d", needed)
	}

	buf := make([]byte, needed+1)
	gotLen := tox_self_get_safety_number(tox, &peer[0], &buf[0], len(buf))
	if gotLen != needed {
		t.Fatalf("unexpected safety number length: got %d want %d", gotLen, needed)
	}
	if buf[gotLen] != 0 {
		t.Fatal("expected null terminator in safety number buffer")
	}
	if gotLen != 71 {
		t.Fatalf("expected 71-char safety number (60 digits + 11 spaces), got %d", gotLen)
	}
}

func TestToxCryptoGenerateKeypair(t *testing.T) {
	publicKey := make([]byte, 32)
	secretKey := make([]byte, 32)

	ok := tox_crypto_generate_keypair(&publicKey[0], &secretKey[0])
	if ok != 1 {
		t.Fatal("tox_crypto_generate_keypair failed")
	}

	if bytes.Equal(publicKey, make([]byte, 32)) {
		t.Fatal("public key should not be all zeros")
	}
	if bytes.Equal(secretKey, make([]byte, 32)) {
		t.Fatal("secret key should not be all zeros")
	}
}

func TestToxCryptoSecureWipe(t *testing.T) {
	data := []byte("sensitive material")
	if tox_crypto_secure_wipe(&data[0], len(data)) != 1 {
		t.Fatal("tox_crypto_secure_wipe failed")
	}

	for i, b := range data {
		if b != 0 {
			t.Fatalf("byte %d not wiped (value=%d)", i, b)
		}
	}
}

func TestToxABIVersion(t *testing.T) {
	major := tox_abi_version_major()
	minor := tox_abi_version_minor()
	patch := tox_abi_version_patch()

	if int(major) != toxABIVersionMajor || int(minor) != toxABIVersionMinor || int(patch) != toxABIVersionPatch {
		t.Fatalf("unexpected ABI version tuple: got %d.%d.%d", major, minor, patch)
	}

	need := tox_abi_version_string(nil, 0)
	if need <= 0 {
		t.Fatalf("expected positive ABI version string length, got %d", need)
	}

	buf := make([]byte, need+1)
	got := tox_abi_version_string(&buf[0], len(buf))
	if got != need {
		t.Fatalf("unexpected ABI version string len: got %d want %d", got, need)
	}
	if buf[got] != 0 {
		t.Fatal("expected null terminator in ABI version string buffer")
	}

	want := fmt.Sprintf("%d.%d.%d", toxABIVersionMajor, toxABIVersionMinor, toxABIVersionPatch)
	if string(buf[:got]) != want {
		t.Fatalf("unexpected ABI version string: got %q want %q", string(buf[:got]), want)
	}
}

func TestToxABIFeatureFlags(t *testing.T) {
	flags := uint64(tox_abi_feature_flags())
	if flags != toxABIFeatureMask {
		t.Fatalf("unexpected ABI feature mask: got 0x%x want 0x%x", flags, toxABIFeatureMask)
	}

	required := toxABIFeatureGenerateKeypair | toxABIFeatureSecureWipe | toxABIFeatureSafetyNumber
	if flags&required != required {
		t.Fatalf("missing required feature bits: flags=0x%x required=0x%x", flags, required)
	}
}
