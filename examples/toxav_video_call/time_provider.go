// ToxAV Video Call Demo - Time Provider
//
// This file provides an injectable time provider interface for deterministic testing.
// The TimeProvider abstraction allows replacing system time with mock time in tests.

package main

import "time"

// TimeProvider is an interface for getting the current time.
// This allows injecting a mock time provider for deterministic testing.
type TimeProvider interface {
	// Now returns the current time.
	Now() time.Time
	// Since returns the duration since the given time.
	Since(t time.Time) time.Duration
	// NewTicker returns a new time.Ticker that ticks at the given interval.
	// For mock providers, this may return a controllable ticker.
	NewTicker(d time.Duration) *time.Ticker
}

// RealTimeProvider implements TimeProvider using actual system time.
type RealTimeProvider struct{}

// Now returns the current system time.
func (RealTimeProvider) Now() time.Time {
	return time.Now()
}

// Since returns the duration since the given time.
func (RealTimeProvider) Since(t time.Time) time.Duration {
	return time.Since(t)
}

// NewTicker returns a new time.Ticker that ticks at the given interval.
func (RealTimeProvider) NewTicker(d time.Duration) *time.Ticker {
	return time.NewTicker(d)
}

// MockTimeProvider is a deterministic time provider for testing.
type MockTimeProvider struct {
	currentTime time.Time
}

// Now returns the mock time.
func (m *MockTimeProvider) Now() time.Time {
	return m.currentTime
}

// Since returns the duration since the given time using mock time.
func (m *MockTimeProvider) Since(t time.Time) time.Duration {
	return m.currentTime.Sub(t)
}

// NewTicker returns a real ticker for mock provider (mock tickers require more complexity).
// For testing, use Advance() to control time progression.
func (m *MockTimeProvider) NewTicker(d time.Duration) *time.Ticker {
	return time.NewTicker(d)
}

// Advance moves the mock time forward by the given duration.
func (m *MockTimeProvider) Advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

// SetTime sets the mock time to a specific value.
func (m *MockTimeProvider) SetTime(t time.Time) {
	m.currentTime = t
}
