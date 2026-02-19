package transport

import (
	"bytes"
	"testing"
)

// TestPacketSerialize tests the Packet.Serialize method.
func TestPacketSerialize(t *testing.T) {
	tests := []struct {
		name    string
		packet  *Packet
		wantErr bool
	}{
		{
			name: "valid packet",
			packet: &Packet{
				PacketType: PacketPingRequest,
				Data:       []byte{1, 2, 3, 4},
			},
			wantErr: false,
		},
		{
			name: "empty data",
			packet: &Packet{
				PacketType: PacketPingRequest,
				Data:       []byte{},
			},
			wantErr: false,
		},
		{
			name: "nil data",
			packet: &Packet{
				PacketType: PacketPingRequest,
				Data:       nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.packet.Serialize()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify format: [packet type (1 byte)][data]
			if len(result) != 1+len(tt.packet.Data) {
				t.Errorf("Expected length %d, got %d", 1+len(tt.packet.Data), len(result))
			}
			if result[0] != byte(tt.packet.PacketType) {
				t.Errorf("Expected packet type %d, got %d", tt.packet.PacketType, result[0])
			}
			if len(tt.packet.Data) > 0 && !bytes.Equal(result[1:], tt.packet.Data) {
				t.Error("Data mismatch")
			}
		})
	}
}

// TestParsePacket tests the ParsePacket function.
func TestParsePacket(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantType PacketType
		wantData []byte
		wantErr  bool
	}{
		{
			name:     "valid packet",
			data:     []byte{byte(PacketPingRequest), 1, 2, 3, 4},
			wantType: PacketPingRequest,
			wantData: []byte{1, 2, 3, 4},
			wantErr:  false,
		},
		{
			name:     "packet with only type",
			data:     []byte{byte(PacketPingResponse)},
			wantType: PacketPingResponse,
			wantData: []byte{},
			wantErr:  false,
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
		},
		{
			name:    "nil data",
			data:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet, err := ParsePacket(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if packet.PacketType != tt.wantType {
				t.Errorf("Expected packet type %d, got %d", tt.wantType, packet.PacketType)
			}
			if !bytes.Equal(packet.Data, tt.wantData) {
				t.Errorf("Expected data %v, got %v", tt.wantData, packet.Data)
			}
		})
	}
}

// TestNodePacketSerialize tests the NodePacket.Serialize method.
func TestNodePacketSerialize(t *testing.T) {
	pubKey := [32]byte{}
	for i := range pubKey {
		pubKey[i] = byte(i)
	}

	nonce := [24]byte{}
	for i := range nonce {
		nonce[i] = byte(i + 32)
	}

	tests := []struct {
		name   string
		packet *NodePacket
	}{
		{
			name: "valid node packet",
			packet: &NodePacket{
				PublicKey: pubKey,
				Nonce:     nonce,
				Payload:   []byte{1, 2, 3, 4},
			},
		},
		{
			name: "empty payload",
			packet: &NodePacket{
				PublicKey: pubKey,
				Nonce:     nonce,
				Payload:   []byte{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.packet.Serialize()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify format: [public key (32 bytes)][nonce (24 bytes)][payload]
			expectedLen := 32 + 24 + len(tt.packet.Payload)
			if len(result) != expectedLen {
				t.Errorf("Expected length %d, got %d", expectedLen, len(result))
			}

			// Verify public key
			if !bytes.Equal(result[0:32], tt.packet.PublicKey[:]) {
				t.Error("Public key mismatch")
			}

			// Verify nonce
			if !bytes.Equal(result[32:56], tt.packet.Nonce[:]) {
				t.Error("Nonce mismatch")
			}

			// Verify payload
			if len(tt.packet.Payload) > 0 && !bytes.Equal(result[56:], tt.packet.Payload) {
				t.Error("Payload mismatch")
			}
		})
	}
}

// TestParseNodePacket tests the ParseNodePacket function.
func TestParseNodePacket(t *testing.T) {
	// Create valid data
	pubKey := [32]byte{}
	for i := range pubKey {
		pubKey[i] = byte(i)
	}
	nonce := [24]byte{}
	for i := range nonce {
		nonce[i] = byte(i + 32)
	}
	payload := []byte{1, 2, 3, 4}

	validData := make([]byte, 32+24+len(payload))
	copy(validData[0:32], pubKey[:])
	copy(validData[32:56], nonce[:])
	copy(validData[56:], payload)

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "valid node packet",
			data:    validData,
			wantErr: false,
		},
		{
			name:    "minimum valid size (just pubkey and nonce)",
			data:    validData[:56],
			wantErr: false,
		},
		{
			name:    "too short - missing nonce",
			data:    make([]byte, 32),
			wantErr: true,
		},
		{
			name:    "too short - empty",
			data:    []byte{},
			wantErr: true,
		},
		{
			name:    "nil data",
			data:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet, err := ParseNodePacket(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify public key
			if !bytes.Equal(packet.PublicKey[:], tt.data[0:32]) {
				t.Error("Public key mismatch")
			}

			// Verify nonce
			if !bytes.Equal(packet.Nonce[:], tt.data[32:56]) {
				t.Error("Nonce mismatch")
			}

			// Verify payload
			expectedPayload := tt.data[56:]
			if !bytes.Equal(packet.Payload, expectedPayload) {
				t.Errorf("Payload mismatch: got %v, want %v", packet.Payload, expectedPayload)
			}
		})
	}
}

// TestPacketSerializeRoundTrip tests that serialize and parse are inverse operations.
func TestPacketSerializeRoundTrip(t *testing.T) {
	original := &Packet{
		PacketType: PacketFileData,
		Data:       []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}

	serialized, err := original.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	parsed, err := ParsePacket(serialized)
	if err != nil {
		t.Fatalf("ParsePacket failed: %v", err)
	}

	if parsed.PacketType != original.PacketType {
		t.Errorf("PacketType mismatch: got %d, want %d", parsed.PacketType, original.PacketType)
	}

	if !bytes.Equal(parsed.Data, original.Data) {
		t.Errorf("Data mismatch: got %v, want %v", parsed.Data, original.Data)
	}
}

// TestNodePacketSerializeRoundTrip tests that NodePacket serialize and parse are inverse operations.
func TestNodePacketSerializeRoundTrip(t *testing.T) {
	pubKey := [32]byte{}
	for i := range pubKey {
		pubKey[i] = byte(i * 2)
	}
	nonce := [24]byte{}
	for i := range nonce {
		nonce[i] = byte(i * 3)
	}

	original := &NodePacket{
		PublicKey: pubKey,
		Nonce:     nonce,
		Payload:   []byte{0xCA, 0xFE, 0xBA, 0xBE},
	}

	serialized, err := original.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	parsed, err := ParseNodePacket(serialized)
	if err != nil {
		t.Fatalf("ParseNodePacket failed: %v", err)
	}

	if !bytes.Equal(parsed.PublicKey[:], original.PublicKey[:]) {
		t.Error("PublicKey mismatch")
	}

	if !bytes.Equal(parsed.Nonce[:], original.Nonce[:]) {
		t.Error("Nonce mismatch")
	}

	if !bytes.Equal(parsed.Payload, original.Payload) {
		t.Errorf("Payload mismatch: got %v, want %v", parsed.Payload, original.Payload)
	}
}
