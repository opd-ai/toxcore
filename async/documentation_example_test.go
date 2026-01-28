package async

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// TestDirectMessageStorageAPIExample validates the README.md documentation example
// for Direct Message Storage API using ForwardSecurityManager
func TestDirectMessageStorageAPIExample(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "doc_example_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create storage instance with automatic capacity
	storageKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate storage key pair: %v", err)
	}
	dataDir := filepath.Join(tempDir, "storage")
	storage := NewMessageStorage(storageKeyPair, dataDir)

	// Monitor storage capacity (automatically calculated)
	if storage.GetMaxCapacity() <= 0 {
		t.Error("Storage capacity should be greater than 0")
	}

	// Create forward security manager for forward-secure messaging
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}
	senderDataDir := filepath.Join(tempDir, "sender")
	senderFSM, err := NewForwardSecurityManager(senderKeyPair, senderDataDir)
	if err != nil {
		t.Fatalf("Failed to create sender FSM: %v", err)
	}

	// Recipient must also create their own forward security manager
	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}
	recipientDataDir := filepath.Join(tempDir, "recipient")
	recipientFSM, err := NewForwardSecurityManager(recipientKeyPair, recipientDataDir)
	if err != nil {
		t.Fatalf("Failed to create recipient FSM: %v", err)
	}

	// Step 1: Exchange pre-keys between sender and recipient (both directions)
	// Sender generates pre-keys for recipient
	if err := senderFSM.GeneratePreKeysForPeer(recipientKeyPair.Public); err != nil {
		t.Fatalf("Failed to generate sender pre-keys for recipient: %v", err)
	}
	senderPreKeyMsg, err := senderFSM.ExchangePreKeys(recipientKeyPair.Public)
	if err != nil {
		t.Fatalf("Failed to create sender pre-key exchange: %v", err)
	}

	// Recipient generates pre-keys for sender
	if err := recipientFSM.GeneratePreKeysForPeer(senderKeyPair.Public); err != nil {
		t.Fatalf("Failed to generate recipient pre-keys for sender: %v", err)
	}
	recipientPreKeyMsg, err := recipientFSM.ExchangePreKeys(senderKeyPair.Public)
	if err != nil {
		t.Fatalf("Failed to create recipient pre-key exchange: %v", err)
	}

	// Exchange pre-keys (in real usage, this happens over the network)
	if err := senderFSM.ProcessPreKeyExchange(recipientPreKeyMsg); err != nil {
		t.Fatalf("Failed to process recipient pre-key exchange on sender: %v", err)
	}
	if err := recipientFSM.ProcessPreKeyExchange(senderPreKeyMsg); err != nil {
		t.Fatalf("Failed to process sender pre-key exchange on recipient: %v", err)
	}

	// Verify sender can send messages
	if !senderFSM.CanSendMessage(recipientKeyPair.Public) {
		t.Error("Sender should be able to send messages after key exchange")
	}

	// Step 2: Send forward-secure message
	message := "Hello, offline friend!"
	fsMsg, err := senderFSM.SendForwardSecureMessage(recipientKeyPair.Public, []byte(message), MessageTypeNormal)
	if err != nil {
		t.Fatalf("Failed to send forward-secure message: %v", err)
	}

	// Verify message properties
	if fsMsg.SenderPK != senderKeyPair.Public {
		t.Error("Message sender PK should match sender's public key")
	}
	if fsMsg.RecipientPK != recipientKeyPair.Public {
		t.Error("Message recipient PK should match recipient's public key")
	}
	if fsMsg.MessageType != MessageTypeNormal {
		t.Error("Message type should be MessageTypeNormal")
	}

	// Step 3: Retrieve and decrypt messages (recipient side)
	// In real usage, recipient would retrieve stored forward-secure messages
	decrypted, err := recipientFSM.DecryptForwardSecureMessage(fsMsg)
	if err != nil {
		t.Fatalf("Failed to decrypt forward-secure message: %v", err)
	}

	// Verify decrypted message matches original
	if string(decrypted) != message {
		t.Errorf("Expected decrypted message %q, got %q", message, string(decrypted))
	}

	// Verify that the deprecated EncryptForRecipient function fails as expected
	_, _, err = EncryptForRecipient([]byte("test"), recipientKeyPair.Public, senderKeyPair.Private)
	if err == nil {
		t.Error("EncryptForRecipient should return an error (deprecated)")
	}
	if err != nil && err.Error() != "deprecated: EncryptForRecipient does not provide forward secrecy - use ForwardSecurityManager instead" {
		t.Errorf("Unexpected error from EncryptForRecipient: %v", err)
	}
}
