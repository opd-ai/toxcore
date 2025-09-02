package toxcore

import (
	"strings"
	"testing"
)

// TestGap4MessageLengthUTF8ByteCounting tests that message length validation
// correctly counts UTF-8 bytes, not Unicode code points
// Regression test for Gap #4: Message Length Validation Uses Byte Count Instead of UTF-8 Rune Count
func TestGap4MessageLengthUTF8ByteCounting(t *testing.T) {
	// Create a minimal Tox instance for testing
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test cases demonstrating correct UTF-8 byte counting
	testCases := []struct {
		name          string
		message       string
		expectedBytes int
		shouldPass    bool
		description   string
	}{
		{
			name:          "simple ASCII text",
			message:       "Hello, World!",
			expectedBytes: 13,
			shouldPass:    true,
			description:   "ASCII characters are 1 byte each",
		},
		{
			name:          "emoji characters",
			message:       "🎉🎊🎈",
			expectedBytes: 12, // Each emoji is 4 bytes in UTF-8
			shouldPass:    true,
			description:   "Emojis are multiple bytes in UTF-8",
		},
		{
			name:          "mixed text and emoji",
			message:       "Hello 🎉",
			expectedBytes: 10, // "Hello " (6 bytes) + 🎉 (4 bytes)
			shouldPass:    true,
			description:   "Mixed ASCII and emoji",
		},
		{
			name:          "maximum allowed length",
			message:       strings.Repeat("a", 1372),
			expectedBytes: 1372,
			shouldPass:    true,
			description:   "Exactly at the 1372 byte limit",
		},
		{
			name:          "over limit with ASCII",
			message:       strings.Repeat("a", 1373),
			expectedBytes: 1373,
			shouldPass:    false,
			description:   "One byte over the limit",
		},
		{
			name:          "over limit with emoji",
			message:       strings.Repeat("🎉", 344), // 344 * 4 = 1376 bytes
			expectedBytes: 1376,
			shouldPass:    false,
			description:   "Over limit due to multi-byte UTF-8 characters",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify our expected byte count is correct
			actualBytes := len([]byte(tc.message))
			if actualBytes != tc.expectedBytes {
				t.Errorf("Test case setup error: expected %d bytes, got %d bytes for message %q",
					tc.expectedBytes, actualBytes, tc.message)
			}

			// Test the message validation
			// We use a dummy friend ID since we're only testing length validation
			err := tox.SendFriendMessage(0, tc.message)

			if tc.shouldPass {
				// For valid messages, we expect an error about friend not existing, not length
				if err != nil && strings.Contains(err.Error(), "message too long") {
					t.Errorf("Expected message to pass length validation, but got length error: %v", err)
				}
			} else {
				// For invalid messages, we expect a length error
				if err == nil || !strings.Contains(err.Error(), "message too long") {
					t.Errorf("Expected 'message too long' error, but got: %v", err)
				}
			}

			t.Logf("%s: %d bytes (%d characters) - %s",
				tc.description, actualBytes, len([]rune(tc.message)), tc.message[:min(20, len(tc.message))])
		})
	}
}

// Helper function for Go versions that don't have min built-in
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
