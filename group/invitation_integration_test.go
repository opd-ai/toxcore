package group

import (
	"net"
	"testing"

	"github.com/opd-ai/toxcore/transport"
)

// TestInvitationNetworkIntegration verifies that group invitations are sent over the network
func TestInvitationNetworkIntegration(t *testing.T) {
	// Create mock transport and friend resolver
	mockTrans := &mockTransport{}
	mockResolver := newMockFriendResolver()

	// Create a group with transport and friend resolver
	chat, err := Create("Integration Test Group", ChatTypeText, PrivacyPublic, mockTrans, nil)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}
	defer unregisterGroup(chat.ID)

	// Set friend resolver
	chat.SetFriendResolver(mockResolver.resolve)

	// Add a friend to the resolver
	friendID := uint32(42)
	friendAddr := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 100), Port: 33445}
	mockResolver.addFriend(friendID, friendAddr)

	// Invite the friend
	err = chat.InviteFriend(friendID)
	if err != nil {
		t.Fatalf("Failed to invite friend: %v", err)
	}

	// Verify that a packet was sent
	calls := mockTrans.getSendCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 packet to be sent, got %d", len(calls))
	}

	// Verify the packet type
	sentPacket := calls[0].packet
	if sentPacket.PacketType != transport.PacketGroupInvite {
		t.Errorf("Expected PacketGroupInvite, got %v", sentPacket.PacketType)
	}

	// Verify the destination address
	sentAddr := calls[0].addr
	if sentAddr.String() != friendAddr.String() {
		t.Errorf("Expected packet sent to %v, got %v", friendAddr, sentAddr)
	}

	// Verify invitation was recorded locally
	if _, exists := chat.PendingInvitations[friendID]; !exists {
		t.Error("Invitation was not recorded in PendingInvitations")
	}
}

// TestInvitationWithoutResolver verifies that invitations fail gracefully without a resolver
func TestInvitationWithoutResolver(t *testing.T) {
	mockTrans := &mockTransport{}

	chat := &Chat{
		ID:                 1,
		Name:               "Test Group",
		Privacy:            PrivacyPublic,
		Peers:              make(map[uint32]*Peer),
		PendingInvitations: make(map[uint32]*Invitation),
		transport:          mockTrans,
		// No friend resolver set
	}

	friendID := uint32(100)

	err := chat.InviteFriend(friendID)
	if err == nil {
		t.Fatal("Expected error when inviting without friend resolver")
	}

	// Verify no packets were sent
	if len(mockTrans.getSendCalls()) != 0 {
		t.Error("No packets should be sent when friend resolver is missing")
	}

	// Verify invitation was still created (happens before network send)
	if _, exists := chat.PendingInvitations[friendID]; !exists {
		t.Error("Invitation should still be created even if send fails")
	}
}

// TestInvitationWithoutTransport verifies that invitations fail gracefully without transport
func TestInvitationWithoutTransport(t *testing.T) {
	mockResolver := newMockFriendResolver()
	friendID := uint32(100)
	friendAddr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 100), Port: 33445}
	mockResolver.addFriend(friendID, friendAddr)

	chat := &Chat{
		ID:                 1,
		Name:               "Test Group",
		Privacy:            PrivacyPublic,
		Peers:              make(map[uint32]*Peer),
		PendingInvitations: make(map[uint32]*Invitation),
		// No transport set
		friendResolver: mockResolver.resolve,
	}

	err := chat.InviteFriend(friendID)
	if err == nil {
		t.Fatal("Expected error when inviting without transport")
	}

	// Verify invitation was still created (happens before network send)
	if _, exists := chat.PendingInvitations[friendID]; !exists {
		t.Error("Invitation should still be created even if send fails")
	}
}

// TestInvitationPacketStructure verifies the invitation packet contains required fields
func TestInvitationPacketStructure(t *testing.T) {
	mockTrans := &mockTransport{}
	mockResolver := newMockFriendResolver()

	groupName := "Test Group with Long Name"
	chat, err := Create(groupName, ChatTypeText, PrivacyPrivate, mockTrans, nil)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}
	defer unregisterGroup(chat.ID)

	chat.SetFriendResolver(mockResolver.resolve)

	friendID := uint32(123)
	friendAddr := &net.UDPAddr{IP: net.IPv4(172, 16, 0, 1), Port: 33445}
	mockResolver.addFriend(friendID, friendAddr)

	err = chat.InviteFriend(friendID)
	if err != nil {
		t.Fatalf("Failed to invite friend: %v", err)
	}

	calls := mockTrans.getSendCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 packet, got %d", len(calls))
	}

	packet := calls[0].packet
	if len(packet.Data) == 0 {
		t.Fatal("Packet data is empty")
	}

	// Packet should contain: GroupID(4) + GroupName_Length(1) + GroupName + Expires(8) + Privacy(1)
	minExpectedSize := 4 + 1 + 1 + 8 + 1 // At least 1 char in name
	if len(packet.Data) < minExpectedSize {
		t.Errorf("Packet data too small: got %d bytes, expected at least %d", len(packet.Data), minExpectedSize)
	}
}
