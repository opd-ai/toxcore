package messaging

import (
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/limits"
)

func TestSendMessage_MaxLengthValidation(t *testing.T) {
	tests := []struct {
		name       string
		textLength int
		expectErr  error
	}{
		{"Empty message rejected", 0, nil},                                            // empty has its own error
		{"Single byte accepted", 1, nil},                                              // minimum valid
		{"Max length accepted", limits.MaxPlaintextMessage, nil},                      // exactly at limit
		{"One over max rejected", limits.MaxPlaintextMessage + 1, ErrMessageTooLong},  // one over limit
		{"Large message rejected", limits.MaxPlaintextMessage * 2, ErrMessageTooLong}, // way over limit
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mm := NewMessageManager()

			text := strings.Repeat("a", tt.textLength)
			_, err := mm.SendMessage(testDefaultFriendID, text, MessageTypeNormal)

			if tt.textLength == 0 {
				// Empty message has its own error
				if err == nil {
					t.Error("expected error for empty message")
				}
				return
			}

			if tt.expectErr != nil {
				if err != tt.expectErr {
					t.Errorf("expected error %v, got %v", tt.expectErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestPadMessage(t *testing.T) {
	tests := []struct {
		name        string
		inputLen    int
		expectedLen int
	}{
		{"Empty message pads to 256", 0, 256},
		{"1 byte pads to 256", 1, 256},
		{"255 bytes pads to 256", 255, 256},
		{"256 bytes stays 256", 256, 256},
		{"257 bytes pads to 1024", 257, 1024},
		{"1023 bytes pads to 1024", 1023, 1024},
		{"1024 bytes stays 1024", 1024, 1024},
		{"1025 bytes pads to 4096", 1025, 4096},
		{"4095 bytes pads to 4096", 4095, 4096},
		{"4096 bytes stays 4096", 4096, 4096},
		{"4097 bytes unchanged", 4097, 4097}, // exceeds all tiers
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := make([]byte, tt.inputLen)
			// Fill with non-zero pattern to verify content preservation
			for i := range input {
				input[i] = byte(i % 256)
			}

			result := padMessage(input)

			if len(result) != tt.expectedLen {
				t.Errorf("expected padded length %d, got %d", tt.expectedLen, len(result))
			}

			// Verify original content is preserved
			for i := 0; i < tt.inputLen && i < len(result); i++ {
				if result[i] != input[i] {
					t.Errorf("content at position %d changed: expected %d, got %d", i, input[i], result[i])
					break
				}
			}

			// Verify padding bytes are zero
			for i := tt.inputLen; i < len(result); i++ {
				if result[i] != 0 {
					t.Errorf("padding at position %d should be zero, got %d", i, result[i])
					break
				}
			}
		})
	}
}

func TestPadMessage_ContentPreservation(t *testing.T) {
	// Test that message content is preserved exactly
	original := []byte("Hello, World! This is a test message.")
	padded := padMessage(original)

	if len(padded) != 256 {
		t.Errorf("expected padded length 256, got %d", len(padded))
	}

	for i, b := range original {
		if padded[i] != b {
			t.Errorf("byte at position %d differs: expected %d, got %d", i, b, padded[i])
		}
	}
}

func TestTimeProvider_DeterministicTimestamp(t *testing.T) {
	mm := NewMessageManager()

	// Set deterministic time
	fixedTime := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	mockTime := &mockTimeProvider{currentTime: fixedTime}
	mm.SetTimeProvider(mockTime)

	// Send a message
	msg, err := mm.SendMessage(testDefaultFriendID, "test", MessageTypeNormal)
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// Verify timestamp is deterministic
	if !msg.Timestamp.Equal(fixedTime) {
		t.Errorf("expected timestamp %v, got %v", fixedTime, msg.Timestamp)
	}
}

func TestTimeProvider_RetryIntervalControl(t *testing.T) {
	mm := NewMessageManager()
	mm.retryInterval = testRetryInterval

	// Set deterministic time
	startTime := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	mockTime := &mockTimeProvider{currentTime: startTime}
	mm.SetTimeProvider(mockTime)

	// Create message directly to test shouldProcessMessage
	msg := newMessageWithTime(1, "test", MessageTypeNormal, startTime)
	msg.LastAttempt = startTime

	// Before retry interval: should NOT process
	mockTime.Advance(testRetryAdvanceStep)
	if mm.shouldProcessMessage(msg) {
		t.Error("message should not be ready before retry interval")
	}

	// After retry interval: should process
	mockTime.Advance(testRetryAdvanceStep) // Now 6 seconds total
	// Need to set message back to pending state
	msg.SetState(MessageStatePending)
	if !mm.shouldProcessMessage(msg) {
		t.Error("message should be ready after retry interval")
	}
}

func TestDefaultTimeProvider(t *testing.T) {
	tp := DefaultTimeProvider{}

	// Verify Now() returns approximately current time
	before := time.Now()
	actual := tp.Now()
	after := time.Now()

	if actual.Before(before) || actual.After(after) {
		t.Error("DefaultTimeProvider.Now() should return current time")
	}

	// Verify Since() works correctly
	past := time.Now().Add(-time.Second)
	since := tp.Since(past)
	if since < time.Second || since > 2*time.Second {
		t.Errorf("DefaultTimeProvider.Since() returned unexpected duration: %v", since)
	}
}
