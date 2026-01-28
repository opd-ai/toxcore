package async

import (
	"os"
	"runtime"
	"testing"
)

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
