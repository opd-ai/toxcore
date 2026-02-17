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

	"github.com/opd-ai/toxcore/crypto"
)

// TimeProvider abstracts time operations for deterministic testing.
type TimeProvider interface {
	Now() time.Time
	Since(t time.Time) time.Duration
}

// DefaultTimeProvider uses the standard library time functions.
type DefaultTimeProvider struct{}

// Now returns the current time.
func (DefaultTimeProvider) Now() time.Time { return time.Now() }

// Since returns the duration since the given time.
func (DefaultTimeProvider) Since(t time.Time) time.Duration { return time.Since(t) }

// defaultTimeProvider is the package-level default for standalone functions.
var defaultTimeProvider TimeProvider = DefaultTimeProvider{}

// SetDefaultTimeProvider sets the package-level time provider for testing.
// Pass nil to reset to the default implementation.
func SetDefaultTimeProvider(tp TimeProvider) {
	if tp == nil {
		tp = DefaultTimeProvider{}
	}
	defaultTimeProvider = tp
}

// getDefaultTimeProvider returns the package-level time provider.
func getDefaultTimeProvider() TimeProvider {
	return defaultTimeProvider
}

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
	return NewNodeWithTimeProvider(id, addr, nil)
}

// NewNodeWithTimeProvider creates a node object with a custom time provider.
func NewNodeWithTimeProvider(id crypto.ToxID, addr net.Addr, tp TimeProvider) *Node {
	if tp == nil {
		tp = getDefaultTimeProvider()
	}
	node := &Node{
		ID:       id,
		Address:  addr,
		LastSeen: tp.Now(),
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
	n.UpdateWithTimeProvider(status, nil)
}

// UpdateWithTimeProvider marks the node as recently seen with a custom time provider.
func (n *Node) UpdateWithTimeProvider(status NodeStatus, tp TimeProvider) {
	if tp == nil {
		tp = getDefaultTimeProvider()
	}
	n.LastSeen = tp.Now()
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
	n.RecordPingSentWithTimeProvider(nil)
}

// RecordPingSentWithTimeProvider marks that a ping was sent with a custom time provider.
func (n *Node) RecordPingSentWithTimeProvider(tp TimeProvider) {
	if tp == nil {
		tp = getDefaultTimeProvider()
	}
	n.PingStats.LastPingSent = tp.Now()
	n.PingStats.PingCount++
}

// RecordPingResponse marks that a ping response was received from this node.
//
//export ToxDHTNodeRecordPingResponse
func (n *Node) RecordPingResponse(success bool) {
	n.RecordPingResponseWithTimeProvider(success, nil)
}

// RecordPingResponseWithTimeProvider marks a ping response with a custom time provider.
func (n *Node) RecordPingResponseWithTimeProvider(success bool, tp TimeProvider) {
	if tp == nil {
		tp = getDefaultTimeProvider()
	}
	if success {
		n.PingStats.LastPingReceived = tp.Now()
		n.PingStats.SuccessCount++
		n.UpdateWithTimeProvider(StatusGood, tp)
	} else {
		n.PingStats.FailureCount++
		if n.PingStats.FailureCount > n.PingStats.SuccessCount {
			n.UpdateWithTimeProvider(StatusBad, tp)
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
