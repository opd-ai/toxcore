// Package friend implements the friend management system for the Tox protocol.
package friend

import (
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

// FriendStore provides typed access to sharded friend storage.
// This is a wrapper around ShardedFriendStore that provides type-safe
// operations for Friend pointers, eliminating the need for type assertions
// at call sites.
//
// FriendStore is designed to be a drop-in replacement for
// map[uint32]*Friend + sync.RWMutex patterns in toxcore.go.
//
// Thread Safety: FriendStore is safe for concurrent use from multiple
// goroutines, as it delegates to the thread-safe ShardedFriendStore.
type FriendStore[T any] struct {
	store *ShardedFriendStore
	count int64 // atomic counter for O(1) Count()
}

// NewFriendStore creates a new typed friend store backed by sharded storage.
func NewFriendStore[T any]() *FriendStore[T] {
	logrus.WithFields(logrus.Fields{
		"function": "NewFriendStore",
	}).Info("Creating typed friend store")
	return &FriendStore[T]{
		store: NewShardedFriendStore(),
	}
}

// Get retrieves a friend by ID. Returns nil if not found.
func (fs *FriendStore[T]) Get(friendID uint32) *T {
	val := fs.store.Get(friendID)
	if val == nil {
		return nil
	}
	if typed, ok := val.(*T); ok {
		return typed
	}
	return nil
}

// Set stores a friend by ID.
func (fs *FriendStore[T]) Set(friendID uint32, friend *T) {
	// Check if this is a new entry
	if !fs.store.Exists(friendID) {
		atomic.AddInt64(&fs.count, 1)
	}
	fs.store.Set(friendID, friend)
}

// Delete removes a friend by ID. Returns true if the friend existed.
func (fs *FriendStore[T]) Delete(friendID uint32) bool {
	existed := fs.store.Delete(friendID)
	if existed {
		atomic.AddInt64(&fs.count, -1)
	}
	return existed
}

// Exists checks if a friend ID exists in the store.
func (fs *FriendStore[T]) Exists(friendID uint32) bool {
	return fs.store.Exists(friendID)
}

// Update atomically retrieves and updates a friend by ID.
// The updateFn is called with the friend while holding the shard lock,
// ensuring thread-safe modifications to friend fields.
// Returns true if the friend existed and was updated.
func (fs *FriendStore[T]) Update(friendID uint32, updateFn func(*T)) bool {
	return fs.store.Update(friendID, func(val interface{}) {
		if typed, ok := val.(*T); ok {
			updateFn(typed)
		}
	})
}

// Read atomically reads from a friend by ID.
// The readFn is called with the friend while holding a read lock on the shard,
// ensuring thread-safe reads of friend fields.
// Returns false if the friend doesn't exist.
func (fs *FriendStore[T]) Read(friendID uint32, readFn func(*T)) bool {
	return fs.store.Read(friendID, func(val interface{}) {
		if typed, ok := val.(*T); ok {
			readFn(typed)
		}
	})
}

// Count returns the total number of friends in the store.
// This is O(1) using atomic counter.
func (fs *FriendStore[T]) Count() int {
	return int(atomic.LoadInt64(&fs.count))
}

// Range iterates over all friends in the store.
// The callback receives friend ID and friend pointer.
// If the callback returns false, iteration stops.
//
// Note: This provides eventually consistent iteration - modifications
// during iteration may or may not be visible.
func (fs *FriendStore[T]) Range(fn func(friendID uint32, friend *T) bool) {
	fs.store.Range(func(friendID uint32, val interface{}) bool {
		if typed, ok := val.(*T); ok {
			return fn(friendID, typed)
		}
		return true // Skip invalid entries
	})
}

// RangeWithSnapshot creates a snapshot of all friends and iterates over it.
// This provides a consistent view but uses more memory.
func (fs *FriendStore[T]) RangeWithSnapshot(fn func(friendID uint32, friend *T) bool) {
	fs.store.RangeWithSnapshot(func(friendID uint32, val interface{}) bool {
		if typed, ok := val.(*T); ok {
			return fn(friendID, typed)
		}
		return true // Skip invalid entries
	})
}

// GetAll returns a copy of all friends as a map.
// This is useful for serialization operations.
func (fs *FriendStore[T]) GetAll() map[uint32]*T {
	result := make(map[uint32]*T)
	fs.store.Range(func(friendID uint32, val interface{}) bool {
		if typed, ok := val.(*T); ok {
			result[friendID] = typed
		}
		return true
	})
	return result
}

// Clear removes all friends from the store.
func (fs *FriendStore[T]) Clear() {
	fs.store.Clear()
	atomic.StoreInt64(&fs.count, 0)
}

// GetShardStats returns statistics for each shard.
// Useful for monitoring shard distribution and diagnosing hotspots.
func (fs *FriendStore[T]) GetShardStats() []ShardStats {
	return fs.store.GetShardStats()
}

// FindByPublicKey searches for a friend by public key.
// Returns the friend ID and friend, or 0 and nil if not found.
// This is an O(n) operation as it must search all shards.
func (fs *FriendStore[T]) FindByPublicKey(publicKey [32]byte, getPublicKey func(*T) [32]byte) (uint32, *T) {
	id, val := fs.store.FindByPublicKey(publicKey, func(v interface{}) [32]byte {
		if typed, ok := v.(*T); ok {
			return getPublicKey(typed)
		}
		return [32]byte{}
	})
	if val == nil {
		return 0, nil
	}
	if typed, ok := val.(*T); ok {
		return id, typed
	}
	return 0, nil
}
