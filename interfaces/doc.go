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
// like friend address management and connection state tracking.
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
