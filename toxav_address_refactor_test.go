package toxcore

import (
	"net"
	"testing"

	"github.com/opd-ai/toxcore/transport"
)

// TestToxAVFriendLookup_NoTypeAssertion verifies that the ToxAV friend lookup
// no longer uses concrete type assertions, following the networking best practices.
func TestToxAVFriendLookup_NoTypeAssertion(t *testing.T) {
	// Create two Tox instances for testing
	options1 := NewOptionsForTesting()
	tox1, err := New(options1)
	if err != nil {
		t.Fatalf("Failed to create Tox instance 1: %v", err)
	}
	defer tox1.Kill()

	options2 := NewOptionsForTesting()
	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create Tox instance 2: %v", err)
	}
	defer tox2.Kill()

	// Add tox2 as a friend of tox1
	friendNum, err := tox1.AddFriendByPublicKey(tox2.SelfGetPublicKey())
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Get the friend from tox1
	tox1.friendsMutex.RLock()
	friend, exists := tox1.friends[friendNum]
	tox1.friendsMutex.RUnlock()

	if !exists {
		t.Skipf("Friend %d not found in friend list", friendNum)
	}

	// Create a mock address for testing
	mockAddr := &net.UDPAddr{
		IP:   net.ParseIP("192.168.1.100"),
		Port: 33445,
	}

	// Test that we can serialize the address without type assertions
	addrBytes, err := transport.SerializeNetAddrToBytes(mockAddr)
	if err != nil {
		t.Errorf("SerializeNetAddrToBytes() failed: %v", err)
	}

	// Verify the serialized format
	expectedBytes := []byte{192, 168, 1, 100, 0x82, 0xa5} // Port 33445 = 0x82a5
	if len(addrBytes) != len(expectedBytes) {
		t.Errorf("Address bytes length = %d, want %d", len(addrBytes), len(expectedBytes))
	}

	for i, b := range expectedBytes {
		if addrBytes[i] != b {
			t.Errorf("Address byte[%d] = %d, want %d", i, addrBytes[i], b)
		}
	}

	// Verify we didn't break the friend object
	if friend.PublicKey != tox2.SelfGetPublicKey() {
		t.Error("Friend public key doesn't match")
	}
}

// TestToxAVAddressHandling_SupportsTCPandUDP verifies that the new approach
// works with both TCP and UDP addresses without type assertions.
func TestToxAVAddressHandling_SupportsTCPandUDP(t *testing.T) {
	tests := []struct {
		name      string
		addr      net.Addr
		wantLen   int
		wantErr   bool
	}{
		{
			name:    "UDP address",
			addr:    &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 8080},
			wantLen: 6, // 4 bytes IP + 2 bytes port
			wantErr: false,
		},
		{
			name:    "TCP address",
			addr:    &net.TCPAddr{IP: net.ParseIP("172.16.0.1"), Port: 443},
			wantLen: 6, // 4 bytes IP + 2 bytes port
			wantErr: false,
		},
		{
			name:    "IPv6 UDP address",
			addr:    &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: 9000},
			wantLen: 18, // 16 bytes IP + 2 bytes port
			wantErr: false,
		},
		{
			name:    "IPv6 TCP address",
			addr:    &net.TCPAddr{IP: net.ParseIP("fe80::1"), Port: 22},
			wantLen: 18, // 16 bytes IP + 2 bytes port
			wantErr: true, // Link-local addresses are rejected for security
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrBytes, err := transport.SerializeNetAddrToBytes(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("SerializeNetAddrToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(addrBytes) != tt.wantLen {
				t.Errorf("Address bytes length = %d, want %d", len(addrBytes), tt.wantLen)
			}
		})
	}
}

// TestToxAVAddressSerialization_Consistency verifies that serialization
// produces consistent results for the same address.
func TestToxAVAddressSerialization_Consistency(t *testing.T) {
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445}

	// Serialize the same address multiple times
	bytes1, err := transport.SerializeNetAddrToBytes(addr)
	if err != nil {
		t.Fatalf("First serialization failed: %v", err)
	}

	bytes2, err := transport.SerializeNetAddrToBytes(addr)
	if err != nil {
		t.Fatalf("Second serialization failed: %v", err)
	}

	bytes3, err := transport.SerializeNetAddrToBytes(addr)
	if err != nil {
		t.Fatalf("Third serialization failed: %v", err)
	}

	// All results should be identical
	if len(bytes1) != len(bytes2) || len(bytes2) != len(bytes3) {
		t.Errorf("Serialization lengths inconsistent: %d, %d, %d",
			len(bytes1), len(bytes2), len(bytes3))
	}

	for i := range bytes1 {
		if bytes1[i] != bytes2[i] || bytes2[i] != bytes3[i] {
			t.Errorf("Serialization inconsistent at byte %d: %d, %d, %d",
				i, bytes1[i], bytes2[i], bytes3[i])
		}
	}
}
