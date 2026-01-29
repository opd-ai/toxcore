package async

import (
	"bytes"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// ============================================================================
// Message Size Leak Tests - Testing message size obfuscation and padding
// ============================================================================

// TestMessageSizeLeakageVulnerability demonstrates that without padding,
// message size is leaked through encrypted payload size.
func TestMessageSizeLeakageVulnerability(t *testing.T) {
	// Generate sender and recipient keys
	senderKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Create a new obfuscation manager
	epochManager := NewEpochManager()
	obfuscation := NewObfuscationManager(senderKey, epochManager)

	// Derive a shared secret for demonstration
	var sharedSecret [32]byte

	// Send messages of different sizes and check if they leak size information
	messageSmall := []byte("Small message")                                        // 13 bytes
	messageMedium := []byte("This is a medium-sized message for testing purposes") // 49 bytes
	messageLarge := bytes.Repeat([]byte("A"), 1024)                                // 1024 bytes

	// Create obfuscated messages of different sizes
	obfMsgSmall, err := obfuscation.CreateObfuscatedMessage(
		senderKey.Private, recipientKey.Public, messageSmall, sharedSecret)
	if err != nil {
		t.Fatalf("Failed to create small obfuscated message: %v", err)
	}

	obfMsgMedium, err := obfuscation.CreateObfuscatedMessage(
		senderKey.Private, recipientKey.Public, messageMedium, sharedSecret)
	if err != nil {
		t.Fatalf("Failed to create medium obfuscated message: %v", err)
	}

	obfMsgLarge, err := obfuscation.CreateObfuscatedMessage(
		senderKey.Private, recipientKey.Public, messageLarge, sharedSecret)
	if err != nil {
		t.Fatalf("Failed to create large obfuscated message: %v", err)
	}

	// Compare the sizes of the encrypted payloads
	smallSize := len(obfMsgSmall.EncryptedPayload)
	mediumSize := len(obfMsgMedium.EncryptedPayload)
	largeSize := len(obfMsgLarge.EncryptedPayload)

	t.Logf("Small message encrypted size: %d bytes", smallSize)
	t.Logf("Medium message encrypted size: %d bytes", mediumSize)
	t.Logf("Large message encrypted size: %d bytes", largeSize)

	// The test will demonstrate that messages of different sizes have different encrypted payload sizes
	if smallSize == mediumSize || smallSize == largeSize || mediumSize == largeSize {
		t.Fatalf("Expected encrypted payload sizes to differ, but got: small=%d, medium=%d, large=%d",
			smallSize, mediumSize, largeSize)
	}

	// This is where the vulnerability exists - the message size is leaked through the encrypted payload size
	t.Log("Vulnerability confirmed: message size leaks through encrypted payload size")
}

// TestMessageSizeLeakageFix tests that message padding normalizes message sizes
// to prevent size leakage.
func TestMessageSizeLeakageFix(t *testing.T) {
	// Create messages of different sizes
	smallMessage := []byte("Hello")
	mediumMessage := bytes.Repeat([]byte("A"), 300)
	largeMessage := bytes.Repeat([]byte("B"), 1500)

	// Apply padding to normalize message sizes
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
	paddedAnotherSmall, err := PadMessageToStandardSize(anotherSmall)
	if err != nil {
		t.Fatalf("Failed to pad another small message: %v", err)
	}

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
		padded, err := PadMessageToStandardSize(original)
		if err != nil {
			t.Errorf("Failed to pad message: %v", err)
			continue
		}
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
