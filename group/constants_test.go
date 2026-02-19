package group

import "time"

// Common test network addresses used across multiple test files.
const (
	testLocalAddress = "127.0.0.1:33445"
	testPeerAddress  = "192.168.1.100:33445"
)

// Common test timeout durations used across multiple test files.
const (
	testAsyncWait   = 100 * time.Millisecond
	testShortWait   = 50 * time.Millisecond
	testMinimalWait = 5 * time.Millisecond
	testAnnounceTTL = 24 * time.Hour
)

// Common test peer IDs used across multiple test files.
const (
	testDefaultPeerID     = uint32(100)
	testNonExistentPeerID = uint32(999)
)

// Common goroutine and concurrency counts used across multiple test files.
const (
	testSmallConcurrency  = 10
	testMediumConcurrency = 20
	testLargeConcurrency  = 50
	testMaxConcurrency    = 100
)

// testWorkerPoolLimit is the maximum number of concurrent broadcast workers.
const testWorkerPoolLimit = 10
