package ratchet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTelemetryTracking validates that telemetry correctly tracks key lifecycle events.
func TestTelemetryTracking(t *testing.T) {
	tel := NewTelemetry()

	// Initial state
	stats := tel.Stats()
	assert.Equal(t, uint64(0), stats.KeysDeleted)
	assert.Equal(t, uint64(0), stats.MessagesSent)

	// Record some events
	tel.RecordMessageSent()
	tel.RecordMessageSent()
	tel.RecordMessageReceived()
	tel.RecordKeyDeleted()

	stats = tel.Stats()
	assert.Equal(t, uint64(2), stats.MessagesSent)
	assert.Equal(t, uint64(1), stats.MessagesReceived)
	assert.Equal(t, uint64(1), stats.KeysDeleted)
}

// TestTelemetrySkippedKeyTracking validates tracking of skipped key events.
func TestTelemetrySkippedKeyTracking(t *testing.T) {
	tel := NewTelemetry()

	// Record skipped key events
	tel.RecordSkippedKeyStored(100)
	tel.RecordSkippedKeyStored(200)
	tel.RecordSkippedKeyRetrieved() // This also increments KeysDeleted
	tel.RecordSkippedKeyEvicted()

	stats := tel.Stats()
	assert.Equal(t, uint64(2), stats.SkippedKeysStored)
	assert.Equal(t, uint64(1), stats.SkippedKeysRetrieved)
	assert.Equal(t, uint64(1), stats.SkippedKeysEvicted)
	assert.Equal(t, uint64(2), stats.KeysDeleted) // From RecordSkippedKeyRetrieved and RecordSkippedKeyEvicted
}

// TestTelemetryDecryptionErrors validates tracking of decryption failures.
func TestTelemetryDecryptionErrors(t *testing.T) {
	tel := NewTelemetry()

	// Record successful and failed decryption
	tel.RecordMessageReceived()
	tel.RecordDecryptionError()
	tel.RecordMessageReceived()
	tel.RecordDecryptionError()
	tel.RecordDecryptionError()

	stats := tel.Stats()
	assert.Equal(t, uint64(2), stats.MessagesReceived)
	assert.Equal(t, uint64(3), stats.DecryptionErrors)
}

// TestTelemetryNilSafety validates that telemetry methods are nil-safe.
func TestTelemetryNilSafety(t *testing.T) {
	var tel *Telemetry

	// Should not panic
	tel.RecordMessageSent()
	tel.RecordKeyDeleted()
	tel.RecordSkippedKeyStored(100)
	tel.RecordDecryptionError()

	stats := tel.Stats()
	assert.Equal(t, TelemetryStats{}, stats)
}

// TestTelemetryWarningThreshold validates that skipped key warning threshold is respected.
func TestTelemetryWarningThreshold(t *testing.T) {
	tel := NewTelemetry()
	tel.MaxSkippedKeysWarningThreshold = 80

	// Record skipped keys at various levels
	// At 80% of MaxSkippedKeys (800), should trigger warning condition
	tel.RecordSkippedKeyStored(799) // Below threshold - no warning
	tel.RecordSkippedKeyStored(801) // Above threshold - would warn
	tel.RecordSkippedKeyStored(900) // Well above threshold

	// Verify counts are correct
	stats := tel.Stats()
	assert.Equal(t, uint64(3), stats.SkippedKeysStored)
}

// TestTelemetryConcurrency validates that telemetry is thread-safe.
func TestTelemetryConcurrency(t *testing.T) {
	tel := NewTelemetry()

	// Simulate concurrent updates
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				tel.RecordMessageSent()
				tel.RecordMessageReceived()
				tel.RecordKeyDeleted()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final counts
	stats := tel.Stats()
	assert.Equal(t, uint64(1000), stats.MessagesSent)
	assert.Equal(t, uint64(1000), stats.MessagesReceived)
	assert.Equal(t, uint64(1000), stats.KeysDeleted)
}

// TestTelemetryStatsString validates the string representation.
func TestTelemetryStatsString(t *testing.T) {
	stats := TelemetryStats{
		MessagesSent:     100,
		MessagesReceived: 99,
		KeysDeleted:      50,
		DecryptionErrors: 1,
	}

	str := stats.String()
	assert.Contains(t, str, "sent=100")
	assert.Contains(t, str, "recv=99")
	assert.Contains(t, str, "deleted=50")
	assert.Contains(t, str, "decrypt_errors=1")
}

// TestKeyDeletionLifecycle validates the complete lifecycle of key deletion.
// This demonstrates how telemetry tracks keys from creation through deletion.
func TestKeyDeletionLifecycle(t *testing.T) {
	tel := NewTelemetry()

	// Simulate message encryption (creates message key)
	tel.RecordMessageSent()

	// Message key is used and deleted
	tel.RecordKeyDeleted()

	// Simulate out-of-order message storage
	tel.RecordSkippedKeyStored(10)

	// Later, the skipped key is used
	tel.RecordSkippedKeyRetrieved()

	stats := tel.Stats()
	assert.Equal(t, uint64(1), stats.MessagesSent)
	assert.Equal(t, uint64(2), stats.KeysDeleted) // Initial + skipped retrieval
	assert.Equal(t, uint64(1), stats.SkippedKeysStored)
	assert.Equal(t, uint64(1), stats.SkippedKeysRetrieved)
}

// TestSkippedKeyEvictionMetrics validates tracking of skipped key evictions.
// This helps detect when the session is under heavy out-of-order message load.
func TestSkippedKeyEvictionMetrics(t *testing.T) {
	tel := NewTelemetry()

	// Simulate normal message flow with some out-of-order messages
	for i := 0; i < 10; i++ {
		tel.RecordMessageSent()
	}

	// Some messages arrive out of order and keys are stored
	for i := 0; i < 5; i++ {
		tel.RecordSkippedKeyStored(i)
	}

	// If we receive too many out-of-order messages, some keys get evicted
	for i := 0; i < 2; i++ {
		tel.RecordSkippedKeyEvicted()
	}

	stats := tel.Stats()
	assert.Equal(t, uint64(10), stats.MessagesSent)
	assert.Equal(t, uint64(5), stats.SkippedKeysStored)
	assert.Equal(t, uint64(2), stats.SkippedKeysEvicted)
	assert.Greater(t, stats.SkippedKeysEvicted, uint64(0), "evictions indicate network issues")
}
