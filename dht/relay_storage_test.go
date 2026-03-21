package dht

import (
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRelayStorage(t *testing.T) {
	rs := NewRelayStorage()
	require.NotNil(t, rs)
	assert.Empty(t, rs.GetAllAnnouncements())
}

func TestRelayStorage_StoreAndGet(t *testing.T) {
	rs := NewRelayStorage()

	announcement := &RelayAnnouncement{
		PublicKey: [32]byte{1, 2, 3, 4},
		Address:   "relay.example.com",
		Port:      33445,
		Priority:  1,
		Timestamp: time.Now(),
		TTL:       time.Hour,
		Capacity:  1000,
		Load:      50,
	}

	rs.StoreAnnouncement(announcement)

	retrieved, found := rs.GetAnnouncement(announcement.PublicKey)
	require.True(t, found)
	assert.Equal(t, announcement.Address, retrieved.Address)
	assert.Equal(t, announcement.Port, retrieved.Port)
}

func TestRelayStorage_ExpiredAnnouncement(t *testing.T) {
	rs := NewRelayStorage()

	announcement := &RelayAnnouncement{
		PublicKey: [32]byte{1, 2, 3, 4},
		Address:   "relay.example.com",
		Port:      33445,
		Priority:  1,
		Timestamp: time.Now().Add(-2 * time.Hour),
		TTL:       time.Hour, // Expired 1 hour ago
		Capacity:  1000,
		Load:      50,
	}

	rs.StoreAnnouncement(announcement)

	_, found := rs.GetAnnouncement(announcement.PublicKey)
	assert.False(t, found, "Expired announcement should not be returned")
}

func TestRelayStorage_GetAllAnnouncements(t *testing.T) {
	rs := NewRelayStorage()

	announcement1 := &RelayAnnouncement{
		PublicKey: [32]byte{1},
		Address:   "relay1.example.com",
		Port:      33445,
		Timestamp: time.Now(),
		TTL:       time.Hour,
	}

	announcement2 := &RelayAnnouncement{
		PublicKey: [32]byte{2},
		Address:   "relay2.example.com",
		Port:      33446,
		Timestamp: time.Now(),
		TTL:       time.Hour,
	}

	rs.StoreAnnouncement(announcement1)
	rs.StoreAnnouncement(announcement2)

	all := rs.GetAllAnnouncements()
	assert.Len(t, all, 2)
}

func TestRelayStorage_CleanExpired(t *testing.T) {
	rs := NewRelayStorage()

	validAnnouncement := &RelayAnnouncement{
		PublicKey: [32]byte{1},
		Address:   "valid.example.com",
		Port:      33445,
		Timestamp: time.Now(),
		TTL:       time.Hour,
	}

	expiredAnnouncement := &RelayAnnouncement{
		PublicKey: [32]byte{2},
		Address:   "expired.example.com",
		Port:      33446,
		Timestamp: time.Now().Add(-2 * time.Hour),
		TTL:       time.Hour,
	}

	rs.StoreAnnouncement(validAnnouncement)
	rs.StoreAnnouncement(expiredAnnouncement)

	rs.CleanExpired()

	all := rs.GetAllAnnouncements()
	assert.Len(t, all, 1)
	assert.Equal(t, "valid.example.com", all[0].Address)
}

func TestRelayAnnouncement_Serialization(t *testing.T) {
	original := &RelayAnnouncement{
		PublicKey: [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
		Address:   "relay.example.com",
		Port:      33445,
		Priority:  5,
		Timestamp: time.Unix(1234567890, 0),
		TTL:       24 * time.Hour,
		Capacity:  5000,
		Load:      75,
	}

	data, err := SerializeRelayAnnouncement(original)
	require.NoError(t, err)

	deserialized, err := DeserializeRelayAnnouncement(data)
	require.NoError(t, err)

	assert.Equal(t, original.PublicKey, deserialized.PublicKey)
	assert.Equal(t, original.Address, deserialized.Address)
	assert.Equal(t, original.Port, deserialized.Port)
	assert.Equal(t, original.Priority, deserialized.Priority)
	assert.Equal(t, original.Timestamp.Unix(), deserialized.Timestamp.Unix())
	assert.Equal(t, original.Capacity, deserialized.Capacity)
	assert.Equal(t, original.Load, deserialized.Load)
}

func TestDeserializeRelayAnnouncement_TooShort(t *testing.T) {
	data := make([]byte, 10) // Too short
	_, err := DeserializeRelayAnnouncement(data)
	assert.Error(t, err)
}

func TestRelayAnnouncement_ToTransportServerInfo(t *testing.T) {
	announcement := &RelayAnnouncement{
		PublicKey: [32]byte{1, 2, 3, 4},
		Address:   "relay.example.com",
		Port:      33445,
		Priority:  3,
	}

	info := announcement.ToTransportServerInfo()

	assert.Equal(t, announcement.Address, info.Address)
	assert.Equal(t, announcement.PublicKey, info.PublicKey)
	assert.Equal(t, announcement.Port, info.Port)
	assert.Equal(t, announcement.Priority, info.Priority)
}

func TestRelayStorage_ResponseCallback(t *testing.T) {
	rs := NewRelayStorage()

	var receivedAnnouncement *RelayAnnouncement
	rs.SetResponseCallback(func(announcement *RelayAnnouncement) {
		receivedAnnouncement = announcement
	})

	announcement := &RelayAnnouncement{
		PublicKey: [32]byte{1},
		Address:   "relay.example.com",
		Port:      33445,
		Timestamp: time.Now(),
		TTL:       time.Hour,
	}

	rs.notifyResponse(announcement)

	assert.NotNil(t, receivedAnnouncement)
	assert.Equal(t, announcement.Address, receivedAnnouncement.Address)
}

func TestRelayStorage_QueryRegistration(t *testing.T) {
	rs := NewRelayStorage()

	queryID, ch := rs.registerQuery()
	assert.NotNil(t, ch)
	assert.Greater(t, queryID, uint64(0))

	// Test that we can send to the channel
	testRelays := []*RelayAnnouncement{{Address: "test.com"}}
	rs.notifyQueryResponse(queryID, testRelays)

	select {
	case received := <-ch:
		assert.Len(t, received, 1)
		assert.Equal(t, "test.com", received[0].Address)
	default:
		t.Fatal("Expected to receive relays on channel")
	}

	// Deregister and verify cleanup
	rs.deregisterQuery(queryID, ch)

	rs.pendingMu.Lock()
	_, exists := rs.pendingQueries[queryID]
	rs.pendingMu.Unlock()
	assert.False(t, exists, "Query should be deregistered")
}

func TestRoutingTable_RelayStorageIntegration(t *testing.T) {
	// Test that RoutingTable properly initializes relay storage
	var publicKey [32]byte
	toxID := crypto.NewToxID(publicKey, [4]byte{})
	rt := NewRoutingTable(*toxID, 8)
	require.NotNil(t, rt)
	require.NotNil(t, rt.GetRelayStorage())
}

func TestRoutingTable_GetLocalRelays(t *testing.T) {
	var publicKey [32]byte
	toxID := crypto.NewToxID(publicKey, [4]byte{})
	rt := NewRoutingTable(*toxID, 8)

	// Initially no relays
	relays := rt.getLocalRelays()
	assert.Empty(t, relays)

	// Add a relay
	announcement := &RelayAnnouncement{
		PublicKey: [32]byte{1},
		Address:   "relay.example.com",
		Port:      33445,
		Timestamp: time.Now(),
		TTL:       time.Hour,
	}
	rt.relayStorage.StoreAnnouncement(announcement)

	relays = rt.getLocalRelays()
	assert.Len(t, relays, 1)
}
