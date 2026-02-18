package crypto

import (
	"testing"
	"time"
)

// MockTimeProvider is a test double that allows controlling time.
type MockTimeProvider struct {
	currentTime time.Time
}

// Now returns the mock current time.
func (m *MockTimeProvider) Now() time.Time { return m.currentTime }

// Since returns the duration since the given time.
func (m *MockTimeProvider) Since(t time.Time) time.Duration { return m.currentTime.Sub(t) }

// Advance moves the mock time forward by the given duration.
func (m *MockTimeProvider) Advance(d time.Duration) { m.currentTime = m.currentTime.Add(d) }

// Set sets the mock time to the given time.
func (m *MockTimeProvider) Set(t time.Time) { m.currentTime = t }

// NewMockTimeProvider creates a new MockTimeProvider initialized to the given time.
func NewMockTimeProvider(t time.Time) *MockTimeProvider {
	return &MockTimeProvider{currentTime: t}
}

func TestTimeProvider_Default(t *testing.T) {
	t.Parallel()

	// Test DefaultTimeProvider
	dp := DefaultTimeProvider{}

	before := time.Now()
	now := dp.Now()
	after := time.Now()

	if now.Before(before) || now.After(after) {
		t.Error("DefaultTimeProvider.Now() should return current time")
	}

	// Test Since
	pastTime := time.Now().Add(-time.Hour)
	since := dp.Since(pastTime)
	if since < time.Hour || since > time.Hour+time.Second {
		t.Errorf("DefaultTimeProvider.Since() returned unexpected duration: %v", since)
	}
}

func TestTimeProvider_Package_Level(t *testing.T) {
	// Not parallel due to modifying package-level state

	// Save original and restore after test
	original := GetDefaultTimeProvider()
	defer SetDefaultTimeProvider(original)

	// Test setting a mock provider
	mockTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	mock := NewMockTimeProvider(mockTime)
	SetDefaultTimeProvider(mock)

	provider := GetDefaultTimeProvider()
	if provider.Now() != mockTime {
		t.Errorf("Expected mock time %v, got %v", mockTime, provider.Now())
	}

	// Test advancing time
	mock.Advance(time.Hour)
	expected := mockTime.Add(time.Hour)
	if provider.Now() != expected {
		t.Errorf("Expected %v after advance, got %v", expected, provider.Now())
	}

	// Test resetting to nil (should restore default)
	SetDefaultTimeProvider(nil)
	provider = GetDefaultTimeProvider()
	_, ok := provider.(DefaultTimeProvider)
	if !ok {
		t.Error("SetDefaultTimeProvider(nil) should restore DefaultTimeProvider")
	}
}

func TestKeyRotationManager_WithTimeProvider(t *testing.T) {
	t.Parallel()

	// Create a fixed time for deterministic testing
	fixedTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	mock := NewMockTimeProvider(fixedTime)

	// Generate a test key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create manager with mock time provider
	krm := NewKeyRotationManagerWithTimeProvider(keyPair, mock)

	// Verify key creation time is the mock time
	if !krm.KeyCreationTime.Equal(fixedTime) {
		t.Errorf("Expected KeyCreationTime %v, got %v", fixedTime, krm.KeyCreationTime)
	}

	// Set a short rotation period for testing
	krm.RotationPeriod = time.Hour

	// Should not need rotation initially
	if krm.ShouldRotate() {
		t.Error("Should not need rotation immediately")
	}

	// Advance time by 30 minutes - still should not need rotation
	mock.Advance(30 * time.Minute)
	if krm.ShouldRotate() {
		t.Error("Should not need rotation after 30 minutes with 1 hour period")
	}

	// Advance time past the rotation period
	mock.Advance(31 * time.Minute)
	if !krm.ShouldRotate() {
		t.Error("Should need rotation after 61 minutes with 1 hour period")
	}

	// Rotate the key
	newKey, err := krm.RotateKey()
	if err != nil {
		t.Fatalf("RotateKey failed: %v", err)
	}
	if newKey == nil {
		t.Fatal("RotateKey returned nil")
	}

	// After rotation, the new creation time should be the current mock time
	expectedTime := fixedTime.Add(61 * time.Minute)
	if !krm.KeyCreationTime.Equal(expectedTime) {
		t.Errorf("Expected KeyCreationTime %v after rotation, got %v", expectedTime, krm.KeyCreationTime)
	}

	// Should not need rotation immediately after rotation
	if krm.ShouldRotate() {
		t.Error("Should not need rotation immediately after rotation")
	}
}

func TestKeyRotationManager_SetTimeProvider(t *testing.T) {
	t.Parallel()

	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create with default provider
	krm := NewKeyRotationManager(keyPair)
	krm.RotationPeriod = time.Hour

	// Set a mock time provider
	fixedTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mock := NewMockTimeProvider(fixedTime)
	krm.SetTimeProvider(mock)

	// Manually set key creation time to a known value for testing
	krm.mu.Lock()
	krm.KeyCreationTime = fixedTime.Add(-30 * time.Minute)
	krm.mu.Unlock()

	// Should not need rotation (30 min < 1 hour)
	if krm.ShouldRotate() {
		t.Error("Should not need rotation with 30 minutes elapsed")
	}

	// Advance mock time
	mock.Advance(31 * time.Minute)

	// Now should need rotation (61 min > 1 hour)
	if !krm.ShouldRotate() {
		t.Error("Should need rotation with 61 minutes elapsed")
	}

	// Test setting nil (should use default)
	krm.SetTimeProvider(nil)
	// Should still work without panic
	_ = krm.ShouldRotate()
}
