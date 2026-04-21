// toxcore_lifecycle.go contains lifecycle-related functionality including
// iteration, shutdown, state persistence, and time management.

package toxcore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/file"
	"github.com/sirupsen/logrus"
)

// Iterate performs a single iteration of the Tox event loop.
//
//export ToxIterate
func (t *Tox) Iterate() {
	t.iterationCount++

	// Process DHT maintenance
	t.doDHTMaintenance()

	// Process friend connections
	t.doFriendConnections()

	// Process message queue
	t.doMessageProcessing()

	// Retry pending friend requests (production retry queue)
	t.retryPendingFriendRequests()
}

// IterationInterval returns the recommended interval between Iterate() calls.
//
//export ToxIterationInterval
func (t *Tox) IterationInterval() time.Duration {
	return t.iterationTime
}

// IsRunning checks if the Tox instance is still running.
//
//export ToxIsRunning
func (t *Tox) IsRunning() bool {
	return t.running
}

// SetTimeProvider sets a custom time provider for deterministic testing.
// This should only be used in tests. In production, the default RealTimeProvider is used.
func (t *Tox) SetTimeProvider(tp TimeProvider) {
	t.timeProvider = tp
}

// now returns the current time using the configured time provider.
func (t *Tox) now() time.Time {
	return t.timeProvider.Now()
}

// Kill stops the Tox instance and releases all resources.
//
//export ToxKill
func (t *Tox) Kill() {
	t.running = false
	t.cancel()

	t.closeTransports()
	t.stopBackgroundServices()
	t.cleanupManagers()
	t.clearCallbacks()
}

// closeTransports closes UDP and TCP transport connections.
func (t *Tox) closeTransports() {
	if t.udpTransport != nil {
		if err := t.udpTransport.Close(); err != nil {
			logrus.WithError(err).Warn("Failed to close UDP transport")
		}
	}

	if t.tcpTransport != nil {
		if err := t.tcpTransport.Close(); err != nil {
			logrus.WithError(err).Warn("Failed to close TCP transport")
		}
	}
}

// stopBackgroundServices stops async manager and LAN discovery services.
func (t *Tox) stopBackgroundServices() {
	t.stopAsyncManager()
	t.stopLANDiscovery()
	t.closeNATTraversal()
	t.clearDHT()
	t.clearBootstrapManager()
}

// stopAsyncManager stops the async manager if running.
func (t *Tox) stopAsyncManager() {
	if t.asyncManager != nil {
		t.asyncManager.Stop()
	}
}

// stopLANDiscovery stops the LAN discovery service if running.
func (t *Tox) stopLANDiscovery() {
	if t.lanDiscovery != nil {
		t.lanDiscovery.Stop()
	}
}

// closeNATTraversal closes and clears the NAT traversal component.
func (t *Tox) closeNATTraversal() {
	if t.natTraversal != nil {
		if err := t.natTraversal.Close(); err != nil {
			logrus.WithError(err).Warn("Failed to close NAT traversal")
		}
		t.natTraversal = nil
	}
}

// clearDHT clears the DHT reference under lock.
func (t *Tox) clearDHT() {
	t.dhtMutex.Lock()
	t.dht = nil
	t.dhtMutex.Unlock()
}

// clearBootstrapManager clears the bootstrap manager reference.
func (t *Tox) clearBootstrapManager() {
	if t.bootstrapManager != nil {
		t.bootstrapManager = nil
	}
}

// cleanupManagers cleans up all manager instances and the friends list.
func (t *Tox) cleanupManagers() {
	t.messageManagerMu.Lock()
	if t.messageManager != nil {
		t.messageManager = nil
	}
	t.messageManagerMu.Unlock()

	if t.fileManager != nil {
		t.fileManager = nil
	}

	if t.requestManager != nil {
		t.requestManager = nil
	}

	t.cancelActiveFileTransfers()

	// Clear the friends store
	if t.friends != nil {
		t.friends.Clear()
	}
}

// cancelActiveFileTransfers cancels all in-progress file transfers and closes
// their file handles to prevent file descriptor leaks on shutdown.
// Transfers are copied out of the map under the lock; Cancel() is then called
// without holding the lock so that completeCallbacks (which may need Tox state)
// cannot deadlock against transfersMu.
func (t *Tox) cancelActiveFileTransfers() {
	t.transfersMu.Lock()
	pending := make(map[uint64]*file.Transfer, len(t.fileTransfers))
	for k, v := range t.fileTransfers {
		pending[k] = v
	}
	t.fileTransfers = make(map[uint64]*file.Transfer)
	t.transfersMu.Unlock()

	for key, transfer := range pending {
		if err := transfer.Cancel(); err != nil {
			if errors.Is(err, file.ErrTransferAlreadyFinished) {
				logrus.WithField("transfer_key", key).
					Debug("cancelActiveFileTransfers: transfer already finished, skipping")
			} else {
				logrus.WithFields(logrus.Fields{
					"function":     "cancelActiveFileTransfers",
					"transfer_key": key,
					"error":        err.Error(),
				}).Warn("Failed to cancel file transfer during shutdown")
			}
		}
	}
}

// clearCallbacks clears all callback functions to prevent memory leaks.
func (t *Tox) clearCallbacks() {
	t.friendRequestCallback = nil
	t.friendMessageCallback = nil
	t.simpleFriendMessageCallback = nil
	t.friendStatusCallback = nil
	t.connectionStatusCallback = nil
	t.friendConnectionStatusCallback = nil
	t.friendStatusChangeCallback = nil
}

// doDHTMaintenance performs periodic DHT maintenance tasks.
// Runs every ~6 seconds (120 iterations × 50 ms tick) to avoid flooding the network.
func (t *Tox) doDHTMaintenance() {
	if t.dht == nil || t.keyPair == nil || t.bootstrapManager == nil {
		return
	}

	// Rate-limit: run once every 120 iterations (~6 s at 50 ms/tick).
	if t.iterationCount%120 != 0 {
		return
	}

	selfToxID := crypto.NewToxID(t.keyPair.Public, t.nospam)
	allNodes := t.dht.FindClosestNodes(*selfToxID, 100)

	if len(allNodes) < 10 {
		// Routing table is sparse — re-bootstrap to replenish it.
		ctx, cancel := context.WithTimeout(t.ctx, t.options.BootstrapTimeout)
		defer cancel()

		if err := t.bootstrapManager.Bootstrap(ctx); err != nil {
			logrus.WithFields(logrus.Fields{
				"function":   "doDHTMaintenance",
				"node_count": len(allNodes),
				"error":      err.Error(),
			}).Debug("DHT re-bootstrap attempt failed")
		}
	} else {
		// Routing table has nodes — send FIND_NODE queries toward our own key to
		// keep buckets fresh. We reuse Bootstrap to ping the known bootstrap nodes;
		// a full FIND_NODE walk is handled by the DHT Maintainer when present.
		if t.bootstrapManager.IsBootstrapped() {
			return
		}
		ctx, cancel := context.WithTimeout(t.ctx, t.options.BootstrapTimeout)
		defer cancel()
		_ = t.bootstrapManager.Bootstrap(ctx) //nolint:errcheck // best-effort refresh
	}
}

// doFriendConnections manages friend connections.
// Rate-limited to every 240 iterations (~12 s at 50 ms/tick).
func (t *Tox) doFriendConnections() {
	if !t.shouldRunFriendConnections() {
		return
	}

	offlineKeys := t.collectOfflineFriendKeys()
	if len(offlineKeys) == 0 {
		return
	}

	t.scheduleFriendRequestRetries(offlineKeys)
}

// shouldRunFriendConnections checks if friend connection processing should run.
func (t *Tox) shouldRunFriendConnections() bool {
	if t.friends.Count() == 0 || t.dht == nil {
		return false
	}
	// Only run every 240 iterations to avoid DHT flooding.
	return t.iterationCount%240 == 0
}

// collectOfflineFriendKeys returns public keys of all offline friends.
func (t *Tox) collectOfflineFriendKeys() [][32]byte {
	offlineKeys := make([][32]byte, 0, t.friends.Count())
	t.friends.Range(func(_ uint32, f *Friend) bool {
		if f.ConnectionStatus == ConnectionNone {
			offlineKeys = append(offlineKeys, f.PublicKey)
		}
		return true
	})
	return offlineKeys
}

// scheduleFriendRequestRetries schedules immediate retries for pending friend requests
// when DHT routes are found for offline friends.
func (t *Tox) scheduleFriendRequestRetries(offlineKeys [][32]byte) {
	now := t.now()
	t.pendingFriendReqsMux.Lock()
	defer t.pendingFriendReqsMux.Unlock()

	for _, pk := range offlineKeys {
		t.maybeScheduleRetryForFriend(pk, now)
	}
}

// maybeScheduleRetryForFriend checks if a DHT route exists for the given friend
// and schedules immediate retry if so.
func (t *Tox) maybeScheduleRetryForFriend(pk [32]byte, now time.Time) {
	friendToxID := crypto.NewToxID(pk, [4]byte{})
	nodes := t.dht.FindClosestNodes(*friendToxID, 1)
	if len(nodes) == 0 {
		return
	}

	for i, req := range t.pendingFriendReqs {
		if req.targetPublicKey == pk && now.After(req.nextRetry) {
			t.pendingFriendReqs[i].nextRetry = now // schedule immediate retry
			logrus.WithFields(logrus.Fields{
				"function":  "doFriendConnections",
				"target_pk": fmt.Sprintf("%x", pk[:8]),
				"dht_nodes": len(nodes),
			}).Debug("DHT route found for offline friend, scheduling request retry")
		}
	}
}

// Save saves the Tox state to a byte slice.
//
//export ToxSave
func (t *Tox) Save() ([]byte, error) {
	// Use the existing GetSavedata implementation to serialize state
	savedata := t.GetSavedata()
	if savedata == nil {
		return nil, errors.New("failed to serialize Tox state")
	}

	return savedata, nil
}

// Load loads the Tox state from a byte slice created by GetSavedata.
// This method restores the private key, friends list, and configuration
// from previously saved data.
//
// The Tox instance must be in a clean state before calling Load.
// This method will overwrite existing keys and friends.
//
// Load restores the Tox instance state from saved data.
//
// The function loads a previously saved Tox state including keypair,
// friends list, self information, and nospam value. It validates the
// data integrity and maintains backward compatibility with older formats.
//
//export ToxLoad
func (t *Tox) Load(data []byte) error {
	if err := t.validateLoadData(data); err != nil {
		return err
	}

	saveData, err := t.unmarshalSaveData(data)
	if err != nil {
		return err
	}

	if err := t.restoreKeyPair(saveData); err != nil {
		return err
	}

	t.restoreFriendsList(saveData)
	t.restoreOptions(saveData)
	t.restoreSelfInformation(saveData)
	if err := t.restoreNospamValue(saveData); err != nil {
		return err
	}

	return nil
}

// SaveSnapshot saves the Tox state in binary snapshot format for faster recovery.
// The binary format is significantly faster to serialize/deserialize than JSON,
// making it suitable for frequent checkpoints and fast startup times.
//
// The snapshot format includes a magic header and version for compatibility checking.
// Use LoadSnapshot or Load (which auto-detects format) to restore the state.
//
//export ToxSaveSnapshot
func (t *Tox) SaveSnapshot() ([]byte, error) {
	t.selfMutex.RLock()
	defer t.selfMutex.RUnlock()

	saveData := toxSaveData{
		KeyPair:       t.keyPair,
		Friends:       make(map[uint32]*Friend),
		Options:       t.options,
		SelfName:      t.selfName,
		SelfStatusMsg: t.selfStatusMsg,
		Nospam:        t.nospam,
	}

	// Copy friends data using sharded store's GetAll
	for id, f := range t.friends.GetAll() {
		saveData.Friends[id] = &Friend{
			PublicKey:        f.PublicKey,
			Status:           f.Status,
			ConnectionStatus: f.ConnectionStatus,
			Name:             f.Name,
			StatusMessage:    f.StatusMessage,
			LastSeen:         f.LastSeen,
		}
	}

	return saveData.marshalBinary()
}

// LoadSnapshot loads the Tox state from a binary snapshot.
// This is an explicit method for loading binary snapshots. The regular Load
// method will also auto-detect and load binary snapshots.
//
//export ToxLoadSnapshot
func (t *Tox) LoadSnapshot(data []byte) error {
	if err := t.validateLoadData(data); err != nil {
		return err
	}

	if !isSnapshotFormat(data) {
		return errors.New("not a binary snapshot format")
	}

	var saveData toxSaveData
	if err := saveData.unmarshalBinary(data); err != nil {
		return fmt.Errorf("snapshot unmarshal: %w", err)
	}

	if err := t.restoreKeyPair(&saveData); err != nil {
		return err
	}

	t.restoreFriendsList(&saveData)
	t.restoreOptions(&saveData)
	t.restoreSelfInformation(&saveData)
	if err := t.restoreNospamValue(&saveData); err != nil {
		return err
	}

	return nil
}

// validateLoadData checks if the provided save data is valid for loading.
func (t *Tox) validateLoadData(data []byte) error {
	if len(data) == 0 {
		return errors.New("save data is empty")
	}
	return nil
}

// unmarshalSaveData parses the binary save data into a structured format.
// Automatically detects binary snapshot format vs legacy JSON format.
func (t *Tox) unmarshalSaveData(data []byte) (*toxSaveData, error) {
	var saveData toxSaveData

	// Auto-detect format
	if isSnapshotFormat(data) {
		if err := saveData.unmarshalBinary(data); err != nil {
			return nil, fmt.Errorf("binary snapshot unmarshal: %w", err)
		}
	} else {
		if err := saveData.unmarshal(data); err != nil {
			return nil, fmt.Errorf("json unmarshal: %w", err)
		}
	}
	return &saveData, nil
}

// restoreKeyPair validates and restores the cryptographic key pair.
func (t *Tox) restoreKeyPair(saveData *toxSaveData) error {
	if saveData.KeyPair == nil {
		return errors.New("save data missing key pair")
	}
	t.keyPair = saveData.KeyPair
	return nil
}

// restoreFriendsList reconstructs the friends list from saved data.
func (t *Tox) restoreFriendsList(saveData *toxSaveData) {
	if saveData.Friends != nil {
		// Clear and re-populate the friends store
		t.friends.Clear()

		// Reset the async known-sender list so it exactly matches the restored
		// friend set.  Without this, repeated Load() calls (e.g., hot-reload)
		// would accumulate stale keys from friends that were removed between saves.
		if t.asyncManager != nil {
			t.asyncManager.ResetKnownSenders()
		}

		for id, f := range saveData.Friends {
			if f != nil {
				t.friends.Set(id, &Friend{
					PublicKey:        f.PublicKey,
					Status:           f.Status,
					ConnectionStatus: f.ConnectionStatus,
					Name:             f.Name,
					StatusMessage:    f.StatusMessage,
					LastSeen:         f.LastSeen,
					// UserData is not restored as it was not serialized
				})
				// Register each restored friend with the async manager so that
				// offline messages sent while we were offline can be decrypted.
				if t.asyncManager != nil {
					t.asyncManager.AddFriend(f.PublicKey)
				}
			}
		}
	}
}

// restoreOptions selectively restores safe configuration options.
func (t *Tox) restoreOptions(saveData *toxSaveData) {
	if saveData.Options != nil && t.options != nil {
		// Only restore certain safe options, not all options should be restored
		// as some are runtime-specific (like network settings)
		t.options.SavedataType = saveData.Options.SavedataType
		t.options.SavedataData = saveData.Options.SavedataData
		t.options.SavedataLength = saveData.Options.SavedataLength
	}
}

// restoreSelfInformation restores the user's profile information.
func (t *Tox) restoreSelfInformation(saveData *toxSaveData) {
	t.selfMutex.Lock()
	defer t.selfMutex.Unlock()
	t.selfName = saveData.SelfName
	t.selfStatusMsg = saveData.SelfStatusMsg
}

// restoreNospamValue restores or generates the nospam value for backward compatibility.
// Returns an error if generation fails for old savedata without nospam.
func (t *Tox) restoreNospamValue(saveData *toxSaveData) error {
	if saveData.Nospam == [4]byte{} {
		// Old savedata without nospam - generate a new one
		nospam, err := generateNospam()
		if err != nil {
			return fmt.Errorf("failed to generate nospam during restore: %w", err)
		}
		t.nospam = nospam
	} else {
		t.nospam = saveData.Nospam
	}
	return nil
}

// loadSavedState loads saved state from options during initialization.
// This method handles different savedata types and integrates with the existing Load functionality.
func (t *Tox) loadSavedState(options *Options) error {
	if options == nil {
		return nil
	}

	switch options.SavedataType {
	case SaveDataTypeNone:
		// No saved data to load
		return nil
	case SaveDataTypeSecretKey:
		// Secret key is already handled in createKeyPair
		return nil
	case SaveDataTypeToxSave:
		// Load complete Tox state including friends
		if len(options.SavedataData) == 0 {
			return errors.New("savedata type is ToxSave but no data provided")
		}

		// Validate savedata length matches
		if options.SavedataLength > 0 && len(options.SavedataData) != int(options.SavedataLength) {
			return errors.New("savedata length mismatch")
		}

		// Use the existing Load method to restore state
		return t.Load(options.SavedataData)
	default:
		return errors.New("unknown savedata type")
	}
}
