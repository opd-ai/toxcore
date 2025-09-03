package async

import (
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// BenchmarkStoreMessage benchmarks the core message storage operation
func BenchmarkStoreMessage(b *testing.B) {
	// Create test storage
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair for storage:", err)
	}
	storage := NewMessageStorage(keyPair, b.TempDir())

	// Pre-generate test data
	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate sender key pair:", err)
	}

	message := []byte("test message for benchmarking storage performance")
	encryptedData, nonce, err := encryptForRecipientInternal(message, recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		b.Fatal("Failed to encrypt message:", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := storage.StoreMessage(recipientKeyPair.Public, senderKeyPair.Public,
			encryptedData, nonce, MessageTypeNormal)
		if err != nil {
			b.Fatal("Failed to store message:", err)
		}
	}
}

// BenchmarkRetrieveMessages benchmarks message retrieval with varying message counts
func BenchmarkRetrieveMessages(b *testing.B) {
	// Create test storage
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair for storage:", err)
	}
	storage := NewMessageStorage(keyPair, b.TempDir())

	// Pre-generate test data
	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate sender key pair:", err)
	}

	message := []byte("test message for retrieval benchmarking")
	encryptedData, nonce, err := encryptForRecipientInternal(message, recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		b.Fatal("Failed to encrypt message:", err)
	}

	// Store 100 messages for realistic retrieval testing
	for i := 0; i < 100; i++ {
		_, err := storage.StoreMessage(recipientKeyPair.Public, senderKeyPair.Public,
			encryptedData, nonce, MessageTypeNormal)
		if err != nil {
			b.Fatal("Failed to store message:", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := storage.RetrieveMessages(recipientKeyPair.Public)
		if err != nil {
			b.Fatal("Failed to retrieve messages:", err)
		}
	}
}

// BenchmarkStoreObfuscatedMessage benchmarks obfuscated message storage
func BenchmarkStoreObfuscatedMessage(b *testing.B) {
	// Create test storage
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair for storage:", err)
	}
	storage := NewMessageStorage(keyPair, b.TempDir())

	// Pre-generate test data
	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate sender key pair:", err)
	}

	// Create obfuscation manager
	epochManager := NewEpochManager()
	obfuscationManager := NewObfuscationManager(senderKeyPair, epochManager)

	// Create test message and obfuscate it
	message := []byte("test message for obfuscated storage benchmarking")
	encryptedData, nonce, err := encryptForRecipientInternal(message, recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		b.Fatal("Failed to encrypt message:", err)
	}

	// Create forward secure message (simplified for benchmarking)
	testMessage := &ForwardSecureMessage{
		Type:          "forward_secure_message",
		MessageID:     [32]byte{1, 2, 3, 4},
		SenderPK:      senderKeyPair.Public,
		RecipientPK:   recipientKeyPair.Public,
		PreKeyID:      1,
		EncryptedData: encryptedData,
		Nonce:         nonce,
		MessageType:   MessageTypeNormal,
	}

	// Create a mock transport for AsyncClient methods
	mockTransport := &MockTransport{}

	// Create AsyncClient to use serialization and key derivation methods
	asyncClient := NewAsyncClient(senderKeyPair, mockTransport)

	// Serialize the message for obfuscation
	serializedMsg, err := asyncClient.serializeForwardSecureMessage(testMessage)
	if err != nil {
		b.Fatal("Failed to serialize message:", err)
	}

	// Derive shared secret for obfuscation
	sharedSecret, err := asyncClient.deriveSharedSecret(recipientKeyPair.Public)
	if err != nil {
		b.Fatal("Failed to derive shared secret:", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		obfuscatedMsg, err := obfuscationManager.CreateObfuscatedMessage(
			senderKeyPair.Private, recipientKeyPair.Public, serializedMsg, sharedSecret)
		if err != nil {
			b.Fatal("Failed to create obfuscated message:", err)
		}

		err = storage.StoreObfuscatedMessage(obfuscatedMsg)
		if err != nil {
			b.Fatal("Failed to store obfuscated message:", err)
		}
	}
}

// BenchmarkRetrieveObfuscatedMessages benchmarks obfuscated message retrieval
func BenchmarkRetrieveObfuscatedMessages(b *testing.B) {
	// Create test storage
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair for storage:", err)
	}
	storage := NewMessageStorage(keyPair, b.TempDir())

	// Pre-generate test data
	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate sender key pair:", err)
	}

	// Create obfuscation manager
	epochManager := NewEpochManager()
	obfuscationManager := NewObfuscationManager(senderKeyPair, epochManager)

	// Create a mock transport for AsyncClient methods
	mockTransport := &MockTransport{}
	asyncClient := NewAsyncClient(senderKeyPair, mockTransport)

	// Pre-populate with 50 obfuscated messages
	for i := 0; i < 50; i++ {
		message := []byte("test message for obfuscated retrieval benchmarking")
		encryptedData, nonce, err := encryptForRecipientInternal(message, recipientKeyPair.Public, senderKeyPair.Private)
		if err != nil {
			b.Fatal("Failed to encrypt message:", err)
		}

		testMessage := &ForwardSecureMessage{
			Type:          "forward_secure_message",
			MessageID:     [32]byte{byte(i)},
			SenderPK:      senderKeyPair.Public,
			RecipientPK:   recipientKeyPair.Public,
			PreKeyID:      1,
			EncryptedData: encryptedData,
			Nonce:         nonce,
			MessageType:   MessageTypeNormal,
		}

		serializedMsg, err := asyncClient.serializeForwardSecureMessage(testMessage)
		if err != nil {
			b.Fatal("Failed to serialize message:", err)
		}

		sharedSecret, err := asyncClient.deriveSharedSecret(recipientKeyPair.Public)
		if err != nil {
			b.Fatal("Failed to derive shared secret:", err)
		}

		obfuscatedMsg, err := obfuscationManager.CreateObfuscatedMessage(
			senderKeyPair.Private, recipientKeyPair.Public, serializedMsg, sharedSecret)
		if err != nil {
			b.Fatal("Failed to create obfuscated message:", err)
		}

		err = storage.StoreObfuscatedMessage(obfuscatedMsg)
		if err != nil {
			b.Fatal("Failed to store obfuscated message:", err)
		}
	}

	// Get current epoch and pseudonym for retrieval
	currentEpoch := epochManager.GetCurrentEpoch()
	recipientPseudonym, err := obfuscationManager.GenerateRecipientPseudonym(recipientKeyPair.Public, currentEpoch)
	if err != nil {
		b.Fatal("Failed to generate recipient pseudonym:", err)
	}
	epochs := []uint64{currentEpoch, currentEpoch - 1, currentEpoch - 2}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := storage.RetrieveMessagesByPseudonym(recipientPseudonym, epochs)
		if err != nil {
			b.Fatal("Failed to retrieve obfuscated messages:", err)
		}
	}
}
