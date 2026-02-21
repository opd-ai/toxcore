// Package interfaces defines core abstractions for packet delivery and network
// transport operations in toxcore-go.
//
// This package provides the foundational interfaces that enable switching between
// simulation and real network implementations, supporting both production deployments
// and deterministic testing scenarios.
//
// # Core Interfaces
//
// [IPacketDelivery] is the primary interface for delivering packets to friends.
// It abstracts the underlying transport mechanism, allowing the same application
// code to work with both real networks and simulated environments:
//
//	delivery := factory.NewPacketDeliveryFactory().CreatePacketDelivery(transport)
//	err := delivery.DeliverPacket(friendID, packetData)
//	if err != nil {
//	    log.Printf("delivery failed: %v", err)
//	}
//
// [INetworkTransport] extends transport capabilities with network-specific operations
// like friend address management and connection state tracking:
//
//	// Implement INetworkTransport for custom network backends
//	type MyTransport struct {
//	    friends map[uint32]net.Addr
//	    conn    net.PacketConn
//	}
//
//	func (t *MyTransport) Send(packet []byte, addr net.Addr) error {
//	    _, err := t.conn.WriteTo(packet, addr)
//	    return err
//	}
//
//	func (t *MyTransport) SendToFriend(friendID uint32, packet []byte) error {
//	    addr, ok := t.friends[friendID]
//	    if !ok {
//	        return errors.New("friend not registered")
//	    }
//	    return t.Send(packet, addr)
//	}
//
//	func (t *MyTransport) GetFriendAddress(friendID uint32) (net.Addr, error) {
//	    if addr, ok := t.friends[friendID]; ok {
//	        return addr, nil
//	    }
//	    return nil, errors.New("friend not found")
//	}
//
//	func (t *MyTransport) RegisterFriend(friendID uint32, addr net.Addr) error {
//	    if addr == nil {
//	        return errors.New("address cannot be nil")
//	    }
//	    t.friends[friendID] = addr
//	    return nil
//	}
//
//	func (t *MyTransport) Close() error { return t.conn.Close() }
//	func (t *MyTransport) IsConnected() bool { return t.conn != nil }
//
// # Configuration
//
// [PacketDeliveryConfig] holds settings for packet delivery implementations:
//
//	config := &interfaces.PacketDeliveryConfig{
//	    UseSimulation:   false,
//	    NetworkTimeout:  5000,  // milliseconds
//	    RetryAttempts:   3,
//	    EnableBroadcast: true,
//	}
//	if err := config.Validate(); err != nil {
//	    log.Fatalf("invalid config: %v", err)
//	}
//
// # Implementation Selection
//
// The factory package creates implementations based on configuration:
//   - UseSimulation=true: Creates SimulatedPacketDelivery from testing package
//   - UseSimulation=false: Creates RealPacketDelivery from real package
//
// Simulation implementations are useful for:
//   - Unit testing without network dependencies
//   - Deterministic test scenarios
//   - Debugging packet flow
//
// Real implementations are used for:
//   - Production deployments
//   - Integration testing with actual networks
//   - Performance benchmarking
//
// # Thread Safety
//
// All implementations of these interfaces must be safe for concurrent use.
// The interfaces do not specify internal synchronization requirements, but
// callers should expect that methods can be called from multiple goroutines.
//
// # Error Handling
//
// Methods return errors for:
//   - DeliverPacket: Friend not found, transport failure, timeout
//   - BroadcastPacket: Partial delivery failure (returns error if any delivery fails)
//   - SetNetworkTransport: Invalid transport, transport already closed
//   - Send/SendToFriend: Network errors, address resolution failures
//
// # Network Interface Compliance
//
// This package uses net.Addr interface types throughout, following toxcore-go
// networking standards. Implementations should never use concrete types like
// net.UDPAddr or net.TCPAddr directly.
package interfaces
