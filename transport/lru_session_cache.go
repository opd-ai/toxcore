package transport

import (
	"container/list"
	"sync"
)

const (
	// DefaultMaxSessions is the default maximum number of Noise sessions to cache.
	DefaultMaxSessions = 10000

	// MinMaxSessions is the minimum allowed value for max sessions.
	MinMaxSessions = 100
)

// LRUSessionCache provides an LRU cache for Noise sessions with bounded memory usage.
// When the cache is full, the least recently used session is evicted.
type LRUSessionCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*list.Element // addr string -> list element
	order    *list.List               // Front = most recently used, Back = least recently used

	// Statistics
	hits      uint64
	misses    uint64
	evictions uint64
}

// sessionEntry wraps a session with its key for the LRU list.
type sessionEntry struct {
	key     string
	session *NoiseSession
}

// NewLRUSessionCache creates a new LRU session cache with the specified capacity.
func NewLRUSessionCache(maxSessions int) *LRUSessionCache {
	if maxSessions < MinMaxSessions {
		maxSessions = MinMaxSessions
	}
	return &LRUSessionCache{
		capacity: maxSessions,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

// Get retrieves a session from the cache and marks it as recently used.
// Returns nil if not found.
func (c *LRUSessionCache) Get(addrKey string) *NoiseSession {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.items[addrKey]
	if !exists {
		c.misses++
		return nil
	}

	c.hits++
	// Move to front (most recently used)
	c.order.MoveToFront(elem)
	return elem.Value.(*sessionEntry).session
}

// Put adds or updates a session in the cache.
// If the cache is full, the least recently used session is evicted.
func (c *LRUSessionCache) Put(addrKey string, session *NoiseSession) *NoiseSession {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists - update and move to front
	if elem, exists := c.items[addrKey]; exists {
		c.order.MoveToFront(elem)
		entry := elem.Value.(*sessionEntry)
		oldSession := entry.session
		entry.session = session
		return oldSession
	}

	// Evict if at capacity
	var evicted *NoiseSession
	if c.order.Len() >= c.capacity {
		evicted = c.evictOldest()
	}

	// Add new entry at front
	entry := &sessionEntry{key: addrKey, session: session}
	elem := c.order.PushFront(entry)
	c.items[addrKey] = elem

	return evicted
}

// evictOldest removes the least recently used session.
// Must be called with lock held.
func (c *LRUSessionCache) evictOldest() *NoiseSession {
	back := c.order.Back()
	if back == nil {
		return nil
	}

	entry := back.Value.(*sessionEntry)
	c.order.Remove(back)
	delete(c.items, entry.key)
	c.evictions++

	return entry.session
}

// Delete removes a session from the cache.
func (c *LRUSessionCache) Delete(addrKey string) *NoiseSession {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.items[addrKey]
	if !exists {
		return nil
	}

	entry := elem.Value.(*sessionEntry)
	c.order.Remove(elem)
	delete(c.items, entry.key)
	return entry.session
}

// Len returns the current number of sessions in the cache.
func (c *LRUSessionCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

// Capacity returns the maximum number of sessions the cache can hold.
func (c *LRUSessionCache) Capacity() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capacity
}

// Clear removes all sessions from the cache.
func (c *LRUSessionCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.order.Init()
}

// Stats returns cache statistics.
func (c *LRUSessionCache) Stats() LRUCacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return LRUCacheStats{
		Size:      c.order.Len(),
		Capacity:  c.capacity,
		Hits:      c.hits,
		Misses:    c.misses,
		Evictions: c.evictions,
	}
}

// LRUCacheStats contains statistics about the LRU cache.
type LRUCacheStats struct {
	Size      int
	Capacity  int
	Hits      uint64
	Misses    uint64
	Evictions uint64
}

// HitRate returns the cache hit rate as a percentage (0-100).
func (s LRUCacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total) * 100
}

// Range calls fn for each session in the cache (from most to least recently used).
// If fn returns false, iteration stops.
func (c *LRUSessionCache) Range(fn func(addrKey string, session *NoiseSession) bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for elem := c.order.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*sessionEntry)
		if !fn(entry.key, entry.session) {
			break
		}
	}
}

// Oldest returns the least recently used session without removing it.
// Returns nil if the cache is empty.
func (c *LRUSessionCache) Oldest() *NoiseSession {
	c.mu.RLock()
	defer c.mu.RUnlock()

	back := c.order.Back()
	if back == nil {
		return nil
	}
	return back.Value.(*sessionEntry).session
}

// Touch marks a session as recently used without retrieving it.
// Returns true if the session was found.
func (c *LRUSessionCache) Touch(addrKey string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.items[addrKey]
	if !exists {
		return false
	}

	c.order.MoveToFront(elem)
	return true
}
