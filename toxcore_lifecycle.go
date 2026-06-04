// toxcore_lifecycle.go contains lifecycle-related functionality including
// iteration, shutdown, state persistence, and time management.

package toxcore

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
	"github.com/opd-ai/toxcore/file"
	"github.com/sirupsen/logrus"
)

// Iterate performs a single iteration of the Tox event loop.
//
//export ToxIterate
func (t *Tox) Iterate() {
	// Update connection status based on bootstrap state
	t.updateConnectionStatus()

	// Process DHT maintenance
	t.doDHTMaintenance()

	// Process friend connections
	t.doFriendConnections()

	// Process message queue
	t.doMessageProcessing()

	// Retry pending friend requests (production retry queue)
	t.retryPendingFriendRequests()

	// Increment iteration count after processing
	atomic.AddUint64(&t.iterationCount, 1)
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
	return atomic.LoadInt32(&t.running) == 1
}

// SetTimeProvider sets a custom time provider for deterministic testing.
// This should only be used in tests. In production, the default RealTimeProvider is used.
func (t *Tox) SetTimeProvider(tp TimeProvider) {
	if tp == nil {
		return
	}
	t.timeProviderMu.Lock()
	t.timeProvider = tp
	t.timeProviderMu.Unlock()
}

// now returns the current time using the configured time provider.
func (t *Tox) now() time.Time {
	t.timeProviderMu.RLock()
	tp := t.timeProvider
	t.timeProviderMu.RUnlock()
	return tp.Now()
}

// Kill stops the Tox instance and releases all resources.
//
//export ToxKill
func (t *Tox) Kill() {
	atomic.StoreInt32(&t.running, 0)
	t.cancel()

	t.closeTransports()
	t.stopBackgroundServices()
	t.cleanupManagers()
	t.clearCallbacks()
}

// Context returns the lifecycle context of this Tox instance.
// The context is cancelled when Kill is called, allowing any component
// whose goroutines should stop together with the Tox instance to derive
// a child context from the one returned here.
func (t *Tox) Context() context.Context {
	return t.ctx
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
	t.bootstrapManagerMu.Lock()
	t.bootstrapManager = nil
	t.bootstrapManagerMu.Unlock()
}

// snapshotBootstrapManager returns the current bootstrap manager under a read lock.
// Returns nil if Kill() has already cleared it.
func (t *Tox) snapshotBootstrapManager() *dht.BootstrapManager {
	t.bootstrapManagerMu.RLock()
	bm := t.bootstrapManager
	t.bootstrapManagerMu.RUnlock()
	return bm
}

// cleanupManagers cleans up all manager instances and the friends list.
func (t *Tox) cleanupManagers() {
	t.cancelActiveFileTransfers()

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

	// Clear the friends store
	if t.friends != nil {
		t.friends.Clear()
	}
}

// cancelActiveFileTransfers cancels all tracked transfers and clears the transfer map.
func (t *Tox) cancelActiveFileTransfers() {
	t.transfersMu.Lock()
	if len(t.fileTransfers) == 0 {
		t.transfersMu.Unlock()
		return
	}

	transfers := make([]*file.Transfer, 0, len(t.fileTransfers))
	for _, transfer := range t.fileTransfers {
		transfers = append(transfers, transfer)
	}
	clear(t.fileTransfers)
	t.transfersMu.Unlock()

	for _, transfer := range transfers {
		if err := transfer.Cancel(); err != nil && !errors.Is(err, file.ErrTransferAlreadyFinished) {
			logrus.WithError(err).Warn("Failed to cancel active file transfer during shutdown")
		}
	}
}

// clearCallbacks clears all callback functions to prevent memory leaks.
// Must hold callbackMu.Lock() around all field assignments to prevent a data
// race against dispatchers that read callbacks under callbackMu.RLock() (H-06).
// Clears every registered callback field (L-02: previously omitted file/name/status/typing).
func (t *Tox) clearCallbacks() {
	t.callbackMu.Lock()
	defer t.callbackMu.Unlock()

	t.friendRequestCallback = nil
	t.friendMessageCallback = nil
	t.simpleFriendMessageCallback = nil
	t.friendStatusCallback = nil
	t.connectionStatusCallback = nil
	t.friendConnectionStatusCallback = nil
	t.friendStatusChangeCallback = nil
	t.fileRecvCallback = nil
	t.fileRecvChunkCallback = nil
	t.fileChunkRequestCallback = nil
	t.friendNameCallback = nil
	t.friendStatusMessageCallback = nil
	t.friendTypingCallback = nil
	t.friendDeletedCallback = nil
}

// doDHTMaintenance performs periodic DHT maintenance tasks.
// Runs every ~6 seconds (120 iterations × 50 ms tick) to avoid flooding the network.
func (t *Tox) doDHTMaintenance() {
	// Protect t.dht access against concurrent Kill() → clearDHT().
	t.dhtMutex.RLock()
	dht := t.dht
	t.dhtMutex.RUnlock()

	if dht == nil {
		return
	}

	bm := t.snapshotBootstrapManager()
	if bm == nil {
		return
	}

	// Rate-limit: run once every 120 iterations (~6 s at 50 ms/tick).
	if atomic.LoadUint64(&t.iterationCount)%120 != 0 {
		return
	}

	// Snapshot self identity data under lock
	t.selfMutex.RLock()
	if t.keyPair == nil {
		t.selfMutex.RUnlock()
		return
	}
	publicKey := t.keyPair.Public
	nospam := t.nospam
	t.selfMutex.RUnlock()

	selfToxID := crypto.NewToxID(publicKey, nospam)
	allNodes := dht.FindClosestNodes(*selfToxID, 100)

	if len(allNodes) < 10 {
		// Routing table is sparse — re-bootstrap to replenish it.
		ctx, cancel := context.WithTimeout(t.ctx, t.options.BootstrapTimeout)

		if err := bm.Bootstrap(ctx); err != nil {
			logrus.WithFields(logrus.Fields{
				"function":   "doDHTMaintenance",
				"node_count": len(allNodes),
				"error":      err.Error(),
			}).Debug("DHT re-bootstrap attempt failed")
		}
		cancel()
	} else {
		// Routing table has nodes — send FIND_NODE queries toward our own key to
		// keep buckets fresh. We reuse Bootstrap to ping the known bootstrap nodes;
		// a full FIND_NODE walk is handled by the DHT Maintainer when present.
		if bm.IsBootstrapped() {
			return
		}
		ctx, cancel := context.WithTimeout(t.ctx, t.options.BootstrapTimeout)
		_ = bm.Bootstrap(ctx) //nolint:errcheck // best-effort refresh
		cancel()
	}
}

// doFriendConnections manages friend connections.
// Rate-limited to every 240 iterations (~12 s at 50 ms/tick).
func (t *Tox) doFriendConnections() {
	// Snapshot t.dht under dhtMutex once to prevent a race with Kill → clearDHT
	// (H-07: doFriendConnections reads t.dht without dhtMutex).
	t.dhtMutex.RLock()
	dhtSnapshot := t.dht
	t.dhtMutex.RUnlock()

	if !t.shouldRunFriendConnections(dhtSnapshot) {
		return
	}

	offlineKeys := t.collectOfflineFriendKeys()
	if len(offlineKeys) == 0 {
		return
	}

	t.scheduleFriendRequestRetries(dhtSnapshot, offlineKeys)
}

// shouldRunFriendConnections checks if friend connection processing should run.
// dhtSnapshot must be the already-snapshotted pointer (not re-read from t.dht).
func (t *Tox) shouldRunFriendConnections(dhtSnapshot *dht.RoutingTable) bool {
	if t.friends.Count() == 0 || dhtSnapshot == nil {
		return false
	}
	// Only run every 240 iterations to avoid DHT flooding.
	return atomic.LoadUint64(&t.iterationCount)%240 == 0
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
func (t *Tox) scheduleFriendRequestRetries(dhtSnapshot *dht.RoutingTable, offlineKeys [][32]byte) {
	now := t.now()
	t.pendingFriendReqsMux.Lock()
	defer t.pendingFriendReqsMux.Unlock()

	for _, pk := range offlineKeys {
		t.maybeScheduleRetryForFriend(dhtSnapshot, pk, now)
	}
}

// maybeScheduleRetryForFriend checks if a DHT route exists for the given friend
// and schedules immediate retry if so.
// dhtSnapshot is the already-snapshotted DHT pointer; if nil the call is a no-op.
func (t *Tox) maybeScheduleRetryForFriend(dhtSnapshot *dht.RoutingTable, pk [32]byte, now time.Time) {
	if dhtSnapshot == nil {
		return
	}
	friendToxID := crypto.NewToxID(pk, [4]byte{})
	nodes := dhtSnapshot.FindClosestNodes(*friendToxID, 1)
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

	saveData := t.buildSaveDataSnapshot()
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
	t.selfMutex.Lock()
	t.keyPair = saveData.KeyPair
	t.selfMutex.Unlock()
	return nil
}

// restoreFriendsList reconstructs the friends list from saved data.
func (t *Tox) restoreFriendsList(saveData *toxSaveData) {
	if saveData.Friends == nil {
		return
	}
	// Clear and re-populate the friends store.
	t.friends.Clear()
	if t.asyncManager != nil {
		t.asyncManager.ResetKnownSenders()
	}
	for id, f := range saveData.Friends {
		t.restoreFriendEntry(id, f)
	}
}

// restoreFriendEntry restores one friend and re-registers async decryption state.
func (t *Tox) restoreFriendEntry(id uint32, friend *Friend) {
	if friend == nil {
		return
	}
	t.friends.Set(id, cloneFriendEntry(friend))
	if t.asyncManager != nil {
		t.asyncManager.AddFriend(friend.PublicKey)
	}
}

// restoreOptions selectively restores safe configuration options.
func (t *Tox) restoreOptions(saveData *toxSaveData) {
	if saveData.Options != nil && t.options != nil {
		// Only restore certain safe options, not all options should be restored
		// as some are runtime-specific (like network settings).
		// Hold selfMutex to prevent concurrent reads of t.options fields.
		t.selfMutex.Lock()
		t.options.SavedataType = saveData.Options.SavedataType
		t.options.SavedataData = saveData.Options.SavedataData
		t.options.SavedataLength = saveData.Options.SavedataLength
		t.selfMutex.Unlock()
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
