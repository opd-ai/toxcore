package net

import (
	"testing"
	"time"

	"github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/crypto"
)

func TestDialTimeout(t *testing.T) {
	// Create a Tox instance
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create a valid but unreachable Tox ID for testing timeout
	publicKey := [32]byte{
		0x76, 0x51, 0x84, 0x06, 0xF6, 0xA9, 0xF2, 0x21,
		0x7E, 0x8D, 0xC4, 0x87, 0xCC, 0x78, 0x3C, 0x25,
		0xCC, 0x16, 0xA1, 0x5E, 0xB3, 0x6F, 0xF3, 0x2E,
		0x33, 0x53, 0x64, 0xEC, 0x37, 0x16, 0x6A, 0x87,
	}
	nospam := [4]byte{0x12, 0xA2, 0x0C, 0x01}

	toxID := crypto.NewToxID(publicKey, nospam)
	toxIDString := toxID.String()

	// Test dial with very short timeout - should fail
	start := time.Now()
	_, err = DialTimeout(toxIDString, tox, 10*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error")
	}

	// Should timeout reasonably quickly
	if elapsed > 200*time.Millisecond {
		t.Errorf("Timeout took too long: %v", elapsed)
	}
}

func TestDialInvalidToxID(t *testing.T) {
	// Create a Tox instance
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test invalid Tox ID
	_, err = Dial("invalid_tox_id", tox)
	if err == nil {
		t.Error("Expected error for invalid Tox ID")
	}

	// Check it's a ToxNetError
	toxErr, ok := err.(*ToxNetError)
	if !ok {
		t.Errorf("Expected ToxNetError, got %T", err)
	} else {
		if toxErr.Op != "parse" {
			t.Errorf("Expected op 'parse', got '%s'", toxErr.Op)
		}
	}
}

func TestToxConnInterface(t *testing.T) {
	// Test that ToxConn properly implements net.Conn interface
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create addresses
	localPublicKey := tox.SelfGetPublicKey()
	localNospam := tox.SelfGetNospam()
	localAddr := NewToxAddrFromPublicKey(localPublicKey, localNospam)

	remotePublicKey := [32]byte{1, 2, 3, 4, 5}
	remoteNospam := [4]byte{6, 7, 8, 9}
	remoteAddr := NewToxAddrFromPublicKey(remotePublicKey, remoteNospam)

	// Create connection
	conn := newToxConn(tox, 123, localAddr, remoteAddr)
	defer conn.Close()

	// Test interface methods
	if conn.LocalAddr() != localAddr {
		t.Error("LocalAddr mismatch")
	}

	if conn.RemoteAddr() != remoteAddr {
		t.Error("RemoteAddr mismatch")
	}

	// Test deadline setting
	deadline := time.Now().Add(time.Hour)
	err = conn.SetDeadline(deadline)
	if err != nil {
		t.Errorf("SetDeadline failed: %v", err)
	}

	err = conn.SetReadDeadline(deadline)
	if err != nil {
		t.Errorf("SetReadDeadline failed: %v", err)
	}

	err = conn.SetWriteDeadline(deadline)
	if err != nil {
		t.Errorf("SetWriteDeadline failed: %v", err)
	}

	// Test FriendID method
	if conn.FriendID() != 123 {
		t.Errorf("Expected FriendID 123, got %d", conn.FriendID())
	}

	// Test IsConnected (should be false since friend is not actually online)
	if conn.IsConnected() {
		t.Error("Expected IsConnected to be false")
	}
}

func TestListenerInterface(t *testing.T) {
	// Test that ToxListener properly implements net.Listener interface
	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create listener
	listener := newToxListener(tox, true)
	defer listener.Close()

	// Test Addr method
	addr := listener.Addr()
	if addr.Network() != "tox" {
		t.Errorf("Expected network 'tox', got '%s'", addr.Network())
	}

	// Test auto-accept configuration
	if !listener.IsAutoAccept() {
		t.Error("Expected auto-accept to be true")
	}

	listener.SetAutoAccept(false)
	if listener.IsAutoAccept() {
		t.Error("Expected auto-accept to be false")
	}

	// Test Close
	err = listener.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Test Accept after close (should return error)
	_, err = listener.Accept()
	if err != ErrListenerClosed {
		t.Errorf("Expected ErrListenerClosed, got %v", err)
	}
}
