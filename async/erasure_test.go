package async

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultErasureCodingConfig(t *testing.T) {
	config := DefaultErasureCodingConfig()

	assert.Equal(t, 3, config.DataShards, "should have 3 data shards")
	assert.Equal(t, 2, config.ParityShards, "should have 2 parity shards")
	assert.Equal(t, 3, config.MinShards, "should need 3 shards minimum")
	assert.Equal(t, 5, config.TotalShards(), "should have 5 total shards")
}

func TestNewErasureEncoder(t *testing.T) {
	tests := []struct {
		name        string
		config      *ErasureCodingConfig
		shouldError bool
	}{
		{
			name:        "nil config uses defaults",
			config:      nil,
			shouldError: false,
		},
		{
			name:        "valid default config",
			config:      DefaultErasureCodingConfig(),
			shouldError: false,
		},
		{
			name: "valid custom config",
			config: &ErasureCodingConfig{
				DataShards:   5,
				ParityShards: 3,
				MinShards:    5,
			},
			shouldError: false,
		},
		{
			name: "invalid zero data shards",
			config: &ErasureCodingConfig{
				DataShards:   0,
				ParityShards: 2,
				MinShards:    1,
			},
			shouldError: true,
		},
		{
			name: "invalid zero parity shards",
			config: &ErasureCodingConfig{
				DataShards:   3,
				ParityShards: 0,
				MinShards:    1,
			},
			shouldError: true,
		},
		{
			name: "invalid min shards greater than data",
			config: &ErasureCodingConfig{
				DataShards:   3,
				ParityShards: 2,
				MinShards:    4,
			},
			shouldError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoder, err := NewErasureEncoder(tc.config)
			if tc.shouldError {
				assert.Error(t, err)
				assert.Nil(t, encoder)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, encoder)
			}
		})
	}
}

func TestErasureEncoder_EncodeMessage(t *testing.T) {
	encoder, err := NewErasureEncoder(nil)
	require.NoError(t, err)

	t.Run("encode small message", func(t *testing.T) {
		var messageID [32]byte
		rand.Read(messageID[:])
		data := []byte("Hello, World!")

		shards, err := encoder.EncodeMessage(messageID, data)
		require.NoError(t, err)
		assert.Len(t, shards, 5, "should produce 5 shards (3 data + 2 parity)")

		dataShardCount := 0
		parityShardCount := 0
		for _, shard := range shards {
			assert.Equal(t, messageID, shard.MessageID)
			assert.Equal(t, 5, shard.TotalShards)
			assert.Equal(t, 3, shard.DataShards)
			if shard.IsParity {
				parityShardCount++
			} else {
				dataShardCount++
			}
		}
		assert.Equal(t, 3, dataShardCount, "should have 3 data shards")
		assert.Equal(t, 2, parityShardCount, "should have 2 parity shards")
	})

	t.Run("encode large message", func(t *testing.T) {
		var messageID [32]byte
		rand.Read(messageID[:])
		data := make([]byte, 10000)
		rand.Read(data)

		shards, err := encoder.EncodeMessage(messageID, data)
		require.NoError(t, err)
		assert.Len(t, shards, 5)
	})

	t.Run("encode empty message fails", func(t *testing.T) {
		var messageID [32]byte
		shards, err := encoder.EncodeMessage(messageID, []byte{})
		assert.Error(t, err)
		assert.Nil(t, shards)
	})
}

func TestErasureEncoder_DecodeShards_AllShards(t *testing.T) {
	encoder, err := NewErasureEncoder(nil)
	require.NoError(t, err)

	var messageID [32]byte
	rand.Read(messageID[:])
	originalData := []byte("Test message for erasure coding reconstruction")

	shards, err := encoder.EncodeMessage(messageID, originalData)
	require.NoError(t, err)

	reconstructed, err := encoder.DecodeShards(shards, len(originalData))
	require.NoError(t, err)
	assert.Equal(t, originalData, reconstructed)
}

func TestErasureEncoder_DecodeShards_MinimumShards(t *testing.T) {
	encoder, err := NewErasureEncoder(nil)
	require.NoError(t, err)

	var messageID [32]byte
	rand.Read(messageID[:])
	originalData := []byte("Erasure coding allows reconstruction from partial data")

	shards, err := encoder.EncodeMessage(messageID, originalData)
	require.NoError(t, err)

	// Test with only 3 shards (minimum required)
	// Simulating 2 node failures by removing 2 shards
	partialShards := make([]*EncodedShard, 5)
	partialShards[0] = shards[0] // data shard 0
	partialShards[1] = shards[1] // data shard 1
	partialShards[4] = shards[4] // parity shard 1

	reconstructed, err := encoder.DecodeShards(partialShards, len(originalData))
	require.NoError(t, err)
	assert.Equal(t, originalData, reconstructed)
}

func TestErasureEncoder_DecodeShards_InsufficientShards(t *testing.T) {
	encoder, err := NewErasureEncoder(nil)
	require.NoError(t, err)

	var messageID [32]byte
	rand.Read(messageID[:])
	originalData := []byte("Test message")

	shards, err := encoder.EncodeMessage(messageID, originalData)
	require.NoError(t, err)

	// Only 2 shards (below minimum of 3)
	insufficientShards := make([]*EncodedShard, 5)
	insufficientShards[0] = shards[0]
	insufficientShards[1] = shards[1]

	_, err = encoder.DecodeShards(insufficientShards, len(originalData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient shards")
}

func TestErasureEncoder_VerifyShards(t *testing.T) {
	encoder, err := NewErasureEncoder(nil)
	require.NoError(t, err)

	var messageID [32]byte
	rand.Read(messageID[:])
	data := []byte("Verification test data")

	shards, err := encoder.EncodeMessage(messageID, data)
	require.NoError(t, err)

	valid, err := encoder.VerifyShards(shards)
	require.NoError(t, err)
	assert.True(t, valid, "valid shards should verify")
}

func TestErasureStorage_StoreAndReconstruct(t *testing.T) {
	storage, err := NewErasureStorage(nil)
	require.NoError(t, err)

	var messageID [32]byte
	rand.Read(messageID[:])
	originalData := []byte("Storage test message for full cycle")

	shards, err := storage.StoreMessage(messageID, originalData)
	require.NoError(t, err)
	assert.Len(t, shards, 5)

	reconstructed, err := storage.ReconstructMessage(messageID)
	require.NoError(t, err)
	assert.Equal(t, originalData, reconstructed)
}

func TestErasureStorage_StoreShard(t *testing.T) {
	storage, err := NewErasureStorage(nil)
	require.NoError(t, err)

	var messageID [32]byte
	rand.Read(messageID[:])
	originalData := []byte("Partial storage test")

	// Create shards via encoder but store individually
	encoder, _ := NewErasureEncoder(nil)
	shards, err := encoder.EncodeMessage(messageID, originalData)
	require.NoError(t, err)

	// Store shards one by one (simulating retrieval from multiple nodes)
	for i, shard := range shards {
		err := storage.StoreShard(shard)
		require.NoError(t, err, "storing shard %d should succeed", i)
	}

	storage.SetOriginalLength(messageID, len(originalData))

	reconstructed, err := storage.ReconstructMessage(messageID)
	require.NoError(t, err)
	assert.Equal(t, originalData, reconstructed)
}

func TestErasureStorage_HasSufficientShards(t *testing.T) {
	storage, err := NewErasureStorage(nil)
	require.NoError(t, err)

	var messageID [32]byte
	rand.Read(messageID[:])

	assert.False(t, storage.HasSufficientShards(messageID), "no shards stored yet")

	encoder, _ := NewErasureEncoder(nil)
	shards, _ := encoder.EncodeMessage(messageID, []byte("test"))

	// Add 2 shards (insufficient)
	storage.StoreShard(shards[0])
	storage.StoreShard(shards[1])
	assert.False(t, storage.HasSufficientShards(messageID), "only 2 shards")

	// Add 3rd shard (sufficient)
	storage.StoreShard(shards[2])
	assert.True(t, storage.HasSufficientShards(messageID), "3 shards should be sufficient")
}

func TestErasureStorage_GetShardCount(t *testing.T) {
	storage, err := NewErasureStorage(nil)
	require.NoError(t, err)

	var messageID [32]byte
	rand.Read(messageID[:])

	assert.Equal(t, 0, storage.GetShardCount(messageID))

	encoder, _ := NewErasureEncoder(nil)
	shards, _ := encoder.EncodeMessage(messageID, []byte("test"))

	storage.StoreShard(shards[0])
	assert.Equal(t, 1, storage.GetShardCount(messageID))

	storage.StoreShard(shards[1])
	storage.StoreShard(shards[2])
	assert.Equal(t, 3, storage.GetShardCount(messageID))
}

func TestErasureStorage_DeleteMessage(t *testing.T) {
	storage, err := NewErasureStorage(nil)
	require.NoError(t, err)

	var messageID [32]byte
	rand.Read(messageID[:])
	storage.StoreMessage(messageID, []byte("to be deleted"))

	assert.True(t, storage.HasSufficientShards(messageID))

	storage.DeleteMessage(messageID)

	assert.False(t, storage.HasSufficientShards(messageID))
	assert.Equal(t, 0, storage.GetShardCount(messageID))
}

func TestErasureStorage_GetMissingShardIndices(t *testing.T) {
	storage, err := NewErasureStorage(nil)
	require.NoError(t, err)

	var messageID [32]byte
	rand.Read(messageID[:])

	encoder, _ := NewErasureEncoder(nil)
	shards, _ := encoder.EncodeMessage(messageID, []byte("test"))

	// Store only shards 0 and 2
	storage.StoreShard(shards[0])
	storage.StoreShard(shards[2])

	missing := storage.GetMissingShardIndices(messageID)
	assert.Contains(t, missing, 1)
	assert.Contains(t, missing, 3)
	assert.Contains(t, missing, 4)
	assert.Len(t, missing, 3)
}

func TestErasureStorage_GetStats(t *testing.T) {
	storage, err := NewErasureStorage(nil)
	require.NoError(t, err)

	var messageID1, messageID2 [32]byte
	rand.Read(messageID1[:])
	rand.Read(messageID2[:])

	// Store complete message
	storage.StoreMessage(messageID1, []byte("complete message"))

	// Store partial message (only 2 shards)
	encoder, _ := NewErasureEncoder(nil)
	shards, _ := encoder.EncodeMessage(messageID2, []byte("partial"))
	storage.StoreShard(shards[0])
	storage.StoreShard(shards[1])

	stats := storage.GetStats()
	assert.Equal(t, 2, stats.TotalMessages)
	assert.Equal(t, 7, stats.TotalShards) // 5 + 2
	assert.Equal(t, 1, stats.CompleteMessages)
	assert.Equal(t, 1, stats.PartialMessages)
	assert.Equal(t, 3, stats.DataShards)
	assert.Equal(t, 2, stats.ParityShards)
}

func TestErasureShardEnvelope(t *testing.T) {
	encoder, err := NewErasureEncoder(nil)
	require.NoError(t, err)

	var messageID [32]byte
	rand.Read(messageID[:])
	data := []byte("envelope test")

	shards, err := encoder.EncodeMessage(messageID, data)
	require.NoError(t, err)

	var recipientPK [32]byte
	rand.Read(recipientPK[:])

	envelope, err := NewErasureShardEnvelope(shards[0], recipientPK, len(data))
	require.NoError(t, err)
	assert.NotNil(t, envelope)
	assert.Equal(t, shards[0], envelope.Shard)
	assert.Equal(t, recipientPK, envelope.RecipientPK)
	assert.Equal(t, len(data), envelope.OriginalLength)

	// Nonce should be non-zero
	assert.NotEqual(t, [24]byte{}, envelope.Nonce)
}

func TestErasureShardEnvelope_NilShard(t *testing.T) {
	var recipientPK [32]byte
	envelope, err := NewErasureShardEnvelope(nil, recipientPK, 100)
	assert.Error(t, err)
	assert.Nil(t, envelope)
}

func TestErasureEncoder_LargeDataReconstruction(t *testing.T) {
	encoder, err := NewErasureEncoder(nil)
	require.NoError(t, err)

	var messageID [32]byte
	rand.Read(messageID[:])

	// Test with 100KB of random data
	originalData := make([]byte, 100*1024)
	rand.Read(originalData)

	shards, err := encoder.EncodeMessage(messageID, originalData)
	require.NoError(t, err)

	// Reconstruct with minimum shards
	partialShards := make([]*EncodedShard, 5)
	partialShards[0] = shards[0]
	partialShards[2] = shards[2]
	partialShards[4] = shards[4] // parity

	reconstructed, err := encoder.DecodeShards(partialShards, len(originalData))
	require.NoError(t, err)
	assert.True(t, bytes.Equal(originalData, reconstructed), "large data should reconstruct correctly")
}

func TestErasureEncoder_Concurrency(t *testing.T) {
	encoder, err := NewErasureEncoder(nil)
	require.NoError(t, err)

	const goroutines = 10
	done := make(chan bool, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			var messageID [32]byte
			rand.Read(messageID[:])
			data := []byte("concurrent test data")

			shards, err := encoder.EncodeMessage(messageID, data)
			if err != nil || len(shards) != 5 {
				done <- false
				return
			}

			reconstructed, err := encoder.DecodeShards(shards, len(data))
			if err != nil || !bytes.Equal(data, reconstructed) {
				done <- false
				return
			}

			done <- true
		}()
	}

	for i := 0; i < goroutines; i++ {
		assert.True(t, <-done, "concurrent operation should succeed")
	}
}

func BenchmarkErasureEncoder_Encode(b *testing.B) {
	encoder, _ := NewErasureEncoder(nil)
	var messageID [32]byte
	rand.Read(messageID[:])
	data := make([]byte, 1024) // 1KB message
	rand.Read(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoder.EncodeMessage(messageID, data)
	}
}

func BenchmarkErasureEncoder_Decode(b *testing.B) {
	encoder, _ := NewErasureEncoder(nil)
	var messageID [32]byte
	rand.Read(messageID[:])
	data := make([]byte, 1024)
	rand.Read(data)

	shards, _ := encoder.EncodeMessage(messageID, data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoder.DecodeShards(shards, len(data))
	}
}

func BenchmarkErasureEncoder_DecodePartial(b *testing.B) {
	encoder, _ := NewErasureEncoder(nil)
	var messageID [32]byte
	rand.Read(messageID[:])
	data := make([]byte, 1024)
	rand.Read(data)

	shards, _ := encoder.EncodeMessage(messageID, data)

	// Only 3 of 5 shards
	partialShards := make([]*EncodedShard, 5)
	partialShards[0] = shards[0]
	partialShards[1] = shards[1]
	partialShards[4] = shards[4]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoder.DecodeShards(partialShards, len(data))
	}
}
