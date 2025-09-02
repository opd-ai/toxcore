package noise

import (
	"crypto/rand"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// Test basic handshake creation
func TestNewIKHandshake(t *testing.T) {
	// Generate test keys
	staticKey1 := make([]byte, 32)
	staticKey2 := make([]byte, 32)
	_, err := rand.Read(staticKey1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = rand.Read(staticKey2)
	if err != nil {
		t.Fatal(err)
	}

	// Test initiator creation
	initiator, err := NewIKHandshake(staticKey1, staticKey2, Initiator)
	if err != nil {
		t.Fatalf("Failed to create initiator: %v", err)
	}
	if initiator.role != Initiator {
		t.Error("Expected initiator role")
	}
	if initiator.IsComplete() {
		t.Error("Handshake should not be complete initially")
	}

	// Test responder creation
	responder, err := NewIKHandshake(staticKey2, nil, Responder)
	if err != nil {
		t.Fatalf("Failed to create responder: %v", err)
	}
	if responder.role != Responder {
		t.Error("Expected responder role")
	}
	if responder.IsComplete() {
		t.Error("Handshake should not be complete initially")
	}
}

// Test input validation
func TestNewIKHandshakeValidation(t *testing.T) {
	validKey := make([]byte, 32)
	_, err := rand.Read(validKey) // Fill with random data instead of zeros
	if err != nil {
		t.Fatal(err)
	}
	invalidKey := make([]byte, 16) // Wrong size

	// Test invalid static key size
	_, err = NewIKHandshake(invalidKey, validKey, Initiator)
	if err == nil {
		t.Error("Expected error for invalid static key size")
	}

	// Test initiator without peer key
	_, err = NewIKHandshake(validKey, nil, Initiator)
	if err == nil {
		t.Error("Expected error for initiator without peer key")
	}

	// Test initiator with invalid peer key size
	_, err = NewIKHandshake(validKey, invalidKey, Initiator)
	if err == nil {
		t.Error("Expected error for invalid peer key size")
	}

	// Test responder without peer key (should succeed)
	_, err = NewIKHandshake(validKey, nil, Responder)
	if err != nil {
		t.Errorf("Unexpected error for responder without peer key: %v", err)
	}
}

// Test complete IK handshake flow
func TestIKHandshakeFlow(t *testing.T) {
	// Generate test keys
	initiatorKey := make([]byte, 32)
	responderKey := make([]byte, 32)
	_, err := rand.Read(initiatorKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = rand.Read(responderKey)
	if err != nil {
		t.Fatal(err)
	}

	// Derive responder's public key from private key (this is what initiator needs to know)
	var responderPrivArray [32]byte
	copy(responderPrivArray[:], responderKey)
	responderKeyPair, err := crypto.FromSecretKey(responderPrivArray)
	if err != nil {
		t.Fatalf("Failed to derive responder key pair: %v", err)
	}

	// Create initiator (knows responder's public key)
	initiator, err := NewIKHandshake(initiatorKey, responderKeyPair.Public[:], Initiator)
	if err != nil {
		t.Fatalf("Failed to create initiator: %v", err)
	}

	// Create responder
	responder, err := NewIKHandshake(responderKey, nil, Responder)
	if err != nil {
		t.Fatalf("Failed to create responder: %v", err)
	}

	// Step 1: Initiator creates first message
	payload1 := []byte("Hello from initiator")
	msg1, complete1, err := initiator.WriteMessage(payload1, nil)
	if err != nil {
		t.Fatalf("Initiator WriteMessage failed: %v", err)
	}
	if len(msg1) == 0 {
		t.Error("Expected non-empty message from initiator")
	}

	// Note: In IK pattern, initiator might complete after first message
	// depending on the implementation details

	// Step 2: Responder processes message and creates response
	payload2 := []byte("Hello from responder")
	msg2, complete2, err := responder.WriteMessage(payload2, msg1)
	if err != nil {
		t.Fatalf("Responder WriteMessage failed: %v", err)
	}
	if len(msg2) == 0 {
		t.Error("Expected non-empty response from responder")
	}
	if !complete2 {
		t.Error("Responder should complete after response in IK pattern")
	}

	// Step 3: If initiator is not complete, process responder's response
	if !complete1 {
		_, complete3, err := initiator.ReadMessage(msg2)
		if err != nil {
			t.Fatalf("Initiator ReadMessage failed: %v", err)
		}
		if !complete3 {
			t.Error("Initiator should complete after reading response")
		}
	}

	// Verify both parties completed handshake
	if !initiator.IsComplete() {
		t.Error("Initiator handshake should be complete")
	}
	if !responder.IsComplete() {
		t.Error("Responder handshake should be complete")
	}

	// Test cipher state availability
	sendCipher1, recvCipher1, err := initiator.GetCipherStates()
	if err != nil {
		t.Fatalf("Failed to get initiator cipher states: %v", err)
	}
	if sendCipher1 == nil || recvCipher1 == nil {
		t.Error("Initiator cipher states should not be nil")
	}

	sendCipher2, recvCipher2, err := responder.GetCipherStates()
	if err != nil {
		t.Fatalf("Failed to get responder cipher states: %v", err)
	}
	if sendCipher2 == nil || recvCipher2 == nil {
		t.Error("Responder cipher states should not be nil")
	}
}

// Test error cases for completed handshakes
func TestHandshakeCompleteErrors(t *testing.T) {
	// Create and complete a simple handshake
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	_, err := rand.Read(key1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = rand.Read(key2)
	if err != nil {
		t.Fatal(err)
	}

	// Derive key2's public key for initiator
	var key2Array [32]byte
	copy(key2Array[:], key2)
	key2Pair, err := crypto.FromSecretKey(key2Array)
	if err != nil {
		t.Fatal(err)
	}

	initiator, err := NewIKHandshake(key1, key2Pair.Public[:], Initiator)
	if err != nil {
		t.Fatal(err)
	}

	responder, err := NewIKHandshake(key2, nil, Responder)
	if err != nil {
		t.Fatal(err)
	}

	// Complete handshake
	msg1, _, err := initiator.WriteMessage([]byte("test"), nil)
	if err != nil {
		t.Fatal(err)
	}

	msg2, _, err := responder.WriteMessage([]byte("response"), msg1)
	if err != nil {
		t.Fatal(err)
	}

	// Initiator reads responder's response to complete handshake
	_, _, err = initiator.ReadMessage(msg2)
	if err != nil {
		t.Fatal(err)
	}

	// Test operations on completed handshake
	_, _, err = initiator.WriteMessage([]byte("again"), nil)
	if err != ErrHandshakeComplete {
		t.Errorf("Expected ErrHandshakeComplete, got %v", err)
	}

	_, _, err = responder.WriteMessage([]byte("again"), nil)
	if err != ErrHandshakeComplete {
		t.Errorf("Expected ErrHandshakeComplete, got %v", err)
	}
}

// Test operations on incomplete handshakes
func TestHandshakeIncompleteErrors(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatal(err)
	}

	handshake, err := NewIKHandshake(key, nil, Responder)
	if err != nil {
		t.Fatal(err)
	}

	// Test getting cipher states before completion
	_, _, err = handshake.GetCipherStates()
	if err != ErrHandshakeNotComplete {
		t.Errorf("Expected ErrHandshakeNotComplete, got %v", err)
	}

	// Test getting remote static key before completion
	_, err = handshake.GetRemoteStaticKey()
	if err != ErrHandshakeNotComplete {
		t.Errorf("Expected ErrHandshakeNotComplete, got %v", err)
	}
}

// Test responder ReadMessage error
func TestResponderReadMessageError(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatal(err)
	}

	responder, err := NewIKHandshake(key, nil, Responder)
	if err != nil {
		t.Fatal(err)
	}

	// Responder should not be able to call ReadMessage
	_, _, err = responder.ReadMessage([]byte("test"))
	if err == nil {
		t.Error("Expected error when responder calls ReadMessage")
	}
}

// Benchmark handshake creation
func BenchmarkNewIKHandshake(b *testing.B) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewIKHandshake(key1, key2, Initiator)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark complete handshake flow
func BenchmarkIKHandshakeFlow(b *testing.B) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)

	// Derive key2's public key for initiator
	var key2Array [32]byte
	copy(key2Array[:], key2)
	key2Pair, err := crypto.FromSecretKey(key2Array)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		initiator, err := NewIKHandshake(key1, key2Pair.Public[:], Initiator)
		if err != nil {
			b.Fatal(err)
		}

		responder, err := NewIKHandshake(key2, nil, Responder)
		if err != nil {
			b.Fatal(err)
		}

		msg1, _, err := initiator.WriteMessage([]byte("test"), nil)
		if err != nil {
			b.Fatal(err)
		}

		msg2, _, err := responder.WriteMessage([]byte("response"), msg1)
		if err != nil {
			b.Fatal(err)
		}

		_, _, err = initiator.ReadMessage(msg2)
		if err != nil {
			b.Fatal(err)
		}
	}
}
