//go:build !windows
// +build !windows

package async

import "errors"

// getWindowsDiskSpace is a stub for non-Windows platforms.
// Returns an error instead of panicking for graceful failure handling
// if build tags are misconfigured or cross-compilation issues occur.
func getWindowsDiskSpace(dir string) (totalBytes, availableBytes, usedBytes uint64, err error) {
	return 0, 0, 0, errors.New("getWindowsDiskSpace not available on non-Windows platforms")
}
