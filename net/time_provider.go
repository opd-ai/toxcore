package net

import "time"

// TimeProvider is an interface for getting the current time.
// This allows injecting a mock time provider for deterministic testing.
type TimeProvider interface {
	Now() time.Time
}

// RealTimeProvider implements TimeProvider using the actual system time.
type RealTimeProvider struct{}

// Now returns the current system time.
func (RealTimeProvider) Now() time.Time {
	return time.Now()
}

// defaultTimeProvider is the package-level default time provider.
// Used by types that don't have an explicitly set time provider.
var defaultTimeProvider TimeProvider = RealTimeProvider{}

// SetDefaultTimeProvider sets the package-level default time provider.
// This is primarily useful for testing to inject deterministic time.
func SetDefaultTimeProvider(tp TimeProvider) {
	if tp == nil {
		tp = RealTimeProvider{}
	}
	defaultTimeProvider = tp
}

// getTimeProvider returns the provided TimeProvider if non-nil,
// otherwise returns the package-level default.
func getTimeProvider(tp TimeProvider) TimeProvider {
	if tp != nil {
		return tp
	}
	return defaultTimeProvider
}
