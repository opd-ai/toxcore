package bootstrap_test

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/bootstrap"
)

// TestNewWithNilConfig verifies that New accepts a nil config and uses defaults.
func TestNewWithNilConfig(t *testing.T) {
	srv, err := bootstrap.New(nil)
	if err != nil {
		t.Fatalf("New(nil) returned unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("New(nil) returned nil server")
	}
}

// TestNewWithDefaultConfig verifies that New succeeds with DefaultConfig.
func TestNewWithDefaultConfig(t *testing.T) {
	cfg := bootstrap.DefaultConfig()
	srv, err := bootstrap.New(cfg)
	if err != nil {
		t.Fatalf("New(DefaultConfig()) returned unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("New returned nil server")
	}
}

// TestGetPublicKey verifies that the public key is 32 non-zero bytes.
func TestGetPublicKey(t *testing.T) {
	srv, err := bootstrap.New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	pk := srv.GetPublicKey()
	allZero := true
	for _, b := range pk {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("GetPublicKey returned an all-zero key")
	}
}

// TestGetPublicKeyHex verifies the hex encoding of the public key.
func TestGetPublicKeyHex(t *testing.T) {
	srv, err := bootstrap.New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	hex := srv.GetPublicKeyHex()
	if len(hex) != 64 {
		t.Errorf("GetPublicKeyHex length = %d, want 64", len(hex))
	}
	for _, c := range hex {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("GetPublicKeyHex contains non-hex character %q", c)
		}
	}
}

// TestIsRunningBeforeStart verifies that IsRunning returns false before Start.
func TestIsRunningBeforeStart(t *testing.T) {
	srv, err := bootstrap.New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if srv.IsRunning() {
		t.Error("IsRunning() returned true before Start()")
	}
}

// TestClearnetStartStop verifies a clearnet bootstrap server can start and stop cleanly.
func TestClearnetStartStop(t *testing.T) {
	cfg := bootstrap.DefaultConfig()
	cfg.ClearnetEnabled = true
	cfg.OnionEnabled = false
	cfg.I2PEnabled = false
	cfg.ClearnetPort = findFreePort(t)

	srv, err := bootstrap.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if !srv.IsRunning() {
		t.Error("IsRunning() returned false after Start()")
	}

	addr := srv.GetClearnetAddr()
	if addr == "" {
		t.Error("GetClearnetAddr() returned empty string after Start()")
	}
	t.Logf("Clearnet address: %s", addr)
	t.Logf("Public key: %s", srv.GetPublicKeyHex())

	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if srv.IsRunning() {
		t.Error("IsRunning() returned true after Stop()")
	}
}

// TestDoubleStart verifies that calling Start twice returns an error.
func TestDoubleStart(t *testing.T) {
	cfg := bootstrap.DefaultConfig()
	cfg.ClearnetEnabled = true
	cfg.OnionEnabled = false
	cfg.I2PEnabled = false
	cfg.ClearnetPort = findFreePort(t)

	srv, err := bootstrap.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	defer srv.Stop() //nolint:errcheck

	if err := srv.Start(ctx); err == nil {
		t.Error("second Start() should have returned an error")
	}
}

// TestStartAfterStop verifies that Start returns an error after Stop.
func TestStartAfterStop(t *testing.T) {
	cfg := bootstrap.DefaultConfig()
	cfg.ClearnetEnabled = true
	cfg.OnionEnabled = false
	cfg.I2PEnabled = false
	cfg.ClearnetPort = findFreePort(t)

	srv, err := bootstrap.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if err := srv.Start(ctx); err == nil {
		t.Error("Start() after Stop() should have returned an error")
	}
}

// TestStopBeforeStart verifies that Stop on an unstarted server is a no-op.
func TestStopBeforeStart(t *testing.T) {
	srv, err := bootstrap.New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop on unstarted server returned error: %v", err)
	}
}

// TestOnionDisabledAddr verifies GetOnionAddr returns "" when onion is disabled.
func TestOnionDisabledAddr(t *testing.T) {
	cfg := bootstrap.DefaultConfig()
	cfg.OnionEnabled = false

	srv, err := bootstrap.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if srv.GetOnionAddr() != "" {
		t.Errorf("GetOnionAddr() = %q, want \"\" when onion disabled", srv.GetOnionAddr())
	}
}

// TestI2PDisabledAddr verifies GetI2PAddr returns "" when I2P is disabled.
func TestI2PDisabledAddr(t *testing.T) {
	cfg := bootstrap.DefaultConfig()
	cfg.I2PEnabled = false

	srv, err := bootstrap.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if srv.GetI2PAddr() != "" {
		t.Errorf("GetI2PAddr() = %q, want \"\" when I2P disabled", srv.GetI2PAddr())
	}
}

// TestPublicKeyStableAcrossRestarts verifies that injecting a saved secret key
// produces the same public key.
func TestPublicKeyStableAcrossRestarts(t *testing.T) {
	cfg := bootstrap.DefaultConfig()
	cfg.ClearnetEnabled = true
	cfg.OnionEnabled = false
	cfg.I2PEnabled = false
	cfg.ClearnetPort = findFreePort(t)

	// First server
	srv1, err := bootstrap.New(cfg)
	if err != nil {
		t.Fatalf("New (1): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv1.Start(ctx); err != nil {
		t.Fatalf("Start (1): %v", err)
	}
	pk1 := srv1.GetPublicKey()
	srv1.Stop() //nolint:errcheck

	// Second server with the same secret key (via SaveSecretKey)
	cfg2 := bootstrap.DefaultConfig()
	cfg2.ClearnetEnabled = true
	cfg2.OnionEnabled = false
	cfg2.I2PEnabled = false
	cfg2.ClearnetPort = findFreePort(t)
	cfg2.SecretKey = srv1.GetPrivateKey()

	srv2, err := bootstrap.New(cfg2)
	if err != nil {
		t.Fatalf("New (2): %v", err)
	}
	if err := srv2.Start(ctx); err != nil {
		t.Fatalf("Start (2): %v", err)
	}
	pk2 := srv2.GetPublicKey()
	srv2.Stop() //nolint:errcheck

	if pk1 != pk2 {
		t.Errorf("public key changed between restarts: first=%x second=%x", pk1, pk2)
	}
}

// findFreePort returns a suitable port for the clearnet bootstrap service in tests.
// It returns 0 to let the OS choose an available ephemeral port; tests that
// only need to verify Start/Stop success do not need the actual port number.
func findFreePort(t *testing.T) uint16 {
	t.Helper()
	return 0
}

// TestStartOnionTimeout verifies that startOnion returns a timeout error when
// the Tor listener cannot be established within StartupTimeout.
// A 1ms timeout is used to reliably exercise the timeout path without a Tor daemon.
func TestStartOnionTimeout(t *testing.T) {
	cfg := bootstrap.DefaultConfig()
	cfg.ClearnetEnabled = false
	cfg.OnionEnabled = true
	cfg.I2PEnabled = false
	cfg.StartupTimeout = time.Millisecond

	srv, err := bootstrap.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = srv.Start(context.Background())
	if err == nil {
		srv.Stop() //nolint:errcheck
		t.Skip("Tor daemon appears to be running; skipping timeout test")
	}
	if !containsAny(err.Error(), "onion", "timeout", "timed out") {
		t.Errorf("expected onion/timeout error, got: %v", err)
	}
}

// TestStartI2PTimeout verifies that startI2P returns a timeout error when
// the I2P SAM bridge cannot be reached within StartupTimeout.
func TestStartI2PTimeout(t *testing.T) {
	cfg := bootstrap.DefaultConfig()
	cfg.ClearnetEnabled = false
	cfg.OnionEnabled = false
	cfg.I2PEnabled = true
	cfg.I2PSAMAddr = "127.0.0.1:7656"
	cfg.StartupTimeout = time.Millisecond

	srv, err := bootstrap.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = srv.Start(context.Background())
	if err == nil {
		srv.Stop() //nolint:errcheck
		t.Skip("I2P SAM bridge appears to be running; skipping timeout test")
	}
	if !containsAny(err.Error(), "I2P", "timeout", "timed out") {
		t.Errorf("expected I2P/timeout error, got: %v", err)
	}
}

// TestStartAlreadyRunning verifies that calling Start twice returns an error.
func TestStartAlreadyRunning(t *testing.T) {
	cfg := bootstrap.DefaultConfig()
	cfg.ClearnetEnabled = true
	cfg.OnionEnabled = false
	cfg.I2PEnabled = false
	cfg.ClearnetPort = 0

	srv, err := bootstrap.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Stop() //nolint:errcheck

	if err := srv.Start(ctx); err == nil {
		t.Fatal("expected error on second Start(), got nil")
	}
}

// TestStopIdempotent verifies that calling Stop on an already-stopped server is a no-op.
func TestStopIdempotent(t *testing.T) {
	cfg := bootstrap.DefaultConfig()
	cfg.ClearnetEnabled = true
	cfg.OnionEnabled = false
	cfg.I2PEnabled = false
	cfg.ClearnetPort = 0

	srv, err := bootstrap.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop (first): %v", err)
	}
	if err := srv.Stop(); err != nil {
		t.Errorf("Stop (second, idempotent) returned unexpected error: %v", err)
	}
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && containsStr(s, sub) {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
