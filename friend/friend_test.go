package friend

import (
	"bytes"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	// Test creating a new friend
	var publicKey [32]byte
	for i := 0; i < 32; i++ {
		publicKey[i] = byte(i)
	}

	f := New(publicKey)

	// Verify initialization
	if !bytes.Equal(f.PublicKey[:], publicKey[:]) {
		t.Errorf("Expected public key %v, got %v", publicKey, f.PublicKey)
	}
	if f.Status != StatusNone {
		t.Errorf("Expected Status %v, got %v", StatusNone, f.Status)
	}
	if f.ConnectionStatus != ConnectionNone {
		t.Errorf("Expected ConnectionStatus %v, got %v", ConnectionNone, f.ConnectionStatus)
	}
	if f.Name != "" {
		t.Errorf("Expected Name to be empty, got %v", f.Name)
	}
	if f.StatusMessage != "" {
		t.Errorf("Expected StatusMessage to be empty, got %v", f.StatusMessage)
	}

	// LastSeen should be initialized to current time
	timeDiff := time.Since(f.LastSeen)
	if timeDiff > time.Second {
		t.Errorf("LastSeen time wasn't set correctly, diff: %v", timeDiff)
	}
}

func TestFriend_NameFunctions(t *testing.T) {
	var publicKey [32]byte
	f := New(publicKey)

	testCases := []struct {
		name     string
		setName  string
		expected string
	}{
		{"Empty name", "", ""},
		{"Normal name", "Alice", "Alice"},
		{"Unicode name", "友達", "友達"},
		{"Long name", "This is a very long name that might potentially be too long for some systems", "This is a very long name that might potentially be too long for some systems"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f.SetName(tc.setName)

			if got := f.GetName(); got != tc.expected {
				t.Errorf("GetName() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestFriend_StatusMessageFunctions(t *testing.T) {
	var publicKey [32]byte
	f := New(publicKey)

	testCases := []struct {
		name     string
		setMsg   string
		expected string
	}{
		{"Empty message", "", ""},
		{"Normal message", "Available for chat", "Available for chat"},
		{"Unicode message", "こんにちは", "こんにちは"},
		{"Long message", "This is a very long status message that contains a lot of information", "This is a very long status message that contains a lot of information"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f.SetStatusMessage(tc.setMsg)

			if got := f.GetStatusMessage(); got != tc.expected {
				t.Errorf("GetStatusMessage() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestFriend_StatusFunctions(t *testing.T) {
	var publicKey [32]byte
	f := New(publicKey)

	testCases := []struct {
		name      string
		setStatus Status
		expected  Status
	}{
		{"Status None", StatusNone, StatusNone},
		{"Status Away", StatusAway, StatusAway},
		{"Status Busy", StatusBusy, StatusBusy},
		{"Status Online", StatusOnline, StatusOnline},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f.SetStatus(tc.setStatus)

			if got := f.GetStatus(); got != tc.expected {
				t.Errorf("GetStatus() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestFriend_ConnectionStatusFunctions(t *testing.T) {
	var publicKey [32]byte
	f := New(publicKey)

	testCases := []struct {
		name           string
		setConnStatus  ConnectionStatus
		expectedStatus ConnectionStatus
		expectOnline   bool
	}{
		{"Connection None", ConnectionNone, ConnectionNone, false},
		{"Connection TCP", ConnectionTCP, ConnectionTCP, true},
		{"Connection UDP", ConnectionUDP, ConnectionUDP, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Store the time before changing status
			beforeTime := time.Now().Add(-time.Millisecond)

			f.SetConnectionStatus(tc.setConnStatus)

			// Verify the connection status was set
			if got := f.GetConnectionStatus(); got != tc.expectedStatus {
				t.Errorf("GetConnectionStatus() = %v, want %v", got, tc.expectedStatus)
			}

			// Verify online status
			if got := f.IsOnline(); got != tc.expectOnline {
				t.Errorf("IsOnline() = %v, want %v", got, tc.expectOnline)
			}

			// Verify LastSeen was updated
			if !f.LastSeen.After(beforeTime) {
				t.Errorf("LastSeen was not updated correctly. Expected after %v, got %v", beforeTime, f.LastSeen)
			}
		})
	}
}

func TestFriend_LastSeenDuration(t *testing.T) {
	var publicKey [32]byte
	f := New(publicKey)

	// Test immediate check
	if d := f.LastSeenDuration(); d > time.Second {
		t.Errorf("Expected small duration after creation, got %v", d)
	}

	// Test after a delay
	oldTime := time.Now().Add(-2 * time.Second)
	f.LastSeen = oldTime

	duration := f.LastSeenDuration()
	if duration < 2*time.Second || duration > 3*time.Second {
		t.Errorf("Expected duration around 2 seconds, got %v", duration)
	}
}
