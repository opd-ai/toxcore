// Package real provides production network-based packet delivery for toxcore-go.
//
// # Stability
//
// This package is stable (v1.x) and follows semantic versioning:
//   - STABLE API: Core types and methods (NewRealPacketDelivery, DeliverPacket,
//     BroadcastPacket, AddFriend, RemoveFriend, GetTypedStats)
//   - DEPRECATED: GetStats() - will be removed in v2.0.0, use GetTypedStats()
//
// # Version Compatibility
//
// Compatible with toxcore-go v1.0.0 and later. Breaking changes are reserved
// for major version releases only.
//
// This package implements the interfaces.IPacketDelivery interface using actual
// network transports, enabling real peer-to-peer communication over the Tox
// protocol. It serves as the production implementation, distinct from simulation
// or mock implementations used for testing.
//
// # Architecture
//
// The package centers on RealPacketDelivery, which wraps an INetworkTransport
// to provide reliable packet delivery with automatic retries, friend address
// caching, and broadcast capabilities:
//
//	┌─────────────────────────────────────────┐
//	│          RealPacketDelivery             │
//	│  ┌─────────────┐  ┌─────────────────┐   │
//	│  │ Friend Addr │  │   Retry Logic   │   │
//	│  │   Cache     │  │ (configurable)  │   │
//	│  └─────────────┘  └─────────────────┘   │
//	└───────────────┬─────────────────────────┘
//	                │
//	                ▼
//	┌─────────────────────────────────────────┐
//	│      INetworkTransport (abstraction)    │
//	│    UDP/TCP transport implementations    │
//	└─────────────────────────────────────────┘
//
// # Usage
//
// Create a RealPacketDelivery instance with a transport and configuration:
//
//	config := &interfaces.PacketDeliveryConfig{
//	    NetworkTimeout:  30 * time.Second,
//	    RetryAttempts:   3,
//	    EnableBroadcast: true,
//	}
//
//	transport := // ... obtain INetworkTransport instance
//	delivery := real.NewRealPacketDelivery(transport, config)
//
//	// Register friends before sending
//	delivery.AddFriend(friendID, friendAddr)
//
//	// Send packets with automatic retry
//	err := delivery.DeliverPacket(friendID, packetData)
//
// # Factory Integration
//
// The package is typically instantiated via the factory pattern:
//
//	factory := factory.NewPacketDeliveryFactory()
//	delivery := factory.CreateRealDelivery(transport, config)
//
// This ensures consistent initialization and proper interface compliance
// throughout the toxcore-go codebase.
//
// # Retry Behavior
//
// DeliverPacket automatically retries failed deliveries based on the
// RetryAttempts configuration. Each retry uses exponential backoff
// (500ms * attempt number) to avoid overwhelming the network during
// transient failures.
//
// # Thread Safety
//
// All methods on RealPacketDelivery are safe for concurrent use.
// The implementation uses sync.RWMutex to protect the friend address
// cache while allowing concurrent reads during packet delivery.
//
// # Testing Support
//
// The package supports deterministic testing via the Sleeper interface,
// which can be injected using SetSleeper() to control retry timing:
//
//	type mockSleeper struct {
//	    sleepCalls []time.Duration
//	}
//
//	func (m *mockSleeper) Sleep(d time.Duration) {
//	    m.sleepCalls = append(m.sleepCalls, d)
//	}
//
//	delivery.SetSleeper(&mockSleeper{})
//
// # Comparison with Simulation
//
// The real package provides actual network delivery, while simulation
// implementations (testnet package) provide in-memory packet routing
// for testing without network I/O. Use real for:
//
//   - Production deployments
//   - Integration tests requiring network behavior
//   - Performance benchmarking with real network latency
//
// Use simulation implementations for:
//
//   - Unit tests requiring deterministic behavior
//   - CI/CD pipelines without network access
//   - Protocol development and debugging
package real
