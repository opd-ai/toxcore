package async

import (
	"os"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// TestPreKeyExchangeRejectUnknownSender verifies that pre-key exchanges from
// unknown senders are rejected to mitigate the HMAC authentication limitation.
//
// Related to AUDIT.md Gap #6: Pre-Key Exchange HMAC Verification
// The HMAC in pre-key packets provides integrity but not authentication (sender
// uses their private key which receiver doesn't have). To prevent spam/injection,
// we only accept pre-keys from known friends.
func TestPreKeyExchangeRejectUnknownSender(t *testing.T) {
	// Create temp directories
	aliceDir, err := os.MkdirTemp("", "alice_prekey_test")
	if err != nil {
		t.Fatal("Failed to create Alice's temp dir:", err)
	}
	defer os.RemoveAll(aliceDir)

	bobDir, err := os.MkdirTemp("", "bob_prekey_test")
	if err != nil {
		t.Fatal("Failed to create Bob's temp dir:", err)
	}
	defer os.RemoveAll(bobDir)

	// Create two managers: Alice (receiver) and Bob (unknown sender)
	aliceKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal("Failed to generate Alice's keys:", err)
	}

	bobKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal("Failed to generate Bob's keys:", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:33445")
	aliceManager, err := NewAsyncManager(aliceKeys, mockTransport, aliceDir)
	if err != nil {
		t.Fatal("Failed to create Alice's manager:", err)
	}

	bobManager, err := NewAsyncManager(bobKeys, mockTransport, bobDir)
	if err != nil {
		t.Fatal("Failed to create Bob's manager:", err)
	}

	// Create a pre-key exchange message manually from Bob
	bobPreKeys := []PreKeyForExchange{
		{ID: 1, PublicKey: bobKeys.Public},
		{ID: 2, PublicKey: bobKeys.Public},
	}
	exchange := &PreKeyExchangeMessage{
		SenderPK: bobKeys.Public,
		PreKeys:  bobPreKeys,
	}

	// Create the packet
	packet, err := bobManager.createPreKeyExchangePacket(exchange)
	if err != nil {
		t.Fatal("Failed to create pre-key exchange packet:", err)
	}

	preKeyPacket := &transport.Packet{
		PacketType: transport.PacketAsyncPreKeyExchange,
		Data:       packet,
	}

	// Alice receives the packet from unknown Bob
	mockAddr := &MockAddr{network: "mock", address: "bob.node:33445"}
	aliceManager.handlePreKeyExchangePacket(preKeyPacket, mockAddr)

	// Verify that Alice did NOT accept Bob's pre-keys (he's not a known friend)
	aliceManager.mutex.RLock()
	bobPreKeys2, hasKeys := aliceManager.forwardSecurity.peerPreKeys[bobKeys.Public]
	aliceManager.mutex.RUnlock()

	if hasKeys && len(bobPreKeys2) > 0 {
		t.Error("Alice accepted pre-keys from unknown sender Bob (security vulnerability!)")
	}

	t.Log("Successfully rejected pre-keys from unknown sender")
}

// TestPreKeyExchangeAcceptKnownFriend verifies that pre-key exchanges from
// known friends ARE accepted.
func TestPreKeyExchangeAcceptKnownFriend(t *testing.T) {
	// Create temp directories
	aliceDir, err := os.MkdirTemp("", "alice_known_friend_test")
	if err != nil {
		t.Fatal("Failed to create Alice's temp dir:", err)
	}
	defer os.RemoveAll(aliceDir)

	bobDir, err := os.MkdirTemp("", "bob_known_friend_test")
	if err != nil {
		t.Fatal("Failed to create Bob's temp dir:", err)
	}
	defer os.RemoveAll(bobDir)

	// Create two managers: Alice and Bob
	aliceKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal("Failed to generate Alice's keys:", err)
	}

	bobKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal("Failed to generate Bob's keys:", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:33445")
	aliceManager, err := NewAsyncManager(aliceKeys, mockTransport, aliceDir)
	if err != nil {
		t.Fatal("Failed to create Alice's manager:", err)
	}

	bobManager, err := NewAsyncManager(bobKeys, mockTransport, bobDir)
	if err != nil {
		t.Fatal("Failed to create Bob's manager:", err)
	}

	// Alice registers Bob as a known friend
	bobAddr := &MockAddr{network: "mock", address: "bob.node:33445"}
	aliceManager.SetFriendAddress(bobKeys.Public, bobAddr)
	aliceManager.SetFriendOnlineStatus(bobKeys.Public, true)

	// Bob creates pre-key exchange message
	bobPreKeys := []PreKeyForExchange{
		{ID: 1, PublicKey: bobKeys.Public},
		{ID: 2, PublicKey: bobKeys.Public},
	}
	exchange := &PreKeyExchangeMessage{
		SenderPK: bobKeys.Public,
		PreKeys:  bobPreKeys,
	}

	// Create the packet
	packet, err := bobManager.createPreKeyExchangePacket(exchange)
	if err != nil {
		t.Fatal("Failed to create pre-key exchange packet:", err)
	}

	preKeyPacket := &transport.Packet{
		PacketType: transport.PacketAsyncPreKeyExchange,
		Data:       packet,
	}

	// Alice receives the packet from known friend Bob
	aliceManager.handlePreKeyExchangePacket(preKeyPacket, bobAddr)

	// Verify that Alice DID accept Bob's pre-keys (he's a known friend)
	aliceManager.mutex.RLock()
	bobPreKeys2, hasKeys := aliceManager.forwardSecurity.peerPreKeys[bobKeys.Public]
	aliceManager.mutex.RUnlock()

	if !hasKeys || len(bobPreKeys2) == 0 {
		t.Error("Alice rejected pre-keys from known friend Bob (should have accepted)")
	}

	t.Logf("Alice successfully accepted %d pre-keys from friend Bob", len(bobPreKeys2))
}

// TestPreKeyExchangeHMACIntegrityCheck verifies that malformed HMAC fields
// are rejected during packet parsing.
func TestPreKeyExchangeHMACIntegrityCheck(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hmac_integrity_test")
	if err != nil {
		t.Fatal("Failed to create temp dir:", err)
	}
	defer os.RemoveAll(tempDir)

	aliceKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal("Failed to generate keys:", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:33445")
	manager, err := NewAsyncManager(aliceKeys, mockTransport, tempDir)
	if err != nil {
		t.Fatal("Failed to create manager:", err)
	}

	// Create a valid pre-key exchange message
	preKeys := []PreKeyForExchange{
		{ID: 1, PublicKey: aliceKeys.Public},
	}
	exchange := &PreKeyExchangeMessage{
		SenderPK: aliceKeys.Public,
		PreKeys:  preKeys,
	}

	// Generate a valid packet
	validPacket, err := manager.createPreKeyExchangePacket(exchange)
	if err != nil {
		t.Fatal("Failed to create valid packet:", err)
	}

	// Test 1: Valid packet with correct HMAC size (should parse successfully)
	_, _, err = manager.parsePreKeyExchangePacket(validPacket)
	if err != nil {
		t.Errorf("Valid packet rejected: %v", err)
	}

	// Test 2: Packet with truncated HMAC (should be rejected)
	truncatedPacket := validPacket[:len(validPacket)-16] // Remove last 16 bytes of HMAC
	_, _, err = manager.parsePreKeyExchangePacket(truncatedPacket)
	if err == nil {
		t.Error("Truncated HMAC was accepted (should be rejected)")
	}

	// Test 3: Packet with corrupted HMAC (wrong size check)
	// Note: We can't verify HMAC authenticity without sender's private key,
	// but we can verify the structure is correct
	corruptedPacket := append([]byte{}, validPacket...)
	corruptedPacket = append(corruptedPacket, []byte{0xFF, 0xFF, 0xFF, 0xFF}...) // Add extra bytes
	_, _, err = manager.parsePreKeyExchangePacket(corruptedPacket)
	if err == nil {
		t.Error("Packet with wrong size was accepted (should be rejected)")
	}

	t.Log("HMAC integrity checks passed")
}

// TestPreKeyExchangeDocumentedLimitation documents the current HMAC limitation
// and serves as a reminder for future enhancement.
func TestPreKeyExchangeDocumentedLimitation(t *testing.T) {
	t.Log("SECURITY LIMITATION DOCUMENTATION:")
	t.Log("----------------------------------")
	t.Log("Pre-key exchange HMAC provides INTEGRITY protection (detects corruption)")
	t.Log("but NOT AUTHENTICATION (cannot verify sender without their private key).")
	t.Log("")
	t.Log("Current design: Sender signs with their private key, receiver cannot verify.")
	t.Log("")
	t.Log("Mitigation: Only accept pre-key exchanges from known friends (verified via")
	t.Log("the Tox friend system). Unknown senders are rejected in handlePreKeyExchangePacket.")
	t.Log("")
	t.Log("TODO(security): Consider future enhancements:")
	t.Log("  1. Switch to Ed25519 digital signatures for authentication")
	t.Log("  2. Use challenge-response protocol for pre-key exchange initiation")
	t.Log("  3. Derive shared secret via ECDH for mutual authentication")
	t.Log("")
	t.Log("This limitation is documented in AUDIT.md Gap #6 and async/manager.go")

	// This test always passes - it's documentation only
}

// TestPreKeySpamPrevention verifies that the friend verification prevents
// spam attacks from malicious senders.
func TestPreKeySpamPrevention(t *testing.T) {
	aliceDir, err := os.MkdirTemp("", "alice_spam_test")
	if err != nil {
		t.Fatal("Failed to create Alice's temp dir:", err)
	}
	defer os.RemoveAll(aliceDir)

	aliceKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal("Failed to generate Alice's keys:", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:33445")
	aliceManager, err := NewAsyncManager(aliceKeys, mockTransport, aliceDir)
	if err != nil {
		t.Fatal("Failed to create Alice's manager:", err)
	}

	// Simulate spam attack: 100 pre-key exchanges from unknown senders
	spamCount := 100
	for i := 0; i < spamCount; i++ {
		// Create attacker keys
		attackerKeys, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal("Failed to generate attacker keys:", err)
		}

		// Create spam pre-keys
		spamPreKeys := []PreKeyForExchange{
			{ID: uint32(i), PublicKey: attackerKeys.Public},
		}
		exchange := &PreKeyExchangeMessage{
			SenderPK: attackerKeys.Public,
			PreKeys:  spamPreKeys,
		}

		// Create a temporary manager for attacker to create packet
		attackerDir, _ := os.MkdirTemp("", "attacker_test")
		attackerManager, err := NewAsyncManager(attackerKeys, mockTransport, attackerDir)
		if err == nil {
			// Create packet
			packet, err := attackerManager.createPreKeyExchangePacket(exchange)
			if err == nil {
				preKeyPacket := &transport.Packet{
					PacketType: transport.PacketAsyncPreKeyExchange,
					Data:       packet,
				}

				// Alice receives spam packet
				attackerAddr := &MockAddr{network: "mock", address: "attacker.node:33445"}
				aliceManager.handlePreKeyExchangePacket(preKeyPacket, attackerAddr)
			}
		}
		os.RemoveAll(attackerDir)
	}

	// Verify that Alice did NOT accept any spam pre-keys
	aliceManager.mutex.RLock()
	totalAcceptedKeys := 0
	for _, keys := range aliceManager.forwardSecurity.peerPreKeys {
		totalAcceptedKeys += len(keys)
	}
	aliceManager.mutex.RUnlock()

	if totalAcceptedKeys > 0 {
		t.Errorf("Alice accepted %d pre-keys from unknown spammers (should be 0)", totalAcceptedKeys)
	}

	t.Logf("Successfully blocked %d spam pre-key exchanges", spamCount)
}
