// Package async implements an asynchronous message delivery system for Tox.
// This file provides erasure-coded redundant storage for message survival.
package async

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"

	"github.com/klauspost/reedsolomon"
)

// ErasureCodingConfig holds the configuration for erasure-coded storage.
// The default configuration uses 3 data shards and 2 parity shards (3+2=5),
// allowing reconstruction from any 3 of 5 storage nodes.
type ErasureCodingConfig struct {
	// DataShards is the number of data shards (k in Reed-Solomon)
	DataShards int
	// ParityShards is the number of parity shards (m in Reed-Solomon)
	ParityShards int
	// MinShards is the minimum shards needed for reconstruction
	// Should equal DataShards for optimal reconstruction
	MinShards int
}

// DefaultErasureCodingConfig returns the default 3+2 erasure coding configuration.
// This allows 2-of-5 node failures while maintaining message availability.
func DefaultErasureCodingConfig() *ErasureCodingConfig {
	return &ErasureCodingConfig{
		DataShards:   3,
		ParityShards: 2,
		MinShards:    3,
	}
}

// TotalShards returns the total number of shards (data + parity).
func (c *ErasureCodingConfig) TotalShards() int {
	return c.DataShards + c.ParityShards
}

// ErasureEncoder provides thread-safe encoding and decoding of messages
// using Reed-Solomon erasure coding for redundant distributed storage.
type ErasureEncoder struct {
	config  *ErasureCodingConfig
	encoder reedsolomon.Encoder
	mutex   sync.RWMutex
}

// NewErasureEncoder creates a new encoder with the given configuration.
// Returns an error if the configuration is invalid.
func NewErasureEncoder(config *ErasureCodingConfig) (*ErasureEncoder, error) {
	if config == nil {
		config = DefaultErasureCodingConfig()
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid erasure config: %w", err)
	}

	enc, err := reedsolomon.New(config.DataShards, config.ParityShards)
	if err != nil {
		return nil, fmt.Errorf("failed to create Reed-Solomon encoder: %w", err)
	}

	return &ErasureEncoder{
		config:  config,
		encoder: enc,
	}, nil
}

// validateConfig validates erasure coding configuration parameters.
func validateConfig(config *ErasureCodingConfig) error {
	if config.DataShards < 1 {
		return errors.New("data shards must be at least 1")
	}
	if config.ParityShards < 1 {
		return errors.New("parity shards must be at least 1")
	}
	if config.DataShards > 256 || config.ParityShards > 256 {
		return errors.New("shard count exceeds maximum of 256")
	}
	if config.MinShards < 1 || config.MinShards > config.DataShards {
		return errors.New("min shards must be between 1 and data shards")
	}
	return nil
}

// EncodedShard represents a single shard of erasure-coded data.
type EncodedShard struct {
	// Index is the shard index (0 to TotalShards-1)
	Index int
	// Data is the shard content
	Data []byte
	// IsParity indicates if this is a parity shard (vs data shard)
	IsParity bool
	// MessageID links the shard to its original message
	MessageID [32]byte
	// TotalShards is the total number of shards for reconstruction
	TotalShards int
	// DataShards is the number of data shards needed for reconstruction
	DataShards int
}

// EncodeMessage splits a message into data shards and generates parity shards.
// Returns all shards (data + parity) ready for distribution to storage nodes.
func (e *ErasureEncoder) EncodeMessage(messageID [32]byte, data []byte) ([]*EncodedShard, error) {
	if len(data) == 0 {
		return nil, errors.New("cannot encode empty data")
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	shardSize := calculateShardSize(len(data), e.config.DataShards)
	shards, err := e.splitIntoShards(data, shardSize)
	if err != nil {
		return nil, fmt.Errorf("failed to split data: %w", err)
	}

	if err := e.encoder.Encode(shards); err != nil {
		return nil, fmt.Errorf("failed to encode parity shards: %w", err)
	}

	return e.createEncodedShards(messageID, shards), nil
}

// calculateShardSize determines the size each shard should have.
// Rounds up to ensure all data fits in data shards.
func calculateShardSize(dataLen, dataShards int) int {
	shardSize := dataLen / dataShards
	if dataLen%dataShards != 0 {
		shardSize++
	}
	return shardSize
}

// splitIntoShards divides data into equal-sized shards with padding.
func (e *ErasureEncoder) splitIntoShards(data []byte, shardSize int) ([][]byte, error) {
	totalShards := e.config.TotalShards()
	shards := make([][]byte, totalShards)

	for i := range totalShards {
		shards[i] = make([]byte, shardSize)
	}

	for i, b := range data {
		shardIndex := i / shardSize
		byteIndex := i % shardSize
		if shardIndex < e.config.DataShards {
			shards[shardIndex][byteIndex] = b
		}
	}

	return shards, nil
}

// createEncodedShards wraps raw shards into EncodedShard structs.
func (e *ErasureEncoder) createEncodedShards(messageID [32]byte, shards [][]byte) []*EncodedShard {
	encodedShards := make([]*EncodedShard, len(shards))
	for i, shardData := range shards {
		encodedShards[i] = &EncodedShard{
			Index:       i,
			Data:        shardData,
			IsParity:    i >= e.config.DataShards,
			MessageID:   messageID,
			TotalShards: e.config.TotalShards(),
			DataShards:  e.config.DataShards,
		}
	}
	return encodedShards
}

// DecodeShards reconstructs the original message from available shards.
// Requires at least DataShards shards (any combination of data/parity).
// Missing shards should be provided as nil in the shards slice.
func (e *ErasureEncoder) DecodeShards(shards []*EncodedShard, originalLen int) ([]byte, error) {
	if len(shards) == 0 {
		return nil, errors.New("no shards provided")
	}

	if originalLen <= 0 {
		return nil, errors.New("original length must be positive")
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	rawShards, availableCount := e.extractRawShards(shards)
	if availableCount < e.config.DataShards {
		return nil, fmt.Errorf("insufficient shards: have %d, need %d",
			availableCount, e.config.DataShards)
	}

	if err := e.encoder.Reconstruct(rawShards); err != nil {
		return nil, fmt.Errorf("failed to reconstruct shards: %w", err)
	}

	return e.reassembleData(rawShards, originalLen), nil
}

// extractRawShards converts EncodedShard slice to raw byte slices for decoder.
func (e *ErasureEncoder) extractRawShards(shards []*EncodedShard) ([][]byte, int) {
	rawShards := make([][]byte, e.config.TotalShards())
	availableCount := 0

	for _, shard := range shards {
		if shard != nil && shard.Index < len(rawShards) {
			rawShards[shard.Index] = shard.Data
			availableCount++
		}
	}

	return rawShards, availableCount
}

// reassembleData reconstructs the original data from decoded data shards.
func (e *ErasureEncoder) reassembleData(shards [][]byte, originalLen int) []byte {
	if originalLen <= 0 {
		return nil
	}

	result := make([]byte, 0, originalLen)
	for i := 0; i < e.config.DataShards && len(result) < originalLen; i++ {
		toAppend := shards[i]
		remaining := originalLen - len(result)
		if len(toAppend) > remaining {
			toAppend = toAppend[:remaining]
		}
		result = append(result, toAppend...)
	}

	return result
}

// VerifyShards checks if the provided shards can reconstruct valid data.
// Returns true if the shards pass verification, false otherwise.
func (e *ErasureEncoder) VerifyShards(shards []*EncodedShard) (bool, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	rawShards, availableCount := e.extractRawShards(shards)
	if availableCount < e.config.TotalShards() {
		return false, nil
	}

	return e.encoder.Verify(rawShards)
}

// GetConfig returns a copy of the encoder's configuration.
func (e *ErasureEncoder) GetConfig() ErasureCodingConfig {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return *e.config
}

// ErasureStorage manages erasure-coded message storage with distributed
// shard placement across multiple storage nodes.
type ErasureStorage struct {
	encoder *ErasureEncoder
	mutex   sync.RWMutex
	// messageShards maps messageID -> shard index -> shard data
	messageShards map[[32]byte]map[int]*EncodedShard
	// originalLengths stores the original message length for reconstruction
	originalLengths map[[32]byte]int
}

// NewErasureStorage creates a new erasure-coded storage manager.
func NewErasureStorage(config *ErasureCodingConfig) (*ErasureStorage, error) {
	encoder, err := NewErasureEncoder(config)
	if err != nil {
		return nil, err
	}

	return &ErasureStorage{
		encoder:         encoder,
		messageShards:   make(map[[32]byte]map[int]*EncodedShard),
		originalLengths: make(map[[32]byte]int),
	}, nil
}

// StoreMessage encodes and stores all shards of a message locally.
// Returns the encoded shards for distribution to storage nodes.
func (es *ErasureStorage) StoreMessage(messageID [32]byte, data []byte) ([]*EncodedShard, error) {
	shards, err := es.encoder.EncodeMessage(messageID, data)
	if err != nil {
		return nil, err
	}

	es.mutex.Lock()
	defer es.mutex.Unlock()

	es.messageShards[messageID] = make(map[int]*EncodedShard)
	for _, shard := range shards {
		es.messageShards[messageID][shard.Index] = shard
	}
	es.originalLengths[messageID] = len(data)

	return shards, nil
}

// StoreShard stores a single shard retrieved from a storage node.
func (es *ErasureStorage) StoreShard(shard *EncodedShard) error {
	if shard == nil {
		return errors.New("nil shard")
	}

	es.mutex.Lock()
	defer es.mutex.Unlock()

	if _, exists := es.messageShards[shard.MessageID]; !exists {
		es.messageShards[shard.MessageID] = make(map[int]*EncodedShard)
	}
	es.messageShards[shard.MessageID][shard.Index] = shard

	return nil
}

// SetOriginalLength sets the original message length for reconstruction.
// This should be stored alongside shards for proper decoding.
func (es *ErasureStorage) SetOriginalLength(messageID [32]byte, length int) {
	es.mutex.Lock()
	defer es.mutex.Unlock()
	es.originalLengths[messageID] = length
}

// ReconstructMessage attempts to reconstruct a message from available shards.
func (es *ErasureStorage) ReconstructMessage(messageID [32]byte) ([]byte, error) {
	es.mutex.RLock()
	shardMap, exists := es.messageShards[messageID]
	originalLen := es.originalLengths[messageID]
	es.mutex.RUnlock()

	if !exists || len(shardMap) == 0 {
		return nil, ErrMessageNotFound
	}

	if originalLen <= 0 {
		return nil, errors.New("unknown original message length")
	}

	shards := make([]*EncodedShard, es.encoder.config.TotalShards())
	for idx, shard := range shardMap {
		if idx < len(shards) {
			shards[idx] = shard
		}
	}

	return es.encoder.DecodeShards(shards, originalLen)
}

// HasSufficientShards checks if enough shards exist for message reconstruction.
func (es *ErasureStorage) HasSufficientShards(messageID [32]byte) bool {
	es.mutex.RLock()
	defer es.mutex.RUnlock()

	shardMap, exists := es.messageShards[messageID]
	if !exists {
		return false
	}

	return len(shardMap) >= es.encoder.config.DataShards
}

// GetShardCount returns the number of available shards for a message.
func (es *ErasureStorage) GetShardCount(messageID [32]byte) int {
	es.mutex.RLock()
	defer es.mutex.RUnlock()

	return len(es.messageShards[messageID])
}

// DeleteMessage removes all shards for a message from local storage.
func (es *ErasureStorage) DeleteMessage(messageID [32]byte) {
	es.mutex.Lock()
	defer es.mutex.Unlock()

	delete(es.messageShards, messageID)
	delete(es.originalLengths, messageID)
}

// GetMissingShardIndices returns the indices of shards not yet received.
func (es *ErasureStorage) GetMissingShardIndices(messageID [32]byte) []int {
	es.mutex.RLock()
	defer es.mutex.RUnlock()

	shardMap := es.messageShards[messageID]
	totalShards := es.encoder.config.TotalShards()

	var missing []int
	for i := 0; i < totalShards; i++ {
		if _, exists := shardMap[i]; !exists {
			missing = append(missing, i)
		}
	}

	return missing
}

// ErasureShardEnvelope wraps a shard with metadata for network transmission.
type ErasureShardEnvelope struct {
	Shard           *EncodedShard
	RecipientPK     [32]byte
	OriginalLength  int
	Nonce           [24]byte
	StorageNodeAddr string
}

// NewErasureShardEnvelope creates an envelope for transmitting a shard.
func NewErasureShardEnvelope(shard *EncodedShard, recipientPK [32]byte, originalLen int) (*ErasureShardEnvelope, error) {
	if shard == nil {
		return nil, errors.New("nil shard")
	}

	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	return &ErasureShardEnvelope{
		Shard:          shard,
		RecipientPK:    recipientPK,
		OriginalLength: originalLen,
		Nonce:          nonce,
	}, nil
}

// ErasureStats provides statistics about erasure-coded message storage.
type ErasureStats struct {
	TotalMessages    int
	TotalShards      int
	CompleteMessages int
	PartialMessages  int
	DataShards       int
	ParityShards     int
}

// GetStats returns statistics about the current erasure storage state.
func (es *ErasureStorage) GetStats() ErasureStats {
	es.mutex.RLock()
	defer es.mutex.RUnlock()

	stats := ErasureStats{
		TotalMessages: len(es.messageShards),
		DataShards:    es.encoder.config.DataShards,
		ParityShards:  es.encoder.config.ParityShards,
	}

	for _, shardMap := range es.messageShards {
		stats.TotalShards += len(shardMap)
		if len(shardMap) >= es.encoder.config.TotalShards() {
			stats.CompleteMessages++
		} else {
			stats.PartialMessages++
		}
	}

	return stats
}
