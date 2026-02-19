package file

import (
	"net"
	"time"

	"github.com/opd-ai/toxcore/transport"
)

// mockTimeProvider provides deterministic time for testing.
type mockTimeProvider struct {
	currentTime time.Time
}

func (m *mockTimeProvider) Now() time.Time {
	return m.currentTime
}

func (m *mockTimeProvider) Since(t time.Time) time.Duration {
	return m.currentTime.Sub(t)
}

func (m *mockTimeProvider) advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

func newMockTimeProvider() *mockTimeProvider {
	return &mockTimeProvider{
		currentTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// mockTransport implements transport.Transport for testing.
type mockTransport struct {
	packets []sentPacket
	handler map[transport.PacketType]transport.PacketHandler
}

type sentPacket struct {
	packet *transport.Packet
	addr   net.Addr
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		packets: make([]sentPacket, 0),
		handler: make(map[transport.PacketType]transport.PacketHandler),
	}
}

func (m *mockTransport) Send(packet *transport.Packet, addr net.Addr) error {
	m.packets = append(m.packets, sentPacket{packet: packet, addr: addr})
	return nil
}

func (m *mockTransport) Close() error {
	return nil
}

func (m *mockTransport) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP(testIP), Port: testPort}
}

func (m *mockTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
	m.handler[packetType] = handler
}

func (m *mockTransport) simulateReceive(packetType transport.PacketType, data []byte, addr net.Addr) {
	if handler, exists := m.handler[packetType]; exists {
		packet := &transport.Packet{
			PacketType: packetType,
			Data:       data,
		}
		handler(packet, addr)
	}
}

func (m *mockTransport) getLastPacket() *sentPacket {
	if len(m.packets) == 0 {
		return nil
	}
	return &m.packets[len(m.packets)-1]
}

func (m *mockTransport) clearPackets() {
	m.packets = make([]sentPacket, 0)
}

func (m *mockTransport) IsConnectionOriented() bool {
	return false
}

// mockAddr implements net.Addr for testing.
type mockAddr struct {
	network string
	address string
}

func (m *mockAddr) Network() string {
	return m.network
}

func (m *mockAddr) String() string {
	return m.address
}
