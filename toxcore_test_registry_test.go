package toxcore

// toxcore_test_registry_test.go — test-only cross-instance friend-request delivery.
//
// These helpers allow same-process integration tests to simulate network delivery
// without requiring actual UDP/TCP sockets. They must never be called from
// production (non-test) code paths.

import (
	"sync"
	"testing"

	"github.com/opd-ai/toxcore/transport"
)

// globalFriendRequestRegistry is a thread-safe in-process store used exclusively
// by integration tests to deliver friend-request packets between Tox instances.
var globalFriendRequestRegistry = struct { //nolint:gochecknoglobals
	sync.RWMutex
	requests map[[32]byte][]byte
}{
	requests: make(map[[32]byte][]byte),
}

// registerGlobalFriendRequest stores a friend request in the test registry.
func registerGlobalFriendRequest(targetPublicKey [32]byte, packetData []byte) {
	globalFriendRequestRegistry.Lock()
	defer globalFriendRequestRegistry.Unlock()
	globalFriendRequestRegistry.requests[targetPublicKey] = packetData
}

// checkGlobalFriendRequest retrieves and removes a friend request from the test registry.
func checkGlobalFriendRequest(publicKey [32]byte) []byte {
	globalFriendRequestRegistry.Lock()
	defer globalFriendRequestRegistry.Unlock()

	packetData, exists := globalFriendRequestRegistry.requests[publicKey]
	if exists {
		delete(globalFriendRequestRegistry.requests, publicKey)
		return packetData
	}
	return nil
}

// processPendingFriendRequests delivers any queued test-registry packet to this
// Tox instance. Call this from test loops instead of relying on Iterate().
func (t *Tox) processPendingFriendRequests() {
	myPublicKey := t.keyPair.Public
	if packetData := checkGlobalFriendRequest(myPublicKey); packetData != nil {
		packet := &transport.Packet{
			PacketType: transport.PacketFriendRequest,
			Data:       packetData,
		}
		_ = t.handleFriendRequestPacket(packet, nil) //nolint:errcheck
	}
}

// simulateFriendRequestDelivery extracts the most-recently queued friend request
// from sender and inserts it into the test registry addressed to receiver.
// This simulates what would happen over the real network without requiring sockets.
func simulateFriendRequestDelivery(tb testing.TB, sender, receiver *Tox) {
	tb.Helper()

	sender.pendingFriendReqsMux.Lock()
	if len(sender.pendingFriendReqs) == 0 {
		sender.pendingFriendReqsMux.Unlock()
		return
	}
	packetData := sender.pendingFriendReqs[len(sender.pendingFriendReqs)-1].packetData
	sender.pendingFriendReqsMux.Unlock()

	var receiverPK [32]byte
	copy(receiverPK[:], receiver.keyPair.Public[:])
	registerGlobalFriendRequest(receiverPK, packetData)
}
