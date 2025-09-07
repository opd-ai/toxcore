package async

import (
	"os"
	"path/filepath"
	"syscall"

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
		if err := os.MkdirAll(dir, 0755); err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "GetStorageInfo",
				"dir":      dir,
				"error":    err.Error(),
			}).Error("Failed to create directory")
			return nil, err
		}
	}

	// Get filesystem statistics
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "GetStorageInfo",
			"dir":      dir,
			"error":    err.Error(),
		}).Error("Failed to get filesystem statistics")
		return nil, err
	}

	// Calculate storage information
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	availableBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - (stat.Bfree * uint64(stat.Bsize))

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
// This is set to 1% of total available storage
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

	// Use 1% of total storage for async messages
	onePercentOfTotal := info.TotalBytes / 100

	// Ensure we have a reasonable minimum (1MB) and maximum (1GB)
	const minLimit = 1024 * 1024        // 1MB minimum
	const maxLimit = 1024 * 1024 * 1024 // 1GB maximum

	var finalLimit uint64
	if onePercentOfTotal < minLimit {
		finalLimit = minLimit
		logrus.WithFields(logrus.Fields{
			"function":             "CalculateAsyncStorageLimit",
			"calculated_1_percent": onePercentOfTotal,
			"applied_minimum":      minLimit,
		}).Debug("Applied minimum storage limit")
	} else if onePercentOfTotal > maxLimit {
		finalLimit = maxLimit
		logrus.WithFields(logrus.Fields{
			"function":             "CalculateAsyncStorageLimit",
			"calculated_1_percent": onePercentOfTotal,
			"applied_maximum":      maxLimit,
		}).Debug("Applied maximum storage limit")
	} else {
		finalLimit = onePercentOfTotal
		logrus.WithFields(logrus.Fields{
			"function":             "CalculateAsyncStorageLimit",
			"calculated_1_percent": onePercentOfTotal,
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

	// Ensure we have reasonable bounds
	const minCapacity = 100
	const maxCapacity = 100000

	var finalCapacity int
	if capacity < minCapacity {
		finalCapacity = minCapacity
		logrus.WithFields(logrus.Fields{
			"function":        "EstimateMessageCapacity",
			"calculated":      capacity,
			"applied_minimum": minCapacity,
		}).Debug("Applied minimum capacity")
	} else if capacity > maxCapacity {
		finalCapacity = maxCapacity
		logrus.WithFields(logrus.Fields{
			"function":        "EstimateMessageCapacity",
			"calculated":      capacity,
			"applied_maximum": maxCapacity,
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
