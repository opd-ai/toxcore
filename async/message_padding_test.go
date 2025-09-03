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
			padded := PadMessageToStandardSize(tc.message)
			
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
	mediumMessage := bytes.Repeat([]byte("A"), 300)  // Medium message > 256 bytes
	largeMessage := bytes.Repeat([]byte("B"), 1500)  // Large message > 1024 bytes

	// Pad each message
	paddedSmall := PadMessageToStandardSize(smallMessage)
	paddedMedium := PadMessageToStandardSize(mediumMessage)
	paddedLarge := PadMessageToStandardSize(largeMessage)

	// Verify all small messages have the same size after padding
	smallMessage2 := []byte("Hi")
	paddedSmall2 := PadMessageToStandardSize(smallMessage2)
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
