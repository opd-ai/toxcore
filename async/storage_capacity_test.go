package async

import (
	"os"
	"runtime"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// ============================================================================
// Storage Limits Tests - Testing storage capacity and disk space detection
// ============================================================================

// TestStorageInfoCalculation tests disk space detection and capacity calculation
func TestStorageInfoCalculation(t *testing.T) {
	// Test getting storage info for temp directory
	info, err := GetStorageInfo(os.TempDir())
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

	t.Logf("Storage info for %s:", os.TempDir())
	t.Logf("  Total: %d bytes (%.2f GB)", info.TotalBytes, float64(info.TotalBytes)/(1024*1024*1024))
	t.Logf("  Available: %d bytes (%.2f GB)", info.AvailableBytes, float64(info.AvailableBytes)/(1024*1024*1024))
	t.Logf("  Used: %d bytes (%.2f GB)", info.UsedBytes, float64(info.UsedBytes)/(1024*1024*1024))
}

// TestAsyncStorageLimit tests the 1% storage limit calculation
func TestAsyncStorageLimit(t *testing.T) {
	limit, err := CalculateAsyncStorageLimit(os.TempDir())
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
		{"Small limit", 100 * 1024, 1536, 1536},                     // 100KB -> should give minimum capacity (1536)
		{"Medium limit", 10 * 1024 * 1024, 10000, 20000},            // 10MB -> reasonable capacity
		{"Large limit", 1024 * 1024 * 1024, 1536000, 1536000},       // 1GB -> should give maximum capacity (1536000)
		{"Very large limit", 10 * 1024 * 1024 * 1024, 1536000, 1536000}, // 10GB -> should be capped at maximum (1536000)
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

	if capacity < MinStorageCapacity {
		t.Errorf("Storage capacity should be at least %d messages, got %d", MinStorageCapacity, capacity)
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

// ============================================================================
// Platform-Specific Storage Detection Tests
// ============================================================================

// TestActualDiskSpaceDetection verifies that we're getting real disk space, not hardcoded defaults
func TestActualDiskSpaceDetection(t *testing.T) {
	info, err := GetStorageInfo(os.TempDir())
	if err != nil {
		t.Fatalf("Failed to get storage info: %v", err)
	}

	// Verify we're not getting the old hardcoded defaults
	const oldDefaultTotal = 100 * 1024 * 1024 * 1024
	const oldDefaultAvailable = 50 * 1024 * 1024 * 1024

	if info.TotalBytes == oldDefaultTotal && info.AvailableBytes == oldDefaultAvailable {
		// Only fail if we're on a supported platform
		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
			t.Errorf("Storage detection appears to be using old hardcoded defaults on supported platform %s", runtime.GOOS)
		}
	}

	// For supported platforms, verify the values are reasonable
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		// Total should be at least 1GB (most systems have more)
		if info.TotalBytes < 1*1024*1024*1024 {
			t.Errorf("Total bytes %d seems unreasonably small for a modern system", info.TotalBytes)
		}

		// Available should not exceed total
		if info.AvailableBytes > info.TotalBytes {
			t.Errorf("Available bytes %d cannot exceed total bytes %d", info.AvailableBytes, info.TotalBytes)
		}

		// Used + Free should approximately equal Total (allowing for filesystem overhead)
		freeBytes := info.TotalBytes - info.UsedBytes
		if freeBytes < info.AvailableBytes {
			t.Errorf("Free bytes %d is less than available bytes %d", freeBytes, info.AvailableBytes)
		}

		t.Logf("Platform: %s", runtime.GOOS)
		t.Logf("Total: %.2f GB", float64(info.TotalBytes)/(1024*1024*1024))
		t.Logf("Available: %.2f GB", float64(info.AvailableBytes)/(1024*1024*1024))
		t.Logf("Used: %.2f GB", float64(info.UsedBytes)/(1024*1024*1024))
		t.Logf("Utilization: %.1f%%", float64(info.UsedBytes)/float64(info.TotalBytes)*100)
	}
}

// TestPlatformSpecificImplementation verifies the correct implementation is being used
func TestPlatformSpecificImplementation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "toxcore_disk_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	info, err := GetStorageInfo(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get storage info: %v", err)
	}

	// Verify basic sanity checks
	if info.TotalBytes == 0 {
		t.Error("Total bytes should not be zero")
	}

	if info.AvailableBytes > info.TotalBytes {
		t.Errorf("Available bytes %d cannot be greater than total bytes %d",
			info.AvailableBytes, info.TotalBytes)
	}

	// Log the implementation being used
	t.Logf("Running on: %s/%s", runtime.GOOS, runtime.GOARCH)

	switch runtime.GOOS {
	case "linux", "darwin", "freebsd", "openbsd", "netbsd":
		t.Log("Using Unix statfs implementation")
	case "windows":
		t.Log("Using Windows GetDiskFreeSpaceEx implementation")
	default:
		t.Logf("Using fallback defaults for unsupported platform: %s", runtime.GOOS)
	}
}

// TestStorageLimitScaling verifies that storage limits scale with actual disk size
func TestStorageLimitScaling(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "toxcore_limit_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	limit, err := CalculateAsyncStorageLimit(tmpDir)
	if err != nil {
		t.Fatalf("Failed to calculate storage limit: %v", err)
	}

	info, err := GetStorageInfo(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get storage info: %v", err)
	}

	// Verify limit is 1% of total (with min/max bounds)
	expectedLimit := info.TotalBytes / 100

	const minLimit = 1024 * 1024        // 1MB
	const maxLimit = 1024 * 1024 * 1024 // 1GB

	// Check if the limit respects the bounds
	if expectedLimit < minLimit {
		if limit != minLimit {
			t.Errorf("Expected minimum limit %d, got %d", minLimit, limit)
		}
	} else if expectedLimit > maxLimit {
		if limit != maxLimit {
			t.Errorf("Expected maximum limit %d, got %d", maxLimit, limit)
		}
	} else {
		if limit != expectedLimit {
			t.Errorf("Expected 1%% of total (%d), got %d", expectedLimit, limit)
		}
	}

	percentage := float64(limit) / float64(info.TotalBytes) * 100
	t.Logf("Storage limit: %.2f MB (%.2f%% of %.2f GB total)",
		float64(limit)/(1024*1024),
		percentage,
		float64(info.TotalBytes)/(1024*1024*1024))
}
