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

// TestFriendInfo_Marshal tests FriendInfo serialization.
func TestFriendInfo_Marshal(t *testing.T) {
	var publicKey [32]byte
	for i := 0; i < 32; i++ {
		publicKey[i] = byte(i * 2)
	}

	f := New(publicKey)
	_ = f.SetName("TestUser")
	_ = f.SetStatusMessage("Hello World")
	f.SetStatus(StatusBusy)
	f.SetConnectionStatus(ConnectionUDP)

	// Marshal the friend info
	data, err := f.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Verify data is not empty
	if len(data) == 0 {
		t.Error("Marshal returned empty data")
	}

	// Verify it's valid JSON
	if data[0] != '{' {
		t.Error("Marshal did not return JSON object")
	}
}

// TestFriendInfo_Unmarshal tests FriendInfo deserialization.
func TestFriendInfo_Unmarshal(t *testing.T) {
	var publicKey [32]byte
	for i := 0; i < 32; i++ {
		publicKey[i] = byte(i * 3)
	}

	original := New(publicKey)
	_ = original.SetName("OriginalUser")
	_ = original.SetStatusMessage("Original Status")
	original.SetStatus(StatusAway)
	original.SetConnectionStatus(ConnectionTCP)

	// Marshal and unmarshal
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	restored := &FriendInfo{}
	err = restored.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify all fields are restored
	if restored.PublicKey != original.PublicKey {
		t.Errorf("PublicKey mismatch: got %v, want %v", restored.PublicKey, original.PublicKey)
	}
	if restored.Name != original.Name {
		t.Errorf("Name mismatch: got %q, want %q", restored.Name, original.Name)
	}
	if restored.StatusMessage != original.StatusMessage {
		t.Errorf("StatusMessage mismatch: got %q, want %q", restored.StatusMessage, original.StatusMessage)
	}
	if restored.Status != original.Status {
		t.Errorf("Status mismatch: got %v, want %v", restored.Status, original.Status)
	}
	if restored.ConnectionStatus != original.ConnectionStatus {
		t.Errorf("ConnectionStatus mismatch: got %v, want %v", restored.ConnectionStatus, original.ConnectionStatus)
	}
	if !restored.LastSeen.Equal(original.LastSeen) {
		t.Errorf("LastSeen mismatch: got %v, want %v", restored.LastSeen, original.LastSeen)
	}
}

// TestUnmarshalFriendInfo tests the convenience function for creating FriendInfo from data.
func TestUnmarshalFriendInfo(t *testing.T) {
	var publicKey [32]byte
	for i := 0; i < 32; i++ {
		publicKey[i] = byte(i + 100)
	}

	original := New(publicKey)
	_ = original.SetName("ConvenienceTest")

	data, _ := original.Marshal()

	restored, err := UnmarshalFriendInfo(data)
	if err != nil {
		t.Fatalf("UnmarshalFriendInfo failed: %v", err)
	}

	if restored.Name != original.Name {
		t.Errorf("Name mismatch: got %q, want %q", restored.Name, original.Name)
	}
}

// TestFriendInfo_UnmarshalInvalidData tests Unmarshal with invalid JSON.
func TestFriendInfo_UnmarshalInvalidData(t *testing.T) {
	f := &FriendInfo{}

	testCases := []struct {
		name string
		data []byte
	}{
		{"Empty data", []byte{}},
		{"Invalid JSON", []byte("not json")},
		{"Malformed JSON", []byte("{broken")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := f.Unmarshal(tc.data)
			if err == nil {
				t.Error("Expected error for invalid data")
			}
		})
	}
}

// TestRequest_Marshal tests Request serialization.
func TestRequest_Marshal(t *testing.T) {
	var recipientPublicKey [32]byte
	var senderSecretKey [32]byte
	for i := 0; i < 32; i++ {
		senderSecretKey[i] = byte(i + 50)
	}

	req, err := NewRequest(recipientPublicKey, "Friend request message", senderSecretKey)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	// Set sender public key for testing
	for i := 0; i < 32; i++ {
		req.SenderPublicKey[i] = byte(i + 100)
	}

	data, err := req.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Marshal returned empty data")
	}
}

// TestRequest_Unmarshal tests Request deserialization.
func TestRequest_Unmarshal(t *testing.T) {
	var recipientPublicKey [32]byte
	var senderSecretKey [32]byte
	for i := 0; i < 32; i++ {
		senderSecretKey[i] = byte(i + 50)
	}

	original, err := NewRequest(recipientPublicKey, "Original message", senderSecretKey)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	// Set sender public key for testing
	for i := 0; i < 32; i++ {
		original.SenderPublicKey[i] = byte(i + 200)
	}
	original.Handled = true

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	restored := &Request{}
	err = restored.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify all fields are restored
	if restored.SenderPublicKey != original.SenderPublicKey {
		t.Errorf("SenderPublicKey mismatch")
	}
	if restored.Message != original.Message {
		t.Errorf("Message mismatch: got %q, want %q", restored.Message, original.Message)
	}
	if restored.Nonce != original.Nonce {
		t.Errorf("Nonce mismatch")
	}
	if !restored.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp mismatch: got %v, want %v", restored.Timestamp, original.Timestamp)
	}
	if restored.Handled != original.Handled {
		t.Errorf("Handled mismatch: got %v, want %v", restored.Handled, original.Handled)
	}
}

// TestUnmarshalRequest tests the convenience function for creating Request from data.
func TestUnmarshalRequest(t *testing.T) {
	var recipientPublicKey [32]byte
	var senderSecretKey [32]byte

	original, _ := NewRequest(recipientPublicKey, "Convenience test", senderSecretKey)
	data, _ := original.Marshal()

	restored, err := UnmarshalRequest(data)
	if err != nil {
		t.Fatalf("UnmarshalRequest failed: %v", err)
	}

	if restored.Message != original.Message {
		t.Errorf("Message mismatch: got %q, want %q", restored.Message, original.Message)
	}
}

// TestRequest_UnmarshalInvalidData tests Unmarshal with invalid JSON.
func TestRequest_UnmarshalInvalidData(t *testing.T) {
	r := &Request{}

	testCases := []struct {
		name string
		data []byte
	}{
		{"Empty data", []byte{}},
		{"Invalid JSON", []byte("not json")},
		{"Malformed JSON", []byte("{broken")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := r.Unmarshal(tc.data)
			if err == nil {
				t.Error("Expected error for invalid data")
			}
		})
	}
}

// TestSerializationRoundTrip tests complete round-trip serialization.
func TestSerializationRoundTrip(t *testing.T) {
	t.Run("FriendInfo round-trip", func(t *testing.T) {
		var publicKey [32]byte
		for i := 0; i < 32; i++ {
			publicKey[i] = byte(i)
		}

		original := New(publicKey)
		_ = original.SetName("RoundTripUser")
		_ = original.SetStatusMessage("Testing round-trip")
		original.SetStatus(StatusOnline)
		original.SetConnectionStatus(ConnectionUDP)

		// Multiple round-trips should preserve data
		for i := 0; i < 3; i++ {
			data, err := original.Marshal()
			if err != nil {
				t.Fatalf("Marshal iteration %d failed: %v", i, err)
			}

			restored, err := UnmarshalFriendInfo(data)
			if err != nil {
				t.Fatalf("Unmarshal iteration %d failed: %v", i, err)
			}

			if restored.Name != original.Name {
				t.Errorf("Iteration %d: Name mismatch", i)
			}

			// Use restored for next iteration
			original = restored
		}
	})

	t.Run("Request round-trip", func(t *testing.T) {
		var recipientPublicKey [32]byte
		var senderSecretKey [32]byte

		original, _ := NewRequest(recipientPublicKey, "Round-trip request", senderSecretKey)

		// Multiple round-trips should preserve data
		for i := 0; i < 3; i++ {
			data, err := original.Marshal()
			if err != nil {
				t.Fatalf("Marshal iteration %d failed: %v", i, err)
			}

			restored, err := UnmarshalRequest(data)
			if err != nil {
				t.Fatalf("Unmarshal iteration %d failed: %v", i, err)
			}

			if restored.Message != original.Message {
				t.Errorf("Iteration %d: Message mismatch", i)
			}

			// Use restored for next iteration
			original = restored
		}
	})
}
