package messaging

import (
	"encoding/base64"
	"encoding/binary"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// transportPacketCapture captures the packet bytes as they would be sent by toxcore.go
type transportPacketCapture struct {
	packets    [][]byte
	shouldFail bool
}

func (t *transportPacketCapture) SendMessagePacket(friendID uint32, message *Message) error {
	if t.shouldFail {
		return NewMessageError("transport failure")
	}

	// Simulate the packet building from toxcore.go:3447-3452
	// Build packet: [TYPE(1)][FRIEND_ID(4)][MESSAGE_TYPE(1)][MESSAGE...]
	packet := make([]byte, 6+len(message.Text))
	packet[0] = 0x01 // Friend message packet type
	binary.BigEndian.PutUint32(packet[1:5], friendID)
	packet[5] = byte(message.Type)
	copy(packet[6:], message.Text)

	t.packets = append(t.packets, packet)
	return nil
}

// TestTransportLayerBase64ByteIntegrity verifies that base64-encoded encrypted messages
// maintain data integrity when copied as bytes to the transport packet.
// This tests the integration between messaging.encryptMessage() and toxcore.SendMessagePacket().
func TestTransportLayerBase64ByteIntegrity(t *testing.T) {
	// Create message manager
	mm := NewMessageManager()

	// Create key provider
	keyProvider := newMockKeyProvider()
	friendKeyPair, _ := crypto.GenerateKeyPair()
	keyProvider.friendPublicKeys[1] = friendKeyPair.Public
	mm.SetKeyProvider(keyProvider)

	// Create packet-capturing transport (simulates toxcore.go behavior)
	transport := &transportPacketCapture{}
	mm.SetTransport(transport)

	// Test message
	originalText := "Hello, this is a test message for transport integration!"

	// Send message
	_, err := mm.SendMessage(1, originalText, MessageTypeNormal)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for async processing
	time.Sleep(100 * time.Millisecond)

	// Verify packet was captured
	if len(transport.packets) == 0 {
		t.Fatal("No packets captured by transport")
	}

	packet := transport.packets[0]

	// Verify packet structure
	if packet[0] != 0x01 {
		t.Errorf("Expected packet type 0x01, got: 0x%02x", packet[0])
	}

	friendID := binary.BigEndian.Uint32(packet[1:5])
	if friendID != 1 {
		t.Errorf("Expected friendID 1, got: %d", friendID)
	}

	messageType := MessageType(packet[5])
	if messageType != MessageTypeNormal {
		t.Errorf("Expected MessageTypeNormal, got: %v", messageType)
	}

	// Extract message bytes from packet
	messageBytes := packet[6:]

	// Verify the message bytes are valid base64
	messageText := string(messageBytes)
	decoded, err := base64.StdEncoding.DecodeString(messageText)
	if err != nil {
		t.Fatalf("Message bytes are not valid base64: %v", err)
	}

	// Verify decoded data is non-empty (contains encrypted content)
	if len(decoded) == 0 {
		t.Error("Decoded message is empty")
	}

	// Verify the encrypted data is padded to expected size
	// Minimum padding size is 256 bytes
	if len(decoded) < 256 {
		t.Errorf("Expected padded message >= 256 bytes, got: %d", len(decoded))
	}
}

// TestTransportLayerPreservesAllBase64Characters verifies that all 64 base64 characters
// plus padding are correctly preserved when copied as bytes to the transport layer.
func TestTransportLayerPreservesAllBase64Characters(t *testing.T) {
	// This test verifies that the byte copy operation in toxcore.go:3452
	// correctly handles all base64 characters including padding

	tests := []struct {
		name string
		text string
	}{
		{
			name: "short message with padding",
			text: "Hi",
		},
		{
			name: "message without padding",
			text: "Hello!",
		},
		{
			name: "message requiring single padding",
			text: "Hello",
		},
		{
			name: "long message",
			text: "This is a longer message that should produce base64 with various characters including potentially + and /",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mm := NewMessageManager()

			keyProvider := newMockKeyProvider()
			friendKeyPair, _ := crypto.GenerateKeyPair()
			keyProvider.friendPublicKeys[1] = friendKeyPair.Public
			mm.SetKeyProvider(keyProvider)

			transport := &transportPacketCapture{}
			mm.SetTransport(transport)

			_, err := mm.SendMessage(1, tt.text, MessageTypeNormal)
			if err != nil {
				t.Fatalf("Failed to send message: %v", err)
			}

			time.Sleep(100 * time.Millisecond)

			if len(transport.packets) == 0 {
				t.Fatal("No packets captured")
			}

			// Extract and verify message bytes
			messageBytes := transport.packets[0][6:]
			messageText := string(messageBytes)

			// Must be valid base64
			_, err = base64.StdEncoding.DecodeString(messageText)
			if err != nil {
				t.Errorf("Transport packet message is not valid base64: %v", err)
			}

			// Verify all characters are valid base64
			for i, c := range messageText {
				isBase64 := (c >= 'A' && c <= 'Z') ||
					(c >= 'a' && c <= 'z') ||
					(c >= '0' && c <= '9') ||
					c == '+' || c == '/' || c == '='
				if !isBase64 {
					t.Errorf("Invalid base64 character at position %d: %q (0x%02x)", i, c, c)
				}
			}
		})
	}
}

// TestTransportLayerActionMessageType verifies that action messages (MessageTypeAction)
// are correctly encoded in the transport packet header.
func TestTransportLayerActionMessageType(t *testing.T) {
	mm := NewMessageManager()

	keyProvider := newMockKeyProvider()
	friendKeyPair, _ := crypto.GenerateKeyPair()
	keyProvider.friendPublicKeys[1] = friendKeyPair.Public
	mm.SetKeyProvider(keyProvider)

	transport := &transportPacketCapture{}
	mm.SetTransport(transport)

	// Send an action message (like /me)
	_, err := mm.SendMessage(1, "does something", MessageTypeAction)
	if err != nil {
		t.Fatalf("Failed to send action message: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if len(transport.packets) == 0 {
		t.Fatal("No packets captured")
	}

	// Verify message type in packet header
	messageType := MessageType(transport.packets[0][5])
	if messageType != MessageTypeAction {
		t.Errorf("Expected MessageTypeAction (%d), got: %d", MessageTypeAction, messageType)
	}
}

// TestTransportLayerUnencryptedMessageIntegrity verifies that unencrypted messages
// (when no key provider is configured) are correctly transmitted through the transport layer.
func TestTransportLayerUnencryptedMessageIntegrity(t *testing.T) {
	mm := NewMessageManager()

	// No key provider - unencrypted mode
	transport := &transportPacketCapture{}
	mm.SetTransport(transport)

	originalText := "Unencrypted test message"

	_, err := mm.SendMessage(1, originalText, MessageTypeNormal)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if len(transport.packets) == 0 {
		t.Fatal("No packets captured")
	}

	// Extract message from packet
	messageBytes := transport.packets[0][6:]
	messageText := string(messageBytes)

	// Unencrypted message should be plaintext
	if messageText != originalText {
		t.Errorf("Expected plaintext %q, got: %q", originalText, messageText)
	}
}

// TestTransportLayerMultipleFriendsPacketRouting verifies that messages to different
// friends produce packets with correct friendID headers.
func TestTransportLayerMultipleFriendsPacketRouting(t *testing.T) {
	mm := NewMessageManager()

	keyProvider := newMockKeyProvider()
	for fid := uint32(1); fid <= 3; fid++ {
		friendKeyPair, _ := crypto.GenerateKeyPair()
		keyProvider.friendPublicKeys[fid] = friendKeyPair.Public
	}
	mm.SetKeyProvider(keyProvider)

	transport := &transportPacketCapture{}
	mm.SetTransport(transport)

	// Send to multiple friends
	for fid := uint32(1); fid <= 3; fid++ {
		_, err := mm.SendMessage(fid, "Hello friend", MessageTypeNormal)
		if err != nil {
			t.Fatalf("Failed to send to friend %d: %v", fid, err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	if len(transport.packets) != 3 {
		t.Fatalf("Expected 3 packets, got: %d", len(transport.packets))
	}

	// Verify each packet has correct friendID
	// Note: packets may arrive in any order due to async processing
	friendIDsSeen := make(map[uint32]bool)
	for _, packet := range transport.packets {
		friendID := binary.BigEndian.Uint32(packet[1:5])
		friendIDsSeen[friendID] = true

		// Verify message content is valid base64
		messageText := string(packet[6:])
		_, err := base64.StdEncoding.DecodeString(messageText)
		if err != nil {
			t.Errorf("Packet for friend %d has invalid base64: %v", friendID, err)
		}
	}

	// Verify all friends received messages
	for fid := uint32(1); fid <= 3; fid++ {
		if !friendIDsSeen[fid] {
			t.Errorf("No packet found for friend %d", fid)
		}
	}
}

// TestTransportLayerBinaryDataPreservation ensures that base64-encoded encrypted
// data containing all possible byte values is correctly preserved through the transport.
func TestTransportLayerBinaryDataPreservation(t *testing.T) {
	mm := NewMessageManager()

	keyProvider := newMockKeyProvider()
	friendKeyPair, _ := crypto.GenerateKeyPair()
	keyProvider.friendPublicKeys[1] = friendKeyPair.Public
	mm.SetKeyProvider(keyProvider)

	transport := &transportPacketCapture{}
	mm.SetTransport(transport)

	// Send a message that produces varied base64 output
	// Due to encryption producing pseudo-random bytes, this will test
	// the full range of base64 characters across multiple messages
	for i := 0; i < 5; i++ {
		_, err := mm.SendMessage(1, "Test message iteration", MessageTypeNormal)
		if err != nil {
			t.Fatalf("Failed to send message iteration %d: %v", i, err)
		}
	}

	time.Sleep(300 * time.Millisecond)

	// Verify all packets have valid base64 content
	for i, packet := range transport.packets {
		messageText := string(packet[6:])
		decoded, err := base64.StdEncoding.DecodeString(messageText)
		if err != nil {
			t.Errorf("Packet %d failed base64 decode: %v", i, err)
		}

		// Re-encode to verify round-trip integrity
		reencoded := base64.StdEncoding.EncodeToString(decoded)
		if reencoded != messageText {
			t.Errorf("Packet %d: round-trip encoding mismatch", i)
		}
	}
}
