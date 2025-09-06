package toxcore

import (
	"testing"
)

// test_edge_case_empty_message_validation_inconsistency reproduces the bug where
// empty message handling is inconsistent between send and receive paths.
// Send path returns error, receive path silently ignores.
func TestEmptyMessageValidationInconsistency(t *testing.T) {
	// Test the send path validation - should return error for empty message
	tox := &Tox{}
	err := tox.validateMessageInput("")
	if err == nil {
		t.Error("Expected validateMessageInput to return error for empty message, got nil")
	}
	expectedError := "message cannot be empty"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}

	// Test the receive path behavior - should now consistently ignore empty messages
	// using the same validation logic as the send path
	tox.friends = make(map[uint32]*Friend)
	tox.friends[1] = &Friend{PublicKey: [32]byte{1}}

	// Set up a callback to verify if message was processed
	messageReceived := false
	tox.OnFriendMessage(func(friendID uint32, message string) {
		messageReceived = true
	})

	// Call receiveFriendMessage with empty message - should now use consistent validation
	tox.receiveFriendMessage(1, "", MessageTypeNormal)

	// After fix: both paths should consistently reject empty messages
	// Send path: validateMessageInput("") returns error
	// Receive path: receiveFriendMessage(id, "", type) silently ignores (no callback)
	if messageReceived {
		t.Error("Empty message was processed by receive path - validation inconsistency still exists")
	}

	// Test that valid messages still work
	tox.receiveFriendMessage(1, "Hello", MessageTypeNormal)
	if !messageReceived {
		t.Error("Valid message was not processed by receive path")
	}
}
