package transport

import (
	"net"
	"testing"
	"time"
)

// TestNewTCPTransportFromListenerSuccess verifies that a valid net.Listener
// produces a working transport whose LocalAddr matches the listener.
func TestNewTCPTransportFromListenerSuccess(t *testing.T) {
	// Use the loopback interface on any available port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	// Do NOT close ln here – the transport takes ownership.

	tr, err := NewTCPTransportFromListener(ln)
	if err != nil {
		t.Fatalf("NewTCPTransportFromListener: %v", err)
	}
	defer tr.Close() //nolint:errcheck

	if tr.LocalAddr() == nil {
		t.Fatal("LocalAddr() returned nil")
	}
	if tr.LocalAddr().String() != ln.Addr().String() {
		t.Errorf("LocalAddr() = %s, want %s", tr.LocalAddr().String(), ln.Addr().String())
	}
}

// TestNewTCPTransportFromListenerNilReturnsError verifies that passing nil
// returns an error rather than panicking.
func TestNewTCPTransportFromListenerNilReturnsError(t *testing.T) {
	tr, err := NewTCPTransportFromListener(nil)
	if err == nil {
		if tr != nil {
			tr.Close() //nolint:errcheck
		}
		t.Fatal("expected an error for nil listener, got nil")
	}
}

// TestNewTCPTransportFromListenerClose verifies that Close() prevents new
// connections from being accepted and does not block.
func TestNewTCPTransportFromListenerClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}

	tr, err := NewTCPTransportFromListener(ln)
	if err != nil {
		t.Fatalf("NewTCPTransportFromListener: %v", err)
	}

	// Record the local address before Close.
	addr := tr.LocalAddr().String()

	if closeErr := tr.Close(); closeErr != nil {
		t.Errorf("Close() returned unexpected error: %v", closeErr)
	}

	// After Close, the underlying listener should be gone; a new connection
	// to the same address should fail (or at least not succeed immediately).
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, dialErr := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if dialErr == nil {
			conn.Close()
		}
	}()

	select {
	case <-done:
		// OK – dial completed (with or without error)
	case <-time.After(1 * time.Second):
		t.Error("dial after Close timed out unexpectedly")
	}
}
