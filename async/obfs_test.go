package async

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

func TestNewObfuscationManager(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	epochManager := NewEpochManager()
	obfManager := NewObfuscationManager(keyPair, epochManager)

	if obfManager == nil {
		t.Fatal("NewObfuscationManager returned nil")
	}

	if obfManager.keyPair != keyPair {
		t.Error("ObfuscationManager key pair not set correctly")
	}

	if obfManager.epochManager != epochManager {
		t.Error("ObfuscationManager epoch manager not set correctly")
	}
}

func TestGenerateRecipientPseudonym(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	epochManager := NewEpochManager()
	obfManager := NewObfuscationManager(keyPair, epochManager)

	recipientPK := [32]byte{0x01, 0x02, 0x03}
	epoch := uint64(12345)

	// Test basic functionality
	pseudonym1, err := obfManager.GenerateRecipientPseudonym(recipientPK, epoch)
	if err != nil {
		t.Fatalf("GenerateRecipientPseudonym failed: %v", err)
	}

	// Should be deterministic - same inputs produce same output
	pseudonym2, err := obfManager.GenerateRecipientPseudonym(recipientPK, epoch)
	if err != nil {
		t.Fatalf("GenerateRecipientPseudonym failed on second call: %v", err)
	}

	if pseudonym1 != pseudonym2 {
		t.Error("GenerateRecipientPseudonym not deterministic")
	}

	// Different epochs should produce different pseudonyms
	pseudonym3, err := obfManager.GenerateRecipientPseudonym(recipientPK, epoch+1)
	if err != nil {
		t.Fatalf("GenerateRecipientPseudonym failed with different epoch: %v", err)
	}

	if pseudonym1 == pseudonym3 {
		t.Error("Different epochs produced same pseudonym")
	}

	// Different recipients should produce different pseudonyms
	differentRecipient := [32]byte{0x04, 0x05, 0x06}
	pseudonym4, err := obfManager.GenerateRecipientPseudonym(differentRecipient, epoch)
	if err != nil {
		t.Fatalf("GenerateRecipientPseudonym failed with different recipient: %v", err)
	}

	if pseudonym1 == pseudonym4 {
		t.Error("Different recipients produced same pseudonym")
	}
}

func TestGenerateSenderPseudonym(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	epochManager := NewEpochManager()
	obfManager := NewObfuscationManager(keyPair, epochManager)

	senderSK := [32]byte{0x01, 0x02, 0x03}
	recipientPK := [32]byte{0x04, 0x05, 0x06}
	messageNonce := [24]byte{0x07, 0x08, 0x09}

	// Test basic functionality
	pseudonym1, err := obfManager.GenerateSenderPseudonym(senderSK, recipientPK, messageNonce)
	if err != nil {
		t.Fatalf("GenerateSenderPseudonym failed: %v", err)
	}

	// Should be deterministic for same inputs
	pseudonym2, err := obfManager.GenerateSenderPseudonym(senderSK, recipientPK, messageNonce)
	if err != nil {
		t.Fatalf("GenerateSenderPseudonym failed on second call: %v", err)
	}

	if pseudonym1 != pseudonym2 {
		t.Error("GenerateSenderPseudonym not deterministic")
	}

	// Different nonces should produce different pseudonyms (unlinkability)
	differentNonce := [24]byte{0x0A, 0x0B, 0x0C}
	pseudonym3, err := obfManager.GenerateSenderPseudonym(senderSK, recipientPK, differentNonce)
	if err != nil {
		t.Fatalf("GenerateSenderPseudonym failed with different nonce: %v", err)
	}

	if pseudonym1 == pseudonym3 {
		t.Error("Different nonces produced same pseudonym - unlinkability broken")
	}

	// Different sender keys should produce different pseudonyms
	differentSender := [32]byte{0x0D, 0x0E, 0x0F}
	pseudonym4, err := obfManager.GenerateSenderPseudonym(differentSender, recipientPK, messageNonce)
	if err != nil {
		t.Fatalf("GenerateSenderPseudonym failed with different sender: %v", err)
	}

	if pseudonym1 == pseudonym4 {
		t.Error("Different senders produced same pseudonym")
	}

	// Different recipients should produce different pseudonyms
	differentRecipient := [32]byte{0x10, 0x11, 0x12}
	pseudonym5, err := obfManager.GenerateSenderPseudonym(senderSK, differentRecipient, messageNonce)
	if err != nil {
		t.Fatalf("GenerateSenderPseudonym failed with different recipient: %v", err)
	}

	if pseudonym1 == pseudonym5 {
		t.Error("Different recipients produced same pseudonym")
	}
}

func TestRecipientProofGeneration(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	epochManager := NewEpochManager()
	obfManager := NewObfuscationManager(keyPair, epochManager)

	recipientPK := [32]byte{0x01, 0x02, 0x03}
	messageID := [32]byte{0x04, 0x05, 0x06}
	epoch := uint64(12345)

	// Test basic generation
	proof1, err := obfManager.GenerateRecipientProof(recipientPK, messageID, epoch)
	if err != nil {
		t.Fatalf("GenerateRecipientProof failed: %v", err)
	}

	// Should be deterministic
	proof2, err := obfManager.GenerateRecipientProof(recipientPK, messageID, epoch)
	if err != nil {
		t.Fatalf("GenerateRecipientProof failed on second call: %v", err)
	}

	if proof1 != proof2 {
		t.Error("GenerateRecipientProof not deterministic")
	}

	// Different inputs should produce different proofs
	differentMessage := [32]byte{0x07, 0x08, 0x09}
	proof3, err := obfManager.GenerateRecipientProof(recipientPK, differentMessage, epoch)
	if err != nil {
		t.Fatalf("GenerateRecipientProof failed with different message: %v", err)
	}

	if proof1 == proof3 {
		t.Error("Different message IDs produced same proof")
	}
}

func TestRecipientProofVerification(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	epochManager := NewEpochManager()
	obfManager := NewObfuscationManager(keyPair, epochManager)

	recipientPK := [32]byte{0x01, 0x02, 0x03}
	messageID := [32]byte{0x04, 0x05, 0x06}
	epoch := uint64(12345)

	// Generate a valid proof
	proof, err := obfManager.GenerateRecipientProof(recipientPK, messageID, epoch)
	if err != nil {
		t.Fatalf("GenerateRecipientProof failed: %v", err)
	}

	// Valid proof should verify
	if !obfManager.VerifyRecipientProof(recipientPK, messageID, epoch, proof) {
		t.Error("Valid proof failed verification")
	}

	// Invalid proof should not verify
	invalidProof := [32]byte{0xFF, 0xFF, 0xFF}
	if obfManager.VerifyRecipientProof(recipientPK, messageID, epoch, invalidProof) {
		t.Error("Invalid proof passed verification")
	}

	// Wrong recipient should not verify
	wrongRecipient := [32]byte{0x0A, 0x0B, 0x0C}
	if obfManager.VerifyRecipientProof(wrongRecipient, messageID, epoch, proof) {
		t.Error("Proof verified with wrong recipient")
	}

	// Wrong message ID should not verify
	wrongMessage := [32]byte{0x0D, 0x0E, 0x0F}
	if obfManager.VerifyRecipientProof(recipientPK, wrongMessage, epoch, proof) {
		t.Error("Proof verified with wrong message ID")
	}

	// Wrong epoch should not verify
	if obfManager.VerifyRecipientProof(recipientPK, messageID, epoch+1, proof) {
		t.Error("Proof verified with wrong epoch")
	}
}

func TestPayloadKeyDerivation(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	epochManager := NewEpochManager()
	obfManager := NewObfuscationManager(keyPair, epochManager)

	sharedSecret := [32]byte{0x01, 0x02, 0x03}
	messageNonce := [24]byte{0x04, 0x05, 0x06}
	epoch := uint64(12345)

	// Test basic derivation
	key1, err := obfManager.DerivePayloadKey(sharedSecret, messageNonce, epoch)
	if err != nil {
		t.Fatalf("DerivePayloadKey failed: %v", err)
	}

	// Should be deterministic
	key2, err := obfManager.DerivePayloadKey(sharedSecret, messageNonce, epoch)
	if err != nil {
		t.Fatalf("DerivePayloadKey failed on second call: %v", err)
	}

	if key1 != key2 {
		t.Error("DerivePayloadKey not deterministic")
	}

	// Different inputs should produce different keys
	differentSecret := [32]byte{0x07, 0x08, 0x09}
	key3, err := obfManager.DerivePayloadKey(differentSecret, messageNonce, epoch)
	if err != nil {
		t.Fatalf("DerivePayloadKey failed with different secret: %v", err)
	}

	if key1 == key3 {
		t.Error("Different shared secrets produced same key")
	}

	// Different epochs should produce different keys (forward secrecy)
	key4, err := obfManager.DerivePayloadKey(sharedSecret, messageNonce, epoch+1)
	if err != nil {
		t.Fatalf("DerivePayloadKey failed with different epoch: %v", err)
	}

	if key1 == key4 {
		t.Error("Different epochs produced same key - forward secrecy broken")
	}
}

func TestPayloadEncryptionDecryption(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	epochManager := NewEpochManager()
	obfManager := NewObfuscationManager(keyPair, epochManager)

	// Test data
	payload := []byte("This is a test message for encryption")
	payloadKey := [32]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20,
	}

	// Test encryption
	encryptedData, nonce, tag, err := obfManager.EncryptPayload(payload, payloadKey)
	if err != nil {
		t.Fatalf("EncryptPayload failed: %v", err)
	}

	if len(encryptedData) == 0 {
		t.Error("Encrypted data is empty")
	}

	if bytes.Equal(encryptedData, payload) {
		t.Error("Encrypted data equals plaintext - encryption failed")
	}

	// Test decryption
	decryptedData, err := obfManager.DecryptPayload(encryptedData, nonce, tag, payloadKey)
	if err != nil {
		t.Fatalf("DecryptPayload failed: %v", err)
	}

	if !bytes.Equal(decryptedData, payload) {
		t.Error("Decrypted data does not match original payload")
	}

	// Test with wrong key
	wrongKey := [32]byte{
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	}

	_, err = obfManager.DecryptPayload(encryptedData, nonce, tag, wrongKey)
	if err == nil {
		t.Error("Decryption with wrong key should fail")
	}

	// Test with corrupted tag
	corruptedTag := tag
	corruptedTag[0] ^= 0xFF
	_, err = obfManager.DecryptPayload(encryptedData, nonce, corruptedTag, payloadKey)
	if err == nil {
		t.Error("Decryption with corrupted tag should fail")
	}
}

func TestCreateObfuscatedMessage(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	epochManager := NewEpochManager()
	obfManager := NewObfuscationManager(keyPair, epochManager)

	senderSK := [32]byte{0x01, 0x02, 0x03}
	recipientPK := [32]byte{0x04, 0x05, 0x06}
	sharedSecret := [32]byte{0x07, 0x08, 0x09}
	forwardSecureMsg := []byte("Test forward secure message")

	// Test message creation
	obfMsg, err := obfManager.CreateObfuscatedMessage(senderSK, recipientPK, forwardSecureMsg, sharedSecret)
	if err != nil {
		t.Fatalf("CreateObfuscatedMessage failed: %v", err)
	}

	// Verify message structure
	if obfMsg == nil {
		t.Fatal("CreateObfuscatedMessage returned nil")
	}

	if obfMsg.Type != "obfuscated_async_message" {
		t.Errorf("Expected type 'obfuscated_async_message', got '%s'", obfMsg.Type)
	}

	if obfMsg.Epoch != obfManager.epochManager.GetCurrentEpoch() {
		t.Errorf("Expected epoch %d, got %d", obfManager.epochManager.GetCurrentEpoch(), obfMsg.Epoch)
	}

	if len(obfMsg.EncryptedPayload) == 0 {
		t.Error("Encrypted payload is empty")
	}

	if time.Until(obfMsg.ExpiresAt) > 25*time.Hour || time.Until(obfMsg.ExpiresAt) < 23*time.Hour {
		t.Error("Message expiration time not set correctly (should be ~24 hours)")
	}

	// Verify that pseudonyms are different from real keys
	if bytes.Equal(obfMsg.SenderPseudonym[:], senderSK[:]) {
		t.Error("Sender pseudonym equals sender private key")
	}

	if bytes.Equal(obfMsg.RecipientPseudonym[:], recipientPK[:]) {
		t.Error("Recipient pseudonym equals recipient public key")
	}

	// Verify recipient proof is valid
	if !obfManager.VerifyRecipientProof(recipientPK, obfMsg.MessageID, obfMsg.Epoch, obfMsg.RecipientProof) {
		t.Error("Generated recipient proof is invalid")
	}
}

func TestDecryptObfuscatedMessage(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	epochManager := NewEpochManager()
	obfManager := NewObfuscationManager(keyPair, epochManager)

	senderSK := [32]byte{0x01, 0x02, 0x03}
	recipientPK := keyPair.Public // Use the manager's public key as recipient
	senderPK := [32]byte{0x07, 0x08, 0x09}
	sharedSecret := [32]byte{0x0A, 0x0B, 0x0C}
	forwardSecureMsg := []byte("Test forward secure message")

	// Create an obfuscated message
	obfMsg, err := obfManager.CreateObfuscatedMessage(senderSK, recipientPK, forwardSecureMsg, sharedSecret)
	if err != nil {
		t.Fatalf("CreateObfuscatedMessage failed: %v", err)
	}

	// Test successful decryption
	decryptedMsg, err := obfManager.DecryptObfuscatedMessage(obfMsg, keyPair.Private, senderPK, sharedSecret)
	if err != nil {
		t.Fatalf("DecryptObfuscatedMessage failed: %v", err)
	}

	if !bytes.Equal(decryptedMsg, forwardSecureMsg) {
		t.Error("Decrypted message does not match original")
	}

	// Test with wrong recipient (different key pair)
	wrongKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate wrong key pair: %v", err)
	}

	wrongObfManager := NewObfuscationManager(wrongKeyPair, epochManager)
	_, err = wrongObfManager.DecryptObfuscatedMessage(obfMsg, wrongKeyPair.Private, senderPK, sharedSecret)
	if err == nil {
		t.Error("Decryption should fail with wrong recipient key")
	}

	// Test with corrupted recipient proof
	corruptedMsg := *obfMsg
	corruptedMsg.RecipientProof[0] ^= 0xFF
	_, err = obfManager.DecryptObfuscatedMessage(&corruptedMsg, keyPair.Private, senderPK, sharedSecret)
	if err == nil {
		t.Error("Decryption should fail with corrupted recipient proof")
	}
}

func TestEndToEndObfuscation(t *testing.T) {
	// Simulate a complete end-to-end obfuscation workflow

	// Create sender and recipient managers
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	epochManager := NewEpochManager()
	senderObfManager := NewObfuscationManager(senderKeyPair, epochManager)
	recipientObfManager := NewObfuscationManager(recipientKeyPair, epochManager)

	// Simulate shared secret computation (normally done via key exchange)
	sharedSecret := [32]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x20,
	}

	originalMessage := []byte("Hello, this is a secret message!")

	// Sender creates obfuscated message
	obfMsg, err := senderObfManager.CreateObfuscatedMessage(
		senderKeyPair.Private,
		recipientKeyPair.Public,
		originalMessage,
		sharedSecret,
	)
	if err != nil {
		t.Fatalf("Sender failed to create obfuscated message: %v", err)
	}

	// Recipient decrypts obfuscated message
	decryptedMessage, err := recipientObfManager.DecryptObfuscatedMessage(
		obfMsg,
		recipientKeyPair.Private,
		senderKeyPair.Public,
		sharedSecret,
	)
	if err != nil {
		t.Fatalf("Recipient failed to decrypt obfuscated message: %v", err)
	}

	// Verify the message was transmitted correctly
	if !bytes.Equal(decryptedMessage, originalMessage) {
		t.Errorf("Message corrupted during obfuscation: expected %s, got %s",
			string(originalMessage), string(decryptedMessage))
	}
}

// Benchmark tests to ensure performance is acceptable
func BenchmarkGenerateRecipientPseudonym(b *testing.B) {
	keyPair, _ := crypto.GenerateKeyPair()
	epochManager := NewEpochManager()
	obfManager := NewObfuscationManager(keyPair, epochManager)

	recipientPK := [32]byte{0x01, 0x02, 0x03}
	epoch := uint64(12345)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = obfManager.GenerateRecipientPseudonym(recipientPK, epoch)
	}
}

func BenchmarkGenerateSenderPseudonym(b *testing.B) {
	keyPair, _ := crypto.GenerateKeyPair()
	epochManager := NewEpochManager()
	obfManager := NewObfuscationManager(keyPair, epochManager)

	senderSK := [32]byte{0x01, 0x02, 0x03}
	recipientPK := [32]byte{0x04, 0x05, 0x06}
	messageNonce := [24]byte{0x07, 0x08, 0x09}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = obfManager.GenerateSenderPseudonym(senderSK, recipientPK, messageNonce)
	}
}

func BenchmarkEncryptPayload(b *testing.B) {
	keyPair, _ := crypto.GenerateKeyPair()
	epochManager := NewEpochManager()
	obfManager := NewObfuscationManager(keyPair, epochManager)

	payload := make([]byte, 1024) // 1KB payload
	rand.Read(payload)

	payloadKey := [32]byte{}
	rand.Read(payloadKey[:])

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = obfManager.EncryptPayload(payload, payloadKey)
	}
}

func BenchmarkCreateObfuscatedMessage(b *testing.B) {
	keyPair, _ := crypto.GenerateKeyPair()
	epochManager := NewEpochManager()
	obfManager := NewObfuscationManager(keyPair, epochManager)

	senderSK := [32]byte{0x01, 0x02, 0x03}
	recipientPK := [32]byte{0x04, 0x05, 0x06}
	sharedSecret := [32]byte{0x07, 0x08, 0x09}
	forwardSecureMsg := make([]byte, 1024) // 1KB message
	rand.Read(forwardSecureMsg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = obfManager.CreateObfuscatedMessage(senderSK, recipientPK, forwardSecureMsg, sharedSecret)
	}
}
