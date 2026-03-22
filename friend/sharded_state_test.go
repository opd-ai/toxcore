package friend

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewShardedFriendStore(t *testing.T) {
	store := NewShardedFriendStore()
	require.NotNil(t, store)
	assert.Equal(t, 0, store.Count())
}

func TestShardedFriendStore_SetGetDelete(t *testing.T) {
	store := NewShardedFriendStore()

	// Test Set and Get
	friend1 := "friend1"
	store.Set(1, friend1)
	assert.Equal(t, friend1, store.Get(1))
	assert.True(t, store.Exists(1))
	assert.Equal(t, 1, store.Count())

	// Test Get non-existent
	assert.Nil(t, store.Get(999))
	assert.False(t, store.Exists(999))

	// Test Delete
	deleted := store.Delete(1)
	assert.True(t, deleted)
	assert.Nil(t, store.Get(1))
	assert.False(t, store.Exists(1))
	assert.Equal(t, 0, store.Count())

	// Test Delete non-existent
	deleted = store.Delete(999)
	assert.False(t, deleted)
}

func TestShardedFriendStore_ShardDistribution(t *testing.T) {
	store := NewShardedFriendStore()

	// Add friends with IDs that should distribute across shards
	for i := uint32(0); i < 160; i++ {
		store.Set(i, struct{}{})
	}

	stats := store.GetShardStats()
	assert.Len(t, stats, NumShards)

	// Each shard should have approximately 10 friends (160/16)
	for _, stat := range stats {
		assert.Equal(t, 10, stat.FriendCount, "Shard %d should have 10 friends", stat.ShardIndex)
	}
}

func TestShardedFriendStore_Range(t *testing.T) {
	store := NewShardedFriendStore()

	// Add 100 friends
	for i := uint32(0); i < 100; i++ {
		store.Set(i, i*10)
	}

	// Count using Range
	count := 0
	store.Range(func(friendID uint32, friend interface{}) bool {
		count++
		return true
	})
	assert.Equal(t, 100, count)

	// Test early termination
	earlyCount := 0
	store.Range(func(friendID uint32, friend interface{}) bool {
		earlyCount++
		return earlyCount < 50
	})
	assert.Equal(t, 50, earlyCount)
}

func TestShardedFriendStore_RangeWithSnapshot(t *testing.T) {
	store := NewShardedFriendStore()

	for i := uint32(0); i < 50; i++ {
		store.Set(i, i)
	}

	count := 0
	store.RangeWithSnapshot(func(friendID uint32, friend interface{}) bool {
		count++
		return true
	})
	assert.Equal(t, 50, count)
}

func TestShardedFriendStore_GetAll(t *testing.T) {
	store := NewShardedFriendStore()

	for i := uint32(0); i < 25; i++ {
		store.Set(i, i*2)
	}

	all := store.GetAll()
	assert.Len(t, all, 25)

	for i := uint32(0); i < 25; i++ {
		val, ok := all[i]
		assert.True(t, ok)
		assert.Equal(t, i*2, val)
	}
}

func TestShardedFriendStore_Clear(t *testing.T) {
	store := NewShardedFriendStore()

	for i := uint32(0); i < 100; i++ {
		store.Set(i, i)
	}
	assert.Equal(t, 100, store.Count())

	store.Clear()
	assert.Equal(t, 0, store.Count())

	// Verify all shards are empty
	stats := store.GetShardStats()
	for _, stat := range stats {
		assert.Equal(t, 0, stat.FriendCount)
	}
}

func TestShardedFriendStore_FindByPublicKey(t *testing.T) {
	store := NewShardedFriendStore()

	type testFriend struct {
		ID        uint32
		PublicKey [32]byte
	}

	// Add friends with unique public keys
	for i := uint32(0); i < 50; i++ {
		var pk [32]byte
		pk[0] = byte(i)
		pk[1] = byte(i + 100)
		store.Set(i, &testFriend{ID: i, PublicKey: pk})
	}

	// Create the public key getter function
	getPublicKey := func(f interface{}) [32]byte {
		return f.(*testFriend).PublicKey
	}

	// Find existing friend
	var searchKey [32]byte
	searchKey[0] = 25
	searchKey[1] = 125
	id, found := store.FindByPublicKey(searchKey, getPublicKey)
	assert.NotNil(t, found)
	assert.Equal(t, uint32(25), id)

	// Find non-existent friend
	var missingKey [32]byte
	missingKey[0] = 200
	id, found = store.FindByPublicKey(missingKey, getPublicKey)
	assert.Nil(t, found)
	assert.Equal(t, uint32(0), id)
}

func TestShardedFriendStore_ConcurrentAccess(t *testing.T) {
	store := NewShardedFriendStore()
	var wg sync.WaitGroup
	numGoroutines := 100
	numOpsPerGoroutine := 100

	// Concurrent writes to different IDs
	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(base int) {
			defer wg.Done()
			for i := 0; i < numOpsPerGoroutine; i++ {
				id := uint32(base*numOpsPerGoroutine + i)
				store.Set(id, id)
			}
		}(g)
	}
	wg.Wait()

	assert.Equal(t, numGoroutines*numOpsPerGoroutine, store.Count())

	// Concurrent reads
	wg.Add(numGoroutines)
	var readCount int64
	for g := 0; g < numGoroutines; g++ {
		go func(base int) {
			defer wg.Done()
			for i := 0; i < numOpsPerGoroutine; i++ {
				id := uint32(base*numOpsPerGoroutine + i)
				if store.Get(id) != nil {
					atomic.AddInt64(&readCount, 1)
				}
			}
		}(g)
	}
	wg.Wait()

	assert.Equal(t, int64(numGoroutines*numOpsPerGoroutine), readCount)

	// Concurrent deletes
	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(base int) {
			defer wg.Done()
			for i := 0; i < numOpsPerGoroutine; i++ {
				id := uint32(base*numOpsPerGoroutine + i)
				store.Delete(id)
			}
		}(g)
	}
	wg.Wait()

	assert.Equal(t, 0, store.Count())
}

func TestShardedFriendStore_ConcurrentReadWrite(t *testing.T) {
	store := NewShardedFriendStore()
	var wg sync.WaitGroup

	// Pre-populate
	for i := uint32(0); i < 100; i++ {
		store.Set(i, i)
	}

	// Concurrent readers and writers
	numReaders := 50
	numWriters := 50
	numOps := 100

	wg.Add(numReaders + numWriters)

	// Readers
	for r := 0; r < numReaders; r++ {
		go func() {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				id := uint32(i % 100)
				_ = store.Get(id)
				_ = store.Exists(id)
			}
		}()
	}

	// Writers
	for w := 0; w < numWriters; w++ {
		go func(offset int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				id := uint32(100 + offset*numOps + i)
				store.Set(id, id)
			}
		}(w)
	}

	wg.Wait()
	// Should have 100 original + 50*100 new
	assert.Equal(t, 100+numWriters*numOps, store.Count())
}

func TestGetShardIndex(t *testing.T) {
	// Test that shard index uses lower 4 bits
	testCases := []struct {
		friendID      uint32
		expectedShard int
	}{
		{0, 0},
		{1, 1},
		{15, 15},
		{16, 0}, // wraps around
		{17, 1},
		{32, 0},
		{255, 15},
		{256, 0},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tc.expectedShard, getShardIndex(tc.friendID),
				"friendID %d should map to shard %d", tc.friendID, tc.expectedShard)
		})
	}
}

func BenchmarkShardedFriendStore_Set(b *testing.B) {
	store := NewShardedFriendStore()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Set(uint32(i), i)
	}
}

func BenchmarkShardedFriendStore_Get(b *testing.B) {
	store := NewShardedFriendStore()
	for i := uint32(0); i < 10000; i++ {
		store.Set(i, i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Get(uint32(i % 10000))
	}
}

func BenchmarkShardedFriendStore_ConcurrentSet(b *testing.B) {
	store := NewShardedFriendStore()
	b.RunParallel(func(pb *testing.PB) {
		id := uint32(0)
		for pb.Next() {
			store.Set(id, id)
			id++
		}
	})
}

func BenchmarkShardedFriendStore_ConcurrentGet(b *testing.B) {
	store := NewShardedFriendStore()
	for i := uint32(0); i < 10000; i++ {
		store.Set(i, i)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := uint32(0)
		for pb.Next() {
			store.Get(id % 10000)
			id++
		}
	})
}

// Comparison benchmark against a simple mutex-protected map
type simpleFriendStore struct {
	mu      sync.RWMutex
	friends map[uint32]interface{}
}

func (s *simpleFriendStore) Set(id uint32, friend interface{}) {
	s.mu.Lock()
	s.friends[id] = friend
	s.mu.Unlock()
}

func (s *simpleFriendStore) Get(id uint32) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.friends[id]
}

func BenchmarkSimpleFriendStore_ConcurrentSet(b *testing.B) {
	store := &simpleFriendStore{friends: make(map[uint32]interface{})}
	b.RunParallel(func(pb *testing.PB) {
		id := uint32(0)
		for pb.Next() {
			store.Set(id, id)
			id++
		}
	})
}

func BenchmarkSimpleFriendStore_ConcurrentGet(b *testing.B) {
	store := &simpleFriendStore{friends: make(map[uint32]interface{})}
	for i := uint32(0); i < 10000; i++ {
		store.Set(i, i)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := uint32(0)
		for pb.Next() {
			store.Get(id % 10000)
			id++
		}
	})
}
