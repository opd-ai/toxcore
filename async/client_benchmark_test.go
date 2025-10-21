package async

import (
	"testing"

	"github.com/opd-ai/toxforge/crypto"
)

// BenchmarkAsyncClientSendMessage benchmarks sending messages via AsyncClient
func BenchmarkAsyncClientSendMessage(b *testing.B) {
	// Create test clients
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate sender key pair:", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	mockTransport := &MockTransport{}
	client := NewAsyncClient(senderKeyPair, mockTransport)

	message := []byte("test message for benchmarking send performance")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := client.SendAsyncMessage(recipientKeyPair.Public, message, MessageTypeNormal)
		if err != nil {
			b.Fatal("Failed to send async message:", err)
		}
	}
}

// BenchmarkAsyncClientRetrieveMessages benchmarks retrieving messages via AsyncClient
func BenchmarkAsyncClientRetrieveMessages(b *testing.B) {
	// Create test clients
	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate sender key pair:", err)
	}

	mockTransport := &MockTransport{}
	client := NewAsyncClient(recipientKeyPair, mockTransport)

	// Add sender as known for decryption
	client.AddKnownSender(senderKeyPair.Public)

	// Pre-populate with messages using a sender client
	senderClient := NewAsyncClient(senderKeyPair, mockTransport)

	// Send 20 messages for retrieval benchmarking
	for i := 0; i < 20; i++ {
		message := []byte("test message for benchmarking retrieval performance")
		err := senderClient.SendAsyncMessage(recipientKeyPair.Public, message, MessageTypeNormal)
		if err != nil {
			b.Fatal("Failed to send async message:", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := client.RetrieveAsyncMessages()
		if err != nil {
			b.Fatal("Failed to retrieve async messages:", err)
		}
	}
}

// BenchmarkAsyncClientAddKnownSender benchmarks adding known senders
func BenchmarkAsyncClientAddKnownSender(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair:", err)
	}

	mockTransport := &MockTransport{}
	client := NewAsyncClient(keyPair, mockTransport)

	// Pre-generate sender keys
	senderKeys := make([][32]byte, b.N)
	for i := 0; i < b.N; i++ {
		senderKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			b.Fatal("Failed to generate sender key pair:", err)
		}
		senderKeys[i] = senderKeyPair.Public
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		client.AddKnownSender(senderKeys[i])
	}
}

// BenchmarkAsyncClientGetKnownSenders benchmarks retrieving known senders list
func BenchmarkAsyncClientGetKnownSenders(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair:", err)
	}

	mockTransport := &MockTransport{}
	client := NewAsyncClient(keyPair, mockTransport)

	// Add 100 known senders for realistic testing
	for i := 0; i < 100; i++ {
		senderKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			b.Fatal("Failed to generate sender key pair:", err)
		}
		client.AddKnownSender(senderKeyPair.Public)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = client.GetKnownSenders()
	}
}

// BenchmarkAsyncClientDecryptMessage benchmarks message decryption
func BenchmarkAsyncClientDecryptMessage(b *testing.B) {
	// Create test clients
	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate sender key pair:", err)
	}

	mockTransport := &MockTransport{}
	client := NewAsyncClient(recipientKeyPair, mockTransport)
	client.AddKnownSender(senderKeyPair.Public)

	// Create an obfuscated message for decryption benchmarking
	senderClient := NewAsyncClient(senderKeyPair, mockTransport)

	// Create a test message
	message := []byte("test message for decryption benchmarking")
	encryptedData, nonce, err := encryptForRecipientInternal(message, recipientKeyPair.Public, senderKeyPair.Private)
	if err != nil {
		b.Fatal("Failed to encrypt message:", err)
	}

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

	serializedMsg, err := senderClient.serializeForwardSecureMessage(testMessage)
	if err != nil {
		b.Fatal("Failed to serialize message:", err)
	}

	sharedSecret, err := senderClient.deriveSharedSecret(recipientKeyPair.Public)
	if err != nil {
		b.Fatal("Failed to derive shared secret:", err)
	}

	obfuscatedMsg, err := senderClient.obfuscation.CreateObfuscatedMessage(
		senderKeyPair.Private, recipientKeyPair.Public, serializedMsg, sharedSecret)
	if err != nil {
		b.Fatal("Failed to create obfuscated message:", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := client.decryptObfuscatedMessage(obfuscatedMsg)
		if err != nil {
			b.Fatal("Failed to decrypt obfuscated message:", err)
		}
	}
}
