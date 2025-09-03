package async

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// PreKeyBundle represents a collection of one-time keys for a specific peer
type PreKeyBundle struct {
	PeerPK           [32]byte  `json:"peer_pk"`
	Keys             []PreKey  `json:"keys"`
	CreatedAt        time.Time `json:"created_at"`
	UsedCount        int       `json:"used_count"`
	MaxKeys          int       `json:"max_keys"`
	LastRefreshOffer time.Time `json:"last_refresh_offer"`
}

// PreKey represents a single one-time key for forward secrecy
type PreKey struct {
	ID      uint32          `json:"id"`
	KeyPair *crypto.KeyPair `json:"keypair"`
	Used    bool            `json:"used"`
	UsedAt  *time.Time      `json:"used_at,omitempty"`
}

// PreKeyStore manages on-disk storage of pre-keys for forward secrecy
type PreKeyStore struct {
	mutex   sync.RWMutex
	dataDir string
	keyPair *crypto.KeyPair            // Our main identity key
	bundles map[[32]byte]*PreKeyBundle // In-memory cache of pre-key bundles
}

// PreKeyRefreshMessage is sent when peers are online to refresh pre-keys
type PreKeyRefreshMessage struct {
	Type       string    `json:"type"`
	PeerPK     [32]byte  `json:"peer_pk"`
	NewPreKeys []PreKey  `json:"new_pre_keys"`
	Timestamp  time.Time `json:"timestamp"`
}

const (
	PreKeysPerPeer         = 100
	PreKeyRefreshThreshold = 20                  // Refresh when less than 20 keys remain
	MaxPreKeyAge           = 30 * 24 * time.Hour // 30 days
)

// NewPreKeyStore creates a new pre-key storage manager
func NewPreKeyStore(keyPair *crypto.KeyPair, dataDir string) (*PreKeyStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	store := &PreKeyStore{
		dataDir: dataDir,
		keyPair: keyPair,
		bundles: make(map[[32]byte]*PreKeyBundle),
	}

	// Load existing bundles from disk
	if err := store.loadBundles(); err != nil {
		return nil, fmt.Errorf("failed to load pre-key bundles: %w", err)
	}

	return store, nil
}

// GeneratePreKeys creates a new bundle of one-time keys for a peer
func (pks *PreKeyStore) GeneratePreKeys(peerPK [32]byte) (*PreKeyBundle, error) {
	pks.mutex.Lock()
	defer pks.mutex.Unlock()

	keys := make([]PreKey, PreKeysPerPeer)
	for i := 0; i < PreKeysPerPeer; i++ {
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate pre-key %d: %w", i, err)
		}

		// Generate random key ID
		idBytes := make([]byte, 4)
		if _, err := rand.Read(idBytes); err != nil {
			return nil, fmt.Errorf("failed to generate key ID: %w", err)
		}
		keyID := uint32(idBytes[0])<<24 | uint32(idBytes[1])<<16 | uint32(idBytes[2])<<8 | uint32(idBytes[3])

		keys[i] = PreKey{
			ID:      keyID,
			KeyPair: keyPair,
			Used:    false,
		}
	}

	bundle := &PreKeyBundle{
		PeerPK:    peerPK,
		Keys:      keys,
		CreatedAt: time.Now(),
		UsedCount: 0,
		MaxKeys:   PreKeysPerPeer,
	}

	pks.bundles[peerPK] = bundle

	// Save to disk
	if err := pks.saveBundleToDisk(bundle); err != nil {
		return nil, fmt.Errorf("failed to save bundle to disk: %w", err)
	}

	return bundle, nil
}

// GetAvailablePreKey returns an unused pre-key for a peer, if available
func (pks *PreKeyStore) GetAvailablePreKey(peerPK [32]byte) (*PreKey, error) {
	pks.mutex.Lock()
	defer pks.mutex.Unlock()

	bundle, exists := pks.bundles[peerPK]
	if !exists {
		return nil, fmt.Errorf("no pre-key bundle found for peer %x", peerPK[:8])
	}

	// Find an unused key
	for i := range bundle.Keys {
		if !bundle.Keys[i].Used {
			// Create a copy of the key before removing it from storage
			// This ensures we return a valid key that the caller can use
			keyPairCopy := &crypto.KeyPair{
				Public:  bundle.Keys[i].KeyPair.Public,
				Private: bundle.Keys[i].KeyPair.Private,
			}

			// Store key ID and timestamp for the result
			keyID := bundle.Keys[i].ID
			now := time.Now()

			// Securely wipe the private key in storage before removing it
			if err := crypto.WipeKeyPair(bundle.Keys[i].KeyPair); err != nil {
				return nil, fmt.Errorf("failed to wipe private key material: %w", err)
			}

			// Remove the key from the bundle completely
			// First, create a new slice without the used key
			newKeys := make([]PreKey, 0, len(bundle.Keys)-1)
			for j := range bundle.Keys {
				if j != i {
					newKeys = append(newKeys, bundle.Keys[j])
				}
			}

			// Update the bundle
			bundle.Keys = newKeys
			bundle.UsedCount++

			// Save updated bundle to disk
			if err := pks.saveBundleToDisk(bundle); err != nil {
				return nil, fmt.Errorf("failed to save updated bundle: %w", err)
			}

			// Return the copy
			result := PreKey{
				ID:      keyID,
				KeyPair: keyPairCopy,
				Used:    true,
				UsedAt:  &now,
			}
			return &result, nil
		}
	}

	return nil, fmt.Errorf("no available pre-keys for peer %x", peerPK[:8])
} // NeedsRefresh checks if a peer's pre-key bundle needs refreshing
func (pks *PreKeyStore) NeedsRefresh(peerPK [32]byte) bool {
	pks.mutex.RLock()
	defer pks.mutex.RUnlock()

	bundle, exists := pks.bundles[peerPK]
	if !exists {
		return true // No bundle exists, needs initial generation
	}

	availableKeys := PreKeysPerPeer - bundle.UsedCount
	if availableKeys <= PreKeyRefreshThreshold {
		return true
	}

	// Check if bundle is too old
	if time.Since(bundle.CreatedAt) > MaxPreKeyAge {
		return true
	}

	return false
}

// RefreshPreKeys generates new pre-keys for a peer, replacing old ones
func (pks *PreKeyStore) RefreshPreKeys(peerPK [32]byte) (*PreKeyBundle, error) {
	pks.mutex.Lock()
	defer pks.mutex.Unlock()

	// Remove old bundle if it exists
	if oldBundle, exists := pks.bundles[peerPK]; exists {
		delete(pks.bundles, peerPK)
		// Remove old bundle file
		if err := pks.removeBundleFromDisk(oldBundle); err != nil {
			// Log but don't fail - continue with refresh
			fmt.Printf("Warning: failed to remove old bundle from disk: %v\n", err)
		}
	}

	// Temporarily release lock to call GeneratePreKeys
	pks.mutex.Unlock()
	bundle, err := pks.GeneratePreKeys(peerPK)
	pks.mutex.Lock()

	if err != nil {
		return nil, fmt.Errorf("failed to generate new pre-keys: %w", err)
	}

	bundle.LastRefreshOffer = time.Now()
	return bundle, nil
}

// GetBundle returns the pre-key bundle for a peer (for key exchange)
func (pks *PreKeyStore) GetBundle(peerPK [32]byte) (*PreKeyBundle, error) {
	pks.mutex.RLock()
	defer pks.mutex.RUnlock()

	bundle, exists := pks.bundles[peerPK]
	if !exists {
		return nil, fmt.Errorf("no pre-key bundle found for peer %x", peerPK[:8])
	}

	return bundle, nil
}

// GetRemainingKeyCount returns the number of unused keys for a peer
func (pks *PreKeyStore) GetRemainingKeyCount(peerPK [32]byte) int {
	pks.mutex.RLock()
	defer pks.mutex.RUnlock()

	bundle, exists := pks.bundles[peerPK]
	if !exists {
		return 0
	}

	return PreKeysPerPeer - bundle.UsedCount
}

// loadBundles loads all pre-key bundles from disk
func (pks *PreKeyStore) loadBundles() error {
	preKeyDir := filepath.Join(pks.dataDir, "prekeys")
	if _, err := os.Stat(preKeyDir); os.IsNotExist(err) {
		return nil // No pre-keys directory yet
	}

	entries, err := os.ReadDir(preKeyDir)
	if err != nil {
		return fmt.Errorf("failed to read pre-keys directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			ext := filepath.Ext(entry.Name())
			// Check for both encrypted (.enc) and legacy (.json) files
			if ext == ".enc" || ext == ".json" {
				bundlePath := filepath.Join(preKeyDir, entry.Name())
				bundle, err := pks.loadBundleFromDisk(bundlePath)
				if err != nil {
					fmt.Printf("Warning: failed to load bundle %s: %v\n", entry.Name(), err)
					continue
				}
				pks.bundles[bundle.PeerPK] = bundle

				// If we loaded a legacy unencrypted file, re-save it encrypted
				if ext == ".json" {
					// Save it encrypted
					err = pks.saveBundleToDisk(bundle)
					if err != nil {
						fmt.Printf("Warning: failed to re-save bundle encrypted: %v\n", err)
						continue
					}

					// Remove the old unencrypted file
					oldPath := bundlePath
					if err := os.Remove(oldPath); err != nil {
						fmt.Printf("Warning: failed to remove legacy unencrypted bundle: %v\n", err)
					}
				}
			}
		}
	}

	return nil
}

// saveBundleToDisk saves a pre-key bundle to disk with encryption
func (pks *PreKeyStore) saveBundleToDisk(bundle *PreKeyBundle) error {
	preKeyDir := filepath.Join(pks.dataDir, "prekeys")
	if err := os.MkdirAll(preKeyDir, 0755); err != nil {
		return fmt.Errorf("failed to create pre-keys directory: %w", err)
	}

	filename := fmt.Sprintf("%x.json.enc", bundle.PeerPK)
	bundlePath := filepath.Join(preKeyDir, filename)

	// Marshal the data
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal bundle: %w", err)
	}

	// Encrypt the data using our identity key as the encryption key
	encryptedData, err := encryptData(data, pks.keyPair.Private[:])
	if err != nil {
		return fmt.Errorf("failed to encrypt bundle data: %w", err)
	}

	// Write the encrypted data to disk with more restrictive permissions
	if err := os.WriteFile(bundlePath, encryptedData, 0600); err != nil {
		return fmt.Errorf("failed to write bundle to disk: %w", err)
	}

	return nil
}

// loadBundleFromDisk loads a pre-key bundle from disk and decrypts it
func (pks *PreKeyStore) loadBundleFromDisk(bundlePath string) (*PreKeyBundle, error) {
	encryptedData, err := os.ReadFile(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle file: %w", err)
	}

	// Check if the file is encrypted (has .enc extension)
	isEncrypted := strings.HasSuffix(bundlePath, ".enc")

	var data []byte
	if isEncrypted {
		// Decrypt the data
		data, err = decryptData(encryptedData, pks.keyPair.Private[:])
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt bundle file: %w", err)
		}
	} else {
		// Handle legacy unencrypted files (for backward compatibility)
		data = encryptedData
	}

	var bundle PreKeyBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bundle: %w", err)
	}

	return &bundle, nil
}

// removeBundleFromDisk removes a pre-key bundle file from disk
func (pks *PreKeyStore) removeBundleFromDisk(bundle *PreKeyBundle) error {
	preKeyDir := filepath.Join(pks.dataDir, "prekeys")
	filename := fmt.Sprintf("%x.json", bundle.PeerPK)
	bundlePath := filepath.Join(preKeyDir, filename)

	if err := os.Remove(bundlePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove bundle file: %w", err)
	}

	return nil
}

// ListPeers returns all peers with pre-key bundles
func (pks *PreKeyStore) ListPeers() [][32]byte {
	pks.mutex.RLock()
	defer pks.mutex.RUnlock()

	peers := make([][32]byte, 0, len(pks.bundles))
	for peerPK := range pks.bundles {
		peers = append(peers, peerPK)
	}

	return peers
}

// GetPreKeyByID finds a specific pre-key by peer and key ID
func (pks *PreKeyStore) GetPreKeyByID(peerPK [32]byte, keyID uint32) (*PreKey, error) {
	pks.mutex.RLock()
	defer pks.mutex.RUnlock()

	bundle, exists := pks.bundles[peerPK]
	if !exists {
		return nil, fmt.Errorf("no pre-key bundle found for peer %x", peerPK[:8])
	}

	for i := range bundle.Keys {
		if bundle.Keys[i].ID == keyID {
			return &bundle.Keys[i], nil
		}
	}

	return nil, fmt.Errorf("pre-key %d not found for peer %x", keyID, peerPK[:8])
}

// MarkPreKeyUsed marks a specific pre-key as used and securely erases its private key data
func (pks *PreKeyStore) MarkPreKeyUsed(peerPK [32]byte, keyID uint32) error {
	pks.mutex.Lock()
	defer pks.mutex.Unlock()

	bundle, exists := pks.bundles[peerPK]
	if !exists {
		return fmt.Errorf("no pre-key bundle found for peer %x", peerPK[:8])
	}

	for i := range bundle.Keys {
		if bundle.Keys[i].ID == keyID {
			if bundle.Keys[i].Used {
				return fmt.Errorf("pre-key %d already marked as used", keyID)
			}

			// Securely wipe the private key material
			if err := crypto.WipeKeyPair(bundle.Keys[i].KeyPair); err != nil {
				return fmt.Errorf("failed to securely wipe key: %w", err)
			}

			bundle.Keys[i].Used = true
			now := time.Now()
			bundle.Keys[i].UsedAt = &now
			bundle.UsedCount++

			// Save updated bundle to disk
			if err := pks.saveBundleToDisk(bundle); err != nil {
				return fmt.Errorf("failed to save updated bundle: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("pre-key %d not found for peer %x", keyID, peerPK[:8])
}

// CleanupExpiredBundles removes old or fully used pre-key bundles
func (pks *PreKeyStore) CleanupExpiredBundles() int {
	pks.mutex.Lock()
	defer pks.mutex.Unlock()

	cleaned := 0
	for peerPK, bundle := range pks.bundles {
		// Remove if all keys used or bundle is too old
		if bundle.UsedCount >= PreKeysPerPeer || time.Since(bundle.CreatedAt) > MaxPreKeyAge {
			delete(pks.bundles, peerPK)
			if err := pks.removeBundleFromDisk(bundle); err != nil {
				fmt.Printf("Warning: failed to remove expired bundle from disk: %v\n", err)
			}
			cleaned++
		}
	}

	return cleaned
}
