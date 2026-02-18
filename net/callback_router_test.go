package net

import (
	"testing"
	"time"

	"github.com/opd-ai/toxcore"
)

// TestCallbackRouterMultipleConnections verifies that multiple ToxConn instances
// sharing the same Tox instance receive messages correctly without collision.
func TestCallbackRouterMultipleConnections(t *testing.T) {
	// Create a Tox instance
	opts := toxcore.NewOptions()
	tox1, err := toxcore.New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Get local address
	localAddr := NewToxAddrFromPublicKey([32]byte{}, [4]byte{})

	// Create two ToxConn instances with different friendIDs
	// These simulate connections to two different friends
	pk1 := [32]byte{1}
	pk2 := [32]byte{2}
	remoteAddr1 := NewToxAddrFromPublicKey(pk1, [4]byte{})
	remoteAddr2 := NewToxAddrFromPublicKey(pk2, [4]byte{})

	conn1 := newToxConn(tox1, 0, localAddr, remoteAddr1)
	conn2 := newToxConn(tox1, 1, localAddr, remoteAddr2)

	defer conn1.Close()
	defer conn2.Close()

	// Verify both connections are using the same router
	if conn1.router != conn2.router {
		t.Error("Expected both connections to share the same router")
	}

	// Verify the router has 2 connections registered
	if count := conn1.router.connectionCount(); count != 2 {
		t.Errorf("Expected 2 connections in router, got %d", count)
	}
}

// TestCallbackRouterIsolation verifies that messages are routed to the correct
// ToxConn based on friendID and don't leak to other connections.
func TestCallbackRouterIsolation(t *testing.T) {
	// Create a Tox instance
	opts := toxcore.NewOptions()
	tox1, err := toxcore.New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox1.Kill()

	localAddr := NewToxAddrFromPublicKey([32]byte{}, [4]byte{})
	pk1 := [32]byte{1}
	pk2 := [32]byte{2}
	remoteAddr1 := NewToxAddrFromPublicKey(pk1, [4]byte{})
	remoteAddr2 := NewToxAddrFromPublicKey(pk2, [4]byte{})

	conn1 := newToxConn(tox1, 0, localAddr, remoteAddr1)
	conn2 := newToxConn(tox1, 1, localAddr, remoteAddr2)

	defer conn1.Close()
	defer conn2.Close()

	// Simulate message delivery to conn1 (friendID 0)
	router := conn1.router
	router.mu.RLock()
	targetConn := router.connections[0]
	router.mu.RUnlock()

	if targetConn != conn1 {
		t.Error("Router should map friendID 0 to conn1")
	}

	// Simulate message to conn2 (friendID 1)
	router.mu.RLock()
	targetConn = router.connections[1]
	router.mu.RUnlock()

	if targetConn != conn2 {
		t.Error("Router should map friendID 1 to conn2")
	}
}

// TestCallbackRouterCleanup verifies that closing all connections removes the router.
func TestCallbackRouterCleanup(t *testing.T) {
	opts := toxcore.NewOptions()
	tox1, err := toxcore.New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox1.Kill()

	localAddr := NewToxAddrFromPublicKey([32]byte{}, [4]byte{})
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1}, [4]byte{})

	conn := newToxConn(tox1, 0, localAddr, remoteAddr)

	// Verify router exists
	globalRoutersMu.Lock()
	_, exists := globalRouters[tox1]
	globalRoutersMu.Unlock()

	if !exists {
		t.Error("Expected router to exist for Tox instance")
	}

	// Close connection
	conn.Close()

	// Allow cleanup to happen
	time.Sleep(10 * time.Millisecond)

	// Verify router was cleaned up
	globalRoutersMu.Lock()
	_, exists = globalRouters[tox1]
	globalRoutersMu.Unlock()

	if exists {
		t.Error("Expected router to be cleaned up after all connections closed")
	}
}

// TestCallbackRouterDifferentToxInstances verifies that different Tox instances
// get different routers.
func TestCallbackRouterDifferentToxInstances(t *testing.T) {
	opts := toxcore.NewOptions()
	tox1, err := toxcore.New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance 1: %v", err)
	}
	defer tox1.Kill()

	tox2, err := toxcore.New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance 2: %v", err)
	}
	defer tox2.Kill()

	localAddr1 := NewToxAddrFromPublicKey([32]byte{}, [4]byte{})
	localAddr2 := NewToxAddrFromPublicKey([32]byte{}, [4]byte{})
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1}, [4]byte{})

	conn1 := newToxConn(tox1, 0, localAddr1, remoteAddr)
	conn2 := newToxConn(tox2, 0, localAddr2, remoteAddr)

	defer conn1.Close()
	defer conn2.Close()

	// Different Tox instances should have different routers
	if conn1.router == conn2.router {
		t.Error("Expected different Tox instances to have different routers")
	}
}

// TestCallbackRouterGetConnection verifies the getConnection method works correctly.
func TestCallbackRouterGetConnection(t *testing.T) {
	opts := toxcore.NewOptions()
	tox1, err := toxcore.New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox1.Kill()

	localAddr := NewToxAddrFromPublicKey([32]byte{}, [4]byte{})
	remoteAddr := NewToxAddrFromPublicKey([32]byte{1}, [4]byte{})

	conn := newToxConn(tox1, 42, localAddr, remoteAddr)
	defer conn.Close()

	router := conn.router

	// Get existing connection
	result := router.getConnection(42)
	if result != conn {
		t.Errorf("getConnection(42) = %v, want %v", result, conn)
	}

	// Get non-existing connection
	result = router.getConnection(999)
	if result != nil {
		t.Errorf("getConnection(999) = %v, want nil", result)
	}
}

// TestCallbackRouterSingleInitialization verifies callbacks are set up only once
// per Tox instance regardless of how many connections are created.
func TestCallbackRouterSingleInitialization(t *testing.T) {
	opts := toxcore.NewOptions()
	tox1, err := toxcore.New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox1.Kill()

	localAddr := NewToxAddrFromPublicKey([32]byte{}, [4]byte{})

	// Create multiple connections
	conns := make([]*ToxConn, 5)
	for i := 0; i < 5; i++ {
		pk := [32]byte{}
		pk[0] = byte(i)
		remoteAddr := NewToxAddrFromPublicKey(pk, [4]byte{})
		conns[i] = newToxConn(tox1, uint32(i), localAddr, remoteAddr)
	}

	// All should share the same router
	router := conns[0].router
	for i := 1; i < 5; i++ {
		if conns[i].router != router {
			t.Errorf("Connection %d has different router", i)
		}
	}

	// Router should be initialized only once
	if !router.initialized {
		t.Error("Router should be marked as initialized")
	}

	// Clean up
	for _, conn := range conns {
		conn.Close()
	}
}
