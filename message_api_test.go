package toxcore

import (
	"testing"
)

// TestSendFriendMessageAPI tests the primary SendFriendMessage API
// with various message types and parameter combinations.
func TestSendFriendMessageAPI(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create a test friend
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Set friend as connected for testing
	tox.friendsMutex.Lock()
	if friend, exists := tox.friends[friendID]; exists {
		friend.ConnectionStatus = ConnectionUDP
	}
	tox.friendsMutex.Unlock()

	tests := []struct {
		name        string
		message     string
		messageType []MessageType
		expectError bool
		description string
	}{
		{
			name:        "Normal message with default type",
			message:     "Hello, world!",
			messageType: nil, // No type specified, should default to Normal
			expectError: false,
			description: "Should send normal message with default type",
		},
		{
			name:        "Normal message with explicit type",
			message:     "Hello, world!",
			messageType: []MessageType{MessageTypeNormal},
			expectError: false,
			description: "Should send normal message with explicit type",
		},
		{
			name:        "Action message",
			message:     "/me waves",
			messageType: []MessageType{MessageTypeAction},
			expectError: false,
			description: "Should send action message",
		},
		{
			name:        "Empty message",
			message:     "",
			messageType: nil,
			expectError: true,
			description: "Should reject empty message",
		},
		{
			name:        "Long message",
			message:     string(make([]byte, 1373)), // Over 1372 byte limit
			messageType: nil,
			expectError: true,
			description: "Should reject message that exceeds byte limit",
		},
		{
			name:        "Maximum length message",
			message:     string(make([]byte, 1372)), // Exactly at limit
			messageType: nil,
			expectError: false,
			description: "Should accept message at byte limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if len(tt.messageType) == 0 {
				err = tox.SendFriendMessage(friendID, tt.message)
			} else {
				err = tox.SendFriendMessage(friendID, tt.message, tt.messageType[0])
			}

			if tt.expectError && err == nil {
				t.Errorf("Expected error for %s, but got none", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.description, err)
			}
		})
	}
}

// TestSendFriendMessageErrorCases tests error handling scenarios
func TestSendFriendMessageErrorCases(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test sending to non-existent friend
	err = tox.SendFriendMessage(999, "Hello")
	if err == nil {
		t.Error("Expected error when sending to non-existent friend")
	}
	if err.Error() != "friend not found" {
		t.Errorf("Expected 'friend not found' error, got: %v", err)
	}

	// Create a friend but leave them disconnected
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Test sending to disconnected friend
	err = tox.SendFriendMessage(friendID, "Hello")
	if err == nil {
		t.Error("Expected error when sending to disconnected friend")
	}
	if err.Error() != "friend not connected" {
		t.Errorf("Expected 'friend not connected' error, got: %v", err)
	}
}

// TestFriendSendMessageLegacyAPI tests the deprecated FriendSendMessage method
// to ensure backward compatibility is maintained.
func TestFriendSendMessageLegacyAPI(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create and connect a test friend
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Set friend as connected
	tox.friendsMutex.Lock()
	if friend, exists := tox.friends[friendID]; exists {
		friend.ConnectionStatus = ConnectionUDP
	}
	tox.friendsMutex.Unlock()

	// Test legacy API
	messageID, err := tox.FriendSendMessage(friendID, "Test message", MessageTypeNormal)
	if err != nil {
		t.Errorf("Legacy FriendSendMessage failed: %v", err)
	}
	if messageID == 0 {
		t.Error("Expected non-zero message ID from legacy API")
	}

	// Test legacy API with action message
	messageID, err = tox.FriendSendMessage(friendID, "/me tests", MessageTypeAction)
	if err != nil {
		t.Errorf("Legacy FriendSendMessage with action failed: %v", err)
	}
	if messageID == 0 {
		t.Error("Expected non-zero message ID from legacy API with action")
	}
}

// TestMessageAPIConsistency verifies that both APIs produce consistent results
func TestMessageAPIConsistency(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create and connect a test friend
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Set friend as connected
	tox.friendsMutex.Lock()
	if friend, exists := tox.friends[friendID]; exists {
		friend.ConnectionStatus = ConnectionUDP
	}
	tox.friendsMutex.Unlock()

	// Test that both APIs handle the same error cases consistently
	testCases := []struct {
		name        string
		friendID    uint32
		message     string
		expectError bool
		expectedMsg string
	}{
		{
			name:        "Non-existent friend",
			friendID:    999,
			message:     "Hello",
			expectError: true,
			expectedMsg: "friend not found",
		},
		{
			name:        "Empty message",
			friendID:    friendID,
			message:     "",
			expectError: true,
			expectedMsg: "message cannot be empty",
		},
	}

	for _, tc := range testCases {
		t.Run("Primary_API_"+tc.name, func(t *testing.T) {
			err := tox.SendFriendMessage(tc.friendID, tc.message)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error from primary API")
				} else if err.Error() != tc.expectedMsg {
					t.Errorf("Expected error message '%s', got '%s'", tc.expectedMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error from primary API: %v", err)
			}
		})

		t.Run("Legacy_API_"+tc.name, func(t *testing.T) {
			_, err := tox.FriendSendMessage(tc.friendID, tc.message, MessageTypeNormal)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error from legacy API")
				} else if err.Error() != tc.expectedMsg {
					t.Errorf("Expected error message '%s', got '%s'", tc.expectedMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error from legacy API: %v", err)
			}
		})
	}
}

// TestMessageTypesAPI tests that different message types work correctly
func TestMessageTypesAPI(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create and connect a test friend
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Set friend as connected
	tox.friendsMutex.Lock()
	if friend, exists := tox.friends[friendID]; exists {
		friend.ConnectionStatus = ConnectionUDP
	}
	tox.friendsMutex.Unlock()

	// Test default message type (should be Normal)
	err = tox.SendFriendMessage(friendID, "Default type message")
	if err != nil {
		t.Errorf("Failed to send message with default type: %v", err)
	}

	// Test explicit Normal message type
	err = tox.SendFriendMessage(friendID, "Explicit normal message", MessageTypeNormal)
	if err != nil {
		t.Errorf("Failed to send normal message: %v", err)
	}

	// Test Action message type
	err = tox.SendFriendMessage(friendID, "/me sends an action", MessageTypeAction)
	if err != nil {
		t.Errorf("Failed to send action message: %v", err)
	}
}

// TestReadmeExampleCompatibility ensures the documented examples work
func TestReadmeExampleCompatibility(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// This should compile and work exactly as shown in README.md
	tox.OnFriendMessage(func(friendID uint32, message string) {
		// This is the exact line from README.md
		tox.SendFriendMessage(friendID, "You said: "+message)
	})

	// Verify the callback works (compilation test)
	if tox.simpleFriendMessageCallback == nil {
		t.Error("OnFriendMessage callback was not set properly")
	}

	t.Log("README.md example compatibility verified")
}
