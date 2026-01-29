package async

import (
	"net"
	"sync"

	"github.com/opd-ai/toxcore/transport"
	"github.com/sirupsen/logrus"
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
	logrus.Warn("SIMULATION FUNCTION - NOT A REAL OPERATION")
	logrus.WithFields(logrus.Fields{
		"function": "NewMockTransport",
		"addr":     addr,
	}).Info("Creating mock transport for testing")

	localAddr := &MockAddr{network: "mock", address: addr}
	transport := &MockTransport{
		packets:   make([]MockPacketSend, 0),
		handlers:  make(map[transport.PacketType]transport.PacketHandler),
		localAddr: localAddr,
		sendFunc:  func(packet *transport.Packet, addr net.Addr) error { return nil },
	}

	logrus.WithFields(logrus.Fields{
		"function":   "NewMockTransport",
		"local_addr": localAddr.String(),
	}).Info("Mock transport created successfully")

	return transport
}

// Send implements Transport.Send
func (m *MockTransport) Send(packet *transport.Packet, addr net.Addr) error {
	logrus.WithFields(logrus.Fields{
		"function":    "MockTransport.Send",
		"packet_type": packet.PacketType,
		"destination": addr.String(),
		"packet_size": len(packet.Data),
	}).Debug("Mock transport sending packet")

	m.mu.Lock()
	defer m.mu.Unlock()
	m.packets = append(m.packets, MockPacketSend{packet: packet, addr: addr})

	logrus.WithFields(logrus.Fields{
		"total_packets_sent": len(m.packets),
	}).Debug("Packet added to mock transport history")

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

// IsConnectionOriented returns false for mock transport (defaults to connectionless).
func (m *MockTransport) IsConnectionOriented() bool {
	return false
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
	logrus.Warn("SIMULATION FUNCTION - NOT A REAL OPERATION")
	logrus.WithFields(logrus.Fields{
		"function":    "SimulateReceive",
		"packet_type": packet.PacketType,
		"source":      addr.String(),
		"packet_size": len(packet.Data),
	}).Info("Simulating packet reception")

	m.mu.Lock()
	handler, exists := m.handlers[packet.PacketType]
	m.mu.Unlock()

	if exists {
		logrus.WithFields(logrus.Fields{
			"packet_type": packet.PacketType,
		}).Debug("Handler found, processing packet")
		err := handler(packet, addr)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"packet_type": packet.PacketType,
				"error":       err.Error(),
			}).Error("Packet handler returned error")
			return err
		}
		logrus.WithFields(logrus.Fields{
			"packet_type": packet.PacketType,
		}).Debug("Packet processed successfully")
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"packet_type": packet.PacketType,
	}).Debug("No handler found for packet type")
	return nil
}
