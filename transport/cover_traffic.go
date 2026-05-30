package transport

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	minRandomDummySize  = 32
	maxRandomDummySize  = 256
	randomDummySizeSpan = maxRandomDummySize - minRandomDummySize + 1
)

// CoverTrafficConfig controls dummy-packet injection for a live session.
//
// Zero values are valid: they activate safe, conservative defaults.
type CoverTrafficConfig struct {
	// MinInterval is the shortest random delay between dummy packets.
	// Default: 500ms.
	MinInterval time.Duration
	// MaxInterval is the longest random delay between dummy packets.
	// Default: 2s.
	MaxInterval time.Duration
	// DummyPayloadSize is the encrypted payload length (bytes) for dummy packets.
	// A random size between 32 and 256 bytes is chosen when this is 0, which
	// further obscures traffic patterns.  Non-zero values pin the size.
	DummyPayloadSize int
}

func (c *CoverTrafficConfig) minInterval() time.Duration {
	if c.MinInterval <= 0 {
		return 500 * time.Millisecond
	}
	return c.MinInterval
}

func (c *CoverTrafficConfig) maxInterval() time.Duration {
	if c.MaxInterval <= 0 {
		return 2 * time.Second
	}
	return c.MaxInterval
}

// CoverTrafficManager injects randomly-timed dummy packets into live Noise
// sessions.  Each managed peer gets its own goroutine that wakes on a random
// schedule and transmits a PacketCoverTraffic packet indistinguishable (at
// the wire layer) from a real message.
//
// Recipients that recognise PacketCoverTraffic MUST silently discard it.
// Legacy peers that do not recognise the type will also discard it because
// NoiseTransport ignores packets with unregistered handlers.
//
// Usage:
//
//	ct := transport.NewCoverTrafficManager(nt, cfg)
//	ct.AddPeer(peerAddr)
//	// … later …
//	ct.RemovePeer(peerAddr)
//	ct.Close()
type CoverTrafficManager struct {
	transport Transport
	config    CoverTrafficConfig

	mu    sync.Mutex
	peers map[string]chan struct{} // addr.String() → stop channel
	wg    sync.WaitGroup
}

// NewCoverTrafficManager creates a manager that injects dummy packets through
// the given transport.  cfg controls the injection schedule; a zero-value cfg
// uses conservative defaults.
func NewCoverTrafficManager(t Transport, cfg CoverTrafficConfig) *CoverTrafficManager {
	return &CoverTrafficManager{
		transport: t,
		config:    cfg,
		peers:     make(map[string]chan struct{}),
	}
}

// AddPeer starts cover-traffic injection toward addr.  If addr is already
// managed, AddPeer is a no-op.
func (ct *CoverTrafficManager) AddPeer(addr net.Addr) {
	key := addr.String()

	ct.mu.Lock()
	defer ct.mu.Unlock()

	if _, exists := ct.peers[key]; exists {
		return
	}

	stop := make(chan struct{})
	ct.peers[key] = stop
	ct.wg.Add(1)
	go ct.runPeer(addr, stop)
}

// RemovePeer stops cover-traffic injection toward addr.  It blocks until the
// associated goroutine exits.
func (ct *CoverTrafficManager) RemovePeer(addr net.Addr) {
	key := addr.String()

	ct.mu.Lock()
	stop, exists := ct.peers[key]
	if exists {
		delete(ct.peers, key)
	}
	ct.mu.Unlock()

	if exists {
		close(stop)
	}
}

// Close stops all active cover-traffic goroutines and waits for them to exit.
func (ct *CoverTrafficManager) Close() {
	ct.mu.Lock()
	stops := make([]chan struct{}, 0, len(ct.peers))
	for _, stop := range ct.peers {
		stops = append(stops, stop)
	}
	ct.peers = make(map[string]chan struct{})
	ct.mu.Unlock()

	for _, stop := range stops {
		close(stop)
	}
	ct.wg.Wait()
}

// runPeer is the per-peer goroutine: sleep a random interval then send a dummy.
func (ct *CoverTrafficManager) runPeer(addr net.Addr, stop <-chan struct{}) {
	defer ct.wg.Done()

	for {
		delay, err := ct.randomDelay()
		if err != nil {
			logrus.WithError(err).Warn("CoverTrafficManager: failed to generate random delay; using MinInterval")
			delay = ct.config.minInterval()
		}

		select {
		case <-stop:
			return
		case <-time.After(delay):
		}

		// Check stop again before sending (avoid redundant send after shutdown).
		select {
		case <-stop:
			return
		default:
		}

		if err := ct.sendDummy(addr); err != nil {
			logrus.WithFields(logrus.Fields{
				"peer":  addr.String(),
				"error": err,
			}).Debug("CoverTrafficManager: dummy send failed (session may not be ready)")
		}
	}
}

// randomDelay returns a cryptographically random duration in [min, max].
func (ct *CoverTrafficManager) randomDelay() (time.Duration, error) {
	min := ct.config.minInterval()
	max := ct.config.maxInterval()
	if max <= min {
		return min, nil
	}
	span := int64(max - min)
	n, err := rand.Int(rand.Reader, big.NewInt(span))
	if err != nil {
		return 0, fmt.Errorf("rand.Int: %w", err)
	}
	return min + time.Duration(n.Int64()), nil
}

// sendDummy constructs and transmits a single dummy packet.
func (ct *CoverTrafficManager) sendDummy(addr net.Addr) error {
	payload, err := ct.randomPayload()
	if err != nil {
		return fmt.Errorf("randomPayload: %w", err)
	}

	pkt := &Packet{
		PacketType: PacketCoverTraffic,
		Data:       payload,
	}
	return ct.transport.Send(pkt, addr)
}

// randomPayload returns a random-length random-content byte slice that mimics
// normal message size distribution.
func (ct *CoverTrafficManager) randomPayload() ([]byte, error) {
	size := ct.config.DummyPayloadSize
	if size <= 0 {
		// Pick a random size in [minRandomDummySize, maxRandomDummySize]
		// to vary the traffic fingerprint.
		n, err := rand.Int(rand.Reader, big.NewInt(randomDummySizeSpan))
		if err != nil {
			return nil, fmt.Errorf("rand.Int for size: %w", err)
		}
		size = minRandomDummySize + int(n.Int64())
	}

	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("rand.Read: %w", err)
	}
	return buf, nil
}
