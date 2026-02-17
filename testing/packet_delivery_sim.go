package testing

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/interfaces"
	"github.com/sirupsen/logrus"
)

// SimulatedPacketDelivery implements simulation-based packet delivery for testing
type SimulatedPacketDelivery struct {
	deliveryLog []DeliveryRecord
	friendMap   map[uint32]bool
	config      *interfaces.PacketDeliveryConfig
	mu          sync.RWMutex
}

// DeliveryRecord represents a packet delivery event for testing verification.
// Each record captures metadata about a delivery attempt, enabling test code
// to verify correct delivery behavior including timing, success/failure, and
// packet sizes.
type DeliveryRecord struct {
	// FriendID is the target friend identifier for this delivery.
	FriendID uint32
	// PacketSize is the size of the delivered packet in bytes.
	PacketSize int
	// Timestamp is the Unix nanoseconds when the delivery occurred.
	Timestamp int64
	// Success indicates whether the delivery completed successfully.
	Success bool
	// Error holds any error that occurred during delivery (nil on success).
	Error error
}

// NewSimulatedPacketDelivery creates a new simulation implementation for testing
func NewSimulatedPacketDelivery(config *interfaces.PacketDeliveryConfig) *SimulatedPacketDelivery {
	logrus.Warn("SIMULATION FUNCTION - NOT A REAL OPERATION")
	logrus.WithFields(logrus.Fields{
		"function": "NewSimulatedPacketDelivery",
		"timeout":  config.NetworkTimeout,
		"retries":  config.RetryAttempts,
	}).Info("Creating simulated packet delivery for testing")

	return &SimulatedPacketDelivery{
		deliveryLog: make([]DeliveryRecord, 0),
		friendMap:   make(map[uint32]bool),
		config:      config,
	}
}

// DeliverPacket implements IPacketDelivery.DeliverPacket with simulation
func (s *SimulatedPacketDelivery) DeliverPacket(friendID uint32, packet []byte) error {
	logrus.Warn("SIMULATION FUNCTION - NOT A REAL OPERATION")
	logrus.WithFields(logrus.Fields{
		"function":    "SimulatedPacketDelivery.DeliverPacket",
		"friend_id":   friendID,
		"packet_size": len(packet),
	}).Info("Simulating packet delivery")

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if friend exists
	if !s.friendMap[friendID] {
		err := fmt.Errorf("friend %d not found in simulation", friendID)
		s.deliveryLog = append(s.deliveryLog, DeliveryRecord{
			FriendID:   friendID,
			PacketSize: len(packet),
			Timestamp:  time.Now().UnixNano(),
			Success:    false,
			Error:      err,
		})

		logrus.WithFields(logrus.Fields{
			"function":  "SimulatedPacketDelivery.DeliverPacket",
			"friend_id": friendID,
			"error":     err.Error(),
		}).Error("Friend not found in simulation")

		return err
	}

	// Simulate successful delivery
	s.deliveryLog = append(s.deliveryLog, DeliveryRecord{
		FriendID:   friendID,
		PacketSize: len(packet),
		Timestamp:  time.Now().UnixNano(),
		Success:    true,
		Error:      nil,
	})

	logrus.WithFields(logrus.Fields{
		"function":         "SimulatedPacketDelivery.DeliverPacket",
		"friend_id":        friendID,
		"packet_size":      len(packet),
		"total_deliveries": len(s.deliveryLog),
	}).Info("Packet delivery simulated successfully")

	return nil
}

// BroadcastPacket implements IPacketDelivery.BroadcastPacket with simulation
func (s *SimulatedPacketDelivery) BroadcastPacket(packet []byte, excludeFriends []uint32) error {
	if !s.config.EnableBroadcast {
		return fmt.Errorf("broadcast is disabled in configuration")
	}

	logrus.Warn("SIMULATION FUNCTION - NOT A REAL OPERATION")
	logrus.WithFields(logrus.Fields{
		"function":      "SimulatedPacketDelivery.BroadcastPacket",
		"packet_size":   len(packet),
		"exclude_count": len(excludeFriends),
	}).Info("Simulating packet broadcast")

	// Create exclude map
	excludeMap := make(map[uint32]bool)
	for _, friendID := range excludeFriends {
		excludeMap[friendID] = true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var successCount int
	var failedCount int

	// Simulate delivery to each friend
	for friendID := range s.friendMap {
		if excludeMap[friendID] {
			failedCount++ // Track excluded friends as "not delivered"
			continue
		}

		// Simulate delivery
		s.deliveryLog = append(s.deliveryLog, DeliveryRecord{
			FriendID:   friendID,
			PacketSize: len(packet),
			Timestamp:  time.Now().UnixNano(),
			Success:    true,
			Error:      nil,
		})
		successCount++
	}

	logrus.WithFields(logrus.Fields{
		"function":         "SimulatedPacketDelivery.BroadcastPacket",
		"success_count":    successCount,
		"failed_count":     failedCount,
		"total_friends":    len(s.friendMap),
		"total_deliveries": len(s.deliveryLog),
	}).Info("Broadcast packet simulation completed")

	return nil
}

// SetNetworkTransport implements IPacketDelivery.SetNetworkTransport (no-op for simulation)
func (s *SimulatedPacketDelivery) SetNetworkTransport(transport interfaces.INetworkTransport) error {
	logrus.Warn("SIMULATION FUNCTION - NOT A REAL OPERATION")
	logrus.WithFields(logrus.Fields{
		"function": "SimulatedPacketDelivery.SetNetworkTransport",
	}).Info("Simulating network transport change (no-op)")

	// For simulation, we don't actually use the transport
	return nil
}

// IsSimulation implements IPacketDelivery.IsSimulation
func (s *SimulatedPacketDelivery) IsSimulation() bool {
	return true
}

// AddFriend registers a friend in the simulation, enabling packet delivery to them.
// This implements IPacketDelivery.AddFriend. The addr parameter is ignored in
// simulation mode since no actual network operations are performed.
// This must be called before attempting to deliver packets to a friend.
// Safe for concurrent use.
func (s *SimulatedPacketDelivery) AddFriend(friendID uint32, addr net.Addr) error {
	logrus.Warn("SIMULATION FUNCTION - NOT A REAL OPERATION")
	logrus.WithFields(logrus.Fields{
		"function":  "SimulatedPacketDelivery.AddFriend",
		"friend_id": friendID,
		"address":   addrString(addr),
	}).Info("Adding friend to simulation")

	s.mu.Lock()
	defer s.mu.Unlock()

	s.friendMap[friendID] = true

	logrus.WithFields(logrus.Fields{
		"function":      "SimulatedPacketDelivery.AddFriend",
		"friend_id":     friendID,
		"total_friends": len(s.friendMap),
	}).Info("Friend added to simulation successfully")

	return nil
}

// addrString safely converts net.Addr to string, handling nil addresses.
func addrString(addr net.Addr) string {
	if addr == nil {
		return "<nil>"
	}
	return addr.String()
}

// RemoveFriend removes a friend from the simulation.
// This implements IPacketDelivery.RemoveFriend.
// After removal, DeliverPacket calls to this friend will fail.
// Safe for concurrent use.
func (s *SimulatedPacketDelivery) RemoveFriend(friendID uint32) error {
	logrus.Warn("SIMULATION FUNCTION - NOT A REAL OPERATION")
	logrus.WithFields(logrus.Fields{
		"function":  "SimulatedPacketDelivery.RemoveFriend",
		"friend_id": friendID,
	}).Info("Removing friend from simulation")

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.friendMap, friendID)

	logrus.WithFields(logrus.Fields{
		"function":          "SimulatedPacketDelivery.RemoveFriend",
		"friend_id":         friendID,
		"remaining_friends": len(s.friendMap),
	}).Info("Friend removed from simulation successfully")

	return nil
}

// GetDeliveryLog returns a copy of the complete delivery log for test verification.
// The returned slice is a copy; modifications do not affect the internal log.
// Safe for concurrent use.
func (s *SimulatedPacketDelivery) GetDeliveryLog() []DeliveryRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modifications
	log := make([]DeliveryRecord, len(s.deliveryLog))
	copy(log, s.deliveryLog)
	return log
}

// ClearDeliveryLog removes all entries from the delivery log.
// Call this between test cases to reset the simulation state.
// Safe for concurrent use.
func (s *SimulatedPacketDelivery) ClearDeliveryLog() {
	logrus.Warn("SIMULATION FUNCTION - NOT A REAL OPERATION")
	logrus.WithFields(logrus.Fields{
		"function": "SimulatedPacketDelivery.ClearDeliveryLog",
	}).Info("Clearing simulation delivery log")

	s.mu.Lock()
	defer s.mu.Unlock()

	s.deliveryLog = make([]DeliveryRecord, 0)

	logrus.WithFields(logrus.Fields{
		"function": "SimulatedPacketDelivery.ClearDeliveryLog",
	}).Info("Simulation delivery log cleared")
}

// GetStats returns statistics about the simulation state.
// The returned map contains:
//   - total_friends: number of registered friends
//   - total_deliveries: total delivery attempts in the log
//   - successful_deliveries: count of successful deliveries
//   - failed_deliveries: count of failed deliveries
//   - is_simulation: always true for this implementation
//   - broadcast_enabled: whether broadcast is enabled in config
//   - retry_attempts: configured retry attempts
//   - network_timeout: configured timeout in milliseconds
//
// Safe for concurrent use.
func (s *SimulatedPacketDelivery) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	successCount := 0
	failedCount := 0
	for _, record := range s.deliveryLog {
		if record.Success {
			successCount++
		} else {
			failedCount++
		}
	}

	return map[string]interface{}{
		"total_friends":         len(s.friendMap),
		"total_deliveries":      len(s.deliveryLog),
		"successful_deliveries": successCount,
		"failed_deliveries":     failedCount,
		"is_simulation":         true,
		"broadcast_enabled":     s.config.EnableBroadcast,
		"retry_attempts":        s.config.RetryAttempts,
		"network_timeout":       s.config.NetworkTimeout,
	}
}
