package transport

import (
	"bytes"
	"crypto/rand"
	"net"
	"testing"
	"time"
)

func TestSerializeVersionedHandshakeRequest(t *testing.T) {
	tests := []struct {
		name    string
		request *VersionedHandshakeRequest
		wantErr bool
	}{
		{
			name: "valid request with noise message",
			request: &VersionedHandshakeRequest{
				ProtocolVersion:   ProtocolNoiseIK,
				SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
				NoiseMessage:      []byte("test noise message"),
				LegacyData:        []byte("legacy data"),
			},
			wantErr: false,
		},
		{
			name: "valid request without noise message",
			request: &VersionedHandshakeRequest{
				ProtocolVersion:   ProtocolLegacy,
				SupportedVersions: []ProtocolVersion{ProtocolLegacy},
				NoiseMessage:      nil,
				LegacyData:        []byte("legacy only"),
			},
			wantErr: false,
		},
		{
			name:    "nil request",
			request: nil,
			wantErr: true,
		},
		{
			name: "no supported versions",
			request: &VersionedHandshakeRequest{
				ProtocolVersion:   ProtocolLegacy,
				SupportedVersions: []ProtocolVersion{},
				NoiseMessage:      nil,
				LegacyData:        nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := SerializeVersionedHandshakeRequest(tt.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("SerializeVersionedHandshakeRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			// Test that we can parse what we serialize
			parsed, err := ParseVersionedHandshakeRequest(data)
			if err != nil {
				t.Errorf("Failed to parse serialized request: %v", err)
				return
			}

			// Verify the parsed data matches the original
			if parsed.ProtocolVersion != tt.request.ProtocolVersion {
				t.Errorf("ProtocolVersion mismatch: got %v, want %v", parsed.ProtocolVersion, tt.request.ProtocolVersion)
			}

			if len(parsed.SupportedVersions) != len(tt.request.SupportedVersions) {
				t.Errorf("SupportedVersions length mismatch: got %v, want %v", len(parsed.SupportedVersions), len(tt.request.SupportedVersions))
			}

			for i, version := range tt.request.SupportedVersions {
				if parsed.SupportedVersions[i] != version {
					t.Errorf("SupportedVersions[%d] mismatch: got %v, want %v", i, parsed.SupportedVersions[i], version)
				}
			}

			if !bytes.Equal(parsed.NoiseMessage, tt.request.NoiseMessage) {
				t.Errorf("NoiseMessage mismatch: got %v, want %v", parsed.NoiseMessage, tt.request.NoiseMessage)
			}

			if !bytes.Equal(parsed.LegacyData, tt.request.LegacyData) {
				t.Errorf("LegacyData mismatch: got %v, want %v", parsed.LegacyData, tt.request.LegacyData)
			}
		})
	}
}

func TestSerializeVersionedHandshakeResponse(t *testing.T) {
	tests := []struct {
		name     string
		response *VersionedHandshakeResponse
		wantErr  bool
	}{
		{
			name: "valid response with noise message",
			response: &VersionedHandshakeResponse{
				AgreedVersion: ProtocolNoiseIK,
				NoiseMessage:  []byte("noise response"),
				LegacyData:    []byte("legacy response"),
			},
			wantErr: false,
		},
		{
			name: "valid response legacy only",
			response: &VersionedHandshakeResponse{
				AgreedVersion: ProtocolLegacy,
				NoiseMessage:  nil,
				LegacyData:    []byte("legacy only response"),
			},
			wantErr: false,
		},
		{
			name:     "nil response",
			response: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := SerializeVersionedHandshakeResponse(tt.response)
			if (err != nil) != tt.wantErr {
				t.Errorf("SerializeVersionedHandshakeResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			// Test that we can parse what we serialize
			parsed, err := ParseVersionedHandshakeResponse(data)
			if err != nil {
				t.Errorf("Failed to parse serialized response: %v", err)
				return
			}

			// Verify the parsed data matches the original
			if parsed.AgreedVersion != tt.response.AgreedVersion {
				t.Errorf("AgreedVersion mismatch: got %v, want %v", parsed.AgreedVersion, tt.response.AgreedVersion)
			}

			if !bytes.Equal(parsed.NoiseMessage, tt.response.NoiseMessage) {
				t.Errorf("NoiseMessage mismatch: got %v, want %v", parsed.NoiseMessage, tt.response.NoiseMessage)
			}

			if !bytes.Equal(parsed.LegacyData, tt.response.LegacyData) {
				t.Errorf("LegacyData mismatch: got %v, want %v", parsed.LegacyData, tt.response.LegacyData)
			}
		})
	}
}

func TestParseVersionedHandshakeRequest_InvalidData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty data", []byte{}},
		{"too short", []byte{0x01}},
		{"invalid noise length", []byte{0x01, 0x01, 0x00, 0xFF, 0xFF}}, // Claims 65535 byte noise message
		{"truncated", []byte{0x01, 0x02, 0x00, 0x01, 0x00, 0x05}},      // Claims 5 byte noise message but no data
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseVersionedHandshakeRequest(tt.data)
			if err == nil {
				t.Errorf("ParseVersionedHandshakeRequest() should have failed for invalid data")
			}
		})
	}
}

func TestParseVersionedHandshakeResponse_InvalidData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty data", []byte{}},
		{"too short", []byte{0x01}},
		{"invalid noise length", []byte{0x01, 0xFF, 0xFF}}, // Claims 65535 byte noise message
		{"truncated", []byte{0x01, 0x00, 0x05}},            // Claims 5 byte noise message but no data
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseVersionedHandshakeResponse(tt.data)
			if err == nil {
				t.Errorf("ParseVersionedHandshakeResponse() should have failed for invalid data")
			}
		})
	}
}

func TestVersionedHandshakeManager(t *testing.T) {
	// Generate test key pairs
	var staticPrivKey1, staticPrivKey2 [32]byte
	rand.Read(staticPrivKey1[:])
	rand.Read(staticPrivKey2[:])

	// Get corresponding public keys (this would use actual crypto in real implementation)
	var staticPubKey1, staticPubKey2 [32]byte
	// For testing, we'll use the private key as public key (not secure, just for testing)
	copy(staticPubKey1[:], staticPrivKey1[:])
	copy(staticPubKey2[:], staticPrivKey2[:])

	supportedVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}
	preferredVersion := ProtocolNoiseIK

	manager1 := NewVersionedHandshakeManager(staticPrivKey1, supportedVersions, preferredVersion)

	// Test version selection
	peerVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}
	bestVersion := manager1.selectBestVersion(peerVersions)
	if bestVersion != ProtocolNoiseIK {
		t.Errorf("Expected best version to be ProtocolNoiseIK, got %v", bestVersion)
	}

	// Test version support check
	if !manager1.isVersionSupported(ProtocolNoiseIK) {
		t.Errorf("Expected manager to support ProtocolNoiseIK")
	}

	if manager1.isVersionSupported(ProtocolVersion(99)) {
		t.Errorf("Expected manager to not support unknown protocol version")
	}

	// Test handshake request creation (mock transport)
	mockTransport := NewMockTransport("127.0.0.1:8080")

	// Test that handshake request is sent properly
	// We'll use a goroutine and simulate a response
	peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8081")

	// Set a short timeout for this test
	manager1.handshakeTimeout = 1 * time.Second

	// Start handshake in background
	responseChan := make(chan *VersionedHandshakeResponse)
	errChan := make(chan error)

	go func() {
		resp, err := manager1.InitiateHandshake(staticPubKey2, mockTransport, peerAddr)
		if err != nil {
			errChan <- err
			return
		}
		responseChan <- resp
	}()

	// Give time for the handshake to send the request
	time.Sleep(50 * time.Millisecond)

	// Verify a packet was sent
	packets := mockTransport.GetPackets()
	if len(packets) != 1 {
		t.Errorf("Expected 1 packet to be sent, got %d", len(packets))
	}

	if packets[0].packet.PacketType != PacketNoiseHandshake {
		t.Errorf("Expected PacketNoiseHandshake, got %v", packets[0].packet.PacketType)
	}

	// Simulate receiving a response
	mockResponse := &VersionedHandshakeResponse{
		AgreedVersion: ProtocolLegacy,
		NoiseMessage:  nil,
		LegacyData:    []byte{},
	}

	responseData, err := SerializeVersionedHandshakeResponse(mockResponse)
	if err != nil {
		t.Fatalf("Failed to serialize response: %v", err)
	}

	responsePacket := &Packet{
		PacketType: PacketNoiseHandshake,
		Data:       responseData,
	}

	// Simulate receiving the response
	err = mockTransport.SimulateReceive(responsePacket, peerAddr)
	if err != nil {
		t.Fatalf("Failed to simulate response: %v", err)
	}

	// Wait for handshake to complete
	select {
	case resp := <-responseChan:
		if resp.AgreedVersion != ProtocolLegacy {
			t.Errorf("Expected agreed version to be ProtocolLegacy, got %v", resp.AgreedVersion)
		}
	case err := <-errChan:
		t.Errorf("InitiateHandshake failed: %v", err)
	case <-time.After(2 * time.Second):
		t.Error("Handshake did not complete within timeout")
	}
}

func TestVersionedHandshakeManager_HandleHandshakeRequest(t *testing.T) {
	var staticPrivKey [32]byte
	rand.Read(staticPrivKey[:])

	supportedVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}
	preferredVersion := ProtocolNoiseIK

	manager := NewVersionedHandshakeManager(staticPrivKey, supportedVersions, preferredVersion)
	mockTransport := NewMockTransport("127.0.0.1:8080")

	tests := []struct {
		name            string
		request         *VersionedHandshakeRequest
		expectedVersion ProtocolVersion
		wantErr         bool
	}{
		{
			name: "noise-ik request",
			request: &VersionedHandshakeRequest{
				ProtocolVersion:   ProtocolNoiseIK,
				SupportedVersions: []ProtocolVersion{ProtocolNoiseIK},
				NoiseMessage:      make([]byte, 32), // Mock noise message (will fail crypto validation)
				LegacyData:        nil,
			},
			expectedVersion: ProtocolNoiseIK,
			wantErr:         true, // Expected to fail due to invalid noise message
		},
		{
			name: "legacy request",
			request: &VersionedHandshakeRequest{
				ProtocolVersion:   ProtocolLegacy,
				SupportedVersions: []ProtocolVersion{ProtocolLegacy},
				NoiseMessage:      nil,
				LegacyData:        []byte("legacy handshake data"),
			},
			expectedVersion: ProtocolLegacy,
			wantErr:         false,
		},
		{
			name: "unsupported version",
			request: &VersionedHandshakeRequest{
				ProtocolVersion:   ProtocolVersion(99),
				SupportedVersions: []ProtocolVersion{ProtocolVersion(99)},
				NoiseMessage:      nil,
				LegacyData:        nil,
			},
			expectedVersion: ProtocolLegacy, // Should fallback to legacy
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8081")
			response, err := manager.HandleHandshakeRequest(tt.request, mockTransport, peerAddr)
			if (err != nil) != tt.wantErr {
				t.Errorf("HandleHandshakeRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if response.AgreedVersion != tt.expectedVersion {
					t.Errorf("Expected agreed version %v, got %v", tt.expectedVersion, response.AgreedVersion)
				}
			}
		})
	}
}

// TestVersionedHandshakeResponseWaiting tests that InitiateHandshake waits for actual responses
func TestVersionedHandshakeResponseWaiting(t *testing.T) {
	var staticPrivKey1, staticPrivKey2 [32]byte
	rand.Read(staticPrivKey1[:])
	rand.Read(staticPrivKey2[:])

	var staticPubKey1, staticPubKey2 [32]byte
	copy(staticPubKey1[:], staticPrivKey1[:])
	copy(staticPubKey2[:], staticPrivKey2[:])

	supportedVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}
	preferredVersion := ProtocolNoiseIK

	manager := NewVersionedHandshakeManager(staticPrivKey1, supportedVersions, preferredVersion)
	mockTransport := NewMockTransport("127.0.0.1:8080")

	peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8081")

	t.Run("timeout when no response", func(t *testing.T) {
		// Set a short timeout for this test
		manager.handshakeTimeout = 100 * time.Millisecond

		// Initiate handshake - should timeout since we don't send a response
		_, err := manager.InitiateHandshake(staticPubKey2, mockTransport, peerAddr)
		if err != ErrHandshakeTimeout {
			t.Errorf("Expected timeout error, got: %v", err)
		}
	})

	t.Run("successful response handling", func(t *testing.T) {
		// Reset timeout
		manager.handshakeTimeout = 5 * time.Second
		mockTransport.ClearPackets()

		// Start handshake in a goroutine
		responseChan := make(chan *VersionedHandshakeResponse)
		errChan := make(chan error)

		go func() {
			resp, err := manager.InitiateHandshake(staticPubKey2, mockTransport, peerAddr)
			if err != nil {
				errChan <- err
				return
			}
			responseChan <- resp
		}()

		// Give the goroutine time to register and send the request
		time.Sleep(50 * time.Millisecond)

		// Verify a request was sent
		if len(mockTransport.GetPackets()) != 1 {
			t.Fatalf("Expected 1 packet sent, got %d", len(mockTransport.GetPackets()))
		}

		// Simulate receiving a response
		mockResponse := &VersionedHandshakeResponse{
			AgreedVersion: ProtocolLegacy,
			NoiseMessage:  nil,
			LegacyData:    []byte("legacy response"),
		}

		responseData, err := SerializeVersionedHandshakeResponse(mockResponse)
		if err != nil {
			t.Fatalf("Failed to serialize response: %v", err)
		}

		responsePacket := &Packet{
			PacketType: PacketNoiseHandshake,
			Data:       responseData,
		}

		// Simulate receiving the response by calling the handler directly
		err = mockTransport.SimulateReceive(responsePacket, peerAddr)
		if err != nil {
			t.Fatalf("Failed to simulate response: %v", err)
		}

		// Wait for the handshake to complete
		select {
		case resp := <-responseChan:
			if resp.AgreedVersion != ProtocolLegacy {
				t.Errorf("Expected agreed version %v, got %v", ProtocolLegacy, resp.AgreedVersion)
			}
		case err := <-errChan:
			t.Errorf("Handshake failed: %v", err)
		case <-time.After(1 * time.Second):
			t.Error("Handshake did not complete within timeout")
		}
	})
}

// Benchmark tests
func BenchmarkSerializeVersionedHandshakeRequest(b *testing.B) {
	request := &VersionedHandshakeRequest{
		ProtocolVersion:   ProtocolNoiseIK,
		SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
		NoiseMessage:      make([]byte, 100), // Realistic noise message size
		LegacyData:        make([]byte, 50),  // Some legacy data
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := SerializeVersionedHandshakeRequest(request)
		if err != nil {
			b.Fatalf("Serialization failed: %v", err)
		}
	}
}

func BenchmarkParseVersionedHandshakeRequest(b *testing.B) {
	request := &VersionedHandshakeRequest{
		ProtocolVersion:   ProtocolNoiseIK,
		SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
		NoiseMessage:      make([]byte, 100),
		LegacyData:        make([]byte, 50),
	}

	data, err := SerializeVersionedHandshakeRequest(request)
	if err != nil {
		b.Fatalf("Failed to serialize request: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseVersionedHandshakeRequest(data)
		if err != nil {
			b.Fatalf("Parsing failed: %v", err)
		}
	}
}

// TestVersionedHandshakeManager_GetSupportedVersions tests the GetSupportedVersions method.
func TestVersionedHandshakeManager_GetSupportedVersions(t *testing.T) {
	var staticPrivKey [32]byte
	rand.Read(staticPrivKey[:])

	supportedVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}
	preferredVersion := ProtocolNoiseIK

	manager := NewVersionedHandshakeManager(staticPrivKey, supportedVersions, preferredVersion)

	t.Run("returns copy of supported versions", func(t *testing.T) {
		versions := manager.GetSupportedVersions()

		// Verify length matches
		if len(versions) != len(supportedVersions) {
			t.Errorf("Expected %d versions, got %d", len(supportedVersions), len(versions))
		}

		// Verify content matches
		for i, v := range versions {
			if v != supportedVersions[i] {
				t.Errorf("Expected version %v at index %d, got %v", supportedVersions[i], i, v)
			}
		}

		// Verify it's a copy (modifying returned slice doesn't affect internal state)
		versions[0] = ProtocolVersion(99)
		originalVersions := manager.GetSupportedVersions()
		if originalVersions[0] != ProtocolLegacy {
			t.Errorf("GetSupportedVersions returned reference instead of copy")
		}
	})
}
