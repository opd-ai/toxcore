// Package testing provides simulation-based packet delivery infrastructure for
// deterministic testing of the toxcore-go library.
//
// # Overview
//
// This package implements a simulated packet delivery system that mirrors the
// production network transport but operates entirely in-memory. This allows
// tests to verify packet delivery logic without actual network operations,
// ensuring reproducible and fast test execution.
//
// # Simulation vs Real Implementation
//
// The toxcore-go library supports two packet delivery modes:
//
//   - Simulation (this package): All packets are delivered in-memory with
//     delivery logs for verification. Used for unit and integration testing.
//
//   - Real (real package): Packets are transmitted over actual network
//     connections using UDP/TCP. Used for production deployments.
//
// Both implementations conform to the interfaces.IPacketDelivery interface,
// allowing seamless switching via the factory package.
//
// # Usage
//
// Create a simulated delivery instance for testing:
//
//	config := &interfaces.PacketDeliveryConfig{
//	    UseSimulation:   true,
//	    NetworkTimeout:  5000,
//	    RetryAttempts:   3,
//	    EnableBroadcast: true,
//	}
//	sim := testing.NewSimulatedPacketDelivery(config)
//
//	// Register friends for the simulation
//	sim.AddFriend(1)
//	sim.AddFriend(2)
//
//	// Deliver packets
//	err := sim.DeliverPacket(1, []byte("hello"))
//
//	// Verify delivery via logs
//	log := sim.GetDeliveryLog()
//	if len(log) != 1 || !log[0].Success {
//	    t.Error("expected successful delivery")
//	}
//
// # Delivery Logs
//
// The simulation maintains a complete delivery log that can be inspected
// during test verification. Each DeliveryRecord contains:
//
//   - FriendID: The target friend identifier
//   - PacketSize: Size of the delivered packet in bytes
//   - Timestamp: Unix nanoseconds when delivery occurred
//   - Success: Whether the delivery succeeded
//   - Error: Any error that occurred during delivery
//
// Use GetDeliveryLog to retrieve the log, and ClearDeliveryLog to reset
// between test cases.
//
// # Thread Safety
//
// All methods on SimulatedPacketDelivery are safe for concurrent use from
// multiple goroutines. Internal synchronization uses sync.RWMutex.
package testing
