package crypto

import "time"

// TimeProvider abstracts time operations for deterministic testing.
// Implementations must be safe for concurrent use.
type TimeProvider interface {
	Now() time.Time
	Since(t time.Time) time.Duration
}

// DefaultTimeProvider uses the standard library time functions.
type DefaultTimeProvider struct{}

// Now returns the current time.
func (DefaultTimeProvider) Now() time.Time { return time.Now() }

// Since returns the duration since the given time.
func (DefaultTimeProvider) Since(t time.Time) time.Duration { return time.Since(t) }

// defaultTimeProvider is the package-level default for functions that need time.
var defaultTimeProvider TimeProvider = DefaultTimeProvider{}

// SetDefaultTimeProvider sets the package-level time provider for testing.
// Pass nil to reset to the default implementation.
func SetDefaultTimeProvider(tp TimeProvider) {
	if tp == nil {
		tp = DefaultTimeProvider{}
	}
	defaultTimeProvider = tp
}

// GetDefaultTimeProvider returns the current package-level time provider.
func GetDefaultTimeProvider() TimeProvider {
	return defaultTimeProvider
}
