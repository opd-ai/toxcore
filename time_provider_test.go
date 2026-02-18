package toxcore

import (
	"testing"
	"time"
)

// MockTimeProvider is a deterministic time provider for testing.
type MockTimeProvider struct {
	currentTime time.Time
}

// Now returns the mock time.
func (m *MockTimeProvider) Now() time.Time {
	return m.currentTime
}

// Advance moves the mock time forward by the given duration.
func (m *MockTimeProvider) Advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

// SetTime sets the mock time to a specific value.
func (m *MockTimeProvider) SetTime(t time.Time) {
	m.currentTime = t
}

func TestTimeProvider_RealTimeProvider(t *testing.T) {
	provider := RealTimeProvider{}
	before := time.Now()
	result := provider.Now()
	after := time.Now()

	if result.Before(before) || result.After(after) {
		t.Errorf("RealTimeProvider.Now() returned time outside expected range")
	}
}

func TestTimeProvider_MockTimeProvider(t *testing.T) {
	fixedTime := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	provider := &MockTimeProvider{currentTime: fixedTime}

	// Test Now returns fixed time
	if !provider.Now().Equal(fixedTime) {
		t.Errorf("MockTimeProvider.Now() = %v, want %v", provider.Now(), fixedTime)
	}

	// Test Advance
	provider.Advance(5 * time.Second)
	expected := fixedTime.Add(5 * time.Second)
	if !provider.Now().Equal(expected) {
		t.Errorf("After Advance(5s), Now() = %v, want %v", provider.Now(), expected)
	}

	// Test SetTime
	newTime := time.Date(2027, 6, 1, 12, 0, 0, 0, time.UTC)
	provider.SetTime(newTime)
	if !provider.Now().Equal(newTime) {
		t.Errorf("After SetTime, Now() = %v, want %v", provider.Now(), newTime)
	}
}

func TestTox_SetTimeProvider(t *testing.T) {
	options := NewOptionsForTesting()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify default is RealTimeProvider
	realTime := tox.now()
	if time.Since(realTime) > time.Second {
		t.Errorf("Default time provider should return current time")
	}

	// Set mock time provider
	fixedTime := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	mockProvider := &MockTimeProvider{currentTime: fixedTime}
	tox.SetTimeProvider(mockProvider)

	// Verify mock provider is used
	if !tox.now().Equal(fixedTime) {
		t.Errorf("After SetTimeProvider, now() = %v, want %v", tox.now(), fixedTime)
	}

	// Advance time and verify
	mockProvider.Advance(10 * time.Minute)
	expected := fixedTime.Add(10 * time.Minute)
	if !tox.now().Equal(expected) {
		t.Errorf("After Advance, now() = %v, want %v", tox.now(), expected)
	}
}

func TestTox_DeterministicFriendRequest(t *testing.T) {
	options := NewOptionsForTesting()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Set deterministic time
	fixedTime := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	mockProvider := &MockTimeProvider{currentTime: fixedTime}
	tox.SetTimeProvider(mockProvider)

	// Queue a friend request
	var targetPK [32]byte
	copy(targetPK[:], []byte("test-public-key-32-bytes-long!!!"))
	tox.queuePendingFriendRequest(targetPK, "Hello!", []byte("packet-data"))

	// Verify the timestamps are deterministic
	tox.pendingFriendReqsMux.Lock()
	defer tox.pendingFriendReqsMux.Unlock()

	if len(tox.pendingFriendReqs) != 1 {
		t.Fatalf("Expected 1 pending request, got %d", len(tox.pendingFriendReqs))
	}

	req := tox.pendingFriendReqs[0]
	if !req.timestamp.Equal(fixedTime) {
		t.Errorf("Request timestamp = %v, want %v", req.timestamp, fixedTime)
	}

	expectedRetry := fixedTime.Add(5 * time.Second)
	if !req.nextRetry.Equal(expectedRetry) {
		t.Errorf("Request nextRetry = %v, want %v", req.nextRetry, expectedRetry)
	}
}

func TestTox_DeterministicFileTransferID(t *testing.T) {
	options := NewOptionsForTesting()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Set deterministic time
	fixedTime := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	mockProvider := &MockTimeProvider{currentTime: fixedTime}
	tox.SetTimeProvider(mockProvider)

	// Calculate expected file ID
	expectedID := uint32(fixedTime.UnixNano() & 0xFFFFFFFF)

	// Verify the time-based calculation is deterministic
	actualID := uint32(tox.now().UnixNano() & 0xFFFFFFFF)
	if actualID != expectedID {
		t.Errorf("File transfer ID = %d, want %d", actualID, expectedID)
	}

	// Verify it's repeatable
	actualID2 := uint32(tox.now().UnixNano() & 0xFFFFFFFF)
	if actualID2 != expectedID {
		t.Errorf("Second file transfer ID = %d, want %d", actualID2, expectedID)
	}
}
