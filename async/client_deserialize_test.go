package async

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/limits"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeserializeRetrieveResponseOversized tests that oversized gob payloads are rejected
// M-1 remediation: prevent memory exhaustion via unbounded gob decode
func TestDeserializeRetrieveResponseOversized(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)
	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Create a gob payload that declares a very large message count
	// This would cause memory exhaustion if decoded without bounds checking
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	// Create a slice that gob will decode into a message claiming many elements
	// We use a deliberately oversized slice
	largeSlice := make([]*ObfuscatedAsyncMessage, MaxMessagesPerRecipient+1)
	for i := range largeSlice {
		largeSlice[i] = &ObfuscatedAsyncMessage{}
	}

	err = encoder.Encode(largeSlice)
	require.NoError(t, err)

	oversizedData := buf.Bytes()

	// Attempt to deserialize - should fail with count validation error
	result, err := client.deserializeRetrieveResponse(oversizedData)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

// TestDeserializeRetrieveResponseValidSize tests that valid-sized payloads are accepted
func TestDeserializeRetrieveResponseValidSize(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)
	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Create a valid gob payload with messages at the limit
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	validSlice := make([]*ObfuscatedAsyncMessage, MaxMessagesPerRecipient)
	for i := range validSlice {
		validSlice[i] = &ObfuscatedAsyncMessage{
			EncryptedPayload: []byte("test"),
		}
	}

	err = encoder.Encode(validSlice)
	require.NoError(t, err)

	validData := buf.Bytes()

	// Should succeed with valid count
	result, err := client.deserializeRetrieveResponse(validData)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, MaxMessagesPerRecipient, len(result))
}

// TestDeserializeRetrieveResponseEmpty tests that empty responses are handled correctly
func TestDeserializeRetrieveResponseEmpty(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)
	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Test empty slice
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err = encoder.Encode([]*ObfuscatedAsyncMessage{})
	require.NoError(t, err)

	emptyData := buf.Bytes()
	result, err := client.deserializeRetrieveResponse(emptyData)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, len(result))
}

// TestDeserializeRetrieveResponseProcessingBufferLimit tests that MaxProcessingBuffer is enforced
// M-1 remediation: validate input against absolute maximum before decoding
func TestDeserializeRetrieveResponseProcessingBufferLimit(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)
	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Create data that exceeds MaxProcessingBuffer
	// We'll just use raw bytes since we're testing the size check before decoding
	oversizedData := make([]byte, limits.MaxProcessingBuffer+1)

	result, err := client.deserializeRetrieveResponse(oversizedData)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "buffer exceeds maximum")
}

// TestDeserializeRetrieveResponseNilData tests nil/empty input handling
func TestDeserializeRetrieveResponseNilData(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)
	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	result, err := client.deserializeRetrieveResponse(nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "empty")

	result, err = client.deserializeRetrieveResponse([]byte{})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "empty")
}
