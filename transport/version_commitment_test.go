package transport

import (
	"testing"
	"time"
)

func TestCreateVersionCommitment(t *testing.T) {
	tests := []struct {
		name          string
		version       ProtocolVersion
		handshakeHash []byte
		wantErr       bool
	}{
		{
			name:          "valid commitment NoiseIK",
			version:       ProtocolNoiseIK,
			handshakeHash: make([]byte, 32),
			wantErr:       false,
		},
		{
			name:          "valid commitment Legacy",
			version:       ProtocolLegacy,
			handshakeHash: make([]byte, 32),
			wantErr:       false,
		},
		{
			name:          "empty handshake hash",
			version:       ProtocolNoiseIK,
			handshakeHash: []byte{},
			wantErr:       true,
		},
		{
			name:          "nil handshake hash",
			version:       ProtocolNoiseIK,
			handshakeHash: nil,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commitment, err := CreateVersionCommitment(tt.version, tt.handshakeHash)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if commitment.Version != tt.version {
				t.Errorf("version = %v, want %v", commitment.Version, tt.version)
			}
			if commitment.Timestamp == 0 {
				t.Error("timestamp should not be zero")
			}
			// HMAC should not be all zeros
			allZero := true
			for _, b := range commitment.HMAC {
				if b != 0 {
					allZero = false
					break
				}
			}
			if allZero {
				t.Error("HMAC should not be all zeros")
			}
		})
	}
}

func TestSerializeParseVersionCommitment(t *testing.T) {
	handshakeHash := make([]byte, 32)
	for i := range handshakeHash {
		handshakeHash[i] = byte(i)
	}

	original, err := CreateVersionCommitment(ProtocolNoiseIK, handshakeHash)
	if err != nil {
		t.Fatalf("failed to create commitment: %v", err)
	}

	// Serialize
	data, err := SerializeVersionCommitment(original)
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	// Check length
	if len(data) != 41 {
		t.Errorf("serialized length = %d, want 41", len(data))
	}

	// Parse
	parsed, err := ParseVersionCommitment(data)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Verify fields match
	if parsed.Version != original.Version {
		t.Errorf("version = %v, want %v", parsed.Version, original.Version)
	}
	if parsed.Timestamp != original.Timestamp {
		t.Errorf("timestamp = %v, want %v", parsed.Timestamp, original.Timestamp)
	}
	if parsed.HMAC != original.HMAC {
		t.Error("HMAC mismatch after serialization round-trip")
	}
}

func TestParseVersionCommitmentErrors(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "too short",
			data:    make([]byte, 40),
			wantErr: true,
		},
		{
			name:    "too long",
			data:    make([]byte, 42),
			wantErr: true,
		},
		{
			name:    "empty",
			data:    []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseVersionCommitment(tt.data)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestVerifyVersionCommitment(t *testing.T) {
	handshakeHash := make([]byte, 32)
	for i := range handshakeHash {
		handshakeHash[i] = byte(i)
	}

	validCommitment, err := CreateVersionCommitment(ProtocolNoiseIK, handshakeHash)
	if err != nil {
		t.Fatalf("failed to create commitment: %v", err)
	}

	tests := []struct {
		name            string
		commitment      *VersionCommitment
		expectedVersion ProtocolVersion
		handshakeHash   []byte
		wantErr         error
	}{
		{
			name:            "valid commitment",
			commitment:      validCommitment,
			expectedVersion: ProtocolNoiseIK,
			handshakeHash:   handshakeHash,
			wantErr:         nil,
		},
		{
			name:            "version mismatch",
			commitment:      validCommitment,
			expectedVersion: ProtocolLegacy,
			handshakeHash:   handshakeHash,
			wantErr:         ErrCommitmentVersionMismatch,
		},
		{
			name:            "wrong handshake hash",
			commitment:      validCommitment,
			expectedVersion: ProtocolNoiseIK,
			handshakeHash:   make([]byte, 32), // Different hash
			wantErr:         ErrInvalidCommitmentMAC,
		},
		{
			name:            "nil commitment",
			commitment:      nil,
			expectedVersion: ProtocolNoiseIK,
			handshakeHash:   handshakeHash,
			wantErr:         nil, // Will check for specific error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyVersionCommitment(tt.commitment, tt.expectedVersion, tt.handshakeHash)
			if tt.name == "nil commitment" {
				if err == nil {
					t.Error("expected error for nil commitment")
				}
				return
			}
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
				} else if !containsSubstr(err.Error(), tt.wantErr.Error()) {
					t.Errorf("expected error containing %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestVerifyVersionCommitmentTimestamp(t *testing.T) {
	handshakeHash := make([]byte, 32)

	// Test old timestamp
	oldCommitment := &VersionCommitment{
		Version:   ProtocolNoiseIK,
		Timestamp: time.Now().Add(-CommitmentMaxAge - time.Minute).Unix(),
	}
	mac := computeCommitmentMAC(oldCommitment.Version, oldCommitment.Timestamp, handshakeHash)
	copy(oldCommitment.HMAC[:], mac)

	err := VerifyVersionCommitment(oldCommitment, ProtocolNoiseIK, handshakeHash)
	if err == nil || !containsSubstr(err.Error(), ErrCommitmentTooOld.Error()) {
		t.Errorf("expected ErrCommitmentTooOld, got %v", err)
	}

	// Test future timestamp
	futureCommitment := &VersionCommitment{
		Version:   ProtocolNoiseIK,
		Timestamp: time.Now().Add(CommitmentMaxFutureDrift + time.Minute).Unix(),
	}
	mac = computeCommitmentMAC(futureCommitment.Version, futureCommitment.Timestamp, handshakeHash)
	copy(futureCommitment.HMAC[:], mac)

	err = VerifyVersionCommitment(futureCommitment, ProtocolNoiseIK, handshakeHash)
	if err == nil || !containsSubstr(err.Error(), ErrCommitmentFromFuture.Error()) {
		t.Errorf("expected ErrCommitmentFromFuture, got %v", err)
	}
}

func TestVersionCommitmentExchange(t *testing.T) {
	handshakeHash := make([]byte, 32)
	for i := range handshakeHash {
		handshakeHash[i] = byte(i)
	}

	// Create exchange for Alice
	aliceExchange, err := NewVersionCommitmentExchange(ProtocolNoiseIK, handshakeHash)
	if err != nil {
		t.Fatalf("failed to create Alice's exchange: %v", err)
	}

	// Create exchange for Bob
	bobExchange, err := NewVersionCommitmentExchange(ProtocolNoiseIK, handshakeHash)
	if err != nil {
		t.Fatalf("failed to create Bob's exchange: %v", err)
	}

	// Alice sends commitment to Bob
	aliceCommitment, err := aliceExchange.GetLocalCommitment()
	if err != nil {
		t.Fatalf("failed to get Alice's commitment: %v", err)
	}

	// Bob sends commitment to Alice
	bobCommitment, err := bobExchange.GetLocalCommitment()
	if err != nil {
		t.Fatalf("failed to get Bob's commitment: %v", err)
	}

	// Bob processes Alice's commitment
	if err := bobExchange.ProcessPeerCommitment(aliceCommitment); err != nil {
		t.Errorf("Bob failed to process Alice's commitment: %v", err)
	}

	// Alice processes Bob's commitment
	if err := aliceExchange.ProcessPeerCommitment(bobCommitment); err != nil {
		t.Errorf("Alice failed to process Bob's commitment: %v", err)
	}

	// Both should be verified
	if !aliceExchange.IsVerified() {
		t.Error("Alice's exchange should be verified")
	}
	if !bobExchange.IsVerified() {
		t.Error("Bob's exchange should be verified")
	}

	// Both should report the agreed version
	aliceVersion, err := aliceExchange.GetAgreedVersion()
	if err != nil {
		t.Errorf("failed to get Alice's agreed version: %v", err)
	}
	if aliceVersion != ProtocolNoiseIK {
		t.Errorf("Alice's version = %v, want %v", aliceVersion, ProtocolNoiseIK)
	}

	bobVersion, err := bobExchange.GetAgreedVersion()
	if err != nil {
		t.Errorf("failed to get Bob's agreed version: %v", err)
	}
	if bobVersion != ProtocolNoiseIK {
		t.Errorf("Bob's version = %v, want %v", bobVersion, ProtocolNoiseIK)
	}
}

func TestVersionCommitmentExchangeVersionMismatch(t *testing.T) {
	handshakeHash := make([]byte, 32)

	// Alice uses NoiseIK
	aliceExchange, err := NewVersionCommitmentExchange(ProtocolNoiseIK, handshakeHash)
	if err != nil {
		t.Fatalf("failed to create Alice's exchange: %v", err)
	}

	// Bob uses Legacy (attacker tampered with negotiation)
	bobExchange, err := NewVersionCommitmentExchange(ProtocolLegacy, handshakeHash)
	if err != nil {
		t.Fatalf("failed to create Bob's exchange: %v", err)
	}

	// Alice sends commitment to Bob
	aliceCommitment, err := aliceExchange.GetLocalCommitment()
	if err != nil {
		t.Fatalf("failed to get Alice's commitment: %v", err)
	}

	// Bob processes Alice's commitment - should fail due to version mismatch
	err = bobExchange.ProcessPeerCommitment(aliceCommitment)
	if err == nil {
		t.Error("expected error due to version mismatch")
	}
	if !containsSubstr(err.Error(), "version commitment mismatch") {
		t.Errorf("expected version mismatch error, got: %v", err)
	}

	// Bob's exchange should NOT be verified
	if bobExchange.IsVerified() {
		t.Error("Bob's exchange should not be verified after mismatch")
	}
}

func TestVersionCommitmentExchangeTamperedMAC(t *testing.T) {
	handshakeHash := make([]byte, 32)

	aliceExchange, err := NewVersionCommitmentExchange(ProtocolNoiseIK, handshakeHash)
	if err != nil {
		t.Fatalf("failed to create exchange: %v", err)
	}

	// Get valid commitment and tamper with MAC
	validCommitment, _ := aliceExchange.GetLocalCommitment()
	tamperedCommitment := make([]byte, len(validCommitment))
	copy(tamperedCommitment, validCommitment)
	// Flip a bit in the MAC portion (bytes 9-40)
	tamperedCommitment[20] ^= 0xFF

	bobExchange, _ := NewVersionCommitmentExchange(ProtocolNoiseIK, handshakeHash)
	err = bobExchange.ProcessPeerCommitment(tamperedCommitment)
	if err == nil {
		t.Error("expected error for tampered MAC")
	}
	if !containsSubstr(err.Error(), "MAC verification failed") {
		t.Errorf("expected MAC verification error, got: %v", err)
	}
}

// containsSubstr checks if s contains substr
func containsSubstr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstrHelper(s, substr))
}

func containsSubstrHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
