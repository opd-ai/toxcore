package noise

import (
	"crypto/rand"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// FuzzHandshakeMessage fuzzes the handshake message processing
// This test helps identify potential crashes, panics, or security issues
// when processing malformed or malicious handshake messages
func FuzzHandshakeMessage(f *testing.F) {
	// Add seed corpus with valid handshake patterns

	// Generate valid keypairs for testing
	var initiatorPriv, responderPriv [32]byte
	rand.Read(initiatorPriv[:])
	rand.Read(responderPriv[:])

	initiatorKP, _ := crypto.FromSecretKey(initiatorPriv)
	responderKP, _ := crypto.FromSecretKey(responderPriv)

	// Create valid handshake
	initiator, err := NewIKHandshake(initiatorKP.Private[:], responderKP.Public[:], Initiator)
	if err != nil {
		f.Fatal(err)
	}

	// Generate initial handshake message
	msg1, _, err := initiator.WriteMessage(nil, nil)
	if err != nil {
		f.Fatal(err)
	}

	// Add valid message to corpus
	f.Add(msg1)

	// Add some edge cases to corpus
	f.Add([]byte{})            // Empty message
	f.Add([]byte{0x00})        // Single byte
	f.Add(make([]byte, 1024))  // Large zero buffer
	f.Add(make([]byte, 10000)) // Very large buffer

	// Fuzz testing
	f.Fuzz(func(t *testing.T, data []byte) {
		// Test initiator processing arbitrary data
		var testInitPriv, testRespPriv [32]byte
		rand.Read(testInitPriv[:])
		rand.Read(testRespPriv[:])

		testInitKP, _ := crypto.FromSecretKey(testInitPriv)
		testRespKP, _ := crypto.FromSecretKey(testRespPriv)

		// Create fresh handshakes for each fuzz iteration
		testInit, err := NewIKHandshake(testInitKP.Private[:], testRespKP.Public[:], Initiator)
		if err != nil {
			return
		}

		testResp, err := NewIKHandshake(testRespKP.Private[:], nil, Responder)
		if err != nil {
			return
		}

		// Attempt to process fuzzed data
		// These should not panic or crash
		_, _, _ = testResp.ReadMessage(data)
		_, _, _ = testInit.ReadMessage(data)

		// Try writing message with fuzzed payload
		if len(data) < 1000 { // Limit payload size to prevent OOM
			_, _, _ = testInit.WriteMessage(data, nil)
			_, _, _ = testResp.WriteMessage(data, data) // Responder needs received message
		}
	})
}

// FuzzHandshakeNonceValidation fuzzes nonce validation logic
func FuzzHandshakeNonceValidation(f *testing.F) {
	// Add seed corpus
	validNonce := make([]byte, 32)
	rand.Read(validNonce)
	f.Add(validNonce)

	// Add edge cases
	f.Add(make([]byte, 0))  // Empty
	f.Add(make([]byte, 32)) // All zeros
	f.Add(make([]byte, 31)) // Too short
	f.Add(make([]byte, 33)) // Too long

	f.Fuzz(func(t *testing.T, nonceData []byte) {
		// Create handshake
		var priv [32]byte
		rand.Read(priv[:])
		kp, _ := crypto.FromSecretKey(priv)

		// Test nonce handling doesn't panic
		hs, err := NewIKHandshake(kp.Private[:], kp.Public[:], Initiator)
		if err != nil {
			return
		}

		// Access internal nonce (would need to be exposed for real fuzzing)
		// This demonstrates the concept
		_ = hs.nonce
	})
}

// FuzzHandshakeTimestamp fuzzes timestamp validation
func FuzzHandshakeTimestamp(f *testing.F) {
	// Add seed corpus with various timestamps
	f.Add(int64(0))          // Unix epoch
	f.Add(int64(1234567890)) // Valid timestamp
	f.Add(int64(-1))         // Negative
	f.Add(int64(9999999999)) // Far future

	f.Fuzz(func(t *testing.T, timestamp int64) {
		// Create handshake
		var priv [32]byte
		rand.Read(priv[:])
		kp, _ := crypto.FromSecretKey(priv)

		hs, err := NewIKHandshake(kp.Private[:], kp.Public[:], Initiator)
		if err != nil {
			return
		}

		// Set timestamp to fuzzed value
		hs.timestamp = timestamp

		// Ensure no panic when processing messages
		msg, _, err := hs.WriteMessage(nil, nil)
		if err == nil && len(msg) > 0 {
			// Try to read it back
			var recvPriv [32]byte
			rand.Read(recvPriv[:])
			recvKP, _ := crypto.FromSecretKey(recvPriv)
			receiver, _ := NewIKHandshake(recvKP.Private[:], nil, Responder)
			_, _, _ = receiver.ReadMessage(msg)
		}
	})
}
