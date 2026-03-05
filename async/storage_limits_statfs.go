//go:build linux || darwin || freebsd || openbsd || netbsd

package async

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// getUnixFilesystemStats retrieves Unix-like filesystem statistics using statfs.
func getUnixFilesystemStats(dir string) (totalBytes, availableBytes, usedBytes uint64, err error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(dir, &stat); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "getUnixFilesystemStats",
			"dir":      dir,
			"error":    err.Error(),
		}).Error("Failed to get filesystem stats via statfs")
		return 0, 0, 0, fmt.Errorf("failed to get filesystem stats: %w", err)
	}

	totalBytes = uint64(stat.Blocks) * uint64(stat.Bsize)
	availableBytes = uint64(stat.Bavail) * uint64(stat.Bsize)
	usedBytes = totalBytes - (uint64(stat.Bfree) * uint64(stat.Bsize))

	return totalBytes, availableBytes, usedBytes, nil
}
