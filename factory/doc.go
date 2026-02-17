// Package factory provides a factory pattern implementation for creating packet
// delivery implementations in toxcore-go.
//
// The factory abstracts the creation of packet delivery systems, allowing seamless
// switching between simulation (for testing) and real network implementations
// without changing consuming code.
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
// single retry attempt).
//
//	func TestMyFeature(t *testing.T) {
//	    factory := NewPacketDeliveryFactory()
//	    delivery := factory.CreateSimulationForTesting()
//	    // Use delivery in tests...
//	}
//
// # Mode Switching
//
// The factory supports runtime mode switching for integration testing:
//
//	factory := NewPacketDeliveryFactory()
//	factory.SwitchToSimulation()  // Switch to simulation mode
//	factory.SwitchToReal()        // Switch back to real mode
package factory
