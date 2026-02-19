package file

import "time"

// Test network configuration constants.
const (
	testIP        = "127.0.0.1"
	testPort      = 33445
	testPeerPort  = 33446
	testPeerPort2 = 33447

	testLocalAddr = "127.0.0.1:33445"
	testPeerAddr  = "127.0.0.1:33446"
	testPeerAddr2 = "127.0.0.1:33447"
)

// testDefaultStallTimeout matches the expected DefaultStallTimeout value.
const testDefaultStallTimeout = 30 * time.Second

// Common test file size constants.
const (
	testFileSize1KB = 1024
	testFileSize2KB = 2048
	testFileSize1GB = 1073741824
)
