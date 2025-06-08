package crypto

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// RekeyThreshold defines when a session needs rekeying
const (
	DefaultRekeyInterval = 24 * time.Hour  // Rekey every 24 hours
	DefaultRekeyThreshold = 1000000        // Rekey after 1M messages
	MaxSessionAge = 7 * 24 * time.Hour     // Max session age: 7 days
)

// RekeyManager handles automatic session rekeying
//
//export ToxRekeyManager
type RekeyManager struct {
	scheduler    *time.Ticker
	sessions     map[string]*NoiseSession
	mu           sync.RWMutex
	rekeyChannel chan string
	stopChannel  chan struct{}
}

// SessionMetrics tracks session performance and usage
//
//export ToxSessionMetrics
type SessionMetrics struct {
	SessionsCreated     uint64
	SessionsDestroyed   uint64
	RekeysPerformed     uint64
	MessagesSent        uint64
	MessagesReceived    uint64
	AverageLatency      time.Duration
	LastActivity        time.Time
}

// EphemeralKeyManager manages ephemeral key lifecycle
//
//export ToxEphemeralKeyManager
type EphemeralKeyManager struct {
	keyCache         map[string]*KeyPair
	rotationInterval time.Duration
	maxCacheSize     int
	mu               sync.RWMutex
	lastCleanup      time.Time
}

// NewRekeyManager creates a new rekey manager
//
//export ToxNewRekeyManager
func NewRekeyManager() *RekeyManager {
	return &RekeyManager{
		sessions:     make(map[string]*NoiseSession),
		rekeyChannel: make(chan string, 100),
		stopChannel:  make(chan struct{}),
	}
}

// Start begins the automatic rekeying process
//
//export ToxRekeyManagerStart
func (rm *RekeyManager) Start() {
	rm.scheduler = time.NewTicker(1 * time.Hour) // Check every hour
	go rm.rekeyLoop()
}

// Stop terminates the rekeying process
//
//export ToxRekeyManagerStop
func (rm *RekeyManager) Stop() {
	if rm.scheduler != nil {
		rm.scheduler.Stop()
	}
	close(rm.stopChannel)
}

// AddSession adds a session to be monitored for rekeying
//
//export ToxRekeyManagerAddSession
func (rm *RekeyManager) AddSession(peerKey [32]byte, session *NoiseSession) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	peerID := fmt.Sprintf("%x", peerKey)
	rm.sessions[peerID] = session
}

// RemoveSession removes a session from monitoring
//
//export ToxRekeyManagerRemoveSession
func (rm *RekeyManager) RemoveSession(peerKey [32]byte) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	peerID := fmt.Sprintf("%x", peerKey)
	delete(rm.sessions, peerID)
}

// rekeyLoop runs the automatic rekeying process
func (rm *RekeyManager) rekeyLoop() {
	for {
		select {
		case <-rm.scheduler.C:
			rm.checkAndRekeyMissingSessions()
		case peerID := <-rm.rekeyChannel:
			rm.performRekey(peerID)
		case <-rm.stopChannel:
			return
		}
	}
}

// checkAndRekeyMissingSessions checks all sessions and triggers rekeying if needed
func (rm *RekeyManager) checkAndRekeyMissingSessions() {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	for peerID, session := range rm.sessions {
		if session.NeedsRekey() {
			select {
			case rm.rekeyChannel <- peerID:
				// Rekey request queued
			default:
				// Channel full, skip this round
			}
		}
	}
}

// performRekey executes the rekeying process for a specific session
func (rm *RekeyManager) performRekey(peerID string) {
	rm.mu.RLock()
	session, exists := rm.sessions[peerID]
	rm.mu.RUnlock()
	
	if !exists {
		return
	}
	
	// Perform the actual rekey operation
	err := session.PerformRekey()
	if err != nil {
		// Log error and potentially remove session
		return
	}
	
	// Update session metrics
	session.RekeysPerformed++
	session.LastRekey = time.Now()
}

// NeedsRekey determines if a session requires rekeying
//
//export ToxNoiseSessionNeedsRekey
func (ns *NoiseSession) NeedsRekey() bool {
	// Check time-based rekeying
	if time.Since(ns.Established) > DefaultRekeyInterval {
		return true
	}
	
	// Check message count-based rekeying
	if ns.MessageCounter > DefaultRekeyThreshold {
		return true
	}
	
	// Check if explicitly marked for rekeying
	if ns.RekeyNeeded {
		return true
	}
	
	// Check if session is too old
	if time.Since(ns.Established) > MaxSessionAge {
		return true
	}
	
	return false
}

// PerformRekey executes the rekeying process for a session
//
//export ToxNoiseSessionPerformRekey
func (ns *NoiseSession) PerformRekey() error {
	// Create new ephemeral keys
	ephemeralKey, err := GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate ephemeral key: %w", err)
	}
	
	// Store old cipher states for gradual transition
	oldSendCipher := ns.SendCipher
	oldRecvCipher := ns.RecvCipher
	
	// Create new handshake for rekeying
	handshake, err := NewNoiseHandshake(true, ephemeralKey.Private, ns.PeerKey)
	if err != nil {
		return fmt.Errorf("failed to create rekey handshake: %w", err)
	}
	
	// Perform handshake to establish new session keys
	// This would involve network communication in a real implementation
	_, newSession, err := handshake.WriteMessage([]byte("REKEY"))
	if err != nil {
		return fmt.Errorf("failed to perform rekey handshake: %w", err)
	}
	
	// Update session with new cipher states
	ns.SendCipher = newSession.SendCipher
	ns.RecvCipher = newSession.RecvCipher
	ns.EphemeralKeys = ephemeralKey
	ns.LastRekey = time.Now()
	ns.MessageCounter = 0
	ns.RekeyNeeded = false
	
	// Securely clear old cipher states
	// Note: In a real implementation, this would involve secure memory clearing
	_ = oldSendCipher
	_ = oldRecvCipher
	
	return nil
}

// NewEphemeralKeyManager creates a new ephemeral key manager
//
//export ToxNewEphemeralKeyManager
func NewEphemeralKeyManager() *EphemeralKeyManager {
	return &EphemeralKeyManager{
		keyCache:         make(map[string]*KeyPair),
		rotationInterval: 1 * time.Hour,
		maxCacheSize:     100,
		lastCleanup:      time.Now(),
	}
}

// GetEphemeralKey retrieves or generates an ephemeral key for a peer
//
//export ToxEphemeralKeyManagerGet
func (ekm *EphemeralKeyManager) GetEphemeralKey(peerKey [32]byte) (*KeyPair, error) {
	ekm.mu.Lock()
	defer ekm.mu.Unlock()
	
	peerID := fmt.Sprintf("%x", peerKey)
	
	// Check if we have a cached key that's still valid
	if cachedKey, exists := ekm.keyCache[peerID]; exists {
		// In a real implementation, check if key is still within rotation interval
		return cachedKey, nil
	}
	
	// Generate new ephemeral key
	ephemeralKey, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}
	
	// Cache the key
	ekm.keyCache[peerID] = ephemeralKey
	
	// Cleanup old keys if needed
	if len(ekm.keyCache) > ekm.maxCacheSize {
		ekm.cleanupOldKeys()
	}
	
	return ephemeralKey, nil
}

// cleanupOldKeys removes old ephemeral keys from cache
func (ekm *EphemeralKeyManager) cleanupOldKeys() {
	// Simple cleanup: remove random keys if cache is full
	// In a real implementation, this would be time-based
	if len(ekm.keyCache) > ekm.maxCacheSize {
		for peerID := range ekm.keyCache {
			delete(ekm.keyCache, peerID)
			if len(ekm.keyCache) <= ekm.maxCacheSize/2 {
				break
			}
		}
	}
	ekm.lastCleanup = time.Now()
}

// RotateKey forces rotation of an ephemeral key for a specific peer
//
//export ToxEphemeralKeyManagerRotate
func (ekm *EphemeralKeyManager) RotateKey(peerKey [32]byte) (*KeyPair, error) {
	ekm.mu.Lock()
	defer ekm.mu.Unlock()
	
	peerID := fmt.Sprintf("%x", peerKey)
	
	// Remove existing key
	delete(ekm.keyCache, peerID)
	
	// Generate new key
	ephemeralKey, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}
	
	// Cache the new key
	ekm.keyCache[peerID] = ephemeralKey
	
	return ephemeralKey, nil
}

// Enhanced NoiseSession with additional fields for advanced session management
//
//export ToxEnhancedNoiseSession
type EnhancedNoiseSession struct {
	*NoiseSession
	RekeysPerformed  uint64
	LastRekey        time.Time
	MessageCounter   uint64
	SessionID        string
	Quality          SessionQuality
	Metrics          *SessionMetrics
}

// SessionQuality represents the quality/health of a session
type SessionQuality struct {
	Latency       time.Duration
	ErrorRate     float64
	ThroughputMbps float64
	LastMeasured  time.Time
}

// UpdateQuality updates the session quality metrics
//
//export ToxEnhancedNoiseSessionUpdateQuality
func (ens *EnhancedNoiseSession) UpdateQuality(latency time.Duration, errorRate float64, throughput float64) {
	ens.Quality.Latency = latency
	ens.Quality.ErrorRate = errorRate
	ens.Quality.ThroughputMbps = throughput
	ens.Quality.LastMeasured = time.Now()
}

// IsHealthy determines if the session is performing well
//
//export ToxEnhancedNoiseSessionIsHealthy
func (ens *EnhancedNoiseSession) IsHealthy() bool {
	// Define health thresholds
	const (
		maxLatency   = 500 * time.Millisecond
		maxErrorRate = 0.01 // 1%
		minThroughput = 1.0 // 1 Mbps
	)
	
	return ens.Quality.Latency <= maxLatency &&
		   ens.Quality.ErrorRate <= maxErrorRate &&
		   ens.Quality.ThroughputMbps >= minThroughput
}
