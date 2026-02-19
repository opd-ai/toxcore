package toxcore

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/transport"
)

// ---------------------------------------------------------------------------
// MockTimeProvider is a deterministic time provider for testing.
// ---------------------------------------------------------------------------

// MockTimeProvider allows tests to control time deterministically.
type MockTimeProvider struct {
	currentTime time.Time
}

// Now returns the mock time.
func (m *MockTimeProvider) Now() time.Time {
	return m.currentTime
}

// Advance moves the mock time forward by the given duration.
func (m *MockTimeProvider) Advance(d time.Duration) {
	m.currentTime = m.currentTime.Add(d)
}

// SetTime sets the mock time to a specific value.
func (m *MockTimeProvider) SetTime(t time.Time) {
	m.currentTime = t
}

// ---------------------------------------------------------------------------
// testMockAddr implements net.Addr for testing sendPacketToTarget.
// ---------------------------------------------------------------------------

type testMockAddr struct {
	addr string
}

func (m *testMockAddr) Network() string {
	return "udp"
}

func (m *testMockAddr) String() string {
	return m.addr
}

// ---------------------------------------------------------------------------
// mockAddr is a generic net.Addr implementation for integration tests.
// ---------------------------------------------------------------------------

type mockAddr struct {
	network string
	address string
}

func (m *mockAddr) Network() string { return m.network }
func (m *mockAddr) String() string  { return m.address }

// ---------------------------------------------------------------------------
// mockConnection simulates a network connection for integration tests.
// ---------------------------------------------------------------------------

type mockConnection struct {
	address      transport.NetworkAddress
	capabilities transport.NetworkCapabilities
	ctx          context.Context
}

func (m *mockConnection) Connect() error {
	switch m.address.Type {
	case transport.AddressTypeIPv4, transport.AddressTypeIPv6:
		return nil
	case transport.AddressTypeOnion, transport.AddressTypeI2P, transport.AddressTypeNym:
		return nil
	default:
		return fmt.Errorf("Unknown address type: %v", m.address.Type)
	}
}

// ---------------------------------------------------------------------------
// mockUDPTransport implements transport.Transport for frame transport tests.
// ---------------------------------------------------------------------------

type mockUDPTransport struct {
	mu          sync.Mutex
	sentPackets []sentPacket
	handlers    map[transport.PacketType]transport.PacketHandler
}

type sentPacket struct {
	packet *transport.Packet
	addr   net.Addr
}

func newMockUDPTransport() *mockUDPTransport {
	return &mockUDPTransport{
		sentPackets: make([]sentPacket, 0),
		handlers:    make(map[transport.PacketType]transport.PacketHandler),
	}
}

func (m *mockUDPTransport) Send(packet *transport.Packet, addr net.Addr) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sentPackets = append(m.sentPackets, sentPacket{
		packet: packet,
		addr:   addr,
	})
	return nil
}

func (m *mockUDPTransport) RegisterHandler(packetType transport.PacketType, handler transport.PacketHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlers[packetType] = handler
}

func (m *mockUDPTransport) Close() error {
	return nil
}

func (m *mockUDPTransport) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: testDefaultPort}
}

func (m *mockUDPTransport) IsConnectionOriented() bool {
	return false
}

// ---------------------------------------------------------------------------
// mockTransportForPortTest implements transport.Transport for port tests.
// ---------------------------------------------------------------------------

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
}

func (m *mockTransportForPortTest) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: testDefaultPort}
}

func (m *mockTransportForPortTest) Close() error {
	return nil
}

func (m *mockTransportForPortTest) IsConnectionOriented() bool {
	return false
}

// ---------------------------------------------------------------------------
// mockAVTransport is a simple mock for testing av.Manager callbacks.
// ---------------------------------------------------------------------------

type mockAVTransport struct {
	packets map[byte][]func(data, addr []byte) error
	mu      sync.RWMutex
}

func (m *mockAVTransport) Send(packetType byte, data, addr []byte) error {
	return nil
}

func (m *mockAVTransport) RegisterHandler(packetType byte, handler func(data, addr []byte) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.packets == nil {
		m.packets = make(map[byte][]func(data, addr []byte) error)
	}
	m.packets[packetType] = append(m.packets[packetType], handler)
}

// ---------------------------------------------------------------------------
// Integration test helper functions (from integration_test.go).
// ---------------------------------------------------------------------------

func checkNetworkCompatibility(source, target string) bool {
	if getNetworkType(source) == getNetworkType(target) {
		return true
	}
	if isIPNetwork(source) && isIPNetwork(target) {
		return true
	}
	if strings.Contains(source, ".onion") && isIPNetwork(target) {
		return true
	}
	if isIPNetwork(source) && strings.Contains(target, ".onion") {
		return true
	}
	if strings.Contains(source, ".i2p") || strings.Contains(target, ".i2p") {
		return strings.Contains(source, ".i2p") && strings.Contains(target, ".i2p")
	}
	if strings.Contains(source, ".nym") || strings.Contains(target, ".nym") {
		return true
	}
	return false
}

func isLegacyAddress(address string) bool {
	return isIPNetwork(address)
}

func isIPNetwork(address string) bool {
	return !strings.Contains(address, ".onion") &&
		!strings.Contains(address, ".i2p") &&
		!strings.Contains(address, ".nym") &&
		!strings.Contains(address, ".loki")
}

func getNetworkType(address string) string {
	if strings.Contains(address, ".onion") {
		return "onion"
	}
	if strings.Contains(address, ".i2p") {
		return "i2p"
	}
	if strings.Contains(address, ".nym") {
		return "nym"
	}
	if strings.Contains(address, ".loki") {
		return "loki"
	}
	return "ip"
}

// createTestToxInstance is a shared helper that creates a Tox instance with
// sensible defaults and a unique name for use in integration tests.
func createTestToxInstance(t *testing.T, name string) (*Tox, error) {
	t.Helper()
	options := NewOptions()
	options.UDPEnabled = true
	options.StartPort = testDefaultPort
	options.EndPort = testDefaultPort + 100

	tox, err := New(options)
	if err != nil {
		return nil, err
	}

	// Set a unique name for identification
	err = tox.SelfSetName(name)
	if err != nil {
		tox.Kill()
		return nil, err
	}

	return tox, nil
}
