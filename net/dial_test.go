package net

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/crypto"
)

func TestListenAddr(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// ListenAddr should ignore the address parameter and create a valid listener
	listener, err := ListenAddr("ignored-addr", tox)
	if err != nil {
		t.Fatalf("ListenAddr() error = %v", err)
	}
	defer listener.Close()

	if listener.Addr().Network() != "tox" {
		t.Errorf("Expected network 'tox', got '%s'", listener.Addr().Network())
	}
}

func TestLookupToxAddr(t *testing.T) {
	publicKey := [32]byte{
		0x76, 0x51, 0x84, 0x06, 0xF6, 0xA9, 0xF2, 0x21,
		0x7E, 0x8D, 0xC4, 0x87, 0xCC, 0x78, 0x3C, 0x25,
		0xCC, 0x16, 0xA1, 0x5E, 0xB3, 0x6F, 0xF3, 0x2E,
		0x33, 0x53, 0x64, 0xEC, 0x37, 0x16, 0x6A, 0x87,
	}
	nospam := [4]byte{0x12, 0xA2, 0x0C, 0x01}
	toxID := crypto.NewToxID(publicKey, nospam)
	validToxIDString := toxID.String()

	t.Run("valid address", func(t *testing.T) {
		addr, err := LookupToxAddr(validToxIDString)
		if err != nil {
			t.Errorf("LookupToxAddr() error = %v", err)
		}
		if addr == nil {
			t.Error("LookupToxAddr() returned nil addr")
		}
	})

	t.Run("invalid address", func(t *testing.T) {
		_, err := LookupToxAddr("invalid")
		if err == nil {
			t.Error("LookupToxAddr() expected error for invalid address")
		}
	})
}

func TestDialTox(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test with invalid Tox ID
	_, err = DialTox("invalid", tox)
	if err == nil {
		t.Error("DialTox() expected error for invalid Tox ID")
	}
}

func TestListenTox(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	listener, err := ListenTox(tox)
	if err != nil {
		t.Fatalf("ListenTox() error = %v", err)
	}
	defer listener.Close()

	if listener.Addr().Network() != "tox" {
		t.Errorf("Expected network 'tox', got '%s'", listener.Addr().Network())
	}
}

func TestPacketDialInvalidNetwork(t *testing.T) {
	_, err := PacketDial("invalid-network", "some-address")
	if err == nil {
		t.Error("PacketDial() expected error for invalid network")
	}

	toxErr, ok := err.(*ToxNetError)
	if !ok {
		t.Errorf("Expected ToxNetError, got %T", err)
		return
	}
	if toxErr.Op != "dial" {
		t.Errorf("Expected op 'dial', got '%s'", toxErr.Op)
	}
}

func TestPacketListenInvalidNetwork(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	_, err = PacketListen("invalid-network", ":8080", tox)
	if err == nil {
		t.Error("PacketListen() expected error for invalid network")
	}

	toxErr, ok := err.(*ToxNetError)
	if !ok {
		t.Errorf("Expected ToxNetError, got %T", err)
		return
	}
	if toxErr.Op != "listen" {
		t.Errorf("Expected op 'listen', got '%s'", toxErr.Op)
	}
}

func TestPacketListenNilTox(t *testing.T) {
	_, err := PacketListen("tox", ":8080", nil)
	if err == nil {
		t.Error("PacketListen() expected error for nil Tox")
	}

	toxErr, ok := err.(*ToxNetError)
	if !ok {
		t.Errorf("Expected ToxNetError, got %T", err)
		return
	}
	if toxErr.Op != "listen" {
		t.Errorf("Expected op 'listen', got '%s'", toxErr.Op)
	}
}

func TestDialContextCancelled(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create already cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	publicKey := [32]byte{
		0x76, 0x51, 0x84, 0x06, 0xF6, 0xA9, 0xF2, 0x21,
		0x7E, 0x8D, 0xC4, 0x87, 0xCC, 0x78, 0x3C, 0x25,
		0xCC, 0x16, 0xA1, 0x5E, 0xB3, 0x6F, 0xF3, 0x2E,
		0x33, 0x53, 0x64, 0xEC, 0x37, 0x16, 0x6A, 0x87,
	}
	nospam := [4]byte{0x12, 0xA2, 0x0C, 0x01}
	toxID := crypto.NewToxID(publicKey, nospam)

	_, err = DialContext(ctx, toxID.String(), tox)
	if err == nil {
		t.Error("DialContext() expected error for cancelled context")
	}
}

func TestWaitForConnectionCancelledContext(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 123, localAddr, remoteAddr)
	defer conn.Close()

	// Create already cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = waitForConnection(ctx, conn)
	if err != context.Canceled {
		t.Errorf("waitForConnection() error = %v, want context.Canceled", err)
	}
}

func TestWaitForConnectionShortDeadline(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 123, localAddr, remoteAddr)
	defer conn.Close()

	// Very short deadline - should trigger adaptive interval
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = waitForConnection(ctx, conn)
	elapsed := time.Since(start)

	if err != context.DeadlineExceeded {
		t.Errorf("waitForConnection() error = %v, want context.DeadlineExceeded", err)
	}

	// Should complete quickly
	if elapsed > 100*time.Millisecond {
		t.Errorf("waitForConnection() took %v, expected < 100ms", elapsed)
	}
}

func TestToxListenerCloseTwice(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	listener := newToxListener(tox, true)

	// First close should succeed
	err = listener.Close()
	if err != nil {
		t.Errorf("First Close() error = %v", err)
	}

	// Second close should also return nil (idempotent)
	err = listener.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestToxListenerAcceptAfterClose(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	listener := newToxListener(tox, true)
	listener.Close()

	_, err = listener.Accept()
	if err != ErrListenerClosed {
		t.Errorf("Accept() error = %v, want ErrListenerClosed", err)
	}
}

func TestToxListenerSetupConnectionTimers(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	listener := newToxListener(tox, true)
	defer listener.Close()

	timeout, ticker := listener.setupConnectionTimers()
	if timeout == nil {
		t.Error("setupConnectionTimers() returned nil timeout")
	}
	if ticker == nil {
		t.Error("setupConnectionTimers() returned nil ticker")
	}

	listener.cleanupTimers(timeout, ticker)
}

func TestToxListenerCreateNewConnection(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	listener := newToxListener(tox, true)
	defer listener.Close()

	publicKey := [32]byte{1, 2, 3, 4, 5}
	conn := listener.createNewConnection(123, publicKey)
	defer conn.Close()

	if conn == nil {
		t.Error("createNewConnection() returned nil conn")
	}
	if conn.FriendID() != 123 {
		t.Errorf("FriendID() = %d, want 123", conn.FriendID())
	}
}

func TestToxListenerCleanupConnection(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	listener := newToxListener(tox, true)
	defer listener.Close()

	// Should handle nil without panic
	listener.cleanupConnection(nil)

	// Should close non-nil connection
	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 456, localAddr, remoteAddr)
	listener.cleanupConnection(conn)

	if !conn.closed {
		t.Error("cleanupConnection() should close the connection")
	}
}

func TestPacketListenValid(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	listener, err := PacketListen("tox", ":0", tox)
	if err != nil {
		t.Fatalf("PacketListen() error = %v", err)
	}
	defer listener.Close()

	if listener.Addr().Network() != "tox" {
		t.Errorf("Expected network 'tox', got '%s'", listener.Addr().Network())
	}
}

func TestNetworkErrorInterface(t *testing.T) {
	// Verify ToxNetError implements error interface properly
	var _ error = &ToxNetError{}

	// Verify it can be used with net.Error compatible code
	err := &ToxNetError{
		Op:  "test",
		Err: ErrTimeout,
	}

	// Test that it's a standard Go error
	if err.Error() == "" {
		t.Error("ToxNetError.Error() should not be empty")
	}

	// Test unwrap
	if err.Unwrap() != ErrTimeout {
		t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), ErrTimeout)
	}
}

func TestPacketDialInvalidAddress(t *testing.T) {
	_, err := PacketDial("tox", "invalid-address")
	if err == nil {
		t.Error("PacketDial() expected error for invalid address")
	}

	toxErr, ok := err.(*ToxNetError)
	if !ok {
		t.Errorf("Expected ToxNetError, got %T", err)
		return
	}
	if toxErr.Op != "dial" {
		t.Errorf("Expected op 'dial', got '%s'", toxErr.Op)
	}
}

// TestListenerNetInterface verifies ToxListener implements net.Listener
func TestListenerNetInterface(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	listener := newToxListener(tox, true)
	defer listener.Close()

	// Verify it implements net.Listener
	var _ net.Listener = listener

	// Test Addr returns valid net.Addr
	addr := listener.Addr()
	if addr == nil {
		t.Error("Addr() should not return nil")
	}
	if addr.Network() != "tox" {
		t.Errorf("Addr().Network() = %s, want 'tox'", addr.Network())
	}
}

// TestConnNetInterface verifies ToxConn implements net.Conn
func TestConnNetInterface(t *testing.T) {
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	localAddr := NewToxAddrFromPublicKey(tox.SelfGetPublicKey(), tox.SelfGetNospam())
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1, 2, 3}, [4]byte{4, 5, 6, 7})
	conn := newToxConn(tox, 123, localAddr, remoteAddr)
	defer conn.Close()

	// Verify it implements net.Conn
	var _ net.Conn = conn
}
