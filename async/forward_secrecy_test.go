package async

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/toxforge/crypto"
)

func TestPreKeyStore(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "prekey_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test key pair
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create pre-key store
	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}

	// Test generating pre-keys
	peerPK := [32]byte{0x01, 0x02, 0x03, 0x04}
	bundle, err := store.GeneratePreKeys(peerPK)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	if len(bundle.Keys) != PreKeysPerPeer {
		t.Errorf("Expected %d pre-keys, got %d", PreKeysPerPeer, len(bundle.Keys))
	}

	if bundle.UsedCount != 0 {
		t.Errorf("Expected 0 used keys, got %d", bundle.UsedCount)
	}

	// Test getting available pre-key
	preKey, err := store.GetAvailablePreKey(peerPK)
	if err != nil {
		t.Fatalf("Failed to get available pre-key: %v", err)
	}

	if !preKey.Used {
		t.Error("Pre-key should be marked as used after retrieval")
	}

	// Check that used count increased
	bundle, err = store.GetBundle(peerPK)
	if err != nil {
		t.Fatalf("Failed to get bundle: %v", err)
	}

	if bundle.UsedCount != 1 {
		t.Errorf("Expected 1 used key, got %d", bundle.UsedCount)
	}

	// Test remaining key count
	remaining := store.GetRemainingKeyCount(peerPK)
	if remaining != PreKeysPerPeer-1 {
		t.Errorf("Expected %d remaining keys, got %d", PreKeysPerPeer-1, remaining)
	}
}

func TestPreKeyPersistence(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "prekey_persistence_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test key pair
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create first store and generate pre-keys
	store1, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create first pre-key store: %v", err)
	}

	peerPK := [32]byte{0x01, 0x02, 0x03, 0x04}
	_, err = store1.GeneratePreKeys(peerPK)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	// Use one pre-key
	_, err = store1.GetAvailablePreKey(peerPK)
	if err != nil {
		t.Fatalf("Failed to get available pre-key: %v", err)
	}

	// Create second store (simulating restart)
	store2, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create second pre-key store: %v", err)
	}

	// Check that bundle was loaded
	bundle, err := store2.GetBundle(peerPK)
	if err != nil {
		t.Fatalf("Failed to get bundle from second store: %v", err)
	}

	if bundle.UsedCount != 1 {
		t.Errorf("Expected 1 used key after reload, got %d", bundle.UsedCount)
	}

	// We now completely remove used keys, so the total count should be 1 less than the original
	if len(bundle.Keys) != PreKeysPerPeer-1 {
		t.Errorf("Expected %d pre-keys after reload, got %d", PreKeysPerPeer-1, len(bundle.Keys))
	}
}

func TestForwardSecurityManager(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "forward_security_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test key pairs
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Create forward security managers
	senderFSM, err := NewForwardSecurityManager(senderKeyPair, filepath.Join(tempDir, "sender"))
	if err != nil {
		t.Fatalf("Failed to create sender FSM: %v", err)
	}

	recipientFSM, err := NewForwardSecurityManager(recipientKeyPair, filepath.Join(tempDir, "recipient"))
	if err != nil {
		t.Fatalf("Failed to create recipient FSM: %v", err)
	}

	// Generate pre-keys for sender
	err = recipientFSM.GeneratePreKeysForPeer(senderKeyPair.Public)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys for sender: %v", err)
	}

	// Create pre-key exchange
	exchange, err := recipientFSM.ExchangePreKeys(senderKeyPair.Public)
	if err != nil {
		t.Fatalf("Failed to create pre-key exchange: %v", err)
	}

	// Process pre-key exchange on sender side
	err = senderFSM.ProcessPreKeyExchange(exchange)
	if err != nil {
		t.Fatalf("Failed to process pre-key exchange: %v", err)
	}

	// Check that sender can now send messages
	if !senderFSM.CanSendMessage(recipientKeyPair.Public) {
		t.Error("Sender should be able to send messages after key exchange")
	}

	// Test sending forward-secure message
	message := "Hello, forward secrecy!"
	fsMsg, err := senderFSM.SendForwardSecureMessage(recipientKeyPair.Public, []byte(message), MessageTypeNormal)
	if err != nil {
		t.Fatalf("Failed to send forward-secure message: %v", err)
	}

	if fsMsg.SenderPK != senderKeyPair.Public {
		t.Error("Message sender PK should match sender's public key")
	}

	if fsMsg.RecipientPK != recipientKeyPair.Public {
		t.Error("Message recipient PK should match recipient's public key")
	}

	// Test that sender has one less available key
	availableKeys := senderFSM.GetAvailableKeyCount(recipientKeyPair.Public)
	if availableKeys != PreKeysPerPeer-1 {
		t.Errorf("Expected %d available keys after sending message, got %d", PreKeysPerPeer-1, availableKeys)
	}
}

func TestPreKeyRefresh(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "prekey_refresh_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test key pair
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create pre-key store
	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}

	peerPK := [32]byte{0x01, 0x02, 0x03, 0x04}

	// Initially should need refresh (no bundle exists)
	if !store.NeedsRefresh(peerPK) {
		t.Error("Should need refresh when no bundle exists")
	}

	// Generate initial pre-keys
	_, err = store.GeneratePreKeys(peerPK)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	// Should not need refresh with fresh bundle
	if store.NeedsRefresh(peerPK) {
		t.Error("Should not need refresh with fresh bundle")
	}

	// Use most keys to trigger refresh threshold
	for i := 0; i < PreKeysPerPeer-PreKeyRefreshThreshold; i++ {
		_, err = store.GetAvailablePreKey(peerPK)
		if err != nil {
			t.Fatalf("Failed to get pre-key %d: %v", i, err)
		}
	}

	// Should now need refresh
	if !store.NeedsRefresh(peerPK) {
		t.Error("Should need refresh when below threshold")
	}

	// Test refresh
	newBundle, err := store.RefreshPreKeys(peerPK)
	if err != nil {
		t.Fatalf("Failed to refresh pre-keys: %v", err)
	}

	if newBundle.UsedCount != 0 {
		t.Error("Refreshed bundle should have 0 used keys")
	}

	if len(newBundle.Keys) != PreKeysPerPeer {
		t.Errorf("Refreshed bundle should have %d keys, got %d", PreKeysPerPeer, len(newBundle.Keys))
	}
}

func TestPreKeyExhaustion(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "prekey_exhaustion_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test key pairs
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate recipient key pair: %v", err)
	}

	// Create forward security manager
	senderFSM, err := NewForwardSecurityManager(senderKeyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create sender FSM: %v", err)
	}

	// Simulate having pre-keys for recipient
	// Note: We need more than PreKeyMinimum (5) to test exhaustion properly
	preKeys := make([]PreKeyForExchange, 8) // 8 keys total
	for i := range preKeys {
		tempKeyPair, _ := crypto.GenerateKeyPair()
		preKeys[i] = PreKeyForExchange{
			ID:        uint32(i),
			PublicKey: tempKeyPair.Public,
		}
	}

	exchange := &PreKeyExchangeMessage{
		Type:      "pre_key_exchange",
		SenderPK:  recipientKeyPair.Public,
		PreKeys:   preKeys,
		Timestamp: time.Now(),
	}

	err = senderFSM.ProcessPreKeyExchange(exchange)
	if err != nil {
		t.Fatalf("Failed to process pre-key exchange: %v", err)
	}

	// Send messages until we're below the minimum threshold
	// With 8 keys, we can send 3 messages (leaving 5, which is PreKeyMinimum)
	for i := 0; i < 3; i++ {
		if !senderFSM.CanSendMessage(recipientKeyPair.Public) {
			t.Fatalf("Should be able to send message %d", i)
		}

		_, err = senderFSM.SendForwardSecureMessage(recipientKeyPair.Public, []byte("test"), MessageTypeNormal)
		if err != nil {
			t.Fatalf("Failed to send message %d: %v", i, err)
		}
	}

	// Now we have exactly PreKeyMinimum (5) keys left
	// Next send should fail because we're at the minimum threshold
	_, err = senderFSM.SendForwardSecureMessage(recipientKeyPair.Public, []byte("test"), MessageTypeNormal)
	if err == nil {
		t.Error("Should fail to send message when at PreKeyMinimum threshold")
	}
	if err != nil && !strings.Contains(err.Error(), "insufficient pre-keys") {
		t.Errorf("Expected 'insufficient pre-keys' error, got: %v", err)
	}
}
