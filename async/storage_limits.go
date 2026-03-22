package async

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"
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
	logrus.WithFields(logrus.Fields{
		"function": "GetStorageInfo",
		"path":     path,
	}).Debug("Getting storage information")

	dir, err := resolveAndValidateDirectory(path)
	if err != nil {
		return nil, err
	}

	totalBytes, availableBytes, usedBytes, err := getFilesystemStatistics(dir)
	if err != nil {
		return nil, err
	}

	info := &StorageInfo{
		TotalBytes:     totalBytes,
		AvailableBytes: availableBytes,
		UsedBytes:      usedBytes,
	}

	logrus.WithFields(logrus.Fields{
		"function":        "GetStorageInfo",
		"total_bytes":     totalBytes,
		"available_bytes": availableBytes,
		"used_bytes":      usedBytes,
	}).Info("Storage information retrieved successfully")

	return info, nil
}

// resolveAndValidateDirectory resolves path to an absolute directory and ensures it exists.
func resolveAndValidateDirectory(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "resolveAndValidateDirectory",
			"path":     path,
			"error":    err.Error(),
		}).Error("Failed to get absolute path")
		return "", err
	}

	dir := filepath.Dir(absPath)

	logrus.WithFields(logrus.Fields{
		"function": "resolveAndValidateDirectory",
		"abs_path": absPath,
		"dir":      dir,
	}).Debug("Resolved directory path")

	if err := ensureDirectoryExists(dir); err != nil {
		return "", err
	}

	if err := validateIsDirectory(dir); err != nil {
		return "", err
	}

	return dir, nil
}

// ensureDirectoryExists creates the directory if it doesn't exist.
func ensureDirectoryExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		logrus.WithFields(logrus.Fields{
			"function": "ensureDirectoryExists",
			"dir":      dir,
		}).Debug("Directory does not exist, creating")

		if err := os.MkdirAll(dir, 0o755); err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "ensureDirectoryExists",
				"dir":      dir,
				"error":    err.Error(),
			}).Error("Failed to create directory")
			return err
		}
	}
	return nil
}

// validateIsDirectory checks if the path is actually a directory.
func validateIsDirectory(dir string) error {
	fileInfo, err := os.Stat(dir)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "validateIsDirectory",
			"dir":      dir,
			"error":    err.Error(),
		}).Error("Failed to stat directory")
		return fmt.Errorf("failed to stat directory: %w", err)
	}

	if !fileInfo.IsDir() {
		err := fmt.Errorf("path is not a directory")
		logrus.WithFields(logrus.Fields{
			"function": "validateIsDirectory",
			"dir":      dir,
		}).Error("Path is not a directory")
		return err
	}

	return nil
}

// getWindowsFilesystemStats retrieves Windows filesystem statistics.
func getWindowsFilesystemStats(dir string) (totalBytes, availableBytes, usedBytes uint64, err error) {
	totalBytes, availableBytes, usedBytes, err = getWindowsDiskSpace(dir)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "getWindowsFilesystemStats",
			"dir":      dir,
			"error":    err.Error(),
		}).Error("Failed to get Windows disk space, falling back to defaults")

		return getDefaultFilesystemStats(dir)
	}
	return totalBytes, availableBytes, usedBytes, nil
}

// getDefaultFilesystemStats returns conservative default values for unsupported platforms.
// This is used on platforms where statfs is not available (e.g., WASM, Plan 9).
//
// Default values:
//   - Total: 100 GB - assumes a typical modern storage device
//   - Available: 50 GB - conservative estimate assuming 50% free space
//
// Limitation: These are hardcoded values that don't reflect actual disk space.
// On WASM, there's no standard browser API to query disk quotas. Applications
// running on these platforms should monitor their storage usage manually or
// configure limits via CalculateAsyncStorageLimitWithMax().
func getDefaultFilesystemStats(dir string) (totalBytes, availableBytes, usedBytes uint64, err error) {
	logrus.WithFields(logrus.Fields{
		"function": "getDefaultFilesystemStats",
		"os":       runtime.GOOS,
	}).Warn("Platform-specific disk space detection not supported, using defaults")

	const (
		// defaultTotalBytes assumes a typical modern storage device (100 GB).
		defaultTotalBytes uint64 = 100 * 1024 * 1024 * 1024
		// defaultAvailableBytes assumes 50% free space as a conservative estimate.
		defaultAvailableBytes uint64 = 50 * 1024 * 1024 * 1024
	)

	totalBytes = defaultTotalBytes
	availableBytes = defaultAvailableBytes
	usedBytes = totalBytes - availableBytes

	return totalBytes, availableBytes, usedBytes, nil
}

// CalculateAsyncStorageLimit calculates the maximum bytes to use for async storage
// This is set to 1% of available storage
func CalculateAsyncStorageLimit(path string) (uint64, error) {
	logrus.WithFields(logrus.Fields{
		"function": "CalculateAsyncStorageLimit",
		"path":     path,
	}).Info("Calculating async storage limit")

	info, err := GetStorageInfo(path)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "CalculateAsyncStorageLimit",
			"path":     path,
			"error":    err.Error(),
		}).Error("Failed to get storage info")
		return 0, err
	}

	onePercentOfAvailable := info.AvailableBytes / 100
	finalLimit := applyStorageLimitConstraints(onePercentOfAvailable)
	logStorageLimitResult(info.TotalBytes, finalLimit)

	return finalLimit, nil
}

// CalculateAsyncStorageLimitWithMax calculates the storage limit with a custom maximum.
// This is useful for platforms like WASM where disk space detection is not available
// and the application wants to specify its own storage budget.
//
// Parameters:
//   - path: The storage path (used for logging and partial detection)
//   - maxBytes: The maximum bytes to allow (0 means use default max of 1GB)
//
// Returns the calculated limit, which will be min(1% of available, maxBytes).
func CalculateAsyncStorageLimitWithMax(path string, maxBytes uint64) (uint64, error) {
	logrus.WithFields(logrus.Fields{
		"function":  "CalculateAsyncStorageLimitWithMax",
		"path":      path,
		"max_bytes": maxBytes,
	}).Info("Calculating async storage limit with custom max")

	info, err := GetStorageInfo(path)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "CalculateAsyncStorageLimitWithMax",
			"path":     path,
			"error":    err.Error(),
		}).Error("Failed to get storage info")
		return 0, err
	}

	onePercentOfAvailable := info.AvailableBytes / 100

	// Apply custom max if provided
	if maxBytes > 0 && onePercentOfAvailable > maxBytes {
		logrus.WithFields(logrus.Fields{
			"function":             "CalculateAsyncStorageLimitWithMax",
			"calculated_1_percent": onePercentOfAvailable,
			"applied_custom_max":   maxBytes,
		}).Debug("Applied custom maximum storage limit")
		onePercentOfAvailable = maxBytes
	}

	finalLimit := applyStorageLimitConstraints(onePercentOfAvailable)
	logStorageLimitResult(info.TotalBytes, finalLimit)

	return finalLimit, nil
}

// applyStorageLimitConstraints applies minimum and maximum constraints to the calculated storage limit.
func applyStorageLimitConstraints(onePercentOfAvailable uint64) uint64 {
	const minLimit = 1024 * 1024        // 1MB minimum
	const maxLimit = 1024 * 1024 * 1024 // 1GB maximum

	if onePercentOfAvailable < minLimit {
		logrus.WithFields(logrus.Fields{
			"function":             "CalculateAsyncStorageLimit",
			"calculated_1_percent": onePercentOfAvailable,
			"applied_minimum":      minLimit,
		}).Debug("Applied minimum storage limit")
		return minLimit
	}

	if onePercentOfAvailable > maxLimit {
		logrus.WithFields(logrus.Fields{
			"function":             "CalculateAsyncStorageLimit",
			"calculated_1_percent": onePercentOfAvailable,
			"applied_maximum":      maxLimit,
		}).Debug("Applied maximum storage limit")
		return maxLimit
	}

	logrus.WithFields(logrus.Fields{
		"function":             "CalculateAsyncStorageLimit",
		"calculated_1_percent": onePercentOfAvailable,
	}).Debug("Applied calculated 1% storage limit")
	return onePercentOfAvailable
}

// logStorageLimitResult logs the final calculated storage limit.
func logStorageLimitResult(totalBytes, finalLimit uint64) {
	logrus.WithFields(logrus.Fields{
		"function":    "CalculateAsyncStorageLimit",
		"total_bytes": totalBytes,
		"final_limit": finalLimit,
		"limit_mb":    finalLimit / (1024 * 1024),
	}).Info("Async storage limit calculated successfully")
}

// EstimateMessageCapacity estimates how many messages can be stored given a byte limit
func EstimateMessageCapacity(bytesLimit uint64) int {
	logrus.WithFields(logrus.Fields{
		"function":    "EstimateMessageCapacity",
		"bytes_limit": bytesLimit,
		"limit_mb":    bytesLimit / (1024 * 1024),
	}).Debug("Estimating message capacity")

	// Average message size estimation:
	// - AsyncMessage struct overhead: ~150 bytes
	// - Average encrypted message: ~500 bytes
	// Total average: ~650 bytes per message
	const avgMessageSize = 650

	capacity := int(bytesLimit / avgMessageSize)

	// Ensure we have reasonable bounds using package constants
	var finalCapacity int
	if capacity < MinStorageCapacity {
		finalCapacity = MinStorageCapacity
		logrus.WithFields(logrus.Fields{
			"function":        "EstimateMessageCapacity",
			"calculated":      capacity,
			"applied_minimum": MinStorageCapacity,
		}).Debug("Applied minimum capacity")
	} else if capacity > MaxStorageCapacity {
		finalCapacity = MaxStorageCapacity
		logrus.WithFields(logrus.Fields{
			"function":        "EstimateMessageCapacity",
			"calculated":      capacity,
			"applied_maximum": MaxStorageCapacity,
		}).Debug("Applied maximum capacity")
	} else {
		finalCapacity = capacity
		logrus.WithFields(logrus.Fields{
			"function":   "EstimateMessageCapacity",
			"calculated": capacity,
		}).Debug("Applied calculated capacity")
	}

	logrus.WithFields(logrus.Fields{
		"function":       "EstimateMessageCapacity",
		"bytes_limit":    bytesLimit,
		"avg_msg_size":   avgMessageSize,
		"final_capacity": finalCapacity,
	}).Info("Message capacity estimated successfully")

	return finalCapacity
}
