//go:build !windows
// +build !windows

package async

// getWindowsDiskSpace is a stub for non-Windows platforms
// This function is only called on Windows, so it's safe to panic here
func getWindowsDiskSpace(dir string) (totalBytes, availableBytes, usedBytes uint64, err error) {
	panic("getWindowsDiskSpace should not be called on non-Windows platforms")
}
