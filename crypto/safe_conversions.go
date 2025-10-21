package crypto

import (
	"fmt"
	"math"
)

// safeUint64ToInt64 safely converts uint64 to int64, checking for overflow.
// This prevents potential security issues from integer overflow in timestamp conversions.
//
// CWE-190: Integer Overflow or Wraparound
// gosec G115: Integer overflow check
func safeUint64ToInt64(val uint64) (int64, error) {
	if val > math.MaxInt64 {
		return 0, fmt.Errorf("uint64 value exceeds int64 max: %d (max: %d)", val, math.MaxInt64)
	}
	return int64(val), nil
}

// safeInt64ToUint64 safely converts int64 to uint64, checking for negative values.
// This prevents potential security issues from negative timestamp conversions.
//
// CWE-190: Integer Overflow or Wraparound
// gosec G115: Integer overflow check
func safeInt64ToUint64(val int64) (uint64, error) {
	if val < 0 {
		return 0, fmt.Errorf("cannot convert negative int64 to uint64: %d", val)
	}
	return uint64(val), nil
}

// safeDurationToUint64 safely converts time.Duration to uint64.
// Returns an error if the duration is negative or would overflow.
//
// CWE-190: Integer Overflow or Wraparound
func safeDurationToUint64(d int64) (uint64, error) {
	if d < 0 {
		return 0, fmt.Errorf("cannot convert negative duration to uint64: %d", d)
	}
	// Duration is int64 nanoseconds, converting to uint64 is safe for positive values
	return uint64(d), nil
}
