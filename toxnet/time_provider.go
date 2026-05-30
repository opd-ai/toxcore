package toxnet

import (
	"sync"
	"time"
)

// TimeProvider is an interface for getting the current time and creating tickers.
// This allows injecting a mock time provider for deterministic testing.
type TimeProvider interface {
	// Now returns the current time.
	Now() time.Time
	// NewTicker creates a new ticker that fires at the given interval.
	NewTicker(d time.Duration) *time.Ticker
	// NewTimer creates a new timer that fires after the given duration.
	NewTimer(d time.Duration) *time.Timer
}

// RealTimeProvider implements TimeProvider using the actual system time.
type RealTimeProvider struct{}

// Now returns the current system time.
func (RealTimeProvider) Now() time.Time {
	return time.Now()
}

// NewTicker creates a new ticker using the standard library.
func (RealTimeProvider) NewTicker(d time.Duration) *time.Ticker {
	return time.NewTicker(d)
}

// NewTimer creates a new timer using the standard library.
func (RealTimeProvider) NewTimer(d time.Duration) *time.Timer {
	return time.NewTimer(d)
}

var defaultTimeProviderMu sync.RWMutex

// defaultTimeProvider is the package-level default time provider.
// Used by types that don't have an explicitly set time provider.
var defaultTimeProvider TimeProvider = RealTimeProvider{}

// SetDefaultTimeProvider sets the package-level default time provider.
// This is primarily useful for testing to inject deterministic time.
func SetDefaultTimeProvider(tp TimeProvider) {
	if tp == nil {
		tp = RealTimeProvider{}
	}
	defaultTimeProviderMu.Lock()
	defaultTimeProvider = tp
	defaultTimeProviderMu.Unlock()
}

// getTimeProvider returns the provided TimeProvider if non-nil,
// otherwise returns the package-level default.
func getTimeProvider(tp TimeProvider) TimeProvider {
	if tp != nil {
		return tp
	}
	defaultTimeProviderMu.RLock()
	p := defaultTimeProvider
	defaultTimeProviderMu.RUnlock()
	return p
}

// setupDeadlineTimeout creates a timer for a given deadline.
// Returns nil if the deadline is zero (no timeout).
// The caller is responsible for stopping the timer to avoid resource leaks (F-TOXNET-L1).
func setupDeadlineTimeout(deadline time.Time) *time.Timer {
	if deadline.IsZero() {
		return nil
	}
	return time.NewTimer(time.Until(deadline))
}
