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

// getFilesystemStatistics retrieves filesystem statistics on Windows.
func getFilesystemStatistics(dir string) (totalBytes, availableBytes, usedBytes uint64, err error) {
	return getWindowsFilesystemStats(dir)
}

// getWindowsDiskSpace retrieves disk space information on Windows using GetDiskFreeSpaceEx
func getWindowsDiskSpace(dir string) (totalBytes, availableBytes, usedBytes uint64, err error) {
	absPath, err := validateAndPreparePath(dir)
	if err != nil {
		return 0, 0, 0, err
	}

	pathPtr, err := syscall.UTF16PtrFromString(absPath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to convert path to UTF-16: %w", err)
	}

	totalBytes, availableBytes, usedBytes, err = callWindowsDiskSpaceAPI(pathPtr, absPath)
	if err != nil {
		return 0, 0, 0, err
	}

	logWindowsDiskSpaceInfo(absPath, totalBytes, availableBytes, usedBytes)
	return totalBytes, availableBytes, usedBytes, nil
}

func validateAndPreparePath(dir string) (string, error) {
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		if err := os.MkdirAll(absPath, 0o755); err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}
	}

	return absPath, nil
}

func callWindowsDiskSpaceAPI(pathPtr *uint16, absPath string) (totalBytes, availableBytes, usedBytes uint64, err error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx := kernel32.NewProc("GetDiskFreeSpaceExW")

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64

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

	return totalNumberOfBytes, freeBytesAvailable, totalNumberOfBytes - totalNumberOfFreeBytes, nil
}

func logWindowsDiskSpaceInfo(absPath string, totalBytes, availableBytes, usedBytes uint64) {
	logrus.WithFields(logrus.Fields{
		"function":        "getWindowsDiskSpace",
		"path":            absPath,
		"total_bytes":     totalBytes,
		"available_bytes": availableBytes,
		"used_bytes":      usedBytes,
	}).Debug("Retrieved Windows disk space information")
}
