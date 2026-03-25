package group

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/curve25519"
)

func generateKeyPair() ([32]byte, [32]byte, error) {
	var privateKey, publicKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		return privateKey, publicKey, err
	}
	curve25519.ScalarBaseMult(&publicKey, &privateKey)
	return privateKey, publicKey, nil
}

func TestNewSenderKeyManager(t *testing.T) {
	var groupID [32]byte
	rand.Read(groupID[:])

	privKey, pubKey, err := generateKeyPair()
	require.NoError(t, err)

	skm, err := NewSenderKeyManager(groupID, pubKey, privKey)
	require.NoError(t, err)
	assert.NotNil(t, skm)
	assert.Equal(t, groupID, skm.groupID)
	assert.Equal(t, pubKey, skm.selfPublicKey)
	assert.NotNil(t, skm.mySenderKey)
	assert.Equal(t, uint32(1), skm.mySenderKey.KeyID)
}

func TestSenderKeyEncryptDecrypt(t *testing.T) {
	var groupID [32]byte
	rand.Read(groupID[:])

	// Create two members
	privKey1, pubKey1, err := generateKeyPair()
	require.NoError(t, err)
	privKey2, pubKey2, err := generateKeyPair()
	require.NoError(t, err)

	// Create sender key managers for both members
	skm1, err := NewSenderKeyManager(groupID, pubKey1, privKey1)
	require.NoError(t, err)
	skm2, err := NewSenderKeyManager(groupID, pubKey2, privKey2)
	require.NoError(t, err)

	// Member 1 distributes their sender key to Member 2
	distributions, err := skm1.CreateDistributions([][32]byte{pubKey2})
	require.NoError(t, err)
	require.Len(t, distributions, 1)

	// Member 2 processes the distribution
	err = skm2.ProcessDistribution(distributions[0])
	require.NoError(t, err)

	// Member 1 encrypts a message
	plaintext := []byte("Hello, secure group!")
	encryptedMsg, err := skm1.EncryptMessage(plaintext)
	require.NoError(t, err)
	assert.NotNil(t, encryptedMsg)
	assert.Equal(t, groupID, encryptedMsg.GroupID)
	assert.Equal(t, pubKey1, encryptedMsg.SenderPublicKey)

	// Member 2 decrypts the message
	decrypted, err := skm2.DecryptMessage(encryptedMsg)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestSenderKeyRotation(t *testing.T) {
	var groupID [32]byte
	rand.Read(groupID[:])

	privKey1, pubKey1, err := generateKeyPair()
	require.NoError(t, err)
	privKey2, pubKey2, err := generateKeyPair()
	require.NoError(t, err)

	skm1, err := NewSenderKeyManager(groupID, pubKey1, privKey1)
	require.NoError(t, err)
	skm2, err := NewSenderKeyManager(groupID, pubKey2, privKey2)
	require.NoError(t, err)

	// Initial distribution
	distributions, err := skm1.CreateDistributions([][32]byte{pubKey2})
	require.NoError(t, err)
	err = skm2.ProcessDistribution(distributions[0])
	require.NoError(t, err)

	initialKeyID := skm1.GetCurrentKeyID()

	// Rotate key (simulating member removal)
	rotatedDistributions, err := skm1.RotateSenderKey([][32]byte{pubKey2})
	require.NoError(t, err)
	require.Len(t, rotatedDistributions, 1)

	// Key ID should have incremented
	newKeyID := skm1.GetCurrentKeyID()
	assert.Greater(t, newKeyID, initialKeyID)

	// Member 2 processes the new distribution
	err = skm2.ProcessDistribution(rotatedDistributions[0])
	require.NoError(t, err)

	// New messages should work with the new key
	plaintext := []byte("Message after key rotation")
	encryptedMsg, err := skm1.EncryptMessage(plaintext)
	require.NoError(t, err)
	assert.Equal(t, newKeyID, encryptedMsg.KeyID)

	decrypted, err := skm2.DecryptMessage(encryptedMsg)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestSenderKeyRemovePeer(t *testing.T) {
	var groupID [32]byte
	rand.Read(groupID[:])

	privKey1, pubKey1, err := generateKeyPair()
	require.NoError(t, err)
	privKey2, pubKey2, err := generateKeyPair()
	require.NoError(t, err)

	skm1, err := NewSenderKeyManager(groupID, pubKey1, privKey1)
	require.NoError(t, err)

	// Add peer
	distributions, err := skm1.CreateDistributions([][32]byte{pubKey2})
	require.NoError(t, err)

	// Simulate receiving member 2's key
	skm2, err := NewSenderKeyManager(groupID, pubKey2, privKey2)
	require.NoError(t, err)
	distributions2, err := skm2.CreateDistributions([][32]byte{pubKey1})
	require.NoError(t, err)

	err = skm1.ProcessDistribution(distributions2[0])
	require.NoError(t, err)

	assert.True(t, skm1.HasPeerKey(pubKey2))
	assert.Equal(t, 1, skm1.GetPeerCount())

	// Remove peer
	skm1.RemovePeer(pubKey2)

	assert.False(t, skm1.HasPeerKey(pubKey2))
	assert.Equal(t, 0, skm1.GetPeerCount())

	// Attempting to use distribution should still work (processed independently)
	_ = distributions // Distribution was created but receiver was removed
}

func TestSenderKeyMessageSerialization(t *testing.T) {
	var groupID, senderPK [32]byte
	rand.Read(groupID[:])
	rand.Read(senderPK[:])

	original := &SenderKeyMessage{
		GroupID:         groupID,
		SenderPublicKey: senderPK,
		KeyID:           42,
		Counter:         12345,
		Ciphertext:      []byte("encrypted message data"),
	}

	data, err := SerializeSenderKeyMessage(original)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	deserialized, err := DeserializeSenderKeyMessage(data)
	require.NoError(t, err)

	assert.Equal(t, original.GroupID, deserialized.GroupID)
	assert.Equal(t, original.SenderPublicKey, deserialized.SenderPublicKey)
	assert.Equal(t, original.KeyID, deserialized.KeyID)
	assert.Equal(t, original.Counter, deserialized.Counter)
	assert.Equal(t, original.Ciphertext, deserialized.Ciphertext)
}

func TestSenderKeyDistributionSerialization(t *testing.T) {
	var groupID, senderPK [32]byte
	var nonce [24]byte
	rand.Read(groupID[:])
	rand.Read(senderPK[:])
	rand.Read(nonce[:])

	original := &SenderKeyDistribution{
		GroupID:         groupID,
		SenderPublicKey: senderPK,
		KeyID:           7,
		Nonce:           nonce,
		EncryptedKey:    []byte("encrypted sender key material"),
	}

	data, err := SerializeSenderKeyDistribution(original)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	deserialized, err := DeserializeSenderKeyDistribution(data)
	require.NoError(t, err)

	assert.Equal(t, original.GroupID, deserialized.GroupID)
	assert.Equal(t, original.SenderPublicKey, deserialized.SenderPublicKey)
	assert.Equal(t, original.KeyID, deserialized.KeyID)
	assert.Equal(t, original.Nonce, deserialized.Nonce)
	assert.Equal(t, original.EncryptedKey, deserialized.EncryptedKey)
}

func TestSenderKeyMultipleMembers(t *testing.T) {
	var groupID [32]byte
	rand.Read(groupID[:])

	// Create 5 group members
	const numMembers = 5
	members := make([]*SenderKeyManager, numMembers)
	pubKeys := make([][32]byte, numMembers)

	for i := 0; i < numMembers; i++ {
		privKey, pubKey, err := generateKeyPair()
		require.NoError(t, err)

		skm, err := NewSenderKeyManager(groupID, pubKey, privKey)
		require.NoError(t, err)

		members[i] = skm
		pubKeys[i] = pubKey
	}

	// Each member distributes their key to all others
	for i := 0; i < numMembers; i++ {
		// Get all other public keys
		otherKeys := make([][32]byte, 0, numMembers-1)
		for j := 0; j < numMembers; j++ {
			if i != j {
				otherKeys = append(otherKeys, pubKeys[j])
			}
		}

		distributions, err := members[i].CreateDistributions(otherKeys)
		require.NoError(t, err)
		require.Len(t, distributions, numMembers-1)

		// Each other member processes the distribution
		distIdx := 0
		for j := 0; j < numMembers; j++ {
			if i != j {
				err = members[j].ProcessDistribution(distributions[distIdx])
				require.NoError(t, err)
				distIdx++
			}
		}
	}

	// Verify each member has keys from all others
	for i := 0; i < numMembers; i++ {
		assert.Equal(t, numMembers-1, members[i].GetPeerCount())
		for j := 0; j < numMembers; j++ {
			if i != j {
				assert.True(t, members[i].HasPeerKey(pubKeys[j]))
			}
		}
	}

	// Member 0 sends a message
	plaintext := []byte("Hello everyone!")
	encryptedMsg, err := members[0].EncryptMessage(plaintext)
	require.NoError(t, err)

	// All other members should be able to decrypt
	for i := 1; i < numMembers; i++ {
		decrypted, err := members[i].DecryptMessage(encryptedMsg)
		require.NoError(t, err, "Member %d failed to decrypt", i)
		assert.Equal(t, plaintext, decrypted)
	}
}

func TestSenderKeyDecryptionErrors(t *testing.T) {
	var groupID, otherGroupID [32]byte
	rand.Read(groupID[:])
	rand.Read(otherGroupID[:])

	privKey1, pubKey1, err := generateKeyPair()
	require.NoError(t, err)
	privKey2, pubKey2, err := generateKeyPair()
	require.NoError(t, err)

	skm1, err := NewSenderKeyManager(groupID, pubKey1, privKey1)
	require.NoError(t, err)
	skm2, err := NewSenderKeyManager(groupID, pubKey2, privKey2)
	require.NoError(t, err)

	// Setup key distribution
	distributions, err := skm1.CreateDistributions([][32]byte{pubKey2})
	require.NoError(t, err)
	err = skm2.ProcessDistribution(distributions[0])
	require.NoError(t, err)

	// Create a valid message
	plaintext := []byte("Test message")
	encryptedMsg, err := skm1.EncryptMessage(plaintext)
	require.NoError(t, err)

	// Test: Wrong group ID
	wrongGroupMsg := &SenderKeyMessage{
		GroupID:         otherGroupID,
		SenderPublicKey: encryptedMsg.SenderPublicKey,
		KeyID:           encryptedMsg.KeyID,
		Counter:         encryptedMsg.Counter,
		Ciphertext:      encryptedMsg.Ciphertext,
	}
	_, err = skm2.DecryptMessage(wrongGroupMsg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "group ID mismatch")

	// Test: Unknown sender
	var unknownPK [32]byte
	rand.Read(unknownPK[:])
	unknownSenderMsg := &SenderKeyMessage{
		GroupID:         groupID,
		SenderPublicKey: unknownPK,
		KeyID:           encryptedMsg.KeyID,
		Counter:         encryptedMsg.Counter,
		Ciphertext:      encryptedMsg.Ciphertext,
	}
	_, err = skm2.DecryptMessage(unknownSenderMsg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no sender key for peer")

	// Test: Wrong key ID
	wrongKeyIDMsg := &SenderKeyMessage{
		GroupID:         groupID,
		SenderPublicKey: encryptedMsg.SenderPublicKey,
		KeyID:           999,
		Counter:         encryptedMsg.Counter,
		Ciphertext:      encryptedMsg.Ciphertext,
	}
	_, err = skm2.DecryptMessage(wrongKeyIDMsg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key ID mismatch")
}

func TestSenderKeyNeedsRotation(t *testing.T) {
	var groupID [32]byte
	rand.Read(groupID[:])

	privKey, pubKey, err := generateKeyPair()
	require.NoError(t, err)

	skm, err := NewSenderKeyManager(groupID, pubKey, privKey)
	require.NoError(t, err)

	// Set a low threshold for testing
	skm.SetMaxMessageCounter(10)

	assert.False(t, skm.NeedsKeyRotation())

	// Send messages up to the threshold
	for i := 0; i < 10; i++ {
		_, err := skm.EncryptMessage([]byte("test"))
		require.NoError(t, err)
	}

	assert.True(t, skm.NeedsKeyRotation())
}

func BenchmarkSenderKeyEncrypt(b *testing.B) {
	var groupID [32]byte
	rand.Read(groupID[:])

	privKey, pubKey, err := generateKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	skm, err := NewSenderKeyManager(groupID, pubKey, privKey)
	if err != nil {
		b.Fatal(err)
	}

	// Increase max counter to avoid rotation during benchmark
	skm.SetMaxMessageCounter(uint64(b.N + 1000))

	plaintext := make([]byte, 1000) // 1KB message
	rand.Read(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := skm.EncryptMessage(plaintext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSenderKeyDecrypt(b *testing.B) {
	var groupID [32]byte
	rand.Read(groupID[:])

	privKey1, pubKey1, _ := generateKeyPair()
	privKey2, pubKey2, _ := generateKeyPair()

	skm1, _ := NewSenderKeyManager(groupID, pubKey1, privKey1)
	skm2, _ := NewSenderKeyManager(groupID, pubKey2, privKey2)

	distributions, _ := skm1.CreateDistributions([][32]byte{pubKey2})
	skm2.ProcessDistribution(distributions[0])

	plaintext := make([]byte, 1000)
	rand.Read(plaintext)

	// Create many messages for decryption
	skm1.SetMaxMessageCounter(uint64(b.N + 1000))
	messages := make([]*SenderKeyMessage, b.N)
	for i := 0; i < b.N; i++ {
		messages[i], _ = skm1.EncryptMessage(plaintext)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := skm2.DecryptMessage(messages[i])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGroupBroadcastComparison(b *testing.B) {
	// This benchmark compares O(1) sender-key encryption vs O(n) per-member encryption
	memberCounts := []int{10, 100, 1000}

	for _, n := range memberCounts {
		b.Run(fmt.Sprintf("SenderKey_%d_members", n), func(b *testing.B) {
			var groupID [32]byte
			rand.Read(groupID[:])

			privKey, pubKey, _ := generateKeyPair()
			skm, _ := NewSenderKeyManager(groupID, pubKey, privKey)
			skm.SetMaxMessageCounter(uint64(b.N + 1000))

			plaintext := []byte("Group message for benchmark testing")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// With sender-key: single encryption regardless of member count
				_, _ = skm.EncryptMessage(plaintext)
			}
		})
	}
}
