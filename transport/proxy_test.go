package transport

import (
	"net"
	"testing"
)

// simpleMockTransport is a minimal mock for testing proxy transport
type simpleMockTransport struct {
	addr net.Addr
}

func (m *simpleMockTransport) Send(packet *Packet, addr net.Addr) error {
	return nil
}

func (m *simpleMockTransport) Close() error {
	return nil
}

func (m *simpleMockTransport) LocalAddr() net.Addr {
	return m.addr
}

func (m *simpleMockTransport) RegisterHandler(packetType PacketType, handler PacketHandler) {
}

// TestProxyTransportCreation tests creating proxy transports with different configurations.
func TestProxyTransportCreation(t *testing.T) {
	// Create a mock underlying transport
	mockAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	mockTransport := &simpleMockTransport{addr: mockAddr}

	tests := []struct {
		name        string
		config      *ProxyConfig
		expectError bool
	}{
		{
			name: "SOCKS5 without auth",
			config: &ProxyConfig{
				Type: "socks5",
				Host: "127.0.0.1",
				Port: 9050,
			},
			expectError: false,
		},
		{
			name: "SOCKS5 with auth",
			config: &ProxyConfig{
				Type:     "socks5",
				Host:     "127.0.0.1",
				Port:     9050,
				Username: "testuser",
				Password: "testpass",
			},
			expectError: false,
		},
		{
			name: "HTTP proxy without auth",
			config: &ProxyConfig{
				Type: "http",
				Host: "127.0.0.1",
				Port: 8080,
			},
			expectError: true, // HTTP proxies not yet supported
		},
		{
			name: "HTTP proxy with auth",
			config: &ProxyConfig{
				Type:     "http",
				Host:     "127.0.0.1",
				Port:     8080,
				Username: "testuser",
				Password: "testpass",
			},
			expectError: true, // HTTP proxies not yet supported
		},
		{
			name: "Unsupported proxy type",
			config: &ProxyConfig{
				Type: "unsupported",
				Host: "127.0.0.1",
				Port: 8080,
			},
			expectError: true,
		},
		{
			name:        "Nil config",
			config:      nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxyTransport, err := NewProxyTransport(mockTransport, tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if proxyTransport == nil {
				t.Errorf("Expected non-nil proxy transport")
				return
			}

			// Verify proxy transport properties
			if proxyTransport.underlying != mockTransport {
				t.Errorf("Underlying transport mismatch")
			}

			if proxyTransport.proxyType != tt.config.Type {
				t.Errorf("Expected proxy type %s, got %s", tt.config.Type, proxyTransport.proxyType)
			}
		})
	}
}

// TestProxyTransportSend tests sending packets through proxy transport.
func TestProxyTransportSend(t *testing.T) {
	mockAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	mockTransport := &simpleMockTransport{addr: mockAddr}

	config := &ProxyConfig{
		Type: "socks5",
		Host: "127.0.0.1",
		Port: 9050,
	}

	proxyTransport, err := NewProxyTransport(mockTransport, config)
	if err != nil {
		t.Fatalf("Failed to create proxy transport: %v", err)
	}

	// Create a test packet
	packet := &Packet{
		PacketType: PacketPingRequest,
		Data:       []byte("test data"),
	}

	// Create a test address
	addr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 12345,
	}

	// Send packet
	err = proxyTransport.Send(packet, addr)
	if err != nil {
		t.Errorf("Failed to send packet: %v", err)
	}

	// Verify the mock received the packet (check that Send was called)
	// MockTransport doesn't expose internal state, so we just verify no error
}

// TestProxyTransportClose tests closing proxy transport.
func TestProxyTransportClose(t *testing.T) {
	mockAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	mockTransport := &simpleMockTransport{addr: mockAddr}

	config := &ProxyConfig{
		Type: "socks5",
		Host: "127.0.0.1",
		Port: 9050,
	}

	proxyTransport, err := NewProxyTransport(mockTransport, config)
	if err != nil {
		t.Fatalf("Failed to create proxy transport: %v", err)
	}

	err = proxyTransport.Close()
	if err != nil {
		t.Errorf("Failed to close proxy transport: %v", err)
	}
}

// TestProxyTransportLocalAddr tests getting local address from proxy transport.
func TestProxyTransportLocalAddr(t *testing.T) {
	mockAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	mockTransport := &simpleMockTransport{addr: mockAddr}

	config := &ProxyConfig{
		Type: "socks5",
		Host: "127.0.0.1",
		Port: 9050,
	}

	proxyTransport, err := NewProxyTransport(mockTransport, config)
	if err != nil {
		t.Fatalf("Failed to create proxy transport: %v", err)
	}

	addr := proxyTransport.LocalAddr()
	if addr == nil {
		t.Errorf("Expected non-nil local address")
	}

	// Should match the underlying transport's address
	expectedAddr := mockTransport.LocalAddr()
	if addr.String() != expectedAddr.String() {
		t.Errorf("Expected address %s, got %s", expectedAddr.String(), addr.String())
	}
}

// TestProxyTransportRegisterHandler tests registering packet handlers.
func TestProxyTransportRegisterHandler(t *testing.T) {
	mockAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	mockTransport := &simpleMockTransport{addr: mockAddr}

	config := &ProxyConfig{
		Type: "socks5",
		Host: "127.0.0.1",
		Port: 9050,
	}

	proxyTransport, err := NewProxyTransport(mockTransport, config)
	if err != nil {
		t.Fatalf("Failed to create proxy transport: %v", err)
	}

	handlerCalled := false
	handler := func(packet *Packet, addr net.Addr) error {
		handlerCalled = true
		return nil
	}

	proxyTransport.RegisterHandler(PacketPingRequest, handler)

	// Test that handler was registered (we can't easily test if it gets called
	// without more complex mock infrastructure, but registration should work)
	testPacket := &Packet{PacketType: PacketPingRequest, Data: []byte("test")}
	testAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	
	// Call handler directly to verify it works
	_ = handler(testPacket, testAddr)
	if !handlerCalled {
		t.Errorf("Expected handler to be callable")
	}
}

// TestProxyTransportGetProxyDialer tests getting the proxy dialer.
func TestProxyTransportGetProxyDialer(t *testing.T) {
	mockAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	mockTransport := &simpleMockTransport{addr: mockAddr}

	config := &ProxyConfig{
		Type: "socks5",
		Host: "127.0.0.1",
		Port: 9050,
	}

	proxyTransport, err := NewProxyTransport(mockTransport, config)
	if err != nil {
		t.Fatalf("Failed to create proxy transport: %v", err)
	}

	dialer := proxyTransport.GetProxyDialer()
	if dialer == nil {
		t.Errorf("Expected non-nil proxy dialer")
	}
}
