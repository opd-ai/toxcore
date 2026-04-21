// Package toxcore implements the core functionality of the Tox protocol.
// This file contains friend management methods extracted from the main toxcore.go
// to improve maintainability.
package toxcore

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/friend"
	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
)

// AddFriend adds a new friend by their Tox ID and sends a friend request.
// The address must be a valid Tox ID (76 hex characters).
// Returns the new friend's ID on success.
//
//export ToxAddFriend
func (t *Tox) AddFriend(address, message string) (uint32, error) {
	// Parse the Tox ID
	toxID, err := crypto.ToxIDFromString(address)
	if err != nil {
		return 0, err
	}

	// Check if already a friend
	friendID, exists := t.getFriendIDByPublicKey(toxID.PublicKey)
	if exists {
		return friendID, errors.New("already a friend")
	}

	// Create a new friend
	friendID = t.generateFriendID()
	f := &Friend{
		PublicKey:        toxID.PublicKey,
		Status:           FriendStatusNone,
		ConnectionStatus: ConnectionNone,
		LastSeen:         t.now(),
	}

	// Add to friends list
	t.friends.Set(friendID, f)

	// Send friend request
	err = t.sendFriendRequest(toxID.PublicKey, message)
	if err != nil {
		// Remove the friend we just added since sending failed
		t.friends.Delete(friendID)
		return 0, fmt.Errorf("failed to send friend request: %w", err)
	}

	// Register with the async manager so that offline messages from this
	// friend can be decrypted when retrieved from storage nodes.
	if t.asyncManager != nil {
		t.asyncManager.AddFriend(toxID.PublicKey)
	}

	return friendID, nil
}

// AddFriendByPublicKey adds a friend by their public key without sending a friend request.
// This matches the documented API for accepting friend requests: AddFriend(publicKey)
//
//export ToxAddFriendByPublicKey
func (t *Tox) AddFriendByPublicKey(publicKey [32]byte) (uint32, error) {
	// Check if already a friend
	friendID, exists := t.getFriendIDByPublicKey(publicKey)
	if exists {
		return friendID, errors.New("already a friend")
	}

	// Create a new friend
	friendID = t.generateFriendID()
	f := &Friend{
		PublicKey:        publicKey,
		Status:           FriendStatusNone,
		ConnectionStatus: ConnectionNone,
		LastSeen:         t.now(),
	}

	// Add to friends list
	t.friends.Set(friendID, f)

	// Register with the async manager so that offline messages from this
	// friend can be decrypted when retrieved from storage nodes.
	if t.asyncManager != nil {
		t.asyncManager.AddFriend(publicKey)
	}

	return friendID, nil
}

// getFriendIDByPublicKey finds a friend ID by public key.
func (t *Tox) getFriendIDByPublicKey(publicKey [32]byte) (uint32, bool) {
	id, f := t.friends.FindByPublicKey(publicKey, func(f *Friend) [32]byte {
		return f.PublicKey
	})
	return id, f != nil
}

// generateFriendID creates a new unique friend ID.
// Friend IDs start from 1, with 0 reserved as an invalid/not-found sentinel value.
func (t *Tox) generateFriendID() uint32 {
	// Start from 1 to reserve 0 as the invalid/not-found sentinel
	var id uint32 = 1
	for {
		if !t.friends.Exists(id) {
			return id
		}
		id++
	}
}

// GetFriendConnectionStatus returns a friend's connection status.
//
//export ToxGetFriendConnectionStatus
func (t *Tox) GetFriendConnectionStatus(friendID uint32) ConnectionStatus {
	var status ConnectionStatus = ConnectionNone
	t.friends.Read(friendID, func(f *Friend) {
		status = f.ConnectionStatus
	})
	return status
}

// FriendExists checks if a friend exists.
//
//export ToxFriendExists
func (t *Tox) FriendExists(friendID uint32) bool {
	return t.friends.Exists(friendID)
}

// GetFriendByPublicKey gets a friend ID by public key.
//
//export ToxGetFriendByPublicKey
func (t *Tox) GetFriendByPublicKey(publicKey [32]byte) (uint32, error) {
	logger := logrus.WithFields(logrus.Fields{
		"function":        "GetFriendByPublicKey",
		"package":         "toxcore",
		"public_key_hash": fmt.Sprintf("%x", publicKey[:8]),
	})

	logger.Debug("Function entry: looking up friend by public key")

	defer func() {
		logger.Debug("Function exit: GetFriendByPublicKey")
	}()

	id, exists := t.getFriendIDByPublicKey(publicKey)
	if !exists {
		logger.WithFields(logrus.Fields{
			"error":      "friend not found",
			"error_type": "friend_lookup_failed",
			"operation":  "friend_id_lookup",
		}).Debug("Friend lookup failed: public key not found in friends list")
		return 0, errors.New("friend not found")
	}

	logger.WithFields(logrus.Fields{
		"friend_id": id,
		"operation": "friend_lookup_success",
	}).Debug("Friend found successfully by public key")

	return id, nil
}

// GetFriendPublicKey gets a friend's public key.
//
//export ToxGetFriendPublicKey
func (t *Tox) GetFriendPublicKey(friendID uint32) ([32]byte, error) {
	logger := logrus.WithFields(logrus.Fields{
		"function":  "GetFriendPublicKey",
		"package":   "toxcore",
		"friend_id": friendID,
	})

	logger.Debug("Function entry: retrieving friend's public key")

	defer func() {
		logger.Debug("Function exit: GetFriendPublicKey")
	}()

	f := t.friends.Get(friendID)
	if f == nil {
		logger.WithFields(logrus.Fields{
			"error":      "friend not found",
			"error_type": "invalid_friend_id",
			"operation":  "friend_id_validation",
		}).Debug("Friend public key lookup failed: invalid friend ID")
		return [32]byte{}, errors.New("friend not found")
	}

	logger.WithFields(logrus.Fields{
		"public_key_hash": fmt.Sprintf("%x", f.PublicKey[:8]),
		"operation":       "public_key_retrieval_success",
	}).Debug("Friend's public key retrieved successfully")

	return f.PublicKey, nil
}

// GetFriends returns a copy of the friends map.
// This method allows access to the friends list for operations like counting friends.
//
//export ToxGetFriends
func (t *Tox) GetFriends() map[uint32]*Friend {
	logger := logrus.WithFields(logrus.Fields{
		"function": "GetFriends",
		"package":  "toxcore",
	})

	logger.Debug("Function entry: retrieving friends list")

	defer func() {
		logger.Debug("Function exit: GetFriends")
	}()

	friendsCount := t.friends.Count()
	logger.WithFields(logrus.Fields{
		"friends_count": friendsCount,
		"operation":     "friends_list_copy",
	}).Debug("Creating copy of friends list for safe external access")

	// Return a deep copy of the friends map to prevent external modification
	friendsCopy := make(map[uint32]*Friend)
	t.friends.Range(func(id uint32, f *Friend) bool {
		friendsCopy[id] = &Friend{
			PublicKey:        f.PublicKey,
			Status:           f.Status,
			ConnectionStatus: f.ConnectionStatus,
			Name:             f.Name,
			StatusMessage:    f.StatusMessage,
			LastSeen:         f.LastSeen,
			UserData:         f.UserData,
		}
		return true
	})

	logger.WithFields(logrus.Fields{
		"friends_copied": len(friendsCopy),
		"operation":      "friends_list_retrieval_success",
	}).Debug("Friends list copied successfully")

	return friendsCopy
}

// GetFriendsCount returns the number of friends.
// This is a more semantically clear method for counting friends than len(GetFriends()).
//
//export ToxGetFriendsCount
func (t *Tox) GetFriendsCount() int {
	logger := logrus.WithFields(logrus.Fields{
		"function": "GetFriendsCount",
		"package":  "toxcore",
	})

	logger.Debug("Function entry: counting friends")

	defer func() {
		logger.Debug("Function exit: GetFriendsCount")
	}()

	count := t.friends.Count()

	logger.WithFields(logrus.Fields{
		"friends_count": count,
		"operation":     "friends_count_success",
	}).Debug("Friends count retrieved successfully")

	return count
}

// cleanupFriendFileTransfers cancels any pending file transfers for a friend.
func (t *Tox) cleanupFriendFileTransfers(friendID uint32) {
	if t.fileManager == nil {
		return
	}
	cancelled := t.fileManager.CancelTransfersForFriend(friendID)
	if cancelled > 0 {
		logrus.WithFields(logrus.Fields{
			"function":            "cleanupFriendFileTransfers",
			"friend_id":           friendID,
			"cancelled_transfers": cancelled,
		}).Info("Cancelled pending file transfers during friend deletion")
	}
}

// cleanupFriendAsyncMessages clears pending async messages for a friend.
func (t *Tox) cleanupFriendAsyncMessages(friendID uint32, publicKey [32]byte) {
	if t.asyncManager == nil {
		return
	}
	cleared := t.asyncManager.ClearPendingMessagesForFriend(publicKey)
	if cleared > 0 {
		logrus.WithFields(logrus.Fields{
			"function":         "cleanupFriendAsyncMessages",
			"friend_id":        friendID,
			"cleared_messages": cleared,
		}).Info("Cleared pending async messages during friend deletion")
	}
}

// notifyFriendDeleted invokes the friend deleted callback if set.
func (t *Tox) notifyFriendDeleted(friendID uint32) {
	t.callbackMu.RLock()
	cb := t.friendDeletedCallback
	t.callbackMu.RUnlock()
	if cb != nil {
		cb(friendID)
	}
}

// DeleteFriend removes a friend from the friends list and cleans up associated resources.
//
//export ToxDeleteFriend
func (t *Tox) DeleteFriend(friendID uint32) error {
	friend := t.friends.Get(friendID)
	if friend == nil {
		return errors.New("friend not found")
	}

	t.cleanupFriendFileTransfers(friendID)
	t.cleanupFriendAsyncMessages(friendID, friend.PublicKey)

	if !t.friends.Delete(friendID) {
		return errors.New("friend not found")
	}

	t.notifyFriendDeleted(friendID)

	logrus.WithFields(logrus.Fields{
		"function":  "DeleteFriend",
		"friend_id": friendID,
	}).Info("Friend deleted with resource cleanup completed")

	return nil
}

// FriendByPublicKey finds a friend by their public key.
//
//export ToxFriendByPublicKey
func (t *Tox) FriendByPublicKey(publicKey [32]byte) (uint32, error) {
	id, found := t.getFriendIDByPublicKey(publicKey)
	if !found {
		return 0, errors.New("friend not found")
	}
	return id, nil
}

// AddFriendAddress registers a friend's network address for packet delivery
func (t *Tox) AddFriendAddress(friendID uint32, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function":  "AddFriendAddress",
		"friend_id": friendID,
		"address":   addr.String(),
	}).Info("Adding friend address for packet delivery")

	d := t.loadDelivery()
	if d == nil {
		return fmt.Errorf("packet delivery not initialized")
	}

	return d.AddFriend(friendID, addr)
}

// RemoveFriendAddress removes a friend's network address registration
func (t *Tox) RemoveFriendAddress(friendID uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":  "RemoveFriendAddress",
		"friend_id": friendID,
	}).Info("Removing friend address from packet delivery")

	d := t.loadDelivery()
	if d == nil {
		return fmt.Errorf("packet delivery not initialized")
	}

	return d.RemoveFriend(friendID)
}

// GetFriendEncryptionStatus returns the encryption status for a specific friend
//
//export ToxGetFriendEncryptionStatus
func (t *Tox) GetFriendEncryptionStatus(friendID uint32) EncryptionStatus {
	// Check if friend exists
	f := t.friends.Get(friendID)
	if f == nil {
		return EncryptionUnknown
	}

	// Check if friend is online (has connection status)
	if f.ConnectionStatus == ConnectionNone {
		return EncryptionOffline
	}

	// Check if we have async messaging active (indicates forward-secure capability)
	if t.asyncManager != nil {
		// If async manager is enabled and friend supports it, they have forward secrecy
		return EncryptionForwardSecure
	}

	// Check if Noise-IK is available via transport
	if t.tcpTransport != nil || t.udpTransport != nil {
		return EncryptionNoiseIK
	}

	return EncryptionLegacy
}

// invokeFriendNameCallback safely invokes the friend name callback if set
func (t *Tox) invokeFriendNameCallback(friendID uint32, name string) {
	t.callbackMu.RLock()
	callback := t.friendNameCallback
	t.callbackMu.RUnlock()

	if callback != nil {
		callback(friendID, name)
	}
}

// invokeFriendStatusMessageCallback safely invokes the friend status message callback if set
func (t *Tox) invokeFriendStatusMessageCallback(friendID uint32, statusMessage string) {
	t.callbackMu.RLock()
	callback := t.friendStatusMessageCallback
	t.callbackMu.RUnlock()

	if callback != nil {
		callback(friendID, statusMessage)
	}
}

// invokeFriendTypingCallback safely invokes the friend typing callback if set
func (t *Tox) invokeFriendTypingCallback(friendID uint32, isTyping bool) {
	t.callbackMu.RLock()
	callback := t.friendTypingCallback
	t.callbackMu.RUnlock()

	if callback != nil {
		callback(friendID, isTyping)
	}
}

// updateFriendField is a helper for updating friend fields with validation and callbacks.
func (t *Tox) updateFriendField(
	friendID uint32,
	value string,
	maxLen int,
	updateFn func(*Friend, string),
	callbackFn func(uint32, string),
) {
	if len([]byte(value)) > maxLen {
		return // Ignore oversized values
	}

	// Use atomic Update to ensure thread-safe modification of friend fields
	updated := t.friends.Update(friendID, func(f *Friend) {
		updateFn(f, value)
	})

	if updated {
		callbackFn(friendID, value)
	}
}

// receiveFriendNameUpdate processes incoming friend name update packets
func (t *Tox) receiveFriendNameUpdate(friendID uint32, name string) {
	t.updateFriendField(
		friendID,
		name,
		128, // Max name length for Tox protocol
		func(f *Friend, v string) { f.Name = v },
		t.invokeFriendNameCallback,
	)
}

// receiveFriendTyping processes incoming typing notification packets
func (t *Tox) receiveFriendTyping(friendID uint32, isTyping bool) {
	// Use atomic Update to ensure thread-safe modification
	updated := t.friends.Update(friendID, func(f *Friend) {
		f.IsTyping = isTyping
	})

	if updated {
		// Dispatch to typing notification callback
		t.invokeFriendTypingCallback(friendID, isTyping)
	}
}

// receiveFriendRequest processes incoming friend request packets
func (t *Tox) receiveFriendRequest(senderPublicKey [32]byte, message string) {
	// Validate message length (1016 bytes max for Tox friend request message)
	if len([]byte(message)) > 1016 {
		return // Ignore oversized friend request messages
	}

	// Check if this public key is already a friend
	_, exists := t.getFriendIDByPublicKey(senderPublicKey)
	if exists {
		return // Ignore friend requests from existing friends
	}

	// Route through RequestManager if available for centralized request handling
	if t.requestManager != nil {
		// Create a friend.Request to track in RequestManager
		req := &friend.Request{
			SenderPublicKey: senderPublicKey,
			Message:         message,
		}
		t.requestManager.AddRequest(req)
	}

	// Trigger the friend request callback if set
	t.callbackMu.RLock()
	callback := t.friendRequestCallback
	t.callbackMu.RUnlock()
	if callback != nil {
		callback(senderPublicKey, message)
	}
}

// sendFriendRequest sends a friend request packet to the specified public key
func (t *Tox) sendFriendRequest(targetPublicKey [32]byte, message string) error {
	if len([]byte(message)) > 1016 {
		return errors.New("friend request message too long")
	}

	packetData := t.buildFriendRequestPacket(targetPublicKey, message)
	packet := &transport.Packet{
		PacketType: transport.PacketFriendRequest,
		Data:       packetData,
	}

	sentViaNetwork := t.attemptNetworkSend(targetPublicKey, message, packet)

	if !sentViaNetwork {
		t.handleFailedNetworkSend(targetPublicKey, message, packet, packetData)
	}

	return nil
}

// buildFriendRequestPacket constructs the friend request packet data.
func (t *Tox) buildFriendRequestPacket(targetPublicKey [32]byte, message string) []byte {
	packetData := make([]byte, 32+len(message))
	copy(packetData[0:32], t.keyPair.Public[:])
	copy(packetData[32:], message)
	return packetData
}

// attemptNetworkSend tries to send the friend request via DHT network.
func (t *Tox) attemptNetworkSend(targetPublicKey [32]byte, message string, packet *transport.Packet) bool {
	targetToxID := crypto.NewToxID(targetPublicKey, [4]byte{})
	closestNodes := t.dht.FindClosestNodes(*targetToxID, 1)

	if len(closestNodes) == 0 || t.udpTransport == nil || closestNodes[0].Address == nil {
		return false
	}

	logrus.WithFields(logrus.Fields{
		"function":       "sendFriendRequest",
		"target_pk":      fmt.Sprintf("%x", targetPublicKey[:8]),
		"closest_node":   closestNodes[0].Address.String(),
		"message_length": len(message),
	}).Info("Sending friend request via DHT network")

	if err := t.udpTransport.Send(packet, closestNodes[0].Address); err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "sendFriendRequest",
			"error":     err.Error(),
			"node_addr": closestNodes[0].Address.String(),
		}).Warn("Failed to send friend request via DHT, will queue for retry")
		return false
	}

	return true
}

// handleFailedNetworkSend handles friend request when network send fails.
func (t *Tox) handleFailedNetworkSend(targetPublicKey [32]byte, message string, _ *transport.Packet, packetData []byte) {
	t.queuePendingFriendRequest(targetPublicKey, message, packetData)
}

// queuePendingFriendRequest queues a friend request for retry in production scenarios
func (t *Tox) queuePendingFriendRequest(targetPublicKey [32]byte, message string, packetData []byte) {
	t.pendingFriendReqsMux.Lock()
	defer t.pendingFriendReqsMux.Unlock()

	// Check if we already have a pending request for this public key
	for i, req := range t.pendingFriendReqs {
		if req.targetPublicKey == targetPublicKey {
			// Update existing request
			t.pendingFriendReqs[i].message = message
			t.pendingFriendReqs[i].packetData = packetData
			t.pendingFriendReqs[i].timestamp = t.now()
			logrus.WithFields(logrus.Fields{
				"function":  "queuePendingFriendRequest",
				"target_pk": fmt.Sprintf("%x", targetPublicKey[:8]),
			}).Debug("Updated existing pending friend request")
			return
		}
	}

	// Add new pending request
	now := t.now()
	req := &pendingFriendRequest{
		targetPublicKey: targetPublicKey,
		message:         message,
		packetData:      packetData,
		timestamp:       now,
		retryCount:      0,
		nextRetry:       now.Add(5 * time.Second), // Initial retry after 5 seconds
	}
	t.pendingFriendReqs = append(t.pendingFriendReqs, req)

	logrus.WithFields(logrus.Fields{
		"function":   "queuePendingFriendRequest",
		"target_pk":  fmt.Sprintf("%x", targetPublicKey[:8]),
		"next_retry": req.nextRetry,
	}).Info("Queued friend request for retry")
}

// retryPendingFriendRequests attempts to resend friend requests that failed initial delivery
func (t *Tox) retryPendingFriendRequests() {
	t.pendingFriendReqsMux.Lock()
	defer t.pendingFriendReqsMux.Unlock()

	now := t.now()
	var stillPending []*pendingFriendRequest

	for _, req := range t.pendingFriendReqs {
		if now.Before(req.nextRetry) {
			stillPending = append(stillPending, req)
			continue
		}

		if t.attemptSendRequest(req, now) {
			continue
		}

		if t.shouldKeepRetrying(req, now) {
			t.scheduleNextRetry(req, now)
			stillPending = append(stillPending, req)
		}
	}

	t.pendingFriendReqs = stillPending
}

// attemptSendRequest tries to send a friend request via DHT and returns true if successful.
func (t *Tox) attemptSendRequest(req *pendingFriendRequest, now time.Time) bool {
	targetToxID := crypto.NewToxID(req.targetPublicKey, [4]byte{})
	closestNodes := t.dht.FindClosestNodes(*targetToxID, 1)

	if len(closestNodes) == 0 || t.udpTransport == nil || closestNodes[0].Address == nil {
		return false
	}

	packet := &transport.Packet{
		PacketType: transport.PacketFriendRequest,
		Data:       req.packetData,
	}

	if err := t.udpTransport.Send(packet, closestNodes[0].Address); err != nil {
		logrus.WithFields(logrus.Fields{
			"function":    "retryPendingFriendRequests",
			"target_pk":   fmt.Sprintf("%x", req.targetPublicKey[:8]),
			"retry_count": req.retryCount,
			"error":       err.Error(),
		}).Warn("Failed to retry friend request")
		return false
	}

	logrus.WithFields(logrus.Fields{
		"function":    "retryPendingFriendRequests",
		"target_pk":   fmt.Sprintf("%x", req.targetPublicKey[:8]),
		"retry_count": req.retryCount,
		"node_addr":   closestNodes[0].Address.String(),
	}).Info("Successfully retried friend request via DHT")
	return true
}

// shouldKeepRetrying determines if we should continue retrying a failed request.
func (t *Tox) shouldKeepRetrying(req *pendingFriendRequest, now time.Time) bool {
	req.retryCount++

	if req.retryCount >= 10 {
		logrus.WithFields(logrus.Fields{
			"function":    "retryPendingFriendRequests",
			"target_pk":   fmt.Sprintf("%x", req.targetPublicKey[:8]),
			"retry_count": req.retryCount,
			"age":         now.Sub(req.timestamp),
		}).Warn("Giving up on friend request after maximum retries")
		return false
	}
	return true
}

// scheduleNextRetry calculates and schedules the next retry with exponential backoff.
func (t *Tox) scheduleNextRetry(req *pendingFriendRequest, now time.Time) {
	backoff := time.Duration(5*(1<<uint(req.retryCount))) * time.Second
	req.nextRetry = now.Add(backoff)

	logrus.WithFields(logrus.Fields{
		"function":    "retryPendingFriendRequests",
		"target_pk":   fmt.Sprintf("%x", req.targetPublicKey[:8]),
		"retry_count": req.retryCount,
		"next_retry":  req.nextRetry,
		"backoff":     backoff,
	}).Debug("Scheduled friend request retry with exponential backoff")
}
