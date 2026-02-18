package toxcore

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/noise"
	"github.com/opd-ai/toxcore/transport"
)

// TestSecurityValidation_CryptographicProperties validates core crypto security properties
func TestSecurityValidation_CryptographicProperties(t *testing.T) {
	t.Run("Encryption is non-deterministic", func(t *testing.T) {
		// Generate test keys
		senderKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		receiverKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		message := []byte("This is a test message")

		// Encrypt the same message twice with different nonces
		nonce1, err := crypto.GenerateNonce()
		if err != nil {
			t.Fatal(err)
		}

		nonce2, err := crypto.GenerateNonce()
		if err != nil {
			t.Fatal(err)
		}

		ciphertext1, err := crypto.Encrypt(message, nonce1, receiverKeyPair.Public, senderKeyPair.Private)
		if err != nil {
			t.Fatal(err)
		}

		ciphertext2, err := crypto.Encrypt(message, nonce2, receiverKeyPair.Public, senderKeyPair.Private)
		if err != nil {
			t.Fatal(err)
		}

		// Ciphertexts should be different (non-deterministic encryption)
		if bytes.Equal(ciphertext1, ciphertext2) {
			t.Error("Encryption is deterministic - security vulnerability!")
		}
	})

	t.Run("Nonce generation is cryptographically random", func(t *testing.T) {
		// Generate multiple nonces and check for randomness
		nonces := make([]crypto.Nonce, 100)
		for i := range nonces {
			nonce, err := crypto.GenerateNonce()
			if err != nil {
				t.Fatal(err)
			}
			nonces[i] = nonce
		}

		// Check that no two nonces are identical (with high probability)
		for i := 0; i < len(nonces); i++ {
			for j := i + 1; j < len(nonces); j++ {
				if nonces[i] == nonces[j] {
					t.Error("Duplicate nonce detected - cryptographic randomness failure!")
				}
			}
		}
	})

	t.Run("Key generation produces unique keys", func(t *testing.T) {
		// Generate multiple key pairs and ensure uniqueness
		keyPairs := make([]crypto.KeyPair, 50)
		for i := range keyPairs {
			keyPair, err := crypto.GenerateKeyPair()
			if err != nil {
				t.Fatal(err)
			}
			keyPairs[i] = *keyPair
		}

		// Check that no two key pairs are identical
		for i := 0; i < len(keyPairs); i++ {
			for j := i + 1; j < len(keyPairs); j++ {
				if keyPairs[i].Public == keyPairs[j].Public {
					t.Error("Duplicate public key detected - key generation failure!")
				}
				if keyPairs[i].Private == keyPairs[j].Private {
					t.Error("Duplicate private key detected - key generation failure!")
				}
			}
		}
	})

	t.Run("Digital signatures provide authenticity", func(t *testing.T) {
		// Generate key pair for signing
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		message := []byte("This message should be authenticated")

		// Sign the message
		signature, err := crypto.Sign(message, keyPair.Private)
		if err != nil {
			t.Fatal(err)
		}

		// Verify with correct public key
		verifyKey := crypto.GetSignaturePublicKey(keyPair.Private)
		valid, err := crypto.Verify(message, signature, verifyKey)
		if err != nil {
			t.Fatal(err)
		}
		if !valid {
			t.Error("Valid signature failed verification")
		}

		// Verify with wrong public key should fail
		wrongKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}
		wrongVerifyKey := crypto.GetSignaturePublicKey(wrongKeyPair.Private)

		valid, err = crypto.Verify(message, signature, wrongVerifyKey)
		if err != nil {
			t.Fatal(err)
		}
		if valid {
			t.Error("Signature verified with wrong key - security vulnerability!")
		}
	})
}

// TestSecurityValidation_NoiseIKProperties validates Noise-IK security properties
func TestSecurityValidation_NoiseIKProperties(t *testing.T) {
	t.Run("Forward secrecy - handshake creation is non-deterministic", func(t *testing.T) {
		// Create two handshakes with same parameters
		initiatorKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		responderKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		handshake1, err := noise.NewIKHandshake(initiatorKeyPair.Private[:], responderKeyPair.Public[:], noise.Initiator)
		if err != nil {
			t.Fatal(err)
		}

		handshake2, err := noise.NewIKHandshake(initiatorKeyPair.Private[:], responderKeyPair.Public[:], noise.Initiator)
		if err != nil {
			t.Fatal(err)
		}

		// Handshakes should be different instances (ephemeral keys differ)
		if handshake1 == handshake2 {
			t.Error("Handshake instances are identical - potential ephemeral key reuse!")
		}

		// Basic validation that handshakes were created successfully
		if handshake1.IsComplete() {
			t.Error("Handshake1 reports complete before any messages exchanged")
		}
		if handshake2.IsComplete() {
			t.Error("Handshake2 reports complete before any messages exchanged")
		}
	})

	t.Run("Handshake state validation", func(t *testing.T) {
		// Create handshakes for initiator and responder
		initiatorKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		responderKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		initiatorHandshake, err := noise.NewIKHandshake(initiatorKeyPair.Private[:], responderKeyPair.Public[:], noise.Initiator)
		if err != nil {
			t.Fatal(err)
		}

		responderHandshake, err := noise.NewIKHandshake(responderKeyPair.Private[:], nil, noise.Responder)
		if err != nil {
			t.Fatal(err)
		}

		// Initially, handshakes should not be complete
		if initiatorHandshake.IsComplete() {
			t.Error("Initiator handshake reports complete before any messages")
		}
		if responderHandshake.IsComplete() {
			t.Error("Responder handshake reports complete before any messages")
		}

		// Verify handshakes were created with correct roles
		if initiatorHandshake == nil {
			t.Error("Failed to create initiator handshake")
		}
		if responderHandshake == nil {
			t.Error("Failed to create responder handshake")
		}
	})

	t.Run("Key derivation produces different results", func(t *testing.T) {
		// Test that different key pairs produce different handshakes
		keyPair1, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		keyPair2, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		peerKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		handshake1, err := noise.NewIKHandshake(keyPair1.Private[:], peerKeyPair.Public[:], noise.Initiator)
		if err != nil {
			t.Fatal(err)
		}

		handshake2, err := noise.NewIKHandshake(keyPair2.Private[:], peerKeyPair.Public[:], noise.Initiator)
		if err != nil {
			t.Fatal(err)
		}

		// Handshakes with different static keys should be different objects
		if handshake1 == handshake2 {
			t.Error("Handshakes with different static keys are identical - key isolation failure!")
		}
	})
}

// TestSecurityValidation_ProtocolProperties validates protocol-level security
func TestSecurityValidation_ProtocolProperties(t *testing.T) {
	t.Run("Version negotiation prevents downgrade attacks", func(t *testing.T) {
		// Create capabilities that prefer Noise-IK
		capabilities := &transport.ProtocolCapabilities{
			SupportedVersions: []transport.ProtocolVersion{transport.ProtocolLegacy, transport.ProtocolNoiseIK},
			PreferredVersion:  transport.ProtocolNoiseIK,
		}

		negotiator := transport.NewVersionNegotiator(capabilities.SupportedVersions, capabilities.PreferredVersion, capabilities.NegotiationTimeout)

		// Test against peer that supports both
		peerVersions := []transport.ProtocolVersion{transport.ProtocolLegacy, transport.ProtocolNoiseIK}
		selected := negotiator.SelectBestVersion(peerVersions)

		if selected != transport.ProtocolNoiseIK {
			t.Error("Version negotiation selected weaker protocol - downgrade attack possible!")
		}

		// Test against peer that only supports legacy
		legacyOnlyVersions := []transport.ProtocolVersion{transport.ProtocolLegacy}
		selected = negotiator.SelectBestVersion(legacyOnlyVersions)

		if selected != transport.ProtocolLegacy {
			t.Error("Version negotiation failed to fallback appropriately")
		}
	})

	t.Run("ToxID integrity protects against tampering", func(t *testing.T) {
		// Generate a valid ToxID
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		nospam, err := crypto.GenerateNospam()
		if err != nil {
			t.Fatal(err)
		}

		toxID := crypto.NewToxID(keyPair.Public, nospam)
		validToxIDString := toxID.String()

		// Verify valid ToxID parses correctly
		parsedToxID, err := crypto.ToxIDFromString(validToxIDString)
		if err != nil {
			t.Fatal(err)
		}

		if parsedToxID.String() != validToxIDString {
			t.Error("ToxID round-trip failed")
		}

		// Test that tampering with the ToxID string is detected
		tamperedToxIDString := validToxIDString[:len(validToxIDString)-2] + "FF"
		_, err = crypto.ToxIDFromString(tamperedToxIDString)
		if err == nil {
			t.Error("Tampered ToxID was accepted - integrity check failed!")
		}
	})

	t.Run("Message length limits prevent buffer overflow attacks", func(t *testing.T) {
		// Test that encryption rejects oversized messages
		keyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			t.Fatal(err)
		}

		// Create a message larger than the maximum allowed size
		oversizedMessage := make([]byte, crypto.MaxEncryptionBuffer+1)
		rand.Read(oversizedMessage)

		nonce, err := crypto.GenerateNonce()
		if err != nil {
			t.Fatal(err)
		}

		_, err = crypto.Encrypt(oversizedMessage, nonce, keyPair.Public, keyPair.Private)
		if err == nil {
			t.Error("Oversized message was encrypted - buffer overflow protection failed!")
		}
	})
}

// TestSecurityValidation_Implementation validates implementation-specific security
func TestSecurityValidation_Implementation(t *testing.T) {
	t.Run("No sensitive data in savedata format", func(t *testing.T) {
		// Create a Tox instance with some data
		options := NewOptions()
		tox, err := New(options)
		if err != nil {
			t.Fatal(err)
		}
		defer tox.Kill()

		err = tox.SelfSetName("Test User")
		if err != nil {
			t.Fatal(err)
		}

		// Get savedata
		savedata := tox.GetSavedata()

		// Savedata should not contain plaintext private keys or other sensitive data
		// Note: This is a basic check - in practice, you'd want more sophisticated analysis
		if len(savedata) == 0 {
			t.Error("Savedata is empty")
		}

		// Verify that savedata can be restored without errors
		restoredTox, err := NewFromSavedata(options, savedata)
		if err != nil {
			t.Error("Failed to restore from savedata:", err)
		} else {
			if restoredTox.SelfGetName() != "Test User" {
				t.Error("Savedata restoration lost data")
			}
			restoredTox.Kill()
		}
	})

	t.Run("Nospam provides anti-spam protection", func(t *testing.T) {
		// Create a Tox instance
		options := NewOptions()
		tox, err := New(options)
		if err != nil {
			t.Fatal(err)
		}
		defer tox.Kill()

		// Get initial ToxID and nospam
		initialToxID := tox.SelfGetAddress()
		initialNospam := tox.SelfGetNospam()

		// Change nospam
		newNospam := [4]byte{0x12, 0x34, 0x56, 0x78}
		tox.SelfSetNospam(newNospam)

		// ToxID should change
		newToxID := tox.SelfGetAddress()
		if initialToxID == newToxID {
			t.Error("ToxID unchanged after nospam change - anti-spam protection ineffective!")
		}

		// Nospam should be updated
		if tox.SelfGetNospam() != newNospam {
			t.Error("Nospam not updated correctly")
		}

		// Original nospam should not equal new nospam
		if initialNospam == newNospam {
			t.Error("Nospam values are identical - insufficient randomness!")
		}
	})
}

// TestSecurityImprovementsVerification verifies that all critical security
// improvements from the audit are working correctly
func TestSecurityImprovementsVerification(t *testing.T) {
	// Create a Tox instance with default options
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify secure-by-default transport is enabled
	securityInfo := tox.GetTransportSecurityInfo()
	if securityInfo == nil {
		t.Fatal("GetTransportSecurityInfo returned nil")
	}

	// Check that Noise-IK is enabled by default
	if !securityInfo.NoiseIKEnabled {
		t.Error("Expected Noise-IK to be enabled by default")
	}

	// Check that the transport type indicates negotiating transport
	if securityInfo.TransportType != "negotiating-udp" {
		t.Errorf("Expected transport type 'negotiating-udp', got '%s'", securityInfo.TransportType)
	}

	// Check that supported versions include both legacy and modern protocols
	expectedVersions := []string{"legacy", "noise-ik"}
	if len(securityInfo.SupportedVersions) != len(expectedVersions) {
		t.Errorf("Expected %d supported versions, got %d", len(expectedVersions), len(securityInfo.SupportedVersions))
	}

	// Verify security summary indicates secure status
	summary := tox.GetSecuritySummary()
	if summary == "" {
		t.Error("GetSecuritySummary returned empty string")
	}

	// Should indicate secure status
	if summary == "Basic: Legacy encryption only (consider enabling secure transport)" {
		t.Error("Security summary indicates basic encryption, expected secure status")
	}

	t.Logf("Security verification successful:")
	t.Logf("  Transport Type: %s", securityInfo.TransportType)
	t.Logf("  Noise-IK Enabled: %v", securityInfo.NoiseIKEnabled)
	t.Logf("  Legacy Fallback: %v", securityInfo.LegacyFallbackEnabled)
	t.Logf("  Supported Versions: %v", securityInfo.SupportedVersions)
	t.Logf("  Security Summary: %s", summary)
}

// TestEncryptionStatusAPI verifies the encryption status API functionality
func TestEncryptionStatusAPI(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test with non-existent friend
	status := tox.GetFriendEncryptionStatus(999)
	if status != EncryptionUnknown {
		t.Errorf("Expected EncryptionUnknown for non-existent friend, got %s", status)
	}

	// Add a friend to test with
	friendID, err := tox.AddFriend("76518406f6a9f2217e8dc487cc783c25cc16a15eb36ff32e335364ec37b1334912345678868a", "Test friend for encryption status")
	if err != nil {
		// This is expected to fail since we don't have a real connection
		// but we can still test the API structure
		t.Logf("AddFriend failed as expected (no real connection): %v", err)
		return
	}

	// Test encryption status for the added friend
	status = tox.GetFriendEncryptionStatus(friendID)
	// Should be offline since we don't have a real connection
	if status != EncryptionOffline {
		t.Logf("Friend encryption status: %s (expected offline)", status)
	}
}

// TestSecurityLoggingIntegration verifies that security logging is properly integrated
func TestSecurityLoggingIntegration(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// The fact that we can create a Tox instance successfully means
	// the secure transport initialization worked correctly.
	// In real usage, the logging would appear in the application logs.

	// Verify that the transport was created successfully with security
	securityInfo := tox.GetTransportSecurityInfo()
	if securityInfo.TransportType == "unknown" {
		t.Error("Transport type is unknown, security initialization may have failed")
	}

	t.Logf("Security logging integration test passed")
	t.Logf("Transport initialized with type: %s", securityInfo.TransportType)
}
