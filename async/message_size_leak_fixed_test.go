package async

import (
	"bytes"
	"testing"
)

func TestMessageSizeLeakageFixed(t *testing.T) {
	// Create messages of different sizes
	smallMessage := []byte("Hello")
	mediumMessage := bytes.Repeat([]byte("A"), 300)
	largeMessage := bytes.Repeat([]byte("B"), 1500)

	// Apply padding to normalize message sizes
	paddedSmall := PadMessageToStandardSize(smallMessage)
	paddedMedium := PadMessageToStandardSize(mediumMessage)
	paddedLarge := PadMessageToStandardSize(largeMessage)

	// Test with different original message sizes
	t.Logf("Original small message size: %d bytes", len(smallMessage))
	t.Logf("Original medium message size: %d bytes", len(mediumMessage))
	t.Logf("Original large message size: %d bytes", len(largeMessage))

	// Log padded sizes to show normalization
	t.Logf("Padded small message size: %d bytes", len(paddedSmall))
	t.Logf("Padded medium message size: %d bytes", len(paddedMedium))
	t.Logf("Padded large message size: %d bytes", len(paddedLarge))

	// Verify that messages now fall into standard bucket sizes
	if len(paddedSmall) != MessageSizeSmall {
		t.Errorf("Small message not normalized correctly: %d bytes (expected %d)",
			len(paddedSmall), MessageSizeSmall)
	}

	if len(paddedMedium) != MessageSizeMedium {
		t.Errorf("Medium message not normalized correctly: %d bytes (expected %d)",
			len(paddedMedium), MessageSizeMedium)
	}

	if len(paddedLarge) != MessageSizeLarge {
		t.Errorf("Large message not normalized correctly: %d bytes (expected %d)",
			len(paddedLarge), MessageSizeLarge)
	}

	// Verify that another small message of different size produces the same padded size
	anotherSmall := []byte("Hi")
	paddedAnotherSmall := PadMessageToStandardSize(anotherSmall)

	if len(paddedAnotherSmall) != len(paddedSmall) {
		t.Errorf("Different small messages should have the same padded size")
		t.Logf("Padded size for 'Hello': %d bytes", len(paddedSmall))
		t.Logf("Padded size for 'Hi': %d bytes", len(paddedAnotherSmall))
	} else {
		t.Log("Different small messages successfully padded to same size")
	}

	// Verify that padding prevents leaking message size information
	if len(paddedSmall) == len(smallMessage) ||
		len(paddedMedium) == len(mediumMessage) ||
		len(paddedLarge) == len(largeMessage) {
		t.Error("Padding failed: padded size still reflects original message size")
	} else {
		t.Log("Padding successful: original message size obscured")
	}

	// Test round-trip padding/unpadding
	for _, original := range [][]byte{smallMessage, mediumMessage, largeMessage} {
		padded := PadMessageToStandardSize(original)
		unpadded, err := UnpadMessage(padded)
		if err != nil {
			t.Errorf("Failed to unpad message: %v", err)
			continue
		}

		if !bytes.Equal(unpadded, original) {
			t.Errorf("Round-trip padding/unpadding failed")
			t.Logf("Original: %v", original)
			t.Logf("After unpadding: %v", unpadded)
		}
	}
}
