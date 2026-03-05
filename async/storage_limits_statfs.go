//go:build linux || darwin || freebsd || openbsd || netbsd
// +build linux darwin freebsd openbsd netbsd

package async

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// getFilesystemStatistics retrieves filesystem statistics using statfs on Unix-like platforms.
func getFilesystemStatistics(dir string) (totalBytes, availableBytes, usedBytes uint64, err error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(dir, &stat); err != nil {
		logrus.WithFields(logrus.Fields{
			"function": "getFilesystemStatistics",
			"dir":      dir,
			"error":    err,
		}).Error("Failed to get filesystem stats via statfs")
		return 0, 0, 0, fmt.Errorf("failed to get filesystem stats: %w", err)
	}

	totalBytes = uint64(stat.Blocks) * uint64(stat.Bsize)
	availableBytes = uint64(stat.Bavail) * uint64(stat.Bsize)
	usedBytes = totalBytes - (uint64(stat.Bfree) * uint64(stat.Bsize))

	return totalBytes, availableBytes, usedBytes, nil
}
