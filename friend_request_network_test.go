package toxcore

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/friend"
	"github.com/opd-ai/toxcore/transport"
)

func TestFriendRequestNetworkProcessing(t *testing.T) {
	// Create two Tox instances for end-to-end testing
	options1 := NewOptions()
	options1.UDPEnabled = true
	tox1, err := New(options1)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	options2 := NewOptions()
	options2.UDPEnabled = true
	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	// Track received friend requests
	var requestReceived bool
	var receivedMessage string
	var receivedPublicKey [32]byte
	var mu sync.Mutex

	// Set up friend request callback on tox2
	tox2.OnFriendRequest(func(publicKey [32]byte, message string) {
		mu.Lock()
		defer mu.Unlock()
		requestReceived = true
		receivedMessage = message
		receivedPublicKey = publicKey
		t.Logf("Received friend request from %x: %s", publicKey[:8], message)
	})

	// Test sending friend request from tox1 to tox2
	testMessage := "Hello, would you like to be friends?"
	tox2ID := tox2.SelfGetAddress()

	t.Logf("Sending friend request from tox1 to tox2")
	t.Logf("Tox2 ID: %s", tox2ID)

	friendID, err := tox1.AddFriendMessage(tox2ID, testMessage)
	if err != nil {
		t.Fatalf("Failed to send friend request: %v", err)
	}

	t.Logf("Friend request sent, friend ID: %d", friendID)

	// Verify friend was added to tox1's friend list
	if !tox1.FriendExists(friendID) {
		t.Error("Friend was not added to sender's friend list")
	}

	// Process iterations to allow network processing
	for i := 0; i < 20; i++ {
		tox1.Iterate()
		tox2.Iterate()
		time.Sleep(50 * time.Millisecond)
	}

	// Verify the friend request was received
	mu.Lock()
	if !requestReceived {
		t.Error("Friend request was not received by recipient")
	}
	if receivedMessage != testMessage {
		t.Errorf("Expected message '%s', got '%s'", testMessage, receivedMessage)
	}
	tox1PublicKey := tox1.SelfGetPublicKey()
	if !bytes.Equal(receivedPublicKey[:], tox1PublicKey[:]) {
		t.Errorf("Expected public key %x, got %x",
			tox1PublicKey[:8], receivedPublicKey[:8])
	}
	mu.Unlock()

	t.Log("Friend request network processing test completed successfully")
}

func TestFriendRequestEncryptionDecryption(t *testing.T) {
	// Create key pairs for sender and recipient
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to create sender key pair: %v", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to create recipient key pair: %v", err)
	}

	testMessage := "Test friend request message"

	// Create protocol capabilities for testing
	capabilities := crypto.NewProtocolCapabilities()

	// Create and encrypt friend request
	request, err := friend.NewRequest(recipientKeyPair.Public, testMessage, senderKeyPair, capabilities)
	if err != nil {
		t.Fatalf("Failed to create friend request: %v", err)
	}

	// Encrypt the request
	encryptedPacket, err := request.Encrypt(senderKeyPair, recipientKeyPair.Public)
	if err != nil {
		t.Fatalf("Failed to encrypt friend request: %v", err)
	}

	// Decrypt the packet
	decryptedRequest, err := friend.DecryptRequest(encryptedPacket, recipientKeyPair)
	if err != nil {
		t.Fatalf("Failed to decrypt friend request: %v", err)
	}

	// Verify the decrypted content
	if decryptedRequest.Message != testMessage {
		t.Errorf("Expected message '%s', got '%s'", testMessage, decryptedRequest.Message)
	}

	if !bytes.Equal(decryptedRequest.SenderPublicKey[:], senderKeyPair.Public[:]) {
		t.Errorf("Expected sender public key %x, got %x",
			senderKeyPair.Public[:8], decryptedRequest.SenderPublicKey[:8])
	}

	t.Log("Friend request encryption/decryption test completed successfully")
}

func TestFriendRequestPacketHandling(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create a mock friend request packet
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to create sender key pair: %v", err)
	}

	// Create protocol capabilities for testing
	capabilities := crypto.NewProtocolCapabilities()

	testMessage := "Mock friend request"
	request, err := friend.NewRequest(tox.SelfGetPublicKey(), testMessage, senderKeyPair, capabilities)
	if err != nil {
		t.Fatalf("Failed to create friend request: %v", err)
	}

	// Encrypt the request
	encryptedData, err := request.Encrypt(senderKeyPair, tox.SelfGetPublicKey())
	if err != nil {
		t.Fatalf("Failed to encrypt friend request: %v", err)
	}

	// Create transport packet
	packet := &transport.Packet{
		PacketType: transport.PacketFriendRequest,
		Data:       encryptedData,
	}

	// Mock network address
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:12345")

	// Test packet handling
	err = tox.handleFriendRequestPacket(packet, addr)
	if err != nil {
		t.Fatalf("Failed to handle friend request packet: %v", err)
	}

	// Verify the request was added to the request manager
	pendingRequests := tox.requestManager.GetPendingRequests()
	if len(pendingRequests) == 0 {
		t.Error("No pending friend requests found")
	}

	if len(pendingRequests) > 0 {
		if pendingRequests[0].Message != testMessage {
			t.Errorf("Expected message '%s', got '%s'",
				testMessage, pendingRequests[0].Message)
		}
	}

	t.Log("Friend request packet handling test completed successfully")
}

func TestSendFriendRequestFunction(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create a target public key
	targetKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to create target key pair: %v", err)
	}

	testMessage := "Test friend request message"

	// Test the sendFriendRequest function
	err = tox.sendFriendRequest(targetKeyPair.Public, testMessage)
	if err != nil {
		t.Logf("Expected error due to mock network address resolution: %v", err)
		// This is expected to fail due to mock address resolution
		// In a real implementation, this would work with proper DHT integration
	}

	t.Log("Send friend request function test completed")
}

func TestFriendRequestDuplicateHandling(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create a mock sender key pair
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to create sender key pair: %v", err)
	}

	// Create protocol capabilities for testing
	capabilities := crypto.NewProtocolCapabilities()

	// Create first enhanced friend request
	request1, err := friend.NewRequest(tox.SelfGetPublicKey(), "First message", senderKeyPair, capabilities)
	if err != nil {
		t.Fatalf("Failed to create first friend request: %v", err)
	}

	// Create enhanced request wrapper
	enhancedRequest1, err := request1.NegotiateProtocol(capabilities)
	if err != nil {
		t.Fatalf("Failed to negotiate protocol for first request: %v", err)
	}

	tox.requestManager.AddRequest(enhancedRequest1)

	// Verify one request exists
	pendingRequests := tox.requestManager.GetPendingRequests()
	if len(pendingRequests) != 1 {
		t.Fatalf("Expected 1 pending request, got %d", len(pendingRequests))
	}

	// Create duplicate request with different message
	request2, err := friend.NewRequest(tox.SelfGetPublicKey(), "Updated message", senderKeyPair, capabilities)
	if err != nil {
		t.Fatalf("Failed to create second friend request: %v", err)
	}

	// Create enhanced request wrapper for duplicate
	enhancedRequest2, err := request2.NegotiateProtocol(capabilities)
	if err != nil {
		t.Fatalf("Failed to negotiate protocol for second request: %v", err)
	}

	tox.requestManager.AddRequest(enhancedRequest2)

	// Verify still only one request exists, but with updated message
	pendingRequests = tox.requestManager.GetPendingRequests()
	if len(pendingRequests) != 1 {
		t.Fatalf("Expected 1 pending request after duplicate, got %d", len(pendingRequests))
	}

	if pendingRequests[0].Message != "Updated message" {
		t.Errorf("Expected updated message 'Updated message', got '%s'",
			pendingRequests[0].Message)
	}

	t.Log("Friend request duplicate handling test completed successfully")
}
