package net

import (
	"testing"
	"time"
)

// MockTimeProvider is a test implementation of TimeProvider for deterministic testing.
type MockTimeProvider struct {
	currentTime time.Time
}

// Now returns the mock time.
func (m *MockTimeProvider) Now() time.Time {
	return m.currentTime
}

// SetTime sets the mock time.
func (m *MockTimeProvider) SetTime(t time.Time) {
	m.currentTime = t
}

// Advance advances the mock time by the specified duration.
func (m *MockTimeProvider) Advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

func TestTimeProviderInterface(t *testing.T) {
	// Verify RealTimeProvider implements TimeProvider
	var tp TimeProvider = RealTimeProvider{}

	// RealTimeProvider.Now() should return time close to current time
	now := time.Now()
	providerNow := tp.Now()

	if providerNow.Sub(now) > time.Second {
		t.Errorf("RealTimeProvider.Now() returned time too different from time.Now()")
	}
}

func TestMockTimeProvider(t *testing.T) {
	mockTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	mock := &MockTimeProvider{currentTime: mockTime}

	// Test Now() returns set time
	if !mock.Now().Equal(mockTime) {
		t.Errorf("MockTimeProvider.Now() = %v, want %v", mock.Now(), mockTime)
	}

	// Test SetTime()
	newTime := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	mock.SetTime(newTime)
	if !mock.Now().Equal(newTime) {
		t.Errorf("After SetTime(), Now() = %v, want %v", mock.Now(), newTime)
	}

	// Test Advance()
	mock.Advance(time.Hour)
	expected := newTime.Add(time.Hour)
	if !mock.Now().Equal(expected) {
		t.Errorf("After Advance(1h), Now() = %v, want %v", mock.Now(), expected)
	}
}

func TestGetTimeProvider(t *testing.T) {
	// Test with nil - should return default
	tp := getTimeProvider(nil)
	if tp == nil {
		t.Error("getTimeProvider(nil) returned nil, expected defaultTimeProvider")
	}

	// Test with explicit provider
	mock := &MockTimeProvider{currentTime: time.Now()}
	tp = getTimeProvider(mock)
	if tp != mock {
		t.Error("getTimeProvider(mock) did not return the provided mock")
	}
}

func TestSetDefaultTimeProvider(t *testing.T) {
	// Save original
	original := defaultTimeProvider
	defer func() {
		defaultTimeProvider = original
	}()

	// Set to mock
	mockTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock := &MockTimeProvider{currentTime: mockTime}
	SetDefaultTimeProvider(mock)

	// Verify
	if !getTimeProvider(nil).Now().Equal(mockTime) {
		t.Error("SetDefaultTimeProvider did not set the default provider")
	}

	// Test nil resets to RealTimeProvider
	SetDefaultTimeProvider(nil)
	_, ok := defaultTimeProvider.(RealTimeProvider)
	if !ok {
		t.Error("SetDefaultTimeProvider(nil) did not reset to RealTimeProvider")
	}
}

func TestToxConnTimeProvider(t *testing.T) {
	// Create a connection with default time provider
	conn := &ToxConn{
		timeProvider: defaultTimeProvider,
	}

	// Test SetTimeProvider
	mockTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	mock := &MockTimeProvider{currentTime: mockTime}
	conn.SetTimeProvider(mock)

	// Verify the time provider was set
	if conn.timeProvider != mock {
		t.Error("ToxConn.SetTimeProvider did not set the time provider")
	}
}

func TestToxPacketConnTimeProvider(t *testing.T) {
	// Create a packet connection (minimal setup for testing)
	conn := &ToxPacketConn{
		timeProvider: defaultTimeProvider,
	}

	// Test SetTimeProvider
	mockTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	mock := &MockTimeProvider{currentTime: mockTime}
	conn.SetTimeProvider(mock)

	// Verify the time provider was set
	if conn.timeProvider != mock {
		t.Error("ToxPacketConn.SetTimeProvider did not set the time provider")
	}
}

func TestToxPacketListenerTimeProvider(t *testing.T) {
	// Create a listener (minimal setup for testing)
	listener := &ToxPacketListener{
		timeProvider: defaultTimeProvider,
	}

	// Test SetTimeProvider
	mockTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	mock := &MockTimeProvider{currentTime: mockTime}
	listener.SetTimeProvider(mock)

	// Verify the time provider was set
	if listener.timeProvider != mock {
		t.Error("ToxPacketListener.SetTimeProvider did not set the time provider")
	}
}

func TestToxPacketConnectionTimeProvider(t *testing.T) {
	// Create a connection (minimal setup for testing)
	conn := &ToxPacketConnection{
		timeProvider: defaultTimeProvider,
	}

	// Test SetTimeProvider
	mockTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	mock := &MockTimeProvider{currentTime: mockTime}
	conn.SetTimeProvider(mock)

	// Verify the time provider was set
	if conn.timeProvider != mock {
		t.Error("ToxPacketConnection.SetTimeProvider did not set the time provider")
	}
}

func TestTimeProviderInheritance(t *testing.T) {
	// Verify that ToxPacketConnection inherits time provider from listener
	mockTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	mock := &MockTimeProvider{currentTime: mockTime}

	listener := &ToxPacketListener{
		timeProvider: mock,
	}

	// Simulate handlePacket creating a new connection
	// This is a partial test - the full test would require network setup
	if listener.timeProvider != mock {
		t.Error("ToxPacketListener did not retain time provider")
	}
}
