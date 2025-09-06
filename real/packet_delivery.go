package real

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/interfaces"
	"github.com/sirupsen/logrus"
)

// RealPacketDelivery implements actual network-based packet delivery
type RealPacketDelivery struct {
	transport   interfaces.INetworkTransport
	friendAddrs map[uint32]net.Addr
	config      *interfaces.PacketDeliveryConfig
	mu          sync.RWMutex
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
	}
}

// DeliverPacket implements IPacketDelivery.DeliverPacket
func (r *RealPacketDelivery) DeliverPacket(friendID uint32, packet []byte) error {
	logrus.WithFields(logrus.Fields{
		"function":    "RealPacketDelivery.DeliverPacket",
		"friend_id":   friendID,
		"packet_size": len(packet),
	}).Info("Delivering packet via real network transport")

	r.mu.RLock()
	addr, exists := r.friendAddrs[friendID]
	r.mu.RUnlock()

	if !exists {
		// Try to get address from transport
		var err error
		addr, err = r.transport.GetFriendAddress(friendID)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"function":  "RealPacketDelivery.DeliverPacket",
				"friend_id": friendID,
				"error":     err.Error(),
			}).Error("Failed to get friend address")
			return fmt.Errorf("failed to get friend address for %d: %w", friendID, err)
		}

		// Cache the address
		r.mu.Lock()
		r.friendAddrs[friendID] = addr
		r.mu.Unlock()
	}

	// Attempt delivery with retries
	var lastErr error
	for attempt := 0; attempt < r.config.RetryAttempts; attempt++ {
		err := r.transport.SendToFriend(friendID, packet)
		if err == nil {
			logrus.WithFields(logrus.Fields{
				"function":    "RealPacketDelivery.DeliverPacket",
				"friend_id":   friendID,
				"packet_size": len(packet),
				"attempt":     attempt + 1,
			}).Info("Packet delivered successfully via real network")
			return nil
		}

		lastErr = err
		logrus.WithFields(logrus.Fields{
			"function":  "RealPacketDelivery.DeliverPacket",
			"friend_id": friendID,
			"attempt":   attempt + 1,
			"error":     err.Error(),
		}).Warn("Packet delivery attempt failed, retrying")

		if attempt < r.config.RetryAttempts-1 {
			time.Sleep(time.Duration(500*(attempt+1)) * time.Millisecond)
		}
	}

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

	logrus.WithFields(logrus.Fields{
		"function":      "RealPacketDelivery.BroadcastPacket",
		"packet_size":   len(packet),
		"exclude_count": len(excludeFriends),
	}).Info("Broadcasting packet via real network transport")

	// Create exclude map for O(1) lookup
	excludeMap := make(map[uint32]bool)
	for _, friendID := range excludeFriends {
		excludeMap[friendID] = true
	}

	var failedDeliveries []uint32
	var successCount int

	r.mu.RLock()
	friendList := make([]uint32, 0, len(r.friendAddrs))
	for friendID := range r.friendAddrs {
		if !excludeMap[friendID] {
			friendList = append(friendList, friendID)
		}
	}
	r.mu.RUnlock()

	// Deliver to each friend
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

	logrus.WithFields(logrus.Fields{
		"function":      "RealPacketDelivery.BroadcastPacket",
		"success_count": successCount,
		"failed_count":  len(failedDeliveries),
		"total_friends": len(friendList),
	}).Info("Broadcast packet delivery completed")

	if len(failedDeliveries) > 0 {
		return fmt.Errorf("broadcast failed for %d friends: %v", len(failedDeliveries), failedDeliveries)
	}

	return nil
}

// SetNetworkTransport implements IPacketDelivery.SetNetworkTransport
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
			}).Warn("Failed to close old transport")
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

// AddFriend registers a friend's network address for packet delivery
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

// RemoveFriend removes a friend's address registration
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

// GetStats returns statistics about packet delivery
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
