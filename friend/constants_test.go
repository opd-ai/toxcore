package friend

import "time"

const (
	// testRecentThreshold is the maximum acceptable duration for
	// a timestamp to be considered recent in tests.
	testRecentThreshold = time.Second

	// testDelayDuration is a simulated delay used in last-seen tests.
	testDelayDuration = 2 * time.Second
)
