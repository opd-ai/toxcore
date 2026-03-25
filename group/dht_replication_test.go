package group

import (
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/dht"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplicationManagerBasic(t *testing.T) {
	rm := NewReplicationManager(nil, nil)

	assert.NotNil(t, rm)
	assert.Equal(t, DefaultReplicationFactor, rm.ReplicationFactor())
}

func TestReplicationFactorConfig(t *testing.T) {
	rm := NewReplicationManager(nil, nil)

	rm.SetReplicationFactor(7)
	assert.Equal(t, 7, rm.ReplicationFactor())

	rm.SetReplicationFactor(0)
	assert.Equal(t, 1, rm.ReplicationFactor())

	rm.SetReplicationFactor(100)
	assert.Equal(t, 20, rm.ReplicationFactor())
}

func TestReplicateAnnouncementNoRouting(t *testing.T) {
	rm := NewReplicationManager(nil, nil)

	announcement := &dht.GroupAnnouncement{
		GroupID:   123,
		Name:      "Test Group",
		Type:      1,
		Privacy:   0,
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	count, err := rm.ReplicateAnnouncement(announcement)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "routing table not set")
	assert.Equal(t, 0, count)
}

func TestGroupIDToToxID(t *testing.T) {
	rm := NewReplicationManager(nil, nil)

	id1 := rm.groupIDToToxID(123)
	id2 := rm.groupIDToToxID(456)
	id3 := rm.groupIDToToxID(123)

	assert.NotEqual(t, id1.PublicKey, id2.PublicKey)
	assert.Equal(t, id1.PublicKey, id3.PublicKey)

	assert.NotEqual(t, id1.PublicKey, [32]byte{})
}

func TestReplicaMetadataTracking(t *testing.T) {
	rm := NewReplicationManager(nil, nil)

	announcement := &dht.GroupAnnouncement{
		GroupID:   999,
		Name:      "Metadata Test",
		Type:      1,
		Privacy:   0,
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	replicaKeys := [][32]byte{
		{1, 2, 3},
		{4, 5, 6},
		{7, 8, 9},
	}

	rm.storeReplicationMetadata(announcement, replicaKeys)

	assert.Equal(t, 3, rm.GetReplicaCount(999))
	assert.True(t, rm.IsAvailable(999))
	assert.Equal(t, 0, rm.GetReplicaCount(888))
	assert.False(t, rm.IsAvailable(888))
}

func TestReplicaAvailabilityThreshold(t *testing.T) {
	rm := NewReplicationManager(nil, nil)

	announcement := &dht.GroupAnnouncement{
		GroupID:   100,
		Name:      "Threshold Test",
		Type:      1,
		Privacy:   0,
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}

	rm.storeReplicationMetadata(announcement, [][32]byte{{1}})
	assert.False(t, rm.IsAvailable(100))

	rm.storeReplicationMetadata(announcement, [][32]byte{{1}, {2}})
	assert.True(t, rm.IsAvailable(100))

	rm.storeReplicationMetadata(announcement, [][32]byte{{1}, {2}, {3}, {4}, {5}})
	assert.True(t, rm.IsAvailable(100))
}

func TestCleanupExpired(t *testing.T) {
	rm := NewReplicationManager(nil, nil)

	fresh := &dht.GroupAnnouncement{
		GroupID:   1,
		Name:      "Fresh",
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}
	rm.storeReplicationMetadata(fresh, [][32]byte{{1}, {2}})

	expired := &dht.GroupAnnouncement{
		GroupID:   2,
		Name:      "Expired",
		Timestamp: time.Now().Add(-48 * time.Hour),
		TTL:       24 * time.Hour,
	}
	rm.storeReplicationMetadata(expired, [][32]byte{{3}, {4}})

	rm.CleanupExpired()

	assert.Equal(t, 2, rm.GetReplicaCount(1))
	assert.Equal(t, 0, rm.GetReplicaCount(2))
}

func TestBuildQueryPacket(t *testing.T) {
	rm := NewReplicationManager(nil, nil)

	packet := rm.buildQueryPacket(0x12345678)

	require.Len(t, packet.Data, 4)
	assert.Equal(t, byte(0x12), packet.Data[0])
	assert.Equal(t, byte(0x34), packet.Data[1])
	assert.Equal(t, byte(0x56), packet.Data[2])
	assert.Equal(t, byte(0x78), packet.Data[3])
}

func TestReplicationManagerConcurrency(t *testing.T) {
	rm := NewReplicationManager(nil, nil)

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			announcement := &dht.GroupAnnouncement{
				GroupID:   uint32(id),
				Name:      "Concurrent",
				Timestamp: time.Now(),
				TTL:       24 * time.Hour,
			}
			rm.storeReplicationMetadata(announcement, [][32]byte{{byte(id)}, {byte(id + 1)}})
		}(i)
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = rm.GetReplicaCount(uint32(id))
			_ = rm.IsAvailable(uint32(id))
		}(i)
	}

	wg.Wait()
}

func TestRefreshReplicasNotExists(t *testing.T) {
	rm := NewReplicationManager(nil, nil)

	count, err := rm.RefreshReplicas(999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no replication metadata")
	assert.Equal(t, 0, count)
}

func TestQueryWithRedundancyNoRouting(t *testing.T) {
	rm := NewReplicationManager(nil, nil)

	result, err := rm.QueryWithRedundancy(123, 5*time.Second)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "routing table not set")
}

func BenchmarkGroupIDToToxID(b *testing.B) {
	rm := NewReplicationManager(nil, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rm.groupIDToToxID(uint32(i))
	}
}

func BenchmarkStoreMetadata(b *testing.B) {
	rm := NewReplicationManager(nil, nil)
	keys := [][32]byte{{1}, {2}, {3}, {4}, {5}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		announcement := &dht.GroupAnnouncement{
			GroupID:   uint32(i),
			Name:      "Benchmark",
			Timestamp: time.Now(),
			TTL:       24 * time.Hour,
		}
		rm.storeReplicationMetadata(announcement, keys)
	}
}
