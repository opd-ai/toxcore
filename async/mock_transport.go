package async

import (
	"net"
	"sync"

	"github.com/opd-ai/toxcore/transport"
)

// MockTransport implements Transport interface for testing
type MockTransport struct {
	packets   []MockPacketSend
	handlers  map[transport.PacketType]transport.PacketHandler
	localAddr net.Addr
	sendFunc  func(packet *transport.Packet, addr net.Addr) error
	mu        sync.Mutex
}

// MockPacketSend represents a packet that was sent via the mock transport
type MockPacketSend struct {
	packet *transport.Packet
	addr   net.Addr
}

// MockAddr implements net.Addr for testing
type MockAddr struct {
	network string
	address string
}

func (m MockAddr) Network() string { return m.network }
func (m MockAddr) String() string  { return m.address }

// NewMockTransport creates a new mock transport for testing
func NewMockTransport(addr string) *MockTransport {
	localAddr := &MockAddr{network: "mock", address: addr}
	return &MockTransport{
		packets:   make([]MockPacketSend, 0),
		handlers:  make(map[transport.PacketType]transport.PacketHandler),
		localAddr: localAddr,
		sendFunc:  func(packet *transport.Packet, addr net.Addr) error { return nil },
	}
}

// Send implements Transport.Send
func (m *MockTransport) Send(packet *transport.Packet, addr net.Addr) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.packets = append(m.packets, MockPacketSend{packet: packet, addr: addr})
	return m.sendFunc(packet, addr)
}

// Close implements Transport.Close
func (m *MockTransport) Close() error {
	return nil
}

// LocalAddr implements Transport.LocalAddr
func (m *MockTransport) LocalAddr() net.Addr {
	return m.localAddr
}

// RegisterHandler implements Transport.RegisterHandler
func (m *MockTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[packetType] = handler
}

// GetPackets returns all packets sent via this transport
func (m *MockTransport) GetPackets() []MockPacketSend {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]MockPacketSend, len(m.packets))
	copy(result, m.packets)
	return result
}

// SetSendFunc allows customizing the send behavior for testing
func (m *MockTransport) SetSendFunc(f func(packet *transport.Packet, addr net.Addr) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendFunc = f
}

// SimulateReceive simulates receiving a packet from the network
func (m *MockTransport) SimulateReceive(packet *transport.Packet, addr net.Addr) error {
	m.mu.Lock()
	handler, exists := m.handlers[packet.PacketType]
	m.mu.Unlock()

	if exists {
		return handler(packet, addr)
	}
	return nil
}
