package ratchet

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// Telemetry tracks metrics about ratchet session operation.
// This is used to monitor key lifecycle and detect attacks.
type Telemetry struct {
	mu sync.RWMutex

	// KeysDeleted counts the number of keys that have been permanently deleted.
	// This includes keys that were used and cleared (via RecordSkippedKeyRetrieved)
	// and keys evicted from the skipped key store (via RecordSkippedKeyEvicted).
	KeysDeleted uint64

	// SkippedKeysStored counts the total number of keys stored in the skipped key store.
	SkippedKeysStored uint64

	// SkippedKeysEvicted counts the number of keys evicted due to store capacity.
	SkippedKeysEvicted uint64

	// SkippedKeysRetrieved counts the number of skipped keys successfully used for decryption.
	SkippedKeysRetrieved uint64

	// MessagesSent counts the total number of messages encrypted.
	MessagesSent uint64

	// MessagesReceived counts the total number of messages decrypted successfully.
	MessagesReceived uint64

	// DecryptionErrors counts failed decryption attempts (auth failures, etc).
	DecryptionErrors uint64

	// MaxSkippedKeysWarningThreshold is the percentage of MaxSkippedKeys at which to warn.
	// Default is 80% (800 of 1000 keys).
	MaxSkippedKeysWarningThreshold int
}

// NewTelemetry creates a new Telemetry tracker with default settings.
func NewTelemetry() *Telemetry {
	return &Telemetry{
		MaxSkippedKeysWarningThreshold: 80,
	}
}

// RecordKeyDeleted records that a key was permanently deleted.
func (t *Telemetry) RecordKeyDeleted() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.KeysDeleted++
}

// RecordSkippedKeyStored records that a skipped key was stored.
func (t *Telemetry) RecordSkippedKeyStored(currentCount int) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.SkippedKeysStored++

	// Check if we're approaching the limit and warn
	threshold := (MaxSkippedKeys * t.MaxSkippedKeysWarningThreshold) / 100
	if currentCount > threshold {
		logrus.WithFields(logrus.Fields{
			"current_count": currentCount,
			"threshold":     threshold,
			"max":           MaxSkippedKeys,
		}).Warn("skipped key store approaching capacity")
	}
}

// RecordSkippedKeyEvicted records that a skipped key was evicted due to capacity.
// Evicted keys are also counted in KeysDeleted since they are permanently removed from memory.
func (t *Telemetry) RecordSkippedKeyEvicted() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.SkippedKeysEvicted++
	t.KeysDeleted++
}

// RecordSkippedKeyRetrieved records that a skipped key was successfully used.
func (t *Telemetry) RecordSkippedKeyRetrieved() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.SkippedKeysRetrieved++
	t.KeysDeleted++
}

// RecordMessageSent records that a message was encrypted.
func (t *Telemetry) RecordMessageSent() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.MessagesSent++
}

// RecordMessageReceived records that a message was decrypted successfully.
func (t *Telemetry) RecordMessageReceived() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.MessagesReceived++
}

// RecordDecryptionError records a failed decryption attempt.
func (t *Telemetry) RecordDecryptionError() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.DecryptionErrors++
}

// Stats returns a snapshot of current telemetry.
func (t *Telemetry) Stats() TelemetryStats {
	if t == nil {
		return TelemetryStats{}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return TelemetryStats{
		KeysDeleted:          t.KeysDeleted,
		SkippedKeysStored:    t.SkippedKeysStored,
		SkippedKeysEvicted:   t.SkippedKeysEvicted,
		SkippedKeysRetrieved: t.SkippedKeysRetrieved,
		MessagesSent:         t.MessagesSent,
		MessagesReceived:     t.MessagesReceived,
		DecryptionErrors:     t.DecryptionErrors,
	}
}

// TelemetryStats is a snapshot of telemetry at a point in time.
type TelemetryStats struct {
	KeysDeleted          uint64
	SkippedKeysStored    uint64
	SkippedKeysEvicted   uint64
	SkippedKeysRetrieved uint64
	MessagesSent         uint64
	MessagesReceived     uint64
	DecryptionErrors     uint64
}

// String returns a human-readable summary of the stats.
func (s TelemetryStats) String() string {
	return fmt.Sprintf(
		"Telemetry{sent=%d, recv=%d, deleted=%d, skipped_stored=%d, skipped_evicted=%d, skipped_retrieved=%d, decrypt_errors=%d}",
		s.MessagesSent, s.MessagesReceived, s.KeysDeleted,
		s.SkippedKeysStored, s.SkippedKeysEvicted, s.SkippedKeysRetrieved,
		s.DecryptionErrors,
	)
}
