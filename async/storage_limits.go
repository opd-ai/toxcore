package async

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
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

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "GetStorageInfo",
			"path":     path,
			"error":    err.Error(),
		}).Error("Failed to get absolute path")
		return nil, err
	}

	// Get the directory containing the path (in case path is a file)
	dir := filepath.Dir(absPath)

	logrus.WithFields(logrus.Fields{
		"function": "GetStorageInfo",
		"abs_path": absPath,
		"dir":      dir,
	}).Debug("Resolved directory path")

	// Ensure directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		logrus.WithFields(logrus.Fields{
			"function": "GetStorageInfo",
			"dir":      dir,
		}).Debug("Directory does not exist, creating")

		// Try to create the directory
		if err := os.MkdirAll(dir, 0o755); err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "GetStorageInfo",
				"dir":      dir,
				"error":    err.Error(),
			}).Error("Failed to create directory")
			return nil, err
		}
	}

	// Try to get file info to verify directory exists
	fileInfo, err := os.Stat(dir)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "GetStorageInfo",
			"dir":      dir,
			"error":    err.Error(),
		}).Error("Failed to stat directory")
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}

	if !fileInfo.IsDir() {
		err := fmt.Errorf("path is not a directory")
		logrus.WithFields(logrus.Fields{
			"function": "GetStorageInfo",
			"dir":      dir,
		}).Error("Path is not a directory")
		return nil, err
	}

	// Get filesystem statistics using platform-specific syscalls
	var totalBytes, availableBytes, usedBytes uint64

	switch runtime.GOOS {
	case "windows":
		// Windows uses GetDiskFreeSpaceEx API
		var err error
		totalBytes, availableBytes, usedBytes, err = getWindowsDiskSpace(dir)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "GetStorageInfo",
				"dir":      dir,
				"error":    err.Error(),
			}).Error("Failed to get Windows disk space, falling back to defaults")

			// Fall back to conservative defaults
			const (
				defaultTotalBytes     uint64 = 100 * 1024 * 1024 * 1024
				defaultAvailableBytes uint64 = 50 * 1024 * 1024 * 1024
			)
			totalBytes = defaultTotalBytes
			availableBytes = defaultAvailableBytes
			usedBytes = totalBytes - availableBytes
		}

	case "linux", "darwin", "freebsd", "openbsd", "netbsd":
		// Unix-like systems use statfs
		var stat unix.Statfs_t
		if err := unix.Statfs(dir, &stat); err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "GetStorageInfo",
				"dir":      dir,
				"error":    err.Error(),
			}).Error("Failed to get filesystem stats via statfs")
			return nil, fmt.Errorf("failed to get filesystem stats: %w", err)
		}

		// Calculate total, available, and used bytes
		// Bsize is the filesystem block size
		// Blocks is total data blocks in filesystem
		// Bavail is free blocks available to unprivileged user
		totalBytes = uint64(stat.Blocks) * uint64(stat.Bsize)
		availableBytes = uint64(stat.Bavail) * uint64(stat.Bsize)
		usedBytes = totalBytes - (uint64(stat.Bfree) * uint64(stat.Bsize))

	default:
		// For unsupported platforms, use conservative defaults
		logrus.WithFields(logrus.Fields{
			"function": "GetStorageInfo",
			"os":       runtime.GOOS,
		}).Warn("Platform-specific disk space detection not supported, using defaults")

		const (
			defaultTotalBytes     uint64 = 100 * 1024 * 1024 * 1024
			defaultAvailableBytes uint64 = 50 * 1024 * 1024 * 1024
		)

		totalBytes = defaultTotalBytes
		availableBytes = defaultAvailableBytes
		usedBytes = totalBytes - availableBytes
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

	// Use 1% of available storage for async messages
	onePercentOfAvailable := info.AvailableBytes / 100

	// Ensure we have a reasonable minimum (1MB) and maximum (1GB)
	const minLimit = 1024 * 1024        // 1MB minimum
	const maxLimit = 1024 * 1024 * 1024 // 1GB maximum

	var finalLimit uint64
	if onePercentOfAvailable < minLimit {
		finalLimit = minLimit
		logrus.WithFields(logrus.Fields{
			"function":             "CalculateAsyncStorageLimit",
			"calculated_1_percent": onePercentOfAvailable,
			"applied_minimum":      minLimit,
		}).Debug("Applied minimum storage limit")
	} else if onePercentOfAvailable > maxLimit {
		finalLimit = maxLimit
		logrus.WithFields(logrus.Fields{
			"function":             "CalculateAsyncStorageLimit",
			"calculated_1_percent": onePercentOfAvailable,
			"applied_maximum":      maxLimit,
		}).Debug("Applied maximum storage limit")
	} else {
		finalLimit = onePercentOfAvailable
		logrus.WithFields(logrus.Fields{
			"function":             "CalculateAsyncStorageLimit",
			"calculated_1_percent": onePercentOfAvailable,
		}).Debug("Applied calculated 1% storage limit")
	}

	logrus.WithFields(logrus.Fields{
		"function":    "CalculateAsyncStorageLimit",
		"total_bytes": info.TotalBytes,
		"final_limit": finalLimit,
		"limit_mb":    finalLimit / (1024 * 1024),
	}).Info("Async storage limit calculated successfully")

	return finalLimit, nil
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
