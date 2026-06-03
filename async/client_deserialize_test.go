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

// TestDeserializeRetrieveResponseOversized tests that responses containing more messages
// than MaxMessagesPerRecipient are rejected.
// M-1 remediation: the count is validated before any message data is decoded
func TestDeserializeRetrieveResponseOversized(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)
	mockTransport := NewMockTransport("127.0.0.1:8080")
	client := NewAsyncClient(keyPair, mockTransport)

	// Serialize a slice whose element count exceeds the per-recipient limit.
	// The new count-prefixed format lets deserializeRetrieveResponse reject this
	// before allocating or decoding any message data.
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	// Encode a count that is one over the limit, then the corresponding elements
	largeSlice := make([]*ObfuscatedAsyncMessage, MaxMessagesPerRecipient+1)
	for i := range largeSlice {
		largeSlice[i] = &ObfuscatedAsyncMessage{}
	}

	// Encode using the count-prefixed format: write the over-limit count first,
	// then the individual elements.  deserializeRetrieveResponse must reject this
	// at the count-validation step, before decoding any message data.
	count := int32(len(largeSlice))
	require.NoError(t, encoder.Encode(count))
	for _, msg := range largeSlice {
		require.NoError(t, encoder.Encode(msg))
	}

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

	// Create a valid payload with messages exactly at the limit using the
	// count-prefixed format produced by serializeRetrieveResponse.
	validSlice := make([]*ObfuscatedAsyncMessage, MaxMessagesPerRecipient)
	for i := range validSlice {
		validSlice[i] = &ObfuscatedAsyncMessage{
			EncryptedPayload: []byte("test"),
		}
	}

	validData, err := client.serializeRetrieveResponse(validSlice)
	require.NoError(t, err)

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

	// Use serializeRetrieveResponse to produce a valid empty payload.
	emptyData, err := client.serializeRetrieveResponse([]*ObfuscatedAsyncMessage{})
	require.NoError(t, err)

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
