// Package internal provides the core components for the Tox network integration test suite.
package internal

import "time"

// TimeProvider abstracts time operations to enable deterministic testing.
// By default, implementations use the system clock, but tests can inject
// a mock implementation that returns predictable timestamps.
//
// Usage for deterministic testing:
//
//	mockTime := NewMockTimeProvider(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
//	server.SetTimeProvider(mockTime)
//	mockTime.Advance(5 * time.Second)
type TimeProvider interface {
	// Now returns the current time.
	Now() time.Time

	// Since returns the duration elapsed since t.
	Since(t time.Time) time.Duration
}

// DefaultTimeProvider implements TimeProvider using the system clock.
// This is the production implementation used by default.
type DefaultTimeProvider struct{}

// NewDefaultTimeProvider creates a new DefaultTimeProvider.
func NewDefaultTimeProvider() *DefaultTimeProvider {
	return &DefaultTimeProvider{}
}

// Now returns the current system time.
func (p *DefaultTimeProvider) Now() time.Time {
	return time.Now()
}

// Since returns the duration elapsed since t using the system clock.
func (p *DefaultTimeProvider) Since(t time.Time) time.Duration {
	return time.Since(t)
}

// MockTimeProvider is a test implementation that allows controlling time.
// It maintains an internal clock that can be advanced programmatically,
// enabling deterministic test scenarios without relying on real time.
type MockTimeProvider struct {
	currentTime time.Time
}

// NewMockTimeProvider creates a MockTimeProvider starting at the specified time.
func NewMockTimeProvider(startTime time.Time) *MockTimeProvider {
	return &MockTimeProvider{currentTime: startTime}
}

// Now returns the mock's current time.
func (m *MockTimeProvider) Now() time.Time {
	return m.currentTime
}

// Since returns the duration since t based on the mock's current time.
func (m *MockTimeProvider) Since(t time.Time) time.Duration {
	return m.currentTime.Sub(t)
}

// Advance moves the mock time forward by the specified duration.
// Use this to simulate time passage in tests without waiting.
func (m *MockTimeProvider) Advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

// Set updates the mock time to the specified time.
// Useful for testing specific timestamp scenarios.
func (m *MockTimeProvider) Set(t time.Time) {
	m.currentTime = t
}

// defaultTimeProvider is the package-level default used when no provider is set.
var defaultTimeProvider TimeProvider = NewDefaultTimeProvider()

// getTimeProvider returns the provided TimeProvider or the default if nil.
func getTimeProvider(tp TimeProvider) TimeProvider {
	if tp != nil {
		return tp
	}
	return defaultTimeProvider
}
