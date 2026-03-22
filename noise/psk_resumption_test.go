package noise

import (
	"bytes"
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionTicketCreation(t *testing.T) {
	ticket := &SessionTicket{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	// Generate random values
	_, err := rand.Read(ticket.TicketID[:])
	require.NoError(t, err)
	_, err = rand.Read(ticket.PSK[:])
	require.NoError(t, err)
	_, err = rand.Read(ticket.PeerPublicKey[:])
	require.NoError(t, err)

	assert.False(t, ticket.IsExpired())
	assert.True(t, ticket.IsValid())
}

func TestSessionTicketExpiration(t *testing.T) {
	ticket := &SessionTicket{
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour),
	}

	// Generate random PSK
	_, err := rand.Read(ticket.PSK[:])
	require.NoError(t, err)

	assert.True(t, ticket.IsExpired())
	assert.False(t, ticket.IsValid())
}

func TestSessionTicketZeroPSK(t *testing.T) {
	ticket := &SessionTicket{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		// PSK is all zeros by default
	}

	assert.False(t, ticket.IsExpired())
	assert.False(t, ticket.IsValid()) // Invalid because PSK is zero
}

func TestSessionCacheCreation(t *testing.T) {
	config := DefaultSessionCacheConfig()
	cache := NewSessionCache(config)
	defer cache.Close()

	assert.NotNil(t, cache)
	assert.Equal(t, 0, cache.Count())
}

func TestSessionCacheStoreAndRetrieve(t *testing.T) {
	config := DefaultSessionCacheConfig()
	cache := NewSessionCache(config)
	defer cache.Close()

	peerKey := make([]byte, 32)
	_, err := rand.Read(peerKey)
	require.NoError(t, err)

	ticket := &SessionTicket{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_, err = rand.Read(ticket.TicketID[:])
	require.NoError(t, err)
	_, err = rand.Read(ticket.PSK[:])
	require.NoError(t, err)
	copy(ticket.PeerPublicKey[:], peerKey)

	err = cache.StoreTicket(peerKey, ticket)
	require.NoError(t, err)
	assert.Equal(t, 1, cache.Count())

	retrieved, err := cache.GetTicket(peerKey)
	require.NoError(t, err)
	assert.Equal(t, ticket.TicketID, retrieved.TicketID)
	assert.Equal(t, ticket.PSK, retrieved.PSK)
}

func TestSessionCacheGetByID(t *testing.T) {
	config := DefaultSessionCacheConfig()
	cache := NewSessionCache(config)
	defer cache.Close()

	peerKey := make([]byte, 32)
	_, err := rand.Read(peerKey)
	require.NoError(t, err)

	ticket := &SessionTicket{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_, err = rand.Read(ticket.TicketID[:])
	require.NoError(t, err)
	_, err = rand.Read(ticket.PSK[:])
	require.NoError(t, err)
	copy(ticket.PeerPublicKey[:], peerKey)

	err = cache.StoreTicket(peerKey, ticket)
	require.NoError(t, err)

	retrieved, err := cache.GetTicketByID(ticket.TicketID)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(ticket.PeerPublicKey[:], retrieved.PeerPublicKey[:]))
}

func TestSessionCacheRemove(t *testing.T) {
	config := DefaultSessionCacheConfig()
	cache := NewSessionCache(config)
	defer cache.Close()

	peerKey := make([]byte, 32)
	_, err := rand.Read(peerKey)
	require.NoError(t, err)

	ticket := &SessionTicket{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_, err = rand.Read(ticket.PSK[:])
	require.NoError(t, err)

	err = cache.StoreTicket(peerKey, ticket)
	require.NoError(t, err)
	assert.Equal(t, 1, cache.Count())

	cache.RemoveTicket(peerKey)
	assert.Equal(t, 0, cache.Count())

	_, err = cache.GetTicket(peerKey)
	assert.ErrorIs(t, err, ErrSessionTicketNotFound)
}

func TestSessionCacheExpiredTicket(t *testing.T) {
	config := DefaultSessionCacheConfig()
	cache := NewSessionCache(config)
	defer cache.Close()

	peerKey := make([]byte, 32)
	_, err := rand.Read(peerKey)
	require.NoError(t, err)

	ticket := &SessionTicket{
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour), // Already expired
	}
	_, err = rand.Read(ticket.PSK[:])
	require.NoError(t, err)

	err = cache.StoreTicket(peerKey, ticket)
	require.NoError(t, err)

	_, err = cache.GetTicket(peerKey)
	assert.ErrorIs(t, err, ErrSessionTicketExpired)
}

func TestSessionCacheReplayProtection(t *testing.T) {
	config := DefaultSessionCacheConfig()
	cache := NewSessionCache(config)
	defer cache.Close()

	peerKey := make([]byte, 32)
	_, err := rand.Read(peerKey)
	require.NoError(t, err)

	ticket := &SessionTicket{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_, err = rand.Read(ticket.TicketID[:])
	require.NoError(t, err)
	_, err = rand.Read(ticket.PSK[:])
	require.NoError(t, err)

	err = cache.StoreTicket(peerKey, ticket)
	require.NoError(t, err)

	// First use of message ID should succeed
	err = cache.CheckAndRecordReplay(ticket.TicketID, 1)
	assert.NoError(t, err)

	// Second use of same message ID should fail
	err = cache.CheckAndRecordReplay(ticket.TicketID, 1)
	assert.ErrorIs(t, err, ErrReplayDetected)

	// Different message ID should succeed
	err = cache.CheckAndRecordReplay(ticket.TicketID, 2)
	assert.NoError(t, err)
}

func TestSessionCacheInvalidPeerKey(t *testing.T) {
	config := DefaultSessionCacheConfig()
	cache := NewSessionCache(config)
	defer cache.Close()

	ticket := &SessionTicket{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	// Too short peer key
	err := cache.StoreTicket(make([]byte, 16), ticket)
	assert.Error(t, err)

	// Too long peer key
	err = cache.StoreTicket(make([]byte, 64), ticket)
	assert.Error(t, err)
}

func TestSessionCacheConcurrentAccess(t *testing.T) {
	config := DefaultSessionCacheConfig()
	cache := NewSessionCache(config)
	defer cache.Close()

	const numGoroutines = 50
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			peerKey := make([]byte, 32)
			_, err := rand.Read(peerKey)
			require.NoError(t, err)

			ticket := &SessionTicket{
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(time.Hour),
			}
			_, err = rand.Read(ticket.TicketID[:])
			require.NoError(t, err)
			_, err = rand.Read(ticket.PSK[:])
			require.NoError(t, err)

			err = cache.StoreTicket(peerKey, ticket)
			assert.NoError(t, err)

			_, err = cache.GetTicket(peerKey)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
	assert.Equal(t, numGoroutines, cache.Count())
}

func TestPSKHandshakeConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    PSKHandshakeConfig
		wantError bool
	}{
		{
			name: "valid initiator config",
			config: PSKHandshakeConfig{
				StaticPrivKey: make([]byte, 32),
				PeerPubKey:    make([]byte, 32),
				PSK:           [32]byte{1, 2, 3}, // Non-zero PSK
				Role:          Initiator,
			},
			wantError: false,
		},
		{
			name: "valid responder config",
			config: PSKHandshakeConfig{
				StaticPrivKey: make([]byte, 32),
				PeerPubKey:    nil, // Responder doesn't need peer key
				PSK:           [32]byte{1, 2, 3},
				Role:          Responder,
			},
			wantError: false,
		},
		{
			name: "invalid private key length",
			config: PSKHandshakeConfig{
				StaticPrivKey: make([]byte, 16), // Too short
				PeerPubKey:    make([]byte, 32),
				PSK:           [32]byte{1, 2, 3},
				Role:          Initiator,
			},
			wantError: true,
		},
		{
			name: "initiator missing peer key",
			config: PSKHandshakeConfig{
				StaticPrivKey: make([]byte, 32),
				PeerPubKey:    nil,
				PSK:           [32]byte{1, 2, 3},
				Role:          Initiator,
			},
			wantError: true,
		},
		{
			name: "zero PSK",
			config: PSKHandshakeConfig{
				StaticPrivKey: make([]byte, 32),
				PeerPubKey:    make([]byte, 32),
				PSK:           [32]byte{}, // All zeros
				Role:          Initiator,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fill in random key data where needed (but keep the structure)
			if len(tt.config.StaticPrivKey) == 32 {
				rand.Read(tt.config.StaticPrivKey)
			}
			if len(tt.config.PeerPubKey) == 32 {
				rand.Read(tt.config.PeerPubKey)
			}

			_, err := NewPSKHandshake(tt.config)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPSKHandshakeInitiatorResponder(t *testing.T) {
	// Generate key pairs
	initiatorPriv := make([]byte, 32)
	responderPriv := make([]byte, 32)
	_, err := rand.Read(initiatorPriv)
	require.NoError(t, err)
	_, err = rand.Read(responderPriv)
	require.NoError(t, err)

	// Derive public keys using crypto package
	initiatorKeyPair, err := createKeyPairFromPrivateKey(initiatorPriv)
	require.NoError(t, err)
	responderKeyPair, err := createKeyPairFromPrivateKey(responderPriv)
	require.NoError(t, err)

	// Shared PSK
	var psk [32]byte
	_, err = rand.Read(psk[:])
	require.NoError(t, err)

	// Create initiator handshake
	initiator, err := NewPSKHandshake(PSKHandshakeConfig{
		StaticPrivKey: initiatorPriv,
		PeerPubKey:    responderKeyPair.Public[:],
		PSK:           psk,
		Role:          Initiator,
	})
	require.NoError(t, err)

	// Create responder handshake
	responder, err := NewPSKHandshake(PSKHandshakeConfig{
		StaticPrivKey: responderPriv,
		PeerPubKey:    initiatorKeyPair.Public[:],
		PSK:           psk,
		Role:          Responder,
	})
	require.NoError(t, err)

	// Initiator writes first message with 0-RTT data
	earlyData := []byte("early application data")
	msg1, complete1, err := initiator.WriteMessage(earlyData, nil)
	require.NoError(t, err)
	assert.False(t, complete1) // Initiator not complete until response

	// Responder processes and responds
	msg2, complete2, err := responder.WriteMessage(nil, msg1)
	require.NoError(t, err)
	assert.True(t, complete2) // Responder completes after response

	// Check responder received early data
	receivedEarlyData := responder.GetEarlyData()
	assert.Equal(t, earlyData, receivedEarlyData)

	// Initiator reads response
	_, complete3, err := initiator.ReadMessage(msg2)
	require.NoError(t, err)
	assert.True(t, complete3)

	// Both should now have cipher states
	sendCipher1, recvCipher1, err := initiator.GetCipherStates()
	require.NoError(t, err)
	assert.NotNil(t, sendCipher1)
	assert.NotNil(t, recvCipher1)

	sendCipher2, recvCipher2, err := responder.GetCipherStates()
	require.NoError(t, err)
	assert.NotNil(t, sendCipher2)
	assert.NotNil(t, recvCipher2)

	// Test encryption/decryption
	plaintext := []byte("test message")
	ciphertext, err := sendCipher1.Encrypt(nil, nil, plaintext)
	require.NoError(t, err)

	decrypted, err := recvCipher2.Decrypt(nil, nil, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestPSKHandshakeGetters(t *testing.T) {
	priv := make([]byte, 32)
	pub := make([]byte, 32)
	_, _ = rand.Read(priv)
	_, _ = rand.Read(pub)

	var psk [32]byte
	var ticketID [32]byte
	_, _ = rand.Read(psk[:])
	_, _ = rand.Read(ticketID[:])

	hs, err := NewPSKHandshake(PSKHandshakeConfig{
		StaticPrivKey: priv,
		PeerPubKey:    pub,
		PSK:           psk,
		TicketID:      ticketID,
		Role:          Initiator,
	})
	require.NoError(t, err)

	// Test getters
	assert.Equal(t, ticketID, hs.GetTicketID())
	assert.NotZero(t, hs.GetTimestamp())
	assert.NotZero(t, hs.GetNonce())
	assert.False(t, hs.IsComplete())
	assert.NotNil(t, hs.GetLocalStaticKey())
}

func TestDeriveSessionTicket(t *testing.T) {
	// Create a completed handshake first
	initiatorPriv := make([]byte, 32)
	responderPriv := make([]byte, 32)
	_, err := rand.Read(initiatorPriv)
	require.NoError(t, err)
	_, err = rand.Read(responderPriv)
	require.NoError(t, err)

	_, err = createKeyPairFromPrivateKey(initiatorPriv)
	require.NoError(t, err)
	responderKeyPair, err := createKeyPairFromPrivateKey(responderPriv)
	require.NoError(t, err)

	// Complete a regular IK handshake
	initiator, err := NewIKHandshake(initiatorPriv, responderKeyPair.Public[:], Initiator)
	require.NoError(t, err)

	responder, err := NewIKHandshake(responderPriv, nil, Responder)
	require.NoError(t, err)

	msg1, _, err := initiator.WriteMessage(nil, nil)
	require.NoError(t, err)

	msg2, _, err := responder.WriteMessage(nil, msg1)
	require.NoError(t, err)

	_, _, err = initiator.ReadMessage(msg2)
	require.NoError(t, err)

	// Get cipher states
	sendCipher, recvCipher, err := initiator.GetCipherStates()
	require.NoError(t, err)

	// Derive session ticket
	ticket, err := DeriveSessionTicket(initiator, sendCipher, recvCipher, time.Hour)
	require.NoError(t, err)
	assert.NotNil(t, ticket)
	assert.True(t, ticket.IsValid())
	assert.False(t, ticket.IsExpired())

	// PSK should not be zero
	var zeroPSK [32]byte
	assert.NotEqual(t, zeroPSK, ticket.PSK)
}

func TestCreateResumptionHandshake(t *testing.T) {
	priv := make([]byte, 32)
	_, err := rand.Read(priv)
	require.NoError(t, err)

	ticket := &SessionTicket{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_, _ = rand.Read(ticket.TicketID[:])
	_, _ = rand.Read(ticket.PSK[:])
	_, _ = rand.Read(ticket.PeerPublicKey[:])

	hs, err := CreateResumptionHandshake(priv, ticket, Initiator)
	require.NoError(t, err)
	assert.NotNil(t, hs)
	assert.Equal(t, ticket.TicketID, hs.GetTicketID())
}

func TestCreateResumptionHandshakeExpiredTicket(t *testing.T) {
	priv := make([]byte, 32)
	_, err := rand.Read(priv)
	require.NoError(t, err)

	ticket := &SessionTicket{
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour), // Expired
	}
	_, _ = rand.Read(ticket.PSK[:])

	_, err = CreateResumptionHandshake(priv, ticket, Initiator)
	assert.ErrorIs(t, err, ErrSessionTicketExpired)
}

func TestCreateResumptionHandshakeInvalidTicket(t *testing.T) {
	priv := make([]byte, 32)
	_, err := rand.Read(priv)
	require.NoError(t, err)

	ticket := &SessionTicket{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		// PSK is all zeros - invalid
	}

	_, err = CreateResumptionHandshake(priv, ticket, Initiator)
	assert.ErrorIs(t, err, ErrSessionTicketInvalid)
}

func TestSupportsResumption(t *testing.T) {
	config := DefaultSessionCacheConfig()
	cache := NewSessionCache(config)
	defer cache.Close()

	peerKey := make([]byte, 32)
	_, err := rand.Read(peerKey)
	require.NoError(t, err)

	// No ticket - should not support resumption
	assert.False(t, SupportsResumption(cache, peerKey))

	// Add valid ticket
	ticket := &SessionTicket{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_, _ = rand.Read(ticket.PSK[:])
	err = cache.StoreTicket(peerKey, ticket)
	require.NoError(t, err)

	// Now should support resumption
	assert.True(t, SupportsResumption(cache, peerKey))

	// Nil cache should return false
	assert.False(t, SupportsResumption(nil, peerKey))
}

func TestSessionCacheReplaceExisting(t *testing.T) {
	config := DefaultSessionCacheConfig()
	cache := NewSessionCache(config)
	defer cache.Close()

	peerKey := make([]byte, 32)
	_, err := rand.Read(peerKey)
	require.NoError(t, err)

	// Store first ticket
	ticket1 := &SessionTicket{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_, _ = rand.Read(ticket1.TicketID[:])
	_, _ = rand.Read(ticket1.PSK[:])
	err = cache.StoreTicket(peerKey, ticket1)
	require.NoError(t, err)

	// Store second ticket for same peer
	ticket2 := &SessionTicket{
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(2 * time.Hour),
	}
	_, _ = rand.Read(ticket2.TicketID[:])
	_, _ = rand.Read(ticket2.PSK[:])
	err = cache.StoreTicket(peerKey, ticket2)
	require.NoError(t, err)

	// Should still have only 1 ticket
	assert.Equal(t, 1, cache.Count())

	// Should get the newer ticket
	retrieved, err := cache.GetTicket(peerKey)
	require.NoError(t, err)
	assert.Equal(t, ticket2.TicketID, retrieved.TicketID)

	// Old ticket should not be findable by ID
	_, err = cache.GetTicketByID(ticket1.TicketID)
	assert.ErrorIs(t, err, ErrSessionTicketNotFound)
}

func TestSessionCacheCleanupExpired(t *testing.T) {
	config := SessionCacheConfig{
		Lifetime:        time.Hour,
		CleanupInterval: 100 * time.Millisecond, // Fast cleanup for test
	}
	cache := NewSessionCache(config)
	defer cache.Close()

	// Add expired ticket
	peerKey := make([]byte, 32)
	_, _ = rand.Read(peerKey)

	expiredTicket := &SessionTicket{
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour), // Already expired
	}
	_, _ = rand.Read(expiredTicket.TicketID[:])
	_, _ = rand.Read(expiredTicket.PSK[:])

	// Manually add to bypass validation
	cache.mu.Lock()
	cache.tickets[string(peerKey)] = expiredTicket
	cache.ticketsByID[expiredTicket.TicketID] = expiredTicket
	cache.mu.Unlock()

	assert.Equal(t, 1, cache.Count())

	// Wait for cleanup
	time.Sleep(250 * time.Millisecond)

	// Should be cleaned up
	assert.Equal(t, 0, cache.Count())
}

func TestDefaultSessionCacheConfig(t *testing.T) {
	config := DefaultSessionCacheConfig()

	assert.Equal(t, DefaultSessionTicketLifetime, config.Lifetime)
	assert.Equal(t, SessionCacheCleanupInterval, config.CleanupInterval)
	assert.Equal(t, 0, config.MaxTickets)
}

func TestSessionCacheConfigValidation(t *testing.T) {
	// Test with invalid lifetime (gets corrected)
	config := SessionCacheConfig{
		Lifetime:        -1 * time.Hour, // Negative
		CleanupInterval: -1 * time.Minute,
	}
	cache := NewSessionCache(config)
	defer cache.Close()

	// Should use defaults
	assert.Equal(t, DefaultSessionTicketLifetime, cache.lifetime)
	assert.Equal(t, SessionCacheCleanupInterval, cache.cleanupInterval)

	// Test with excessive lifetime (gets capped)
	config2 := SessionCacheConfig{
		Lifetime: 30 * 24 * time.Hour, // 30 days - too long
	}
	cache2 := NewSessionCache(config2)
	defer cache2.Close()

	assert.Equal(t, MaxSessionTicketLifetime, cache2.lifetime)
}
