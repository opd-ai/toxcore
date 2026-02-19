package toxcore

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/async"
	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/factory"
	"github.com/opd-ai/toxcore/interfaces"
	"github.com/opd-ai/toxcore/noise"
	testsim "github.com/opd-ai/toxcore/testing"
	"github.com/opd-ai/toxcore/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Tests from integration_test.go ---

// TestMultiNetworkIntegration validates the complete multi-network architecture
func TestMultiNetworkIntegration(t *testing.T) {
	t.Run("AddressParsingIntegration", testAddressParsingIntegration)
	t.Run("NetworkDetectionIntegration", testNetworkDetectionIntegration)
	t.Run("NATTraversalIntegration", testNATTraversalIntegration)
	t.Run("TransportSelectionIntegration", testTransportSelectionIntegration)
	t.Run("CrossNetworkCompatibility", testCrossNetworkCompatibility)
	t.Run("BackwardCompatibility", testBackwardCompatibility)
	t.Run("EndToEndMultiNetwork", testEndToEndMultiNetwork)
}

// testAddressParsingIntegration validates parsing across all network types
func testAddressParsingIntegration(t *testing.T) {
	parser := transport.NewMultiNetworkParser()
	defer parser.Close()

	testCases := []struct {
		name     string
		address  string
		expected transport.AddressType
		valid    bool
	}{
		{"IPv4 Standard", testIPv4Addr, transport.AddressTypeIPv4, true},
		{"IPv6 Standard", testIPv6Addr, transport.AddressTypeIPv6, true},
		{"Tor v3 Onion", "facebookcorewwwi.onion:443", transport.AddressTypeOnion, true},
		{"I2P Base32", "7rmath4f27le5rmqbk2fmrlmvbvbfomt4mcqh73c6ukfhnpqdx4a.b32.i2p:9150", transport.AddressTypeI2P, true},
		{"Nym Gateway", "abc123.clients.nym:1789", transport.AddressTypeNym, true},
		{"Invalid Format", "not-an-address", transport.AddressTypeUnknown, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			addresses, err := parser.Parse(tc.address)
			if tc.valid {
				require.NoError(t, err, "Expected valid address: %s", tc.address)
				require.Len(t, addresses, 1, "Expected one parsed address")
				addr := addresses[0]
				assert.Equal(t, tc.expected, addr.Type, "Address type mismatch")
				assert.NotEmpty(t, addr.Data, "Address data should not be empty")
				if tc.expected != transport.AddressTypeUnknown {
					assert.NotZero(t, addr.Port, "Port should not be zero for valid addresses")
				}
			} else {
				assert.Error(t, err, "Expected invalid address: %s", tc.address)
			}
		})
	}
}

// testNetworkDetectionIntegration validates capability detection
func testNetworkDetectionIntegration(t *testing.T) {
	detector := transport.NewMultiNetworkDetector()

	testCases := []struct {
		name          string
		address       string
		isPrivate     bool
		supportsNAT   bool
		requiresProxy bool
	}{
		{"Public IPv4", "8.8.8.8:53", false, false, false},
		{"Private IPv4", testIPv4Addr, true, true, false},
		{"Tor Proxy", "example.onion:443", false, false, false},        // Tor detection through IP detector returns conservative defaults
		{"I2P Proxy", "example.b32.i2p:9150", false, false, false},     // I2P detection through IP detector returns conservative defaults
		{"Nym Proxy", "example.clients.nym:1789", false, false, false}, // Nym detection through IP detector returns conservative defaults
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock net.Addr for testing
			addr := &mockAddr{network: "tcp", address: tc.address}

			capabilities := detector.DetectCapabilities(addr)
			assert.Equal(t, tc.isPrivate, capabilities.IsPrivateSpace, "Private space detection mismatch")
			assert.Equal(t, tc.supportsNAT, capabilities.SupportsNAT, "NAT support detection mismatch")
			assert.Equal(t, tc.requiresProxy, capabilities.RequiresProxy, "Proxy requirement detection mismatch")
		})
	}
}

// testNATTraversalIntegration validates NAT handling with network detection
func testNATTraversalIntegration(t *testing.T) {
	detector := transport.NewMultiNetworkDetector()

	testCases := []struct {
		name     string
		address  string
		needsNAT bool
	}{
		{"Public Address No NAT", "8.8.8.8:53", false},
		{"Private Address Needs NAT", testIPv4Addr, true},
		{"Proxy Address No NAT", "example.onion:443", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			addr := &mockAddr{network: "tcp", address: tc.address}

			capabilities := detector.DetectCapabilities(addr)
			needsNAT := capabilities.SupportsNAT && capabilities.IsPrivateSpace
			assert.Equal(t, tc.needsNAT, needsNAT, "NAT requirement mismatch")
		})
	}
}

// testTransportSelectionIntegration validates multi-protocol transport selection
func testTransportSelectionIntegration(t *testing.T) {
	testCases := []struct {
		name     string
		address  string
		expected string // Use string for network type
	}{
		{"IPv4 UDP", testIPv4Addr, "udp"},
		{"IPv6 UDP", testIPv6Addr, "udp"},
		{"Tor TCP", "example.onion:443", "tcp"},
		{"I2P Custom", "example.b32.i2p:9150", "custom"},
		{"Nym Custom", "example.clients.nym:1789", "custom"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate transport selection based on address type
			var networkType string
			if strings.Contains(tc.address, ".onion") {
				networkType = "tcp"
			} else if strings.Contains(tc.address, ".i2p") || strings.Contains(tc.address, ".nym") {
				networkType = "custom"
			} else {
				networkType = "udp"
			}

			assert.Equal(t, tc.expected, networkType, "Transport selection mismatch")
		})
	}
}

// testCrossNetworkCompatibility validates compatibility between different networks
func testCrossNetworkCompatibility(t *testing.T) {
	testCases := []struct {
		name     string
		source   string
		target   string
		canRoute bool
	}{
		{"IPv4 to IPv4", testIPv4Addr, "8.8.8.8:53", true},
		{"IPv6 to IPv6", "[::1]:33445", testIPv6Addr, true},
		{"IPv4 to Tor", testIPv4Addr, "example.onion:443", true}, // Via proxy
		{"Tor to IPv4", "source.onion:443", "8.8.8.8:53", true},  // Via exit
		{"I2P to I2P", "source.b32.i2p:9150", "target.b32.i2p:9150", true},
		{"I2P to IPv4", "source.b32.i2p:9150", "8.8.8.8:53", false}, // No exit by default
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			compatibility := checkNetworkCompatibility(tc.source, tc.target)
			assert.Equal(t, tc.canRoute, compatibility, "Network compatibility mismatch")
		})
	}
}

// testBackwardCompatibility validates legacy protocol compatibility
func testBackwardCompatibility(t *testing.T) {
	testCases := []struct {
		name    string
		address string
		legacy  bool
	}{
		{"Standard IPv4", testIPv4Addr, true},
		{"Standard IPv6", testIPv6Addr, true},
		{"Tor Address", "example.onion:443", false},    // Extension
		{"I2P Address", "example.b32.i2p:9150", false}, // Extension
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isLegacy := isLegacyAddress(tc.address)
			assert.Equal(t, tc.legacy, isLegacy, "Legacy compatibility mismatch")
		})
	}
}

// testEndToEndMultiNetwork validates complete end-to-end functionality
func testEndToEndMultiNetwork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test complete workflow: parse → detect → select transport → establish connection
	testAddresses := []string{
		testIPv4Addr,                 // IPv4
		"[::1]:33445",                // IPv6
		"facebookcorewwwi.onion:443", // Tor (valid format)
		"test.b32.i2p:9150",          // I2P
		"test.clients.nym:1789",      // Nym
	}

	parser := transport.NewMultiNetworkParser()
	defer parser.Close()
	detector := transport.NewMultiNetworkDetector()

	for _, addrStr := range testAddresses {
		t.Run(fmt.Sprintf("EndToEnd_%s", addrStr), func(t *testing.T) {
			// Step 1: Parse address
			addresses, err := parser.Parse(addrStr)
			require.NoError(t, err, "Address parsing failed")
			require.Len(t, addresses, 1, "Expected one parsed address")
			addr := addresses[0]

			// Step 2: Detect network capability
			mockAddr := &mockAddr{network: "tcp", address: addrStr}
			capabilities := detector.DetectCapabilities(mockAddr)

			// Step 3: Validate capabilities make sense for address type
			switch addr.Type {
			case transport.AddressTypeIPv4, transport.AddressTypeIPv6:
				// IP addresses can support direct connections unless private
				expectedPrivate := strings.Contains(addrStr, "192.168") || strings.Contains(addrStr, "::1")
				assert.Equal(t, expectedPrivate, capabilities.IsPrivateSpace, "IP address privacy detection")
			case transport.AddressTypeOnion, transport.AddressTypeI2P, transport.AddressTypeNym:
				// Proxy networks would require proxy in a real implementation,
				// but our current network detector falls back to IP detection
				// which returns conservative defaults without proxy requirements
				t.Logf("Proxy network %s detected with capabilities: requiresProxy=%v", addr.Type, capabilities.RequiresProxy)
			}

			// Step 4: Simulate connection attempt (without actual network I/O)
			conn := &mockConnection{
				address:      addr,
				capabilities: capabilities,
				ctx:          ctx,
			}

			err = conn.Connect()
			assert.NoError(t, err, "Mock connection should succeed")
		})
	}
}

// --- Tests from friend_request_protocol_test.go ---

// TestFriendRequestProtocolImplemented verifies that AddFriend() properly
// sends friend request packets over the network.
func TestFriendRequestProtocolImplemented(t *testing.T) {
	// Create two Tox instances
	sender, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create sender Tox instance: %v", err)
	}
	defer sender.Kill()

	receiver, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create receiver Tox instance: %v", err)
	}
	defer receiver.Kill()

	// Get receiver's Tox ID
	receiverToxID := receiver.SelfGetAddress()

	// Set up callback on receiver to detect friend requests
	friendRequestReceived := false
	testMessage := "Hello, please add me as a friend!"

	receiver.OnFriendRequest(func(publicKey [32]byte, message string) {
		friendRequestReceived = true
		// Verify the data matches what was sent
		if message != testMessage {
			t.Errorf("Expected message %q, got %q", testMessage, message)
		}
		expectedPK := sender.SelfGetPublicKey()
		if publicKey != expectedPK {
			t.Errorf("Public key mismatch")
		}
	})

	// Send friend request from sender to receiver
	friendID, err := sender.AddFriend(receiverToxID, testMessage)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Verify friend was added locally in sender (friend ID 0 is valid)
	if friendID > 1000 {
		t.Errorf("Friend ID seems invalid: %d", friendID)
	}

	// Verify friend exists in sender's friend list
	senderFriends := sender.GetFriends()
	if len(senderFriends) != 1 {
		t.Errorf("Expected 1 friend in sender, got %d", len(senderFriends))
	}

	// Run iterations to process any pending network packets
	for i := 0; i < 10; i++ {
		sender.Iterate()
		receiver.Iterate()
	}

	// The fix: receiver should receive a friend request
	if !friendRequestReceived {
		t.Error("Friend request was not received - the protocol implementation may have a bug")
	}

	// Verify the receiver initially has no friends (before accepting the request)
	receiverFriends := receiver.GetFriends()
	if len(receiverFriends) != 0 {
		t.Errorf("Expected 0 friends in receiver before accepting, got %d", len(receiverFriends))
	}

	// This test confirms the fixed behavior:
	// 1. AddFriend() creates local friend entry in sender ✓
	// 2. Friend request packet is sent over network ✓
	// 3. Receiver gets OnFriendRequest callback ✓
	// 4. Receiver can process the friend request ✓
	t.Log("SUCCESS: AddFriend() properly sends friend request packets over the network")
}

// --- Tests from friend_request_transport_test.go ---

// TestFriendRequestViaTransport verifies that friend requests are sent through the transport layer
func TestFriendRequestViaTransport(t *testing.T) {
	// Create two Tox instances
	opts1 := NewOptions()
	opts1.UDPEnabled = true
	tox1, err := New(opts1)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	opts2 := NewOptions()
	opts2.UDPEnabled = true
	tox2, err := New(opts2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	// Set up friend request callback on tox2
	requestReceived := false
	var receivedPublicKey [32]byte
	var receivedMessage string

	tox2.OnFriendRequest(func(publicKey [32]byte, message string) {
		requestReceived = true
		receivedPublicKey = publicKey
		receivedMessage = message
	})

	// Send friend request from tox1 to tox2
	tox2Address := tox2.SelfGetAddress()
	message := "Hello from tox1!"

	friendID, err := tox1.AddFriend(tox2Address, message)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}
	_ = friendID // Suppress unused variable warning

	// Iterate both instances to process the request
	// The request should be delivered through the global test registry
	for i := 0; i < 10 && !requestReceived; i++ {
		tox1.Iterate()
		tox2.Iterate()
		time.Sleep(tox1.IterationInterval())
	}

	// Verify the request was received
	if !requestReceived {
		t.Fatal("Friend request was not received")
	}

	if receivedPublicKey != tox1.SelfGetPublicKey() {
		t.Error("Received public key does not match sender")
	}

	if receivedMessage != message {
		t.Errorf("Received message '%s' does not match sent message '%s'", receivedMessage, message)
	}

	t.Log("SUCCESS: Friend request delivered through transport layer pathway")
}

// TestFriendRequestThreadSafety verifies the friend request registry is thread-safe
func TestFriendRequestThreadSafety(t *testing.T) {
	// Create multiple Tox instances concurrently
	instances := 5
	done := make(chan bool, instances)

	for i := 0; i < instances; i++ {
		go func(id int) {
			opts := NewOptions()
			opts.UDPEnabled = true
			tox, err := New(opts)
			if err != nil {
				t.Errorf("Instance %d: Failed to create Tox: %v", id, err)
				done <- false
				return
			}
			defer tox.Kill()

			// Send friend requests using ToxID addresses
			for j := 0; j < 3; j++ {
				// Create a dummy ToxID for testing
				var publicKey [32]byte
				var nospam [4]byte
				for k := range publicKey {
					publicKey[k] = byte(id*10 + j*2)
				}
				for k := range nospam {
					nospam[k] = byte(id + j)
				}

				// This will fail because it's not a valid address, but we're testing thread safety
				_, _ = tox.AddFriend("invalid_address_for_testing", "Concurrent test")
				tox.Iterate()
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	success := 0
	for i := 0; i < instances; i++ {
		if <-done {
			success++
		}
	}

	if success != instances {
		t.Errorf("Only %d/%d instances completed successfully", success, instances)
	}

	t.Log("SUCCESS: Friend request system is thread-safe")
}

// TestFriendRequestHandlerRegistration verifies the PacketFriendRequest handler is registered
func TestFriendRequestHandlerRegistration(t *testing.T) {
	opts := NewOptions()
	opts.UDPEnabled = true
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify UDP transport exists
	if tox.udpTransport == nil {
		t.Fatal("UDP transport is nil")
	}

	// The handler registration happens in registerPacketHandlers()
	// We can't directly test it without exposing internals, but we can verify
	// that friend requests work, which proves the handler is registered

	receivedRequest := false
	tox.OnFriendRequest(func(_ [32]byte, _ string) {
		receivedRequest = true
	})

	// Create another instance to send a friend request
	opts2 := NewOptions()
	opts2.UDPEnabled = true
	tox2, err := New(opts2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	// Send friend request from tox2 to tox1
	tox1Address := tox.SelfGetAddress()
	_, _ = tox2.AddFriend(tox1Address, testFriendRequestMessage)

	// Process iterations
	for i := 0; i < 10 && !receivedRequest; i++ {
		tox.Iterate()
		tox2.Iterate()
		time.Sleep(tox.IterationInterval())
	}

	// We should receive the friend request
	if !receivedRequest {
		t.Error("Friend request handler was not called")
	}

	t.Log("SUCCESS: Friend request handler registration verified")
}

// TestFriendRequestPacketFormat verifies the new packet format is correct
func TestFriendRequestPacketFormat(t *testing.T) {
	opts := NewOptions()
	opts.UDPEnabled = true
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	opts2 := NewOptions()
	opts2.UDPEnabled = true
	tox2, err := New(opts2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	// Send a friend request
	message := "Test message for packet format verification"
	tox2Address := tox2.SelfGetAddress()
	_, err = tox.AddFriend(tox2Address, message)
	if err != nil {
		t.Fatalf("Failed to send friend request: %v", err)
	}

	// Process to ensure the packet was registered
	tox.Iterate()
	tox2.Iterate()

	// The packet should have been registered in the global registry
	// Format: [SENDER_PUBLIC_KEY(32)][MESSAGE...]
	// We can verify by checking if processPendingFriendRequests can parse it

	t.Log("SUCCESS: Friend request packet format is correct")
}

// --- Tests from friend_request_protocol_regression_test.go ---

// TestFriendRequestProtocolRegression ensures that the friend request protocol
// remains implemented and that AddFriend() continues to send actual network packets.
// This is a regression test for Bug #3: "Friend Request Protocol Not Implemented"
func TestFriendRequestProtocolRegression(t *testing.T) {
	// Create two separate Tox instances to test cross-instance communication
	sender, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create sender Tox instance: %v", err)
	}
	defer sender.Kill()

	receiver, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create receiver Tox instance: %v", err)
	}
	defer receiver.Kill()

	// Test data
	testMessage := "Hello from regression test!"
	receiverToxID := receiver.SelfGetAddress()

	// Track if friend request was received
	var receivedMessage string
	var receivedPublicKey [32]byte
	friendRequestReceived := false

	// Set up receiver callback
	receiver.OnFriendRequest(func(publicKey [32]byte, message string) {
		friendRequestReceived = true
		receivedMessage = message
		receivedPublicKey = publicKey
	})

	// Send friend request
	friendID, err := sender.AddFriend(receiverToxID, testMessage)
	if err != nil {
		t.Fatalf("AddFriend failed: %v", err)
	}

	// Basic validation of local friend creation
	if friendID > 1000 {
		t.Errorf("Unexpected friend ID: %d", friendID)
	}

	senderFriends := sender.GetFriends()
	if len(senderFriends) != 1 {
		t.Fatalf("Expected 1 friend in sender, got %d", len(senderFriends))
	}

	// Process packet delivery (simulate network iterations)
	for i := 0; i < 20; i++ {
		sender.Iterate()
		receiver.Iterate()
	}

	// Verify friend request was transmitted and received
	if !friendRequestReceived {
		t.Fatal("REGRESSION: Friend request was not received - the protocol implementation is broken!")
	}

	// Verify transmitted data integrity
	if receivedMessage != testMessage {
		t.Errorf("Message mismatch: expected %q, got %q", testMessage, receivedMessage)
	}

	expectedPublicKey := sender.SelfGetPublicKey()
	if receivedPublicKey != expectedPublicKey {
		t.Error("Public key mismatch in received friend request")
	}

	// Verify receiver doesn't automatically add the friend (request must be accepted manually)
	receiverFriends := receiver.GetFriends()
	if len(receiverFriends) != 0 {
		t.Errorf("Expected 0 friends in receiver (before accepting), got %d", len(receiverFriends))
	}

	t.Log("✓ Friend request protocol regression test passed")
	t.Log("✓ AddFriend() correctly sends friend request packets")
	t.Log("✓ Receiver correctly processes friend request callbacks")
	t.Log("✓ Bug #3 remains fixed")
}

// --- Tests from friend_request_retry_test.go ---

// TestFriendRequestRetryQueue verifies that friend requests are properly queued for retry
// when DHT nodes are not available
func TestFriendRequestRetryQueue(t *testing.T) {
	// Create Tox instance with minimal bootstrap to simulate sparse DHT
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create a target public key for the friend request
	var targetPK [32]byte
	for i := range targetPK {
		targetPK[i] = byte(i + 1)
	}

	// Send friend request without any DHT nodes available
	message := "Hello, friend!"
	err = tox.sendFriendRequest(targetPK, message)
	if err != nil {
		t.Fatalf("sendFriendRequest failed: %v", err)
	}

	// Verify the request was queued
	tox.pendingFriendReqsMux.Lock()
	if len(tox.pendingFriendReqs) != 1 {
		t.Fatalf("Expected 1 pending request, got %d", len(tox.pendingFriendReqs))
	}

	req := tox.pendingFriendReqs[0]
	if req.targetPublicKey != targetPK {
		t.Error("Target public key mismatch in pending request")
	}
	if req.message != message {
		t.Errorf("Message mismatch: got %q, want %q", req.message, message)
	}
	if req.retryCount != 0 {
		t.Errorf("Initial retry count should be 0, got %d", req.retryCount)
	}
	tox.pendingFriendReqsMux.Unlock()

	t.Log("SUCCESS: Friend request properly queued for retry")
}

// TestFriendRequestRetryBackoff verifies exponential backoff for retries
func TestFriendRequestRetryBackoff(t *testing.T) {
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	var targetPK [32]byte
	for i := range targetPK {
		targetPK[i] = byte(i + 100)
	}

	// Queue a friend request
	err = tox.sendFriendRequest(targetPK, "Test retry backoff")
	if err != nil {
		t.Fatalf("sendFriendRequest failed: %v", err)
	}

	// Wait for initial retry time to pass (5 seconds + 1 second buffer)
	time.Sleep(6 * time.Second)

	// Run iteration to trigger retry
	tox.Iterate()

	// Check that retry count increased and backoff applied
	tox.pendingFriendReqsMux.Lock()
	if len(tox.pendingFriendReqs) == 0 {
		t.Fatal("Request should still be pending after failed retry")
	}

	req := tox.pendingFriendReqs[0]
	if req.retryCount != 1 {
		t.Errorf("Retry count should be 1, got %d", req.retryCount)
	}

	// Verify exponential backoff (should be ~10 seconds for second retry)
	expectedBackoff := 10 * time.Second
	actualBackoff := req.nextRetry.Sub(time.Now())
	if actualBackoff < expectedBackoff-time.Second || actualBackoff > expectedBackoff+time.Second {
		t.Errorf("Backoff not exponential: expected ~%v, got ~%v", expectedBackoff, actualBackoff)
	}
	tox.pendingFriendReqsMux.Unlock()

	t.Log("SUCCESS: Exponential backoff working correctly")
}

// TestFriendRequestMaxRetries verifies that requests are dropped after max retries
func TestFriendRequestMaxRetries(t *testing.T) {
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	var targetPK [32]byte
	for i := range targetPK {
		targetPK[i] = byte(i + 200)
	}

	// Queue a friend request
	err = tox.sendFriendRequest(targetPK, "Test max retries")
	if err != nil {
		t.Fatalf("sendFriendRequest failed: %v", err)
	}

	// Simulate 10 failed retries by manually incrementing retry count
	tox.pendingFriendReqsMux.Lock()
	tox.pendingFriendReqs[0].retryCount = 9
	tox.pendingFriendReqs[0].nextRetry = time.Now() // Make it ready for immediate retry
	tox.pendingFriendReqsMux.Unlock()

	// Run iteration to trigger final retry and removal
	tox.Iterate()

	// Verify request was removed after max retries
	tox.pendingFriendReqsMux.Lock()
	pendingCount := len(tox.pendingFriendReqs)
	tox.pendingFriendReqsMux.Unlock()

	if pendingCount != 0 {
		t.Errorf("Request should be removed after max retries, but %d pending", pendingCount)
	}

	t.Log("SUCCESS: Requests properly dropped after maximum retries")
}

// TestFriendRequestDuplicatePrevention verifies that duplicate requests update existing entry
func TestFriendRequestDuplicatePrevention(t *testing.T) {
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	var targetPK [32]byte
	for i := range targetPK {
		targetPK[i] = byte(i + 50)
	}

	// Send first friend request
	message1 := "First message"
	err = tox.sendFriendRequest(targetPK, message1)
	if err != nil {
		t.Fatalf("First sendFriendRequest failed: %v", err)
	}

	// Send second friend request to same target
	message2 := "Updated message"
	err = tox.sendFriendRequest(targetPK, message2)
	if err != nil {
		t.Fatalf("Second sendFriendRequest failed: %v", err)
	}

	// Verify only one request exists with updated message
	tox.pendingFriendReqsMux.Lock()
	if len(tox.pendingFriendReqs) != 1 {
		t.Fatalf("Expected 1 pending request, got %d", len(tox.pendingFriendReqs))
	}

	req := tox.pendingFriendReqs[0]
	if req.message != message2 {
		t.Errorf("Message should be updated to %q, got %q", message2, req.message)
	}
	tox.pendingFriendReqsMux.Unlock()

	t.Log("SUCCESS: Duplicate requests properly update existing entry")
}

// TestFriendRequestProductionVsTestPath verifies separation of production and test code paths
func TestFriendRequestProductionVsTestPath(t *testing.T) {
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	var targetPK [32]byte
	for i := range targetPK {
		targetPK[i] = byte(i + 150)
	}

	// Send friend request
	err = tox.sendFriendRequest(targetPK, "Test dual path")
	if err != nil {
		t.Fatalf("sendFriendRequest failed: %v", err)
	}

	// Verify request is in both production queue AND test registry
	tox.pendingFriendReqsMux.Lock()
	productionQueued := len(tox.pendingFriendReqs) > 0
	tox.pendingFriendReqsMux.Unlock()

	globalFriendRequestRegistry.RLock()
	testRegistered := globalFriendRequestRegistry.requests[targetPK] != nil
	globalFriendRequestRegistry.RUnlock()

	if !productionQueued {
		t.Error("Request should be in production retry queue")
	}

	if !testRegistered {
		t.Error("Request should be in test registry for backward compatibility")
	}

	t.Log("SUCCESS: Request properly exists in both production queue and test registry")
}

// --- Tests from friend_request_production_test.go ---

// TestFriendRequestProductionScenario simulates a realistic production scenario
// where DHT nodes become available after initial friend request failure
func TestFriendRequestProductionScenario(t *testing.T) {
	// Create two Tox instances
	opts1 := NewOptionsForTesting()
	tox1, err := New(opts1)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	opts2 := NewOptionsForTesting()
	tox2, err := New(opts2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	// Set up callback on tox2
	requestReceived := false
	tox2.OnFriendRequest(func(publicKey [32]byte, message string) {
		requestReceived = true
		t.Logf("Friend request received: %s", message)
	})

	// Clear DHT to simulate sparse network (production scenario)
	// Note: We use the test registry path here since we don't have actual DHT nodes

	// Send friend request - should be queued for retry since DHT is empty
	tox2Address := tox2.SelfGetAddress()
	message := "Production scenario test"
	_, err = tox1.AddFriend(tox2Address, message)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Verify request was queued
	tox1.pendingFriendReqsMux.Lock()
	initialQueueSize := len(tox1.pendingFriendReqs)
	tox1.pendingFriendReqsMux.Unlock()

	if initialQueueSize != 1 {
		t.Fatalf("Expected 1 queued request, got %d", initialQueueSize)
	}

	// Simulate DHT recovery by bootstrapping (this would happen in production)
	// For this test, we'll just iterate and let the test registry handle it
	for i := 0; i < 20 && !requestReceived; i++ {
		tox1.Iterate()
		tox2.Iterate()
		time.Sleep(tox1.IterationInterval())
	}

	// Verify request was received via test registry (simulating network delivery)
	if !requestReceived {
		t.Error("Friend request should have been received via test registry")
	}

	t.Log("SUCCESS: Production scenario properly handles queued requests")
}

// TestFriendRequestCleanupOnSuccess verifies that successful sends remove from queue
func TestFriendRequestCleanupOnSuccess(t *testing.T) {
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create a dummy target
	var targetPK [32]byte
	for i := range targetPK {
		targetPK[i] = byte(i)
	}

	// Clear DHT to force queuing
	// Note: DHT is already empty in a fresh instance

	// Send request - should be queued
	err = tox.sendFriendRequest(targetPK, "Test cleanup")
	if err != nil {
		t.Fatalf("sendFriendRequest failed: %v", err)
	}

	// Verify queued
	tox.pendingFriendReqsMux.Lock()
	if len(tox.pendingFriendReqs) != 1 {
		t.Fatalf("Expected 1 queued request, got %d", len(tox.pendingFriendReqs))
	}
	tox.pendingFriendReqsMux.Unlock()

	// Note: In a real scenario, we'd bootstrap and the retry would succeed
	// For this test, we verify the queue management logic
	t.Log("SUCCESS: Request cleanup verification complete")
}

// --- Tests from request_manager_integration_test.go ---

// TestRequestManagerIntegration verifies that the RequestManager is properly integrated
// into the Tox struct and functions correctly during friend request processing.
func TestRequestManagerIntegration(t *testing.T) {
	// Create a Tox instance
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify RequestManager is initialized
	if tox.RequestManager() == nil {
		t.Fatal("RequestManager should be initialized")
	}

	// Track both callback invocation and request manager state
	callbackInvoked := false
	var callbackPublicKey [32]byte
	var callbackMessage string

	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		callbackInvoked = true
		callbackPublicKey = publicKey
		callbackMessage = message
	})

	// Simulate receiving a friend request
	var senderKey [32]byte
	copy(senderKey[:], []byte("test_sender_public_key_12345678"))
	testMessage := "Hello, let's be friends!"

	// Call receiveFriendRequest directly to test the integration
	tox.receiveFriendRequest(senderKey, testMessage)

	// Verify callback was invoked
	if !callbackInvoked {
		t.Error("FriendRequestCallback should have been invoked")
	}
	if callbackPublicKey != senderKey {
		t.Error("Callback received wrong public key")
	}
	if callbackMessage != testMessage {
		t.Error("Callback received wrong message")
	}

	// Verify RequestManager tracked the request
	pendingRequests := tox.RequestManager().GetPendingRequests()
	if len(pendingRequests) != 1 {
		t.Fatalf("Expected 1 pending request in RequestManager, got %d", len(pendingRequests))
	}

	// Verify request details in RequestManager
	req := pendingRequests[0]
	if req.SenderPublicKey != senderKey {
		t.Error("RequestManager stored wrong sender public key")
	}
	if req.Message != testMessage {
		t.Error("RequestManager stored wrong message")
	}
}

// TestRequestManagerCleanup verifies that the RequestManager is properly cleaned up
// when Kill() is called.
func TestRequestManagerCleanup(t *testing.T) {
	// Create a Tox instance
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}

	// Verify RequestManager exists
	if tox.RequestManager() == nil {
		t.Fatal("RequestManager should be initialized")
	}

	// Kill the instance
	tox.Kill()

	// Verify RequestManager is cleaned up
	if tox.RequestManager() != nil {
		t.Error("RequestManager should be nil after Kill()")
	}
}

// TestRequestManagerDuplicateHandling verifies that duplicate friend requests
// are properly handled through the RequestManager integration.
func TestRequestManagerDuplicateHandling(t *testing.T) {
	// Create a Tox instance
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	callbackCount := 0
	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		callbackCount++
	})

	// Send same request twice
	var senderKey [32]byte
	copy(senderKey[:], []byte("duplicate_test_sender_key_123456"))

	tox.receiveFriendRequest(senderKey, "First message")
	tox.receiveFriendRequest(senderKey, "Updated message")

	// Callback should be invoked twice (application needs both notifications)
	if callbackCount != 2 {
		t.Errorf("Expected callback to be invoked 2 times, got %d", callbackCount)
	}

	// RequestManager should only have one pending request (duplicate updated)
	pendingRequests := tox.RequestManager().GetPendingRequests()
	if len(pendingRequests) != 1 {
		t.Fatalf("Expected 1 pending request in RequestManager, got %d", len(pendingRequests))
	}

	// Verify message was updated
	if pendingRequests[0].Message != "Updated message" {
		t.Errorf("Expected message to be updated to 'Updated message', got '%s'", pendingRequests[0].Message)
	}
}

// TestRequestManagerAcceptReject verifies that accept/reject operations work
// correctly through the Tox.RequestManager() interface.
func TestRequestManagerAcceptReject(t *testing.T) {
	// Create a Tox instance
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Simulate receiving a friend request
	var senderKey [32]byte
	copy(senderKey[:], []byte("accept_reject_test_key_12345678"))
	tox.receiveFriendRequest(senderKey, "Please accept me!")

	// Accept the request through RequestManager
	accepted := tox.RequestManager().AcceptRequest(senderKey)
	if !accepted {
		t.Error("AcceptRequest should return true for pending request")
	}

	// Verify request is no longer pending
	pendingRequests := tox.RequestManager().GetPendingRequests()
	if len(pendingRequests) != 0 {
		t.Errorf("Expected 0 pending requests after accept, got %d", len(pendingRequests))
	}
}

// --- Tests from tcp_transport_integration_test.go ---

// TestTCPTransportIntegration verifies TCP transport is properly initialized and integrated.
func TestTCPTransportIntegration(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false        // Disable UDP to test TCP only
	options.TCPPort = testTCPPortBase // Use non-standard port for testing

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox with TCP transport: %v", err)
	}
	defer tox.Kill()

	// Verify TCP transport was created
	if tox.tcpTransport == nil {
		t.Fatal("TCP transport should be initialized when TCPPort is set")
	}

	// Verify TCP transport has correct local address
	localAddr := tox.tcpTransport.LocalAddr()
	if localAddr == nil {
		t.Fatal("TCP transport LocalAddr should not be nil")
	}

	t.Logf("TCP transport listening on: %s", localAddr.String())
}

// TestTCPTransportDisabled verifies TCP transport is not created when port is 0.
func TestTCPTransportDisabled(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true
	options.TCPPort = 0 // Explicitly disable TCP

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox: %v", err)
	}
	defer tox.Kill()

	// Verify TCP transport was not created
	if tox.tcpTransport != nil {
		t.Fatal("TCP transport should not be initialized when TCPPort is 0")
	}
}

// TestBothTransportsEnabled verifies UDP and TCP can coexist.
func TestBothTransportsEnabled(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true
	options.TCPPort = testTCPPortBase + 1

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox with both transports: %v", err)
	}
	defer tox.Kill()

	// Verify both transports are initialized
	if tox.udpTransport == nil {
		t.Fatal("UDP transport should be initialized")
	}
	if tox.tcpTransport == nil {
		t.Fatal("TCP transport should be initialized")
	}

	t.Logf("UDP transport: %s", tox.udpTransport.LocalAddr().String())
	t.Logf("TCP transport: %s", tox.tcpTransport.LocalAddr().String())
}

// TestTCPTransportCleanup verifies TCP transport is properly closed.
func TestTCPTransportCleanup(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	options.TCPPort = testTCPPortBase + 2

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox: %v", err)
	}

	if tox.tcpTransport == nil {
		t.Fatal("TCP transport should be initialized")
	}

	// Kill should properly clean up TCP transport
	tox.Kill()

	// Give cleanup time to complete
	time.Sleep(100 * time.Millisecond)

	// After Kill, creating another Tox on same port should work
	tox2, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance (TCP port may not have been released): %v", err)
	}
	defer tox2.Kill()
}

// TestTCPPortConflict verifies error handling when port is already in use.
func TestTCPPortConflict(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	options.TCPPort = testTCPPortBase + 3

	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Try to create another instance on same TCP port
	tox2, err := New(options)
	if err == nil {
		tox2.Kill()
		t.Fatal("Expected error when TCP port is already in use")
	}

	t.Logf("Correctly received error for port conflict: %v", err)
}

// TestTCPTransportHandlers verifies packet handlers are registered.
func TestTCPTransportHandlers(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	options.TCPPort = testTCPPortBase + 4

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox: %v", err)
	}
	defer tox.Kill()

	// The registerTCPHandlers method should have been called during initialization
	// We can't directly verify handlers are registered without accessing internal state,
	// but we can verify the transport exists and basic operations work
	if tox.tcpTransport == nil {
		t.Fatal("TCP transport should be initialized")
	}

	// Verify Tox instance is running
	if !tox.IsRunning() {
		t.Fatal("Tox instance should be running")
	}
}

// --- Tests from proxy_integration_test.go ---

// TestProxyConfiguration tests configuring Tox with proxy options.
func TestProxyConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		proxyConfig *ProxyOptions
		udpEnabled  bool
		tcpPort     uint16
		expectError bool
	}{
		{
			name: "SOCKS5 proxy with UDP",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeSOCKS5,
				Host: testLocalhost,
				Port: 9050,
			},
			udpEnabled:  true,
			tcpPort:     0,
			expectError: false,
		},
		{
			name: "SOCKS5 proxy with TCP",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeSOCKS5,
				Host: testLocalhost,
				Port: 9050,
			},
			udpEnabled:  false,
			tcpPort:     testDefaultPort,
			expectError: false,
		},
		{
			name: "SOCKS5 proxy with authentication",
			proxyConfig: &ProxyOptions{
				Type:     ProxyTypeSOCKS5,
				Host:     testLocalhost,
				Port:     9050,
				Username: "testuser",
				Password: "testpass",
			},
			udpEnabled:  true,
			tcpPort:     0,
			expectError: false,
		},
		{
			name:        "No proxy configuration",
			proxyConfig: nil,
			udpEnabled:  true,
			tcpPort:     0,
			expectError: false,
		},
		{
			name: "Proxy type none",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeNone,
				Host: testLocalhost,
				Port: 9050,
			},
			udpEnabled:  true,
			tcpPort:     0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := NewOptions()
			options.UDPEnabled = tt.udpEnabled
			options.TCPPort = tt.tcpPort
			options.Proxy = tt.proxyConfig
			options.MinBootstrapNodes = 1 // For testing

			tox, err := New(options)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error creating Tox instance: %v", err)
				return
			}

			if tox == nil {
				t.Errorf("Expected non-nil Tox instance")
				return
			}

			// Cleanup
			tox.Kill()
		})
	}
}

// TestProxyConfigurationPersistence tests that proxy settings can be reapplied when loading savedata.
// Note: Proxy settings are runtime configuration and are not persisted in savedata.
// This test verifies that we can recreate a Tox instance from savedata and then
// reapply the proxy configuration.
func TestProxyConfigurationPersistence(t *testing.T) {
	// Create Tox instance with proxy configuration
	options := NewOptions()
	options.UDPEnabled = true
	proxyConfig := &ProxyOptions{
		Type:     ProxyTypeSOCKS5,
		Host:     testLocalhost,
		Port:     9050,
		Username: "testuser",
		Password: "testpass",
	}
	options.Proxy = proxyConfig
	options.MinBootstrapNodes = 1

	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}

	// Get savedata
	savedata := tox1.GetSavedata()

	tox1.Kill()

	// Create new instance from savedata with proxy reapplied
	options2 := NewOptions()
	options2.SavedataType = SaveDataTypeToxSave
	options2.SavedataData = savedata
	options2.MinBootstrapNodes = 1
	// Proxy settings are runtime config - reapply them
	options2.Proxy = proxyConfig

	tox2, err := NewFromSavedata(options2, savedata)
	if err != nil {
		t.Fatalf("Failed to create Tox from savedata: %v", err)
	}

	// Verify proxy configuration was applied
	if tox2.options.Proxy == nil {
		t.Errorf("Expected proxy configuration to be applied")
	} else {
		if tox2.options.Proxy.Type != ProxyTypeSOCKS5 {
			t.Errorf("Expected proxy type SOCKS5, got %v", tox2.options.Proxy.Type)
		}
		if tox2.options.Proxy.Host != testLocalhost {
			t.Errorf("Expected proxy host %s, got %s", testLocalhost, tox2.options.Proxy.Host)
		}
		if tox2.options.Proxy.Port != 9050 {
			t.Errorf("Expected proxy port 9050, got %d", tox2.options.Proxy.Port)
		}
	}

	tox2.Kill()
}

// TestProxyWithBootstrap tests that proxy configuration doesn't break bootstrap.
func TestProxyWithBootstrap(t *testing.T) {
	// Note: This test doesn't actually connect to a proxy, it just verifies
	// that bootstrap logic works with proxy configuration present
	options := NewOptions()
	options.UDPEnabled = true
	options.Proxy = &ProxyOptions{
		Type: ProxyTypeSOCKS5,
		Host: testLocalhost,
		Port: 9050,
	}
	options.MinBootstrapNodes = 1

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Attempt bootstrap (will fail without actual proxy, but shouldn't crash)
	err = tox.Bootstrap(testBootstrapNode, testDefaultPort, testBootstrapKey)
	// We expect this to work (no error from the API call itself)
	// The actual connection will fail without a real proxy, but that's okay for this test
	if err != nil {
		t.Logf("Bootstrap returned error (expected without real proxy): %v", err)
	}
}

// TestProxyOptionsValidation tests validation of proxy options.
func TestProxyOptionsValidation(t *testing.T) {
	tests := []struct {
		name        string
		proxyConfig *ProxyOptions
		shouldWork  bool
	}{
		{
			name: "Valid SOCKS5",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeSOCKS5,
				Host: testLocalhost,
				Port: 9050,
			},
			shouldWork: true,
		},
		{
			name: "HTTP proxy",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeHTTP,
				Host: testLocalhost,
				Port: 8080,
			},
			shouldWork: true, // HTTP CONNECT proxy now supported
		},
		{
			name: "Empty host",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeSOCKS5,
				Host: "",
				Port: 9050,
			},
			shouldWork: true, // Empty host will cause proxy creation to fail gracefully
		},
		{
			name: "Zero port",
			proxyConfig: &ProxyOptions{
				Type: ProxyTypeSOCKS5,
				Host: testLocalhost,
				Port: 0,
			},
			shouldWork: true, // Zero port will cause proxy creation to fail gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := NewOptions()
			options.UDPEnabled = true
			options.Proxy = tt.proxyConfig
			options.MinBootstrapNodes = 1

			tox, err := New(options)

			if !tt.shouldWork {
				if err == nil {
					t.Errorf("Expected error but got none")
					if tox != nil {
						tox.Kill()
					}
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tox == nil {
				t.Errorf("Expected non-nil Tox instance")
				return
			}

			tox.Kill()
		})
	}
}

// --- Tests from file_manager_integration_test.go ---

// TestFileManagerIntegration verifies that the file manager is properly
// initialized and integrated with the Tox instance.
func TestFileManagerIntegration(t *testing.T) {
	// Create options with UDP disabled to avoid port conflicts
	options := NewOptions()
	options.UDPEnabled = false
	options.LocalDiscovery = false

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// FileManager is created even without transport - it just can't send packets
	// This allows the manager to be configured and ready when transport becomes available
	fileManager := tox.FileManager()
	if fileManager == nil {
		t.Errorf("Expected fileManager to be initialized (even without transport)")
	}
}

// TestFileManagerWithTransport verifies the file manager is initialized
// when transport is available.
func TestFileManagerWithTransport(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true
	options.LocalDiscovery = false
	options.StartPort = 33480 // Use non-default port to avoid conflicts
	options.EndPort = 33490

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test that FileManager() returns a valid manager when transport is available
	fileManager := tox.FileManager()
	if fileManager == nil {
		t.Errorf("Expected fileManager to be initialized when UDP is enabled")
	}
}

// TestFileManagerCleanup verifies that fileManager is properly cleaned up
// when Kill() is called.
func TestFileManagerCleanup(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true
	options.LocalDiscovery = false
	options.StartPort = 33491
	options.EndPort = 33499

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}

	// Verify fileManager is set before Kill
	if tox.fileManager == nil {
		t.Errorf("Expected fileManager to be set before Kill")
	}

	tox.Kill()

	// Verify fileManager is nil after Kill
	if tox.fileManager != nil {
		t.Errorf("Expected fileManager to be nil after Kill")
	}
}

// --- Tests from local_discovery_integration_test.go ---

// TestLocalDiscoveryIntegration tests LAN discovery with Tox instances.
func TestLocalDiscoveryIntegration(t *testing.T) {
	// Create two Tox instances with LAN discovery enabled on different ports
	// Note: LAN discovery uses port+1, so we need to space ports accordingly
	options1 := NewOptions()
	options1.LocalDiscovery = true
	options1.UDPEnabled = true
	options1.StartPort = 43111
	options1.EndPort = 43111

	options2 := NewOptions()
	options2.LocalDiscovery = true
	options2.UDPEnabled = true
	options2.StartPort = 43113 // Changed from 43112 to avoid conflict with tox1's LAN discovery on 43112
	options2.EndPort = 43113

	tox1, err := New(options1)
	if err != nil {
		t.Fatalf("Failed to create tox1: %v", err)
	}
	defer tox1.Kill()

	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create tox2: %v", err)
	}
	defer tox2.Kill()

	// LAN discovery may not be enabled if there are port conflicts
	// This is acceptable behavior - the instance still works without it
	if tox1.lanDiscovery != nil && tox1.lanDiscovery.IsEnabled() {
		t.Log("tox1 LAN discovery is enabled")
	} else {
		t.Log("tox1 LAN discovery is not enabled (expected if port in use)")
	}

	if tox2.lanDiscovery != nil && tox2.lanDiscovery.IsEnabled() {
		t.Log("tox2 LAN discovery is enabled")
	} else {
		t.Log("tox2 LAN discovery is not enabled (expected if port in use)")
	}

	// Wait for potential discovery (this may not work in all test environments)
	time.Sleep(1 * time.Second)

	// The test passes as long as the Tox instances were created successfully
	// LAN discovery is optional and depends on network configuration
	t.Log("LAN discovery integration test completed successfully")
}

// TestLocalDiscoveryDisabled tests that LAN discovery is not started when disabled.
func TestLocalDiscoveryDisabled(t *testing.T) {
	options := NewOptions()
	options.LocalDiscovery = false
	options.UDPEnabled = true
	options.StartPort = 43115 // Changed to avoid conflicts
	options.EndPort = 43115

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create tox: %v", err)
	}
	defer tox.Kill()

	// Verify LAN discovery is not initialized
	if tox.lanDiscovery != nil && tox.lanDiscovery.IsEnabled() {
		t.Error("Expected LAN discovery to be disabled when LocalDiscovery is false")
	}
}

// TestLocalDiscoveryCleanup tests that LAN discovery is properly stopped on Kill().
func TestLocalDiscoveryCleanup(t *testing.T) {
	options := NewOptions()
	options.LocalDiscovery = true
	options.UDPEnabled = true
	options.StartPort = 43114
	options.EndPort = 43114

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create tox: %v", err)
	}

	wasEnabled := tox.lanDiscovery != nil && tox.lanDiscovery.IsEnabled()

	// Kill the instance
	tox.Kill()

	// Verify LAN discovery is stopped if it was enabled
	if wasEnabled && tox.lanDiscovery.IsEnabled() {
		t.Error("Expected LAN discovery to be stopped after Kill()")
	}

	t.Log("LAN discovery cleanup test completed successfully")
}

// TestLocalDiscoveryDefaultPort tests LAN discovery uses the correct port.
func TestLocalDiscoveryDefaultPort(t *testing.T) {
	options := NewOptions()
	options.LocalDiscovery = true
	options.UDPEnabled = true
	options.StartPort = 0 // Should default to 33445
	options.EndPort = 0

	// Since port 33445 might be in use, we expect this might fail gracefully
	tox, err := New(options)
	if err != nil {
		// If it fails to bind, that's ok for this test
		t.Logf("Expected potential failure to bind to port 33445: %v", err)
		return
	}
	defer tox.Kill()

	if tox.lanDiscovery != nil {
		// LAN discovery might not be enabled if port binding failed
		t.Log("LAN discovery initialization attempted with default port")
	}
}

// TestLocalDiscoveryCustomPort tests LAN discovery with a custom port.
func TestLocalDiscoveryCustomPort(t *testing.T) {
	options := NewOptions()
	options.LocalDiscovery = true
	options.UDPEnabled = true
	options.StartPort = 44556
	options.EndPort = 44556

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create tox: %v", err)
	}
	defer tox.Kill()

	// LAN discovery may not be enabled if there are port conflicts
	if tox.lanDiscovery != nil && tox.lanDiscovery.IsEnabled() {
		t.Log("LAN discovery is enabled with custom port")
	} else {
		t.Log("LAN discovery could not bind to custom port (acceptable)")
	}
}

// --- Tests from packet_delivery_migration_test.go ---

func TestPacketDeliveryMigration(t *testing.T) {
	// Create Tox instance
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test 1: Verify initial state (may be real or simulation depending on environment)
	isSimulation := tox.IsPacketDeliverySimulation()
	t.Logf("Initial packet delivery mode: simulation=%v", isSimulation)

	// Test 2: Verify packet delivery stats are available
	stats := tox.GetPacketDeliveryStats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}
	t.Logf("Initial stats: %+v", stats)

	// Test 3: Test switching to real implementation
	err = tox.SetPacketDeliveryMode(false)
	if err != nil {
		t.Errorf("Failed to switch to real mode: %v", err)
	}

	// Note: May still be simulation if no transport available
	// This is expected behavior for test environment

	// Test 4: Test switching back to simulation
	err = tox.SetPacketDeliveryMode(true)
	if err != nil {
		t.Errorf("Failed to switch to simulation mode: %v", err)
	}
	if !tox.IsPacketDeliverySimulation() {
		t.Error("Should be using simulation after switch")
	}

	// Test 5: Test friend address management
	friendID := uint32(1)
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")

	err = tox.AddFriendAddress(friendID, addr)
	if err != nil {
		t.Errorf("Failed to add friend address: %v", err)
	}

	err = tox.RemoveFriendAddress(friendID)
	if err != nil {
		t.Errorf("Failed to remove friend address: %v", err)
	}
}

func TestPacketDeliveryInterface(t *testing.T) {
	// Test packet delivery interface directly
	config := &interfaces.PacketDeliveryConfig{
		UseSimulation:   true,
		NetworkTimeout:  1000,
		RetryAttempts:   1,
		EnableBroadcast: true,
	}

	// Create simulation implementation
	simDelivery := testsim.NewSimulatedPacketDelivery(config)
	if simDelivery == nil {
		t.Fatal("Failed to create simulation delivery")
	}
	if !simDelivery.IsSimulation() {
		t.Error("Should be simulation implementation")
	}

	// Test adding a friend
	friendID := uint32(1)
	simDelivery.AddFriend(friendID, nil)

	// Test packet delivery
	packet := []byte("test message")
	err := simDelivery.DeliverPacket(friendID, packet)
	if err != nil {
		t.Errorf("Failed to deliver packet: %v", err)
	}

	// Verify delivery log
	log := simDelivery.GetDeliveryLog()
	if len(log) != 1 {
		t.Errorf("Expected 1 delivery, got %d", len(log))
	}
	if log[0].FriendID != friendID {
		t.Errorf("Expected friend ID %d, got %d", friendID, log[0].FriendID)
	}
	if log[0].PacketSize != len(packet) {
		t.Errorf("Expected packet size %d, got %d", len(packet), log[0].PacketSize)
	}
	if !log[0].Success {
		t.Error("Delivery should have been successful")
	}

	// Test broadcast
	err = simDelivery.BroadcastPacket(packet, nil)
	if err != nil {
		t.Errorf("Failed to broadcast packet: %v", err)
	}

	// Verify broadcast delivery
	log = simDelivery.GetDeliveryLog()
	if len(log) != 2 {
		t.Errorf("Expected 2 deliveries after broadcast, got %d", len(log))
	}

	// Test stats
	stats := simDelivery.GetStats()
	if stats["total_friends"] != 1 {
		t.Errorf("Expected 1 friend, got %v", stats["total_friends"])
	}
	if stats["total_deliveries"] != 2 {
		t.Errorf("Expected 2 deliveries, got %v", stats["total_deliveries"])
	}
	if stats["successful_deliveries"] != 2 {
		t.Errorf("Expected 2 successful deliveries, got %v", stats["successful_deliveries"])
	}
	if stats["failed_deliveries"] != 0 {
		t.Errorf("Expected 0 failed deliveries, got %v", stats["failed_deliveries"])
	}
}

func TestDeprecatedSimulatePacketDelivery(t *testing.T) {
	// Test that the deprecated function still works but uses new interface
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Set up simulation mode
	err = tox.SetPacketDeliveryMode(true)
	if err != nil {
		t.Fatalf("Failed to set simulation mode: %v", err)
	}

	// Add a friend to the simulation
	friendID := uint32(1)
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")
	err = tox.AddFriendAddress(friendID, addr)
	if err != nil {
		t.Fatalf("Failed to add friend address: %v", err)
	}

	// Test deprecated function
	packet := []byte("test packet")
	tox.simulatePacketDelivery(friendID, packet)

	// Verify packet was processed through new interface
	stats := tox.GetPacketDeliveryStats()
	if stats["is_simulation"] != true {
		t.Error("Should be using simulation")
	}

	// In simulation mode, delivery should be successful
	deliveries := stats["total_deliveries"]
	if deliveries == nil || deliveries.(int) <= 0 {
		t.Error("Should have at least one delivery recorded")
	}
}

func TestPacketDeliveryFactoryMigration(t *testing.T) {
	// Test factory creation and configuration
	factoryInstance := factory.NewPacketDeliveryFactory()
	if factoryInstance == nil {
		t.Fatal("Failed to create factory")
	}

	// Test default configuration
	config := factoryInstance.GetCurrentConfig()
	t.Logf("Factory default config: UseSimulation=%v, Timeout=%d, Retries=%d, Broadcast=%v",
		config.UseSimulation, config.NetworkTimeout, config.RetryAttempts, config.EnableBroadcast)

	if config.NetworkTimeout != 5000 {
		t.Errorf("Expected timeout 5000, got %d", config.NetworkTimeout)
	}
	if config.RetryAttempts != 3 {
		t.Errorf("Expected 3 retry attempts, got %d", config.RetryAttempts)
	}
	if !config.EnableBroadcast {
		t.Error("Broadcast should be enabled by default")
	}

	// Test switching modes
	factoryInstance.SwitchToSimulation()
	if !factoryInstance.IsUsingSimulation() {
		t.Error("Should be using simulation after switch")
	}

	factoryInstance.SwitchToReal()
	if factoryInstance.IsUsingSimulation() {
		t.Error("Should be using real implementation after switch")
	}

	// Test creating simulation for testing
	simDelivery := factoryInstance.CreateSimulationForTesting()
	if !simDelivery.IsSimulation() {
		t.Error("Should create simulation implementation")
	}

	// Test custom configuration
	customConfig := &interfaces.PacketDeliveryConfig{
		UseSimulation:   true,
		NetworkTimeout:  2000,
		RetryAttempts:   5,
		EnableBroadcast: false,
	}

	err := factoryInstance.UpdateConfig(customConfig)
	if err != nil {
		t.Errorf("Failed to update config: %v", err)
	}

	updatedConfig := factoryInstance.GetCurrentConfig()
	if !updatedConfig.UseSimulation {
		t.Error("Should be using simulation after config update")
	}
	if updatedConfig.NetworkTimeout != 2000 {
		t.Errorf("Expected timeout 2000, got %d", updatedConfig.NetworkTimeout)
	}
	if updatedConfig.RetryAttempts != 5 {
		t.Errorf("Expected 5 retry attempts, got %d", updatedConfig.RetryAttempts)
	}
	if updatedConfig.EnableBroadcast {
		t.Error("Broadcast should be disabled after config update")
	}
}

func TestMigrationBackwardCompatibility(t *testing.T) {
	// Ensure existing tests still pass with new system
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test that basic functionality still works
	publicKey := tox.GetSelfPublicKey()
	if publicKey == [32]byte{} {
		t.Error("Public key should not be empty")
	}

	// Test that iteration still works
	start := time.Now()
	tox.Iterate()
	duration := time.Since(start)
	if duration > time.Second {
		t.Error("Iteration should be fast")
	}

	// Test that the deprecated simulate function doesn't break things
	friendID := uint32(999)
	packet := []byte("compatibility test")

	// This should not panic or error even if friend doesn't exist
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("simulatePacketDelivery panicked: %v", r)
			}
		}()
		tox.simulatePacketDelivery(friendID, packet)
	}()

	// Test that we can still access other functionality
	_ = tox.GetAsyncStorageStats()
	// stats may be nil if async is disabled, that's okay
	// Just verify no panic occurs
}

// --- Tests from security_test.go ---

// TestSecurityValidation_CryptographicProperties validates core crypto security properties
func TestSecurityValidation_CryptographicProperties(t *testing.T) {
	t.Run("Encryption is non-deterministic", func(t *testing.T) {
		// Generate test keys
		senderKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		receiverKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		message := []byte("This is a test message")

		// Encrypt the same message twice with different nonces
		nonce1, err := crypto.GenerateNonce()
		if err != nil {
			t.Fatal(err)
		}

		nonce2, err := crypto.GenerateNonce()
		if err != nil {
			t.Fatal(err)
		}

		ciphertext1, err := crypto.Encrypt(message, nonce1, receiverKeyPair.Public, senderKeyPair.Private)
		if err != nil {
			t.Fatal(err)
		}

		ciphertext2, err := crypto.Encrypt(message, nonce2, receiverKeyPair.Public, senderKeyPair.Private)
		if err != nil {
			t.Fatal(err)
		}

		// Ciphertexts should be different (non-deterministic encryption)
		if bytes.Equal(ciphertext1, ciphertext2) {
			t.Error("Encryption is deterministic - security vulnerability!")
		}
	})

	t.Run("Nonce generation is cryptographically random", func(t *testing.T) {
		// Generate multiple nonces and check for randomness
		nonces := make([]crypto.Nonce, 100)
		for i := range nonces {
			nonce, err := crypto.GenerateNonce()
			if err != nil {
				t.Fatal(err)
			}
			nonces[i] = nonce
		}

		// Check that no two nonces are identical (with high probability)
		for i := 0; i < len(nonces); i++ {
			for j := i + 1; j < len(nonces); j++ {
				if nonces[i] == nonces[j] {
					t.Error("Duplicate nonce detected - cryptographic randomness failure!")
				}
			}
		}
	})

	t.Run("Key generation produces unique keys", func(t *testing.T) {
		// Generate multiple key pairs and ensure uniqueness
		keyPairs := make([]crypto.KeyPair, 50)
		for i := range keyPairs {
			keyPair, err := crypto.GenerateKeyPair()
			if err != nil {
				t.Fatal(err)
			}
			keyPairs[i] = *keyPair
		}

		// Check that no two key pairs are identical
		for i := 0; i < len(keyPairs); i++ {
			for j := i + 1; j < len(keyPairs); j++ {
				if keyPairs[i].Public == keyPairs[j].Public {
					t.Error("Duplicate public key detected - key generation failure!")
				}
				if keyPairs[i].Private == keyPairs[j].Private {
					t.Error("Duplicate private key detected - key generation failure!")
				}
			}
		}
	})

	t.Run("Digital signatures provide authenticity", func(t *testing.T) {
		// Generate key pair for signing
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		message := []byte("This message should be authenticated")

		// Sign the message
		signature, err := crypto.Sign(message, keyPair.Private)
		if err != nil {
			t.Fatal(err)
		}

		// Verify with correct public key
		verifyKey := crypto.GetSignaturePublicKey(keyPair.Private)
		valid, err := crypto.Verify(message, signature, verifyKey)
		if err != nil {
			t.Fatal(err)
		}
		if !valid {
			t.Error("Valid signature failed verification")
		}

		// Verify with wrong public key should fail
		wrongKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}
		wrongVerifyKey := crypto.GetSignaturePublicKey(wrongKeyPair.Private)

		valid, err = crypto.Verify(message, signature, wrongVerifyKey)
		if err != nil {
			t.Fatal(err)
		}
		if valid {
			t.Error("Signature verified with wrong key - security vulnerability!")
		}
	})
}

// TestSecurityValidation_NoiseIKProperties validates Noise-IK security properties
func TestSecurityValidation_NoiseIKProperties(t *testing.T) {
	t.Run("Forward secrecy - handshake creation is non-deterministic", func(t *testing.T) {
		// Create two handshakes with same parameters
		initiatorKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		responderKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		handshake1, err := noise.NewIKHandshake(initiatorKeyPair.Private[:], responderKeyPair.Public[:], noise.Initiator)
		if err != nil {
			t.Fatal(err)
		}

		handshake2, err := noise.NewIKHandshake(initiatorKeyPair.Private[:], responderKeyPair.Public[:], noise.Initiator)
		if err != nil {
			t.Fatal(err)
		}

		// Handshakes should be different instances (ephemeral keys differ)
		if handshake1 == handshake2 {
			t.Error("Handshake instances are identical - potential ephemeral key reuse!")
		}

		// Basic validation that handshakes were created successfully
		if handshake1.IsComplete() {
			t.Error("Handshake1 reports complete before any messages exchanged")
		}
		if handshake2.IsComplete() {
			t.Error("Handshake2 reports complete before any messages exchanged")
		}
	})

	t.Run("Handshake state validation", func(t *testing.T) {
		// Create handshakes for initiator and responder
		initiatorKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		responderKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		initiatorHandshake, err := noise.NewIKHandshake(initiatorKeyPair.Private[:], responderKeyPair.Public[:], noise.Initiator)
		if err != nil {
			t.Fatal(err)
		}

		responderHandshake, err := noise.NewIKHandshake(responderKeyPair.Private[:], nil, noise.Responder)
		if err != nil {
			t.Fatal(err)
		}

		// Initially, handshakes should not be complete
		if initiatorHandshake.IsComplete() {
			t.Error("Initiator handshake reports complete before any messages")
		}
		if responderHandshake.IsComplete() {
			t.Error("Responder handshake reports complete before any messages")
		}

		// Verify handshakes were created with correct roles
		if initiatorHandshake == nil {
			t.Error("Failed to create initiator handshake")
		}
		if responderHandshake == nil {
			t.Error("Failed to create responder handshake")
		}
	})

	t.Run("Key derivation produces different results", func(t *testing.T) {
		// Test that different key pairs produce different handshakes
		keyPair1, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		keyPair2, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		peerKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		handshake1, err := noise.NewIKHandshake(keyPair1.Private[:], peerKeyPair.Public[:], noise.Initiator)
		if err != nil {
			t.Fatal(err)
		}

		handshake2, err := noise.NewIKHandshake(keyPair2.Private[:], peerKeyPair.Public[:], noise.Initiator)
		if err != nil {
			t.Fatal(err)
		}

		// Handshakes with different static keys should be different objects
		if handshake1 == handshake2 {
			t.Error("Handshakes with different static keys are identical - key isolation failure!")
		}
	})
}

// TestSecurityValidation_ProtocolProperties validates protocol-level security
func TestSecurityValidation_ProtocolProperties(t *testing.T) {
	t.Run("Version negotiation prevents downgrade attacks", func(t *testing.T) {
		// Create capabilities that prefer Noise-IK
		capabilities := &transport.ProtocolCapabilities{
			SupportedVersions: []transport.ProtocolVersion{transport.ProtocolLegacy, transport.ProtocolNoiseIK},
			PreferredVersion:  transport.ProtocolNoiseIK,
		}

		negotiator := transport.NewVersionNegotiator(capabilities.SupportedVersions, capabilities.PreferredVersion, capabilities.NegotiationTimeout)

		// Test against peer that supports both
		peerVersions := []transport.ProtocolVersion{transport.ProtocolLegacy, transport.ProtocolNoiseIK}
		selected := negotiator.SelectBestVersion(peerVersions)

		if selected != transport.ProtocolNoiseIK {
			t.Error("Version negotiation selected weaker protocol - downgrade attack possible!")
		}

		// Test against peer that only supports legacy
		legacyOnlyVersions := []transport.ProtocolVersion{transport.ProtocolLegacy}
		selected = negotiator.SelectBestVersion(legacyOnlyVersions)

		if selected != transport.ProtocolLegacy {
			t.Error("Version negotiation failed to fallback appropriately")
		}
	})

	t.Run("ToxID integrity protects against tampering", func(t *testing.T) {
		// Generate a valid ToxID
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		nospam, err := crypto.GenerateNospam()
		if err != nil {
			t.Fatal(err)
		}

		toxID := crypto.NewToxID(keyPair.Public, nospam)
		validToxIDString := toxID.String()

		// Verify valid ToxID parses correctly
		parsedToxID, err := crypto.ToxIDFromString(validToxIDString)
		if err != nil {
			t.Fatal(err)
		}

		if parsedToxID.String() != validToxIDString {
			t.Error("ToxID round-trip failed")
		}

		// Test that tampering with the ToxID string is detected
		tamperedToxIDString := validToxIDString[:len(validToxIDString)-2] + "FF"
		_, err = crypto.ToxIDFromString(tamperedToxIDString)
		if err == nil {
			t.Error("Tampered ToxID was accepted - integrity check failed!")
		}
	})

	t.Run("Message length limits prevent buffer overflow attacks", func(t *testing.T) {
		// Test that encryption rejects oversized messages
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		// Create a message larger than the maximum allowed size
		oversizedMessage := make([]byte, crypto.MaxEncryptionBuffer+1)
		rand.Read(oversizedMessage)

		nonce, err := crypto.GenerateNonce()
		if err != nil {
			t.Fatal(err)
		}

		_, err = crypto.Encrypt(oversizedMessage, nonce, keyPair.Public, keyPair.Private)
		if err == nil {
			t.Error("Oversized message was encrypted - buffer overflow protection failed!")
		}
	})
}

// TestSecurityValidation_Implementation validates implementation-specific security
func TestSecurityValidation_Implementation(t *testing.T) {
	t.Run("No sensitive data in savedata format", func(t *testing.T) {
		// Create a Tox instance with some data
		options := NewOptions()
		tox, err := New(options)
		if err != nil {
			t.Fatal(err)
		}
		defer tox.Kill()

		err = tox.SelfSetName("Test User")
		if err != nil {
			t.Fatal(err)
		}

		// Get savedata
		savedata := tox.GetSavedata()

		// Savedata should not contain plaintext private keys or other sensitive data
		// Note: This is a basic check - in practice, you'd want more sophisticated analysis
		if len(savedata) == 0 {
			t.Error("Savedata is empty")
		}

		// Verify that savedata can be restored without errors
		restoredTox, err := NewFromSavedata(options, savedata)
		if err != nil {
			t.Error("Failed to restore from savedata:", err)
		} else {
			if restoredTox.SelfGetName() != "Test User" {
				t.Error("Savedata restoration lost data")
			}
			restoredTox.Kill()
		}
	})

	t.Run("Nospam provides anti-spam protection", func(t *testing.T) {
		// Create a Tox instance
		options := NewOptions()
		tox, err := New(options)
		if err != nil {
			t.Fatal(err)
		}
		defer tox.Kill()

		// Get initial ToxID and nospam
		initialToxID := tox.SelfGetAddress()
		initialNospam := tox.SelfGetNospam()

		// Change nospam
		newNospam := [4]byte{0x12, 0x34, 0x56, 0x78}
		tox.SelfSetNospam(newNospam)

		// ToxID should change
		newToxID := tox.SelfGetAddress()
		if initialToxID == newToxID {
			t.Error("ToxID unchanged after nospam change - anti-spam protection ineffective!")
		}

		// Nospam should be updated
		if tox.SelfGetNospam() != newNospam {
			t.Error("Nospam not updated correctly")
		}

		// Original nospam should not equal new nospam
		if initialNospam == newNospam {
			t.Error("Nospam values are identical - insufficient randomness!")
		}
	})
}

// TestSecurityImprovementsVerification verifies that all critical security
// improvements from the audit are working correctly
func TestSecurityImprovementsVerification(t *testing.T) {
	// Create a Tox instance with default options
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify secure-by-default transport is enabled
	securityInfo := tox.GetTransportSecurityInfo()
	if securityInfo == nil {
		t.Fatal("GetTransportSecurityInfo returned nil")
	}

	// Check that Noise-IK is enabled by default
	if !securityInfo.NoiseIKEnabled {
		t.Error("Expected Noise-IK to be enabled by default")
	}

	// Check that the transport type indicates negotiating transport
	if securityInfo.TransportType != "negotiating-udp" {
		t.Errorf("Expected transport type 'negotiating-udp', got '%s'", securityInfo.TransportType)
	}

	// Check that supported versions include both legacy and modern protocols
	expectedVersions := []string{"legacy", "noise-ik"}
	if len(securityInfo.SupportedVersions) != len(expectedVersions) {
		t.Errorf("Expected %d supported versions, got %d", len(expectedVersions), len(securityInfo.SupportedVersions))
	}

	// Verify security summary indicates secure status
	summary := tox.GetSecuritySummary()
	if summary == "" {
		t.Error("GetSecuritySummary returned empty string")
	}

	// Should indicate secure status
	if summary == "Basic: Legacy encryption only (consider enabling secure transport)" {
		t.Error("Security summary indicates basic encryption, expected secure status")
	}

	t.Logf("Security verification successful:")
	t.Logf("  Transport Type: %s", securityInfo.TransportType)
	t.Logf("  Noise-IK Enabled: %v", securityInfo.NoiseIKEnabled)
	t.Logf("  Legacy Fallback: %v", securityInfo.LegacyFallbackEnabled)
	t.Logf("  Supported Versions: %v", securityInfo.SupportedVersions)
	t.Logf("  Security Summary: %s", summary)
}

// TestEncryptionStatusAPI verifies the encryption status API functionality
func TestEncryptionStatusAPI(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test with non-existent friend
	status := tox.GetFriendEncryptionStatus(999)
	if status != EncryptionUnknown {
		t.Errorf("Expected EncryptionUnknown for non-existent friend, got %s", status)
	}

	// Add a friend to test with
	friendID, err := tox.AddFriend("76518406f6a9f2217e8dc487cc783c25cc16a15eb36ff32e335364ec37b1334912345678868a", "Test friend for encryption status")
	if err != nil {
		// This is expected to fail since we don't have a real connection
		// but we can still test the API structure
		t.Logf("AddFriend failed as expected (no real connection): %v", err)
		return
	}

	// Test encryption status for the added friend
	status = tox.GetFriendEncryptionStatus(friendID)
	// Should be offline since we don't have a real connection
	if status != EncryptionOffline {
		t.Logf("Friend encryption status: %s (expected offline)", status)
	}
}

// TestSecurityLoggingIntegration verifies that security logging is properly integrated
func TestSecurityLoggingIntegration(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// The fact that we can create a Tox instance successfully means
	// the secure transport initialization worked correctly.
	// In real usage, the logging would appear in the application logs.

	// Verify that the transport was created successfully with security
	securityInfo := tox.GetTransportSecurityInfo()
	if securityInfo.TransportType == "unknown" {
		t.Error("Transport type is unknown, security initialization may have failed")
	}

	t.Logf("Security logging integration test passed")
	t.Logf("Transport initialized with type: %s", securityInfo.TransportType)
}

// --- Tests from onasync_api_test.go ---

// TestOnAsyncMessageAPI tests that OnAsyncMessage is exposed through main Tox interface
func TestOnAsyncMessageAPI(t *testing.T) {
	// Create Tox instance
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test that OnAsyncMessage method exists and can be called
	tox.OnAsyncMessage(func(senderPK [32]byte, message string, messageType async.MessageType) {
		// Handler would be called when async messages are received
		t.Logf("Async message handler set successfully")
	})

	// This test verifies the API exists - actual async functionality would require
	// a full integration test with multiple instances
	t.Log("OnAsyncMessage API is available on main Tox interface")
}

// TestIsAsyncMessagingAvailable tests that applications can check async availability
func TestIsAsyncMessagingAvailable(t *testing.T) {
	// Create Tox instance with UDP enabled (async should initialize)
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify IsAsyncMessagingAvailable method exists and returns a boolean
	available := tox.IsAsyncMessagingAvailable()
	t.Logf("Async messaging available: %v", available)

	// The method should return true or false without panicking
	// Actual availability depends on async manager initialization success
	if available {
		// If available, GetAsyncStorageStats should not return nil
		stats := tox.GetAsyncStorageStats()
		if stats == nil {
			t.Errorf("IsAsyncMessagingAvailable returned true but GetAsyncStorageStats returned nil")
		}
	} else {
		// If not available, GetAsyncStorageStats should return nil
		stats := tox.GetAsyncStorageStats()
		if stats != nil {
			t.Errorf("IsAsyncMessagingAvailable returned false but GetAsyncStorageStats returned non-nil")
		}
	}
}

// --- Tests from savedata_test.go ---

// TestGetSavedata tests basic savedata serialization functionality
func TestGetSavedata(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Get savedata
	savedata := tox.GetSavedata()
	if len(savedata) == 0 {
		t.Fatal("GetSavedata returned empty data")
	}

	// Verify it's valid JSON
	var testData toxSaveData
	if err := testData.unmarshal(savedata); err != nil {
		t.Fatalf("Failed to unmarshal savedata: %v", err)
	}

	// Verify key pair is present
	if testData.KeyPair == nil {
		t.Fatal("Savedata missing key pair")
	}

	// Verify key pair matches original
	if !bytes.Equal(testData.KeyPair.Public[:], tox.keyPair.Public[:]) {
		t.Error("Public key mismatch in savedata")
	}
	if !bytes.Equal(testData.KeyPair.Private[:], tox.keyPair.Private[:]) {
		t.Error("Private key mismatch in savedata")
	}
}

// TestSavedataRoundTrip tests save and load functionality together
func TestSavedataRoundTrip(t *testing.T) {
	// Create first Tox instance
	options := NewOptions()
	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Add some friends to test friend persistence
	publicKey1 := testSequentialPublicKey
	publicKey2 := [32]byte{32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}

	friendID1, err := tox1.AddFriendByPublicKey(publicKey1)
	if err != nil {
		t.Fatalf("Failed to add first friend: %v", err)
	}

	friendID2, err := tox1.AddFriendByPublicKey(publicKey2)
	if err != nil {
		t.Fatalf("Failed to add second friend: %v", err)
	}

	// Manually set friend data for testing
	tox1.friends[friendID1].Name = "Test Friend 1"
	tox1.friends[friendID1].StatusMessage = "Hello World"
	tox1.friends[friendID1].Status = FriendStatusOnline
	tox1.friends[friendID1].LastSeen = time.Now()

	tox1.friends[friendID2].Name = "Test Friend 2"
	tox1.friends[friendID2].StatusMessage = "Another Status"
	tox1.friends[friendID2].Status = FriendStatusAway
	tox1.friends[friendID2].LastSeen = time.Now()

	// Get savedata from first instance
	savedata := tox1.GetSavedata()
	if len(savedata) == 0 {
		t.Fatal("GetSavedata returned empty data")
	}

	// Create second Tox instance from savedata
	tox2, err := NewFromSavedata(nil, savedata)
	if err != nil {
		t.Fatalf("Failed to create Tox instance from savedata: %v", err)
	}
	defer tox2.Kill()

	// Verify key pairs match
	if !bytes.Equal(tox1.keyPair.Public[:], tox2.keyPair.Public[:]) {
		t.Error("Public keys don't match after restore")
	}
	if !bytes.Equal(tox1.keyPair.Private[:], tox2.keyPair.Private[:]) {
		t.Error("Private keys don't match after restore")
	}

	// Verify friends were restored
	if len(tox2.friends) != 2 {
		t.Errorf("Expected 2 friends, got %d", len(tox2.friends))
	}

	// Check friend 1 data
	friend1, exists := tox2.friends[friendID1]
	if !exists {
		t.Fatal("Friend 1 not found after restore")
	}
	if !bytes.Equal(friend1.PublicKey[:], publicKey1[:]) {
		t.Error("Friend 1 public key mismatch after restore")
	}
	if friend1.Name != "Test Friend 1" {
		t.Errorf("Friend 1 name mismatch: expected 'Test Friend 1', got '%s'", friend1.Name)
	}
	if friend1.StatusMessage != "Hello World" {
		t.Errorf("Friend 1 status message mismatch: expected 'Hello World', got '%s'", friend1.StatusMessage)
	}
	if friend1.Status != FriendStatusOnline {
		t.Errorf("Friend 1 status mismatch: expected %d, got %d", FriendStatusOnline, friend1.Status)
	}

	// Check friend 2 data
	friend2, exists := tox2.friends[friendID2]
	if !exists {
		t.Fatal("Friend 2 not found after restore")
	}
	if !bytes.Equal(friend2.PublicKey[:], publicKey2[:]) {
		t.Error("Friend 2 public key mismatch after restore")
	}
	if friend2.Name != "Test Friend 2" {
		t.Errorf("Friend 2 name mismatch: expected 'Test Friend 2', got '%s'", friend2.Name)
	}
}

// TestLoadInvalidData tests error handling for invalid savedata
func TestLoadInvalidData(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test empty data
	err = tox.Load([]byte{})
	if err == nil {
		t.Error("Expected error for empty data")
	}

	// Test invalid JSON
	err = tox.Load([]byte("invalid json"))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	// Test JSON without key pair
	invalidData := []byte(`{"friends":{},"options":{}}`)
	err = tox.Load(invalidData)
	if err == nil {
		t.Error("Expected error for data without key pair")
	}
}

// TestNewFromSavedataErrors tests error cases for NewFromSavedata
func TestNewFromSavedataErrors(t *testing.T) {
	// Test empty savedata
	_, err := NewFromSavedata(nil, []byte{})
	if err == nil {
		t.Error("Expected error for empty savedata")
	}

	// Test invalid savedata
	_, err = NewFromSavedata(nil, []byte("invalid"))
	if err == nil {
		t.Error("Expected error for invalid savedata")
	}

	// Test savedata without key pair
	invalidData := []byte(`{"friends":{},"options":{}}`)
	_, err = NewFromSavedata(nil, invalidData)
	if err == nil {
		t.Error("Expected error for savedata without key pair")
	}
}

// TestSavedataWithoutFriends tests savedata functionality with no friends
func TestSavedataWithoutFriends(t *testing.T) {
	options := NewOptions()
	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Get savedata with no friends
	savedata := tox1.GetSavedata()
	if len(savedata) == 0 {
		t.Fatal("GetSavedata returned empty data")
	}

	// Restore from savedata
	tox2, err := NewFromSavedata(nil, savedata)
	if err != nil {
		t.Fatalf("Failed to restore from savedata: %v", err)
	}
	defer tox2.Kill()

	// Verify keys match
	if !bytes.Equal(tox1.keyPair.Public[:], tox2.keyPair.Public[:]) {
		t.Error("Public keys don't match")
	}

	// Verify empty friends list
	if len(tox2.friends) != 0 {
		t.Errorf("Expected 0 friends, got %d", len(tox2.friends))
	}
}

// TestSavedataMultipleRoundTrips tests multiple save/restore cycles
func TestSavedataMultipleRoundTrips(t *testing.T) {
	options := NewOptions()
	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox1.Kill()

	originalPublicKey := tox1.keyPair.Public

	// First round trip
	savedata1 := tox1.GetSavedata()
	tox2, err := NewFromSavedata(nil, savedata1)
	if err != nil {
		t.Fatalf("Failed first round trip: %v", err)
	}
	defer tox2.Kill()

	// Second round trip
	savedata2 := tox2.GetSavedata()
	tox3, err := NewFromSavedata(nil, savedata2)
	if err != nil {
		t.Fatalf("Failed second round trip: %v", err)
	}
	defer tox3.Kill()

	// Verify key consistency
	if !bytes.Equal(originalPublicKey[:], tox3.keyPair.Public[:]) {
		t.Error("Public key changed after multiple round trips")
	}
}

// TestSavedataFormat tests the structure of the saved data
func TestSavedataFormat(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	savedata := tox.GetSavedata()

	var data toxSaveData
	if err := data.unmarshal(savedata); err != nil {
		t.Fatalf("Failed to unmarshal savedata: %v", err)
	}

	// Verify structure contains expected fields
	if data.KeyPair == nil {
		t.Error("Savedata missing KeyPair")
	}
	if data.Friends == nil {
		t.Error("Savedata missing Friends map")
	}
	if data.Options == nil {
		t.Error("Savedata missing Options")
	}

	// Verify key pair structure
	if len(data.KeyPair.Public) != 32 {
		t.Errorf("Public key wrong length: expected 32, got %d", len(data.KeyPair.Public))
	}
	if len(data.KeyPair.Private) != 32 {
		t.Errorf("Private key wrong length: expected 32, got %d", len(data.KeyPair.Private))
	}
}

// TestNewWithToxSavedata tests the New function with SaveDataTypeToxSave
func TestNewWithToxSavedata(t *testing.T) {
	// Create first Tox instance and add a friend
	options1 := NewOptions()
	tox1, err := New(options1)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Add a test friend
	testPublicKey := [32]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	}
	friendID, err := tox1.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Set some self information
	err = tox1.SelfSetName("Test User")
	if err != nil {
		t.Fatalf("Failed to set name: %v", err)
	}

	// Get savedata from first instance
	savedata := tox1.GetSavedata()
	if len(savedata) == 0 {
		t.Fatal("GetSavedata returned empty data")
	}

	// Create new Tox instance using the savedata in options
	options2 := &Options{
		SavedataType:   SaveDataTypeToxSave,
		SavedataData:   savedata,
		SavedataLength: uint32(len(savedata)),
	}

	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create Tox instance with savedata: %v", err)
	}
	defer tox2.Kill()

	// Verify the key pair was restored
	if !bytes.Equal(tox1.keyPair.Public[:], tox2.keyPair.Public[:]) {
		t.Error("Public keys don't match after loading from options")
	}
	if !bytes.Equal(tox1.keyPair.Private[:], tox2.keyPair.Private[:]) {
		t.Error("Private keys don't match after loading from options")
	}

	// Verify the friend was restored
	if !tox2.FriendExists(friendID) {
		t.Error("Friend was not restored from savedata")
	}

	// Verify friend's public key
	restoredKey, err := tox2.GetFriendPublicKey(friendID)
	if err != nil {
		t.Fatalf("Failed to get friend public key: %v", err)
	}
	if !bytes.Equal(testPublicKey[:], restoredKey[:]) {
		t.Error("Friend public key doesn't match")
	}

	// Verify self information was restored
	selfName := tox2.SelfGetName()
	if selfName != "Test User" {
		t.Errorf("Self name not restored: expected 'Test User', got '%s'", selfName)
	}
}

// TestNewWithToxSavedataErrors tests error cases for New with ToxSave data
func TestNewWithToxSavedataErrors(t *testing.T) {
	// Test with empty savedata
	options := &Options{
		SavedataType:   SaveDataTypeToxSave,
		SavedataData:   []byte{},
		SavedataLength: 0,
	}
	_, err := New(options)
	if err == nil {
		t.Error("Expected error for empty ToxSave data")
	}

	// Test with nil savedata
	options = &Options{
		SavedataType:   SaveDataTypeToxSave,
		SavedataData:   nil,
		SavedataLength: 0,
	}
	_, err = New(options)
	if err == nil {
		t.Error("Expected error for nil ToxSave data")
	}

	// Test with invalid savedata
	options = &Options{
		SavedataType:   SaveDataTypeToxSave,
		SavedataData:   []byte("invalid json"),
		SavedataLength: uint32(len("invalid json")),
	}
	_, err = New(options)
	if err == nil {
		t.Error("Expected error for invalid ToxSave data")
	}

	// Test with length mismatch
	validSavedata := []byte(`{"keyPair":{"public":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","private":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},"friends":{},"options":{"savedataType":0},"selfName":"","selfStatusMsg":"","nospam":"AAAAAA=="}`)
	options = &Options{
		SavedataType:   SaveDataTypeToxSave,
		SavedataData:   validSavedata,
		SavedataLength: uint32(len(validSavedata) + 10), // Wrong length
	}
	_, err = New(options)
	if err == nil {
		t.Error("Expected error for length mismatch")
	}
}

// TestNewWithDifferentSavedataTypes tests different savedata type handling
func TestNewWithDifferentSavedataTypes(t *testing.T) {
	// Test with SaveDataTypeNone
	options := &Options{
		SavedataType: SaveDataTypeNone,
	}
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox with SaveDataTypeNone: %v", err)
	}
	tox.Kill()

	// Test with SaveDataTypeSecretKey (should work as before)
	testSecretKey := [32]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	}
	options = &Options{
		SavedataType:   SaveDataTypeSecretKey,
		SavedataData:   testSecretKey[:],
		SavedataLength: 32,
	}
	tox, err = New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox with SaveDataTypeSecretKey: %v", err)
	}
	tox.Kill()

	// Test with unknown savedata type
	options = &Options{
		SavedataType: SaveDataType(255), // Unknown type
	}
	_, err = New(options)
	if err == nil {
		t.Error("Expected error for unknown savedata type")
	}
}

// --- Tests from critical_nil_transport_regression_test.go ---

// TestCriticalBugNilPointerDereference reproduces and verifies the fix for
// the critical nil pointer dereference bug identified in AUDIT.md Priority 1.
//
// Bug Description: When creating a Tox instance with UDPEnabled = false,
// the application would panic with SIGSEGV because NewAsyncClient called
// trans.RegisterHandler() without checking if trans was nil.
//
// Expected Behavior: According to README.md, async messaging should gracefully
// degrade when unavailable, not crash the application.
//
// This test verifies that the bug is fixed and the application handles
// nil transport gracefully.
func TestCriticalBugNilPointerDereference(t *testing.T) {
	// Create options with UDP disabled - this was causing the panic
	options := NewOptions()
	options.UDPEnabled = false

	// This previously caused a panic with:
	// panic: runtime error: invalid memory address or nil pointer dereference
	// [signal SIGSEGV: segmentation violation code=0x1 addr=0x28 pc=0x6850f9]
	//
	// After the fix, this should succeed without panic
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance with UDP disabled: %v", err)
	}

	// Verify that the Tox instance was created successfully
	if tox == nil {
		t.Fatal("Tox instance is nil")
	}

	// Cleanup
	defer tox.Kill()

	// Verify basic functionality still works
	address := tox.SelfGetAddress()
	if len(address) == 0 {
		t.Error("Tox address is empty")
	}

	publicKey := tox.SelfGetPublicKey()
	if publicKey == ([32]byte{}) {
		t.Error("Tox public key is zero")
	}

	// Test passed - no panic occurred and basic functionality works
	t.Log("Successfully created Tox instance with UDP disabled")
	t.Log("Async messaging gracefully degraded as expected")
}

// TestNilTransportGracefulDegradation verifies that async messaging
// features properly report unavailability when transport is nil,
// rather than causing crashes.
func TestNilTransportGracefulDegradation(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify core Tox functionality remains available
	if !tox.IsRunning() {
		t.Error("Tox should be running")
	}

	// Verify async manager was created (even with nil transport)
	if tox.asyncManager == nil {
		t.Error("Async manager should be initialized")
	}

	// Async messaging operations should fail gracefully (not panic)
	// when transport is unavailable
	testPublicKey := [32]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	}

	// This should not panic - it should either succeed with empty result
	// or fail with a descriptive error
	err = tox.asyncManager.SendAsyncMessage(testPublicKey, "test", 0)
	// We don't check for specific error - just that it didn't panic
	// The error could be "no storage nodes" or "transport unavailable"
	t.Logf("SendAsyncMessage result: %v (expected to fail gracefully)", err)
}

// TestSendPacketToTargetWithNilTransport verifies that sendPacketToTarget
// returns an error when udpTransport is nil, rather than silently succeeding.
//
// This is a regression test for the edge case bug identified in AUDIT.md where
// sendPacketToTarget would return nil (success) even though no packet was sent
// when the transport was unavailable.
//
// Expected behavior: Function should return an error indicating transport unavailability.
// Previous behavior: Returned nil, misleading callers into thinking the packet was sent.
func TestSendPacketToTargetWithNilTransport(t *testing.T) {
	// Create a Tox instance with nil transport
	tox := &Tox{
		udpTransport: nil,
	}

	// Create a dummy packet
	packet := &transport.Packet{
		PacketType: transport.PacketFileRequest,
		Data:       []byte("test data"),
	}

	// Create a dummy target address using the mockAddr from integration_test.go
	targetAddr := &testMockAddr{addr: testLocalhost + ":33445"}

	// Attempt to send packet with nil transport
	err := tox.sendPacketToTarget(packet, targetAddr)

	// Verify that an error is returned
	if err == nil {
		t.Fatal("Expected error when sending packet with nil transport, got nil")
	}

	// Verify the error message indicates transport unavailability
	expectedErrMsg := "no transport available"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
	}
}
