package av

import (
	"testing"
	"time"
)

// mockTransport implements TransportInterface for testing
type mockTransport struct {
	handlers    map[byte]func([]byte, []byte) error
	sentPackets []mockPacket
}

type mockPacket struct {
	packetType byte
	data       []byte
	addr       []byte
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		handlers:    make(map[byte]func([]byte, []byte) error),
		sentPackets: make([]mockPacket, 0),
	}
}

func (m *mockTransport) Send(packetType byte, data, addr []byte) error {
	m.sentPackets = append(m.sentPackets, mockPacket{
		packetType: packetType,
		data:       append([]byte(nil), data...),
		addr:       append([]byte(nil), addr...),
	})
	return nil
}

func (m *mockTransport) RegisterHandler(packetType byte, handler func([]byte, []byte) error) {
	m.handlers[packetType] = handler
}

func (m *mockTransport) simulatePacket(packetType byte, data, addr []byte) error {
	if handler, exists := m.handlers[packetType]; exists {
		return handler(data, addr)
	}
	return nil
}

// mockFriendLookup provides mock friend address lookup for testing
func mockFriendLookup(friendNumber uint32) ([]byte, error) {
	// Return a simple address based on friend number
	return []byte{byte(friendNumber), 0, 0, 0}, nil
}

// TestNewManagerWithTransport verifies that NewManager creates a manager with transport integration.
func TestNewManagerWithTransport(t *testing.T) {
	transport := newMockTransport()
	manager, err := NewManager(transport, mockFriendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	if manager.IsRunning() {
		t.Error("Manager should not be running initially")
	}

	// Test that packet handlers are registered
	// We should have 6 handlers:
	// - 0x30: CallRequest
	// - 0x31: CallResponse
	// - 0x32: CallControl
	// - 0x33: AudioFrame
	// - 0x34: VideoFrame
	// - 0x35: BitrateControl
	if len(transport.handlers) != 6 {
		t.Errorf("Expected 6 packet handlers, got %d", len(transport.handlers))
	}

	// Check for specific packet type handlers
	expectedHandlers := []byte{0x30, 0x31, 0x32, 0x33, 0x34, 0x35} // AV packet types
	for _, packetType := range expectedHandlers {
		if _, exists := transport.handlers[packetType]; !exists {
			t.Errorf("Handler for packet type 0x%02x not registered", packetType)
		}
	}
}

// TestManagerLifecycle verifies manager start/stop lifecycle.
func TestManagerLifecycle(t *testing.T) {
	transport := newMockTransport()
	manager, err := NewManager(transport, mockFriendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Test starting
	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	if !manager.IsRunning() {
		t.Error("Manager should be running after Start()")
	}

	// Test stopping
	err = manager.Stop()
	if err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}

	if manager.IsRunning() {
		t.Error("Manager should not be running after Stop()")
	}
}

// TestStartCall verifies that starting a call works with transport integration.
func TestStartCall(t *testing.T) {
	transport := newMockTransport()
	manager, err := NewManager(transport, mockFriendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	friendNumber := uint32(1)
	audioBitRate := uint32(48000)
	videoBitRate := uint32(500000)

	// Start a call
	err = manager.StartCall(friendNumber, audioBitRate, videoBitRate)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// Verify call was created
	call := manager.GetCall(friendNumber)
	if call == nil {
		t.Fatal("Call should exist after StartCall()")
	}

	if call.GetFriendNumber() != friendNumber {
		t.Errorf("Expected friend number %d, got %d", friendNumber, call.GetFriendNumber())
	}

	// Verify packet was sent
	if len(transport.sentPackets) != 1 {
		t.Fatalf("Expected 1 sent packet, got %d", len(transport.sentPackets))
	}

	packet := transport.sentPackets[0]
	if packet.packetType != 0x30 { // PacketAVCallRequest
		t.Errorf("Expected packet type 0x30, got 0x%02x", packet.packetType)
	}

	// Verify call count
	if manager.GetCallCount() != 1 {
		t.Errorf("Expected 1 active call, got %d", manager.GetCallCount())
	}
}

// TestAnswerCall verifies that answering a call works with transport integration.
func TestAnswerCall(t *testing.T) {
	transport := newMockTransport()
	manager, err := NewManager(transport, mockFriendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	friendNumber := uint32(1)
	callID := uint32(123)

	// Simulate incoming call request
	req := &CallRequestPacket{
		CallID:       callID,
		AudioBitRate: 48000,
		VideoBitRate: 500000,
		Timestamp:    time.Now(),
	}

	data, err := SerializeCallRequest(req)
	if err != nil {
		t.Fatalf("Failed to serialize call request: %v", err)
	}

	// Simulate packet reception
	addr := []byte{byte(friendNumber), 0, 0, 0}
	err = transport.simulatePacket(0x30, data, addr)
	if err != nil {
		t.Fatalf("Failed to simulate call request: %v", err)
	}

	// Verify incoming call was created
	call := manager.GetCall(friendNumber)
	if call == nil {
		t.Fatal("Call should exist after receiving call request")
	}

	// Answer the call
	err = manager.AnswerCall(friendNumber, 48000, 500000)
	if err != nil {
		t.Fatalf("Failed to answer call: %v", err)
	}

	// Verify response packet was sent
	if len(transport.sentPackets) != 1 {
		t.Fatalf("Expected 1 sent packet, got %d", len(transport.sentPackets))
	}

	packet := transport.sentPackets[0]
	if packet.packetType != 0x31 { // PacketAVCallResponse
		t.Errorf("Expected packet type 0x31, got 0x%02x", packet.packetType)
	}
}

// TestEndCall verifies that ending a call works with transport integration.
func TestEndCall(t *testing.T) {
	transport := newMockTransport()
	manager, err := NewManager(transport, mockFriendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	friendNumber := uint32(1)

	// Start a call first
	err = manager.StartCall(friendNumber, 48000, 500000)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	// Clear sent packets from StartCall
	transport.sentPackets = nil

	// End the call
	err = manager.EndCall(friendNumber)
	if err != nil {
		t.Fatalf("Failed to end call: %v", err)
	}

	// Verify call was removed
	call := manager.GetCall(friendNumber)
	if call != nil {
		t.Error("Call should not exist after EndCall()")
	}

	// Verify control packet was sent
	if len(transport.sentPackets) != 1 {
		t.Fatalf("Expected 1 sent packet, got %d", len(transport.sentPackets))
	}

	packet := transport.sentPackets[0]
	if packet.packetType != 0x32 { // PacketAVCallControl
		t.Errorf("Expected packet type 0x32, got 0x%02x", packet.packetType)
	}

	// Verify call count
	if manager.GetCallCount() != 0 {
		t.Errorf("Expected 0 active calls, got %d", manager.GetCallCount())
	}
}

// TestIteration verifies that the manager iteration works correctly.
func TestIteration(t *testing.T) {
	transport := newMockTransport()
	manager, err := NewManager(transport, mockFriendLookup)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Test iteration interval
	interval := manager.IterationInterval()
	expectedInterval := 20 * time.Millisecond
	if interval != expectedInterval {
		t.Errorf("Expected iteration interval %v, got %v", expectedInterval, interval)
	}

	// Test iteration doesn't crash
	manager.Iterate()

	// Start a call and iterate
	friendNumber := uint32(1)
	err = manager.StartCall(friendNumber, 48000, 0)
	if err != nil {
		t.Fatalf("Failed to start call: %v", err)
	}

	manager.Iterate()

	// Verify call still exists
	call := manager.GetCall(friendNumber)
	if call == nil {
		t.Error("Call should still exist after iteration")
	}
}

// TestNilParameters verifies that NewManager rejects nil parameters.
func TestNilParameters(t *testing.T) {
	// Test nil transport
	_, err := NewManager(nil, mockFriendLookup)
	if err == nil {
		t.Error("Expected error for nil transport")
	}

	// Test nil friend lookup
	transport := newMockTransport()
	_, err = NewManager(transport, nil)
	if err == nil {
		t.Error("Expected error for nil friend lookup function")
	}
}
