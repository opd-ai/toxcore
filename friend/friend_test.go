package friend

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
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
	if f.Status != FriendStatusNone {
		t.Errorf("Expected Status %v, got %v", FriendStatusNone, f.Status)
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
	if timeDiff > testRecentThreshold {
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
		setStatus FriendStatus
		expected  FriendStatus
	}{
		{"Status None", FriendStatusNone, FriendStatusNone},
		{"Status Away", FriendStatusAway, FriendStatusAway},
		{"Status Busy", FriendStatusBusy, FriendStatusBusy},
		{"Status Online", FriendStatusOnline, FriendStatusOnline},
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
	if d := f.LastSeenDuration(); d > testRecentThreshold {
		t.Errorf("Expected small duration after creation, got %v", d)
	}

	// Test after a delay
	oldTime := time.Now().Add(-testDelayDuration)
	f.LastSeen = oldTime

	duration := f.LastSeenDuration()
	if duration < testDelayDuration || duration > 3*time.Second {
		t.Errorf("Expected duration around %v, got %v", testDelayDuration, duration)
	}
}

func TestNewRequest_MessageValidation(t *testing.T) {
	var recipientPublicKey [32]byte
	// Generate a valid key pair for testing
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key pair: %v", err)
	}
	senderSecretKey := keyPair.Private

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
	f.SetStatus(FriendStatusBusy)
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
	original.SetStatus(FriendStatusAway)
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
	// Generate a valid key pair for testing
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key pair: %v", err)
	}
	senderSecretKey := keyPair.Private

	original, err := NewRequest(recipientPublicKey, "Convenience test", senderSecretKey)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

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
		original.SetStatus(FriendStatusOnline)
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
		// Generate a valid key pair for testing
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate test key pair: %v", err)
		}
		senderSecretKey := keyPair.Private

		original, err := NewRequest(recipientPublicKey, "Round-trip request", senderSecretKey)
		if err != nil {
			t.Fatalf("NewRequest failed: %v", err)
		}

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

// TestRequest_EncryptDecrypt tests encryption and decryption of friend requests.
func TestRequest_EncryptDecrypt(t *testing.T) {
	// Import crypto package is already available via request.go

	// Generate sender and recipient key pairs using crypto package
	senderKeyPair, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	testCases := []struct {
		name    string
		message string
	}{
		{"Short message", "Hi, let's be friends!"},
		{"Unicode message", "友達になりましょう！"},
		{"Long message", "This is a longer message that tests encryption with more content to process."},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a friend request
			req, err := NewRequest(recipientKeyPair.Public, tc.message, senderKeyPair.Private)
			if err != nil {
				t.Fatalf("NewRequest failed: %v", err)
			}

			// Encrypt the request
			encrypted, err := req.Encrypt(senderKeyPair, recipientKeyPair.Public)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			// Verify encrypted data is not empty
			if len(encrypted) == 0 {
				t.Error("Encrypted data is empty")
			}

			// Decrypt the request
			decrypted, err := DecryptRequest(encrypted, recipientKeyPair.Private)
			if err != nil {
				t.Fatalf("DecryptRequest failed: %v", err)
			}

			// Verify decrypted message matches original
			if decrypted.Message != tc.message {
				t.Errorf("Message mismatch: got %q, want %q", decrypted.Message, tc.message)
			}

			// Verify sender public key is correct
			if decrypted.SenderPublicKey != senderKeyPair.Public {
				t.Error("Sender public key mismatch")
			}
		})
	}
}

// TestDecryptRequest_InvalidPacket tests decryption with invalid packets.
func TestDecryptRequest_InvalidPacket(t *testing.T) {
	var recipientSecretKey [32]byte
	for i := 0; i < 32; i++ {
		recipientSecretKey[i] = byte(i + 1)
	}

	testCases := []struct {
		name   string
		packet []byte
	}{
		{"Empty packet", []byte{}},
		{"Too short (55 bytes)", make([]byte, 55)},
		{"Too short (32 bytes)", make([]byte, 32)},
		{"Too short (24 bytes)", make([]byte, 24)},
		{"Exactly 56 bytes (min, invalid crypto)", make([]byte, 56)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecryptRequest(tc.packet, recipientSecretKey)
			if err == nil {
				t.Error("Expected error for invalid packet")
			}
		})
	}
}

// TestDecryptRequest_InvalidCrypto tests decryption with corrupted data.
func TestDecryptRequest_InvalidCrypto(t *testing.T) {
	// Generate valid keys
	senderKeyPair, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Create and encrypt a valid request
	req, err := NewRequest(recipientKeyPair.Public, "Test message", senderKeyPair.Private)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	encrypted, err := req.Encrypt(senderKeyPair, recipientKeyPair.Public)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	t.Run("Corrupted encrypted data", func(t *testing.T) {
		corrupted := make([]byte, len(encrypted))
		copy(corrupted, encrypted)
		// Corrupt the encrypted portion (after public key + nonce)
		if len(corrupted) > 60 {
			corrupted[60] ^= 0xFF
		}

		_, err := DecryptRequest(corrupted, recipientKeyPair.Private)
		if err == nil {
			t.Error("Expected error for corrupted encrypted data")
		}
	})

	t.Run("Wrong recipient key", func(t *testing.T) {
		var wrongKey [32]byte
		for i := 0; i < 32; i++ {
			wrongKey[i] = byte(i + 100)
		}

		_, err := DecryptRequest(encrypted, wrongKey)
		if err == nil {
			t.Error("Expected error for wrong recipient key")
		}
	})
}

// TestRequestManager_NewRequestManager tests RequestManager creation.
func TestRequestManager_NewRequestManager(t *testing.T) {
	rm := NewRequestManager()

	if rm == nil {
		t.Fatal("NewRequestManager returned nil")
	}

	// Verify initial state
	pending := rm.GetPendingRequests()
	if len(pending) != 0 {
		t.Errorf("Expected empty pending requests, got %d", len(pending))
	}
}

// TestRequestManager_SetHandler tests handler registration.
func TestRequestManager_SetHandler(t *testing.T) {
	rm := NewRequestManager()

	handlerCalled := false
	rm.SetHandler(func(request *Request) bool {
		handlerCalled = true
		return true
	})

	// Create a test request
	var senderPK [32]byte
	for i := 0; i < 32; i++ {
		senderPK[i] = byte(i)
	}

	req := &Request{
		SenderPublicKey: senderPK,
		Message:         "Test message",
	}

	// Add request should trigger handler
	rm.AddRequest(req)

	if !handlerCalled {
		t.Error("Handler was not called")
	}
}

// TestRequestManager_AddRequest tests adding requests.
func TestRequestManager_AddRequest(t *testing.T) {
	rm := NewRequestManager()

	// Create test requests with different sender public keys
	var pk1, pk2 [32]byte
	for i := 0; i < 32; i++ {
		pk1[i] = byte(i)
		pk2[i] = byte(i + 100)
	}

	req1 := &Request{
		SenderPublicKey: pk1,
		Message:         "First request",
	}

	req2 := &Request{
		SenderPublicKey: pk2,
		Message:         "Second request",
	}

	// Add first request
	rm.AddRequest(req1)
	pending := rm.GetPendingRequests()
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending request, got %d", len(pending))
	}

	// Add second request
	rm.AddRequest(req2)
	pending = rm.GetPendingRequests()
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending requests, got %d", len(pending))
	}
}

// TestRequestManager_AddRequest_DuplicateUpdate tests updating existing request.
func TestRequestManager_AddRequest_DuplicateUpdate(t *testing.T) {
	rm := NewRequestManager()

	var pk [32]byte
	for i := 0; i < 32; i++ {
		pk[i] = byte(i)
	}

	req1 := &Request{
		SenderPublicKey: pk,
		Message:         "First message",
	}

	req2 := &Request{
		SenderPublicKey: pk, // Same sender
		Message:         "Updated message",
	}

	// Add first request
	rm.AddRequest(req1)

	// Add duplicate (should update existing)
	rm.AddRequest(req2)

	pending := rm.GetPendingRequests()
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending request after duplicate, got %d", len(pending))
	}

	// Verify message was updated
	if pending[0].Message != "Updated message" {
		t.Errorf("Message was not updated: got %q", pending[0].Message)
	}
}

// TestRequestManager_AcceptRequest tests accepting requests.
func TestRequestManager_AcceptRequest(t *testing.T) {
	rm := NewRequestManager()

	var pk [32]byte
	for i := 0; i < 32; i++ {
		pk[i] = byte(i)
	}

	req := &Request{
		SenderPublicKey: pk,
		Message:         "Friend request",
	}

	rm.AddRequest(req)

	// Accept the request
	result := rm.AcceptRequest(pk)
	if !result {
		t.Error("AcceptRequest returned false for valid request")
	}

	// Verify request is marked as handled
	pending := rm.GetPendingRequests()
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending requests after accept, got %d", len(pending))
	}

	// Accept again should fail (already handled)
	result = rm.AcceptRequest(pk)
	if result {
		t.Error("AcceptRequest should return false for already handled request")
	}
}

// TestRequestManager_AcceptRequest_NotFound tests accepting non-existent request.
func TestRequestManager_AcceptRequest_NotFound(t *testing.T) {
	rm := NewRequestManager()

	var pk [32]byte
	for i := 0; i < 32; i++ {
		pk[i] = byte(i)
	}

	// Accept non-existent request
	result := rm.AcceptRequest(pk)
	if result {
		t.Error("AcceptRequest should return false for non-existent request")
	}
}

// TestRequestManager_RejectRequest tests rejecting requests.
func TestRequestManager_RejectRequest(t *testing.T) {
	rm := NewRequestManager()

	var pk [32]byte
	for i := 0; i < 32; i++ {
		pk[i] = byte(i)
	}

	req := &Request{
		SenderPublicKey: pk,
		Message:         "Friend request",
	}

	rm.AddRequest(req)

	// Reject the request
	result := rm.RejectRequest(pk)
	if !result {
		t.Error("RejectRequest returned false for valid request")
	}

	// Verify request is removed
	pending := rm.GetPendingRequests()
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending requests after reject, got %d", len(pending))
	}

	// Reject again should fail (already removed)
	result = rm.RejectRequest(pk)
	if result {
		t.Error("RejectRequest should return false for already removed request")
	}
}

// TestRequestManager_RejectRequest_NotFound tests rejecting non-existent request.
func TestRequestManager_RejectRequest_NotFound(t *testing.T) {
	rm := NewRequestManager()

	var pk [32]byte
	for i := 0; i < 32; i++ {
		pk[i] = byte(i)
	}

	// Reject non-existent request
	result := rm.RejectRequest(pk)
	if result {
		t.Error("RejectRequest should return false for non-existent request")
	}
}

// TestRequestManager_GetPendingRequests_FilterHandled tests filtering handled requests.
func TestRequestManager_GetPendingRequests_FilterHandled(t *testing.T) {
	rm := NewRequestManager()

	// Create requests with different public keys
	var pk1, pk2, pk3 [32]byte
	for i := 0; i < 32; i++ {
		pk1[i] = byte(i)
		pk2[i] = byte(i + 50)
		pk3[i] = byte(i + 100)
	}

	req1 := &Request{SenderPublicKey: pk1, Message: "Request 1"}
	req2 := &Request{SenderPublicKey: pk2, Message: "Request 2"}
	req3 := &Request{SenderPublicKey: pk3, Message: "Request 3"}

	rm.AddRequest(req1)
	rm.AddRequest(req2)
	rm.AddRequest(req3)

	// Accept one request
	rm.AcceptRequest(pk2)

	pending := rm.GetPendingRequests()
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending requests, got %d", len(pending))
	}

	// Verify pk2 is not in pending
	for _, p := range pending {
		if p.SenderPublicKey == pk2 {
			t.Error("Handled request should not be in pending list")
		}
	}
}

// TestRequestManager_HandlerAcceptReject tests handler return value affecting Handled status.
func TestRequestManager_HandlerAcceptReject(t *testing.T) {
	t.Run("Handler accepts", func(t *testing.T) {
		rm := NewRequestManager()
		rm.SetHandler(func(request *Request) bool {
			return true // Accept
		})

		var pk [32]byte
		req := &Request{SenderPublicKey: pk, Message: "Test"}
		rm.AddRequest(req)

		pending := rm.GetPendingRequests()
		if len(pending) != 0 {
			t.Errorf("Request should be handled (accepted), got %d pending", len(pending))
		}
	})

	t.Run("Handler rejects", func(t *testing.T) {
		rm := NewRequestManager()
		rm.SetHandler(func(request *Request) bool {
			return false // Reject
		})

		var pk [32]byte
		req := &Request{SenderPublicKey: pk, Message: "Test"}
		rm.AddRequest(req)

		pending := rm.GetPendingRequests()
		if len(pending) != 1 {
			t.Errorf("Request should not be handled (rejected), got %d pending", len(pending))
		}
	})
}

// TestNewWithTimeProvider tests FriendInfo creation with custom time provider.
func TestNewWithTimeProvider(t *testing.T) {
	var publicKey [32]byte
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	mockTP := &mockTimeProvider{fixedTime: fixedTime}

	f := NewWithTimeProvider(publicKey, mockTP)

	if !f.LastSeen.Equal(fixedTime) {
		t.Errorf("LastSeen should be %v, got %v", fixedTime, f.LastSeen)
	}

	// Test with nil time provider (should use default)
	f2 := NewWithTimeProvider(publicKey, nil)
	if f2.LastSeen.IsZero() {
		t.Error("LastSeen should not be zero with nil time provider")
	}
}

// TestNewRequestWithTimeProvider tests Request creation with custom time provider.
func TestNewRequestWithTimeProvider(t *testing.T) {
	var recipientPK [32]byte
	// Generate a valid key pair for testing
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key pair: %v", err)
	}
	senderSK := keyPair.Private
	fixedTime := time.Date(2024, 2, 20, 14, 45, 0, 0, time.UTC)
	mockTP := &mockTimeProvider{fixedTime: fixedTime}

	req, err := NewRequestWithTimeProvider(recipientPK, "Hello", senderSK, mockTP)
	if err != nil {
		t.Fatalf("NewRequestWithTimeProvider failed: %v", err)
	}

	if !req.Timestamp.Equal(fixedTime) {
		t.Errorf("Timestamp should be %v, got %v", fixedTime, req.Timestamp)
	}

	// Test with nil time provider
	req2, err := NewRequestWithTimeProvider(recipientPK, "Hello", senderSK, nil)
	if err != nil {
		t.Fatalf("NewRequestWithTimeProvider failed: %v", err)
	}
	if req2.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero with nil time provider")
	}
}

// TestDecryptRequestWithTimeProvider tests decryption with custom time provider.
func TestDecryptRequestWithTimeProvider(t *testing.T) {
	senderKeyPair, _ := generateTestKeyPair()
	recipientKeyPair, _ := generateTestKeyPair()

	req, _ := NewRequest(recipientKeyPair.Public, "Test", senderKeyPair.Private)
	encrypted, _ := req.Encrypt(senderKeyPair, recipientKeyPair.Public)

	fixedTime := time.Date(2024, 3, 10, 9, 0, 0, 0, time.UTC)
	mockTP := &mockTimeProvider{fixedTime: fixedTime}

	decrypted, err := DecryptRequestWithTimeProvider(encrypted, recipientKeyPair.Private, mockTP)
	if err != nil {
		t.Fatalf("DecryptRequestWithTimeProvider failed: %v", err)
	}

	if !decrypted.Timestamp.Equal(fixedTime) {
		t.Errorf("Timestamp should be %v, got %v", fixedTime, decrypted.Timestamp)
	}

	// Test with nil time provider
	decrypted2, err := DecryptRequestWithTimeProvider(encrypted, recipientKeyPair.Private, nil)
	if err != nil {
		t.Fatalf("DecryptRequestWithTimeProvider failed: %v", err)
	}
	if decrypted2.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero with nil time provider")
	}
}

// TestFriendInfo_ConcurrentAccess tests that FriendInfo is safe for concurrent access.
func TestFriendInfo_ConcurrentAccess(t *testing.T) {
	var publicKey [32]byte
	for i := 0; i < 32; i++ {
		publicKey[i] = byte(i)
	}

	f := New(publicKey)

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent writers
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = f.SetName("TestName")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = f.SetStatusMessage("Status")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			f.SetStatus(FriendStatusOnline)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			f.SetConnectionStatus(ConnectionUDP)
		}
	}()

	// Concurrent readers
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = f.GetName()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = f.GetStatusMessage()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = f.GetStatus()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = f.GetConnectionStatus()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = f.IsOnline()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = f.LastSeenDuration()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_, _ = f.Marshal()
		}
	}()

	wg.Wait()
}

// TestNewRequest_SenderPublicKeyPopulated verifies that NewRequest correctly populates SenderPublicKey.
func TestNewRequest_SenderPublicKeyPopulated(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	var recipientPK [32]byte
	req, err := NewRequest(recipientPK, "Hello", keyPair.Private)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	// Verify SenderPublicKey is populated (not zero)
	var zeroPK [32]byte
	if bytes.Equal(req.SenderPublicKey[:], zeroPK[:]) {
		t.Error("SenderPublicKey should not be zero")
	}

	// Verify SenderPublicKey matches the derived public key from the key pair
	if !bytes.Equal(req.SenderPublicKey[:], keyPair.Public[:]) {
		t.Errorf("SenderPublicKey mismatch: got %x, want %x", req.SenderPublicKey[:8], keyPair.Public[:8])
	}
}
