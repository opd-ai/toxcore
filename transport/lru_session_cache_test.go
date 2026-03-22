package transport

import (
	"fmt"
	"testing"
	"time"
)

func TestLRUSessionCache_Basic(t *testing.T) {
	cache := NewLRUSessionCache(100)

	if cache.Len() != 0 {
		t.Errorf("Expected empty cache, got %d items", cache.Len())
	}

	if cache.Capacity() != 100 {
		t.Errorf("Expected capacity 100, got %d", cache.Capacity())
	}
}

func TestLRUSessionCache_MinCapacity(t *testing.T) {
	cache := NewLRUSessionCache(10) // Below minimum

	if cache.Capacity() != MinMaxSessions {
		t.Errorf("Expected capacity %d (minimum), got %d", MinMaxSessions, cache.Capacity())
	}
}

func TestLRUSessionCache_PutGet(t *testing.T) {
	cache := NewLRUSessionCache(100)

	session := &NoiseSession{
		createdAt: time.Now(),
	}

	// Put a session
	evicted := cache.Put("addr1", session)
	if evicted != nil {
		t.Error("Should not evict on first insert")
	}

	// Get the session
	retrieved := cache.Get("addr1")
	if retrieved != session {
		t.Error("Retrieved session doesn't match inserted session")
	}

	// Get non-existent
	missing := cache.Get("addr2")
	if missing != nil {
		t.Error("Should return nil for non-existent key")
	}
}

func TestLRUSessionCache_Update(t *testing.T) {
	cache := NewLRUSessionCache(100)

	session1 := &NoiseSession{createdAt: time.Now()}
	session2 := &NoiseSession{createdAt: time.Now().Add(time.Hour)}

	cache.Put("addr1", session1)
	old := cache.Put("addr1", session2)

	if old != session1 {
		t.Error("Should return old session on update")
	}

	retrieved := cache.Get("addr1")
	if retrieved != session2 {
		t.Error("Should have updated session")
	}

	if cache.Len() != 1 {
		t.Errorf("Expected 1 session, got %d", cache.Len())
	}
}

func TestLRUSessionCache_Eviction(t *testing.T) {
	cache := NewLRUSessionCache(MinMaxSessions) // Use minimum capacity

	// Fill cache beyond capacity
	for i := 0; i < MinMaxSessions+10; i++ {
		session := &NoiseSession{createdAt: time.Now()}
		cache.Put(fmt.Sprintf("addr%d", i), session)
	}

	// Cache should not exceed capacity
	if cache.Len() > MinMaxSessions {
		t.Errorf("Cache exceeded capacity: %d > %d", cache.Len(), MinMaxSessions)
	}

	// First items should be evicted
	if cache.Get("addr0") != nil {
		t.Error("Oldest item should have been evicted")
	}

	// Last items should still be present
	if cache.Get(fmt.Sprintf("addr%d", MinMaxSessions+9)) == nil {
		t.Error("Newest item should still be present")
	}
}

func TestLRUSessionCache_LRUOrder(t *testing.T) {
	cache := NewLRUSessionCache(MinMaxSessions)

	// Add 3 sessions
	sessions := make([]*NoiseSession, 3)
	for i := 0; i < 3; i++ {
		sessions[i] = &NoiseSession{createdAt: time.Now()}
		cache.Put(fmt.Sprintf("addr%d", i), sessions[i])
	}

	// Access addr0, making it most recently used
	cache.Get("addr0")

	// Fill the cache to force eviction
	for i := 3; i < MinMaxSessions+1; i++ {
		session := &NoiseSession{createdAt: time.Now()}
		cache.Put(fmt.Sprintf("addr%d", i), session)
	}

	// addr0 should still exist (was accessed recently)
	if cache.Get("addr0") == nil {
		t.Error("Recently accessed item should not be evicted")
	}

	// addr1 should be evicted (was least recently used among original 3)
	if cache.Get("addr1") != nil {
		t.Error("Least recently used item should be evicted")
	}
}

func TestLRUSessionCache_Delete(t *testing.T) {
	cache := NewLRUSessionCache(100)

	session := &NoiseSession{createdAt: time.Now()}
	cache.Put("addr1", session)

	deleted := cache.Delete("addr1")
	if deleted != session {
		t.Error("Delete should return the deleted session")
	}

	if cache.Get("addr1") != nil {
		t.Error("Deleted session should not be retrievable")
	}

	// Delete non-existent
	if cache.Delete("addr2") != nil {
		t.Error("Delete of non-existent should return nil")
	}
}

func TestLRUSessionCache_Clear(t *testing.T) {
	cache := NewLRUSessionCache(100)

	for i := 0; i < 10; i++ {
		session := &NoiseSession{createdAt: time.Now()}
		cache.Put(fmt.Sprintf("addr%d", i), session)
	}

	cache.Clear()

	if cache.Len() != 0 {
		t.Errorf("Expected empty cache after clear, got %d items", cache.Len())
	}

	for i := 0; i < 10; i++ {
		if cache.Get(fmt.Sprintf("addr%d", i)) != nil {
			t.Error("Items should not be retrievable after clear")
		}
	}
}

func TestLRUSessionCache_Stats(t *testing.T) {
	cache := NewLRUSessionCache(100)

	session := &NoiseSession{createdAt: time.Now()}
	cache.Put("addr1", session)

	// Hit
	cache.Get("addr1")
	// Miss
	cache.Get("addr2")
	cache.Get("addr3")

	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 2 {
		t.Errorf("Expected 2 misses, got %d", stats.Misses)
	}
	if stats.Size != 1 {
		t.Errorf("Expected size 1, got %d", stats.Size)
	}
}

func TestLRUSessionCache_HitRate(t *testing.T) {
	stats := LRUCacheStats{
		Hits:   75,
		Misses: 25,
	}

	hitRate := stats.HitRate()
	if hitRate != 75.0 {
		t.Errorf("Expected hit rate 75.0, got %f", hitRate)
	}

	// Zero case
	zeroStats := LRUCacheStats{}
	if zeroStats.HitRate() != 0 {
		t.Error("Expected 0 hit rate for empty stats")
	}
}

func TestLRUSessionCache_Range(t *testing.T) {
	cache := NewLRUSessionCache(100)

	for i := 0; i < 5; i++ {
		session := &NoiseSession{createdAt: time.Now()}
		cache.Put(fmt.Sprintf("addr%d", i), session)
	}

	// Access to change order: addr0 becomes most recent
	cache.Get("addr0")

	// Collect keys in order
	var keys []string
	cache.Range(func(key string, _ *NoiseSession) bool {
		keys = append(keys, key)
		return true
	})

	// First should be addr0 (most recently accessed)
	if len(keys) == 0 || keys[0] != "addr0" {
		t.Errorf("Expected addr0 first, got %v", keys)
	}
}

func TestLRUSessionCache_RangeStop(t *testing.T) {
	cache := NewLRUSessionCache(100)

	for i := 0; i < 10; i++ {
		session := &NoiseSession{createdAt: time.Now()}
		cache.Put(fmt.Sprintf("addr%d", i), session)
	}

	count := 0
	cache.Range(func(_ string, _ *NoiseSession) bool {
		count++
		return count < 3 // Stop after 3
	})

	if count != 3 {
		t.Errorf("Expected to process 3 items, processed %d", count)
	}
}

func TestLRUSessionCache_Oldest(t *testing.T) {
	cache := NewLRUSessionCache(100)

	// Empty cache
	if cache.Oldest() != nil {
		t.Error("Oldest should return nil for empty cache")
	}

	session1 := &NoiseSession{createdAt: time.Now()}
	session2 := &NoiseSession{createdAt: time.Now().Add(time.Hour)}

	cache.Put("addr1", session1)
	cache.Put("addr2", session2)

	oldest := cache.Oldest()
	if oldest != session1 {
		t.Error("Oldest should return first inserted session")
	}
}

func TestLRUSessionCache_Touch(t *testing.T) {
	cache := NewLRUSessionCache(MinMaxSessions)

	session1 := &NoiseSession{createdAt: time.Now()}
	session2 := &NoiseSession{createdAt: time.Now()}

	cache.Put("addr1", session1)
	cache.Put("addr2", session2)

	// Touch non-existent
	if cache.Touch("addr3") {
		t.Error("Touch should return false for non-existent key")
	}

	// Touch addr1 to make it most recent
	if !cache.Touch("addr1") {
		t.Error("Touch should return true for existing key")
	}

	// Fill cache to force eviction
	for i := 3; i < MinMaxSessions+2; i++ {
		session := &NoiseSession{createdAt: time.Now()}
		cache.Put(fmt.Sprintf("addr%d", i), session)
	}

	// addr1 should still exist (was touched)
	if cache.Get("addr1") == nil {
		t.Error("Touched item should not be evicted")
	}

	// addr2 should be evicted
	if cache.Get("addr2") != nil {
		t.Error("Non-touched item should be evicted")
	}
}

func TestLRUSessionCache_EvictionStats(t *testing.T) {
	cache := NewLRUSessionCache(MinMaxSessions)

	// Fill to capacity
	for i := 0; i < MinMaxSessions; i++ {
		session := &NoiseSession{createdAt: time.Now()}
		cache.Put(fmt.Sprintf("addr%d", i), session)
	}

	stats := cache.Stats()
	if stats.Evictions != 0 {
		t.Errorf("Expected 0 evictions when filling to capacity, got %d", stats.Evictions)
	}

	// Add more to trigger evictions
	for i := MinMaxSessions; i < MinMaxSessions+10; i++ {
		session := &NoiseSession{createdAt: time.Now()}
		cache.Put(fmt.Sprintf("addr%d", i), session)
	}

	stats = cache.Stats()
	if stats.Evictions != 10 {
		t.Errorf("Expected 10 evictions, got %d", stats.Evictions)
	}
}

func BenchmarkLRUSessionCache_Get(b *testing.B) {
	cache := NewLRUSessionCache(10000)

	// Pre-populate
	for i := 0; i < 5000; i++ {
		session := &NoiseSession{createdAt: time.Now()}
		cache.Put(fmt.Sprintf("addr%d", i), session)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(fmt.Sprintf("addr%d", i%5000))
	}
}

func BenchmarkLRUSessionCache_Put(b *testing.B) {
	cache := NewLRUSessionCache(10000)

	sessions := make([]*NoiseSession, b.N)
	for i := 0; i < b.N; i++ {
		sessions[i] = &NoiseSession{createdAt: time.Now()}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(fmt.Sprintf("addr%d", i%10000), sessions[i])
	}
}

func BenchmarkLRUSessionCache_PutWithEviction(b *testing.B) {
	cache := NewLRUSessionCache(MinMaxSessions) // Small cache to force evictions

	sessions := make([]*NoiseSession, b.N)
	for i := 0; i < b.N; i++ {
		sessions[i] = &NoiseSession{createdAt: time.Now()}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(fmt.Sprintf("addr%d", i), sessions[i]) // Unique keys force evictions
	}
}
