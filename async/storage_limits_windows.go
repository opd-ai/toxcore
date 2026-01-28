//go:build windows
// +build windows

package async

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/sirupsen/logrus"
)

// getWindowsDiskSpace retrieves disk space information on Windows using GetDiskFreeSpaceEx
func getWindowsDiskSpace(dir string) (totalBytes, availableBytes, usedBytes uint64, err error) {
	// Get absolute path
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Ensure directory exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		if err := os.MkdirAll(absPath, 0o755); err != nil {
			return 0, 0, 0, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Convert path to UTF-16 pointer for Windows API
	pathPtr, err := syscall.UTF16PtrFromString(absPath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to convert path to UTF-16: %w", err)
	}

	// Load kernel32.dll
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx := kernel32.NewProc("GetDiskFreeSpaceExW")

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64

	// Call GetDiskFreeSpaceExW
	ret, _, err := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)

	if ret == 0 {
		logrus.WithFields(logrus.Fields{
			"function": "getWindowsDiskSpace",
			"path":     absPath,
			"error":    err.Error(),
		}).Error("GetDiskFreeSpaceExW failed")
		return 0, 0, 0, fmt.Errorf("GetDiskFreeSpaceExW failed: %w", err)
	}

	totalBytes = totalNumberOfBytes
	availableBytes = freeBytesAvailable
	usedBytes = totalNumberOfBytes - totalNumberOfFreeBytes

	logrus.WithFields(logrus.Fields{
		"function":        "getWindowsDiskSpace",
		"path":            absPath,
		"total_bytes":     totalBytes,
		"available_bytes": availableBytes,
		"used_bytes":      usedBytes,
	}).Debug("Retrieved Windows disk space information")

	return totalBytes, availableBytes, usedBytes, nil
}
