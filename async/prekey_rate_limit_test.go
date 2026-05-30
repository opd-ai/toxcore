package async

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// makeFullPreKeyPool adds PreKeyMinimum+n one-time pre-keys for peerPK into
// fsm, enabling n outbound sends before the pool is exhausted.
func makeFullPreKeyPool(t *testing.T, fsm *ForwardSecurityManager, peerPK [32]byte, extra int) {
	t.Helper()
	total := PreKeyMinimum + extra
	keys := make([]PreKeyForExchange, total)
	for i := range keys {
		kp, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair: %v", err)
		}
		keys[i] = PreKeyForExchange{ID: uint32(i + 1), PublicKey: kp.Public}
	}
	exchange := &PreKeyExchangeMessage{
		Type:      "pre_key_exchange",
		SenderPK:  peerPK,
		PreKeys:   keys,
		Timestamp: time.Now(),
	}
	if err := fsm.ProcessPreKeyExchange(exchange); err != nil {
		t.Fatalf("ProcessPreKeyExchange: %v", err)
	}
}

// TestPreKeyRateLimitEnforced verifies that a peer cannot consume more than
// PreKeyRateLimit one-time pre-keys within PreKeyRateWindowDuration.
func TestPreKeyRateLimitEnforced(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rate-limit-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	senderKP, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair sender: %v", err)
	}
	recipientKP, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair recipient: %v", err)
	}

	fsm, err := NewForwardSecurityManager(senderKP, tmpDir)
	if err != nil {
		t.Fatalf("NewForwardSecurityManager: %v", err)
	}
	defer fsm.Close()

	// Provide a large pool so exhaustion is not the limiting factor.
	makeFullPreKeyPool(t, fsm, recipientKP.Public, PreKeyRateLimit+10)

	// Send up to the rate limit — all must succeed.
	for i := 0; i < PreKeyRateLimit; i++ {
		_, err := fsm.SendForwardSecureMessage(recipientKP.Public, []byte("hello"), MessageTypeNormal)
		if err != nil {
			t.Fatalf("send %d should succeed: %v", i, err)
		}
	}

	// The next send must be rate-limited.
	_, err = fsm.SendForwardSecureMessage(recipientKP.Public, []byte("overflow"), MessageTypeNormal)
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected 'rate limit' in error, got: %v", err)
	}
}

// TestPreKeyLowWatermarkHookFired verifies that the hook is called when the
// peer's remaining pre-key count drops to or below PreKeyLowWatermark.
func TestPreKeyLowWatermarkHookFired(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watermark-hook-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	senderKP, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair sender: %v", err)
	}
	recipientKP, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair recipient: %v", err)
	}

	fsm, err := NewForwardSecurityManager(senderKP, tmpDir)
	if err != nil {
		t.Fatalf("NewForwardSecurityManager: %v", err)
	}
	defer fsm.Close()

	// Provide PreKeyLowWatermark+2 keys so consuming 2 keys puts us exactly at
	// the watermark and triggers the hook.
	makeFullPreKeyPool(t, fsm, recipientKP.Public, PreKeyLowWatermark-PreKeyMinimum+2)

	var mu sync.Mutex
	var hookCalled bool
	fsm.SetPreKeyLowWatermarkHook(func(peerPK [32]byte, remaining int) {
		mu.Lock()
		hookCalled = true
		mu.Unlock()
	})

	// Consume keys until we cross the watermark.
	consumed := 0
	for i := 0; i < 3; i++ {
		if !fsm.CanSendMessage(recipientKP.Public) {
			break
		}
		_, err := fsm.SendForwardSecureMessage(recipientKP.Public, []byte("x"), MessageTypeNormal)
		if err != nil {
			break
		}
		consumed++
	}
	if consumed == 0 {
		t.Skip("could not consume any pre-keys")
	}

	// Give the goroutine a moment to run.
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	fired := hookCalled
	mu.Unlock()
	if !fired {
		t.Fatal("low-watermark hook was not called after consuming pre-keys to/below watermark")
	}
}

// TestPreKeyLowWatermarkHookClearable verifies that passing nil to
// SetPreKeyLowWatermarkHook removes the hook.
func TestPreKeyLowWatermarkHookClearable(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watermark-clear-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	senderKP, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair sender: %v", err)
	}
	recipientKP, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair recipient: %v", err)
	}

	fsm, err := NewForwardSecurityManager(senderKP, tmpDir)
	if err != nil {
		t.Fatalf("NewForwardSecurityManager: %v", err)
	}
	defer fsm.Close()

	makeFullPreKeyPool(t, fsm, recipientKP.Public, PreKeyLowWatermark-PreKeyMinimum+5)

	called := false
	fsm.SetPreKeyLowWatermarkHook(func(_ [32]byte, _ int) { called = true })
	fsm.SetPreKeyLowWatermarkHook(nil) // clear it

	for i := 0; i < 5; i++ {
		if !fsm.CanSendMessage(recipientKP.Public) {
			break
		}
		fsm.SendForwardSecureMessage(recipientKP.Public, []byte("y"), MessageTypeNormal) //nolint:errcheck
	}

	time.Sleep(50 * time.Millisecond)
	if called {
		t.Fatal("hook was called even after being cleared with nil")
	}
}
