package async

import (
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// ============================================================================
// Pre-Key Count and Accuracy Tests
// ============================================================================

// TestGetRemainingKeyCountAccuracy verifies that GetRemainingKeyCount
// accurately reflects the actual number of remaining keys after extraction.
func TestGetRemainingKeyCountAccuracy(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prekey-count-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	peerKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}

	// Generate initial bundle
	_, err = store.GeneratePreKeys(peerKey)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	// Initial count should be PreKeysPerPeer (100)
	initialCount := store.GetRemainingKeyCount(peerKey)
	if initialCount != PreKeysPerPeer {
		t.Fatalf("Expected initial count=%d, got %d", PreKeysPerPeer, initialCount)
	}

	// Extract multiple keys rapidly
	numKeysToExtract := 10
	for i := 0; i < numKeysToExtract; i++ {
		_, err := store.GetAvailablePreKey(peerKey)
		if err != nil {
			t.Fatalf("Failed to get pre-key %d: %v", i, err)
		}
	}

	// Verify count matches actual remaining keys
	countAfterExtraction := store.GetRemainingKeyCount(peerKey)
	expectedCount := PreKeysPerPeer - numKeysToExtract

	if countAfterExtraction != expectedCount {
		t.Fatalf("Expected count=%d after extracting %d keys, got %d",
			expectedCount, numKeysToExtract, countAfterExtraction)
	}

	// Verify bundle.Keys slice length matches the count
	bundle, err := store.GetBundle(peerKey)
	if err != nil {
		t.Fatalf("Failed to get bundle: %v", err)
	}

	actualKeysInBundle := len(bundle.Keys)
	if actualKeysInBundle != countAfterExtraction {
		t.Fatalf("Mismatch: GetRemainingKeyCount=%d but len(bundle.Keys)=%d",
			countAfterExtraction, actualKeysInBundle)
	}

	// Verify UsedCount is tracked correctly
	if bundle.UsedCount != numKeysToExtract {
		t.Fatalf("Expected UsedCount=%d, got %d", numKeysToExtract, bundle.UsedCount)
	}

	// Critical assertion: remaining count should equal slice length
	if countAfterExtraction != actualKeysInBundle {
		t.Fatalf("GetRemainingKeyCount must return len(bundle.Keys)")
	}
}

// TestNeedsRefreshAccuracy verifies that NeedsRefresh correctly triggers
// based on actual remaining keys.
func TestNeedsRefreshAccuracy(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prekey-refresh-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	peerKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}

	// Generate initial bundle
	_, err = store.GeneratePreKeys(peerKey)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	// Should not need refresh initially
	if store.NeedsRefresh(peerKey) {
		t.Fatal("Should not need refresh with full bundle")
	}

	// Extract keys until we're just above the threshold
	keysToExtract := PreKeysPerPeer - (PreKeyRefreshThreshold + 1)
	for i := 0; i < keysToExtract; i++ {
		_, err := store.GetAvailablePreKey(peerKey)
		if err != nil {
			t.Fatalf("Failed to get pre-key %d: %v", i, err)
		}
	}

	// At exactly threshold + 1 (21 keys), should not need refresh yet
	if store.NeedsRefresh(peerKey) {
		remaining := store.GetRemainingKeyCount(peerKey)
		t.Fatalf("Should not need refresh with %d keys (threshold=%d)",
			remaining, PreKeyRefreshThreshold)
	}

	// Extract one more key to reach exactly the threshold (20 keys)
	_, err = store.GetAvailablePreKey(peerKey)
	if err != nil {
		t.Fatalf("Failed to extract key to threshold: %v", err)
	}

	// At exactly the threshold, should need refresh (condition is <=)
	if !store.NeedsRefresh(peerKey) {
		remaining := store.GetRemainingKeyCount(peerKey)
		t.Fatalf("Should need refresh with %d keys remaining (threshold=%d)",
			remaining, PreKeyRefreshThreshold)
	}
}

// ============================================================================
// Pre-Key Removal Tests
// ============================================================================

// TestPreKeyRemovalAfterUse verifies that pre-keys are properly removed after use.
func TestPreKeyRemovalAfterUse(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prekey-removal-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	peerKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}

	// Generate pre-keys
	_, err = store.GeneratePreKeys(peerKey)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	initialCount := store.GetRemainingKeyCount(peerKey)
	if initialCount <= 0 {
		t.Fatalf("Expected positive key count, got %d", initialCount)
	}

	preKey, err := store.GetAvailablePreKey(peerKey)
	if err != nil {
		t.Fatalf("Failed to get available pre-key: %v", err)
	}

	if preKey == nil || preKey.KeyPair == nil {
		t.Fatalf("Returned pre-key is nil or missing key pair")
	}

	if !preKey.Used {
		t.Fatalf("Pre-key was not marked as used")
	}

	countAfterUse := store.GetRemainingKeyCount(peerKey)
	if countAfterUse != initialCount-1 {
		t.Fatalf("Expected key count to decrease by 1")
	}

	// Create a new store instance to test loading from disk
	store2, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create second pre-key store: %v", err)
	}

	bundle, err := store2.GetBundle(peerKey)
	if err != nil {
		t.Fatalf("Failed to get bundle from new store instance: %v", err)
	}

	if bundle.UsedCount != 1 {
		t.Fatalf("Expected UsedCount=1, got %d", bundle.UsedCount)
	}

	// Verify that used keys are completely removed
	usedKeysFound := 0
	for _, key := range bundle.Keys {
		if key.Used {
			usedKeysFound++
		}
	}

	if usedKeysFound > 0 {
		t.Fatalf("Found %d keys marked as used, expected 0", usedKeysFound)
	}

	if len(bundle.Keys) != PreKeysPerPeer-1 {
		t.Fatalf("Expected %d keys in bundle, got %d", PreKeysPerPeer-1, len(bundle.Keys))
	}
}

// ============================================================================
// Pre-Key Race Condition Tests
// ============================================================================

// TestPreKeyRefreshRaceCondition verifies that RefreshPreKeys is atomic.
func TestPreKeyRefreshRaceCondition(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prekey_race_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}

	peerPK := [32]byte{0x01, 0x02, 0x03, 0x04}

	_, err = store.GeneratePreKeys(peerPK)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Start multiple goroutines that will try to access the bundle
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				remaining := store.GetRemainingKeyCount(peerPK)
				if remaining < 0 || remaining > PreKeysPerPeer {
					errors <- fmt.Errorf("invalid remaining count: %d", remaining)
					return
				}
				time.Sleep(time.Microsecond)
			}
		}()
	}

	// Start goroutines that refresh pre-keys
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				_, err := store.RefreshPreKeys(peerPK)
				if err != nil {
					errors <- err
					return
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}

	// Start goroutines that try to get available pre-keys
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, err := store.GetAvailablePreKey(peerPK)
				if err != nil {
					if err.Error() != "no pre-key bundle found for peer 01020304" {
						errors <- err
						return
					}
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}

	bundle, err := store.GetBundle(peerPK)
	if err != nil {
		t.Fatalf("Failed to get final bundle: %v", err)
	}

	if len(bundle.Keys) == 0 {
		t.Error("Final bundle should have keys")
	}
}

// TestPreKeyRefreshAtomicity verifies bundle consistency during refresh.
func TestPreKeyRefreshAtomicity(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prekey_atomic_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}

	peerPK := [32]byte{0x05, 0x06, 0x07, 0x08}

	initialBundle, err := store.GeneratePreKeys(peerPK)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	// Use most keys to trigger refresh threshold
	for i := 0; i < PreKeysPerPeer-PreKeyRefreshThreshold; i++ {
		_, err = store.GetAvailablePreKey(peerPK)
		if err != nil {
			t.Fatalf("Failed to get pre-key %d: %v", i, err)
		}
	}

	type bundleState struct {
		keyCount  int
		usedCount int
		createdAt time.Time
	}
	statesSeen := make(chan bundleState, 1000)

	var wg sync.WaitGroup

	// Monitor bundle state continuously
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			bundle, err := store.GetBundle(peerPK)
			if err == nil && bundle != nil {
				statesSeen <- bundleState{
					keyCount:  len(bundle.Keys),
					usedCount: bundle.UsedCount,
					createdAt: bundle.CreatedAt,
				}
			}
			time.Sleep(100 * time.Microsecond)
		}
	}()

	time.Sleep(time.Millisecond)
	newBundle, err := store.RefreshPreKeys(peerPK)
	if err != nil {
		t.Fatalf("Failed to refresh pre-keys: %v", err)
	}

	wg.Wait()
	close(statesSeen)

	validStateCount := 0
	for state := range statesSeen {
		isOldBundle := state.createdAt.Equal(initialBundle.CreatedAt)
		isNewBundle := state.createdAt.Equal(newBundle.CreatedAt)

		if !isOldBundle && !isNewBundle {
			t.Errorf("Invalid intermediate state seen: keyCount=%d, usedCount=%d",
				state.keyCount, state.usedCount)
		}

		if isOldBundle && state.keyCount == 0 {
			t.Error("Old bundle seen with no keys (inconsistent state)")
		}
		if isNewBundle && state.keyCount != PreKeysPerPeer {
			t.Errorf("New bundle should have %d keys, saw %d", PreKeysPerPeer, state.keyCount)
		}
		if isNewBundle && state.usedCount != 0 {
			t.Errorf("New bundle should have 0 used keys, saw %d", state.usedCount)
		}

		validStateCount++
	}

	if validStateCount == 0 {
		t.Error("No bundle states were monitored")
	}
}

// TestPreKeyRefreshConcurrentReads verifies reads during refresh.
func TestPreKeyRefreshConcurrentReads(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prekey_concurrent_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	store, err := NewPreKeyStore(keyPair, tempDir)
	if err != nil {
		t.Fatalf("Failed to create pre-key store: %v", err)
	}

	peerPK := [32]byte{0x0A, 0x0B, 0x0C, 0x0D}

	_, err = store.GeneratePreKeys(peerPK)
	if err != nil {
		t.Fatalf("Failed to generate pre-keys: %v", err)
	}

	var wg sync.WaitGroup
	successfulReads := make(chan int, 100)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			reads := 0
			for j := 0; j < 50; j++ {
				bundle, err := store.GetBundle(peerPK)
				if err == nil {
					if bundle == nil {
						t.Errorf("Reader %d: Got nil bundle without error", id)
						return
					}
					if len(bundle.Keys) == 0 {
						t.Errorf("Reader %d: Got bundle with zero keys", id)
						return
					}
					reads++
				}
				time.Sleep(10 * time.Microsecond)
			}
			successfulReads <- reads
		}(i)
	}

	time.Sleep(100 * time.Microsecond)
	for i := 0; i < 10; i++ {
		_, err := store.RefreshPreKeys(peerPK)
		if err != nil {
			t.Errorf("Refresh %d failed: %v", i, err)
		}
		time.Sleep(time.Millisecond)
	}

	wg.Wait()
	close(successfulReads)

	totalReads := 0
	for reads := range successfulReads {
		totalReads += reads
	}

	if totalReads == 0 {
		t.Error("No successful reads occurred during refresh operations")
	}

	t.Logf("Completed %d successful concurrent reads during 10 refresh operations", totalReads)
}

// ============================================================================
// Pre-Key Network Exchange Tests
// ============================================================================

// TestPreKeyExchangeOverNetwork verifies that pre-key exchange packets are sent over the network.
func TestPreKeyExchangeOverNetwork(t *testing.T) {
	aliceKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Alice's key pair: %v", err)
	}

	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Bob's key pair: %v", err)
	}

	aliceTransport := NewMockTransport("127.0.0.1:5000")
	bobTransport := NewMockTransport("127.0.0.1:5001")

	aliceDir := t.TempDir()
	bobDir := t.TempDir()

	aliceManager, err := NewAsyncManager(aliceKeyPair, aliceTransport, aliceDir)
	if err != nil {
		t.Fatalf("Failed to create Alice's async manager: %v", err)
	}

	bobManager, err := NewAsyncManager(bobKeyPair, bobTransport, bobDir)
	if err != nil {
		t.Fatalf("Failed to create Bob's async manager: %v", err)
	}

	aliceAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5000")
	bobAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5001")

	aliceManager.SetFriendAddress(bobKeyPair.Public, bobAddr)
	bobManager.SetFriendAddress(aliceKeyPair.Public, aliceAddr)

	var bobReceivedPreKey bool

	aliceTransport.sendFunc = func(packet *transport.Packet, addr net.Addr) error {
		if packet.PacketType == transport.PacketAsyncPreKeyExchange {
			bobReceivedPreKey = true
			go bobManager.handlePreKeyExchangePacket(packet, aliceAddr)
		}
		return nil
	}

	bobTransport.sendFunc = func(packet *transport.Packet, addr net.Addr) error {
		if packet.PacketType == transport.PacketAsyncPreKeyExchange {
			go aliceManager.handlePreKeyExchangePacket(packet, bobAddr)
		}
		return nil
	}

	aliceManager.SetFriendOnlineStatus(bobKeyPair.Public, true)

	time.Sleep(100 * time.Millisecond)

	if !bobReceivedPreKey {
		t.Error("Alice should have sent pre-key exchange packet to Bob")
	}

	if !bobManager.CanSendAsyncMessage(aliceKeyPair.Public) {
		t.Error("Bob should be able to send async messages to Alice after key exchange")
	}
}

// TestPreKeyPacketFormat verifies the pre-key packet format is correct.
func TestPreKeyPacketFormat(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	dir := t.TempDir()
	mockTransport := NewMockTransport("127.0.0.1:6000")

	manager, err := NewAsyncManager(keyPair, mockTransport, dir)
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}

	peerKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate peer key pair: %v", err)
	}

	peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:6001")
	manager.SetFriendAddress(peerKey.Public, peerAddr)

	var capturedPacket *transport.Packet

	mockTransport.sendFunc = func(packet *transport.Packet, addr net.Addr) error {
		if packet.PacketType == transport.PacketAsyncPreKeyExchange {
			capturedPacket = packet
		}
		return nil
	}

	manager.SetFriendOnlineStatus(peerKey.Public, true)

	time.Sleep(100 * time.Millisecond)

	if capturedPacket == nil {
		t.Fatal("No pre-key exchange packet was captured")
	}

	if capturedPacket.PacketType != transport.PacketAsyncPreKeyExchange {
		t.Errorf("Expected packet type %d, got %d",
			transport.PacketAsyncPreKeyExchange, capturedPacket.PacketType)
	}

	if len(capturedPacket.Data) < 32 {
		t.Error("Pre-key packet data is too short to contain sender's public key")
	}
}

// TestMultipleFriendsPreKeyExchange tests pre-key exchange with multiple friends.
func TestMultipleFriendsPreKeyExchange(t *testing.T) {
	aliceKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate Alice's key pair: %v", err)
	}

	aliceTransport := NewMockTransport("127.0.0.1:7000")
	aliceDir := t.TempDir()

	aliceManager, err := NewAsyncManager(aliceKeyPair, aliceTransport, aliceDir)
	if err != nil {
		t.Fatalf("Failed to create Alice's async manager: %v", err)
	}

	numFriends := 3
	friendKeys := make([]*crypto.KeyPair, numFriends)
	friendAddrs := make([]net.Addr, numFriends)
	exchangeCount := make(map[string]int)

	for i := 0; i < numFriends; i++ {
		friendKeys[i], err = crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate friend %d key pair: %v", i, err)
		}
		friendAddrs[i], _ = net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:700%d", i+1))
		aliceManager.SetFriendAddress(friendKeys[i].Public, friendAddrs[i])
	}

	aliceTransport.sendFunc = func(packet *transport.Packet, addr net.Addr) error {
		if packet.PacketType == transport.PacketAsyncPreKeyExchange {
			exchangeCount[addr.String()]++
		}
		return nil
	}

	for i := 0; i < numFriends; i++ {
		aliceManager.SetFriendOnlineStatus(friendKeys[i].Public, true)
	}

	time.Sleep(200 * time.Millisecond)

	for i := 0; i < numFriends; i++ {
		addrStr := friendAddrs[i].String()
		if exchangeCount[addrStr] == 0 {
			t.Errorf("No pre-key exchange sent to friend %d at %s", i, addrStr)
		}
	}
}
