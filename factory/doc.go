// Package factory provides a factory pattern implementation for creating packet
// delivery implementations in toxcore-go.
//
// The factory abstracts the creation of packet delivery systems, allowing seamless
// switching between simulation (for testing) and real network implementations
// without changing consuming code. All factory methods are safe for concurrent use.
//
// # Factory Pattern Rationale
//
// The factory pattern is used here to:
//   - Decouple packet delivery consumers from concrete implementations
//   - Enable dependency injection for testing scenarios
//   - Centralize configuration management for packet delivery
//   - Support runtime switching between simulation and production modes
//
// # Configuration
//
// The factory supports configuration via environment variables:
//   - TOX_USE_SIMULATION: "true" or "false" to enable simulation mode
//   - TOX_NETWORK_TIMEOUT: integer milliseconds for network timeout
//   - TOX_RETRY_ATTEMPTS: integer number of retry attempts
//   - TOX_ENABLE_BROADCAST: "true" or "false" to enable broadcast
//
// # Usage
//
// Create a factory and use it to create packet delivery implementations:
//
//	// Create factory with default configuration
//	factory := NewPacketDeliveryFactory()
//
//	// Create real implementation with transport
//	delivery, err := factory.CreatePacketDelivery(transport)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Or create simulation for testing
//	simDelivery := factory.CreateSimulationForTesting()
//
// # Testing Support
//
// For testing scenarios, use CreateSimulationForTesting() which creates a
// simulation implementation with test-optimized configuration (shorter timeouts,
// single retry attempt). You can customize the test configuration using
// functional options:
//
//	func TestMyFeature(t *testing.T) {
//	    factory := NewPacketDeliveryFactory()
//	    // Use defaults
//	    delivery := factory.CreateSimulationForTesting()
//
//	    // Or customize with options
//	    delivery = factory.CreateSimulationForTesting(
//	        WithNetworkTimeout(5000),   // Custom timeout
//	        WithRetryAttempts(3),       // Custom retry count
//	        WithBroadcast(false),       // Disable broadcast
//	    )
//	    // Use delivery in tests...
//	}
//
// # Thread Safety
//
// All factory methods are protected by an internal mutex, making the factory
// safe for concurrent use across multiple goroutines. Configuration updates
// and reads are synchronized to prevent race conditions.
//
// # Mode Switching
//
// The factory supports runtime mode switching for integration testing:
//
//	factory := NewPacketDeliveryFactory()
//	factory.SwitchToSimulation()  // Switch to simulation mode
//	factory.SwitchToReal()        // Switch back to real mode
package factory
