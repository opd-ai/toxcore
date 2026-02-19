package messaging

import "time"

// Test timeout durations for async processing waits.
const (
	testAsyncWait       = 100 * time.Millisecond
	testAsyncWaitShort  = 50 * time.Millisecond
	testAsyncWaitMedium = 200 * time.Millisecond
	testAsyncWaitLong   = 300 * time.Millisecond
	testGoroutineStart  = 10 * time.Millisecond
	testCloseTimeout    = 2 * time.Second
	testSlowDelay       = 100 * time.Millisecond
)

// Test friend identifiers.
const (
	testDefaultFriendID     = 1
	testMultiFriendCount    = 3
	testConcurrentFriendMax = 10
	testInvalidFriendID     = 999
)

// Test retry configuration.
const (
	testReducedRetries   = 2
	testSingleRetry      = 1
	testRetryInterval    = 5 * time.Second
	testRetryAdvanceStep = 3 * time.Second
)

// Test message and packet constants.
const (
	testLongMessageSize     = 1000
	testBinaryIterations    = 5
	testPacketTypeFriendMsg = 0x01
)
