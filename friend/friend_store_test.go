package friend

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFriend is a simple struct for testing the generic FriendStore
type testFriend struct {
	PublicKey [32]byte
	Name      string
}

func TestFriendStore_BasicOperations(t *testing.T) {
	store := NewFriendStore[testFriend]()

	// Test initial state
	assert.Equal(t, 0, store.Count())
	assert.Nil(t, store.Get(1))
	assert.False(t, store.Exists(1))

	// Test Set and Get
	f1 := &testFriend{Name: "Alice"}
	f1.PublicKey[0] = 0x01
	store.Set(1, f1)

	assert.Equal(t, 1, store.Count())
	assert.True(t, store.Exists(1))
	retrieved := store.Get(1)
	require.NotNil(t, retrieved)
	assert.Equal(t, "Alice", retrieved.Name)

	// Test Set overwrites (count should not increase)
	f1Updated := &testFriend{Name: "Alice Updated"}
	f1Updated.PublicKey[0] = 0x01
	store.Set(1, f1Updated)
	assert.Equal(t, 1, store.Count())
	assert.Equal(t, "Alice Updated", store.Get(1).Name)

	// Test Delete
	deleted := store.Delete(1)
	assert.True(t, deleted)
	assert.Equal(t, 0, store.Count())
	assert.Nil(t, store.Get(1))

	// Delete non-existent
	deleted = store.Delete(999)
	assert.False(t, deleted)
}

func TestFriendStore_MultipleFriends(t *testing.T) {
	store := NewFriendStore[testFriend]()

	// Add multiple friends
	for i := uint32(0); i < 100; i++ {
		f := &testFriend{Name: "Friend"}
		f.PublicKey[0] = byte(i)
		store.Set(i, f)
	}

	assert.Equal(t, 100, store.Count())

	// Verify all exist
	for i := uint32(0); i < 100; i++ {
		assert.True(t, store.Exists(i))
		f := store.Get(i)
		require.NotNil(t, f)
		assert.Equal(t, byte(i), f.PublicKey[0])
	}

	// Delete half
	for i := uint32(0); i < 50; i++ {
		store.Delete(i)
	}

	assert.Equal(t, 50, store.Count())

	// Verify deleted ones are gone
	for i := uint32(0); i < 50; i++ {
		assert.False(t, store.Exists(i))
	}

	// Verify remaining ones exist
	for i := uint32(50); i < 100; i++ {
		assert.True(t, store.Exists(i))
	}
}

func TestFriendStore_Range(t *testing.T) {
	store := NewFriendStore[testFriend]()

	for i := uint32(0); i < 10; i++ {
		f := &testFriend{Name: "Friend"}
		f.PublicKey[0] = byte(i)
		store.Set(i, f)
	}

	// Count using Range
	count := 0
	store.Range(func(friendID uint32, friend *testFriend) bool {
		count++
		return true
	})
	assert.Equal(t, 10, count)

	// Early termination
	count = 0
	store.Range(func(friendID uint32, friend *testFriend) bool {
		count++
		return count < 5 // Stop after 5
	})
	assert.Equal(t, 5, count)
}

func TestFriendStore_RangeWithSnapshot(t *testing.T) {
	store := NewFriendStore[testFriend]()

	for i := uint32(0); i < 10; i++ {
		f := &testFriend{Name: "Friend"}
		f.PublicKey[0] = byte(i)
		store.Set(i, f)
	}

	// Count using RangeWithSnapshot
	count := 0
	store.RangeWithSnapshot(func(friendID uint32, friend *testFriend) bool {
		count++
		return true
	})
	assert.Equal(t, 10, count)
}

func TestFriendStore_GetAll(t *testing.T) {
	store := NewFriendStore[testFriend]()

	for i := uint32(0); i < 10; i++ {
		f := &testFriend{Name: "Friend"}
		f.PublicKey[0] = byte(i)
		store.Set(i, f)
	}

	all := store.GetAll()
	assert.Len(t, all, 10)

	for i := uint32(0); i < 10; i++ {
		f, ok := all[i]
		assert.True(t, ok)
		assert.Equal(t, byte(i), f.PublicKey[0])
	}
}

func TestFriendStore_Clear(t *testing.T) {
	store := NewFriendStore[testFriend]()

	for i := uint32(0); i < 100; i++ {
		f := &testFriend{Name: "Friend"}
		store.Set(i, f)
	}

	assert.Equal(t, 100, store.Count())

	store.Clear()

	assert.Equal(t, 0, store.Count())
	assert.False(t, store.Exists(0))
}

func TestFriendStore_FindByPublicKey(t *testing.T) {
	store := NewFriendStore[testFriend]()

	// Add friends with unique public keys
	for i := uint32(0); i < 10; i++ {
		f := &testFriend{Name: "Friend"}
		f.PublicKey[0] = byte(i + 100)
		f.PublicKey[1] = byte(i)
		store.Set(i, f)
	}

	getPublicKey := func(f *testFriend) [32]byte {
		return f.PublicKey
	}

	// Find existing
	var searchKey [32]byte
	searchKey[0] = 105
	searchKey[1] = 5
	id, f := store.FindByPublicKey(searchKey, getPublicKey)
	assert.Equal(t, uint32(5), id)
	require.NotNil(t, f)
	assert.Equal(t, searchKey, f.PublicKey)

	// Find non-existent
	searchKey[0] = 255
	id, f = store.FindByPublicKey(searchKey, getPublicKey)
	assert.Equal(t, uint32(0), id)
	assert.Nil(t, f)
}

func TestFriendStore_Concurrent(t *testing.T) {
	store := NewFriendStore[testFriend]()

	var wg sync.WaitGroup
	numGoroutines := 100
	numOps := 100

	// Concurrent writes
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				friendID := uint32(goroutineID*numOps + i)
				f := &testFriend{Name: "Concurrent"}
				f.PublicKey[0] = byte(friendID % 256)
				store.Set(friendID, f)
			}
		}(g)
	}
	wg.Wait()

	assert.Equal(t, numGoroutines*numOps, store.Count())

	// Concurrent reads
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				friendID := uint32(goroutineID*numOps + i)
				f := store.Get(friendID)
				assert.NotNil(t, f)
			}
		}(g)
	}
	wg.Wait()

	// Concurrent deletes
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				friendID := uint32(goroutineID*numOps + i)
				store.Delete(friendID)
			}
		}(g)
	}
	wg.Wait()

	assert.Equal(t, 0, store.Count())
}

func TestFriendStore_GetShardStats(t *testing.T) {
	store := NewFriendStore[testFriend]()

	// Add 160 friends (10 per shard on average)
	for i := uint32(0); i < 160; i++ {
		f := &testFriend{Name: "Friend"}
		store.Set(i, f)
	}

	stats := store.GetShardStats()
	assert.Len(t, stats, NumShards)

	totalCount := 0
	for _, stat := range stats {
		totalCount += stat.FriendCount
	}
	assert.Equal(t, 160, totalCount)
}

func BenchmarkFriendStore_Get(b *testing.B) {
	store := NewFriendStore[testFriend]()
	for i := uint32(0); i < 1000; i++ {
		f := &testFriend{Name: "Bench"}
		store.Set(i, f)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Get(uint32(i % 1000))
	}
}

func BenchmarkFriendStore_Set(b *testing.B) {
	store := NewFriendStore[testFriend]()
	f := &testFriend{Name: "Bench"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Set(uint32(i), f)
	}
}

func BenchmarkFriendStore_ConcurrentAccess(b *testing.B) {
	store := NewFriendStore[testFriend]()
	for i := uint32(0); i < 1000; i++ {
		f := &testFriend{Name: "Bench"}
		store.Set(i, f)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint32(0)
		for pb.Next() {
			if i%2 == 0 {
				store.Get(uint32(i % 1000))
			} else {
				f := &testFriend{Name: "Bench"}
				store.Set(uint32(i%1000), f)
			}
			i++
		}
	})
}
