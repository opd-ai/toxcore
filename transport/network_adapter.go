package transport

import (
	"fmt"
	"net"
	"sync"

	"github.com/sirupsen/logrus"
)

// NetworkTransportAdapter adapts UDPTransport to implement INetworkTransport
type NetworkTransportAdapter struct {
	udpTransport *UDPTransport
	friendAddrs  map[uint32]net.Addr
	mu           sync.RWMutex
}

// NewNetworkTransportAdapter creates a new adapter for UDPTransport
func NewNetworkTransportAdapter(udpTransport *UDPTransport) *NetworkTransportAdapter {
	logrus.WithFields(logrus.Fields{
		"function":   "NewNetworkTransportAdapter",
		"local_addr": udpTransport.LocalAddr().String(),
	}).Info("Creating network transport adapter")

	return &NetworkTransportAdapter{
		udpTransport: udpTransport,
		friendAddrs:  make(map[uint32]net.Addr),
	}
}

// Send implements INetworkTransport.Send
func (n *NetworkTransportAdapter) Send(packet []byte, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function":    "NetworkTransportAdapter.Send",
		"destination": addr.String(),
		"packet_size": len(packet),
	}).Debug("Sending packet via network transport adapter")

	// Create a transport packet
	transportPacket := &Packet{
		PacketType: PacketFriendMessage, // Use friend message type for friend communications
		Data:       packet,
	}

	return n.udpTransport.Send(transportPacket, addr)
}

// SendToFriend implements INetworkTransport.SendToFriend
func (n *NetworkTransportAdapter) SendToFriend(friendID uint32, packet []byte) error {
	logrus.WithFields(logrus.Fields{
		"function":    "NetworkTransportAdapter.SendToFriend",
		"friend_id":   friendID,
		"packet_size": len(packet),
	}).Debug("Sending packet to friend via network transport adapter")

	n.mu.RLock()
	addr, exists := n.friendAddrs[friendID]
	n.mu.RUnlock()

	if !exists {
		err := fmt.Errorf("friend %d address not found", friendID)
		logrus.WithFields(logrus.Fields{
			"function":  "NetworkTransportAdapter.SendToFriend",
			"friend_id": friendID,
			"error":     err.Error(),
		}).Error("Friend address not found")
		return err
	}

	return n.Send(packet, addr)
}

// GetFriendAddress implements INetworkTransport.GetFriendAddress
func (n *NetworkTransportAdapter) GetFriendAddress(friendID uint32) (net.Addr, error) {
	logrus.WithFields(logrus.Fields{
		"function":  "NetworkTransportAdapter.GetFriendAddress",
		"friend_id": friendID,
	}).Debug("Getting friend address from network transport adapter")

	n.mu.RLock()
	defer n.mu.RUnlock()

	addr, exists := n.friendAddrs[friendID]
	if !exists {
		err := fmt.Errorf("friend %d address not found", friendID)
		logrus.WithFields(logrus.Fields{
			"function":  "NetworkTransportAdapter.GetFriendAddress",
			"friend_id": friendID,
			"error":     err.Error(),
		}).Error("Friend address not found")
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"function":  "NetworkTransportAdapter.GetFriendAddress",
		"friend_id": friendID,
		"address":   addr.String(),
	}).Debug("Friend address found")

	return addr, nil
}

// RegisterFriend implements INetworkTransport.RegisterFriend
func (n *NetworkTransportAdapter) RegisterFriend(friendID uint32, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function":  "NetworkTransportAdapter.RegisterFriend",
		"friend_id": friendID,
		"address":   addr.String(),
	}).Info("Registering friend address in network transport adapter")

	n.mu.Lock()
	defer n.mu.Unlock()

	n.friendAddrs[friendID] = addr

	logrus.WithFields(logrus.Fields{
		"function":      "NetworkTransportAdapter.RegisterFriend",
		"friend_id":     friendID,
		"total_friends": len(n.friendAddrs),
	}).Info("Friend address registered successfully")

	return nil
}

// Close implements INetworkTransport.Close
func (n *NetworkTransportAdapter) Close() error {
	logrus.WithFields(logrus.Fields{
		"function": "NetworkTransportAdapter.Close",
	}).Info("Closing network transport adapter")

	if n.udpTransport != nil {
		return n.udpTransport.Close()
	}

	logrus.WithFields(logrus.Fields{
		"function": "NetworkTransportAdapter.Close",
	}).Info("Network transport adapter closed successfully")

	return nil
}

// IsConnected implements INetworkTransport.IsConnected
func (n *NetworkTransportAdapter) IsConnected() bool {
	// For UDP transport, we consider it connected if it's not nil and has a valid local address
	if n.udpTransport == nil {
		return false
	}

	localAddr := n.udpTransport.LocalAddr()
	connected := localAddr != nil

	logrus.WithFields(logrus.Fields{
		"function":  "NetworkTransportAdapter.IsConnected",
		"connected": connected,
		"local_addr": func() string {
			if localAddr != nil {
				return localAddr.String()
			}
			return "nil"
		}(),
	}).Debug("Checked network transport adapter connection status")

	return connected
}

// GetLocalAddr returns the local address of the underlying transport
func (n *NetworkTransportAdapter) GetLocalAddr() net.Addr {
	if n.udpTransport != nil {
		return n.udpTransport.LocalAddr()
	}
	return nil
}

// GetFriendCount returns the number of registered friends
func (n *NetworkTransportAdapter) GetFriendCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.friendAddrs)
}

// RemoveFriend removes a friend's address registration
func (n *NetworkTransportAdapter) RemoveFriend(friendID uint32) error {
	logrus.WithFields(logrus.Fields{
		"function":  "NetworkTransportAdapter.RemoveFriend",
		"friend_id": friendID,
	}).Info("Removing friend from network transport adapter")

	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.friendAddrs, friendID)

	logrus.WithFields(logrus.Fields{
		"function":          "NetworkTransportAdapter.RemoveFriend",
		"friend_id":         friendID,
		"remaining_friends": len(n.friendAddrs),
	}).Info("Friend removed from network transport adapter")

	return nil
}
