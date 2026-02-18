package transport

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestNewRelayClient(t *testing.T) {
	var publicKey [32]byte
	copy(publicKey[:], []byte("test-public-key-for-relay-client"))

	client := NewRelayClient(publicKey)
	if client == nil {
		t.Fatal("NewRelayClient returned nil")
	}

	if client.GetState() != RelayStateDisconnected {
		t.Errorf("expected initial state Disconnected, got %v", client.GetState())
	}

	if client.GetServerCount() != 0 {
		t.Errorf("expected 0 servers, got %d", client.GetServerCount())
	}

	if err := client.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestRelayClient_AddRemoveServer(t *testing.T) {
	client := NewRelayClient([32]byte{})
	defer client.Close()

	server1 := RelayServerInfo{
		Address:  "relay1.example.com",
		Port:     33445,
		Priority: 1,
	}
	server2 := RelayServerInfo{
		Address:  "relay2.example.com",
		Port:     33446,
		Priority: 2,
	}

	client.AddRelayServer(server1)
	if client.GetServerCount() != 1 {
		t.Errorf("expected 1 server, got %d", client.GetServerCount())
	}

	client.AddRelayServer(server2)
	if client.GetServerCount() != 2 {
		t.Errorf("expected 2 servers, got %d", client.GetServerCount())
	}

	client.RemoveRelayServer("relay1.example.com")
	if client.GetServerCount() != 1 {
		t.Errorf("expected 1 server after removal, got %d", client.GetServerCount())
	}

	// Removing non-existent server should be a no-op
	client.RemoveRelayServer("nonexistent.example.com")
	if client.GetServerCount() != 1 {
		t.Errorf("expected 1 server, got %d", client.GetServerCount())
	}
}

func TestRelayClient_ConnectNoServers(t *testing.T) {
	client := NewRelayClient([32]byte{})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err == nil {
		t.Fatal("expected error when connecting with no servers")
	}

	if client.GetState() != RelayStateFailed {
		t.Errorf("expected state Failed, got %v", client.GetState())
	}
}

func TestRelayClient_SetTimeout(t *testing.T) {
	client := NewRelayClient([32]byte{})
	defer client.Close()

	newTimeout := 20 * time.Second
	client.SetTimeout(newTimeout)

	if client.timeout != newTimeout {
		t.Errorf("expected timeout %v, got %v", newTimeout, client.timeout)
	}
}

func TestRelayClient_IsConnected(t *testing.T) {
	client := NewRelayClient([32]byte{})
	defer client.Close()

	if client.IsConnected() {
		t.Error("expected IsConnected to return false initially")
	}
}

func TestRelayClient_GetActiveServerNil(t *testing.T) {
	client := NewRelayClient([32]byte{})
	defer client.Close()

	if client.GetActiveServer() != nil {
		t.Error("expected nil active server when not connected")
	}
}

func TestRelayClient_SetDataHandler(t *testing.T) {
	client := NewRelayClient([32]byte{})
	defer client.Close()

	client.SetDataHandler(func(p *Packet, addr net.Addr) error {
		return nil
	})

	if client.dataHandler == nil {
		t.Error("expected data handler to be set")
	}
}

func TestRelayClient_RelayToNotConnected(t *testing.T) {
	client := NewRelayClient([32]byte{})
	defer client.Close()

	packet := &Packet{
		PacketType: PacketPingRequest,
		Data:       []byte("test"),
	}

	var targetKey [32]byte
	err := client.RelayTo(packet, targetKey)
	if err == nil {
		t.Fatal("expected error when relaying while not connected")
	}
}

func TestRelayClient_ServerPrioritySorting(t *testing.T) {
	client := NewRelayClient([32]byte{})
	defer client.Close()

	// Add servers in non-priority order
	client.AddRelayServer(RelayServerInfo{Address: "third.example.com", Port: 33445, Priority: 3})
	client.AddRelayServer(RelayServerInfo{Address: "first.example.com", Port: 33445, Priority: 1})
	client.AddRelayServer(RelayServerInfo{Address: "second.example.com", Port: 33445, Priority: 2})

	sorted := client.getServersByPriority()
	if len(sorted) != 3 {
		t.Fatalf("expected 3 servers, got %d", len(sorted))
	}

	if sorted[0].Priority != 1 {
		t.Errorf("first server should have priority 1, got %d", sorted[0].Priority)
	}
	if sorted[1].Priority != 2 {
		t.Errorf("second server should have priority 2, got %d", sorted[1].Priority)
	}
	if sorted[2].Priority != 3 {
		t.Errorf("third server should have priority 3, got %d", sorted[2].Priority)
	}
}

func TestRelayedAddress(t *testing.T) {
	addr := &RelayedAddress{
		RelayServer: "relay.example.com",
		SourceKey:   make([]byte, 32),
	}
	copy(addr.SourceKey, []byte("abcdefghijklmnopqrstuvwxyz012345"))

	if addr.Network() != "relay" {
		t.Errorf("expected network 'relay', got '%s'", addr.Network())
	}

	str := addr.String()
	if str == "" {
		t.Error("expected non-empty string representation")
	}
}

func TestRelayState_Constants(t *testing.T) {
	// Verify relay state constants have expected values
	if RelayStateDisconnected != 0 {
		t.Errorf("expected RelayStateDisconnected=0, got %d", RelayStateDisconnected)
	}
	if RelayStateConnecting != 1 {
		t.Errorf("expected RelayStateConnecting=1, got %d", RelayStateConnecting)
	}
	if RelayStateConnected != 2 {
		t.Errorf("expected RelayStateConnected=2, got %d", RelayStateConnected)
	}
	if RelayStateFailed != 3 {
		t.Errorf("expected RelayStateFailed=3, got %d", RelayStateFailed)
	}
}

func TestRelayPacketType_Constants(t *testing.T) {
	// Verify relay packet type constants
	if RelayPacketRouting != 0x00 {
		t.Errorf("expected RelayPacketRouting=0x00, got %#x", RelayPacketRouting)
	}
	if RelayPacketData != 0x01 {
		t.Errorf("expected RelayPacketData=0x01, got %#x", RelayPacketData)
	}
	if RelayPacketPing != 0x02 {
		t.Errorf("expected RelayPacketPing=0x02, got %#x", RelayPacketPing)
	}
	if RelayPacketPong != 0x03 {
		t.Errorf("expected RelayPacketPong=0x03, got %#x", RelayPacketPong)
	}
	if RelayPacketDisconnect != 0x04 {
		t.Errorf("expected RelayPacketDisconnect=0x04, got %#x", RelayPacketDisconnect)
	}
}

// TestAdvancedNATTraversal_RelayIntegration tests relay integration with advanced NAT traversal
func TestAdvancedNATTraversal_RelayIntegration(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	var publicKey [32]byte
	copy(publicKey[:], []byte("test-public-key-for-nat-traversal"))

	ant, err := NewAdvancedNATTraversalWithKey(localAddr, publicKey)
	if err != nil {
		t.Fatalf("NewAdvancedNATTraversalWithKey failed: %v", err)
	}
	defer ant.Close()

	// Verify relay client is initialized
	if ant.GetRelayClient() == nil {
		t.Fatal("expected relay client to be initialized")
	}

	// Verify relay is initially not connected
	if ant.IsRelayConnected() {
		t.Error("expected relay to not be connected initially")
	}

	// Add a relay server
	server := RelayServerInfo{
		Address:  "test-relay.example.com",
		Port:     33445,
		Priority: 1,
	}
	ant.AddRelayServer(server)

	if ant.GetRelayClient().GetServerCount() != 1 {
		t.Errorf("expected 1 relay server, got %d", ant.GetRelayClient().GetServerCount())
	}

	// Remove the relay server
	ant.RemoveRelayServer("test-relay.example.com")
	if ant.GetRelayClient().GetServerCount() != 0 {
		t.Errorf("expected 0 relay servers after removal, got %d", ant.GetRelayClient().GetServerCount())
	}
}

// TestAdvancedNATTraversal_RelayConnectionAttempt tests relay connection attempt
func TestAdvancedNATTraversal_RelayConnectionAttempt(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}

	ant, err := NewAdvancedNATTraversal(localAddr)
	if err != nil {
		t.Fatalf("NewAdvancedNATTraversal failed: %v", err)
	}
	defer ant.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	remoteAddr := &net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 33445}

	// Attempt relay connection without servers should fail
	err = ant.attemptRelayConnection(ctx, remoteAddr)
	if err == nil {
		t.Error("expected error when attempting relay connection without servers")
	}
}

// TestAdvancedNATTraversal_EnableRelayMethod tests enabling relay method
func TestAdvancedNATTraversal_EnableRelayMethod(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}

	ant, err := NewAdvancedNATTraversal(localAddr)
	if err != nil {
		t.Fatalf("NewAdvancedNATTraversal failed: %v", err)
	}
	defer ant.Close()

	// Relay should be disabled by default
	if ant.isMethodEnabled(ConnectionRelay) {
		t.Error("expected relay to be disabled by default")
	}

	// Enable relay method
	ant.EnableMethod(ConnectionRelay, true)
	if !ant.isMethodEnabled(ConnectionRelay) {
		t.Error("expected relay to be enabled after EnableMethod call")
	}

	// Disable relay method
	ant.EnableMethod(ConnectionRelay, false)
	if ant.isMethodEnabled(ConnectionRelay) {
		t.Error("expected relay to be disabled after EnableMethod(false) call")
	}
}

// TestRelayClient_CloseTwice tests that closing relay client twice doesn't panic
func TestRelayClient_CloseTwice(t *testing.T) {
	client := NewRelayClient([32]byte{})

	if err := client.Close(); err != nil {
		t.Errorf("first Close failed: %v", err)
	}

	// Second close should not panic
	if err := client.Close(); err != nil {
		t.Errorf("second Close failed: %v", err)
	}
}

// TestAdvancedNATTraversal_CloseWithRelay tests proper cleanup of relay resources
func TestAdvancedNATTraversal_CloseWithRelay(t *testing.T) {
	localAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}

	ant, err := NewAdvancedNATTraversal(localAddr)
	if err != nil {
		t.Fatalf("NewAdvancedNATTraversal failed: %v", err)
	}

	// Add a relay server
	ant.AddRelayServer(RelayServerInfo{
		Address:  "relay.example.com",
		Port:     33445,
		Priority: 1,
	})

	// Close should clean up all resources
	if err := ant.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}
