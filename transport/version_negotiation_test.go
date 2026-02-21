package transport

import (
	"crypto/rand"
	"net"
	"testing"
	"time"
)

func TestProtocolVersionString(t *testing.T) {
	tests := []struct {
		version  ProtocolVersion
		expected string
	}{
		{ProtocolLegacy, "Legacy"},
		{ProtocolNoiseIK, "Noise-IK"},
		{ProtocolVersion(99), "Unknown(99)"},
	}

	for _, test := range tests {
		if got := test.version.String(); got != test.expected {
			t.Errorf("ProtocolVersion(%d).String() = %q, want %q", test.version, got, test.expected)
		}
	}
}

func TestSerializeVersionNegotiation(t *testing.T) {
	packet := &VersionNegotiationPacket{
		SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
		PreferredVersion:  ProtocolNoiseIK,
	}

	data, err := SerializeVersionNegotiation(packet)
	if err != nil {
		t.Fatalf("SerializeVersionNegotiation failed: %v", err)
	}

	expected := []byte{1, 2, 0, 1} // preferred=1, count=2, versions=[0,1]
	if len(data) != len(expected) {
		t.Fatalf("Expected %d bytes, got %d", len(expected), len(data))
	}

	for i, b := range expected {
		if data[i] != b {
			t.Errorf("Byte %d: expected %d, got %d", i, b, data[i])
		}
	}
}

func TestSerializeVersionNegotiationErrors(t *testing.T) {
	tests := []struct {
		name    string
		packet  *VersionNegotiationPacket
		wantErr bool
	}{
		{
			name:    "nil packet",
			packet:  nil,
			wantErr: true,
		},
		{
			name: "empty supported versions",
			packet: &VersionNegotiationPacket{
				SupportedVersions: []ProtocolVersion{},
				PreferredVersion:  ProtocolLegacy,
			},
			wantErr: true,
		},
		{
			name: "valid packet",
			packet: &VersionNegotiationPacket{
				SupportedVersions: []ProtocolVersion{ProtocolLegacy},
				PreferredVersion:  ProtocolLegacy,
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := SerializeVersionNegotiation(test.packet)
			if (err != nil) != test.wantErr {
				t.Errorf("SerializeVersionNegotiation() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestParseVersionNegotiation(t *testing.T) {
	data := []byte{1, 2, 0, 1} // preferred=1, count=2, versions=[0,1]

	packet, err := ParseVersionNegotiation(data)
	if err != nil {
		t.Fatalf("ParseVersionNegotiation failed: %v", err)
	}

	if packet.PreferredVersion != ProtocolNoiseIK {
		t.Errorf("Expected preferred version %d, got %d", ProtocolNoiseIK, packet.PreferredVersion)
	}

	if len(packet.SupportedVersions) != 2 {
		t.Fatalf("Expected 2 supported versions, got %d", len(packet.SupportedVersions))
	}

	expectedVersions := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}
	for i, expected := range expectedVersions {
		if packet.SupportedVersions[i] != expected {
			t.Errorf("Supported version %d: expected %d, got %d", i, expected, packet.SupportedVersions[i])
		}
	}
}

func TestParseVersionNegotiationErrors(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "too short",
			data:    []byte{1},
			wantErr: true,
		},
		{
			name:    "wrong length",
			data:    []byte{1, 3, 0, 1}, // says 3 versions but only has 2 bytes
			wantErr: true,
		},
		{
			name:    "valid data",
			data:    []byte{0, 1, 0},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseVersionNegotiation(test.data)
			if (err != nil) != test.wantErr {
				t.Errorf("ParseVersionNegotiation() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestNewVersionNegotiator(t *testing.T) {
	supported := []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}
	preferred := ProtocolNoiseIK

	vn := NewVersionNegotiator(supported, preferred, 5*time.Second)

	if vn.preferredVersion != preferred {
		t.Errorf("Expected preferred version %d, got %d", preferred, vn.preferredVersion)
	}

	if len(vn.supportedVersions) != len(supported) {
		t.Errorf("Expected %d supported versions, got %d", len(supported), len(vn.supportedVersions))
	}
}

func TestNewVersionNegotiatorFallback(t *testing.T) {
	supported := []ProtocolVersion{ProtocolLegacy}
	preferred := ProtocolNoiseIK // Not in supported list

	vn := NewVersionNegotiator(supported, preferred, 5*time.Second)

	// Should fallback to first supported version
	if vn.preferredVersion != ProtocolLegacy {
		t.Errorf("Expected fallback to %d, got %d", ProtocolLegacy, vn.preferredVersion)
	}
}

func TestVersionNegotiatorSelectBestVersion(t *testing.T) {
	vn := NewVersionNegotiator([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK, 5*time.Second)

	tests := []struct {
		name         string
		peerVersions []ProtocolVersion
		expected     ProtocolVersion
	}{
		{
			name:         "both support noise",
			peerVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
			expected:     ProtocolNoiseIK,
		},
		{
			name:         "only legacy supported",
			peerVersions: []ProtocolVersion{ProtocolLegacy},
			expected:     ProtocolLegacy,
		},
		{
			name:         "no common versions",
			peerVersions: []ProtocolVersion{ProtocolVersion(99)},
			expected:     ProtocolLegacy, // Fallback to lowest
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := vn.SelectBestVersion(test.peerVersions)
			if result != test.expected {
				t.Errorf("SelectBestVersion() = %d, want %d", result, test.expected)
			}
		})
	}
}

func TestVersionNegotiatorIsVersionSupported(t *testing.T) {
	vn := NewVersionNegotiator([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK, 5*time.Second)

	tests := []struct {
		version   ProtocolVersion
		supported bool
	}{
		{ProtocolLegacy, true},
		{ProtocolNoiseIK, true},
		{ProtocolVersion(99), false},
	}

	for _, test := range tests {
		result := vn.IsVersionSupported(test.version)
		if result != test.supported {
			t.Errorf("IsVersionSupported(%d) = %v, want %v", test.version, result, test.supported)
		}
	}
}

func TestDefaultProtocolCapabilities(t *testing.T) {
	caps := DefaultProtocolCapabilities()

	if caps == nil {
		t.Fatal("DefaultProtocolCapabilities returned nil")
	}

	if len(caps.SupportedVersions) == 0 {
		t.Error("Expected at least one supported version")
	}

	if caps.PreferredVersion != ProtocolNoiseIK {
		t.Errorf("Expected preferred version %d, got %d", ProtocolNoiseIK, caps.PreferredVersion)
	}

	if !caps.EnableLegacyFallback {
		t.Error("Expected legacy fallback to be enabled by default")
	}

	if caps.NegotiationTimeout == 0 {
		t.Error("Expected non-zero negotiation timeout")
	}
}

func TestNewNegotiatingTransport(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")
	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	// Test with default capabilities
	nt, err := NewNegotiatingTransport(mockTransport, nil, staticPrivKey)
	if err != nil {
		t.Fatalf("NewNegotiatingTransport failed: %v", err)
	}

	if nt.underlying != mockTransport {
		t.Error("Underlying transport not set correctly")
	}

	if !nt.fallbackEnabled {
		t.Error("Expected fallback to be enabled by default")
	}

	if nt.noiseTransport == nil {
		t.Error("Expected noise transport to be created")
	}
}

func TestNewNegotiatingTransportErrors(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")

	tests := []struct {
		name         string
		capabilities *ProtocolCapabilities
		staticKey    []byte
		wantErr      bool
	}{
		{
			name: "empty supported versions",
			capabilities: &ProtocolCapabilities{
				SupportedVersions: []ProtocolVersion{},
			},
			staticKey: make([]byte, 32),
			wantErr:   true,
		},
		{
			name:      "wrong key size",
			staticKey: make([]byte, 16),
			wantErr:   true,
		},
		{
			name: "legacy only - no noise support",
			capabilities: &ProtocolCapabilities{
				SupportedVersions: []ProtocolVersion{ProtocolLegacy},
				PreferredVersion:  ProtocolLegacy,
			},
			staticKey: make([]byte, 32),
			wantErr:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewNegotiatingTransport(mockTransport, test.capabilities, test.staticKey)
			if (err != nil) != test.wantErr {
				t.Errorf("NewNegotiatingTransport() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestNegotiatingTransportPeerVersionManagement(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")
	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	nt, err := NewNegotiatingTransport(mockTransport, nil, staticPrivKey)
	if err != nil {
		t.Fatalf("NewNegotiatingTransport failed: %v", err)
	}

	peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:9090")

	// Initially unknown
	version := nt.GetPeerVersion(peerAddr)
	if version != ProtocolVersion(255) { // Unknown version sentinel
		t.Errorf("Expected unknown version (255), got %d", version)
	}

	// Set version
	nt.SetPeerVersion(peerAddr, ProtocolNoiseIK)
	version = nt.GetPeerVersion(peerAddr)
	if version != ProtocolNoiseIK {
		t.Errorf("Expected version %d, got %d", ProtocolNoiseIK, version)
	}
}

func TestNegotiatingTransportClose(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")
	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	nt, err := NewNegotiatingTransport(mockTransport, nil, staticPrivKey)
	if err != nil {
		t.Fatalf("NewNegotiatingTransport failed: %v", err)
	}

	err = nt.Close()
	if err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}

// Helper function to create version negotiation packet for testing
func createTestVersionPacket(supported []ProtocolVersion, preferred ProtocolVersion) *Packet {
	vnPacket := &VersionNegotiationPacket{
		SupportedVersions: supported,
		PreferredVersion:  preferred,
	}

	data, _ := SerializeVersionNegotiation(vnPacket)
	return &Packet{
		PacketType: PacketVersionNegotiation,
		Data:       data,
	}
}

func TestNegotiatingTransportVersionNegotiationHandler(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")
	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	// Use unsigned negotiation for this backward-compatibility test
	caps := &ProtocolCapabilities{
		SupportedVersions:        []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
		PreferredVersion:         ProtocolNoiseIK,
		EnableLegacyFallback:     true,
		NegotiationTimeout:       5 * time.Second,
		RequireSignedNegotiation: false, // Test unsigned negotiation
	}

	nt, err := NewNegotiatingTransport(mockTransport, caps, staticPrivKey)
	if err != nil {
		t.Fatalf("NewNegotiatingTransport failed: %v", err)
	}

	peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:9090")

	// Create a version negotiation packet
	packet := createTestVersionPacket([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK)

	// Handle the packet
	err = nt.handleVersionNegotiation(packet, peerAddr)
	if err != nil {
		t.Errorf("handleVersionNegotiation failed: %v", err)
	}

	// Check that peer version was stored
	version := nt.GetPeerVersion(peerAddr)
	if version != ProtocolNoiseIK {
		t.Errorf("Expected peer version %d, got %d", ProtocolNoiseIK, version)
	}

	// Check that a response was sent
	if len(mockTransport.packets) != 1 {
		t.Errorf("Expected 1 response packet, got %d", len(mockTransport.packets))
	}

	sentPacket := mockTransport.packets[0]
	if sentPacket.packet.PacketType != PacketVersionNegotiation {
		t.Errorf("Expected version negotiation response, got packet type %d", sentPacket.packet.PacketType)
	}
}

// TestNegotiateProtocolSynchronous tests that NegotiateProtocol waits for peer response
func TestNegotiateProtocolSynchronous(t *testing.T) {
	// Create two mock transports that can exchange packets
	transport1 := NewMockTransport("127.0.0.1:8080")
	transport2 := NewMockTransport("127.0.0.1:9090")

	// Create negotiator
	vn1 := NewVersionNegotiator([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK, 5*time.Second)

	// Set up a goroutine to simulate peer response
	done := make(chan bool)
	go func() {
		// Wait a bit to ensure NegotiateProtocol has started
		time.Sleep(100 * time.Millisecond)

		// Simulate peer receiving the request and sending response
		responsePacket := &VersionNegotiationPacket{
			SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
			PreferredVersion:  ProtocolNoiseIK,
		}

		// Notify vn1 of the response
		vn1.handleResponse(transport2.LocalAddr(), responsePacket.SupportedVersions)
		done <- true
	}()

	// Perform negotiation
	negotiatedVersion, err := vn1.NegotiateProtocol(transport1, transport2.LocalAddr())
	if err != nil {
		t.Fatalf("NegotiateProtocol failed: %v", err)
	}

	// Verify the negotiated version is the highest mutually supported
	if negotiatedVersion != ProtocolNoiseIK {
		t.Errorf("Expected negotiated version %d, got %d", ProtocolNoiseIK, negotiatedVersion)
	}

	// Wait for goroutine to complete
	<-done

	// Verify a negotiation packet was sent
	if len(transport1.packets) != 1 {
		t.Errorf("Expected 1 negotiation packet to be sent, got %d", len(transport1.packets))
	}

	if transport1.packets[0].packet.PacketType != PacketVersionNegotiation {
		t.Errorf("Expected PacketVersionNegotiation, got %d", transport1.packets[0].packet.PacketType)
	}
}

// TestNegotiateProtocolTimeout tests that negotiation times out if no response
func TestNegotiateProtocolTimeout(t *testing.T) {
	transport1 := NewMockTransport("127.0.0.1:8080")
	transport2 := NewMockTransport("127.0.0.1:9090")

	// Create negotiator with short timeout
	vn := NewVersionNegotiator([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK, 100*time.Millisecond)

	// Perform negotiation without sending response - should timeout
	start := time.Now()
	_, err := vn.NegotiateProtocol(transport1, transport2.LocalAddr())

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	elapsed := time.Since(start)
	if elapsed < 100*time.Millisecond {
		t.Errorf("Timeout occurred too quickly: %v", elapsed)
	}

	// Should have sent one packet
	if len(transport1.packets) != 1 {
		t.Errorf("Expected 1 negotiation packet, got %d", len(transport1.packets))
	}
}

// TestNegotiateProtocolLegacyFallback tests negotiation with legacy-only peer
func TestNegotiateProtocolLegacyFallback(t *testing.T) {
	transport1 := NewMockTransport("127.0.0.1:8080")
	transport2 := NewMockTransport("127.0.0.1:9090")

	// vn1 supports both, vn2 only supports legacy
	vn1 := NewVersionNegotiator([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK, 5*time.Second)

	// Simulate peer response with only legacy support
	go func() {
		time.Sleep(50 * time.Millisecond)
		vn1.handleResponse(transport2.LocalAddr(), []ProtocolVersion{ProtocolLegacy})
	}()

	negotiatedVersion, err := vn1.NegotiateProtocol(transport1, transport2.LocalAddr())
	if err != nil {
		t.Fatalf("NegotiateProtocol failed: %v", err)
	}

	// Should negotiate to legacy since peer only supports that
	if negotiatedVersion != ProtocolLegacy {
		t.Errorf("Expected negotiated version %d (Legacy), got %d", ProtocolLegacy, negotiatedVersion)
	}
}

// TestSerializeSignedVersionNegotiation tests signing version negotiation packets
func TestSerializeSignedVersionNegotiation(t *testing.T) {
	// Generate a test key pair
	var privateKey [32]byte
	rand.Read(privateKey[:])

	packet := &SignedVersionNegotiationPacket{
		VersionNegotiationPacket: VersionNegotiationPacket{
			SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
			PreferredVersion:  ProtocolNoiseIK,
		},
	}

	data, err := SerializeSignedVersionNegotiation(packet, privateKey)
	if err != nil {
		t.Fatalf("SerializeSignedVersionNegotiation failed: %v", err)
	}

	// Minimum length: 32 (pubkey) + 64 (signature) + 4 (version data) = 100
	expectedMinLen := 32 + 64 + 4
	if len(data) < expectedMinLen {
		t.Fatalf("Expected at least %d bytes, got %d", expectedMinLen, len(data))
	}
}

// TestParseSignedVersionNegotiation tests parsing and verifying signed packets
func TestParseSignedVersionNegotiation(t *testing.T) {
	var privateKey [32]byte
	rand.Read(privateKey[:])

	original := &SignedVersionNegotiationPacket{
		VersionNegotiationPacket: VersionNegotiationPacket{
			SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
			PreferredVersion:  ProtocolNoiseIK,
		},
	}

	data, err := SerializeSignedVersionNegotiation(original, privateKey)
	if err != nil {
		t.Fatalf("SerializeSignedVersionNegotiation failed: %v", err)
	}

	parsed, err := ParseSignedVersionNegotiation(data)
	if err != nil {
		t.Fatalf("ParseSignedVersionNegotiation failed: %v", err)
	}

	if parsed.PreferredVersion != original.PreferredVersion {
		t.Errorf("PreferredVersion mismatch: got %d, want %d", parsed.PreferredVersion, original.PreferredVersion)
	}

	if len(parsed.SupportedVersions) != len(original.SupportedVersions) {
		t.Errorf("SupportedVersions length mismatch: got %d, want %d", len(parsed.SupportedVersions), len(original.SupportedVersions))
	}
}

// TestSignedVersionNegotiationRejectsTamperedPacket tests that tampered packets are rejected
func TestSignedVersionNegotiationRejectsTamperedPacket(t *testing.T) {
	var privateKey [32]byte
	rand.Read(privateKey[:])

	packet := &SignedVersionNegotiationPacket{
		VersionNegotiationPacket: VersionNegotiationPacket{
			SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
			PreferredVersion:  ProtocolNoiseIK,
		},
	}

	data, err := SerializeSignedVersionNegotiation(packet, privateKey)
	if err != nil {
		t.Fatalf("SerializeSignedVersionNegotiation failed: %v", err)
	}

	// Tamper with the version data (change preferred version)
	data[32+64] = byte(ProtocolLegacy) // Change preferred version in the data

	_, err = ParseSignedVersionNegotiation(data)
	if err == nil {
		t.Fatal("Expected error for tampered packet, got nil")
	}
}

// TestSignedVersionNegotiationRejectsInvalidSignature tests rejection of invalid signatures
func TestSignedVersionNegotiationRejectsInvalidSignature(t *testing.T) {
	var privateKey [32]byte
	rand.Read(privateKey[:])

	packet := &SignedVersionNegotiationPacket{
		VersionNegotiationPacket: VersionNegotiationPacket{
			SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
			PreferredVersion:  ProtocolNoiseIK,
		},
	}

	data, err := SerializeSignedVersionNegotiation(packet, privateKey)
	if err != nil {
		t.Fatalf("SerializeSignedVersionNegotiation failed: %v", err)
	}

	// Zero out the signature
	for i := 32; i < 32+64; i++ {
		data[i] = 0
	}

	_, err = ParseSignedVersionNegotiation(data)
	if err == nil {
		t.Fatal("Expected error for invalid signature, got nil")
	}
}

// TestNewSignedVersionNegotiator tests creating a signed negotiator
func TestNewSignedVersionNegotiator(t *testing.T) {
	var privateKey [32]byte
	rand.Read(privateKey[:])

	vn := NewSignedVersionNegotiator([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK, 5*time.Second, privateKey)

	if !vn.RequiresSignatures() {
		t.Error("Expected RequiresSignatures() to return true")
	}

	if vn.preferredVersion != ProtocolNoiseIK {
		t.Errorf("Expected preferred version %d, got %d", ProtocolNoiseIK, vn.preferredVersion)
	}
}

// TestVersionNegotiatorParseVersionPacket tests parsing both signed and unsigned packets
func TestVersionNegotiatorParseVersionPacket(t *testing.T) {
	var privateKey [32]byte
	rand.Read(privateKey[:])

	t.Run("unsigned_negotiator_accepts_unsigned", func(t *testing.T) {
		vn := NewVersionNegotiator([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK, 5*time.Second)

		// Create unsigned packet
		unsignedData := []byte{1, 2, 0, 1} // preferred=1, count=2, versions=[0,1]

		parsed, pubKey, err := vn.ParseVersionPacket(unsignedData)
		if err != nil {
			t.Fatalf("ParseVersionPacket failed: %v", err)
		}

		if pubKey != nil {
			t.Error("Expected nil public key for unsigned packet")
		}

		if parsed.PreferredVersion != ProtocolNoiseIK {
			t.Errorf("Expected preferred version %d, got %d", ProtocolNoiseIK, parsed.PreferredVersion)
		}
	})

	t.Run("unsigned_negotiator_accepts_signed", func(t *testing.T) {
		vn := NewVersionNegotiator([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK, 5*time.Second)

		// Create signed packet
		signedPacket := &SignedVersionNegotiationPacket{
			VersionNegotiationPacket: VersionNegotiationPacket{
				SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
				PreferredVersion:  ProtocolNoiseIK,
			},
		}
		signedData, _ := SerializeSignedVersionNegotiation(signedPacket, privateKey)

		parsed, pubKey, err := vn.ParseVersionPacket(signedData)
		if err != nil {
			t.Fatalf("ParseVersionPacket failed: %v", err)
		}

		if pubKey == nil {
			t.Error("Expected non-nil public key for signed packet")
		}

		if parsed.PreferredVersion != ProtocolNoiseIK {
			t.Errorf("Expected preferred version %d, got %d", ProtocolNoiseIK, parsed.PreferredVersion)
		}
	})

	t.Run("signed_negotiator_rejects_unsigned", func(t *testing.T) {
		vn := NewSignedVersionNegotiator([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK, 5*time.Second, privateKey)

		// Create unsigned packet
		unsignedData := []byte{1, 2, 0, 1} // preferred=1, count=2, versions=[0,1]

		_, _, err := vn.ParseVersionPacket(unsignedData)
		if err == nil {
			t.Fatal("Expected error when signed negotiator receives unsigned packet")
		}
	})

	t.Run("signed_negotiator_accepts_signed", func(t *testing.T) {
		vn := NewSignedVersionNegotiator([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK, 5*time.Second, privateKey)

		// Create signed packet
		signedPacket := &SignedVersionNegotiationPacket{
			VersionNegotiationPacket: VersionNegotiationPacket{
				SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
				PreferredVersion:  ProtocolNoiseIK,
			},
		}
		signedData, _ := SerializeSignedVersionNegotiation(signedPacket, privateKey)

		parsed, pubKey, err := vn.ParseVersionPacket(signedData)
		if err != nil {
			t.Fatalf("ParseVersionPacket failed: %v", err)
		}

		if pubKey == nil {
			t.Error("Expected non-nil public key for signed packet")
		}

		if parsed.PreferredVersion != ProtocolNoiseIK {
			t.Errorf("Expected preferred version %d, got %d", ProtocolNoiseIK, parsed.PreferredVersion)
		}
	})
}

// TestSignedNegotiatingTransportHandler tests the NegotiatingTransport with signed packets
func TestSignedNegotiatingTransportHandler(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")
	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	// Use default capabilities (signed negotiation enabled)
	nt, err := NewNegotiatingTransport(mockTransport, nil, staticPrivKey)
	if err != nil {
		t.Fatalf("NewNegotiatingTransport failed: %v", err)
	}

	peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:9090")

	// Create a SIGNED version negotiation packet
	var peerPrivKey [32]byte
	rand.Read(peerPrivKey[:])

	signedPacket := &SignedVersionNegotiationPacket{
		VersionNegotiationPacket: VersionNegotiationPacket{
			SupportedVersions: []ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK},
			PreferredVersion:  ProtocolNoiseIK,
		},
	}
	signedData, _ := SerializeSignedVersionNegotiation(signedPacket, peerPrivKey)

	packet := &Packet{
		PacketType: PacketVersionNegotiation,
		Data:       signedData,
	}

	// Handle the packet
	err = nt.handleVersionNegotiation(packet, peerAddr)
	if err != nil {
		t.Fatalf("handleVersionNegotiation failed: %v", err)
	}

	// Check that peer version was stored
	version := nt.GetPeerVersion(peerAddr)
	if version != ProtocolNoiseIK {
		t.Errorf("Expected peer version %d, got %d", ProtocolNoiseIK, version)
	}

	// Check that a response was sent
	if len(mockTransport.packets) != 1 {
		t.Fatalf("Expected 1 response packet, got %d", len(mockTransport.packets))
	}

	sentPacket := mockTransport.packets[0]
	if sentPacket.packet.PacketType != PacketVersionNegotiation {
		t.Errorf("Expected version negotiation response, got packet type %d", sentPacket.packet.PacketType)
	}

	// Verify the response is also signed
	_, err = ParseSignedVersionNegotiation(sentPacket.packet.Data)
	if err != nil {
		t.Errorf("Response packet should be signed: %v", err)
	}
}

// TestSignedNegotiationRejectsUnsignedPacket tests that signed transport rejects unsigned packets
func TestSignedNegotiationRejectsUnsignedPacket(t *testing.T) {
	mockTransport := NewMockTransport("127.0.0.1:8080")
	staticPrivKey := make([]byte, 32)
	rand.Read(staticPrivKey)

	// Use default capabilities (signed negotiation enabled)
	nt, err := NewNegotiatingTransport(mockTransport, nil, staticPrivKey)
	if err != nil {
		t.Fatalf("NewNegotiatingTransport failed: %v", err)
	}

	peerAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:9090")

	// Create an UNSIGNED version negotiation packet
	packet := createTestVersionPacket([]ProtocolVersion{ProtocolLegacy, ProtocolNoiseIK}, ProtocolNoiseIK)

	// Handle the packet - should fail
	err = nt.handleVersionNegotiation(packet, peerAddr)
	if err == nil {
		t.Fatal("Expected error when handling unsigned packet with signed negotiation enabled")
	}

	// No response should be sent
	if len(mockTransport.packets) != 0 {
		t.Errorf("Expected 0 response packets for rejected negotiation, got %d", len(mockTransport.packets))
	}
}

// TestDefaultProtocolCapabilitiesRequireSignedNegotiation verifies default behavior
func TestDefaultProtocolCapabilitiesRequireSignedNegotiation(t *testing.T) {
	caps := DefaultProtocolCapabilities()

	if !caps.RequireSignedNegotiation {
		t.Error("Expected RequireSignedNegotiation to be true by default for MITM protection")
	}
}
