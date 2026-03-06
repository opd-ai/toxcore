//go:build !nonet
// +build !nonet

package main

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/bootstrap"
)

// TestBootstrapServerDemoStartStop verifies the demo server can start and stop cleanly.
func TestBootstrapServerDemoStartStop(t *testing.T) {
	cfg := bootstrap.DefaultConfig()
	cfg.ClearnetEnabled = true
	cfg.OnionEnabled = false
	cfg.I2PEnabled = false
	cfg.ClearnetPort = 33610 // Use a port unlikely to conflict

	srv, err := bootstrap.New(cfg)
	if err != nil {
		t.Fatalf("bootstrap.New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	t.Logf("Clearnet addr: %s", srv.GetClearnetAddr())
	t.Logf("Public key:    %s", srv.GetPublicKeyHex())

	if err := srv.Stop(); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

// TestBootstrapServerDemoPublicKeyNonEmpty verifies the public key is non-empty.
func TestBootstrapServerDemoPublicKeyNonEmpty(t *testing.T) {
	srv, err := bootstrap.New(bootstrap.DefaultConfig())
	if err != nil {
		t.Fatalf("bootstrap.New: %v", err)
	}
	if key := srv.GetPublicKeyHex(); len(key) != 64 {
		t.Errorf("GetPublicKeyHex() length = %d, want 64", len(key))
	}
}
