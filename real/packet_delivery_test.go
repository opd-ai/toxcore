package real

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/interfaces"
)

// mockAddr implements net.Addr for testing
type mockAddr struct {
	network string
	address string
}

func (m *mockAddr) Network() string { return m.network }
func (m *mockAddr) String() string  { return m.address }

// mockSleeper implements Sleeper for testing without actual delays
type mockSleeper struct {
	mu         sync.Mutex
	sleepCalls []time.Duration
}

// Sleep records the sleep duration without actually sleeping
func (m *mockSleeper) Sleep(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sleepCalls = append(m.sleepCalls, d)
}

// getSleepCalls returns all recorded sleep durations
func (m *mockSleeper) getSleepCalls() []time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]time.Duration, len(m.sleepCalls))
	copy(result, m.sleepCalls)
	return result
}

// mockTransport implements interfaces.INetworkTransport for testing
type mockTransport struct {
	mu              sync.RWMutex
	friends         map[uint32]net.Addr
	sendErr         error
	sendToFriendErr error
	getFriendErr    error
	registerErr     error
	closeErr        error
	connected       bool
	sendCount       int32
	closeCalled     bool
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		friends:   make(map[uint32]net.Addr),
		connected: true,
	}
}

func (m *mockTransport) Send(_ []byte, _ net.Addr) error {
	atomic.AddInt32(&m.sendCount, 1)
	return m.sendErr
}

func (m *mockTransport) SendToFriend(friendID uint32, _ []byte) error {
	atomic.AddInt32(&m.sendCount, 1)
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.sendToFriendErr != nil {
		return m.sendToFriendErr
	}
	if _, exists := m.friends[friendID]; !exists {
		return errors.New("friend not registered")
	}
	return nil
}

func (m *mockTransport) GetFriendAddress(friendID uint32) (net.Addr, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getFriendErr != nil {
		return nil, m.getFriendErr
	}
	if addr, exists := m.friends[friendID]; exists {
		return addr, nil
	}
	return nil, errors.New("friend not found")
}

func (m *mockTransport) RegisterFriend(friendID uint32, addr net.Addr) error {
	if m.registerErr != nil {
		return m.registerErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.friends[friendID] = addr
	return nil
}

func (m *mockTransport) Close() error {
	m.closeCalled = true
	return m.closeErr
}

func (m *mockTransport) IsConnected() bool {
	return m.connected
}

func (m *mockTransport) setSendToFriendErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendToFriendErr = err
}

// Test helpers
func defaultConfig() *interfaces.PacketDeliveryConfig {
	return &interfaces.PacketDeliveryConfig{
		NetworkTimeout:  5000,
		RetryAttempts:   3,
		EnableBroadcast: true,
	}
}

func TestNewRealPacketDelivery(t *testing.T) {
	transport := newMockTransport()
	config := defaultConfig()

	pd := NewRealPacketDelivery(transport, config)

	if pd == nil {
		t.Fatal("expected non-nil RealPacketDelivery")
	}
	if pd.transport != transport {
		t.Error("transport not set correctly")
	}
	if pd.config != config {
		t.Error("config not set correctly")
	}
	if pd.friendAddrs == nil {
		t.Error("friendAddrs map not initialized")
	}
}

func TestIsSimulation(t *testing.T) {
	transport := newMockTransport()
	pd := NewRealPacketDelivery(transport, defaultConfig())

	if pd.IsSimulation() {
		t.Error("expected IsSimulation to return false")
	}
}

func TestDeliverPacket_Success(t *testing.T) {
	transport := newMockTransport()
	pd := NewRealPacketDelivery(transport, defaultConfig())

	friendID := uint32(1)
	addr := &mockAddr{network: "udp", address: "127.0.0.1:33445"}

	// Register friend first
	transport.friends[friendID] = addr

	err := pd.DeliverPacket(friendID, []byte("test packet"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeliverPacket_AddressLookup(t *testing.T) {
	transport := newMockTransport()
	pd := NewRealPacketDelivery(transport, defaultConfig())

	friendID := uint32(1)
	addr := &mockAddr{network: "udp", address: "127.0.0.1:33445"}

	// Register friend in transport but not in pd.friendAddrs
	transport.friends[friendID] = addr

	err := pd.DeliverPacket(friendID, []byte("test packet"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify address was cached
	pd.mu.RLock()
	cachedAddr, exists := pd.friendAddrs[friendID]
	pd.mu.RUnlock()

	if !exists {
		t.Error("expected address to be cached")
	}
	if cachedAddr != addr {
		t.Error("cached address does not match")
	}
}

func TestDeliverPacket_FriendNotFound(t *testing.T) {
	transport := newMockTransport()
	pd := NewRealPacketDelivery(transport, defaultConfig())

	err := pd.DeliverPacket(999, []byte("test packet"))
	if err == nil {
		t.Error("expected error for unknown friend")
	}
}

func TestDeliverPacket_RetryOnFailure(t *testing.T) {
	transport := newMockTransport()
	config := &interfaces.PacketDeliveryConfig{
		NetworkTimeout:  5000,
		RetryAttempts:   1, // Only 1 attempt to speed up test
		EnableBroadcast: true,
	}
	pd := NewRealPacketDelivery(transport, config)

	friendID := uint32(1)
	addr := &mockAddr{network: "udp", address: "127.0.0.1:33445"}
	transport.friends[friendID] = addr

	// Set error to force failure
	transport.setSendToFriendErr(errors.New("network error"))

	err := pd.DeliverPacket(friendID, []byte("test packet"))
	if err == nil {
		t.Error("expected error after all retries failed")
	}

	// Verify retry count (1 attempt)
	if atomic.LoadInt32(&transport.sendCount) != 1 {
		t.Errorf("expected 1 send attempt, got %d", transport.sendCount)
	}
}

func TestBroadcastPacket_Success(t *testing.T) {
	transport := newMockTransport()
	pd := NewRealPacketDelivery(transport, defaultConfig())

	// Add multiple friends
	for i := uint32(1); i <= 3; i++ {
		addr := &mockAddr{network: "udp", address: "127.0.0.1:33445"}
		transport.friends[i] = addr
		pd.mu.Lock()
		pd.friendAddrs[i] = addr
		pd.mu.Unlock()
	}

	err := pd.BroadcastPacket([]byte("broadcast packet"), nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBroadcastPacket_WithExclusions(t *testing.T) {
	transport := newMockTransport()
	pd := NewRealPacketDelivery(transport, defaultConfig())

	// Add 3 friends
	for i := uint32(1); i <= 3; i++ {
		addr := &mockAddr{network: "udp", address: "127.0.0.1:33445"}
		transport.friends[i] = addr
		pd.mu.Lock()
		pd.friendAddrs[i] = addr
		pd.mu.Unlock()
	}

	// Exclude friend 2
	err := pd.BroadcastPacket([]byte("broadcast packet"), []uint32{2})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should have sent to 2 friends (excluding friend 2)
	if atomic.LoadInt32(&transport.sendCount) != 2 {
		t.Errorf("expected 2 sends (excluding friend 2), got %d", transport.sendCount)
	}
}

func TestBroadcastPacket_Disabled(t *testing.T) {
	transport := newMockTransport()
	config := &interfaces.PacketDeliveryConfig{
		NetworkTimeout:  5000,
		RetryAttempts:   3,
		EnableBroadcast: false, // Disabled
	}
	pd := NewRealPacketDelivery(transport, config)

	err := pd.BroadcastPacket([]byte("broadcast packet"), nil)
	if err == nil {
		t.Error("expected error when broadcast is disabled")
	}
}

func TestBroadcastPacket_PartialFailure(t *testing.T) {
	transport := newMockTransport()
	config := &interfaces.PacketDeliveryConfig{
		NetworkTimeout:  5000,
		RetryAttempts:   1, // Single attempt for speed
		EnableBroadcast: true,
	}
	pd := NewRealPacketDelivery(transport, config)

	// Add friends (friend 1 will fail, friend 2 will succeed)
	for i := uint32(1); i <= 2; i++ {
		addr := &mockAddr{network: "udp", address: "127.0.0.1:33445"}
		// Only register friend 2 in transport to simulate friend 1 failure
		if i == 2 {
			transport.friends[i] = addr
		}
		pd.mu.Lock()
		pd.friendAddrs[i] = addr
		pd.mu.Unlock()
	}

	err := pd.BroadcastPacket([]byte("broadcast packet"), nil)
	if err == nil {
		t.Error("expected error for partial broadcast failure")
	}
}

func TestSetNetworkTransport(t *testing.T) {
	transport1 := newMockTransport()
	transport2 := newMockTransport()
	pd := NewRealPacketDelivery(transport1, defaultConfig())

	// Add a friend to verify cache clearing
	pd.mu.Lock()
	pd.friendAddrs[1] = &mockAddr{network: "udp", address: "127.0.0.1:33445"}
	pd.mu.Unlock()

	err := pd.SetNetworkTransport(transport2)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify old transport was closed
	if !transport1.closeCalled {
		t.Error("expected old transport to be closed")
	}

	// Verify new transport is set
	if pd.transport != transport2 {
		t.Error("transport not updated")
	}

	// Verify friend addresses were cleared
	pd.mu.RLock()
	if len(pd.friendAddrs) != 0 {
		t.Error("expected friend addresses to be cleared")
	}
	pd.mu.RUnlock()
}

func TestSetNetworkTransport_CloseError(t *testing.T) {
	transport1 := newMockTransport()
	transport1.closeErr = errors.New("close error")
	transport2 := newMockTransport()
	pd := NewRealPacketDelivery(transport1, defaultConfig())

	// Should fail and propagate the close error
	err := pd.SetNetworkTransport(transport2)
	if err == nil {
		t.Error("expected error when close fails")
	}

	// Verify error is properly wrapped
	if !errors.Is(err, transport1.closeErr) {
		t.Errorf("expected wrapped close error, got: %v", err)
	}

	// Transport should NOT be updated when close fails
	if pd.transport == transport2 {
		t.Error("transport should not be updated when close fails")
	}

	// Original transport should still be set
	if pd.transport != transport1 {
		t.Error("original transport should remain when close fails")
	}
}

func TestAddFriend_Success(t *testing.T) {
	transport := newMockTransport()
	pd := NewRealPacketDelivery(transport, defaultConfig())

	friendID := uint32(1)
	addr := &mockAddr{network: "udp", address: "127.0.0.1:33445"}

	err := pd.AddFriend(friendID, addr)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify friend was added to local cache
	pd.mu.RLock()
	cachedAddr, exists := pd.friendAddrs[friendID]
	pd.mu.RUnlock()

	if !exists {
		t.Error("expected friend to be added to cache")
	}
	if cachedAddr != addr {
		t.Error("cached address does not match")
	}

	// Verify friend was registered with transport
	transport.mu.RLock()
	transportAddr, exists := transport.friends[friendID]
	transport.mu.RUnlock()

	if !exists {
		t.Error("expected friend to be registered with transport")
	}
	if transportAddr != addr {
		t.Error("transport address does not match")
	}
}

func TestAddFriend_TransportRegisterError(t *testing.T) {
	transport := newMockTransport()
	transport.registerErr = errors.New("register error")
	pd := NewRealPacketDelivery(transport, defaultConfig())

	err := pd.AddFriend(1, &mockAddr{network: "udp", address: "127.0.0.1:33445"})
	if err == nil {
		t.Error("expected error when transport registration fails")
	}
}

func TestAddFriend_NilTransport(t *testing.T) {
	pd := &RealPacketDelivery{
		friendAddrs: make(map[uint32]net.Addr),
		config:      defaultConfig(),
		transport:   nil,
	}

	addr := &mockAddr{network: "udp", address: "127.0.0.1:33445"}
	err := pd.AddFriend(1, addr)
	if err != nil {
		t.Errorf("unexpected error with nil transport: %v", err)
	}

	// Should still add to local cache
	pd.mu.RLock()
	if _, exists := pd.friendAddrs[1]; !exists {
		t.Error("expected friend to be added to local cache even with nil transport")
	}
	pd.mu.RUnlock()
}

func TestRemoveFriend(t *testing.T) {
	transport := newMockTransport()
	pd := NewRealPacketDelivery(transport, defaultConfig())

	friendID := uint32(1)
	addr := &mockAddr{network: "udp", address: "127.0.0.1:33445"}

	// Add friend first
	pd.mu.Lock()
	pd.friendAddrs[friendID] = addr
	pd.mu.Unlock()

	err := pd.RemoveFriend(friendID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify friend was removed
	pd.mu.RLock()
	_, exists := pd.friendAddrs[friendID]
	pd.mu.RUnlock()

	if exists {
		t.Error("expected friend to be removed")
	}
}

func TestRemoveFriend_NonExistent(t *testing.T) {
	transport := newMockTransport()
	pd := NewRealPacketDelivery(transport, defaultConfig())

	// Should not error when removing non-existent friend
	err := pd.RemoveFriend(999)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetStats(t *testing.T) {
	transport := newMockTransport()
	transport.connected = true
	config := defaultConfig()
	pd := NewRealPacketDelivery(transport, config)

	// Add some friends
	for i := uint32(1); i <= 3; i++ {
		pd.mu.Lock()
		pd.friendAddrs[i] = &mockAddr{network: "udp", address: "127.0.0.1:33445"}
		pd.mu.Unlock()
	}

	stats := pd.GetStats()

	if stats["total_friends"].(int) != 3 {
		t.Errorf("expected total_friends=3, got %v", stats["total_friends"])
	}
	if stats["is_simulation"].(bool) != false {
		t.Error("expected is_simulation=false")
	}
	if stats["transport_connected"].(bool) != true {
		t.Error("expected transport_connected=true")
	}
	if stats["broadcast_enabled"].(bool) != true {
		t.Error("expected broadcast_enabled=true")
	}
	if stats["retry_attempts"].(int) != 3 {
		t.Errorf("expected retry_attempts=3, got %v", stats["retry_attempts"])
	}
	if stats["network_timeout"].(int) != 5000 {
		t.Errorf("expected network_timeout=5000, got %v", stats["network_timeout"])
	}
}

func TestGetStats_DisconnectedTransport(t *testing.T) {
	transport := newMockTransport()
	transport.connected = false
	pd := NewRealPacketDelivery(transport, defaultConfig())

	stats := pd.GetStats()

	if stats["transport_connected"].(bool) != false {
		t.Error("expected transport_connected=false")
	}
}

func TestConcurrentAccess(t *testing.T) {
	transport := newMockTransport()
	pd := NewRealPacketDelivery(transport, defaultConfig())

	var wg sync.WaitGroup
	const numGoroutines = 10

	// Concurrent AddFriend
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			addr := &mockAddr{network: "udp", address: "127.0.0.1:33445"}
			_ = pd.AddFriend(uint32(id), addr)
		}(i)
	}

	wg.Wait()

	// Concurrent GetStats while modifying
	for i := 0; i < numGoroutines; i++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			_ = pd.GetStats()
		}(i)
		go func(id int) {
			defer wg.Done()
			_ = pd.RemoveFriend(uint32(id))
		}(i)
	}

	wg.Wait()
}

func TestDeliverPacket_CachedAddress(t *testing.T) {
	transport := newMockTransport()
	pd := NewRealPacketDelivery(transport, defaultConfig())

	friendID := uint32(1)
	addr := &mockAddr{network: "udp", address: "127.0.0.1:33445"}

	// Pre-cache the address in pd.friendAddrs but NOT in transport
	pd.mu.Lock()
	pd.friendAddrs[friendID] = addr
	pd.mu.Unlock()

	// Also need friend in transport for SendToFriend to succeed
	transport.friends[friendID] = addr

	err := pd.DeliverPacket(friendID, []byte("test packet"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBroadcastPacket_EmptyFriendList(t *testing.T) {
	transport := newMockTransport()
	pd := NewRealPacketDelivery(transport, defaultConfig())

	// No friends registered
	err := pd.BroadcastPacket([]byte("broadcast packet"), nil)
	if err != nil {
		t.Errorf("unexpected error with empty friend list: %v", err)
	}

	// Should have sent to 0 friends
	if atomic.LoadInt32(&transport.sendCount) != 0 {
		t.Errorf("expected 0 sends, got %d", transport.sendCount)
	}
}

func TestDeliverPacket_DeterministicSleep(t *testing.T) {
	transport := newMockTransport()
	config := &interfaces.PacketDeliveryConfig{
		NetworkTimeout:  5000,
		RetryAttempts:   3,
		EnableBroadcast: true,
	}
	pd := NewRealPacketDelivery(transport, config)

	// Inject mock sleeper to verify deterministic behavior
	sleeper := &mockSleeper{}
	pd.SetSleeper(sleeper)

	friendID := uint32(1)
	addr := &mockAddr{network: "udp", address: "127.0.0.1:33445"}
	transport.friends[friendID] = addr

	// Set error to force retries
	transport.setSendToFriendErr(errors.New("network error"))

	_ = pd.DeliverPacket(friendID, []byte("test packet"))

	// Verify sleep was called with expected durations
	// With 3 attempts, we should sleep between attempts 1->2 and 2->3
	// Durations: 500ms (attempt 0), 1000ms (attempt 1)
	sleepCalls := sleeper.getSleepCalls()
	if len(sleepCalls) != 2 {
		t.Errorf("expected 2 sleep calls, got %d", len(sleepCalls))
	}

	expectedDurations := []time.Duration{
		500 * time.Millisecond,  // 500 * (0 + 1)
		1000 * time.Millisecond, // 500 * (1 + 1)
	}
	for i, expected := range expectedDurations {
		if i < len(sleepCalls) && sleepCalls[i] != expected {
			t.Errorf("sleep call %d: expected %v, got %v", i, expected, sleepCalls[i])
		}
	}
}

func TestSetSleeper(t *testing.T) {
	transport := newMockTransport()
	pd := NewRealPacketDelivery(transport, defaultConfig())

	// Verify default sleeper is set
	if pd.sleeper == nil {
		t.Fatal("expected default sleeper to be set")
	}

	// Set custom sleeper
	customSleeper := &mockSleeper{}
	pd.SetSleeper(customSleeper)

	pd.mu.RLock()
	currentSleeper := pd.sleeper
	pd.mu.RUnlock()

	if currentSleeper != customSleeper {
		t.Error("expected custom sleeper to be set")
	}
}
