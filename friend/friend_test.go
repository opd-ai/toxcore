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
		name      string
		setName   string
		expected  string
		expectErr bool
	}{
		{"Empty name", "", "", false},
		{"Normal name", "Alice", "Alice", false},
		{"Unicode name", "友達", "友達", false},
		{"Long name within limit", "This is a name that fits in 128 bytes", "This is a name that fits in 128 bytes", false},
		{"Max length name", string(make([]byte, MaxNameLength)), string(make([]byte, MaxNameLength)), false},
		{"Name too long", string(make([]byte, MaxNameLength+1)), "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset name before each test
			f.Name = ""

			err := f.SetName(tc.setName)

			if tc.expectErr {
				if err == nil {
					t.Errorf("Expected error for name length %d, got nil", len(tc.setName))
				}
				// Name should not be modified on error
				if f.GetName() != "" {
					t.Errorf("Name should not be modified on error, got %q", f.GetName())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if got := f.GetName(); got != tc.expected {
					t.Errorf("GetName() = %v, want %v", got, tc.expected)
				}
			}
		})
	}
}

func TestFriend_StatusMessageFunctions(t *testing.T) {
	var publicKey [32]byte
	f := New(publicKey)

	testCases := []struct {
		name      string
		setMsg    string
		expected  string
		expectErr bool
	}{
		{"Empty message", "", "", false},
		{"Normal message", "Available for chat", "Available for chat", false},
		{"Unicode message", "こんにちは", "こんにちは", false},
		{"Long message within limit", "This is a long status message that is still within the 1007 byte limit", "This is a long status message that is still within the 1007 byte limit", false},
		{"Max length message", string(make([]byte, MaxStatusMessageLength)), string(make([]byte, MaxStatusMessageLength)), false},
		{"Message too long", string(make([]byte, MaxStatusMessageLength+1)), "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset status message before each test
			f.StatusMessage = ""

			err := f.SetStatusMessage(tc.setMsg)

			if tc.expectErr {
				if err == nil {
					t.Errorf("Expected error for message length %d, got nil", len(tc.setMsg))
				}
				// StatusMessage should not be modified on error
				if f.GetStatusMessage() != "" {
					t.Errorf("StatusMessage should not be modified on error, got %q", f.GetStatusMessage())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if got := f.GetStatusMessage(); got != tc.expected {
					t.Errorf("GetStatusMessage() = %v, want %v", got, tc.expected)
				}
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

func TestNewRequest_MessageValidation(t *testing.T) {
	var recipientPublicKey [32]byte
	var senderSecretKey [32]byte

	testCases := []struct {
		name      string
		message   string
		expectErr bool
		errMsg    string
	}{
		{"Empty message", "", true, "message cannot be empty"},
		{"Normal message", "Hi, let's be friends!", false, ""},
		{"Max length message", string(make([]byte, MaxFriendRequestMessageLength)), false, ""},
		{"Message too long", string(make([]byte, MaxFriendRequestMessageLength+1)), true, "friend request message exceeds maximum length"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := NewRequest(recipientPublicKey, tc.message, senderSecretKey)

			if tc.expectErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				if req != nil {
					t.Errorf("Request should be nil on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if req == nil {
					t.Errorf("Request should not be nil")
				} else if req.Message != tc.message {
					t.Errorf("Expected message %q, got %q", tc.message, req.Message)
				}
			}
		})
	}
}

func TestValidationErrorTypes(t *testing.T) {
	var publicKey [32]byte
	f := New(publicKey)

	// Test ErrNameTooLong error type
	err := f.SetName(string(make([]byte, MaxNameLength+1)))
	if err == nil {
		t.Error("Expected ErrNameTooLong error")
	}

	// Test ErrStatusMessageTooLong error type
	err = f.SetStatusMessage(string(make([]byte, MaxStatusMessageLength+1)))
	if err == nil {
		t.Error("Expected ErrStatusMessageTooLong error")
	}

	// Test ErrFriendRequestMessageTooLong error type
	var recipientPublicKey [32]byte
	var senderSecretKey [32]byte
	_, err = NewRequest(recipientPublicKey, string(make([]byte, MaxFriendRequestMessageLength+1)), senderSecretKey)
	if err == nil {
		t.Error("Expected ErrFriendRequestMessageTooLong error")
	}
}
