package async

import (
	"testing"

	"github.com/opd-ai/toxcore/crypto"
)

// BenchmarkAsyncManagerSendMessage benchmarks high-level async manager message sending
func BenchmarkAsyncManagerSendMessage(b *testing.B) {
	// Create test key pairs
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate sender key pair:", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	// Create mock transport
	mockTransport := &MockTransport{}

	// Create async manager
	manager, err := NewAsyncManager(senderKeyPair, mockTransport, b.TempDir())
	if err != nil {
		b.Fatal("Failed to create async manager:", err)
	}
	manager.Start()
	defer manager.Stop()

	// Set recipient as online (has pre-keys)
	manager.SetFriendOnlineStatus(recipientKeyPair.Public, true)

	testMessage := "test message for async manager benchmarking"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := manager.SendAsyncMessage(recipientKeyPair.Public, testMessage, MessageTypeNormal)
		if err != nil {
			b.Fatal("Failed to send async message:", err)
		}
	}
}

// BenchmarkAsyncManagerSetFriendOnlineStatus benchmarks friend status updates
func BenchmarkAsyncManagerSetFriendOnlineStatus(b *testing.B) {
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate sender key pair:", err)
	}

	mockTransport := &MockTransport{}

	manager, err := NewAsyncManager(senderKeyPair, mockTransport, b.TempDir())
	if err != nil {
		b.Fatal("Failed to create async manager:", err)
	}
	manager.Start()
	defer manager.Stop()

	// Pre-generate friend keys
	friendKeys := make([][32]byte, b.N)
	for i := 0; i < b.N; i++ {
		friendKeyPair, err := crypto.GenerateKeyPair()
		if err != nil {
			b.Fatal("Failed to generate friend key pair:", err)
		}
		friendKeys[i] = friendKeyPair.Public
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		manager.SetFriendOnlineStatus(friendKeys[i], i%2 == 0) // Alternate online/offline
	}
}

// BenchmarkAsyncManagerCanSendMessage benchmarks checking send capability
func BenchmarkAsyncManagerCanSendMessage(b *testing.B) {
	senderKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate sender key pair:", err)
	}

	recipientKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate recipient key pair:", err)
	}

	mockTransport := &MockTransport{}

	manager, err := NewAsyncManager(senderKeyPair, mockTransport, b.TempDir())
	if err != nil {
		b.Fatal("Failed to create async manager:", err)
	}
	manager.Start()
	defer manager.Stop()

	// Set recipient as online (has pre-keys)
	manager.SetFriendOnlineStatus(recipientKeyPair.Public, true)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = manager.CanSendAsyncMessage(recipientKeyPair.Public)
	}
}

// BenchmarkAsyncManagerGetStorageStats benchmarks storage statistics retrieval
func BenchmarkAsyncManagerGetStorageStats(b *testing.B) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		b.Fatal("Failed to generate key pair:", err)
	}

	mockTransport := &MockTransport{}

	manager, err := NewAsyncManager(keyPair, mockTransport, b.TempDir())
	if err != nil {
		b.Fatal("Failed to create async manager:", err)
	}
	manager.Start()
	defer manager.Stop()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = manager.GetStorageStats()
	}
}
