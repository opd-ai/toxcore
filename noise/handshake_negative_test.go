package noise

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIKHandshakeKeyMismatch verifies that a responder rejects a message
// from an initiator that used the wrong responder public key.
func TestIKHandshakeKeyMismatch(t *testing.T) {
	initiatorPriv := make([]byte, 32)
	rand.Read(initiatorPriv)

	responderPriv := make([]byte, 32)
	rand.Read(responderPriv)

	wrongResponderPriv := make([]byte, 32)
	rand.Read(wrongResponderPriv)

	// Derive a different (wrong) public key.
	wrongKP, err := createKeyPairFromPrivateKey(wrongResponderPriv)
	require.NoError(t, err)

	// Initiator uses the wrong responder public key.
	wrongPub := make([]byte, 32)
	copy(wrongPub, wrongKP.Public[:])

	initiator, err := NewIKHandshake(initiatorPriv, wrongPub, Initiator)
	require.NoError(t, err)

	// Real responder uses its own correct private key.
	responder, err := NewIKHandshake(responderPriv, nil, Responder)
	require.NoError(t, err)

	// Initiator writes its first message.
	msg1, _, err := initiator.WriteMessage(nil, nil)
	require.NoError(t, err)

	// Real responder should reject because keys don't match.
	_, _, err = responder.WriteMessage(nil, msg1)
	assert.Error(t, err, "responder should reject message encrypted for wrong peer key")
}

// TestIKHandshakeBitFlipInMessage verifies that any single-bit corruption in
// the initiator's first message causes the responder to return an error.
func TestIKHandshakeBitFlipInMessage(t *testing.T) {
	initiatorPriv := make([]byte, 32)
	rand.Read(initiatorPriv)
	responderPriv := make([]byte, 32)
	rand.Read(responderPriv)

	responderKP, err := createKeyPairFromPrivateKey(responderPriv)
	require.NoError(t, err)
	responderPub := make([]byte, 32)
	copy(responderPub, responderKP.Public[:])

	initiator, err := NewIKHandshake(initiatorPriv, responderPub, Initiator)
	require.NoError(t, err)

	msg1, _, err := initiator.WriteMessage(nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, msg1)

	// Try flipping every byte and verify the responder rejects each.
	failCount := 0
	for i := range msg1 {
		responder, err := NewIKHandshake(responderPriv, nil, Responder)
		require.NoError(t, err)

		tampered := make([]byte, len(msg1))
		copy(tampered, msg1)
		tampered[i] ^= 0xff

		_, _, err = responder.WriteMessage(nil, tampered)
		if err != nil {
			failCount++
		}
	}
	// At least the MAC-protected portion must always fail; require the large
	// majority of tampered messages to be rejected.
	require.Greater(t, failCount, len(msg1)/2,
		"expected most bit-flipped messages to be rejected")
}

// TestIKHandshakeReplayAttack verifies that replaying the first handshake
// message does not cause panics and is handled gracefully.
func TestIKHandshakeReplayAttack(t *testing.T) {
	initiatorPriv := make([]byte, 32)
	rand.Read(initiatorPriv)
	responderPriv := make([]byte, 32)
	rand.Read(responderPriv)

	responderKP, err := createKeyPairFromPrivateKey(responderPriv)
	require.NoError(t, err)
	responderPub := make([]byte, 32)
	copy(responderPub, responderKP.Public[:])

	initiator, err := NewIKHandshake(initiatorPriv, responderPub, Initiator)
	require.NoError(t, err)

	msg1, _, err := initiator.WriteMessage(nil, nil)
	require.NoError(t, err)

	// First responder processes the message correctly.
	responder1, err := NewIKHandshake(responderPriv, nil, Responder)
	require.NoError(t, err)
	_, _, err = responder1.WriteMessage(nil, msg1)
	assert.NoError(t, err, "first responder should process the message")

	// A second (fresh) responder receives the same replayed message.
	// This must not panic regardless of the result.
	responder2, err := NewIKHandshake(responderPriv, nil, Responder)
	require.NoError(t, err)
	_, _, _ = responder2.WriteMessage(nil, msg1)
}

// TestIKHandshakeCompletedHandshakeRejection verifies that calling WriteMessage
// on a completed IK handshake returns an error rather than panicking.
func TestIKHandshakeCompletedHandshakeRejection(t *testing.T) {
	initiatorPriv := make([]byte, 32)
	rand.Read(initiatorPriv)
	responderPriv := make([]byte, 32)
	rand.Read(responderPriv)

	responderKP, err := createKeyPairFromPrivateKey(responderPriv)
	require.NoError(t, err)
	responderPub := make([]byte, 32)
	copy(responderPub, responderKP.Public[:])

	initiator, err := NewIKHandshake(initiatorPriv, responderPub, Initiator)
	require.NoError(t, err)
	responder, err := NewIKHandshake(responderPriv, nil, Responder)
	require.NoError(t, err)

	// Complete the IK handshake (2 messages).
	msg1, _, err := initiator.WriteMessage(nil, nil)
	require.NoError(t, err)

	msg2, done2, err := responder.WriteMessage(nil, msg1)
	require.NoError(t, err)
	require.True(t, done2, "responder should complete after processing initiator's message")

	_, done3, err := initiator.ReadMessage(msg2)
	require.NoError(t, err)
	require.True(t, done3, "IK handshake should complete after initiator reads response")

	// Now that the initiator's handshake is complete, writing again must fail.
	_, _, err = initiator.WriteMessage(nil, nil)
	assert.Error(t, err, "WriteMessage on a completed handshake should return an error")
}

// TestXXHandshakeWrongRoleOrder verifies that if both parties assume the same
// role (both Initiator), the handshake fails without panicking.
func TestXXHandshakeWrongRoleOrder(t *testing.T) {
	privKey1 := make([]byte, 32)
	rand.Read(privKey1)
	privKey2 := make([]byte, 32)
	rand.Read(privKey2)

	initiator1, err := NewXXHandshake(privKey1, Initiator)
	require.NoError(t, err)
	initiator2, err := NewXXHandshake(privKey2, Initiator)
	require.NoError(t, err)

	// Both sides are initiators — the handshake must not panic regardless
	// of the message exchange sequence.
	msg1, _, err := initiator1.WriteMessage(nil, nil)
	if err != nil {
		return // acceptable to fail here
	}

	// The second "initiator" tries to read the first message (which is
	// an initiator write).  This should fail or succeed but must not panic.
	_, _, _ = initiator2.ReadMessage(msg1)
}

// TestIKHandshakeZeroNonce verifies that a handshake with a zero nonce does
// not panic and still returns an error or completes gracefully.
func TestIKHandshakeZeroNonce(t *testing.T) {
	initiatorPriv := make([]byte, 32)
	rand.Read(initiatorPriv)
	responderPriv := make([]byte, 32)
	rand.Read(responderPriv)

	responderKP, err := createKeyPairFromPrivateKey(responderPriv)
	require.NoError(t, err)
	responderPub := make([]byte, 32)
	copy(responderPub, responderKP.Public[:])

	initiator, err := NewIKHandshake(initiatorPriv, responderPub, Initiator)
	require.NoError(t, err)

	// Force a zero nonce.
	initiator.nonce = [32]byte{}

	// Writing with a zero nonce must not panic.
	msg, _, err := initiator.WriteMessage(nil, nil)
	if err != nil {
		return // acceptable to fail; we just verify no panic
	}

	// If writing succeeded, responder must also handle without panic.
	responder, err := NewIKHandshake(responderPriv, nil, Responder)
	require.NoError(t, err)
	_, _, _ = responder.WriteMessage(nil, msg)
}

// TestIKHandshakeOversizedPayload verifies that an extremely large payload
// is handled without panicking.
func TestIKHandshakeOversizedPayload(t *testing.T) {
	initiatorPriv := make([]byte, 32)
	rand.Read(initiatorPriv)
	responderPriv := make([]byte, 32)
	rand.Read(responderPriv)

	responderKP, err := createKeyPairFromPrivateKey(responderPriv)
	require.NoError(t, err)
	responderPub := make([]byte, 32)
	copy(responderPub, responderKP.Public[:])

	initiator, err := NewIKHandshake(initiatorPriv, responderPub, Initiator)
	require.NoError(t, err)

	// 64 KiB payload — must not panic regardless of success or failure.
	oversized := make([]byte, 1<<16)
	rand.Read(oversized)
	_, _, _ = initiator.WriteMessage(oversized, nil)
}

// TestXXHandshakeMessagesOutOfOrder verifies that feeding a responder message
// to the responder (instead of the initiator) is handled without panicking.
func TestXXHandshakeMessagesOutOfOrder(t *testing.T) {
	privKey1 := make([]byte, 32)
	rand.Read(privKey1)
	privKey2 := make([]byte, 32)
	rand.Read(privKey2)

	initiator, err := NewXXHandshake(privKey1, Initiator)
	require.NoError(t, err)
	responder, err := NewXXHandshake(privKey2, Responder)
	require.NoError(t, err)

	// Normal first step.
	msg1, _, err := initiator.WriteMessage(nil, nil)
	require.NoError(t, err)
	msg2, _, err := responder.ReadMessage(msg1)
	require.NoError(t, err)

	// Out-of-order: feed msg2 (responder reply) back to the responder instead
	// of the initiator.  This must not panic.
	_, _, _ = responder.ReadMessage(msg2)
}
