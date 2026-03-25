package async

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStorageNodeAnnouncement tests the basic announcement data structure.
func TestStorageNodeAnnouncement(t *testing.T) {
	ann := &StorageNodeAnnouncement{
		PublicKey: [32]byte{1, 2, 3},
		Address:   "192.168.1.100",
		Port:      33445,
		Capacity:  10000,
		Load:      25,
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	assert.False(t, ann.IsExpired())

	// Test expiration
	ann.Timestamp = time.Now().Add(-25 * time.Hour)
	assert.True(t, ann.IsExpired())
}

// TestStorageNodeDiscoveryBasic tests basic discovery operations.
func TestStorageNodeDiscoveryBasic(t *testing.T) {
	sd := NewStorageNodeDiscovery()
	require.NotNil(t, sd)

	assert.Equal(t, 0, sd.Count())
	assert.True(t, sd.NeedsDiscovery()) // Initially needs discovery

	// Store an announcement
	ann := &StorageNodeAnnouncement{
		PublicKey: [32]byte{1, 2, 3},
		Address:   "10.0.0.1",
		Port:      33445,
		Capacity:  5000,
		Load:      10,
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	isNew := sd.StoreAnnouncement(ann)
	assert.True(t, isNew)
	assert.Equal(t, 1, sd.Count())

	// Store same announcement again
	isNew = sd.StoreAnnouncement(ann)
	assert.False(t, isNew)

	// Get active nodes
	active := sd.GetActiveNodes()
	assert.Len(t, active, 1)
	assert.Equal(t, ann.Address, active[0].Address)
}

// TestStorageNodeDiscoveryCallback tests the discovery callback.
func TestStorageNodeDiscoveryCallback(t *testing.T) {
	sd := NewStorageNodeDiscovery()

	var discoveredNode *StorageNodeAnnouncement
	var mu sync.Mutex
	done := make(chan struct{})

	sd.OnNodeDiscovered(func(ann *StorageNodeAnnouncement) {
		mu.Lock()
		discoveredNode = ann
		mu.Unlock()
		close(done)
	})

	ann := &StorageNodeAnnouncement{
		PublicKey: [32]byte{5, 6, 7},
		Address:   "192.168.1.50",
		Port:      33446,
		Timestamp: time.Now(),
		TTL:       time.Hour,
	}

	sd.StoreAnnouncement(ann)

	// Wait for callback with timeout
	select {
	case <-done:
		// Callback completed
	case <-time.After(time.Second):
		t.Fatal("Callback not invoked within timeout")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.NotNil(t, discoveredNode)
	assert.Equal(t, ann.Address, discoveredNode.Address)
}

// TestStorageNodeDiscoverySelfIgnored tests that self announcements are ignored.
func TestStorageNodeDiscoverySelfIgnored(t *testing.T) {
	sd := NewStorageNodeDiscovery()

	selfPK := [32]byte{10, 20, 30}
	sd.SetSelfAsStorageNode(selfPK, "127.0.0.1", 33445, 10000)

	ann := &StorageNodeAnnouncement{
		PublicKey: selfPK, // Same as self
		Address:   "127.0.0.1",
		Port:      33445,
		Timestamp: time.Now(),
		TTL:       time.Hour,
	}

	isNew := sd.StoreAnnouncement(ann)
	assert.False(t, isNew, "Self announcement should be ignored")
	assert.Equal(t, 0, sd.Count())
}

// TestStorageNodeDiscoveryExpiration tests cleanup of expired announcements.
func TestStorageNodeDiscoveryExpiration(t *testing.T) {
	sd := NewStorageNodeDiscovery()

	// Add an already-expired announcement
	ann := &StorageNodeAnnouncement{
		PublicKey: [32]byte{1},
		Address:   "expired.node",
		Port:      1234,
		Timestamp: time.Now().Add(-2 * time.Hour),
		TTL:       time.Hour, // Already expired
	}

	sd.StoreAnnouncement(ann)
	assert.Equal(t, 1, sd.Count())

	// Active nodes should not include expired
	active := sd.GetActiveNodes()
	assert.Len(t, active, 0)

	// Clean up should remove expired
	removed := sd.CleanExpired()
	assert.Equal(t, 1, removed)
	assert.Equal(t, 0, sd.Count())
}

// TestStorageNodeDiscoveryLoadFilter tests filtering by load.
func TestStorageNodeDiscoveryLoadFilter(t *testing.T) {
	sd := NewStorageNodeDiscovery()

	// Add nodes with different load levels
	for i := 0; i < 5; i++ {
		ann := &StorageNodeAnnouncement{
			PublicKey: [32]byte{byte(i)},
			Address:   "node",
			Port:      uint16(33445 + i),
			Load:      uint8(i * 20), // 0, 20, 40, 60, 80
			Timestamp: time.Now(),
			TTL:       time.Hour,
		}
		sd.StoreAnnouncement(ann)
	}

	assert.Equal(t, 5, sd.Count())

	// Filter by max load 50%
	filtered := sd.GetActiveNodesByLoad(50)
	assert.Len(t, filtered, 3) // 0, 20, 40
}

// TestAnnouncementSerialization tests JSON serialization.
func TestAnnouncementSerialization(t *testing.T) {
	ann := &StorageNodeAnnouncement{
		PublicKey: [32]byte{0xAA, 0xBB},
		Address:   "test.node.local",
		Port:      33445,
		Capacity:  50000,
		Load:      30,
		Timestamp: time.Now(),
		TTL:       12 * time.Hour,
	}

	data, err := SerializeAnnouncement(ann)
	require.NoError(t, err)

	recovered, err := DeserializeAnnouncement(data)
	require.NoError(t, err)

	assert.Equal(t, ann.PublicKey, recovered.PublicKey)
	assert.Equal(t, ann.Address, recovered.Address)
	assert.Equal(t, ann.Port, recovered.Port)
	assert.Equal(t, ann.Capacity, recovered.Capacity)
	assert.Equal(t, ann.Load, recovered.Load)
}

// TestAnnouncementBinarySerialization tests binary serialization.
func TestAnnouncementBinarySerialization(t *testing.T) {
	ann := &StorageNodeAnnouncement{
		PublicKey: [32]byte{0x11, 0x22, 0x33},
		Address:   "binary.test.node",
		Port:      44556,
		Capacity:  75000,
		Load:      45,
		Timestamp: time.Now(),
		TTL:       6 * time.Hour,
	}

	data := ann.SerializeBinary()
	require.NotEmpty(t, data)

	recovered, err := DeserializeAnnouncementBinary(data)
	require.NoError(t, err)

	assert.Equal(t, ann.PublicKey, recovered.PublicKey)
	assert.Equal(t, ann.Address, recovered.Address)
	assert.Equal(t, ann.Port, recovered.Port)
	assert.Equal(t, ann.Capacity, recovered.Capacity)
	assert.Equal(t, ann.Load, recovered.Load)
}

// TestStorageNodeKeyGeneration tests DHT key generation.
func TestStorageNodeKeyGeneration(t *testing.T) {
	pk := [32]byte{1, 2, 3, 4, 5, 6, 7, 8}
	key := GenerateStorageNodeKey(pk)

	// Verify prefix
	assert.Equal(t, StorageNodeKeyPrefix[:], key[:8])

	// Verify key derivation is deterministic
	key2 := GenerateStorageNodeKey(pk)
	assert.Equal(t, key, key2)

	// Different public key should produce different key
	pk2 := [32]byte{9, 10, 11, 12, 13, 14, 15, 16}
	key3 := GenerateStorageNodeKey(pk2)
	assert.NotEqual(t, key, key3)
}

// TestCreateSelfAnnouncement tests self-announcement creation.
func TestCreateSelfAnnouncement(t *testing.T) {
	sd := NewStorageNodeDiscovery()

	// Not a storage node yet
	ann := sd.CreateSelfAnnouncement(50)
	assert.Nil(t, ann)

	// Configure as storage node
	selfPK := [32]byte{0xDE, 0xAD, 0xBE, 0xEF}
	sd.SetSelfAsStorageNode(selfPK, "my.storage.node", 33445, 100000)

	ann = sd.CreateSelfAnnouncement(50)
	require.NotNil(t, ann)
	assert.Equal(t, selfPK, ann.PublicKey)
	assert.Equal(t, "my.storage.node", ann.Address)
	assert.Equal(t, uint16(33445), ann.Port)
	assert.Equal(t, uint32(100000), ann.Capacity)
	assert.Equal(t, uint8(50), ann.Load)
}
