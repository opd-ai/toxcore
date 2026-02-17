package async

import (
	"os"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// TestPreKeyExchangeSignatureVerification verifies that pre-key exchange packets
// are properly authenticated using Ed25519 signatures.
//
// Related to AUDIT.md Issue: Pre-Key HMAC Provides No Authentication
// This test validates that the migration from HMAC to Ed25519 signatures
// provides cryptographic authentication preventing spoofing attacks.
func TestPreKeyExchangeSignatureVerification(t *testing.T) {
	// Create temp directories
	aliceDir, err := os.MkdirTemp("", "alice_sig_test")
	if err != nil {
		t.Fatal("Failed to create Alice's temp dir:", err)
	}
	defer os.RemoveAll(aliceDir)

	bobDir, err := os.MkdirTemp("", "bob_sig_test")
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

	// Bob creates a valid pre-key exchange message
	bobPreKeys := []PreKeyForExchange{
		{ID: 1, PublicKey: bobKeys.Public},
		{ID: 2, PublicKey: bobKeys.Public},
	}
	exchange := &PreKeyExchangeMessage{
		SenderPK: bobKeys.Public,
		PreKeys:  bobPreKeys,
	}

	// Bob creates and signs the packet
	packet, err := bobManager.createPreKeyExchangePacket(exchange)
	if err != nil {
		t.Fatal("Failed to create pre-key exchange packet:", err)
	}

	// Verify packet has the correct size with Ed25519 signature (64 bytes) and Ed25519 PK (32 bytes)
	expectedSize := 4 + 1 + 32 + 32 + 2 + (2 * 32) + crypto.SignatureSize
	if len(packet) != expectedSize {
		t.Errorf("Packet size incorrect: got %d, want %d", len(packet), expectedSize)
	}

	preKeyPacket := &transport.Packet{
		PacketType: transport.PacketAsyncPreKeyExchange,
		Data:       packet,
	}

	// Alice receives and verifies the packet
	aliceManager.handlePreKeyExchangePacket(preKeyPacket, bobAddr)

	// Verify that Alice accepted Bob's pre-keys (signature was valid)
	aliceManager.mutex.RLock()
	acceptedPreKeys, hasKeys := aliceManager.forwardSecurity.peerPreKeys[bobKeys.Public]
	aliceManager.mutex.RUnlock()

	if !hasKeys || len(acceptedPreKeys) != 2 {
		t.Error("Alice did not accept Bob's pre-keys despite valid signature")
	}

	t.Log("Successfully verified Ed25519 signature authentication")
}

// TestPreKeyExchangeRejectInvalidSignature verifies that packets with invalid
// signatures are rejected, preventing spoofing attacks.
func TestPreKeyExchangeRejectInvalidSignature(t *testing.T) {
	// Create temp directories
	aliceDir, err := os.MkdirTemp("", "alice_invalid_sig_test")
	if err != nil {
		t.Fatal("Failed to create Alice's temp dir:", err)
	}
	defer os.RemoveAll(aliceDir)

	bobDir, err := os.MkdirTemp("", "bob_invalid_sig_test")
	if err != nil {
		t.Fatal("Failed to create Bob's temp dir:", err)
	}
	defer os.RemoveAll(bobDir)

	attackerDir, err := os.MkdirTemp("", "attacker_invalid_sig_test")
	if err != nil {
		t.Fatal("Failed to create attacker's temp dir:", err)
	}
	defer os.RemoveAll(attackerDir)

	// Create Alice (victim), Bob (legitimate sender), and Attacker
	aliceKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal("Failed to generate Alice's keys:", err)
	}

	bobKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal("Failed to generate Bob's keys:", err)
	}

	attackerKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal("Failed to generate attacker's keys:", err)
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
	_ = bobManager // Bob is a known friend but we don't need his manager for this test

	attackerManager, err := NewAsyncManager(attackerKeys, mockTransport, attackerDir)
	if err != nil {
		t.Fatal("Failed to create attacker's manager:", err)
	}

	// Alice registers Bob as a known friend
	bobAddr := &MockAddr{network: "mock", address: "bob.node:33445"}
	aliceManager.SetFriendAddress(bobKeys.Public, bobAddr)

	// Attacker creates a pre-key exchange message claiming to be Bob
	attackerPreKeys := []PreKeyForExchange{
		{ID: 1, PublicKey: attackerKeys.Public},
		{ID: 2, PublicKey: attackerKeys.Public},
	}
	spoofedExchange := &PreKeyExchangeMessage{
		SenderPK: bobKeys.Public, // Attacker claims to be Bob
		PreKeys:  attackerPreKeys,
	}

	// Attacker creates packet - this will be signed with attacker's key
	attackerPacket, err := attackerManager.createPreKeyExchangePacket(spoofedExchange)
	if err != nil {
		t.Fatal("Failed to create attacker's packet:", err)
	}

	// Manually modify the packet to claim it's from Bob (replace sender PK)
	// The signature will be invalid because it was created by attacker
	copy(attackerPacket[5:37], bobKeys.Public[:])

	preKeyPacket := &transport.Packet{
		PacketType: transport.PacketAsyncPreKeyExchange,
		Data:       attackerPacket,
	}

	// Alice receives the spoofed packet
	attackerAddr := &MockAddr{network: "mock", address: "attacker.node:33445"}
	aliceManager.handlePreKeyExchangePacket(preKeyPacket, attackerAddr)

	// Verify that Alice did NOT accept the attacker's pre-keys
	// The signature verification should fail because the packet was signed
	// with attacker's key but claims to be from Bob
	aliceManager.mutex.RLock()
	acceptedPreKeys, hasKeys := aliceManager.forwardSecurity.peerPreKeys[bobKeys.Public]
	aliceManager.mutex.RUnlock()

	if hasKeys && len(acceptedPreKeys) > 0 {
		t.Error("Alice accepted pre-keys with invalid signature - SECURITY VULNERABILITY!")
		t.Errorf("Accepted %d pre-keys from spoofed packet", len(acceptedPreKeys))
	}

	t.Log("Successfully rejected packet with invalid signature")
}

// TestPreKeyExchangeRejectTamperedPayload verifies that tampering with the
// packet payload invalidates the signature.
func TestPreKeyExchangeRejectTamperedPayload(t *testing.T) {
	// Create temp directories
	aliceDir, err := os.MkdirTemp("", "alice_tamper_test")
	if err != nil {
		t.Fatal("Failed to create Alice's temp dir:", err)
	}
	defer os.RemoveAll(aliceDir)

	bobDir, err := os.MkdirTemp("", "bob_tamper_test")
	if err != nil {
		t.Fatal("Failed to create Bob's temp dir:", err)
	}
	defer os.RemoveAll(bobDir)

	// Create Alice and Bob
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

	// Bob creates a valid pre-key exchange message
	bobPreKeys := []PreKeyForExchange{
		{ID: 1, PublicKey: bobKeys.Public},
	}
	exchange := &PreKeyExchangeMessage{
		SenderPK: bobKeys.Public,
		PreKeys:  bobPreKeys,
	}

	// Bob creates and signs the packet
	packet, err := bobManager.createPreKeyExchangePacket(exchange)
	if err != nil {
		t.Fatal("Failed to create pre-key exchange packet:", err)
	}

	// Tamper with the payload (flip a bit in the pre-key data)
	// The signature should become invalid
	packet[50] ^= 0x01 // Flip one bit

	preKeyPacket := &transport.Packet{
		PacketType: transport.PacketAsyncPreKeyExchange,
		Data:       packet,
	}

	// Alice receives the tampered packet
	aliceManager.handlePreKeyExchangePacket(preKeyPacket, bobAddr)

	// Verify that Alice did NOT accept the pre-keys
	aliceManager.mutex.RLock()
	acceptedPreKeys, hasKeys := aliceManager.forwardSecurity.peerPreKeys[bobKeys.Public]
	aliceManager.mutex.RUnlock()

	if hasKeys && len(acceptedPreKeys) > 0 {
		t.Error("Alice accepted tampered pre-keys - integrity check failed!")
	}

	t.Log("Successfully rejected tampered packet")
}

// TestPreKeyExchangeSignatureSize verifies that the packet uses Ed25519
// signatures (64 bytes) instead of HMAC (32 bytes).
func TestPreKeyExchangeSignatureSize(t *testing.T) {
	bobDir, err := os.MkdirTemp("", "bob_sigsize_test")
	if err != nil {
		t.Fatal("Failed to create Bob's temp dir:", err)
	}
	defer os.RemoveAll(bobDir)

	bobKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal("Failed to generate Bob's keys:", err)
	}

	mockTransport := NewMockTransport("127.0.0.1:33445")
	bobManager, err := NewAsyncManager(bobKeys, mockTransport, bobDir)
	if err != nil {
		t.Fatal("Failed to create Bob's manager:", err)
	}

	// Create a pre-key exchange message with 3 keys
	bobPreKeys := []PreKeyForExchange{
		{ID: 1, PublicKey: bobKeys.Public},
		{ID: 2, PublicKey: bobKeys.Public},
		{ID: 3, PublicKey: bobKeys.Public},
	}
	exchange := &PreKeyExchangeMessage{
		SenderPK: bobKeys.Public,
		PreKeys:  bobPreKeys,
	}

	packet, err := bobManager.createPreKeyExchangePacket(exchange)
	if err != nil {
		t.Fatal("Failed to create pre-key exchange packet:", err)
	}

	// Expected size: magic(4) + version(1) + sender_pk(32) + ed25519_pk(32) + count(2) + keys(3*32) + signature(64)
	expectedSize := 4 + 1 + 32 + 32 + 2 + (3 * 32) + crypto.SignatureSize
	if len(packet) != expectedSize {
		t.Errorf("Packet size incorrect: got %d, want %d", len(packet), expectedSize)
	}

	// Verify it's using Ed25519 signature size (64 bytes)
	if crypto.SignatureSize != 64 {
		t.Errorf("Ed25519 signature size should be 64 bytes, got %d", crypto.SignatureSize)
	}

	t.Logf("Packet correctly uses Ed25519 signature (%d bytes)", crypto.SignatureSize)
}
