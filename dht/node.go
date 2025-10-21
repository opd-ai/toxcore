// Package dht implements the Distributed Hash Table for the Tox protocol.
//
// The DHT is based on a modified Kademlia algorithm and is responsible for
// peer discovery and routing in the Tox network.
//
// Example:
//
//	dht := dht.New(options)
//	err := dht.Bootstrap("node.tox.biribiri.org", 33445, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
//	if err != nil {
//	    log.Fatal(err)
//	}
package dht

import (
	"net"
	"strconv"
	"time"

	"github.com/opd-ai/toxforge/crypto"
)

// NodeStatus represents the connection status of a node.
type NodeStatus uint8

const (
	StatusUnknown NodeStatus = iota
	StatusBad
	StatusGood
)

// PingStats tracks ping statistics for a node.
type PingStats struct {
	LastPingSent     time.Time
	LastPingReceived time.Time
	PingCount        uint32
	SuccessCount     uint32
	FailureCount     uint32
}

// Node represents a peer in the Tox DHT network.
//
//export ToxDHTNode
type Node struct {
	ID        crypto.ToxID
	Address   net.Addr
	LastSeen  time.Time
	Status    NodeStatus
	PublicKey [32]byte
	PingStats PingStats // Add ping statistics
}

// NewNode creates a node object with the given Tox ID and network address.
//
//export ToxDHTNodeNew
func NewNode(id crypto.ToxID, addr net.Addr) *Node {
	node := &Node{
		ID:       id,
		Address:  addr,
		LastSeen: time.Now(),
		Status:   StatusUnknown,
	}
	copy(node.PublicKey[:], id.PublicKey[:])
	return node
}

// Distance calculates the XOR distance between this node and another node.
//
//export ToxDHTNodeDistance
func (n *Node) Distance(other *Node) [32]byte {
	var result [32]byte
	for i := 0; i < 32; i++ {
		result[i] = n.PublicKey[i] ^ other.PublicKey[i]
	}
	return result
}

// IsActive checks if the node has been seen within the timeout period.
//
//export ToxDHTNodeIsActive
func (n *Node) IsActive(timeout time.Duration) bool {
	return time.Since(n.LastSeen) < timeout
}

// Update marks the node as recently seen and updates its status.
//
//export ToxDHTNodeUpdate
func (n *Node) Update(status NodeStatus) {
	n.LastSeen = time.Now()
	n.Status = status
}

// IPPort returns the address and port of the node.
// For IP:Port addresses, returns the IP and port.
// For other address types (like .onion, .b32.i2p), returns the full address and port 0.
//
//export ToxDHTNodeIPPort
func (n *Node) IPPort() (string, uint16) {
	// Try to parse as host:port first
	host, portstr, err := net.SplitHostPort(n.Address.String())
	if err != nil {
		// If we can't split host:port, return the full address string
		// This handles .onion, .b32.i2p, and other non-IP address types
		return n.Address.String(), 0
	}

	// Parse the port if we successfully split
	port, err := strconv.ParseUint(portstr, 10, 16)
	if err != nil {
		// If port parsing fails, return host with port 0
		return host, 0
	}

	return host, uint16(port)
}

// RecordPingSent marks that a ping was sent to this node.
//
//export ToxDHTNodeRecordPingSent
func (n *Node) RecordPingSent() {
	n.PingStats.LastPingSent = time.Now()
	n.PingStats.PingCount++
}

// RecordPingResponse marks that a ping response was received from this node.
//
//export ToxDHTNodeRecordPingResponse
func (n *Node) RecordPingResponse(success bool) {
	if success {
		n.PingStats.LastPingReceived = time.Now()
		n.PingStats.SuccessCount++
		n.Update(StatusGood)
	} else {
		n.PingStats.FailureCount++
		if n.PingStats.FailureCount > n.PingStats.SuccessCount {
			n.Update(StatusBad)
		}
	}
}

// GetReliability returns a reliability score for this node (0.0-1.0).
//
//export ToxDHTNodeGetReliability
func (n *Node) GetReliability() float64 {
	if n.PingStats.PingCount == 0 {
		return 0.0
	}
	return float64(n.PingStats.SuccessCount) / float64(n.PingStats.PingCount)
}
