package transport_test

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/transport"
)

// mockTransport is a minimal Transport that counts packets of each type.
type mockTransport struct {
	sent atomic.Int64
}

func (m *mockTransport) Send(p *transport.Packet, _ net.Addr) error {
	if p.PacketType == transport.PacketCoverTraffic {
		m.sent.Add(1)
	}
	return nil
}

func (m *mockTransport) Receive() (*transport.Packet, net.Addr, error) {
	time.Sleep(time.Hour) // block forever
	return nil, nil, nil
}

func (m *mockTransport) LocalAddr() net.Addr { return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0} }
func (m *mockTransport) Close() error        { return nil }
func (m *mockTransport) IsConnectionOriented() bool { return false }
func (m *mockTransport) RegisterHandler(_ transport.PacketType, _ transport.PacketHandler) {}

func TestCoverTrafficManagerSendsDummyPackets(t *testing.T) {
	t.Parallel()

	mt := &mockTransport{}
	cfg := transport.CoverTrafficConfig{
		MinInterval:      10 * time.Millisecond,
		MaxInterval:      20 * time.Millisecond,
		DummyPayloadSize: 64,
	}

	ct := transport.NewCoverTrafficManager(mt, cfg)
	peer := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9999}
	ct.AddPeer(peer)

	// Wait for at least 3 dummy packets.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mt.sent.Load() >= 3 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	ct.Close()

	if got := mt.sent.Load(); got < 3 {
		t.Errorf("expected ≥3 dummy packets, got %d", got)
	}
}

func TestCoverTrafficManagerAddPeerIdempotent(t *testing.T) {
	t.Parallel()

	mt := &mockTransport{}
	ct := transport.NewCoverTrafficManager(mt, transport.CoverTrafficConfig{
		MinInterval: 10 * time.Millisecond,
		MaxInterval: 20 * time.Millisecond,
	})
	peer := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9998}
	ct.AddPeer(peer)
	ct.AddPeer(peer) // second call must not panic or spin up a second goroutine
	ct.Close()
}

func TestCoverTrafficManagerRemovePeer(t *testing.T) {
	t.Parallel()

	mt := &mockTransport{}
	ct := transport.NewCoverTrafficManager(mt, transport.CoverTrafficConfig{
		MinInterval: 100 * time.Millisecond,
		MaxInterval: 200 * time.Millisecond,
	})
	peer := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9997}
	ct.AddPeer(peer)
	ct.RemovePeer(peer) // must not block indefinitely
	ct.Close()
}

func TestCoverTrafficManagerDefaultPayloadSize(t *testing.T) {
	t.Parallel()

	mt := &mockTransport{}
	cfg := transport.CoverTrafficConfig{
		MinInterval: 10 * time.Millisecond,
		MaxInterval: 20 * time.Millisecond,
		// DummyPayloadSize left as 0 → random 32-256 bytes
	}

	ct := transport.NewCoverTrafficManager(mt, cfg)
	peer := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9996}
	ct.AddPeer(peer)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mt.sent.Load() >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	ct.Close()

	if got := mt.sent.Load(); got < 2 {
		t.Errorf("expected ≥2 dummy packets with random payload size, got %d", got)
	}
}
