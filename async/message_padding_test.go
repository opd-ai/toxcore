package async

import (
	"bytes"
	"testing"
)

func TestMessagePadding(t *testing.T) {
	// Test cases with different message sizes
	testCases := []struct {
		name    string
		message []byte
	}{
		{"EmptyMessage", []byte{}},
		{"SmallMessage", []byte("Hello world")},
		{"MediumMessage", bytes.Repeat([]byte("A"), 300)},
		{"LargeMessage", bytes.Repeat([]byte("B"), 1500)},
		{"VeryLargeMessage", bytes.Repeat([]byte("C"), 5000)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Pad the message
			padded, err := PadMessageToStandardSize(tc.message)
			if err != nil {
				t.Fatalf("Failed to pad message: %v", err)
			}

			// Verify padding creates a standard bucket size
			switch {
			case len(tc.message) <= MessageSizeSmall:
				if len(padded) != MessageSizeSmall {
					t.Errorf("Expected padded size %d, got %d", MessageSizeSmall, len(padded))
				}
			case len(tc.message) <= MessageSizeMedium:
				if len(padded) != MessageSizeMedium {
					t.Errorf("Expected padded size %d, got %d", MessageSizeMedium, len(padded))
				}
			case len(tc.message) <= MessageSizeLarge:
				if len(padded) != MessageSizeLarge {
					t.Errorf("Expected padded size %d, got %d", MessageSizeLarge, len(padded))
				}
			default:
				if len(padded) != MessageSizeMax {
					t.Errorf("Expected padded size %d, got %d", MessageSizeMax, len(padded))
				}
			}

			// Unpad and verify we get the original message back
			unpadded, err := UnpadMessage(padded)
			if err != nil {
				t.Fatalf("Failed to unpad message: %v", err)
			}

			if !bytes.Equal(unpadded, tc.message) {
				t.Errorf("Original message and unpadded message don't match")
				t.Logf("Original: %v", tc.message)
				t.Logf("Unpadded: %v", unpadded)
			}
		})
	}
}

func TestMessageSizeNormalization(t *testing.T) {
	// Create messages of different sizes
	smallMessage := []byte("Hello")
	mediumMessage := bytes.Repeat([]byte("A"), 300) // Medium message > 256 bytes
	largeMessage := bytes.Repeat([]byte("B"), 1500) // Large message > 1024 bytes

	// Pad each message
	paddedSmall, err := PadMessageToStandardSize(smallMessage)
	if err != nil {
		t.Fatalf("Failed to pad small message: %v", err)
	}
	paddedMedium, err := PadMessageToStandardSize(mediumMessage)
	if err != nil {
		t.Fatalf("Failed to pad medium message: %v", err)
	}
	paddedLarge, err := PadMessageToStandardSize(largeMessage)
	if err != nil {
		t.Fatalf("Failed to pad large message: %v", err)
	}

	// Verify all small messages have the same size after padding
	smallMessage2 := []byte("Hi")
	paddedSmall2, err := PadMessageToStandardSize(smallMessage2)
	if err != nil {
		t.Fatalf("Failed to pad small message 2: %v", err)
	}
	if len(paddedSmall) != len(paddedSmall2) {
		t.Errorf("Small messages should have the same size after padding: %d vs %d", len(paddedSmall), len(paddedSmall2))
	}

	// Verify messages fall into distinct size buckets
	if len(paddedSmall) == len(paddedMedium) || len(paddedMedium) == len(paddedLarge) {
		t.Errorf("Messages should fall into distinct size buckets")
	}

	t.Logf("Padded small message size: %d bytes", len(paddedSmall))
	t.Logf("Padded medium message size: %d bytes", len(paddedMedium))
	t.Logf("Padded large message size: %d bytes", len(paddedLarge))
}

func TestMessageTruncationError(t *testing.T) {
	// Test that messages exceeding the maximum size return an error
	tooLargeMessage := bytes.Repeat([]byte("X"), MessageSizeMax)
	
	_, err := PadMessageToStandardSize(tooLargeMessage)
	if err == nil {
		t.Error("Expected error for message exceeding maximum size, got nil")
	}
	
	if err != ErrMessageTooLarge {
		t.Errorf("Expected ErrMessageTooLarge, got: %v", err)
	}
	
	// Test edge case: exactly at the limit (MessageSizeMax - LengthPrefixSize) should succeed
	maxAllowedMessage := bytes.Repeat([]byte("Y"), MessageSizeMax-LengthPrefixSize)
	padded, err := PadMessageToStandardSize(maxAllowedMessage)
	if err != nil {
		t.Errorf("Message at exact size limit should not error: %v", err)
	}
	if len(padded) != MessageSizeMax {
		t.Errorf("Expected padded size %d, got %d", MessageSizeMax, len(padded))
	}
	
	// Verify it can be unpadded correctly
	unpadded, err := UnpadMessage(padded)
	if err != nil {
		t.Errorf("Failed to unpad max-size message: %v", err)
	}
	if !bytes.Equal(unpadded, maxAllowedMessage) {
		t.Error("Max-size message did not round-trip correctly")
	}
	
	// Test one byte over the limit
	justOverLimit := bytes.Repeat([]byte("Z"), MessageSizeMax-LengthPrefixSize+1)
	_, err = PadMessageToStandardSize(justOverLimit)
	if err != ErrMessageTooLarge {
		t.Errorf("Expected ErrMessageTooLarge for message one byte over limit, got: %v", err)
	}
}
