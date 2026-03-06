package main

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/bootstrap"
)

// getFreeUDPPort returns an available UDP port for use in tests.
func getFreeUDPPort(t *testing.T) uint16 {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.ListenPacket: %v", err)
	}
	defer func() {
		if closeErr := pc.Close(); closeErr != nil {
			t.Fatalf("pc.Close: %v", closeErr)
		}
	}()
	_, portStr, err := net.SplitHostPort(pc.LocalAddr().String())
	if err != nil {
		t.Fatalf("net.SplitHostPort: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("strconv.Atoi(%q): %v", portStr, err)
	}
	return uint16(port)
}

// TestBootstrapServerDemoStartStop verifies the demo server can start and stop cleanly.
func TestBootstrapServerDemoStartStop(t *testing.T) {
	cfg := bootstrap.DefaultConfig()
	cfg.ClearnetEnabled = true
	cfg.OnionEnabled = false
	cfg.I2PEnabled = false
	cfg.ClearnetPort = getFreeUDPPort(t)

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
