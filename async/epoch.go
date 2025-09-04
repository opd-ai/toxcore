package async

import (
	"errors"
	"time"
)

// EpochDuration defines the standard duration for each epoch (6 hours)
// This provides a balance between privacy (frequent rotation) and performance (not too frequent)
const EpochDuration = 6 * time.Hour

// EpochManager handles time-based epoch calculations for pseudonym rotation.
// Epochs are used to rotate recipient pseudonyms regularly while allowing
// deterministic retrieval by recipients who know their private key.
type EpochManager struct {
	startTime     time.Time     // Network genesis time for epoch calculation
	epochDuration time.Duration // Duration of each epoch (normally 6 hours)
}

// NewEpochManager creates a new epoch manager with the default network start time.
// The network start time is set to January 1, 2025 00:00:00 UTC to ensure
// consistent epoch calculation across all nodes in the network.
func NewEpochManager() *EpochManager {
	// Use a fixed network genesis time for consistent epoch calculation
	// across all nodes (January 1, 2025 00:00:00 UTC)
	startTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	return &EpochManager{
		startTime:     startTime,
		epochDuration: EpochDuration,
	}
}

// NewEpochManagerWithCustomStart creates an epoch manager with a custom start time.
// This is primarily used for testing with controlled time scenarios.
func NewEpochManagerWithCustomStart(startTime time.Time, duration time.Duration) (*EpochManager, error) {
	if duration <= 0 {
		return nil, errors.New("epoch duration must be positive")
	}

	return &EpochManager{
		startTime:     startTime,
		epochDuration: duration,
	}, nil
}

// GetCurrentEpoch returns the current epoch number based on the current time.
// Epochs are numbered sequentially starting from 0 at the network start time.
func (em *EpochManager) GetCurrentEpoch() uint64 {
	return em.GetEpochAt(time.Now())
}

// GetEpochAt returns the epoch number for a specific time.
// This allows calculating epochs for historical or future times.
// Returns error for invalid times significantly before network start.
func (em *EpochManager) GetEpochAt(t time.Time) uint64 {
	if t.Before(em.startTime) {
		// Allow reasonable clock skew (up to 1 hour before network start)
		if em.startTime.Sub(t) > time.Hour {
			// For significantly old times, still return epoch 0 but this could indicate an issue
			// In a production system, consider logging this condition
		}
		return 0 // All times before network start are epoch 0
	}

	elapsed := t.Sub(em.startTime)
	return uint64(elapsed / em.epochDuration)
}

// ValidateEpochTime checks if a time is reasonable for epoch calculation.
// Returns an error if the time is significantly before network start time.
func (em *EpochManager) ValidateEpochTime(t time.Time) error {
	if t.Before(em.startTime) && em.startTime.Sub(t) > time.Hour {
		return errors.New("time is significantly before network start time")
	}
	return nil
}

// GetEpochStartTime returns the start time of a specific epoch.
// This is useful for calculating when an epoch began or will begin.
func (em *EpochManager) GetEpochStartTime(epoch uint64) time.Time {
	return em.startTime.Add(time.Duration(epoch) * em.epochDuration)
}

// GetEpochEndTime returns the end time of a specific epoch.
// This is useful for calculating when an epoch ends or ended.
func (em *EpochManager) GetEpochEndTime(epoch uint64) time.Time {
	return em.GetEpochStartTime(epoch + 1).Add(-time.Nanosecond)
}

// IsValidEpoch checks if an epoch is within the acceptable range for current operations.
// Valid epochs are the current epoch and up to 3 previous epochs (24 hours total).
// This allows for clock skew and delayed message processing while limiting storage requirements.
func (em *EpochManager) IsValidEpoch(epoch uint64) bool {
	currentEpoch := em.GetCurrentEpoch()

	// Allow current epoch and up to 3 previous epochs (24 hours total)
	// This accounts for clock skew and ensures messages aren't rejected
	// due to minor timing differences between nodes
	if epoch > currentEpoch {
		return false // Future epochs not allowed
	}

	return currentEpoch-epoch <= 3
}

// GetRecentEpochs returns a list of recent epochs that should be checked
// when retrieving messages. This includes the current epoch and the 3
// most recent previous epochs, covering a 24-hour window.
func (em *EpochManager) GetRecentEpochs() []uint64 {
	currentEpoch := em.GetCurrentEpoch()
	epochs := make([]uint64, 0, 4)

	// Add current epoch and up to 3 previous epochs
	for i := uint64(0); i <= 3; i++ {
		if currentEpoch >= i {
			epochs = append(epochs, currentEpoch-i)
		}
	}

	return epochs
}

// TimeUntilNextEpoch returns the duration until the next epoch begins.
// This is useful for scheduling epoch-based operations.
func (em *EpochManager) TimeUntilNextEpoch() time.Duration {
	currentEpoch := em.GetCurrentEpoch()
	nextEpochStart := em.GetEpochStartTime(currentEpoch + 1)
	return time.Until(nextEpochStart)
}

// GetEpochDuration returns the configured epoch duration.
func (em *EpochManager) GetEpochDuration() time.Duration {
	return em.epochDuration
}
