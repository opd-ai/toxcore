package transport

import (
	"bytes"
	"crypto/rand"
	"net"
	"testing"
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

	// This would test actual handshake in a full implementation
	// For now, we just test that the method doesn't crash
	peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8081")
	_, err := manager1.InitiateHandshake(staticPubKey2, mockTransport, peerAddr)
	if err != nil {
		t.Errorf("InitiateHandshake failed: %v", err)
	}

	// Verify a packet was sent
	if len(mockTransport.packets) != 1 {
		t.Errorf("Expected 1 packet to be sent, got %d", len(mockTransport.packets))
	}

	if mockTransport.packets[0].packet.PacketType != PacketNoiseHandshake {
		t.Errorf("Expected PacketNoiseHandshake, got %v", mockTransport.packets[0].packet.PacketType)
	}
}

func TestVersionedHandshakeManager_HandleHandshakeRequest(t *testing.T) {
	var staticPrivKey [32]byte
	rand.Read(staticPrivKey[:])

	supportedVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}
	preferredVersion := ProtocolNoiseIK

	manager := NewVersionedHandshakeManager(staticPrivKey, supportedVersions, preferredVersion)

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
			response, err := manager.HandleHandshakeRequest(tt.request, peerAddr)
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
