package async

import (
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreKeyDHTManagerBasic(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	assert.NotNil(t, pm)
	assert.Equal(t, DefaultPreKeyReplicationFactor, pm.replicationFactor)
}

func TestPreKeyDHTSetReplicationFactor(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	pm.SetReplicationFactor(5)
	assert.Equal(t, 5, pm.replicationFactor)

	pm.SetReplicationFactor(0)
	assert.Equal(t, 1, pm.replicationFactor)

	pm.SetReplicationFactor(100)
	assert.Equal(t, 10, pm.replicationFactor)
}

func TestPreKeyDHTPublishNoNodeFinder(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	err = pm.PublishPreKeys()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node finder not set")
}

func TestPreKeyDHTRetrieveNoNodeFinder(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	var peerPK [32]byte
	_, err = pm.RetrievePreKeys(peerPK)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node finder not set")
}

func TestPreKeyDHTBundleSerialization(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	bundle := &PreKeyDHTBundle{
		OwnerPK: keyPair.Public,
		PreKeys: []PreKeyForExchange{
			{ID: 1, PublicKey: [32]byte{1, 2, 3}},
			{ID: 2, PublicKey: [32]byte{4, 5, 6}},
		},
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Version:   1,
	}

	data, err := pm.serializeBundle(bundle)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	parsed, err := pm.deserializeBundle(data)
	require.NoError(t, err)

	assert.Equal(t, bundle.OwnerPK, parsed.OwnerPK)
	assert.Len(t, parsed.PreKeys, 2)
	assert.Equal(t, bundle.Version, parsed.Version)
}

func TestPreKeyDHTBundleSigningData(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	now := time.Now()
	signingPK := crypto.GetSignaturePublicKey(keyPair.Private)
	bundle := &PreKeyDHTBundle{
		OwnerPK:   keyPair.Public,
		SigningPK: signingPK,
		Timestamp: now,
		ExpiresAt: now.Add(24 * time.Hour),
		Version:   42,
	}

	data := pm.bundleDataForSigning(bundle)
	// Updated: 32 (ownerPK) + 32 (signingPK) + 8 (timestamp) + 8 (expiresAt) + 4 (version) = 84 bytes
	require.Len(t, data, 84)

	assert.Equal(t, bundle.OwnerPK[:], data[0:32])
	assert.Equal(t, bundle.SigningPK[:], data[32:64])
}

func TestPreKeyDHTCaching(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	var peerPK [32]byte
	peerPK[0] = 1

	bundle := &PreKeyDHTBundle{
		OwnerPK:   peerPK,
		PreKeys:   []PreKeyForExchange{{ID: 1}},
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Version:   1,
	}

	pm.mu.Lock()
	pm.localCache[peerPK] = bundle
	pm.mu.Unlock()

	cached, exists := pm.GetCachedBundle(peerPK)
	assert.True(t, exists)
	assert.Equal(t, bundle, cached)

	var unknownPK [32]byte
	unknownPK[0] = 99
	_, exists = pm.GetCachedBundle(unknownPK)
	assert.False(t, exists)
}

func TestPreKeyDHTCacheExpiration(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	var peerPK [32]byte
	peerPK[0] = 1

	expiredBundle := &PreKeyDHTBundle{
		OwnerPK:   peerPK,
		PreKeys:   []PreKeyForExchange{{ID: 1}},
		Timestamp: time.Now().Add(-48 * time.Hour),
		ExpiresAt: time.Now().Add(-24 * time.Hour),
		Version:   1,
	}

	pm.mu.Lock()
	pm.localCache[peerPK] = expiredBundle
	pm.mu.Unlock()

	_, exists := pm.GetCachedBundle(peerPK)
	assert.False(t, exists)
}

func TestPreKeyDHTClearCache(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	var peerPK [32]byte
	peerPK[0] = 1

	bundle := &PreKeyDHTBundle{
		OwnerPK:   peerPK,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	pm.mu.Lock()
	pm.localCache[peerPK] = bundle
	pm.mu.Unlock()

	pm.ClearCache()

	_, exists := pm.GetCachedBundle(peerPK)
	assert.False(t, exists)
}

func TestPreKeyDHTNeedsRefresh(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	assert.True(t, pm.NeedsRefresh())

	pm.mu.Lock()
	pm.publishedAt = time.Now()
	pm.mu.Unlock()
	assert.False(t, pm.NeedsRefresh())

	pm.mu.Lock()
	pm.publishedAt = time.Now().Add(-25 * time.Hour)
	pm.mu.Unlock()
	assert.True(t, pm.NeedsRefresh())
}

func TestPreKeyDHTVersionTracking(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	assert.Equal(t, uint32(0), pm.GetPublishedVersion())

	pm.mu.Lock()
	pm.version = 5
	pm.mu.Unlock()

	assert.Equal(t, uint32(5), pm.GetPublishedVersion())
}

func TestPreKeyDHTBuildQueryPacket(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	var peerPK [32]byte
	peerPK[0] = 0xAB
	peerPK[31] = 0xCD

	packet := pm.buildQueryPacket(peerPK)

	require.Len(t, packet.Data, 32)
	assert.Equal(t, byte(0xAB), packet.Data[0])
	assert.Equal(t, byte(0xCD), packet.Data[31])
}

func TestPreKeyDHTValidateExpiredBundle(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	bundle := &PreKeyDHTBundle{
		OwnerPK:   keyPair.Public,
		Timestamp: time.Now().Add(-48 * time.Hour),
		ExpiresAt: time.Now().Add(-24 * time.Hour),
		Version:   1,
	}

	err = pm.validateBundle(bundle)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestPreKeyDHTConcurrentAccess(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			var pk [32]byte
			pk[0] = byte(id)

			bundle := &PreKeyDHTBundle{
				OwnerPK:   pk,
				ExpiresAt: time.Now().Add(24 * time.Hour),
				Version:   uint32(id),
			}

			pm.mu.Lock()
			pm.localCache[pk] = bundle
			pm.mu.Unlock()
		}(i)
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			var pk [32]byte
			pk[0] = byte(id)
			_, _ = pm.GetCachedBundle(pk)
		}(i)
	}

	wg.Wait()
}

func TestPreKeyDHTStopAutoRefresh(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	pm.StopAutoRefresh()
	pm.StopAutoRefresh()
}

func TestValidateAndRegisterBundleForPeerRejectsNilBundle(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	err = pm.ValidateAndRegisterBundleForPeer(nil, keyPair.Public)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bundle validation failed")
	assert.Contains(t, err.Error(), "nil bundle")
}

func TestValidateAndRegisterBundleForPeerProcessFailureDoesNotCacheOrPin(t *testing.T) {
	ownerKeyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	fsManager, err := NewForwardSecurityManager(ownerKeyPair, t.TempDir())
	require.NoError(t, err)
	defer fsManager.Close()

	pm := NewPreKeyDHTManager(ownerKeyPair, fsManager, nil, nil)

	bundle := makeSignedTestBundle(t, pm, ownerKeyPair, nil)

	err = pm.ValidateAndRegisterBundleForPeer(bundle, ownerKeyPair.Public)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to process pre-key exchange")
	assert.Contains(t, err.Error(), "empty pre-key exchange")

	_, exists := pm.GetCachedBundle(ownerKeyPair.Public)
	assert.False(t, exists)

	pm.mu.RLock()
	_, known := pm.knownSigningKeys[ownerKeyPair.Public]
	pm.mu.RUnlock()
	assert.False(t, known)
}

func TestHandlePreKeyPacketUnknownPeerIsPendingNotCached(t *testing.T) {
	localKeyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)
	peerKeyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(localKeyPair, nil, nil, nil)
	bundle := makeSignedTestBundle(t, pm, peerKeyPair, []PreKeyForExchange{{ID: 1, PublicKey: [32]byte{1}}})

	data, err := pm.serializeBundle(bundle)
	require.NoError(t, err)

	err = pm.HandlePreKeyPacket(&transport.Packet{
		PacketType: transport.PacketAsyncPreKeyExchange,
		Data:       data,
	})
	require.NoError(t, err)

	_, cached := pm.GetCachedBundle(peerKeyPair.Public)
	assert.False(t, cached)

	pending, err := pm.RetrievePreKeys(peerKeyPair.Public)
	require.NoError(t, err)
	require.NotNil(t, pending)
	assert.Equal(t, peerKeyPair.Public, pending.OwnerPK)
}

func TestValidateAndRegisterBundleForPeerPinsAndCachesBundle(t *testing.T) {
	localKeyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)
	peerKeyPair, err := crypto.GenerateKeyPair()
	require.NoError(t, err)

	pm := NewPreKeyDHTManager(localKeyPair, nil, nil, nil)
	bundle := makeSignedTestBundle(t, pm, peerKeyPair, []PreKeyForExchange{{ID: 1, PublicKey: [32]byte{2}}})

	err = pm.ValidateAndRegisterBundleForPeer(bundle, peerKeyPair.Public)
	require.NoError(t, err)

	cached, exists := pm.GetCachedBundle(peerKeyPair.Public)
	require.True(t, exists)
	assert.Equal(t, bundle, cached)

	pm.mu.RLock()
	signingPK, known := pm.knownSigningKeys[peerKeyPair.Public]
	pm.mu.RUnlock()
	require.True(t, known)
	assert.Equal(t, bundle.SigningPK, signingPK)
}

func makeSignedTestBundle(t *testing.T, pm *PreKeyDHTManager, ownerKeyPair *crypto.KeyPair, preKeys []PreKeyForExchange) *PreKeyDHTBundle {
	t.Helper()

	now := time.Now()
	bundle := &PreKeyDHTBundle{
		OwnerPK:   ownerKeyPair.Public,
		SigningPK: crypto.GetSignaturePublicKey(ownerKeyPair.Private),
		PreKeys:   preKeys,
		Timestamp: now,
		ExpiresAt: now.Add(24 * time.Hour),
		Version:   1,
	}

	signature, err := crypto.Sign(pm.bundleDataForSigning(bundle), ownerKeyPair.Private)
	require.NoError(t, err)
	copy(bundle.Signature[:], signature[:])

	return bundle
}

func BenchmarkPreKeyDHTSerialization(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(b, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	bundle := &PreKeyDHTBundle{
		OwnerPK: keyPair.Public,
		PreKeys: make([]PreKeyForExchange, 100),
		Version: 1,
	}
	for i := 0; i < 100; i++ {
		bundle.PreKeys[i] = PreKeyForExchange{ID: uint32(i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pm.serializeBundle(bundle)
	}
}

func BenchmarkPreKeyDHTCacheLookup(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	require.NoError(b, err)

	pm := NewPreKeyDHTManager(keyPair, nil, nil, nil)

	var peerPK [32]byte
	bundle := &PreKeyDHTBundle{
		OwnerPK:   peerPK,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	pm.mu.Lock()
	pm.localCache[peerPK] = bundle
	pm.mu.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pm.GetCachedBundle(peerPK)
	}
}
