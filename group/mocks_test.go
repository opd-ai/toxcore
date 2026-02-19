package group

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/toxcore/transport"
)

// sendCall records a single transport.Send invocation for assertion in tests.
type sendCall struct {
	packet *transport.Packet
	addr   net.Addr
}

// mockTransport is a test transport that tracks send calls
type mockTransport struct {
	mu            sync.Mutex
	sendCalls     []sendCall
	shouldFail    bool
	failOnAddress net.Addr
}

func (m *mockTransport) Send(packet *transport.Packet, addr net.Addr) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sendCalls = append(m.sendCalls, sendCall{packet: packet, addr: addr})

	if m.shouldFail {
		return errors.New("mock transport error")
	}

	if m.failOnAddress != nil && addr.String() == m.failOnAddress.String() {
		return errors.New("address-specific failure")
	}

	return nil
}

func (m *mockTransport) Close() error {
	return nil
}

func (m *mockTransport) LocalAddr() net.Addr {
	return &mockAddr{address: "127.0.0.1:33445"}
}

func (m *mockTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
}

func (m *mockTransport) IsConnectionOriented() bool {
	return false // Mock transport defaults to connectionless
}

func (m *mockTransport) getSendCalls() []sendCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]sendCall{}, m.sendCalls...)
}

// mockAddr implements net.Addr for testing
type mockAddr struct {
	address string
}

func (m *mockAddr) Network() string {
	return "udp"
}

func (m *mockAddr) String() string {
	return m.address
}

// mockDelayTransport simulates network latency for performance testing
type mockDelayTransport struct {
	mu        sync.Mutex
	sendCount int
	delay     time.Duration
}

func (m *mockDelayTransport) Send(packet *transport.Packet, addr net.Addr) error {
	m.mu.Lock()
	m.sendCount++
	m.mu.Unlock()

	// Simulate network latency
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return nil
}

func (m *mockDelayTransport) Close() error {
	return nil
}

func (m *mockDelayTransport) LocalAddr() net.Addr {
	return &mockAddr{address: "127.0.0.1:33445"}
}

func (m *mockDelayTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
}

func (m *mockDelayTransport) IsConnectionOriented() bool {
	return false // Mock delay transport defaults to connectionless
}

func (m *mockDelayTransport) getSendCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sendCount
}

// mockTrackedTransport allows tracking concurrent operations
type mockTrackedTransport struct {
	onSend func()
}

func (m *mockTrackedTransport) Send(packet *transport.Packet, addr net.Addr) error {
	if m.onSend != nil {
		m.onSend()
	}
	return nil
}

func (m *mockTrackedTransport) Close() error {
	return nil
}

func (m *mockTrackedTransport) LocalAddr() net.Addr {
	return &mockAddr{address: "127.0.0.1:33445"}
}

func (m *mockTrackedTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
}

func (m *mockTrackedTransport) IsConnectionOriented() bool {
	return false // Mock tracked transport defaults to connectionless
}

// mockFriendResolver is a simple mock that resolves friend addresses for testing
type mockFriendResolver struct {
	addresses map[uint32]net.Addr
}

func newMockFriendResolver() *mockFriendResolver {
	return &mockFriendResolver{
		addresses: make(map[uint32]net.Addr),
	}
}

func (m *mockFriendResolver) addFriend(friendID uint32, addr net.Addr) {
	m.addresses[friendID] = addr
}

func (m *mockFriendResolver) resolve(friendID uint32) (net.Addr, error) {
	addr, ok := m.addresses[friendID]
	if !ok {
		return nil, fmt.Errorf("friend %d not found", friendID)
	}
	return addr, nil
}
