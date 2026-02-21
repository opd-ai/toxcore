package group

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/dht"
	"github.com/sirupsen/logrus"
)

// TestWithTimeout tests the WithTimeout functional option.
func TestWithTimeout(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected time.Duration
	}{
		{"positive duration", 5 * time.Second, 5 * time.Second},
		{"one minute", time.Minute, time.Minute},
		{"zero duration ignored", 0, 30 * time.Second},                    // default
		{"negative duration ignored", -1 * time.Second, 30 * time.Second}, // default
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultBroadcastConfig()
			WithTimeout(tc.duration)(cfg)
			if cfg.Timeout != tc.expected {
				t.Errorf("expected timeout %v, got %v", tc.expected, cfg.Timeout)
			}
		})
	}
}

// TestWithMaxWorkers tests the WithMaxWorkers functional option.
func TestWithMaxWorkers(t *testing.T) {
	tests := []struct {
		name     string
		workers  int
		expected int
	}{
		{"valid workers", 5, 5},
		{"minimum workers", 1, 1},
		{"zero ignored", 0, 10},      // default
		{"negative ignored", -5, 10}, // default
		{"capped at 100", 200, 100},
		{"exactly 100", 100, 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultBroadcastConfig()
			WithMaxWorkers(tc.workers)(cfg)
			if cfg.MaxWorkers != tc.expected {
				t.Errorf("expected max workers %d, got %d", tc.expected, cfg.MaxWorkers)
			}
		})
	}
}

// TestWithLogger tests the WithLogger functional option.
func TestWithLogger(t *testing.T) {
	t.Run("custom logger set", func(t *testing.T) {
		cfg := defaultBroadcastConfig()
		customLogger := logrus.New()
		WithLogger(customLogger)(cfg)
		if cfg.Logger != customLogger {
			t.Error("expected custom logger to be set")
		}
	})

	t.Run("nil logger ignored", func(t *testing.T) {
		cfg := defaultBroadcastConfig()
		originalLogger := cfg.Logger
		WithLogger(nil)(cfg)
		if cfg.Logger != originalLogger {
			t.Error("nil logger should not replace existing logger")
		}
	})
}

// TestWithOnSuccess tests the WithOnSuccess functional option.
func TestWithOnSuccess(t *testing.T) {
	cfg := defaultBroadcastConfig()
	var callCount int32

	callback := func(peerID uint32) {
		atomic.AddInt32(&callCount, 1)
	}

	WithOnSuccess(callback)(cfg)
	if cfg.OnSuccess == nil {
		t.Error("expected OnSuccess callback to be set")
	}

	// Verify callback is callable
	cfg.OnSuccess(1)
	if atomic.LoadInt32(&callCount) != 1 {
		t.Error("expected callback to be invoked")
	}
}

// TestWithOnFailure tests the WithOnFailure functional option.
func TestWithOnFailure(t *testing.T) {
	cfg := defaultBroadcastConfig()
	var callCount int32
	var capturedErr error

	callback := func(peerID uint32, err error) {
		atomic.AddInt32(&callCount, 1)
		capturedErr = err
	}

	WithOnFailure(callback)(cfg)
	if cfg.OnFailure == nil {
		t.Error("expected OnFailure callback to be set")
	}

	// Verify callback captures error
	testErr := &testError{msg: "test error"}
	cfg.OnFailure(1, testErr)
	if atomic.LoadInt32(&callCount) != 1 {
		t.Error("expected callback to be invoked")
	}
	if capturedErr != testErr {
		t.Error("expected error to be captured")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// TestDefaultBroadcastConfig tests that default config has expected values.
func TestDefaultBroadcastConfig(t *testing.T) {
	cfg := defaultBroadcastConfig()

	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", cfg.Timeout)
	}
	if cfg.MaxWorkers != 10 {
		t.Errorf("expected default max workers 10, got %d", cfg.MaxWorkers)
	}
	if cfg.Logger == nil {
		t.Error("expected default logger to be set")
	}
	if cfg.OnSuccess != nil {
		t.Error("expected OnSuccess to be nil by default")
	}
	if cfg.OnFailure != nil {
		t.Error("expected OnFailure to be nil by default")
	}
}

// TestBroadcastGroupUpdateWithOptions tests the broadcast function with options.
func TestBroadcastGroupUpdateWithOptions(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		SelfPeerID: 1,
		Peers: map[uint32]*Peer{
			1: {ID: 1, PublicKey: [32]byte{0x01}, Connection: 1},
			2: {ID: 2, PublicKey: [32]byte{0x02}, Connection: 1, Address: &testAddr{network: "udp", address: "192.168.1.2:33445"}},
		},
		PeerCount:    2,
		transport:    mockTrans,
		dht:          testDHT,
		timeProvider: &DefaultTimeProvider{},
	}

	t.Run("with custom timeout", func(t *testing.T) {
		err := chat.broadcastGroupUpdateWithOptions("test", map[string]interface{}{
			"key": "value",
		}, WithTimeout(5*time.Second))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("with custom max workers", func(t *testing.T) {
		err := chat.broadcastGroupUpdateWithOptions("test", map[string]interface{}{
			"key": "value",
		}, WithMaxWorkers(5))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("with success callback", func(t *testing.T) {
		var successCount int32
		err := chat.broadcastGroupUpdateWithOptions("test", map[string]interface{}{
			"key": "value",
		}, WithOnSuccess(func(peerID uint32) {
			atomic.AddInt32(&successCount, 1)
		}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if atomic.LoadInt32(&successCount) != 1 {
			t.Errorf("expected 1 success callback, got %d", atomic.LoadInt32(&successCount))
		}
	})

	t.Run("with combined options", func(t *testing.T) {
		var successCount int32
		customLogger := logrus.New()
		customLogger.SetLevel(logrus.DebugLevel)

		err := chat.broadcastGroupUpdateWithOptions("test", map[string]interface{}{
			"key": "value",
		},
			WithTimeout(10*time.Second),
			WithMaxWorkers(3),
			WithLogger(customLogger),
			WithOnSuccess(func(peerID uint32) {
				atomic.AddInt32(&successCount, 1)
			}),
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// TestBroadcastGroupUpdateTypedWithOptions tests the typed broadcast with options.
func TestBroadcastGroupUpdateTypedWithOptions(t *testing.T) {
	mockTrans := &mockTransport{}
	testDHT := createTestRoutingTable([]*dht.Node{})

	chat := &Chat{
		ID:         1,
		SelfPeerID: 1,
		Peers: map[uint32]*Peer{
			1: {ID: 1, PublicKey: [32]byte{0x01}, Connection: 1},
			2: {ID: 2, PublicKey: [32]byte{0x02}, Connection: 1, Address: &testAddr{network: "udp", address: "192.168.1.2:33445"}},
		},
		PeerCount:    2,
		transport:    mockTrans,
		dht:          testDHT,
		timeProvider: &DefaultTimeProvider{},
	}

	t.Run("with typed data and options", func(t *testing.T) {
		var successPeers []uint32
		err := chat.BroadcastGroupUpdateTypedWithOptions("peer_name_change",
			PeerNameChangeData{
				PeerID:  1,
				NewName: "TestUser",
			},
			WithTimeout(5*time.Second),
			WithOnSuccess(func(peerID uint32) {
				successPeers = append(successPeers, peerID)
			}),
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(successPeers) != 1 {
			t.Errorf("expected 1 successful peer, got %d", len(successPeers))
		}
	})
}

// TestCollectBroadcastResultsWithCallbacks tests callback invocation during result collection.
func TestCollectBroadcastResultsWithCallbacks(t *testing.T) {
	t.Run("callbacks invoked correctly", func(t *testing.T) {
		resultChan := make(chan result, 3)
		resultChan <- result{peerID: 1, err: nil, cancelled: false}                // success
		resultChan <- result{peerID: 2, err: &testError{"fail"}, cancelled: false} // failure
		resultChan <- result{peerID: 3, err: &testError{"ctx"}, cancelled: true}   // cancelled

		var successPeers, failurePeers []uint32
		onSuccess := func(peerID uint32) { successPeers = append(successPeers, peerID) }
		onFailure := func(peerID uint32, err error) { failurePeers = append(failurePeers, peerID) }

		successCount, errors := collectBroadcastResultsWithCallbacks(resultChan, 3, onSuccess, onFailure)

		if successCount != 1 {
			t.Errorf("expected 1 success, got %d", successCount)
		}
		if len(errors) != 2 {
			t.Errorf("expected 2 errors, got %d", len(errors))
		}
		if len(successPeers) != 1 || successPeers[0] != 1 {
			t.Errorf("expected success peer 1, got %v", successPeers)
		}
		if len(failurePeers) != 2 {
			t.Errorf("expected 2 failure peers, got %v", failurePeers)
		}
	})

	t.Run("nil callbacks handled safely", func(t *testing.T) {
		resultChan := make(chan result, 1)
		resultChan <- result{peerID: 1, err: nil, cancelled: false}

		// Should not panic with nil callbacks
		successCount, errors := collectBroadcastResultsWithCallbacks(resultChan, 1, nil, nil)
		if successCount != 1 {
			t.Errorf("expected 1 success, got %d", successCount)
		}
		if len(errors) != 0 {
			t.Errorf("expected 0 errors, got %d", len(errors))
		}
	})
}
