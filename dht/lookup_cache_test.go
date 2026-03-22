package dht

import (
	"fmt"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// TestLookupCache tests the LookupCache TTL and eviction behavior.
func TestLookupCache(t *testing.T) {
	t.Run("basic get/put", func(t *testing.T) {
		cache := NewLookupCache(10*time.Second, 10)

		key := [32]byte{1, 2, 3}
		nodes := []*Node{
			{PublicKey: [32]byte{4, 5, 6}},
			{PublicKey: [32]byte{7, 8, 9}},
		}

		// Initially empty
		result := cache.Get(key)
		if result != nil {
			t.Error("Expected nil for non-existent key")
		}

		// Put and get
		cache.Put(key, nodes)
		result = cache.Get(key)
		if len(result) != 2 {
			t.Errorf("Expected 2 nodes, got %d", len(result))
		}
	})

	t.Run("TTL expiration", func(t *testing.T) {
		cache := NewLookupCache(50*time.Millisecond, 10)

		key := [32]byte{1, 2, 3}
		nodes := []*Node{{PublicKey: [32]byte{4, 5, 6}}}

		cache.Put(key, nodes)

		// Should be available immediately
		if cache.Get(key) == nil {
			t.Error("Expected cached result")
		}

		// Wait for TTL to expire
		time.Sleep(100 * time.Millisecond)

		// Should be expired
		if cache.Get(key) != nil {
			t.Error("Expected nil after TTL expiration")
		}
	})

	t.Run("max size eviction", func(t *testing.T) {
		cache := NewLookupCache(10*time.Second, 3)

		// Add 4 entries (exceeds max size of 3)
		for i := 0; i < 4; i++ {
			key := [32]byte{byte(i)}
			cache.Put(key, []*Node{})
		}

		// Should only have 3 entries
		if cache.Size() > 3 {
			t.Errorf("Expected max 3 entries, got %d", cache.Size())
		}
	})

	t.Run("cache statistics", func(t *testing.T) {
		cache := NewLookupCache(10*time.Second, 10)

		key := [32]byte{1}
		cache.Put(key, []*Node{})

		// Miss
		cache.Get([32]byte{2})

		// Hit
		cache.Get(key)

		hits, misses := cache.Stats()
		if hits != 1 {
			t.Errorf("Expected 1 hit, got %d", hits)
		}
		if misses != 1 {
			t.Errorf("Expected 1 miss, got %d", misses)
		}
	})

	t.Run("clear cache", func(t *testing.T) {
		cache := NewLookupCache(10*time.Second, 10)

		for i := 0; i < 5; i++ {
			cache.Put([32]byte{byte(i)}, []*Node{})
		}

		if cache.Size() != 5 {
			t.Errorf("Expected 5 entries, got %d", cache.Size())
		}

		cache.Clear()

		if cache.Size() != 0 {
			t.Errorf("Expected 0 entries after clear, got %d", cache.Size())
		}
	})
}

// mockAddrCache implements net.Addr for testing
type mockAddrCache struct {
	addr string
}

func (m *mockAddrCache) Network() string { return "udp" }
func (m *mockAddrCache) String() string  { return m.addr }

// TestFindClosestNodesCache tests that FindClosestNodes uses caching.
func TestFindClosestNodesCache(t *testing.T) {
	selfID := crypto.NewToxID([32]byte{1}, [4]byte{})
	rt := NewRoutingTable(*selfID, 8)

	// Add some nodes
	for i := 0; i < 10; i++ {
		nodeID := crypto.NewToxID([32]byte{byte(i + 10)}, [4]byte{})
		node := NewNode(*nodeID, &mockAddrCache{fmt.Sprintf("127.0.0.1:%d", 8000+i)})
		rt.AddNode(node)
	}

	// Clear cache after adding nodes
	rt.ClearLookupCache()

	targetID := crypto.NewToxID([32]byte{20}, [4]byte{})

	// First call should miss cache
	rt.FindClosestNodes(*targetID, 5)
	hits1, misses1 := rt.GetLookupCacheStats()

	// Second call should hit cache
	rt.FindClosestNodes(*targetID, 5)
	hits2, misses2 := rt.GetLookupCacheStats()

	if hits2 <= hits1 {
		t.Error("Expected cache hit on second call")
	}

	if misses2 != misses1 {
		t.Error("Expected no additional cache miss on second call")
	}
}

// TestRoutingTableCacheInvalidation tests that cache is invalidated when nodes are added.
func TestRoutingTableCacheInvalidation(t *testing.T) {
	selfID := crypto.NewToxID([32]byte{1}, [4]byte{})
	rt := NewRoutingTable(*selfID, 8)

	// Add initial node
	nodeID := crypto.NewToxID([32]byte{10}, [4]byte{})
	node := NewNode(*nodeID, &mockAddrCache{"127.0.0.1:8000"})
	rt.AddNode(node)

	// Clear cache
	rt.ClearLookupCache()

	// Perform a lookup to populate cache
	targetID := crypto.NewToxID([32]byte{15}, [4]byte{})
	rt.FindClosestNodes(*targetID, 5)

	// Cache should have 1 entry
	if rt.lookupCache.Size() != 1 {
		t.Errorf("Expected 1 cache entry, got %d", rt.lookupCache.Size())
	}

	// Add another node - should invalidate cache
	nodeID2 := crypto.NewToxID([32]byte{20}, [4]byte{})
	node2 := NewNode(*nodeID2, &mockAddrCache{"127.0.0.1:8001"})
	rt.AddNode(node2)

	// Cache should be cleared
	if rt.lookupCache.Size() != 0 {
		t.Errorf("Expected cache to be cleared after AddNode, got %d entries", rt.lookupCache.Size())
	}
}
