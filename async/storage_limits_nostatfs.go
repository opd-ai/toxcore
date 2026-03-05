//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd

package async

import "errors"

// getUnixFilesystemStats is a stub for platforms without statfs support (js/wasm, etc.).
// Returns an error; callers should fall back to default filesystem statistics.
func getUnixFilesystemStats(dir string) (totalBytes, availableBytes, usedBytes uint64, err error) {
	return 0, 0, 0, errors.New("getUnixFilesystemStats not available on this platform")
}
