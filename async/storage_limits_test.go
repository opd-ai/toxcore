package async

import (
	"os"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// TestStorageInfoCalculation tests disk space detection and capacity calculation
func TestStorageInfoCalculation(t *testing.T) {
	// Test getting storage info for /tmp
	info, err := GetStorageInfo("/tmp")
	if err != nil {
		t.Fatalf("Failed to get storage info: %v", err)
	}

	if info.TotalBytes == 0 {
		t.Error("Total bytes should be greater than 0")
	}

	if info.AvailableBytes > info.TotalBytes {
		t.Error("Available bytes cannot be greater than total bytes")
	}

	if info.UsedBytes > info.TotalBytes {
		t.Error("Used bytes cannot be greater than total bytes")
	}

	t.Logf("Storage info for /tmp:")
	t.Logf("  Total: %d bytes (%.2f GB)", info.TotalBytes, float64(info.TotalBytes)/(1024*1024*1024))
	t.Logf("  Available: %d bytes (%.2f GB)", info.AvailableBytes, float64(info.AvailableBytes)/(1024*1024*1024))
	t.Logf("  Used: %d bytes (%.2f GB)", info.UsedBytes, float64(info.UsedBytes)/(1024*1024*1024))
}

// TestAsyncStorageLimit tests the 1% storage limit calculation
func TestAsyncStorageLimit(t *testing.T) {
	limit, err := CalculateAsyncStorageLimit("/tmp")
	if err != nil {
		t.Fatalf("Failed to calculate async storage limit: %v", err)
	}

	if limit == 0 {
		t.Error("Storage limit should be greater than 0")
	}

	// Check that limit is reasonable (between 1MB and 1GB)
	const minLimit = 1024 * 1024        // 1MB
	const maxLimit = 1024 * 1024 * 1024 // 1GB

	if limit < minLimit {
		t.Errorf("Storage limit %d is below minimum %d", limit, minLimit)
	}

	if limit > maxLimit {
		t.Errorf("Storage limit %d is above maximum %d", limit, maxLimit)
	}

	t.Logf("Async storage limit: %d bytes (%.2f MB)", limit, float64(limit)/(1024*1024))
}

// TestMessageCapacityEstimation tests message capacity calculation
func TestMessageCapacityEstimation(t *testing.T) {
	testCases := []struct {
		name        string
		bytesLimit  uint64
		expectedMin int
		expectedMax int
	}{
		{"Small limit", 100 * 1024, 100, 200},             // 100KB -> should give minimum capacity
		{"Medium limit", 10 * 1024 * 1024, 10000, 20000},  // 10MB -> reasonable capacity
		{"Large limit", 100 * 1024 * 1024, 50000, 100000}, // 100MB -> large capacity but capped
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			capacity := EstimateMessageCapacity(tc.bytesLimit)

			if capacity < tc.expectedMin {
				t.Errorf("Capacity %d is below expected minimum %d", capacity, tc.expectedMin)
			}

			if capacity > tc.expectedMax {
				t.Errorf("Capacity %d is above expected maximum %d", capacity, tc.expectedMax)
			}

			t.Logf("Bytes limit: %d, Estimated capacity: %d messages", tc.bytesLimit, capacity)
		})
	}
}

// TestDynamicCapacityStorage tests that storage nodes use dynamic capacity
func TestDynamicCapacityStorage(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create temporary directory for this test
	tmpDir, err := os.MkdirTemp("", "toxcore_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewMessageStorage(keyPair, tmpDir)

	// Check that capacity is set and reasonable
	capacity := storage.GetMaxCapacity()
	if capacity == 0 {
		t.Error("Storage capacity should be greater than 0")
	}

	if capacity < 100 {
		t.Error("Storage capacity should be at least 100 messages")
	}

	// Check initial utilization
	utilization := storage.GetStorageUtilization()
	if utilization != 0.0 {
		t.Errorf("Initial utilization should be 0%%, got %.1f%%", utilization)
	}

	t.Logf("Dynamic storage capacity: %d messages", capacity)
	t.Logf("Initial utilization: %.1f%%", utilization)
}

// TestCapacityUpdate tests that storage capacity can be updated
func TestCapacityUpdate(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create temporary directory for this test
	tmpDir, err := os.MkdirTemp("", "toxcore_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewMessageStorage(keyPair, tmpDir)

	initialCapacity := storage.GetMaxCapacity()

	// Update capacity
	err = storage.UpdateCapacity()
	if err != nil {
		t.Errorf("Failed to update capacity: %v", err)
	}

	newCapacity := storage.GetMaxCapacity()

	// Capacity should be the same or different depending on disk changes
	// For this test, we just verify the update process works
	t.Logf("Initial capacity: %d, Updated capacity: %d", initialCapacity, newCapacity)
}
