package async

import (
	"os"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// TestNewSignedPreKeyValid ensures a freshly generated SignedPreKey passes
// its own Verify() call.
func TestNewSignedPreKeyValid(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	spk, err := NewSignedPreKey(1, kp.Private)
	if err != nil {
		t.Fatalf("NewSignedPreKey: %v", err)
	}
	if err := spk.Verify(); err != nil {
		t.Fatalf("Verify failed for valid SPK: %v", err)
	}
}

// TestSignedPreKeyVerifyRejectsModifiedPublicKey checks that modifying the
// public key after signing causes Verify to fail.
func TestSignedPreKeyVerifyRejectsModifiedPublicKey(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	spk, err := NewSignedPreKey(2, kp.Private)
	if err != nil {
		t.Fatalf("NewSignedPreKey: %v", err)
	}
	// Corrupt one byte of the public key.
	spk.PublicKey[0] ^= 0xFF
	if err := spk.Verify(); err == nil {
		t.Fatal("Verify should fail after PublicKey is modified, but succeeded")
	}
}

// TestSignedPreKeyShouldRotateFalseWhenFresh verifies that a newly created
// SignedPreKey does not yet require rotation.
func TestSignedPreKeyShouldRotateFalseWhenFresh(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	spk, err := NewSignedPreKey(3, kp.Private)
	if err != nil {
		t.Fatalf("NewSignedPreKey: %v", err)
	}
	if spk.ShouldRotate() {
		t.Fatal("ShouldRotate returned true for a freshly created SPK")
	}
}

// TestSignedPreKeyShouldRotateTrueAfterExpiry verifies that ShouldRotate
// returns true once ExpiresAt is in the past.
func TestSignedPreKeyShouldRotateTrueAfterExpiry(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	spk, err := NewSignedPreKey(4, kp.Private)
	if err != nil {
		t.Fatalf("NewSignedPreKey: %v", err)
	}
	// Force expiry into the past.
	spk.ExpiresAt = time.Now().Add(-time.Second)
	if !spk.ShouldRotate() {
		t.Fatal("ShouldRotate returned false for an expired SPK")
	}
}

// TestProcessPreKeyExchangeRejectsInvalidSPKSignature ensures that a
// PreKeyExchangeMessage carrying a SignedPreKey with a bad signature is
// rejected by ProcessPreKeyExchange.
func TestProcessPreKeyExchangeRejectsInvalidSPKSignature(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "spk-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Alice's forward-security manager.
	aliceKP, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate alice key: %v", err)
	}
	mockT := NewMockTransport("127.0.0.1:33445")
	alice, err := NewAsyncManager(aliceKP, mockT, tmpDir)
	if err != nil {
		t.Fatalf("NewAsyncManager: %v", err)
	}

	// Build a legitimate SPK for Bob.
	bobKP, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate bob key: %v", err)
	}
	spk, err := NewSignedPreKey(1, bobKP.Private)
	if err != nil {
		t.Fatalf("NewSignedPreKey: %v", err)
	}
	// Corrupt the signature.
	spk.Signature[0] ^= 0xFF

	exchange := &PreKeyExchangeMessage{
		Type:         "pre_key_exchange",
		SenderPK:     bobKP.Public,
		PreKeys:      []PreKeyForExchange{{ID: 1, PublicKey: bobKP.Public}},
		SignedPreKey: spk,
		Timestamp:    time.Now(),
	}

	// ProcessPreKeyExchange must return a non-nil error.
	alice.mutex.RLock()
	err = alice.forwardSecurity.ProcessPreKeyExchange(exchange)
	alice.mutex.RUnlock()

	if err == nil {
		t.Fatal("ProcessPreKeyExchange should have rejected the invalid SPK signature")
	}
}

// TestProcessPreKeyExchangeAcceptsValidSPK ensures a message with a correctly
// signed SPK is accepted.
func TestProcessPreKeyExchangeAcceptsValidSPK(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "spk-accept-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	aliceKP, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate alice key: %v", err)
	}
	mockT := NewMockTransport("127.0.0.1:33445")
	alice, err := NewAsyncManager(aliceKP, mockT, tmpDir)
	if err != nil {
		t.Fatalf("NewAsyncManager: %v", err)
	}

	bobKP, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate bob key: %v", err)
	}
	spk, err := NewSignedPreKey(1, bobKP.Private)
	if err != nil {
		t.Fatalf("NewSignedPreKey: %v", err)
	}

	exchange := &PreKeyExchangeMessage{
		Type:         "pre_key_exchange",
		SenderPK:     bobKP.Public,
		PreKeys:      []PreKeyForExchange{{ID: 1, PublicKey: bobKP.Public}},
		SignedPreKey: spk,
		Timestamp:    time.Now(),
	}

	alice.mutex.RLock()
	err = alice.forwardSecurity.ProcessPreKeyExchange(exchange)
	alice.mutex.RUnlock()

	if err != nil {
		t.Fatalf("ProcessPreKeyExchange rejected a valid SPK: %v", err)
	}
}

// TestExchangePreKeysIncludesSignedPreKey verifies that ExchangePreKeys
// attaches a non-nil SignedPreKey to the returned message.
func TestExchangePreKeysIncludesSignedPreKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "spk-exchange-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	aliceKP, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate alice key: %v", err)
	}
	mockT := NewMockTransport("127.0.0.1:33445")
	alice, err := NewAsyncManager(aliceKP, mockT, tmpDir)
	if err != nil {
		t.Fatalf("NewAsyncManager: %v", err)
	}

	bobKP, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate bob key: %v", err)
	}

	alice.mutex.RLock()
	msg, err := alice.forwardSecurity.ExchangePreKeys(bobKP.Public)
	alice.mutex.RUnlock()

	if err != nil {
		t.Fatalf("ExchangePreKeys: %v", err)
	}
	if msg.SignedPreKey == nil {
		t.Fatal("ExchangePreKeys returned a message without a SignedPreKey")
	}
	if err := msg.SignedPreKey.Verify(); err != nil {
		t.Fatalf("SignedPreKey from ExchangePreKeys failed Verify: %v", err)
	}
}
