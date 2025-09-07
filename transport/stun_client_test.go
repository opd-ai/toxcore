package transport

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSTUNClient(t *testing.T) {
	client := NewSTUNClient()

	assert.NotNil(t, client)
	assert.Len(t, client.servers, 4)
	assert.Equal(t, 5*time.Second, client.timeout)
	assert.Contains(t, client.servers, "stun.l.google.com:19302")
}

func TestSTUNClient_SetServers(t *testing.T) {
	client := NewSTUNClient()
	customServers := []string{"custom.stun.server:3478"}

	client.SetServers(customServers)

	assert.Equal(t, customServers, client.servers)
}

func TestSTUNClient_SetTimeout(t *testing.T) {
	client := NewSTUNClient()
	newTimeout := 10 * time.Second

	client.SetTimeout(newTimeout)

	assert.Equal(t, newTimeout, client.timeout)
}

func TestSTUNClient_DiscoverPublicAddress_NilLocalAddr(t *testing.T) {
	client := NewSTUNClient()
	ctx := context.Background()

	addr, err := client.DiscoverPublicAddress(ctx, nil)

	assert.Nil(t, addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "local address cannot be nil")
}

func TestSTUNClient_DiscoverPublicAddress_ContextCancellation(t *testing.T) {
	client := NewSTUNClient()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel the context

	localAddr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 12345}
	addr, err := client.DiscoverPublicAddress(ctx, localAddr)

	assert.Nil(t, addr)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestSTUNClient_DiscoverPublicAddress_Timeout(t *testing.T) {
	client := NewSTUNClient()
	client.SetServers([]string{"192.0.2.1:3478"}) // RFC 5737 test address (should not respond)
	client.SetTimeout(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	localAddr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 12345}
	addr, err := client.DiscoverPublicAddress(ctx, localAddr)

	assert.Nil(t, addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all STUN servers failed")
}

func TestSTUNClient_buildBindingRequest(t *testing.T) {
	client := NewSTUNClient()
	transactionID := make([]byte, 12)
	for i := range transactionID {
		transactionID[i] = byte(i)
	}

	request := client.buildBindingRequest(transactionID)

	assert.Len(t, request, stunHeaderSize)

	// Check message type (binding request)
	messageType := (uint16(request[0]) << 8) | uint16(request[1])
	assert.Equal(t, uint16(stunBindingRequest), messageType)

	// Check message length (should be 0 for basic request)
	messageLength := (uint16(request[2]) << 8) | uint16(request[3])
	assert.Equal(t, uint16(0), messageLength)

	// Check magic cookie
	magicCookie := (uint32(request[4]) << 24) | (uint32(request[5]) << 16) |
		(uint32(request[6]) << 8) | uint32(request[7])
	assert.Equal(t, uint32(stunMagicCookie), magicCookie)

	// Check transaction ID
	assert.Equal(t, transactionID, request[8:20])
}

func TestSTUNClient_parseBindingResponse_TooShort(t *testing.T) {
	client := NewSTUNClient()
	response := make([]byte, 10) // Too short
	transactionID := make([]byte, 12)

	addr, err := client.parseBindingResponse(response, transactionID)

	assert.Nil(t, addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestSTUNClient_parseBindingResponse_ErrorResponse(t *testing.T) {
	client := NewSTUNClient()
	response := make([]byte, stunHeaderSize)
	transactionID := make([]byte, 12)

	// Set message type to error response
	response[0] = byte(stunBindingError >> 8)
	response[1] = byte(stunBindingError & 0xFF)

	// Set magic cookie
	response[4] = byte(stunMagicCookie >> 24)
	response[5] = byte((stunMagicCookie >> 16) & 0xFF)
	response[6] = byte((stunMagicCookie >> 8) & 0xFF)
	response[7] = byte(stunMagicCookie & 0xFF)

	addr, err := client.parseBindingResponse(response, transactionID)

	assert.Nil(t, addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error response")
}

func TestSTUNClient_parseBindingResponse_InvalidMagicCookie(t *testing.T) {
	client := NewSTUNClient()
	response := make([]byte, stunHeaderSize)
	transactionID := make([]byte, 12)

	// Set message type to binding response
	response[0] = byte(stunBindingResponse >> 8)
	response[1] = byte(stunBindingResponse & 0xFF)

	// Set invalid magic cookie
	response[4] = 0xFF
	response[5] = 0xFF
	response[6] = 0xFF
	response[7] = 0xFF

	addr, err := client.parseBindingResponse(response, transactionID)

	assert.Nil(t, addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid STUN magic cookie")
}

func TestSTUNClient_parseBindingResponse_TransactionIDMismatch(t *testing.T) {
	client := NewSTUNClient()
	response := make([]byte, stunHeaderSize)
	transactionID := make([]byte, 12)
	for i := range transactionID {
		transactionID[i] = byte(i)
	}

	// Set message type to binding response
	response[0] = byte(stunBindingResponse >> 8)
	response[1] = byte(stunBindingResponse & 0xFF)

	// Set magic cookie
	response[4] = byte(stunMagicCookie >> 24)
	response[5] = byte((stunMagicCookie >> 16) & 0xFF)
	response[6] = byte((stunMagicCookie >> 8) & 0xFF)
	response[7] = byte(stunMagicCookie & 0xFF)

	// Set different transaction ID
	for i := 8; i < 20; i++ {
		response[i] = 0xFF
	}

	addr, err := client.parseBindingResponse(response, transactionID)

	assert.Nil(t, addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction ID mismatch")
}

func TestSTUNClient_parseMappedAddress_IPv4(t *testing.T) {
	client := NewSTUNClient()

	// Create mapped address attribute value for IPv4 192.0.2.1:8080
	attrValue := make([]byte, 8)
	attrValue[0] = 0x00 // Family high byte
	attrValue[1] = 0x01 // Family low byte (IPv4)
	attrValue[2] = 0x1F // Port high byte (8080)
	attrValue[3] = 0x90 // Port low byte
	attrValue[4] = 192  // IP address
	attrValue[5] = 0
	attrValue[6] = 2
	attrValue[7] = 1

	addr, err := client.parseMappedAddress(attrValue)

	require.NoError(t, err)
	require.NotNil(t, addr)

	udpAddr, ok := addr.(*net.UDPAddr)
	require.True(t, ok)
	assert.Equal(t, net.IPv4(192, 0, 2, 1), udpAddr.IP)
	assert.Equal(t, 8080, udpAddr.Port)
}

func TestSTUNClient_parseMappedAddress_IPv6(t *testing.T) {
	client := NewSTUNClient()

	// Create mapped address attribute value for IPv6 [2001:db8::1]:8080
	attrValue := make([]byte, 20)
	attrValue[0] = 0x00 // Family high byte
	attrValue[1] = 0x02 // Family low byte (IPv6)
	attrValue[2] = 0x1F // Port high byte (8080)
	attrValue[3] = 0x90 // Port low byte

	// IPv6 address: 2001:db8::1
	attrValue[4] = 0x20
	attrValue[5] = 0x01
	attrValue[6] = 0x0d
	attrValue[7] = 0xb8
	// bytes 8-15 are zero
	attrValue[19] = 0x01

	addr, err := client.parseMappedAddress(attrValue)

	require.NoError(t, err)
	require.NotNil(t, addr)

	udpAddr, ok := addr.(*net.UDPAddr)
	require.True(t, ok)
	expectedIP := net.ParseIP("2001:db8::1")
	assert.True(t, udpAddr.IP.Equal(expectedIP))
	assert.Equal(t, 8080, udpAddr.Port)
}

func TestSTUNClient_parseMappedAddress_UnsupportedFamily(t *testing.T) {
	client := NewSTUNClient()

	attrValue := make([]byte, 8)
	attrValue[0] = 0x00
	attrValue[1] = 0x03 // Unsupported family

	addr, err := client.parseMappedAddress(attrValue)

	assert.Nil(t, addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported address family")
}

func TestSTUNClient_parseXorMappedAddress_IPv4(t *testing.T) {
	client := NewSTUNClient()
	transactionID := make([]byte, 12)

	// Create XOR mapped address for 192.0.2.1:8080
	// XOR with magic cookie
	attrValue := make([]byte, 8)
	attrValue[0] = 0x00 // Family high byte
	attrValue[1] = 0x01 // Family low byte (IPv4)

	// Port 8080 XOR with upper 16 bits of magic cookie
	xorPort := uint16(8080) ^ uint16(stunMagicCookie>>16)
	attrValue[2] = byte(xorPort >> 8)
	attrValue[3] = byte(xorPort & 0xFF)

	// IP 192.0.2.1 XOR with magic cookie
	originalIP := uint32(192<<24 | 0<<16 | 2<<8 | 1)
	xorIP := originalIP ^ stunMagicCookie
	attrValue[4] = byte(xorIP >> 24)
	attrValue[5] = byte(xorIP >> 16)
	attrValue[6] = byte(xorIP >> 8)
	attrValue[7] = byte(xorIP & 0xFF)

	addr, err := client.parseXorMappedAddress(attrValue, transactionID)

	require.NoError(t, err)
	require.NotNil(t, addr)

	udpAddr, ok := addr.(*net.UDPAddr)
	require.True(t, ok)
	assert.Equal(t, net.IPv4(192, 0, 2, 1), udpAddr.IP)
	assert.Equal(t, 8080, udpAddr.Port)
}

func TestSTUNClient_parseXorMappedAddress_UnsupportedFamily(t *testing.T) {
	client := NewSTUNClient()
	transactionID := make([]byte, 12)

	attrValue := make([]byte, 8)
	attrValue[0] = 0x00
	attrValue[1] = 0x03 // Unsupported family

	addr, err := client.parseXorMappedAddress(attrValue, transactionID)

	assert.Nil(t, addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported address family")
}

// Integration test that attempts to connect to a real STUN server
// This test is marked as integration and may be skipped in CI
func TestSTUNClient_Integration_RealServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewSTUNClient()
	client.SetTimeout(10 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	addr, err := client.DiscoverPublicAddress(ctx, localAddr)

	// This test may fail in environments without internet access
	// or where STUN servers are blocked, so we only log failures
	if err != nil {
		t.Logf("STUN integration test failed (expected in some environments): %v", err)
		return
	}

	t.Logf("Discovered public address: %v", addr)
	assert.NotNil(t, addr)

	// Verify the returned address is a valid UDP address
	udpAddr, ok := addr.(*net.UDPAddr)
	assert.True(t, ok)
	assert.NotNil(t, udpAddr.IP)
	assert.True(t, udpAddr.Port > 0)
}

// Benchmark STUN request building
func BenchmarkSTUNClient_buildBindingRequest(b *testing.B) {
	client := NewSTUNClient()
	transactionID := make([]byte, 12)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.buildBindingRequest(transactionID)
	}
}

// Benchmark mapped address parsing
func BenchmarkSTUNClient_parseMappedAddress(b *testing.B) {
	client := NewSTUNClient()

	// Create test mapped address
	attrValue := make([]byte, 8)
	attrValue[0] = 0x00
	attrValue[1] = 0x01 // IPv4
	attrValue[2] = 0x1F
	attrValue[3] = 0x90 // Port 8080
	attrValue[4] = 192
	attrValue[5] = 0
	attrValue[6] = 2
	attrValue[7] = 1

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.parseMappedAddress(attrValue)
	}
}
