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

// SignedPreKey is a medium-term Curve25519 key whose public half is
// Ed25519-signed by the owner's long-term identity key.
//
// It serves the same role as Signal Protocol's signed pre-key (SPK) in
// an X3DH exchange: it binds the pre-key bundle to the owner's identity,
// preventing a relay or storage node from substituting a bogus bundle.
//
// The key is rotated every [SignedPreKeyRotationInterval] (7 days by default).
type SignedPreKey struct {
	// ID uniquely identifies this signed pre-key within the owner's key history.
	ID uint32 `json:"id"`
	// PublicKey is the Curve25519 public key being signed.
	PublicKey [32]byte `json:"public_key"`
	// Signature is the Ed25519 signature of PublicKey made with the owner's
	// identity key. Verify with [SignerPK].
	Signature [64]byte `json:"signature"`
	// SignerPK is the Ed25519 public key derived from the owner's Curve25519
	// identity private key. Receivers must verify that SignerPK matches the
	// sender's known identity before accepting the bundle.
	SignerPK [32]byte `json:"signer_pk"`
	// CreatedAt records when this signed pre-key was generated.
	CreatedAt time.Time `json:"created_at"`
	// ExpiresAt is the earliest time at which this key may be replaced.
	ExpiresAt time.Time `json:"expires_at"`
}

// NewSignedPreKey generates a fresh Curve25519 key pair and signs the public
// key with the caller's Ed25519 identity key. The resulting SignedPreKey is
// valid for [SignedPreKeyRotationInterval].
func NewSignedPreKey(id uint32, identityPrivKey [32]byte) (*SignedPreKey, error) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("signed pre-key: generate key pair: %w", err)
	}
	defer crypto.WipeKeyPair(kp)

	sig, err := crypto.Sign(kp.Public[:], identityPrivKey)
	if err != nil {
		return nil, fmt.Errorf("signed pre-key: sign public key: %w", err)
	}

	signerPK := crypto.GetSignaturePublicKey(identityPrivKey)

	now := time.Now()
	spk := &SignedPreKey{
		ID:        id,
		PublicKey: kp.Public,
		SignerPK:  signerPK,
		CreatedAt: now,
		ExpiresAt: now.Add(SignedPreKeyRotationInterval),
	}
	copy(spk.Signature[:], sig[:])
	return spk, nil
}

// Verify checks that the SignedPreKey's public key was signed by the claimed
// signer public key. Returns an error if the signature is invalid.
func (spk *SignedPreKey) Verify() error {
	valid, err := crypto.Verify(spk.PublicKey[:], spk.Signature, spk.SignerPK)
	if err != nil {
		return fmt.Errorf("signed pre-key verification error: %w", err)
	}
	if !valid {
		return fmt.Errorf("signed pre-key has invalid signature")
	}
	return nil
}

// ShouldRotate reports whether this signed pre-key has passed its expiration
// time and should be replaced with a freshly generated one.
func (spk *SignedPreKey) ShouldRotate() bool {
	return time.Now().After(spk.ExpiresAt)
}

// Pre-key management constants control key generation and rotation for forward secrecy.
const (
	// PreKeysPerPeer is the number of pre-keys to generate per peer for forward secrecy.
	PreKeysPerPeer = 200
	// PreKeyRefreshThreshold triggers key refresh when remaining keys fall below this count.
	PreKeyRefreshThreshold = 50 // Refresh when less than 50 keys remain
	// MaxPreKeyAge is the maximum duration a pre-key remains valid before expiration.
	MaxPreKeyAge = 30 * 24 * time.Hour // 30 days

	// SignedPreKeyRotationInterval is how often the signed pre-key is rotated.
	// Signal Protocol rotates its signed pre-key weekly; we follow the same cadence.
	SignedPreKeyRotationInterval = 7 * 24 * time.Hour
)

// NewPreKeyStore creates a new pre-key storage manager
func NewPreKeyStore(keyPair *crypto.KeyPair, dataDir string) (*PreKeyStore, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
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

// generatePreKeyBundle creates a bundle of pre-keys without acquiring locks.
// This is a private helper method used by both GeneratePreKeys and RefreshPreKeys.
func (pks *PreKeyStore) generatePreKeyBundle(peerPK [32]byte) (*PreKeyBundle, error) {
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

// GeneratePreKeys creates a new bundle of one-time keys for a peer
func (pks *PreKeyStore) GeneratePreKeys(peerPK [32]byte) (*PreKeyBundle, error) {
	pks.mutex.Lock()
	defer pks.mutex.Unlock()

	return pks.generatePreKeyBundle(peerPK)
}

// GetAvailablePreKey returns an unused pre-key for a peer, if available
func (pks *PreKeyStore) GetAvailablePreKey(peerPK [32]byte) (*PreKey, error) {
	pks.mutex.Lock()
	defer pks.mutex.Unlock()

	bundle, err := pks.findPreKeyBundle(peerPK)
	if err != nil {
		return nil, err
	}

	// Find an unused key
	for i := range bundle.Keys {
		if !bundle.Keys[i].Used {
			preKey, err := pks.extractAndProcessPreKey(bundle, i)
			if err != nil {
				return nil, err
			}
			return preKey, nil
		}
	}

	return nil, fmt.Errorf("no available pre-keys for peer %x", peerPK[:8])
}

// findPreKeyBundle retrieves a pre-key bundle for the specified peer
func (pks *PreKeyStore) findPreKeyBundle(peerPK [32]byte) (*PreKeyBundle, error) {
	bundle, exists := pks.bundles[peerPK]
	if !exists {
		return nil, fmt.Errorf("no pre-key bundle found for peer %x", peerPK[:8])
	}
	return bundle, nil
}

// extractAndProcessPreKey extracts a pre-key from the bundle, processes it and returns a copy
func (pks *PreKeyStore) extractAndProcessPreKey(bundle *PreKeyBundle, keyIndex int) (*PreKey, error) {
	// Create a copy of the key before removing it from storage
	keyPairCopy := pks.copyKeyPair(bundle.Keys[keyIndex].KeyPair)

	// Store key ID for the result
	keyID := bundle.Keys[keyIndex].ID

	// Process the key in the bundle
	if err := pks.processPreKeyInBundle(bundle, keyIndex); err != nil {
		return nil, err
	}

	// Return the copy with timestamp allocated explicitly on the heap
	// to avoid fragile &stack-local pattern.
	usedAt := new(time.Time)
	*usedAt = time.Now()
	result := PreKey{
		ID:      keyID,
		KeyPair: keyPairCopy,
		Used:    true,
		UsedAt:  usedAt,
	}
	return &result, nil
}

// copyKeyPair creates a safe copy of a KeyPair
func (pks *PreKeyStore) copyKeyPair(original *crypto.KeyPair) *crypto.KeyPair {
	return &crypto.KeyPair{
		Public:  original.Public,
		Private: original.Private,
	}
}

// processPreKeyInBundle wipes the key, removes it from the bundle, and saves the updated bundle
func (pks *PreKeyStore) processPreKeyInBundle(bundle *PreKeyBundle, keyIndex int) error {
	// Securely wipe the private key in storage before removing it
	if err := crypto.WipeKeyPair(bundle.Keys[keyIndex].KeyPair); err != nil {
		return fmt.Errorf("failed to wipe private key material: %w", err)
	}

	// Remove the key from the bundle completely
	bundle.Keys = pks.removeKeyFromSlice(bundle.Keys, keyIndex)
	bundle.UsedCount++

	// Save updated bundle to disk
	if err := pks.saveBundleToDisk(bundle); err != nil {
		return fmt.Errorf("failed to save updated bundle: %w", err)
	}

	return nil
}

// removeKeyFromSlice creates a new slice without the key at the specified index
func (pks *PreKeyStore) removeKeyFromSlice(keys []PreKey, indexToRemove int) []PreKey {
	newKeys := make([]PreKey, 0, len(keys)-1)
	for j := range keys {
		if j != indexToRemove {
			newKeys = append(newKeys, keys[j])
		}
	}
	return newKeys
}

// NeedsRefresh checks if a peer's pre-key bundle needs refreshing
func (pks *PreKeyStore) NeedsRefresh(peerPK [32]byte) bool {
	pks.mutex.RLock()
	defer pks.mutex.RUnlock()

	bundle, exists := pks.bundles[peerPK]
	if !exists {
		return true // No bundle exists, needs initial generation
	}

	availableKeys := len(bundle.Keys)
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

	// Generate new bundle while holding the lock continuously
	bundle, err := pks.generatePreKeyBundle(peerPK)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new pre-keys: %w", err)
	}

	bundle.LastRefreshOffer = time.Now()
	return bundle, nil
}

// GetBundle returns a deep copy of the pre-key bundle for a peer (for key exchange)
func (pks *PreKeyStore) GetBundle(peerPK [32]byte) (*PreKeyBundle, error) {
	pks.mutex.RLock()
	defer pks.mutex.RUnlock()

	bundle, exists := pks.bundles[peerPK]
	if !exists {
		return nil, fmt.Errorf("no pre-key bundle found for peer %x", peerPK[:8])
	}

	return clonePreKeyBundle(bundle), nil
}

// clonePreKeyBundle returns a deep copy of b so the caller cannot alias
// the store's internal slice or KeyPair pointers.
func clonePreKeyBundle(b *PreKeyBundle) *PreKeyBundle {
	cp := *b
	cp.Keys = make([]PreKey, len(b.Keys))
	for i, k := range b.Keys {
		cp.Keys[i] = clonePreKey(k)
	}
	return &cp
}


// GetRemainingKeyCount returns the number of unused keys for a peer
func (pks *PreKeyStore) GetRemainingKeyCount(peerPK [32]byte) int {
	pks.mutex.RLock()
	defer pks.mutex.RUnlock()

	bundle, exists := pks.bundles[peerPK]
	if !exists {
		return 0
	}

	return len(bundle.Keys)
}

// loadBundles loads all pre-key bundles from disk
// loadBundles loads all prekey bundles from disk into memory.
// It handles both encrypted and legacy unencrypted bundle files.
func (pks *PreKeyStore) loadBundles() error {
	preKeyDir := filepath.Join(pks.dataDir, "prekeys")

	// Check if directory exists
	if dirExists, err := pks.checkPreKeyDirectoryExists(preKeyDir); !dirExists {
		return err // Will be nil when directory doesn't exist
	}

	entries, err := os.ReadDir(preKeyDir)
	if err != nil {
		return fmt.Errorf("failed to read pre-keys directory: %w", err)
	}

	// Process all bundle files
	return pks.processBundleEntries(entries, preKeyDir)
}

// saveBundleToDisk saves a pre-key bundle to disk with encryption
func (pks *PreKeyStore) saveBundleToDisk(bundle *PreKeyBundle) error {
	preKeyDir := filepath.Join(pks.dataDir, "prekeys")
	if err := os.MkdirAll(preKeyDir, 0o755); err != nil {
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
	if err := os.WriteFile(bundlePath, encryptedData, 0o600); err != nil {
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
	filename := fmt.Sprintf("%x.json.enc", bundle.PeerPK)
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

// validatePreKeyBundle retrieves and validates that a pre-key bundle exists for the given peer.
// Returns the bundle if found, otherwise returns an error.
func (pks *PreKeyStore) validatePreKeyBundle(peerPK [32]byte) (*PreKeyBundle, error) {
	bundle, exists := pks.bundles[peerPK]
	if !exists {
		return nil, fmt.Errorf("no pre-key bundle found for peer %x", peerPK[:8])
	}
	return bundle, nil
}

// markKeyAsUsedSecurely finds the specified key by ID, validates it's not already used,
// securely wipes its private key material, and marks it as used.
// Returns an error if the key is not found, already used, or wiping fails.
func (pks *PreKeyStore) markKeyAsUsedSecurely(bundle *PreKeyBundle, keyID uint32) error {
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

			return nil
		}
	}

	return fmt.Errorf("pre-key %d not found", keyID)
}

// persistBundleChanges saves the updated pre-key bundle to persistent storage.
// Returns an error if the save operation fails.
func (pks *PreKeyStore) persistBundleChanges(bundle *PreKeyBundle) error {
	if err := pks.saveBundleToDisk(bundle); err != nil {
		return fmt.Errorf("failed to save updated bundle: %w", err)
	}
	return nil
}

// MarkPreKeyUsed marks a specific pre-key as used and securely erases its private key data
func (pks *PreKeyStore) MarkPreKeyUsed(peerPK [32]byte, keyID uint32) error {
	pks.mutex.Lock()
	defer pks.mutex.Unlock()

	bundle, err := pks.validatePreKeyBundle(peerPK)
	if err != nil {
		return err
	}

	if err := pks.markKeyAsUsedSecurely(bundle, keyID); err != nil {
		return fmt.Errorf("failed to mark pre-key as used for peer %x: %w", peerPK[:8], err)
	}

	return pks.persistBundleChanges(bundle)
}

// clonePreKey snapshots a pre-key before secure wiping mutates its key material.
func clonePreKey(preKey PreKey) PreKey {
	snapshot := preKey
	if preKey.KeyPair != nil {
		copyKeyPair := *preKey.KeyPair
		snapshot.KeyPair = &copyKeyPair
	}
	return snapshot
}

// findUnusedPreKey locates a pre-key and rejects already-consumed entries.
func findUnusedPreKey(bundle *PreKeyBundle, keyID uint32, peerPK [32]byte) (*PreKey, error) {
	for i := range bundle.Keys {
		if bundle.Keys[i].ID != keyID {
			continue
		}
		if bundle.Keys[i].Used {
			return nil, fmt.Errorf("pre-key %d already used - possible replay attack", keyID)
		}
		snapshot := clonePreKey(bundle.Keys[i])
		return &snapshot, nil
	}
	return nil, fmt.Errorf("pre-key %d not found for peer %x", keyID, peerPK[:8])
}

// CheckAndMarkPreKeyUsed atomically checks if a pre-key is unused and marks it used.
// Returns a copy of the pre-key (before wiping private key material) on success,
// or an error if the key is not found or was already consumed.
// The check-and-mark are performed under a single Lock to prevent two concurrent
// goroutines from both passing the Used==false check (TOCTOU).
func (pks *PreKeyStore) CheckAndMarkPreKeyUsed(peerPK [32]byte, keyID uint32) (*PreKey, error) {
	pks.mutex.Lock()
	defer pks.mutex.Unlock()
	bundle, err := pks.validatePreKeyBundle(peerPK)
	if err != nil {
		return nil, fmt.Errorf("failed to find pre-key %d for sender %x: %w", keyID, peerPK[:8], err)
	}
	snapshot, err := findUnusedPreKey(bundle, keyID, peerPK)
	if err != nil {
		return nil, err
	}
	if err := pks.markKeyAsUsedSecurely(bundle, keyID); err != nil {
		return nil, fmt.Errorf("failed to mark pre-key as used: %w", err)
	}
	if err := pks.persistBundleChanges(bundle); err != nil {
		return nil, err
	}
	return snapshot, nil
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

// checkPreKeyDirectoryExists checks if the prekey directory exists.
// Returns true if the directory exists, false otherwise.
func (pks *PreKeyStore) checkPreKeyDirectoryExists(preKeyDir string) (bool, error) {
	if _, err := os.Stat(preKeyDir); os.IsNotExist(err) {
		return false, nil // No pre-keys directory yet
	}
	return true, nil
}

// processBundleEntries processes all directory entries, looking for bundle files.
func (pks *PreKeyStore) processBundleEntries(entries []os.DirEntry, preKeyDir string) error {
	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip directories
		}

		ext := filepath.Ext(entry.Name())
		// Only process bundle files
		if ext != ".enc" && ext != ".json" {
			continue
		}

		bundlePath := filepath.Join(preKeyDir, entry.Name())
		if err := pks.processBundleFile(bundlePath, ext); err != nil {
			// Non-fatal error, log and continue
			fmt.Printf("Warning: %v\n", err)
		}
	}
	return nil
}

// processBundleFile loads a single bundle file and handles conversion if needed.
func (pks *PreKeyStore) processBundleFile(bundlePath, ext string) error {
	// Load the bundle
	bundle, err := pks.loadBundleFromDisk(bundlePath)
	if err != nil {
		// Silently skip bundles that fail authentication - they belong to different identities
		// This is common when running tests with different key pairs
		if strings.Contains(err.Error(), "cipher: message authentication failed") {
			return nil // Skip silently - bundle belongs to a different identity
		}
		return fmt.Errorf("failed to load bundle %s: %w", filepath.Base(bundlePath), err)
	}

	// Store in memory
	pks.bundles[bundle.PeerPK] = bundle

	// Convert legacy unencrypted file if needed
	if ext == ".json" {
		return pks.convertLegacyBundle(bundle, bundlePath)
	}

	return nil
}

// convertLegacyBundle converts a legacy unencrypted bundle to encrypted format.
func (pks *PreKeyStore) convertLegacyBundle(bundle *PreKeyBundle, oldPath string) error {
	// Save it encrypted
	if err := pks.saveBundleToDisk(bundle); err != nil {
		return fmt.Errorf("failed to re-save bundle encrypted: %w", err)
	}

	// Remove the old unencrypted file
	if err := os.Remove(oldPath); err != nil {
		return fmt.Errorf("failed to remove legacy unencrypted bundle: %w", err)
	}

	return nil
}

// PreKeyBackup is a portable, JSON-serialisable snapshot of all pre-key
// bundles held by a PreKeyStore.  It is designed to survive a user restoring
// from backup: by importing the snapshot the restored client avoids exhausting
// the peer's remaining pre-key pool immediately.
type PreKeyBackup struct {
	// Version identifies the backup format for future compatibility.
	Version int `json:"version"`
	// Bundles contains every in-memory pre-key bundle at the time of export.
	// Unused pre-keys are preserved; already-consumed keys are not included.
	Bundles []*PreKeyBundle `json:"bundles"`
}

const preKeyBackupVersion = 1

// ExportPreKeys produces a PreKeyBackup snapshot of all pre-key bundles
// currently held in memory.  The returned value can be serialised to JSON and
// stored alongside other backup material.  Only unused (non-consumed) pre-keys
// are included so that the backup does not replay already-used key material.
func (pks *PreKeyStore) ExportPreKeys() *PreKeyBackup {
	pks.mutex.RLock()
	defer pks.mutex.RUnlock()

	backup := &PreKeyBackup{
		Version: preKeyBackupVersion,
		Bundles: make([]*PreKeyBundle, 0, len(pks.bundles)),
	}
	for _, bundle := range pks.bundles {
		cp := &PreKeyBundle{
			PeerPK:           bundle.PeerPK,
			CreatedAt:        bundle.CreatedAt,
			UsedCount:        bundle.UsedCount,
			MaxKeys:          bundle.MaxKeys,
			LastRefreshOffer: bundle.LastRefreshOffer,
		}
		for _, k := range bundle.Keys {
			if !k.Used {
				cp.Keys = append(cp.Keys, clonePreKey(k))
			}
		}
		backup.Bundles = append(backup.Bundles, cp)
	}
	return backup
}

// ImportPreKeys merges pre-key bundles from a PreKeyBackup into the store.
// For each peer, imported unused keys are prepended to any existing bundle so
// that the restored client has the maximum available pool.  Bundles for peers
// that are not yet known locally are added wholesale.  Existing keys are not
// duplicated (checked by key ID).
func (pks *PreKeyStore) ImportPreKeys(backup *PreKeyBackup) error {
	if backup == nil {
		return fmt.Errorf("pre-key backup is nil")
	}
	if backup.Version != preKeyBackupVersion {
		return fmt.Errorf("unsupported pre-key backup version %d (want %d)", backup.Version, preKeyBackupVersion)
	}

	pks.mutex.Lock()
	defer pks.mutex.Unlock()

	for _, imported := range backup.Bundles {
		if imported == nil {
			continue
		}
		existing, ok := pks.bundles[imported.PeerPK]
		if !ok {
			// Unknown peer — adopt the bundle wholesale, deep-copying so the
			// store never aliases the caller's KeyPair pointers (M-17).
			cp := clonePreKeyBundle(imported)
			pks.bundles[imported.PeerPK] = cp
			if err := pks.saveBundleToDisk(cp); err != nil {
				return fmt.Errorf("failed to persist imported bundle for peer %x: %w", imported.PeerPK[:4], err)
			}
			continue
		}

		// Build a set of known key IDs to avoid duplicates.
		knownIDs := make(map[uint32]bool, len(existing.Keys))
		for _, k := range existing.Keys {
			knownIDs[k.ID] = true
		}

		added := 0
		for _, k := range imported.Keys {
			if !k.Used && !knownIDs[k.ID] {
				existing.Keys = append(existing.Keys, k)
				knownIDs[k.ID] = true
				added++
			}
		}

		if added > 0 {
			if err := pks.saveBundleToDisk(existing); err != nil {
				return fmt.Errorf("failed to persist merged bundle for peer %x: %w", imported.PeerPK[:4], err)
			}
		}
	}
	return nil
}
