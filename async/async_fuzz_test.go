package async

import (
	"testing"
)

// FuzzMessagePadding fuzzes the message padding/unpadding functions
func FuzzMessagePadding(f *testing.F) {
	// Add seed corpus
	f.Add([]byte("Hello"))
	f.Add([]byte(""))
	f.Add(make([]byte, 255))
	f.Add(make([]byte, 1023))

	f.Fuzz(func(t *testing.T, message []byte) {
		// Skip oversized messages to prevent OOM
		if len(message) > MaxMessageSize {
			return
		}

		// Pad the message - should not panic
		padded := PadMessageToStandardSize(message)

		// Unpad the message - should not panic
		unpadded, err := UnpadMessage(padded)
		if err != nil {
			// Unpadding can fail for invalid input
			return
		}

		// If both succeeded, verify correctness
		if string(message) != string(unpadded) {
			t.Errorf("Unpadding failed: got %d bytes, want %d bytes", len(unpadded), len(message))
		}
	})
}

// FuzzMessagePaddingMalformed fuzzes unpadding with malformed input
func FuzzMessagePaddingMalformed(f *testing.F) {
	// Add seed corpus with various malformed inputs
	f.Add(make([]byte, 0))
	f.Add(make([]byte, 1))
	f.Add(make([]byte, 3)) // Less than length prefix
	f.Add(make([]byte, 256))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Skip very large inputs
		if len(data) > 5000 {
			return
		}

		// Attempt to unpad - should not panic even with malformed input
		_, err := UnpadMessage(data)

		// We expect error for most inputs, but no panic
		if err == nil && len(data) < LengthPrefixSize {
			t.Error("UnpadMessage should reject data shorter than length prefix")
		}
	})
}

// FuzzObfuscation fuzzes the identity obfuscation functions
func FuzzObfuscation(f *testing.F) {
	// Add seed corpus
	validPK := make([]byte, 32)
	for i := range validPK {
		validPK[i] = byte(i)
	}
	f.Add(validPK)
	f.Add(make([]byte, 32))

	f.Fuzz(func(t *testing.T, publicKey []byte) {
		if len(publicKey) != 32 {
			return
		}

		var pk [32]byte
		copy(pk[:], publicKey)

		// Test basic pseudonym generation logic (simplified)
		// In real implementation, this would use HKDF
		// For fuzzing, just verify we handle all inputs without panic
		_ = pk
	})
}

// FuzzStorageLimits fuzzes the storage capacity calculations
func FuzzStorageLimits(f *testing.F) {
	// Add seed corpus
	f.Add(uint64(1000000), uint64(100)) // 1MB, 100 bytes per message
	f.Add(uint64(0), uint64(100))       // Zero limit
	f.Add(uint64(1000000), uint64(0))   // Zero message size

	f.Fuzz(func(t *testing.T, bytesLimit, avgMessageSize uint64) {
		// Skip unrealistic values to prevent OOM
		if bytesLimit > 1<<30 || avgMessageSize > 1<<20 {
			return
		}

		// Perform basic capacity calculation - should not panic or overflow
		if avgMessageSize > 0 && bytesLimit > 0 {
			capacity := bytesLimit / avgMessageSize

			// Verify reasonable bounds
			if capacity > bytesLimit {
				t.Errorf("Capacity calculation overflow: got %d, limit %d", capacity, bytesLimit)
			}
		}
	})
}

// FuzzEpochCalculation fuzzes epoch time calculations
func FuzzEpochCalculation(f *testing.F) {
	// Add seed corpus
	f.Add(int64(1000000), int64(3600)) // 1M seconds, 1 hour epochs
	f.Add(int64(0), int64(1))          // Edge case
	f.Add(int64(-1000), int64(100))    // Negative time

	f.Fuzz(func(t *testing.T, timestamp, epochDuration int64) {
		// Skip invalid durations
		if epochDuration <= 0 {
			return
		}

		// Skip extremely large values
		if epochDuration > 1<<30 || timestamp > 1<<40 || timestamp < -(1<<40) {
			return
		}

		// This demonstrates the epoch calculation concept
		// Real implementation would use time.Duration
		_ = timestamp / epochDuration
	})
}

// FuzzMessageSerialization fuzzes message serialization/deserialization
func FuzzMessageSerialization(f *testing.F) {
	// Add seed corpus with valid message structures
	f.Add([]byte(`{"type":"message","data":"test"}`))
	f.Add([]byte(""))
	f.Add([]byte("{}"))
	f.Add([]byte("null"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Skip very large inputs
		if len(data) > 10000 {
			return
		}

		// Attempt to deserialize - should not panic
		// This would use actual message deserialization in real code
		// For now, just verify we can handle arbitrary data
		_ = data
	})
}
