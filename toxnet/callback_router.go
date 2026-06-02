package toxnet

import (
	"sync"

	"github.com/opd-ai/toxcore"
)

// maxReadBufferBytes is the per-connection read-buffer cap. Incoming messages
// that would grow the buffer beyond this limit are silently dropped to prevent
// memory exhaustion from a slow-draining or malicious peer (M-03 fix).
const maxReadBufferBytes = 4 << 20 // 4 MiB

// callbackRouter manages per-connection callbacks for Tox instances.
// It prevents callback collision by multiplexing callbacks based on friendID.
// Each Tox instance has at most one callbackRouter managing all its ToxConn instances.
type callbackRouter struct {
	tox *toxcore.Tox

	// connections maps friendID to its corresponding ToxConn
	connections map[uint32]*ToxConn
	mu          sync.RWMutex

	// initialized tracks whether callbacks have been set up for this tox instance
	initialized bool
}

// globalRouters tracks callbackRouter instances by Tox instance address.
// This ensures a single router per Tox instance across all ToxConn instances.
var (
	globalRouters   = make(map[*toxcore.Tox]*callbackRouter)
	globalRoutersMu sync.Mutex
)

// getOrCreateRouter returns the callbackRouter for the given Tox instance,
// creating one if it doesn't exist. Thread-safe.
func getOrCreateRouter(tox *toxcore.Tox) *callbackRouter {
	globalRoutersMu.Lock()
	defer globalRoutersMu.Unlock()

	if router, exists := globalRouters[tox]; exists {
		return router
	}

	router := &callbackRouter{
		tox:         tox,
		connections: make(map[uint32]*ToxConn),
	}
	globalRouters[tox] = router
	return router
}

// cleanupRouter removes the router for a Tox instance if it has no connections.
// Called when a ToxConn is closed.
func cleanupRouter(tox *toxcore.Tox) {
	globalRoutersMu.Lock()
	defer globalRoutersMu.Unlock()

	router, exists := globalRouters[tox]
	if !exists {
		return
	}

	router.mu.RLock()
	connCount := len(router.connections)
	router.mu.RUnlock()

	if connCount == 0 {
		delete(globalRouters, tox)
	}
}

// registerConnection adds a ToxConn to the router and sets up callbacks if needed.
func (r *callbackRouter) registerConnection(conn *ToxConn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.connections[conn.friendID] = conn

	// Set up multiplexed callbacks only once per Tox instance
	if !r.initialized {
		r.setupMultiplexedCallbacks()
		r.initialized = true
	}
}

// unregisterConnection removes a ToxConn from the router.
func (r *callbackRouter) unregisterConnection(friendID uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.connections, friendID)
}

// setupMultiplexedCallbacks sets up the Tox callbacks to route messages
// to the appropriate ToxConn based on friendID.
// routeMessageToConnection delivers a message to the appropriate ToxConn buffer.
func (r *callbackRouter) routeMessageToConnection(friendID uint32, message string) {
	r.mu.RLock()
	conn, exists := r.connections[friendID]
	r.mu.RUnlock()

	if exists && conn != nil {
		conn.readMu.Lock()
		if conn.readBuffer.Len()+len(message) <= maxReadBufferBytes {
			conn.readBuffer.WriteString(message)
			conn.broadcastRead()
		}
		conn.readMu.Unlock()
	}
}

// updateConnectionStatus updates connection state and signals status changes.
func updateConnectionStatus(conn *ToxConn, status toxcore.FriendStatus) {
	conn.mu.Lock()
	wasConnected := conn.connected
	conn.connected = (status == toxcore.FriendStatusOnline)
	conn.mu.Unlock()

	if !wasConnected && conn.connected {
		select {
		case conn.connStateCh <- true:
		default:
		}
	}
}

// updateConnectionStatusByConnStatus updates connection state from a
// FriendConnectionStatus callback.  Any non-None status means transport-level
// connectivity is available, which is what DialContext/Write actually need.
func updateConnectionStatusByConnStatus(conn *ToxConn, status toxcore.ConnectionStatus) {
	conn.mu.Lock()
	wasConnected := conn.connected
	conn.connected = (status != toxcore.ConnectionNone)
	conn.mu.Unlock()

	if !wasConnected && conn.connected {
		select {
		case conn.connStateCh <- true:
		default:
		}
	}
}

// routeStatusToConnection delivers status updates to the appropriate ToxConn.
func (r *callbackRouter) routeStatusToConnection(friendID uint32, status toxcore.FriendStatus) {
	r.mu.RLock()
	conn, exists := r.connections[friendID]
	r.mu.RUnlock()

	if exists && conn != nil {
		updateConnectionStatus(conn, status)
	}
}

func (r *callbackRouter) setupMultiplexedCallbacks() {
	r.tox.OnFriendMessage(func(friendID uint32, message string) {
		r.routeMessageToConnection(friendID, message)
	})

	r.tox.OnFriendStatus(func(friendID uint32, status toxcore.FriendStatus) {
		r.routeStatusToConnection(friendID, status)
	})

	// H-NET-5: also subscribe to transport-level connectivity changes.
	// OnFriendStatus fires on presence (Away/Busy/Online) but may not fire when
	// the UDP/TCP transport becomes available.  OnFriendConnectionStatus fires
	// on every transport transition and is the correct signal for dial/write.
	r.tox.OnFriendConnectionStatus(func(friendID uint32, status toxcore.ConnectionStatus) {
		r.mu.RLock()
		conn, exists := r.connections[friendID]
		r.mu.RUnlock()
		if exists && conn != nil {
			updateConnectionStatusByConnStatus(conn, status)
		}
	})
}

// getConnection returns the ToxConn for the given friendID, or nil if not found.
func (r *callbackRouter) getConnection(friendID uint32) *ToxConn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.connections[friendID]
}

// connectionCount returns the number of registered connections.
func (r *callbackRouter) connectionCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.connections)
}
