// Package friend implements the friend management system for the Tox protocol.
package friend

import (
	"sync"

	"github.com/sirupsen/logrus"
)

const (
	// NumShards is the number of shards for the friend store (16 = 2^4, using 4-bit prefix).
	NumShards = 16

	// ShardMask is used to extract the shard index from a friend ID.
	ShardMask = NumShards - 1
)

// shard represents a single partition of the friend store.
type shard struct {
	mu      sync.RWMutex
	friends map[uint32]interface{} // Using interface{} to store any Friend type
}

// ShardedFriendStore provides a sharded friend storage to reduce mutex contention
// at scale. Friends are distributed across 16 shards based on a 4-bit prefix of
// their friend ID, allowing concurrent access to different shards.
//
// This implementation reduces lock contention when handling >1000 concurrent
// friend operations compared to a single global mutex.
//
// Thread Safety: ShardedFriendStore is safe for concurrent use from multiple
// goroutines. Each shard has its own read-write mutex.
type ShardedFriendStore struct {
	shards [NumShards]shard
	// Stats tracking (atomic operations for lock-free reads)
	totalFriends int64
}

// NewShardedFriendStore creates a new sharded friend store with 16 shards.
func NewShardedFriendStore() *ShardedFriendStore {
	store := &ShardedFriendStore{}
	for i := range store.shards {
		store.shards[i].friends = make(map[uint32]interface{})
	}
	logrus.WithFields(logrus.Fields{
		"function":   "NewShardedFriendStore",
		"num_shards": NumShards,
	}).Info("Created sharded friend store")
	return store
}

// getShardIndex returns the shard index for a given friend ID.
// Uses the lower 4 bits to distribute friends evenly across shards.
func getShardIndex(friendID uint32) int {
	return int(friendID & ShardMask)
}

// getShard returns the shard for a given friend ID.
func (s *ShardedFriendStore) getShard(friendID uint32) *shard {
	return &s.shards[getShardIndex(friendID)]
}

// Get retrieves a friend by ID. Returns nil if not found.
func (s *ShardedFriendStore) Get(friendID uint32) interface{} {
	sh := s.getShard(friendID)
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	return sh.friends[friendID]
}

// Set stores a friend by ID.
func (s *ShardedFriendStore) Set(friendID uint32, friend interface{}) {
	sh := s.getShard(friendID)
	sh.mu.Lock()
	defer sh.mu.Unlock()

	_, exists := sh.friends[friendID]
	sh.friends[friendID] = friend

	if !exists {
		logrus.WithFields(logrus.Fields{
			"function":  "ShardedFriendStore.Set",
			"friend_id": friendID,
			"shard":     getShardIndex(friendID),
		}).Debug("Added friend to sharded store")
	}
}

// Update atomically retrieves and updates a friend by ID.
// The updateFn is called with the friend while the shard lock is held.
// Returns true if the friend existed and was updated.
func (s *ShardedFriendStore) Update(friendID uint32, updateFn func(interface{})) bool {
	sh := s.getShard(friendID)
	sh.mu.Lock()
	defer sh.mu.Unlock()

	friend, exists := sh.friends[friendID]
	if exists && friend != nil {
		updateFn(friend)
	}
	return exists
}

// Read atomically reads from a friend by ID.
// The readFn is called with the friend while holding a read lock on the shard.
// Returns true if the friend existed.
func (s *ShardedFriendStore) Read(friendID uint32, readFn func(interface{})) bool {
	sh := s.getShard(friendID)
	sh.mu.RLock()
	defer sh.mu.RUnlock()

	friend, exists := sh.friends[friendID]
	if exists && friend != nil {
		readFn(friend)
	}
	return exists
}

// Delete removes a friend by ID. Returns true if the friend existed.
func (s *ShardedFriendStore) Delete(friendID uint32) bool {
	sh := s.getShard(friendID)
	sh.mu.Lock()
	defer sh.mu.Unlock()

	_, exists := sh.friends[friendID]
	if exists {
		delete(sh.friends, friendID)
		logrus.WithFields(logrus.Fields{
			"function":  "ShardedFriendStore.Delete",
			"friend_id": friendID,
			"shard":     getShardIndex(friendID),
		}).Debug("Removed friend from sharded store")
	}
	return exists
}

// Exists checks if a friend ID exists in the store.
func (s *ShardedFriendStore) Exists(friendID uint32) bool {
	sh := s.getShard(friendID)
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	_, exists := sh.friends[friendID]
	return exists
}

// Count returns the total number of friends across all shards.
func (s *ShardedFriendStore) Count() int {
	count := 0
	for i := range s.shards {
		s.shards[i].mu.RLock()
		count += len(s.shards[i].friends)
		s.shards[i].mu.RUnlock()
	}
	return count
}

// Range iterates over all friends in the store.
// The callback receives friend ID and friend value.
// If the callback returns false, iteration stops.
//
// Note: This acquires read locks on each shard sequentially, which may lead
// to inconsistent views if the store is being modified concurrently.
// For strong consistency, use RangeWithSnapshot.
func (s *ShardedFriendStore) Range(fn func(friendID uint32, friend interface{}) bool) {
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.RLock()
		for id, friend := range sh.friends {
			if !fn(id, friend) {
				sh.mu.RUnlock()
				return
			}
		}
		sh.mu.RUnlock()
	}
}

// RangeWithSnapshot creates a snapshot of all friends and iterates over it.
// This provides a consistent view but uses more memory.
func (s *ShardedFriendStore) RangeWithSnapshot(fn func(friendID uint32, friend interface{}) bool) {
	// Collect snapshot
	snapshot := make(map[uint32]interface{})
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.RLock()
		for id, friend := range sh.friends {
			snapshot[id] = friend
		}
		sh.mu.RUnlock()
	}

	// Iterate over snapshot
	for id, friend := range snapshot {
		if !fn(id, friend) {
			return
		}
	}
}

// GetAll returns a copy of all friends as a map.
// This is useful for serialization operations.
func (s *ShardedFriendStore) GetAll() map[uint32]interface{} {
	result := make(map[uint32]interface{})
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.RLock()
		for id, friend := range sh.friends {
			result[id] = friend
		}
		sh.mu.RUnlock()
	}
	return result
}

// Clear removes all friends from the store.
func (s *ShardedFriendStore) Clear() {
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.Lock()
		sh.friends = make(map[uint32]interface{})
		sh.mu.Unlock()
	}
	logrus.WithFields(logrus.Fields{
		"function": "ShardedFriendStore.Clear",
	}).Info("Cleared all friends from sharded store")
}

// ShardStats contains statistics for each shard.
type ShardStats struct {
	ShardIndex  int
	FriendCount int
}

// GetShardStats returns statistics for each shard.
// Useful for monitoring shard distribution and diagnosing hotspots.
func (s *ShardedFriendStore) GetShardStats() []ShardStats {
	stats := make([]ShardStats, NumShards)
	for i := range s.shards {
		s.shards[i].mu.RLock()
		stats[i] = ShardStats{
			ShardIndex:  i,
			FriendCount: len(s.shards[i].friends),
		}
		s.shards[i].mu.RUnlock()
	}
	return stats
}

// FindByPublicKey searches for a friend by public key.
// Returns the friend ID and friend, or 0 and nil if not found.
// This is an O(n) operation as it must search all shards.
func (s *ShardedFriendStore) FindByPublicKey(publicKey [32]byte, getPublicKey func(interface{}) [32]byte) (uint32, interface{}) {
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.RLock()
		for id, friend := range sh.friends {
			if getPublicKey(friend) == publicKey {
				sh.mu.RUnlock()
				return id, friend
			}
		}
		sh.mu.RUnlock()
	}
	return 0, nil
}
