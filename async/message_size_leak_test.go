package async

import (
	"bytes"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

func TestMessageSizeLeakage(t *testing.T) {
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
	messageSmall := []byte("Small message")          // 13 bytes
	messageMedium := []byte("This is a medium-sized message for testing purposes") // 49 bytes
	messageLarge := bytes.Repeat([]byte("A"), 1024)  // 1024 bytes
	
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
