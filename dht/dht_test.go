package dht

import (
	"bytes"
	"errors"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

// MockAddr implements net.Addr interface for testing
type MockAddr struct {
	network string
	address string
}

func (m MockAddr) Network() string { return m.network }
func (m MockAddr) String() string  { return m.address }

func newMockAddr(address string) *MockAddr {
	return &MockAddr{network: "mock", address: address}
}

// MockTransport implements transport.Transport interface for testing
type MockTransport struct {
	sendFunc      func(packet *transport.Packet, addr net.Addr) error
	localAddr     net.Addr
	handlers      map[transport.PacketType]transport.PacketHandler
	sentPackets   []*transport.Packet
	sentAddresses []net.Addr
	mu            sync.Mutex
}

func newMockTransport(localAddr net.Addr) *MockTransport {
	return &MockTransport{
		localAddr:     localAddr,
		handlers:      make(map[transport.PacketType]transport.PacketHandler),
		sentPackets:   make([]*transport.Packet, 0),
		sentAddresses: make([]net.Addr, 0),
		sendFunc:      func(packet *transport.Packet, addr net.Addr) error { return nil },
	}
}

func (m *MockTransport) Send(packet *transport.Packet, addr net.Addr) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentPackets = append(m.sentPackets, packet)
	m.sentAddresses = append(m.sentAddresses, addr)
	return m.sendFunc(packet, addr)
}

func (m *MockTransport) Close() error {
	return nil
}

func (m *MockTransport) LocalAddr() net.Addr {
	return m.localAddr
}

func (m *MockTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
	m.handlers[packetType] = handler
}

func (m *MockTransport) GetSentPackets() ([]*transport.Packet, []net.Addr) {
	m.mu.Lock()
	defer m.mu.Unlock()
	packets := make([]*transport.Packet, len(m.sentPackets))
	addrs := make([]net.Addr, len(m.sentAddresses))
	copy(packets, m.sentPackets)
	copy(addrs, m.sentAddresses)
	return packets, addrs
}

func (m *MockTransport) ResetSentPackets() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentPackets = make([]*transport.Packet, 0)
	m.sentAddresses = make([]net.Addr, 0)
}

func (m *MockTransport) SimulateReceive(packet *transport.Packet, from net.Addr) error {
	handler, ok := m.handlers[packet.PacketType]
	if !ok {
		return errors.New("no handler registered for packet type")
	}
	return handler(packet, from)
}

// Helper function to create test ToxIDs
func createTestToxID(keyData byte) crypto.ToxID {
	var pubKey [32]byte
	var nospam [4]byte

	for i := 0; i < 32; i++ {
		pubKey[i] = keyData
	}

	return *crypto.NewToxID(pubKey, nospam)
}

// TestNode tests the Node struct and its methods
func TestNode(t *testing.T) {
	t.Run("NewNode", func(t *testing.T) {
		// Arrange
		id := createTestToxID(1)
		addr := newMockAddr("192.168.1.1:33445")

		// Act
		node := NewNode(id, addr)

		// Assert
		if node == nil {
			t.Fatal("Expected node not to be nil")
		}
		if node.ID != id {
			t.Errorf("Expected ID %v, got %v", id, node.ID)
		}
		if node.Address != addr {
			t.Errorf("Expected Address %v, got %v", addr, node.Address)
		}
		if node.Status != StatusUnknown {
			t.Errorf("Expected initial status to be StatusUnknown")
		}
		if time.Since(node.LastSeen) > time.Second {
			t.Errorf("Expected LastSeen to be recent")
		}
		if !bytes.Equal(node.PublicKey[:], id.PublicKey[:]) {
			t.Errorf("Expected PublicKey to match ID's PublicKey")
		}
	})

	t.Run("Distance", func(t *testing.T) {
		// Arrange
		id1 := createTestToxID(0xFF)
		id2 := createTestToxID(0x0F)
		addr := newMockAddr("192.168.1.1:33445")
		node1 := NewNode(id1, addr)
		node2 := NewNode(id2, addr)

		// Act
		dist := node1.Distance(node2)

		// Assert
		expected := [32]byte{}
		for i := 0; i < 32; i++ {
			expected[i] = 0xFF ^ 0x0F
		}

		if dist != expected {
			t.Errorf("Expected distance %v, got %v", expected, dist)
		}
	})

	t.Run("IsActive", func(t *testing.T) {
		// Arrange
		id := createTestToxID(1)
		addr := newMockAddr("192.168.1.1:33445")
		node := NewNode(id, addr)

		// Test cases
		testCases := []struct {
			name     string
			timeout  time.Duration
			sleep    time.Duration
			expected bool
		}{
			{"Active with short timeout", 1 * time.Second, 0, true},
			{"Inactive with passed timeout", 10 * time.Millisecond, 20 * time.Millisecond, false},
			{"Active with long timeout", 1 * time.Hour, 0, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.sleep > 0 {
					time.Sleep(tc.sleep)
				}

				// Act
				result := node.IsActive(tc.timeout)

				// Assert
				if result != tc.expected {
					t.Errorf("Expected IsActive to return %v, got %v", tc.expected, result)
				}
			})
		}
	})

	t.Run("Update", func(t *testing.T) {
		// Arrange
		id := createTestToxID(1)
		addr := newMockAddr("192.168.1.1:33445")
		node := NewNode(id, addr)
		oldTime := node.LastSeen
		time.Sleep(5 * time.Millisecond) // Ensure time difference

		// Act
		node.Update(StatusGood)

		// Assert
		if node.Status != StatusGood {
			t.Errorf("Expected Status to be updated to StatusGood")
		}
		if !node.LastSeen.After(oldTime) {
			t.Errorf("Expected LastSeen to be updated to a later time")
		}
	})

	t.Run("IPPort", func(t *testing.T) {
		// Test cases for different address types
		testCases := []struct {
			name         string
			addr         net.Addr
			expectedIP   string
			expectedPort uint16
		}{
			{
				name:         "UDPAddr",
				addr:         &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445},
				expectedIP:   "192.168.1.1",
				expectedPort: 33445,
			},
			{
				name:         "TCPAddr",
				addr:         &net.TCPAddr{IP: net.ParseIP("192.168.1.2"), Port: 33446},
				expectedIP:   "192.168.1.2",
				expectedPort: 33446,
			},
			{
				name:         "Unknown addr type",
				addr:         newMockAddr("unknown"),
				expectedIP:   "",
				expectedPort: 0,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Arrange
				id := createTestToxID(1)
				node := NewNode(id, tc.addr)

				// Act
				ip, port := node.IPPort()

				// Assert
				if ip != tc.expectedIP {
					t.Errorf("Expected IP %s, got %s", tc.expectedIP, ip)
				}
				if port != tc.expectedPort {
					t.Errorf("Expected Port %d, got %d", tc.expectedPort, port)
				}
			})
		}
	})
}

// TestKBucket tests the KBucket implementation
func TestKBucket(t *testing.T) {
	t.Run("NewKBucket", func(t *testing.T) {
		// Act
		kb := NewKBucket(8)

		// Assert
		if kb == nil {
			t.Fatal("Expected KBucket not to be nil")
		}
		if kb.maxSize != 8 {
			t.Errorf("Expected maxSize 8, got %d", kb.maxSize)
		}
		if len(kb.nodes) != 0 {
			t.Errorf("Expected empty nodes slice, got %d elements", len(kb.nodes))
		}
	})

	t.Run("AddNode", func(t *testing.T) {
		// Test cases
		testCases := []struct {
			name     string
			maxSize  int
			setup    func(*KBucket)
			node     *Node
			expected bool
		}{
			{
				name:     "Add to empty bucket",
				maxSize:  2,
				setup:    func(*KBucket) {},
				node:     NewNode(createTestToxID(1), newMockAddr("1.1.1.1:1")),
				expected: true,
			},
			{
				name:    "Update existing node",
				maxSize: 2,
				setup: func(kb *KBucket) {
					kb.AddNode(NewNode(createTestToxID(1), newMockAddr("1.1.1.1:1")))
				},
				node:     NewNode(createTestToxID(1), newMockAddr("1.1.1.1:2")),
				expected: true,
			},
			{
				name:    "Add to non-full bucket",
				maxSize: 2,
				setup: func(kb *KBucket) {
					kb.AddNode(NewNode(createTestToxID(1), newMockAddr("1.1.1.1:1")))
				},
				node:     NewNode(createTestToxID(2), newMockAddr("1.1.1.2:1")),
				expected: true,
			},
			{
				name:    "Add to full bucket with no bad nodes",
				maxSize: 2,
				setup: func(kb *KBucket) {
					kb.AddNode(NewNode(createTestToxID(1), newMockAddr("1.1.1.1:1")))
					kb.AddNode(NewNode(createTestToxID(2), newMockAddr("1.1.1.2:1")))
				},
				node:     NewNode(createTestToxID(3), newMockAddr("1.1.1.3:1")),
				expected: false,
			},
			{
				name:    "Add to full bucket replacing bad node",
				maxSize: 2,
				setup: func(kb *KBucket) {
					n1 := NewNode(createTestToxID(1), newMockAddr("1.1.1.1:1"))
					n1.Status = StatusBad
					kb.AddNode(n1)
					kb.AddNode(NewNode(createTestToxID(2), newMockAddr("1.1.1.2:1")))
				},
				node:     NewNode(createTestToxID(3), newMockAddr("1.1.1.3:1")),
				expected: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Arrange
				kb := NewKBucket(tc.maxSize)
				tc.setup(kb)

				// Act
				result := kb.AddNode(tc.node)

				// Assert
				if result != tc.expected {
					t.Errorf("Expected AddNode to return %v, got %v", tc.expected, result)
				}

				if result {
					// Verify node was added
					foundNode := false
					for _, node := range kb.nodes {
						if node.ID.String() == tc.node.ID.String() {
							foundNode = true
							break
						}
					}
					if !foundNode {
						t.Errorf("Node was not added to bucket despite returning true")
					}
				}

				// Verify bucket size constraints
				if len(kb.nodes) > kb.maxSize {
					t.Errorf("Bucket exceeds max size: %d > %d", len(kb.nodes), kb.maxSize)
				}
			})
		}
	})

	t.Run("GetNodes", func(t *testing.T) {
		// Arrange
		kb := NewKBucket(4)
		nodes := []*Node{
			NewNode(createTestToxID(1), newMockAddr("1.1.1.1:1")),
			NewNode(createTestToxID(2), newMockAddr("1.1.1.2:1")),
		}

		for _, node := range nodes {
			kb.AddNode(node)
		}

		// Act
		result := kb.GetNodes()

		// Assert
		if len(result) != len(nodes) {
			t.Errorf("Expected %d nodes, got %d", len(nodes), len(result))
		}

		// Verify the returned slice is a copy
		result[0] = NewNode(createTestToxID(3), newMockAddr("1.1.1.3:1"))
		if kb.nodes[0].ID.String() == result[0].ID.String() {
			t.Error("GetNodes should return a copy, not the original slice")
		}
	})

	t.Run("RemoveNode", func(t *testing.T) {
		// Arrange
		kb := NewKBucket(4)
		node1 := NewNode(createTestToxID(1), newMockAddr("1.1.1.1:1"))
		node2 := NewNode(createTestToxID(2), newMockAddr("1.1.1.2:1"))
		node3 := NewNode(createTestToxID(3), newMockAddr("1.1.1.3:1"))

		kb.AddNode(node1)
		kb.AddNode(node2)
		kb.AddNode(node3)

		// Test cases
		testCases := []struct {
			name        string
			nodeID      string
			expectedLen int
			expected    bool
		}{
			{"Remove existing node", node2.ID.String(), 2, true},
			{"Remove non-existent node", "non-existent-id", 2, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Act
				result := kb.RemoveNode(tc.nodeID)

				// Assert
				if result != tc.expected {
					t.Errorf("Expected RemoveNode to return %v, got %v", tc.expected, result)
				}

				if len(kb.nodes) != tc.expectedLen {
					t.Errorf("Expected bucket to contain %d nodes, got %d", tc.expectedLen, len(kb.nodes))
				}

				if tc.expected {
					// Verify node was removed
					for _, node := range kb.nodes {
						if node.ID.String() == tc.nodeID {
							t.Error("Node was not removed despite RemoveNode returning true")
						}
					}
				}
			})
		}
	})
}

// TestRoutingTable tests the RoutingTable implementation
func TestRoutingTable(t *testing.T) {
	t.Run("NewRoutingTable", func(t *testing.T) {
		// Arrange
		selfID := createTestToxID(1)
		maxBucketSize := 8

		// Act
		rt := NewRoutingTable(selfID, maxBucketSize)

		// Assert
		if rt == nil {
			t.Fatal("Expected RoutingTable not to be nil")
		}
		if rt.maxNodes != maxBucketSize*256 {
			t.Errorf("Expected maxNodes %d, got %d", maxBucketSize*256, rt.maxNodes)
		}
		if !reflect.DeepEqual(rt.selfID, selfID) {
			t.Errorf("Expected selfID %v, got %v", selfID, rt.selfID)
		}

		// Check if all k-buckets were initialized
		for i := 0; i < 256; i++ {
			if rt.kBuckets[i] == nil {
				t.Errorf("Expected k-bucket at index %d to be initialized", i)
			}
		}
	})

	t.Run("AddNode", func(t *testing.T) {
		// Arrange
		selfID := createTestToxID(1)
		rt := NewRoutingTable(selfID, 8)

		// Test cases
		testCases := []struct {
			name     string
			node     *Node
			expected bool
		}{
			{
				name:     "Add normal node",
				node:     NewNode(createTestToxID(2), newMockAddr("1.1.1.1:1")),
				expected: true,
			},
			{
				name:     "Add self node",
				node:     NewNode(selfID, newMockAddr("1.1.1.1:1")),
				expected: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Act
				result := rt.AddNode(tc.node)

				// Assert
				if result != tc.expected {
					t.Errorf("Expected AddNode to return %v, got %v", tc.expected, result)
				}

				if tc.expected {
					// Try to find the node in the appropriate bucket
					dist := tc.node.Distance(&Node{ID: rt.selfID})
					bucketIndex := getBucketIndex(dist)

					found := false
					for _, node := range rt.kBuckets[bucketIndex].GetNodes() {
						if node.ID.String() == tc.node.ID.String() {
							found = true
							break
						}
					}

					if !found {
						t.Errorf("Node was not added to the appropriate bucket")
					}
				}
			})
		}
	})

	t.Run("FindClosestNodes", func(t *testing.T) {
		// Arrange
		selfID := createTestToxID(0x00)
		rt := NewRoutingTable(selfID, 8)

		// Add some nodes with varying distances
		nodes := []*Node{
			NewNode(createTestToxID(0x01), newMockAddr("1.1.1.1:1")), // Close
			NewNode(createTestToxID(0x02), newMockAddr("1.1.1.2:1")), // Closer
			NewNode(createTestToxID(0x10), newMockAddr("1.1.1.3:1")), // Far
			NewNode(createTestToxID(0xFF), newMockAddr("1.1.1.4:1")), // Farthest
		}

		for _, node := range nodes {
			rt.AddNode(node)
		}

		// Act
		targetID := createTestToxID(0x03) // Close to 0x02
		result := rt.FindClosestNodes(targetID, 2)

		// Assert
		if len(result) != 2 {
			t.Errorf("Expected 2 nodes, got %d", len(result))
		}

		// The closest nodes to 0x03 should be 0x02 (distance 0x01) and 0x01 (distance 0x02)
		if result[0].PublicKey[0] != 0x02 {
			t.Errorf("Expected closest node to have public key starting with 0x02, got 0x%02x", result[0].PublicKey[0])
		}
		if result[1].PublicKey[0] != 0x01 {
			t.Errorf("Expected second closest node to have public key starting with 0x01, got 0x%02x", result[1].PublicKey[0])
		}
	})

	t.Run("getBucketIndex", func(t *testing.T) {
		// Test cases
		testCases := []struct {
			name     string
			distance [32]byte
			expected int
		}{
			{
				name: "First byte, MSB",
				distance: func() [32]byte {
					var d [32]byte
					d[0] = 0x80 // 10000000
					return d
				}(),
				expected: 0,
			},
			{
				name: "First byte, LSB",
				distance: func() [32]byte {
					var d [32]byte
					d[0] = 0x01 // 00000001
					return d
				}(),
				expected: 7,
			},
			{
				name: "Second byte, MSB",
				distance: func() [32]byte {
					var d [32]byte
					d[1] = 0x80 // 10000000
					return d
				}(),
				expected: 8,
			},
			{
				name: "Last byte, LSB",
				distance: func() [32]byte {
					var d [32]byte
					d[31] = 0x01 // 00000001
					return d
				}(),
				expected: 255,
			},
			{
				name:     "All zeros (should not happen in practice)",
				distance: [32]byte{},
				expected: 255,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Act
				result := getBucketIndex(tc.distance)

				// Assert
				if result != tc.expected {
					t.Errorf("Expected bucket index %d, got %d", tc.expected, result)
				}
			})
		}
	})
}

// TestBootstrapManager tests the BootstrapManager implementation
func TestBootstrapManager(t *testing.T) {
	t.Run("NewBootstrapManager", func(t *testing.T) {
		// Arrange
		selfID := createTestToxID(1)
		addr := newMockAddr("local:1234")
		transport := newMockTransport(addr)
		routingTable := NewRoutingTable(selfID, 8)

		// Act
		bm := NewBootstrapManager(selfID, transport, routingTable)

		// Assert
		if bm == nil {
			t.Fatal("Expected BootstrapManager not to be nil")
		}
		if bm.bootstrapped {
			t.Error("New bootstrap manager should not be bootstrapped")
		}
		if len(bm.nodes) != 0 {
			t.Errorf("Expected empty nodes list, got %d nodes", len(bm.nodes))
		}
	})

	t.Run("AddNode", func(t *testing.T) {
		// Arrange
		selfID := createTestToxID(1)
		addr := newMockAddr("local:1234")
		transport := newMockTransport(addr)
		routingTable := NewRoutingTable(selfID, 8)
		bm := NewBootstrapManager(selfID, transport, routingTable)

		// Test cases
		testCases := []struct {
			name         string
			address      string
			port         uint16
			publicKeyHex string
			expectError  bool
		}{
			{
				name:         "Valid node",
				address:      "example.com",
				port:         33445,
				publicKeyHex: "0000000000000000000000000000000000000000000000000000000000000000",
				expectError:  false,
			},
			{
				name:         "Invalid public key",
				address:      "example.com",
				port:         33445,
				publicKeyHex: "too-short",
				expectError:  true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Act
				err := bm.AddNode(tc.address, tc.port, tc.publicKeyHex)

				// Assert
				if (err != nil) != tc.expectError {
					t.Errorf("Expected error: %v, got: %v", tc.expectError, err)
				}

				if err == nil {
					// Verify node was added
					nodes := bm.GetNodes()
					found := false
					for _, node := range nodes {
						if node.Address == tc.address && node.Port == tc.port {
							found = true
							break
						}
					}

					if !found {
						t.Error("Node was not added despite no error")
					}
				}
			})
		}
	})

	// More tests for other methods would follow...
}

// TestMaintenanceConfig tests MaintenanceConfig and related functions
func TestMaintenanceConfig(t *testing.T) {
	t.Run("DefaultMaintenanceConfig", func(t *testing.T) {
		// Act
		config := DefaultMaintenanceConfig()

		// Assert
		if config == nil {
			t.Fatal("Expected config not to be nil")
		}
		if config.PingInterval != 1*time.Minute {
			t.Errorf("Expected PingInterval to be 1 minute, got %v", config.PingInterval)
		}
		if config.LookupInterval != 5*time.Minute {
			t.Errorf("Expected LookupInterval to be 5 minutes, got %v", config.LookupInterval)
		}
		if config.NodeTimeout != 10*time.Minute {
			t.Errorf("Expected NodeTimeout to be 10 minutes, got %v", config.NodeTimeout)
		}
		if config.PruneTimeout != 1*time.Hour {
			t.Errorf("Expected PruneTimeout to be 1 hour, got %v", config.PruneTimeout)
		}
	})
}

// TestMaintainer tests the DHT maintenance functionality
func TestMaintainer(t *testing.T) {
	t.Run("NewMaintainer", func(t *testing.T) {
		// Arrange
		selfID := createTestToxID(1)
		selfNode := NewNode(selfID, newMockAddr("local:1234"))
		transport := newMockTransport(selfNode.Address)
		routingTable := NewRoutingTable(selfID, 8)
		bootstrapper := NewBootstrapManager(selfID, transport, routingTable)

		testCases := []struct {
			name          string
			config        *MaintenanceConfig
			expectDefault bool
		}{
			{
				name: "With custom config",
				config: &MaintenanceConfig{
					PingInterval:   30 * time.Second,
					LookupInterval: 2 * time.Minute,
					NodeTimeout:    5 * time.Minute,
					PruneTimeout:   30 * time.Minute,
				},
				expectDefault: false,
			},
			{
				name:          "With nil config",
				config:        nil,
				expectDefault: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Act
				maintainer := NewMaintainer(routingTable, bootstrapper, transport, selfNode, tc.config)

				// Assert
				if maintainer == nil {
					t.Fatal("Expected maintainer not to be nil")
				}

				if tc.expectDefault {
					// Should have default config values
					if maintainer.config.PingInterval != 1*time.Minute {
						t.Errorf("Expected default PingInterval, got %v", maintainer.config.PingInterval)
					}
				} else {
					// Should have custom config values
					if maintainer.config != tc.config {
						t.Errorf("Expected provided config, got different config")
					}
				}

				// Check other fields
				if maintainer.isRunning {
					t.Error("New maintainer should not be running")
				}
				if time.Since(maintainer.lastActivity) > time.Second {
					t.Error("lastActivity should be initialized to current time")
				}
			})
		}
	})

	t.Run("Start and Stop", func(t *testing.T) {
		// Arrange
		selfID := createTestToxID(1)
		selfNode := NewNode(selfID, newMockAddr("local:1234"))
		transport := newMockTransport(selfNode.Address)
		routingTable := NewRoutingTable(selfID, 8)
		bootstrapper := NewBootstrapManager(selfID, transport, routingTable)

		// Create a maintainer with very short intervals for testing
		config := &MaintenanceConfig{
			PingInterval:   10 * time.Millisecond,
			LookupInterval: 10 * time.Millisecond,
			NodeTimeout:    100 * time.Millisecond,
			PruneTimeout:   200 * time.Millisecond,
		}

		maintainer := NewMaintainer(routingTable, bootstrapper, transport, selfNode, config)

		// Act
		err := maintainer.Start()
		// Assert
		if err != nil {
			t.Fatalf("Expected no error from Start(), got %v", err)
		}

		if !maintainer.isRunning {
			t.Error("Maintainer should be running after Start()")
		}

		// Start again, should be no-op
		err = maintainer.Start()
		if err != nil {
			t.Errorf("Expected no error from second Start(), got %v", err)
		}

		// Wait a bit to ensure maintenance routines run
		time.Sleep(50 * time.Millisecond)

		// Check that packets were sent (maintenance is running)
		packets, _ := transport.GetSentPackets()
		if len(packets) == 0 {
			t.Error("Expected maintenance routines to send packets")
		}

		// Stop the maintainer
		maintainer.Stop()

		if maintainer.isRunning {
			t.Error("Maintainer should not be running after Stop()")
		}

		// Reset sent packets
		transport.ResetSentPackets()

		// Wait to ensure maintenance routines are stopped
		time.Sleep(50 * time.Millisecond)

		// Check that no new packets were sent
		packets, _ = transport.GetSentPackets()
		if len(packets) > 0 {
			t.Error("Expected no packets after Stop()")
		}

		// Stop again, should be no-op
		maintainer.Stop()
	})

	t.Run("UpdateActivity", func(t *testing.T) {
		// Arrange
		selfID := createTestToxID(1)
		selfNode := NewNode(selfID, newMockAddr("local:1234"))
		transport := newMockTransport(selfNode.Address)
		routingTable := NewRoutingTable(selfID, 8)
		bootstrapper := NewBootstrapManager(selfID, transport, routingTable)
		maintainer := NewMaintainer(routingTable, bootstrapper, transport, selfNode, nil)

		// Record initial time
		initialTime := maintainer.GetLastActivity()

		// Wait a bit
		time.Sleep(10 * time.Millisecond)

		// Act
		maintainer.UpdateActivity()

		// Assert
		updatedTime := maintainer.GetLastActivity()
		if !updatedTime.After(initialTime) {
			t.Error("Expected lastActivity to be updated to a later time")
		}
	})
}

// TestHandler tests the packet handling functionality
func TestPacketHandlers(t *testing.T) {
	t.Run("handleSendNodesPacket", func(t *testing.T) {
		// Arrange
		selfID := createTestToxID(0x01)
		addr := newMockAddr("local:1234")
		mockNodeTransport := newMockTransport(addr)
		routingTable := NewRoutingTable(selfID, 8)
		bm := NewBootstrapManager(selfID, mockNodeTransport, routingTable)

		// Create a send_nodes packet
		// Format: [sender_pk(32)][num_nodes(1)][node_entries]
		data := make([]byte, 33)
		senderPK := [32]byte{0x02}
		copy(data[:32], senderPK[:])
		data[32] = 0 // No nodes

		packet := &transport.Packet{
			PacketType: transport.PacketSendNodes,
			Data:       data,
		}

		senderAddr := &net.UDPAddr{IP: net.ParseIP("1.2.3.4"), Port: 33445}

		// Act
		err := bm.HandlePacket(packet, senderAddr)
		// Assert
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Check if sender was added to routing table
		var nospam [4]byte
		senderID := crypto.NewToxID(senderPK, nospam)
		nodes := routingTable.FindClosestNodes(*senderID, 1)

		if len(nodes) != 1 || nodes[0].ID.String() != senderID.String() {
			t.Error("Sender node was not added to routing table")
		}

		// Check sender status
		if nodes[0].Status != StatusGood {
			t.Errorf("Expected sender status to be StatusGood, got %v", nodes[0].Status)
		}
	})

	t.Run("handlePingPacket", func(t *testing.T) {
		// Arrange
		selfID := createTestToxID(0x01)
		addr := newMockAddr("local:1234")
		mockTransport := newMockTransport(addr)
		routingTable := NewRoutingTable(selfID, 8)
		bm := NewBootstrapManager(selfID, mockTransport, routingTable)

		// Create a ping packet (just some data)
		pingData := []byte{0x01, 0x02, 0x03}
		packet := &transport.Packet{
			PacketType: transport.PacketPingRequest,
			Data:       pingData,
		}

		senderAddr := &net.UDPAddr{IP: net.ParseIP("1.2.3.4"), Port: 33445}

		// Act
		err := bm.HandlePacket(packet, senderAddr)
		// Assert
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Check if a ping response was sent
		packets, addrs := mockTransport.GetSentPackets()
		if len(packets) != 1 {
			t.Fatalf("Expected 1 packet, got %d", len(packets))
		}

		if packets[0].PacketType != transport.PacketPingResponse {
			t.Errorf("Expected PacketPingResponse, got %v", packets[0].PacketType)
		}

		if !bytes.Equal(packets[0].Data, pingData) {
			t.Error("Ping response data should match request data")
		}

		if addrs[0].String() != senderAddr.String() {
			t.Errorf("Response sent to wrong address: %s instead of %s", addrs[0], senderAddr)
		}
	})

	// More handler tests would follow...
}
