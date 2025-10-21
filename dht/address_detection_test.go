package dht

import (
	"net"
	"testing"

	"github.com/opd-ai/toxforge/async"
	"github.com/opd-ai/toxforge/crypto"
	"github.com/opd-ai/toxforge/transport"
	"github.com/sirupsen/logrus"
)

func TestDHTAddressTypeDetection(t *testing.T) {
	// Set up logging for tests
	logrus.SetLevel(logrus.InfoLevel)

	t.Run("AddressTypeDetectorBasicFunctionality", func(t *testing.T) {
		detector := NewAddressTypeDetector()

		// Test IPv4 detection
		addr, _ := net.ResolveUDPAddr("udp", "192.168.1.1:33445")
		addrType, err := detector.DetectAddressType(addr)
		if err != nil {
			t.Errorf("Failed to detect IPv4 address type: %v", err)
		}
		if addrType != transport.AddressTypeIPv4 {
			t.Errorf("Expected IPv4, got %v", addrType)
		}

		// Test IPv6 detection
		addr6, _ := net.ResolveUDPAddr("udp", "[2001:db8::1]:33445")
		addrType6, err := detector.DetectAddressType(addr6)
		if err != nil {
			t.Errorf("Failed to detect IPv6 address type: %v", err)
		}
		if addrType6 != transport.AddressTypeIPv6 {
			t.Errorf("Expected IPv6, got %v", addrType6)
		}

		// Test validation
		if !detector.ValidateAddressType(transport.AddressTypeIPv4) {
			t.Error("IPv4 should be supported")
		}
		if !detector.IsRoutableAddress(transport.AddressTypeIPv4) {
			t.Error("IPv4 should be routable")
		}
	})

	t.Run("AddressTypeStatistics", func(t *testing.T) {
		stats := &AddressTypeStats{}

		// Test incrementing counts
		stats.IncrementCount(transport.AddressTypeIPv4)
		stats.IncrementCount(transport.AddressTypeIPv4)
		stats.IncrementCount(transport.AddressTypeOnion)

		if stats.IPv4Count != 2 {
			t.Errorf("Expected IPv4 count 2, got %d", stats.IPv4Count)
		}
		if stats.OnionCount != 1 {
			t.Errorf("Expected Onion count 1, got %d", stats.OnionCount)
		}
		if stats.TotalCount != 3 {
			t.Errorf("Expected total count 3, got %d", stats.TotalCount)
		}

		// Test dominant type detection
		dominant := stats.GetDominantAddressType()
		if dominant != transport.AddressTypeIPv4 {
			t.Errorf("Expected dominant type IPv4, got %v", dominant)
		}
	})

	t.Run("BootstrapManagerWithAddressDetection", func(t *testing.T) {
		// Create test components
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		mockTransport := async.NewMockTransport("127.0.0.1:33445")

		var nospam [4]byte
		routingTable := NewRoutingTable(*crypto.NewToxID(keyPair.Public, nospam), 20)
		toxID := *crypto.NewToxID(keyPair.Public, nospam)

		// Create bootstrap manager with address detection
		bm := NewBootstrapManagerWithKeyPair(toxID, keyPair, mockTransport, routingTable)

		// Test address validation
		validAddr, _ := net.ResolveUDPAddr("udp", "192.168.1.1:33445")
		err = bm.ValidateNodeAddress(validAddr)
		if err != nil {
			t.Errorf("Valid IPv4 address should pass validation: %v", err)
		}

		// Test nil address
		err = bm.ValidateNodeAddress(nil)
		if err == nil {
			t.Error("Nil address should fail validation")
		}

		// Test supported address types
		supportedTypes := bm.GetSupportedAddressTypes()
		expectedTypes := []transport.AddressType{
			transport.AddressTypeIPv4,
			transport.AddressTypeIPv6,
			transport.AddressTypeOnion,
			transport.AddressTypeI2P,
			transport.AddressTypeNym,
			transport.AddressTypeLoki,
		}

		if len(supportedTypes) != len(expectedTypes) {
			t.Errorf("Expected %d supported types, got %d", len(expectedTypes), len(supportedTypes))
		}

		// Verify all expected types are supported
		supportedMap := make(map[transport.AddressType]bool)
		for _, addrType := range supportedTypes {
			supportedMap[addrType] = true
		}

		for _, expected := range expectedTypes {
			if !supportedMap[expected] {
				t.Errorf("Expected address type %v to be supported", expected)
			}
		}
	})

	t.Run("AddressTypeFilteringInResponseBuilding", func(t *testing.T) {
		// Create test setup
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		mockTransport := async.NewMockTransport("127.0.0.1:33445")

		var nospam [4]byte
		routingTable := NewRoutingTable(*crypto.NewToxID(keyPair.Public, nospam), 20)
		toxID := *crypto.NewToxID(keyPair.Public, nospam)
		bm := NewBootstrapManagerWithKeyPair(toxID, keyPair, mockTransport, routingTable)

		// Create test nodes with different address types
		nodes := []*Node{
			{
				ID:      *crypto.NewToxID(keyPair.Public, nospam),
				Address: &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445},
			},
		}

		// Test response building with legacy protocol (should filter appropriately)
		responseData := bm.buildVersionedResponseData(nodes, transport.ProtocolLegacy)
		if len(responseData) < 33 { // Should at least have header
			t.Error("Response data should contain header even if nodes are filtered")
		}

		// Test response building with extended protocol
		extendedResponseData := bm.buildVersionedResponseData(nodes, transport.ProtocolNoiseIK)
		if len(extendedResponseData) < 33 {
			t.Error("Extended response data should contain header")
		}

		t.Logf("Legacy response size: %d, Extended response size: %d", len(responseData), len(extendedResponseData))
	})

	t.Run("AddressTypeStatisticsIntegration", func(t *testing.T) {
		// Create test setup
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		mockTransport := async.NewMockTransport("127.0.0.1:33445")

		var nospam [4]byte
		routingTable := NewRoutingTable(*crypto.NewToxID(keyPair.Public, nospam), 20)
		toxID := *crypto.NewToxID(keyPair.Public, nospam)
		bm := NewBootstrapManagerWithKeyPair(toxID, keyPair, mockTransport, routingTable)

		// Create test node entry
		nodeEntry := &transport.NodeEntry{
			PublicKey: keyPair.Public,
			Address: &transport.NetworkAddress{
				Type:    transport.AddressTypeIPv4,
				Data:    []byte{192, 168, 1, 1},
				Port:    33445,
				Network: "udp",
			},
		}

		// Process the node entry (this should increment statistics)
		err = bm.processNodeEntryVersionAware(nodeEntry, nospam)
		if err != nil {
			t.Errorf("Failed to process node entry: %v", err)
		}

		// Check statistics
		stats := bm.GetAddressTypeStats()
		if stats.IPv4Count != 1 {
			t.Errorf("Expected IPv4 count 1, got %d", stats.IPv4Count)
		}
		if stats.TotalCount != 1 {
			t.Errorf("Expected total count 1, got %d", stats.TotalCount)
		}

		// Test dominant type
		dominant := bm.GetDominantNetworkType()
		if dominant != transport.AddressTypeIPv4 {
			t.Errorf("Expected dominant type IPv4, got %v", dominant)
		}

		// Test statistics reset
		bm.ResetAddressTypeStats()
		statsAfterReset := bm.GetAddressTypeStats()
		if statsAfterReset.TotalCount != 0 {
			t.Errorf("Expected total count 0 after reset, got %d", statsAfterReset.TotalCount)
		}
	})

	t.Run("MultiNetworkAddressSupport", func(t *testing.T) {
		detector := NewAddressTypeDetector()

		// Test different network address formats
		testCases := []struct {
			network      string
			address      string
			expectedType transport.AddressType
		}{
			{"udp", "192.168.1.1:33445", transport.AddressTypeIPv4},
			{"udp", "[2001:db8::1]:33445", transport.AddressTypeIPv6},
			// Note: For .onion, .i2p etc., we would need custom addr implementations
			// as net.ResolveUDPAddr doesn't handle them
		}

		for _, tc := range testCases {
			addr, err := net.ResolveUDPAddr(tc.network, tc.address)
			if err != nil {
				t.Logf("Skipping test case %s:%s (resolve error: %v)", tc.network, tc.address, err)
				continue
			}

			detectedType, err := detector.DetectAddressType(addr)
			if err != nil {
				t.Errorf("Failed to detect address type for %s:%s: %v", tc.network, tc.address, err)
				continue
			}

			if detectedType != tc.expectedType {
				t.Errorf("For %s:%s, expected %v, got %v", tc.network, tc.address, tc.expectedType, detectedType)
			}

			// Test validation and routability
			if !detector.ValidateAddressType(detectedType) {
				t.Errorf("Address type %v should be supported", detectedType)
			}

			if !detector.IsRoutableAddress(detectedType) {
				t.Errorf("Address type %v should be routable", detectedType)
			}
		}
	})
}
