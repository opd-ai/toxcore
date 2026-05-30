package ratchet

import (
	"bytes"
	"testing"
)

// initPair creates a matched Alice/Bob session pair sharing sharedKey.
func initPair(t *testing.T) (*Session, *Session) {
	t.Helper()
	bobKP, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	var sharedKey [32]byte
	copy(sharedKey[:], "test-shared-key-must-be-32-bytes")

	alice, err := InitInitiator(sharedKey, bobKP.Public)
	if err != nil {
		t.Fatalf("InitInitiator: %v", err)
	}
	bob := InitRecipient(sharedKey, bobKP)
	return alice, bob
}

// TestBasicRoundTrip verifies a single encrypt/decrypt cycle.
func TestBasicRoundTrip(t *testing.T) {
	t.Parallel()
	alice, bob := initPair(t)

	plaintext := []byte("hello double ratchet")
	ad := []byte("associated data")

	h, ct, err := alice.RatchetEncrypt(plaintext, ad)
	if err != nil {
		t.Fatalf("RatchetEncrypt: %v", err)
	}

	got, err := bob.RatchetDecrypt(h, ct, ad)
	if err != nil {
		t.Fatalf("RatchetDecrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext mismatch: got %q want %q", got, plaintext)
	}
}

func TestRecipientCannotEncryptBeforeRatchetStep(t *testing.T) {
	t.Parallel()

	bobKP, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	var sharedKey [32]byte
	copy(sharedKey[:], "test-shared-key-must-be-32-bytes")

	bob := InitRecipient(sharedKey, bobKP)
	if _, _, err := bob.RatchetEncrypt([]byte("premature"), nil); err == nil {
		t.Fatal("expected error when recipient encrypts before first DH ratchet step")
	}
}

// TestForwardSecrecy verifies that encrypting message N does not expose N-1 or N+1.
// After decryption the message key is deleted; reuse of the same ciphertext
// must fail because the key is gone and we cannot re-derive it.
func TestForwardSecrecy(t *testing.T) {
	t.Parallel()
	alice, bob := initPair(t)

	ad := []byte("fs-test")

	// Send three messages; capture the second ciphertext and header.
	const total = 3
	headers := make([]Header, total)
	ciphertexts := make([][]byte, total)
	for i := range total {
		h, ct, err := alice.RatchetEncrypt([]byte("msg"), ad)
		if err != nil {
			t.Fatalf("encrypt %d: %v", i, err)
		}
		headers[i] = h
		ciphertexts[i] = ct
	}

	// Decrypt all in order.
	for i := range total {
		if _, err := bob.RatchetDecrypt(headers[i], ciphertexts[i], ad); err != nil {
			t.Fatalf("decrypt %d: %v", i, err)
		}
	}

	// Re-decrypting message 1 must fail: its key was deleted after first use.
	_, err := bob.RatchetDecrypt(headers[1], ciphertexts[1], ad)
	if err == nil {
		t.Fatal("expected error on key reuse, got nil")
	}
}

// TestOutOfOrderDelivery verifies that messages received out of order decrypt
// correctly via the skipped-key store.
func TestOutOfOrderDelivery(t *testing.T) {
	t.Parallel()
	alice, bob := initPair(t)
	ad := []byte("ood")

	h1, ct1, _ := alice.RatchetEncrypt([]byte("first"), ad)
	h2, ct2, _ := alice.RatchetEncrypt([]byte("second"), ad)
	h3, ct3, _ := alice.RatchetEncrypt([]byte("third"), ad)

	// Deliver in reverse order.
	for _, tc := range []struct {
		h    Header
		ct   []byte
		want string
	}{
		{h3, ct3, "third"},
		{h1, ct1, "first"},
		{h2, ct2, "second"},
	} {
		got, err := bob.RatchetDecrypt(tc.h, tc.ct, ad)
		if err != nil {
			t.Fatalf("RatchetDecrypt(%q): %v", tc.want, err)
		}
		if !bytes.Equal(got, []byte(tc.want)) {
			t.Fatalf("want %q got %q", tc.want, got)
		}
	}
}

// TestDHRatchetStep verifies that after a DH ratchet step, prior cipher states
// cannot be recomputed: a message encrypted before a ratchet step cannot be
// decrypted using the new session state.
func TestDHRatchetStep(t *testing.T) {
	t.Parallel()
	alice, bob := initPair(t)
	ad := []byte("dhr")

	// Alice → Bob (epoch 0)
	h0, ct0, _ := alice.RatchetEncrypt([]byte("before-ratchet"), ad)

	// Bob decrypts; this triggers the first DH ratchet step on Bob's side.
	got, err := bob.RatchetDecrypt(h0, ct0, ad)
	if err != nil {
		t.Fatalf("decrypt epoch-0 msg: %v", err)
	}
	if !bytes.Equal(got, []byte("before-ratchet")) {
		t.Fatalf("unexpected plaintext: %s", got)
	}

	// Bob → Alice (epoch 1, new DH key)
	h1, ct1, err := bob.RatchetEncrypt([]byte("after-ratchet"), ad)
	if err != nil {
		t.Fatalf("bob encrypt: %v", err)
	}

	got2, err := alice.RatchetDecrypt(h1, ct1, ad)
	if err != nil {
		t.Fatalf("alice decrypt: %v", err)
	}
	if !bytes.Equal(got2, []byte("after-ratchet")) {
		t.Fatalf("unexpected plaintext: %s", got2)
	}

	// Replaying epoch-0 ciphertext must fail: the key was deleted.
	_, err = bob.RatchetDecrypt(h0, ct0, ad)
	if err == nil {
		t.Fatal("expected error replaying epoch-0 ciphertext; got nil")
	}
}

// TestSkippedKeyLimit verifies that an attempt to skip more than MaxSkippedKeys
// messages returns an error rather than OOMing.
func TestSkippedKeyLimit(t *testing.T) {
	t.Parallel()
	alice, bob := initPair(t)
	ad := []byte("limit")

	// Encrypt MaxSkippedKeys+2 messages.
	const n = MaxSkippedKeys + 2
	headers := make([]Header, n)
	ciphertexts := make([][]byte, n)
	for i := range n {
		h, ct, err := alice.RatchetEncrypt([]byte("x"), ad)
		if err != nil {
			t.Fatalf("encrypt %d: %v", i, err)
		}
		headers[i] = h
		ciphertexts[i] = ct
	}

	// Delivering the last message forces Bob to skip MaxSkippedKeys+1 keys.
	_, err := bob.RatchetDecrypt(headers[n-1], ciphertexts[n-1], ad)
	if err == nil {
		t.Fatal("expected error for over-limit key skip; got nil")
	}
}

// TestHeaderEncodeDecodeRoundTrip verifies Header serialisation.
func TestHeaderEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()
	var pub [32]byte
	copy(pub[:], "01234567890123456789012345678901")
	h := Header{DHPub: pub, PN: 17, N: 42}

	enc := h.Encode()
	if len(enc) != HeaderSize {
		t.Fatalf("encoded length %d want %d", len(enc), HeaderSize)
	}

	got, err := DecodeHeader(enc)
	if err != nil {
		t.Fatalf("DecodeHeader: %v", err)
	}
	if got != h {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, h)
	}
}

// TestADMismatchFails verifies that decryption fails when associated data
// does not match what was used during encryption.
func TestADMismatchFails(t *testing.T) {
	t.Parallel()
	alice, bob := initPair(t)

	h, ct, err := alice.RatchetEncrypt([]byte("secret"), []byte("real-ad"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = bob.RatchetDecrypt(h, ct, []byte("wrong-ad"))
	if err == nil {
		t.Fatal("expected authentication failure with wrong AD; got nil")
	}
}
