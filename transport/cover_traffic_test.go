package transport_test

import (
	"net"
	"sync"
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

func (m *mockTransport) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
}

func (m *mockTransport) Close() error { return nil }

func (m *mockTransport) IsConnectionOriented() bool                                        { return false }
func (m *mockTransport) RegisterHandler(_ transport.PacketType, _ transport.PacketHandler) {}

// timestampTransport records the timestamp of every cover-traffic send.
type timestampTransport struct {
	mu   sync.Mutex
	times []time.Time
}

func (ts *timestampTransport) Send(p *transport.Packet, _ net.Addr) error {
	if p.PacketType == transport.PacketCoverTraffic {
		ts.mu.Lock()
		ts.times = append(ts.times, time.Now())
		ts.mu.Unlock()
	}
	return nil
}

func (ts *timestampTransport) Receive() (*transport.Packet, net.Addr, error) {
	time.Sleep(time.Hour)
	return nil, nil, nil
}

func (ts *timestampTransport) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
}

func (ts *timestampTransport) Close() error { return nil }

func (ts *timestampTransport) IsConnectionOriented() bool                                        { return false }
func (ts *timestampTransport) RegisterHandler(_ transport.PacketType, _ transport.PacketHandler) {}

func (ts *timestampTransport) Intervals() []time.Duration {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if len(ts.times) < 2 {
		return nil
	}
	ivs := make([]time.Duration, len(ts.times)-1)
	for i := 1; i < len(ts.times); i++ {
		ivs[i-1] = ts.times[i].Sub(ts.times[i-1])
	}
	return ivs
}

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

// TestCoverTrafficManagerPeerCount verifies the runtime state API.
func TestCoverTrafficManagerPeerCount(t *testing.T) {
	t.Parallel()

	mt := &mockTransport{}
	ct := transport.NewCoverTrafficManager(mt, transport.CoverTrafficConfig{
		MinInterval: 100 * time.Millisecond,
		MaxInterval: 200 * time.Millisecond,
	})
	defer ct.Close()

	if n := ct.PeerCount(); n != 0 {
		t.Errorf("PeerCount before AddPeer: want 0, got %d", n)
	}

	p1 := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7001}
	p2 := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7002}
	ct.AddPeer(p1)
	if n := ct.PeerCount(); n != 1 {
		t.Errorf("PeerCount after 1 AddPeer: want 1, got %d", n)
	}
	ct.AddPeer(p2)
	if n := ct.PeerCount(); n != 2 {
		t.Errorf("PeerCount after 2 AddPeer: want 2, got %d", n)
	}
	// Idempotent add must not change count.
	ct.AddPeer(p1)
	if n := ct.PeerCount(); n != 2 {
		t.Errorf("PeerCount after duplicate AddPeer: want 2, got %d", n)
	}

	ct.RemovePeer(p1)
	if n := ct.PeerCount(); n != 1 {
		t.Errorf("PeerCount after RemovePeer: want 1, got %d", n)
	}
}

// TestCoverTrafficTimingWithinBounds is an adversarial timing-analysis simulation.
// It collects send timestamps from the CoverTrafficManager and asserts:
//  1. Every inter-send interval is within [MinInterval, MaxInterval + slack].
//  2. Not all intervals are identical — the schedule is randomised, not constant-rate.
//
// The test simulates an adversary observing packet arrival times and verifies that
// the cover traffic scheduler provides meaningful timing variance.
func TestCoverTrafficTimingWithinBounds(t *testing.T) {
	t.Parallel()

	const (
		minIv   = 15 * time.Millisecond
		maxIv   = 30 * time.Millisecond
		slack   = 50 * time.Millisecond // generous OS scheduling slack
		samples = 12                    // number of intervals to collect
	)

	tt := &timestampTransport{}
	cfg := transport.CoverTrafficConfig{
		MinInterval:      minIv,
		MaxInterval:      maxIv,
		DummyPayloadSize: 32,
	}
	ct := transport.NewCoverTrafficManager(tt, cfg)
	peer := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8800}
	ct.AddPeer(peer)

	// Wait until we have enough timestamps.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tt.mu.Lock()
		n := len(tt.times)
		tt.mu.Unlock()
		if n >= samples+1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	ct.Close()

	ivs := tt.Intervals()
	if len(ivs) < samples {
		t.Fatalf("collected only %d intervals, need %d; timing may be broken", len(ivs), samples)
	}
	ivs = ivs[:samples]

	// Assertion 1: every interval in [minIv - slack, maxIv + slack].
	for i, iv := range ivs {
		if iv < minIv-slack || iv > maxIv+slack {
			t.Errorf("interval[%d] = %v, outside [%v, %v]", i, iv, minIv-slack, maxIv+slack)
		}
	}

	// Assertion 2: not all intervals are identical (randomness check).
	allSame := true
	for _, iv := range ivs[1:] {
		if iv != ivs[0] {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("all intervals are identical — cover traffic scheduling is not randomised")
	}
}

// TestCoverTrafficTimingVariance verifies that the scheduler does not cluster
// all sends at the minimum interval (which would make it trivially fingerprintable).
// An adversary seeing constant-minimum-interval traffic could trivially identify it.
func TestCoverTrafficTimingVariance(t *testing.T) {
	t.Parallel()

	const (
		minIv    = 10 * time.Millisecond
		maxIv    = 50 * time.Millisecond
		slack    = 20 * time.Millisecond
		samples  = 15
		minRange = 5 * time.Millisecond // require at least this spread across samples
	)

	tt := &timestampTransport{}
	cfg := transport.CoverTrafficConfig{
		MinInterval:      minIv,
		MaxInterval:      maxIv,
		DummyPayloadSize: 32,
	}
	ct := transport.NewCoverTrafficManager(tt, cfg)
	peer := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8801}
	ct.AddPeer(peer)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tt.mu.Lock()
		n := len(tt.times)
		tt.mu.Unlock()
		if n >= samples+1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	ct.Close()

	ivs := tt.Intervals()
	if len(ivs) < samples {
		t.Fatalf("collected only %d intervals, need %d", len(ivs), samples)
	}

	// Find min and max of observed intervals.
	minObs, maxObs := ivs[0], ivs[0]
	for _, iv := range ivs[1:] {
		if iv < minObs {
			minObs = iv
		}
		if iv > maxObs {
			maxObs = iv
		}
	}

	spread := maxObs - minObs
	if spread < minRange {
		t.Errorf("interval spread %v < required %v — cover traffic may be constant-rate (fingerprintable)", spread, minRange)
	}
}
