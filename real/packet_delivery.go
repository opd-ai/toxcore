package real

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/interfaces"
	"github.com/sirupsen/logrus"
)

// Sleeper provides an abstraction over time.Sleep for deterministic testing.
type Sleeper interface {
	// Sleep pauses execution for the specified duration.
	Sleep(d time.Duration)
}

// DefaultSleeper implements Sleeper using the standard library time.Sleep.
type DefaultSleeper struct{}

// Sleep pauses execution for the specified duration using time.Sleep.
func (DefaultSleeper) Sleep(d time.Duration) {
	time.Sleep(d)
}

// RealPacketDelivery implements actual network-based packet delivery
type RealPacketDelivery struct {
	transport   interfaces.INetworkTransport
	friendAddrs map[uint32]net.Addr
	config      *interfaces.PacketDeliveryConfig
	mu          sync.RWMutex
	sleeper     Sleeper
}

// NewRealPacketDelivery creates a new real packet delivery implementation
func NewRealPacketDelivery(transport interfaces.INetworkTransport, config *interfaces.PacketDeliveryConfig) *RealPacketDelivery {
	logrus.WithFields(logrus.Fields{
		"function": "NewRealPacketDelivery",
		"timeout":  config.NetworkTimeout,
		"retries":  config.RetryAttempts,
	}).Info("Creating real packet delivery implementation")

	return &RealPacketDelivery{
		transport:   transport,
		friendAddrs: make(map[uint32]net.Addr),
		config:      config,
		sleeper:     DefaultSleeper{},
	}
}

// SetSleeper sets a custom Sleeper implementation (primarily for testing).
func (r *RealPacketDelivery) SetSleeper(s Sleeper) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sleeper = s
}

// DeliverPacket implements IPacketDelivery.DeliverPacket
func (r *RealPacketDelivery) DeliverPacket(friendID uint32, packet []byte) error {
	logrus.WithFields(logrus.Fields{
		"function":    "RealPacketDelivery.DeliverPacket",
		"friend_id":   friendID,
		"packet_size": len(packet),
	}).Info("Delivering packet via real network transport")

	addr, err := r.resolveFriendAddress(friendID)
	if err != nil {
		return err
	}

	// Use addr to maintain cache, though SendToFriend handles actual transmission
	_ = addr

	return r.attemptDeliveryWithRetries(friendID, packet)
}

// resolveFriendAddress retrieves or caches the address for a friend.
func (r *RealPacketDelivery) resolveFriendAddress(friendID uint32) (net.Addr, error) {
	r.mu.RLock()
	addr, exists := r.friendAddrs[friendID]
	r.mu.RUnlock()

	if exists {
		return addr, nil
	}

	return r.fetchAndCacheAddress(friendID)
}

// fetchAndCacheAddress retrieves the friend address from transport and caches it.
func (r *RealPacketDelivery) fetchAndCacheAddress(friendID uint32) (net.Addr, error) {
	addr, err := r.transport.GetFriendAddress(friendID)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"function":  "RealPacketDelivery.DeliverPacket",
			"friend_id": friendID,
			"error":     err.Error(),
		}).Error("Failed to get friend address")
		return nil, fmt.Errorf("failed to get friend address for %d: %w", friendID, err)
	}

	r.mu.Lock()
	r.friendAddrs[friendID] = addr
	r.mu.Unlock()

	return addr, nil
}

// attemptDeliveryWithRetries tries to deliver a packet with exponential backoff.
func (r *RealPacketDelivery) attemptDeliveryWithRetries(friendID uint32, packet []byte) error {
	var lastErr error
	for attempt := 0; attempt < r.config.RetryAttempts; attempt++ {
		if err := r.transport.SendToFriend(friendID, packet); err == nil {
			logDeliverySuccess(friendID, len(packet), attempt+1)
			return nil
		} else {
			lastErr = err
			logDeliveryRetry(friendID, attempt+1, err)
			r.waitBeforeRetry(attempt)
		}
	}

	return r.handleDeliveryFailure(friendID, lastErr)
}

// logDeliverySuccess logs successful packet delivery.
func logDeliverySuccess(friendID uint32, packetSize, attempt int) {
	logrus.WithFields(logrus.Fields{
		"function":    "RealPacketDelivery.DeliverPacket",
		"friend_id":   friendID,
		"packet_size": packetSize,
		"attempt":     attempt,
	}).Info("Packet delivered successfully via real network")
}

// logDeliveryRetry logs a failed delivery attempt.
func logDeliveryRetry(friendID uint32, attempt int, err error) {
	logrus.WithFields(logrus.Fields{
		"function":  "RealPacketDelivery.DeliverPacket",
		"friend_id": friendID,
		"attempt":   attempt,
		"error":     err.Error(),
	}).Warn("Packet delivery attempt failed, retrying")
}

// waitBeforeRetry implements exponential backoff between retries.
func (r *RealPacketDelivery) waitBeforeRetry(attempt int) {
	if attempt < r.config.RetryAttempts-1 {
		r.sleeper.Sleep(time.Duration(500*(attempt+1)) * time.Millisecond)
	}
}

// handleDeliveryFailure logs and returns an error after all retry attempts fail.
func (r *RealPacketDelivery) handleDeliveryFailure(friendID uint32, lastErr error) error {
	logrus.WithFields(logrus.Fields{
		"function":  "RealPacketDelivery.DeliverPacket",
		"friend_id": friendID,
		"attempts":  r.config.RetryAttempts,
		"error":     lastErr.Error(),
	}).Error("All delivery attempts failed")

	return fmt.Errorf("failed to deliver packet after %d attempts: %w", r.config.RetryAttempts, lastErr)
}

// BroadcastPacket implements IPacketDelivery.BroadcastPacket
func (r *RealPacketDelivery) BroadcastPacket(packet []byte, excludeFriends []uint32) error {
	if !r.config.EnableBroadcast {
		return fmt.Errorf("broadcast is disabled in configuration")
	}

	r.logBroadcastStart(len(packet), len(excludeFriends))

	excludeMap := r.buildExcludeMap(excludeFriends)
	friendList := r.collectBroadcastTargets(excludeMap)

	successCount, failedDeliveries := r.deliverToFriends(packet, friendList)

	r.logBroadcastCompletion(successCount, len(failedDeliveries), len(friendList))

	if len(failedDeliveries) > 0 {
		return fmt.Errorf("broadcast failed for %d friends: %v", len(failedDeliveries), failedDeliveries)
	}

	return nil
}

// logBroadcastStart logs the start of a broadcast operation.
func (r *RealPacketDelivery) logBroadcastStart(packetSize, excludeCount int) {
	logrus.WithFields(logrus.Fields{
		"function":      "RealPacketDelivery.BroadcastPacket",
		"packet_size":   packetSize,
		"exclude_count": excludeCount,
	}).Info("Broadcasting packet via real network transport")
}

// buildExcludeMap creates a map for O(1) exclude lookups.
func (r *RealPacketDelivery) buildExcludeMap(excludeFriends []uint32) map[uint32]bool {
	excludeMap := make(map[uint32]bool)
	for _, friendID := range excludeFriends {
		excludeMap[friendID] = true
	}
	return excludeMap
}

// collectBroadcastTargets gathers friend IDs that should receive the broadcast.
func (r *RealPacketDelivery) collectBroadcastTargets(excludeMap map[uint32]bool) []uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	friendList := make([]uint32, 0, len(r.friendAddrs))
	for friendID := range r.friendAddrs {
		if !excludeMap[friendID] {
			friendList = append(friendList, friendID)
		}
	}
	return friendList
}

// deliverToFriends attempts delivery to all friends in the list.
func (r *RealPacketDelivery) deliverToFriends(packet []byte, friendList []uint32) (int, []uint32) {
	var failedDeliveries []uint32
	var successCount int

	for _, friendID := range friendList {
		err := r.DeliverPacket(friendID, packet)
		if err != nil {
			failedDeliveries = append(failedDeliveries, friendID)
			logrus.WithFields(logrus.Fields{
				"function":  "RealPacketDelivery.BroadcastPacket",
				"friend_id": friendID,
				"error":     err.Error(),
			}).Warn("Failed to deliver broadcast packet to friend")
		} else {
			successCount++
		}
	}

	return successCount, failedDeliveries
}

// logBroadcastCompletion logs the completion of a broadcast operation.
func (r *RealPacketDelivery) logBroadcastCompletion(successCount, failedCount, totalFriends int) {
	logrus.WithFields(logrus.Fields{
		"function":      "RealPacketDelivery.BroadcastPacket",
		"success_count": successCount,
		"failed_count":  failedCount,
		"total_friends": totalFriends,
	}).Info("Broadcast packet delivery completed")
}

// SetNetworkTransport implements IPacketDelivery.SetNetworkTransport.
//
// If an existing transport is set, it will be closed before the new transport
// is assigned. If closing the old transport fails, the error is propagated
// to the caller and the new transport is NOT assigned, preserving the old
// state. This ensures callers are aware of cleanup failures that may indicate
// resource leaks or incomplete shutdown.
func (r *RealPacketDelivery) SetNetworkTransport(transport interfaces.INetworkTransport) error {
	logrus.WithFields(logrus.Fields{
		"function": "RealPacketDelivery.SetNetworkTransport",
	}).Info("Setting new network transport")

	r.mu.Lock()
	defer r.mu.Unlock()

	// Close old transport if it exists
	if r.transport != nil {
		if err := r.transport.Close(); err != nil {
			logrus.WithFields(logrus.Fields{
				"function": "RealPacketDelivery.SetNetworkTransport",
				"error":    err.Error(),
			}).Error("Failed to close old transport")
			return fmt.Errorf("failed to close existing transport: %w", err)
		}
	}

	r.transport = transport
	// Clear cached addresses since we have a new transport
	r.friendAddrs = make(map[uint32]net.Addr)

	logrus.WithFields(logrus.Fields{
		"function": "RealPacketDelivery.SetNetworkTransport",
	}).Info("Network transport updated successfully")

	return nil
}

// IsSimulation implements IPacketDelivery.IsSimulation
func (r *RealPacketDelivery) IsSimulation() bool {
	return false
}

// AddFriend registers a friend's network address for packet delivery.
//
// The address is cached locally and also registered with the underlying transport
// if available. Returns an error if transport registration fails, which may occur
// when the transport is unavailable or rejects the registration (e.g., duplicate
// friend ID, invalid address format, or transport-specific constraints).
//
// Thread-safe: uses mutex to protect address cache modification.
func (r *RealPacketDelivery) AddFriend(friendID uint32, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function":  "RealPacketDelivery.AddFriend",
		"friend_id": friendID,
		"address":   addr.String(),
	}).Info("Registering friend address for real packet delivery")

	r.mu.Lock()
	defer r.mu.Unlock()

	r.friendAddrs[friendID] = addr

	// Also register with the transport
	if r.transport != nil {
		err := r.transport.RegisterFriend(friendID, addr)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":  "RealPacketDelivery.AddFriend",
				"friend_id": friendID,
				"error":     err.Error(),
			}).Error("Failed to register friend with transport")
			return fmt.Errorf("failed to register friend with transport: %w", err)
		}
	}

	logrus.WithFields(logrus.Fields{
		"function":      "RealPacketDelivery.AddFriend",
		"friend_id":     friendID,
		"total_friends": len(r.friendAddrs),
	}).Info("Friend registered successfully for real packet delivery")

	return nil
}

// RemoveFriend removes a friend's address registration.
//
// This removes the friend from the local address cache. If the friend ID
// does not exist, this operation is a no-op and returns nil (no error).
// The underlying transport is not notified of the removal.
//
// Thread-safe: uses mutex to protect address cache modification.
func (r *RealPacketDelivery) RemoveFriend(friendID uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":  "RealPacketDelivery.RemoveFriend",
		"friend_id": friendID,
	}).Info("Removing friend address from real packet delivery")

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.friendAddrs, friendID)

	logrus.WithFields(logrus.Fields{
		"function":          "RealPacketDelivery.RemoveFriend",
		"friend_id":         friendID,
		"remaining_friends": len(r.friendAddrs),
	}).Info("Friend removed successfully from real packet delivery")

	return nil
}

// GetStats returns statistics about packet delivery performance and configuration.
//
// The returned map contains:
//   - "total_friends": int - number of registered friend addresses
//   - "is_simulation": bool - always false for RealPacketDelivery
//   - "transport_connected": bool - whether the underlying transport reports connected
//   - "broadcast_enabled": bool - whether broadcast is enabled in configuration
//   - "retry_attempts": int - configured number of delivery retry attempts
//   - "network_timeout": time.Duration - configured network timeout
//
// Deprecated: Use GetTypedStats() for type-safe access to statistics.
// This method will be removed in v2.0.0. Migration timeline:
//   - v1.x: GetStats() available but deprecated
//   - v2.0.0: GetStats() removed, use GetTypedStats() exclusively
//
// Thread-safe: uses read lock for concurrent access.
func (r *RealPacketDelivery) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"total_friends":       len(r.friendAddrs),
		"is_simulation":       false,
		"transport_connected": r.transport != nil && r.transport.IsConnected(),
		"broadcast_enabled":   r.config.EnableBroadcast,
		"retry_attempts":      r.config.RetryAttempts,
		"network_timeout":     r.config.NetworkTimeout,
	}
}

// GetTypedStats returns type-safe statistics about packet delivery.
//
// This method provides structured access to delivery statistics without
// the type assertion requirements of GetStats().
//
// Thread-safe: uses read lock for concurrent access.
func (r *RealPacketDelivery) GetTypedStats() interfaces.PacketDeliveryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return interfaces.PacketDeliveryStats{
		IsSimulation: false,
		FriendCount:  len(r.friendAddrs),
	}
}
