package toxcore

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/dht"
	"github.com/opd-ai/toxcore/transport"
)

// TestToxAVPortHandling verifies that ToxAV correctly handles port information
// when sending packets to friends, fixing the hardcoded port 8080 issue.
func TestToxAVPortHandling(t *testing.T) {
	// Create a Tox instance with testing options
	options := NewOptionsForTesting()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create a friend with a known network address
	friendKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate friend keypair: %v", err)
	}

	// Add the friend
	friendID, err := tox.AddFriendByPublicKey(friendKeyPair.Public)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Simulate friend being online at a specific address with non-8080 port
	expectedIP := net.IPv4(192, 168, 1, 100)
	expectedPort := 33445 // Typical Tox port, not 8080
	expectedAddr := &net.UDPAddr{
		IP:   expectedIP,
		Port: expectedPort,
	}

	// Add the friend's address to DHT so resolveFriendAddress can find it
	friendToxID := crypto.ToxID{
		PublicKey: friendKeyPair.Public,
		Nospam:    [4]byte{},
		Checksum:  [2]byte{},
	}

	dhtNode := &dht.Node{
		ID:      friendToxID,
		Address: expectedAddr,
	}

	// Add node to DHT routing table
	tox.dht.AddNode(dhtNode)

	// Set friend as connected
	tox.friendsMutex.Lock()
	if friend, exists := tox.friends[friendID]; exists {
		friend.ConnectionStatus = ConnectionUDP
	}
	tox.friendsMutex.Unlock()

	// Create ToxAV instance
	toxav, err := NewToxAV(tox)
	if err != nil {
		t.Fatalf("Failed to create ToxAV instance: %v", err)
	}
	defer toxav.Kill()

	// Verify ToxAV was created successfully
	// The actual address lookup will be tested when a call is initiated

	// Test the transport adapter's Send method with correct deserialization
	t.Run("TransportAdapterSend", func(t *testing.T) {
		// Create a mock transport to capture sent packets
		sentPackets := []struct {
			packet *transport.Packet
			addr   net.Addr
		}{}

		mockTransport := &mockTransportForPortTest{
			sendFunc: func(p *transport.Packet, a net.Addr) error {
				sentPackets = append(sentPackets, struct {
					packet *transport.Packet
					addr   net.Addr
				}{p, a})
				return nil
			},
		}

		adapter := &toxAVTransportAdapter{
			udpTransport: mockTransport,
		}

		// Prepare address bytes with specific port
		testPort := 12345
		addrBytes := make([]byte, 6)
		addrBytes[0] = 10
		addrBytes[1] = 0
		addrBytes[2] = 0
		addrBytes[3] = 1
		addrBytes[4] = byte(testPort >> 8)
		addrBytes[5] = byte(testPort & 0xFF)

		// Send a test packet
		err := adapter.Send(0x30, []byte("test data"), addrBytes)
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		// Verify packet was sent to correct address
		if len(sentPackets) != 1 {
			t.Fatalf("Expected 1 sent packet, got %d", len(sentPackets))
		}

		sentAddr, ok := sentPackets[0].addr.(*net.UDPAddr)
		if !ok {
			t.Fatalf("Sent address is not *net.UDPAddr: %T", sentPackets[0].addr)
		}

		// Verify IP
		expectedTestIP := net.IPv4(10, 0, 0, 1)
		if !sentAddr.IP.Equal(expectedTestIP) {
			t.Errorf("IP mismatch: expected %s, got %s", expectedTestIP, sentAddr.IP)
		}

		// Verify port (should NOT be 8080)
		if sentAddr.Port == 8080 {
			t.Error("Port is still hardcoded to 8080 - bug not fixed!")
		}
		if sentAddr.Port != testPort {
			t.Errorf("Port mismatch: expected %d, got %d", testPort, sentAddr.Port)
		}
	})
}

// TestAddressSerialization verifies the address serialization/deserialization logic
func TestAddressSerialization(t *testing.T) {
	testCases := []struct {
		name string
		ip   net.IP
		port int
	}{
		{"StandardPort", net.IPv4(192, 168, 1, 1), 33445},
		{"HighPort", net.IPv4(10, 0, 0, 1), 65535},
		{"LowPort", net.IPv4(127, 0, 0, 1), 1024},
		{"AlternatePort", net.IPv4(172, 16, 0, 1), 12345},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Serialize
			addrBytes := make([]byte, 6)
			ip := tc.ip.To4()
			copy(addrBytes[0:4], ip)
			addrBytes[4] = byte(tc.port >> 8)
			addrBytes[5] = byte(tc.port & 0xFF)

			// Deserialize
			deserializedIP := net.IPv4(addrBytes[0], addrBytes[1], addrBytes[2], addrBytes[3])
			deserializedPort := int(addrBytes[4])<<8 | int(addrBytes[5])

			// Verify
			if !deserializedIP.Equal(tc.ip) {
				t.Errorf("IP mismatch: expected %s, got %s", tc.ip, deserializedIP)
			}
			if deserializedPort != tc.port {
				t.Errorf("Port mismatch: expected %d, got %d", tc.port, deserializedPort)
			}
		})
	}
}

// TestPortByteOrder verifies big-endian port encoding
func TestPortByteOrder(t *testing.T) {
	port := 33445 // 0x82A5 in hex

	// Serialize port as big-endian
	highByte := byte(port >> 8)  // Should be 0x82
	lowByte := byte(port & 0xFF) // Should be 0xA5

	// Verify encoding
	if highByte != 0x82 {
		t.Errorf("High byte mismatch: expected 0x82, got 0x%02X", highByte)
	}
	if lowByte != 0xA5 {
		t.Errorf("Low byte mismatch: expected 0xA5, got 0x%02X", lowByte)
	}

	// Deserialize and verify
	deserializedPort := int(highByte)<<8 | int(lowByte)
	if deserializedPort != port {
		t.Errorf("Port mismatch after round-trip: expected %d, got %d", port, deserializedPort)
	}

	// Also test with standard library binary package for comparison
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(port))
	if buf[0] != highByte || buf[1] != lowByte {
		t.Errorf("Manual encoding doesn't match binary.BigEndian: manual=[%02X %02X], lib=[%02X %02X]",
			highByte, lowByte, buf[0], buf[1])
	}
}

// mockTransportForPortTest implements transport.Transport for testing
type mockTransportForPortTest struct {
	sendFunc func(*transport.Packet, net.Addr) error
}

func (m *mockTransportForPortTest) Send(p *transport.Packet, addr net.Addr) error {
	if m.sendFunc != nil {
		return m.sendFunc(p, addr)
	}
	return nil
}

func (m *mockTransportForPortTest) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
	// No-op for testing
}

func (m *mockTransportForPortTest) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 33445}
}

func (m *mockTransportForPortTest) Close() error {
	return nil
}

func (m *mockTransportForPortTest) IsConnectionOriented() bool {
	return false // UDP is connectionless
}
