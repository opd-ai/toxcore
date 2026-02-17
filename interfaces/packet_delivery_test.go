package interfaces

import (
	"errors"
	"net"
	"testing"
)

// TestPacketDeliveryConfigValidate tests the Validate method of PacketDeliveryConfig.
func TestPacketDeliveryConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  PacketDeliveryConfig
		wantErr error
	}{
		{
			name: "valid config with all fields",
			config: PacketDeliveryConfig{
				UseSimulation:   false,
				NetworkTimeout:  5000,
				RetryAttempts:   3,
				EnableBroadcast: true,
			},
			wantErr: nil,
		},
		{
			name: "valid config with simulation",
			config: PacketDeliveryConfig{
				UseSimulation:   true,
				NetworkTimeout:  1000,
				RetryAttempts:   0,
				EnableBroadcast: false,
			},
			wantErr: nil,
		},
		{
			name: "valid config with zero retries",
			config: PacketDeliveryConfig{
				UseSimulation:   false,
				NetworkTimeout:  100,
				RetryAttempts:   0,
				EnableBroadcast: true,
			},
			wantErr: nil,
		},
		{
			name: "invalid negative timeout",
			config: PacketDeliveryConfig{
				UseSimulation:   false,
				NetworkTimeout:  -1,
				RetryAttempts:   3,
				EnableBroadcast: true,
			},
			wantErr: ErrInvalidTimeout,
		},
		{
			name: "invalid zero timeout",
			config: PacketDeliveryConfig{
				UseSimulation:   false,
				NetworkTimeout:  0,
				RetryAttempts:   3,
				EnableBroadcast: true,
			},
			wantErr: ErrInvalidTimeout,
		},
		{
			name: "invalid negative retry attempts",
			config: PacketDeliveryConfig{
				UseSimulation:   false,
				NetworkTimeout:  5000,
				RetryAttempts:   -1,
				EnableBroadcast: true,
			},
			wantErr: ErrInvalidRetryAttempts,
		},
		{
			name: "valid high timeout value",
			config: PacketDeliveryConfig{
				UseSimulation:   false,
				NetworkTimeout:  60000,
				RetryAttempts:   10,
				EnableBroadcast: true,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// mockPacketDelivery is a minimal implementation for interface compliance testing.
type mockPacketDelivery struct {
	isSimulation bool
}

func (m *mockPacketDelivery) DeliverPacket(friendID uint32, packet []byte) error {
	return nil
}

func (m *mockPacketDelivery) BroadcastPacket(packet []byte, excludeFriends []uint32) error {
	return nil
}

func (m *mockPacketDelivery) SetNetworkTransport(transport INetworkTransport) error {
	return nil
}

func (m *mockPacketDelivery) IsSimulation() bool {
	return m.isSimulation
}

// TestIPacketDeliveryCompliance verifies that mock implements the interface.
func TestIPacketDeliveryCompliance(t *testing.T) {
	var _ IPacketDelivery = (*mockPacketDelivery)(nil)

	mock := &mockPacketDelivery{isSimulation: true}

	if !mock.IsSimulation() {
		t.Error("IsSimulation() should return true for simulation mock")
	}

	err := mock.DeliverPacket(1, []byte("test"))
	if err != nil {
		t.Errorf("DeliverPacket() unexpected error: %v", err)
	}

	err = mock.BroadcastPacket([]byte("test"), nil)
	if err != nil {
		t.Errorf("BroadcastPacket() unexpected error: %v", err)
	}

	err = mock.BroadcastPacket([]byte("test"), []uint32{1, 2, 3})
	if err != nil {
		t.Errorf("BroadcastPacket() with excludes unexpected error: %v", err)
	}
}

// mockNetworkTransport is a minimal implementation for interface compliance testing.
type mockNetworkTransport struct {
	connected   bool
	friends     map[uint32]net.Addr
	sendCalled  bool
	closeCalled bool
}

func newMockNetworkTransport() *mockNetworkTransport {
	return &mockNetworkTransport{
		connected: true,
		friends:   make(map[uint32]net.Addr),
	}
}

func (m *mockNetworkTransport) Send(packet []byte, addr net.Addr) error {
	m.sendCalled = true
	if !m.connected {
		return errors.New("transport not connected")
	}
	return nil
}

func (m *mockNetworkTransport) SendToFriend(friendID uint32, packet []byte) error {
	if !m.connected {
		return errors.New("transport not connected")
	}
	if _, ok := m.friends[friendID]; !ok {
		return errors.New("friend not found")
	}
	return nil
}

func (m *mockNetworkTransport) GetFriendAddress(friendID uint32) (net.Addr, error) {
	addr, ok := m.friends[friendID]
	if !ok {
		return nil, errors.New("friend not found")
	}
	return addr, nil
}

func (m *mockNetworkTransport) RegisterFriend(friendID uint32, addr net.Addr) error {
	if addr == nil {
		return errors.New("address cannot be nil")
	}
	m.friends[friendID] = addr
	return nil
}

func (m *mockNetworkTransport) Close() error {
	m.closeCalled = true
	m.connected = false
	return nil
}

func (m *mockNetworkTransport) IsConnected() bool {
	return m.connected
}

// mockAddr implements net.Addr for testing.
type mockAddr struct {
	network string
	address string
}

func (m *mockAddr) Network() string { return m.network }
func (m *mockAddr) String() string  { return m.address }

// TestINetworkTransportCompliance verifies that mock implements the interface.
func TestINetworkTransportCompliance(t *testing.T) {
	var _ INetworkTransport = (*mockNetworkTransport)(nil)

	transport := newMockNetworkTransport()

	if !transport.IsConnected() {
		t.Error("IsConnected() should return true initially")
	}

	// Test RegisterFriend
	addr := &mockAddr{network: "tox", address: "friend1"}
	err := transport.RegisterFriend(1, addr)
	if err != nil {
		t.Errorf("RegisterFriend() unexpected error: %v", err)
	}

	// Test RegisterFriend with nil address
	err = transport.RegisterFriend(2, nil)
	if err == nil {
		t.Error("RegisterFriend() with nil address should return error")
	}

	// Test GetFriendAddress
	gotAddr, err := transport.GetFriendAddress(1)
	if err != nil {
		t.Errorf("GetFriendAddress() unexpected error: %v", err)
	}
	if gotAddr.String() != addr.String() {
		t.Errorf("GetFriendAddress() = %v, want %v", gotAddr.String(), addr.String())
	}

	// Test GetFriendAddress for non-existent friend
	_, err = transport.GetFriendAddress(999)
	if err == nil {
		t.Error("GetFriendAddress() for non-existent friend should return error")
	}

	// Test Send
	err = transport.Send([]byte("test"), addr)
	if err != nil {
		t.Errorf("Send() unexpected error: %v", err)
	}
	if !transport.sendCalled {
		t.Error("Send() should set sendCalled flag")
	}

	// Test SendToFriend
	err = transport.SendToFriend(1, []byte("test"))
	if err != nil {
		t.Errorf("SendToFriend() unexpected error: %v", err)
	}

	// Test SendToFriend for non-existent friend
	err = transport.SendToFriend(999, []byte("test"))
	if err == nil {
		t.Error("SendToFriend() for non-existent friend should return error")
	}

	// Test Close
	err = transport.Close()
	if err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}
	if !transport.closeCalled {
		t.Error("Close() should set closeCalled flag")
	}
	if transport.IsConnected() {
		t.Error("IsConnected() should return false after Close()")
	}

	// Test operations after close
	err = transport.Send([]byte("test"), addr)
	if err == nil {
		t.Error("Send() after Close() should return error")
	}
}

// TestPacketDeliveryConfigDefaults tests default configuration behavior.
func TestPacketDeliveryConfigDefaults(t *testing.T) {
	// Zero value config should fail validation
	config := PacketDeliveryConfig{}
	err := config.Validate()
	if err == nil {
		t.Error("zero value config should fail validation")
	}
	if !errors.Is(err, ErrInvalidTimeout) {
		t.Errorf("expected ErrInvalidTimeout, got %v", err)
	}
}

// TestPacketDeliveryConfigBoundaries tests boundary values.
func TestPacketDeliveryConfigBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		timeout int
		retries int
		wantErr bool
	}{
		{"min valid timeout", 1, 0, false},
		{"large timeout", 1 << 30, 0, false},
		{"min valid retries", 1, 0, false},
		{"large retries", 1, 1000, false},
		{"boundary timeout zero", 0, 0, true},
		{"boundary retries negative", 1, -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := PacketDeliveryConfig{
				NetworkTimeout: tt.timeout,
				RetryAttempts:  tt.retries,
			}
			err := config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestErrorVariables verifies that error variables are properly defined.
func TestErrorVariables(t *testing.T) {
	if ErrInvalidTimeout == nil {
		t.Error("ErrInvalidTimeout should not be nil")
	}
	if ErrInvalidRetryAttempts == nil {
		t.Error("ErrInvalidRetryAttempts should not be nil")
	}
	if ErrInvalidTimeout.Error() == "" {
		t.Error("ErrInvalidTimeout should have non-empty message")
	}
	if ErrInvalidRetryAttempts.Error() == "" {
		t.Error("ErrInvalidRetryAttempts should have non-empty message")
	}
}

// TestMockAddrInterface verifies mockAddr implements net.Addr.
func TestMockAddrInterface(t *testing.T) {
	var _ net.Addr = (*mockAddr)(nil)

	addr := &mockAddr{network: "udp", address: "127.0.0.1:8080"}
	if addr.Network() != "udp" {
		t.Errorf("Network() = %v, want udp", addr.Network())
	}
	if addr.String() != "127.0.0.1:8080" {
		t.Errorf("String() = %v, want 127.0.0.1:8080", addr.String())
	}
}

// BenchmarkConfigValidate benchmarks the Validate method.
func BenchmarkConfigValidate(b *testing.B) {
	config := PacketDeliveryConfig{
		UseSimulation:   false,
		NetworkTimeout:  5000,
		RetryAttempts:   3,
		EnableBroadcast: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Validate()
	}
}
