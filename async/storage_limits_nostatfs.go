//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !windows
// +build !linux,!darwin,!freebsd,!openbsd,!netbsd,!windows

package async

// getFilesystemStatistics returns conservative default values on platforms
// where statfs is not available (e.g., WASM, Plan 9, etc.).
func getFilesystemStatistics(dir string) (totalBytes, availableBytes, usedBytes uint64, err error) {
	return getDefaultFilesystemStats(dir)
}
