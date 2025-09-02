package async

import (
	"os"
	"path/filepath"
	"syscall"
)

// StorageInfo contains information about available storage
type StorageInfo struct {
	TotalBytes     uint64
	AvailableBytes uint64
	UsedBytes      uint64
}

// GetStorageInfo returns storage information for the given path
// On Unix systems, this uses the statfs system call
func GetStorageInfo(path string) (*StorageInfo, error) {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// Get the directory containing the path (in case path is a file)
	dir := filepath.Dir(absPath)

	// Ensure directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Try to create the directory
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	// Get filesystem statistics
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return nil, err
	}

	// Calculate storage information
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	availableBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - (stat.Bfree * uint64(stat.Bsize))

	return &StorageInfo{
		TotalBytes:     totalBytes,
		AvailableBytes: availableBytes,
		UsedBytes:      usedBytes,
	}, nil
}

// CalculateAsyncStorageLimit calculates the maximum bytes to use for async storage
// This is set to 1% of total available storage
func CalculateAsyncStorageLimit(path string) (uint64, error) {
	info, err := GetStorageInfo(path)
	if err != nil {
		return 0, err
	}

	// Use 1% of total storage for async messages
	onePercentOfTotal := info.TotalBytes / 100

	// Ensure we have a reasonable minimum (1MB) and maximum (1GB)
	const minLimit = 1024 * 1024        // 1MB minimum
	const maxLimit = 1024 * 1024 * 1024 // 1GB maximum

	if onePercentOfTotal < minLimit {
		return minLimit, nil
	}
	if onePercentOfTotal > maxLimit {
		return maxLimit, nil
	}

	return onePercentOfTotal, nil
}

// EstimateMessageCapacity estimates how many messages can be stored given a byte limit
func EstimateMessageCapacity(bytesLimit uint64) int {
	// Average message size estimation:
	// - AsyncMessage struct overhead: ~150 bytes
	// - Average encrypted message: ~500 bytes
	// Total average: ~650 bytes per message
	const avgMessageSize = 650

	capacity := int(bytesLimit / avgMessageSize)

	// Ensure we have reasonable bounds
	const minCapacity = 100
	const maxCapacity = 100000

	if capacity < minCapacity {
		return minCapacity
	}
	if capacity > maxCapacity {
		return maxCapacity
	}

	return capacity
}
