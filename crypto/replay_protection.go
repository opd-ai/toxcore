package crypto

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// NonceStore provides persistent storage for used handshake nonces
// to prevent replay attacks even across application restarts.
//
// The store maintains a map of used nonces with their expiry timestamps,
// periodically cleaning up expired entries. Nonces are persisted to disk
// to ensure replay protection survives application restarts.
//
// Example usage:
//
//	ns, err := crypto.NewNonceStore("/var/lib/tox/nonces")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer ns.Close()
//
//	// Check if nonce is fresh (not a replay)
//	if ns.CheckAndStore(nonce, time.Now().Unix()) {
//	    // Process the message
//	} else {
//	    // Replay attack detected, reject message
//	}
//
// The store is safe for concurrent use and automatically runs a background
// goroutine to cleanup expired nonces.
type NonceStore struct {
	mu           sync.RWMutex
	nonces       map[[32]byte]int64 // nonce -> expiry timestamp
	dataDir      string
	saveFile     string
	stopChan     chan struct{}
	logger       *logrus.Logger
	timeProvider TimeProvider
}

// NewNonceStore creates a persistent nonce store
func NewNonceStore(dataDir string) (*NonceStore, error) {
	return NewNonceStoreWithTimeProvider(dataDir, nil)
}

// NewNonceStoreWithTimeProvider creates a persistent nonce store with a custom TimeProvider.
// Pass nil for timeProvider to use the default time provider.
func NewNonceStoreWithTimeProvider(dataDir string, timeProvider TimeProvider) (*NonceStore, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	if timeProvider == nil {
		timeProvider = DefaultTimeProvider{}
	}

	ns := &NonceStore{
		nonces:       make(map[[32]byte]int64),
		dataDir:      dataDir,
		saveFile:     filepath.Join(dataDir, "handshake_nonces.dat"),
		stopChan:     make(chan struct{}),
		logger:       logrus.StandardLogger(),
		timeProvider: timeProvider,
	}

	// Load existing nonces from disk
	if err := ns.load(); err != nil {
		// Log warning but continue (new instance or corrupted file)
		ns.logger.WithError(err).Warn("Could not load nonce store, starting fresh")
	}

	// Start background cleanup
	go ns.cleanupLoop()

	return ns, nil
}

// CheckAndStore checks if nonce was used and stores it if not.
// Returns true if nonce is new (not a replay), false if replay detected.
func (ns *NonceStore) CheckAndStore(nonce [32]byte, timestamp int64) bool {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Check if nonce exists (replay detection)
	if _, exists := ns.nonces[nonce]; exists {
		ns.logger.WithFields(logrus.Fields{
			"nonce":     fmt.Sprintf("%x", nonce[:8]),
			"timestamp": timestamp,
		}).Warn("Replay attack detected: nonce already used")
		return false
	}

	// Calculate expiry (5 minutes handshake window + 1 minute future drift)
	expiry := timestamp + int64((6 * time.Minute).Seconds())

	// Store nonce
	ns.nonces[nonce] = expiry

	// Note: save() is called synchronously during Close() to ensure persistence
	// Async saves during operation are optional for performance

	return true
}

// load reads nonce store from disk
// readNonceStoreFile reads and validates the nonce store file.
func (ns *NonceStore) readNonceStoreFile() ([]byte, error) {
	data, err := os.ReadFile(ns.saveFile)
	if err != nil {
		if os.IsNotExist(err) {
			ns.logger.Info("No existing nonce store found, starting fresh")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read nonce store: %w", err)
	}

	if len(data) < 8 {
		return nil, fmt.Errorf("corrupted nonce store: file too small")
	}

	return data, nil
}

// parseNonceRecord parses a single nonce record from the data.
func (ns *NonceStore) parseNonceRecord(data []byte, offset int, now int64) (nonce [32]byte, timestamp int64, valid bool) {
	copy(nonce[:], data[offset:offset+32])
	timestampUint := binary.BigEndian.Uint64(data[offset+32 : offset+40])
	timestamp, err := safeUint64ToInt64(timestampUint)
	if err != nil {
		ns.logger.WithFields(logrus.Fields{
			"value": timestampUint,
			"error": err,
		}).Warn("Invalid timestamp in nonce record, skipping")
		return nonce, 0, false
	}

	return nonce, timestamp, timestamp > now
}

func (ns *NonceStore) load() error {
	data, err := ns.readNonceStoreFile()
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}

	count := binary.BigEndian.Uint64(data[0:8])
	offset := 8
	now := ns.getTimeProvider().Now().Unix()
	loaded := 0

	for i := uint64(0); i < count && offset+40 <= len(data); i++ {
		nonce, timestamp, valid := ns.parseNonceRecord(data, offset, now)
		if valid {
			ns.nonces[nonce] = timestamp
			loaded++
		}
		offset += 40
	}

	ns.logger.WithFields(logrus.Fields{
		"total_in_file":  count,
		"loaded":         loaded,
		"expired_pruned": count - uint64(loaded),
	}).Info("Nonce store loaded successfully")

	return nil
}

// save writes nonce store to disk
func (ns *NonceStore) save() error {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	// Calculate size
	buf := make([]byte, 8+len(ns.nonces)*40)
	binary.BigEndian.PutUint64(buf[0:8], uint64(len(ns.nonces)))

	offset := 8
	for nonce, timestamp := range ns.nonces {
		copy(buf[offset:offset+32], nonce[:])
		timestampUint, err := safeInt64ToUint64(timestamp)
		if err != nil {
			ns.logger.WithFields(logrus.Fields{
				"timestamp": timestamp,
				"error":     err,
			}).Warn("Invalid timestamp during save, skipping nonce")
			continue
		}
		binary.BigEndian.PutUint64(buf[offset+32:offset+40], timestampUint)
		offset += 40
	}

	// Atomic write
	tmpFile := ns.saveFile + ".tmp"
	if err := os.WriteFile(tmpFile, buf, 0o600); err != nil {
		return fmt.Errorf("failed to write temporary nonce store: %w", err)
	}

	if err := os.Rename(tmpFile, ns.saveFile); err != nil {
		return fmt.Errorf("failed to rename nonce store: %w", err)
	}

	return nil
}

// cleanupLoop periodically removes expired nonces
func (ns *NonceStore) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ns.cleanup()
		case <-ns.stopChan:
			return
		}
	}
}

// cleanup removes expired nonces
func (ns *NonceStore) cleanup() {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	now := ns.getTimeProvider().Now().Unix()
	removed := 0

	for nonce, expiry := range ns.nonces {
		if expiry < now {
			delete(ns.nonces, nonce)
			removed++
		}
	}

	if removed > 0 {
		ns.logger.WithFields(logrus.Fields{
			"removed":   removed,
			"remaining": len(ns.nonces),
		}).Info("Cleaned up expired nonces")
	}
}

// Close stops the cleanup loop and saves final state
func (ns *NonceStore) Close() error {
	close(ns.stopChan)

	ns.mu.RLock()
	defer ns.mu.RUnlock()

	return ns.save()
}

// Size returns the current number of stored nonces
func (ns *NonceStore) Size() int {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return len(ns.nonces)
}

// SetTimeProvider sets the time provider for deterministic testing.
// Pass nil to reset to the default time provider.
func (ns *NonceStore) SetTimeProvider(tp TimeProvider) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if tp == nil {
		tp = DefaultTimeProvider{}
	}
	ns.timeProvider = tp
}

// getTimeProvider returns the time provider, defaulting to DefaultTimeProvider if not set.
func (ns *NonceStore) getTimeProvider() TimeProvider {
	if ns.timeProvider == nil {
		return DefaultTimeProvider{}
	}
	return ns.timeProvider
}
