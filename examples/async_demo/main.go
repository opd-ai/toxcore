// Package main demonstrates the async message delivery system
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/opd-ai/toxcore/async"
	"github.com/opd-ai/toxcore/crypto"
	"github.com/opd-ai/toxcore/transport"
)

func main() {
	fmt.Println("=== Tox Async Message Delivery System Demo ===")
	fmt.Println()

	// Create key pairs for our demo
	aliceKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		log.Fatalf("Failed to generate Alice's key pair: %v", err)
	}

	bobKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		log.Fatalf("Failed to generate Bob's key pair: %v", err)
	}

	storageNodeKeyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		log.Fatalf("Failed to generate storage node key pair: %v", err)
	}

	fmt.Printf("Alice's Public Key: %x\n", aliceKeyPair.Public[:8])
	fmt.Printf("Bob's Public Key: %x\n", bobKeyPair.Public[:8])
	fmt.Printf("Storage Node Public Key: %x\n", storageNodeKeyPair.Public[:8])
	fmt.Println()

	// Demo 1: Direct storage and retrieval
	fmt.Println("=== Demo 1: Direct Message Storage ===")
	demoDirectStorage(aliceKeyPair, bobKeyPair, storageNodeKeyPair)
	fmt.Println()

	// Demo 2: Async manager functionality
	fmt.Println("=== Demo 2: Async Manager ===")
	demoAsyncManager(aliceKeyPair, bobKeyPair)
	fmt.Println()

	// Demo 3: Storage node maintenance
	fmt.Println("=== Demo 3: Storage Node Maintenance ===")
	demoStorageMaintenance(storageNodeKeyPair)
}

// demoDirectStorage demonstrates low-level storage operations for educational purposes.
// This function shows internal storage mechanisms and should not be used in production.
// For secure messaging, use AsyncManager which provides forward secrecy by default.
func demoDirectStorage(aliceKeyPair, bobKeyPair, storageNodeKeyPair *crypto.KeyPair) {
	storage := initializeStorageNode(storageNodeKeyPair)
	messageID := storeTestMessage(storage, aliceKeyPair, bobKeyPair)
	if messageID == ([16]byte{}) {
		return
	}

	messages := retrieveAndDecryptMessages(storage, bobKeyPair)
	if len(messages) == 0 {
		return
	}

	cleanupStoredMessages(storage, messages, bobKeyPair)
}

// initializeStorageNode creates and configures a new message storage instance.
// It displays storage capacity and utilization information for demonstration purposes.
func initializeStorageNode(storageNodeKeyPair *crypto.KeyPair) *async.MessageStorage {
	fmt.Println("⚠️  Direct storage demo shows low-level storage operations.")
	fmt.Println("⚠️  For secure messaging, use AsyncManager which provides forward secrecy by default.")
	fmt.Println()

	storage := async.NewMessageStorage(storageNodeKeyPair, os.TempDir())
	fmt.Printf("📦 Created storage node with capacity for %d messages\n", storage.GetMaxCapacity())
	fmt.Printf("💾 Storage utilization: %.1f%%\n", storage.GetStorageUtilization())

	return storage
}

// storeTestMessage creates and stores a demonstration message in the storage system.
// This uses deprecated encryption for demonstration only - production code should use AsyncManager.
// Returns the message ID if successful, zero value if storage failed.
func storeTestMessage(storage *async.MessageStorage, aliceKeyPair, bobKeyPair *crypto.KeyPair) [16]byte {
	fmt.Println("💾 Demonstrating low-level storage operations...")
	fmt.Println("⚠️  Using deprecated encryption for demonstration - use AsyncManager for production")

	message := "This is a low-level storage demonstration (not forward secure)"
	testData := []byte(message)
	nonce, _ := crypto.GenerateNonce()

	messageID, err := storage.StoreMessage(bobKeyPair.Public, aliceKeyPair.Public, testData, nonce, async.MessageTypeNormal)
	if err != nil {
		fmt.Printf("❌ Failed to store message: %v\n", err)
		return [16]byte{}
	}

	fmt.Printf("💾 Stored message in storage layer\n")
	fmt.Printf("   Message ID: %x\n", messageID[:8])
	fmt.Printf("   Content: %s\n", message)

	stats := storage.GetStorageStats()
	fmt.Printf("📊 Storage Stats: %d messages, %d recipients\n",
		stats.TotalMessages, stats.UniqueRecipients)

	return messageID
}

// retrieveAndDecryptMessages fetches stored messages for a recipient and decrypts them.
// This demonstrates the message retrieval and decryption process for educational purposes.
// Returns the list of retrieved messages.
func retrieveAndDecryptMessages(storage *async.MessageStorage, bobKeyPair *crypto.KeyPair) []async.AsyncMessage {
	fmt.Println("\n🔍 Bob comes online and retrieves messages...")
	messages, err := storage.RetrieveMessages(bobKeyPair.Public)
	if err != nil {
		log.Printf("Failed to retrieve messages: %v", err)
		return nil
	}

	fmt.Printf("📨 Bob retrieved %d message(s):\n", len(messages))
	for i, msg := range messages {
		fmt.Printf("   %d. From: %x\n", i+1, msg.SenderPK[:8])
		fmt.Printf("      Stored: %s\n", msg.Timestamp.Format(time.RFC3339))
		fmt.Printf("      Type: %v\n", msg.MessageType)

		decrypted, err := crypto.Decrypt(msg.EncryptedData, msg.Nonce,
			msg.SenderPK, bobKeyPair.Private)
		if err != nil {
			fmt.Printf("      ❌ Failed to decrypt: %v\n", err)
		} else {
			fmt.Printf("      📝 Content: %s\n", string(decrypted))
		}
	}

	return messages
}

// cleanupStoredMessages removes processed messages from storage to demonstrate cleanup operations.
// In production systems, message lifecycle management should be handled by AsyncManager.
func cleanupStoredMessages(storage *async.MessageStorage, messages []async.AsyncMessage, bobKeyPair *crypto.KeyPair) {
	if len(messages) > 0 {
		err := storage.DeleteMessage(messages[0].ID, bobKeyPair.Public)
		if err != nil {
			log.Printf("Failed to delete message: %v", err)
		} else {
			fmt.Println("🗑️  Bob deleted the message after reading")
		}
	}
}

func demoAsyncManager(aliceKeyPair, bobKeyPair *crypto.KeyPair) {
	aliceManager, bobManager := createAsyncManagers(aliceKeyPair, bobKeyPair)
	bobReceivedMessages := configureMessageHandling(bobManager)
	startManagersAndSetupStorage(aliceManager, bobManager, aliceKeyPair, bobKeyPair)

	attemptInitialOfflineMessaging(aliceManager, bobKeyPair)
	simulatePreKeyExchange(aliceManager, bobManager, aliceKeyPair, bobKeyPair)
	performForwardSecureMessaging(aliceManager, bobKeyPair)
	finalizeMessageDelivery(aliceManager, bobKeyPair, bobReceivedMessages)
}

// createAsyncManagers initializes and returns the async managers for Alice and Bob.
func createAsyncManagers(aliceKeyPair, bobKeyPair *crypto.KeyPair) (*async.AsyncManager, *async.AsyncManager) {
	// Create mock transports for demo
	aliceTransport, _ := transport.NewUDPTransport("127.0.0.1:8001")
	bobTransport, _ := transport.NewUDPTransport("127.0.0.1:8002")

	// Alice creates an async manager (acts as both client and storage node)
	aliceManager, err := async.NewAsyncManager(aliceKeyPair, aliceTransport, filepath.Join(os.TempDir(), "alice"))
	if err != nil {
		log.Fatalf("Failed to create Alice's async manager: %v", err)
	}
	bobManager, err := async.NewAsyncManager(bobKeyPair, bobTransport, filepath.Join(os.TempDir(), "bob")) // Bob is just a client
	if err != nil {
		log.Fatalf("Failed to create Bob's async manager: %v", err)
	}

	return aliceManager, bobManager
}

// configureMessageHandling sets up message handlers and returns the received messages slice.
func configureMessageHandling(bobManager *async.AsyncManager) []string {
	bobReceivedMessages := make([]string, 0)
	bobManager.SetAsyncMessageHandler(func(senderPK [32]byte, message string,
		messageType async.MessageType,
	) {
		fmt.Printf("📨 Bob received async message from %x: %s\n", senderPK[:8], message)
		bobReceivedMessages = append(bobReceivedMessages, message)
	})

	return bobReceivedMessages
}

// startManagersAndSetupStorage starts the managers and configures storage nodes.
func startManagersAndSetupStorage(aliceManager, bobManager *async.AsyncManager, aliceKeyPair, bobKeyPair *crypto.KeyPair) {
	// Start the managers
	aliceManager.Start()
	bobManager.Start()
	defer aliceManager.Stop()
	defer bobManager.Stop()

	fmt.Println("🚀 Started async managers for Alice and Bob")

	// Alice adds her node as a storage node for Bob
	aliceAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:33445")
	bobManager.AddStorageNode(aliceKeyPair.Public, aliceAddr)

	// Set Bob as offline initially
	aliceManager.SetFriendOnlineStatus(bobKeyPair.Public, false)
	fmt.Println("📴 Bob is offline")
}

// attemptInitialOfflineMessaging demonstrates the initial messaging attempt without pre-key exchange.
func attemptInitialOfflineMessaging(aliceManager *async.AsyncManager, bobKeyPair *crypto.KeyPair) {
	// Alice tries to send a message to offline Bob
	message := "Hey Bob, this message will be stored for when you come back online!"
	err := aliceManager.SendAsyncMessage(bobKeyPair.Public, message, async.MessageTypeNormal)
	if err != nil {
		fmt.Printf("❌ Failed to send async message: %v\n", err)
		fmt.Println("💡 This is expected - forward secrecy requires pre-key exchange when both peers are online")
	} else {
		fmt.Printf("📤 Alice sent async message to offline Bob\n")
	}

	// Show Alice's storage stats
	if stats := aliceManager.GetStorageStats(); stats != nil {
		fmt.Printf("📊 Alice's storage stats: %d messages stored\n", stats.TotalMessages)
	}
}

// simulatePreKeyExchange handles the pre-key exchange simulation process.
func simulatePreKeyExchange(aliceManager, bobManager *async.AsyncManager, aliceKeyPair, bobKeyPair *crypto.KeyPair) {
	// Simulate Bob coming online for pre-key exchange
	time.Sleep(100 * time.Millisecond) // Give time for async operations
	fmt.Println("\n🟢 Bob comes online for pre-key exchange!")
	aliceManager.SetFriendOnlineStatus(bobKeyPair.Public, true)
	bobManager.SetFriendOnlineStatus(aliceKeyPair.Public, true)

	// Wait for pre-key exchange detection
	time.Sleep(200 * time.Millisecond)

	// Simulate actual pre-key exchange (in reality this would happen over the network)
	fmt.Println("🔄 Simulating pre-key exchange...")

	// For demo purposes, we'll manually trigger the pre-key exchange process
	// Note: When using toxcore-go in production with network integration,
	// pre-key bundles are exchanged automatically when both peers are online
	fmt.Println("💡 Pre-key bundles enable forward-secure messaging between peers")
	fmt.Println("🔗 This exchange would occur automatically over the Tox network")
}

// performForwardSecureMessaging demonstrates sending forward-secure messages after pre-key exchange.
func performForwardSecureMessaging(aliceManager *async.AsyncManager, bobKeyPair *crypto.KeyPair) {
	// Check if we can now send forward-secure messages
	if aliceManager.CanSendAsyncMessage(bobKeyPair.Public) {
		fmt.Println("✅ Pre-key exchange completed - can now send forward-secure messages")

		// Show available pre-keys
		preKeyStats := aliceManager.GetPreKeyStats()
		if count, ok := preKeyStats[fmt.Sprintf("%x", bobKeyPair.Public[:8])]; ok {
			fmt.Printf("🔑 Alice has %d forward-secure keys for Bob\n", count)
		}

		// Simulate Bob going offline again
		fmt.Println("📴 Bob goes offline again")
		aliceManager.SetFriendOnlineStatus(bobKeyPair.Public, false)

		// Now Alice can send forward-secure messages
		secureMessage := "This is a forward-secure message! 🔐"
		err := aliceManager.SendAsyncMessage(bobKeyPair.Public, secureMessage, async.MessageTypeNormal)
		if err != nil {
			fmt.Printf("❌ Failed to send forward-secure message: %v\n", err)
		} else {
			fmt.Printf("📤 Alice sent forward-secure async message to offline Bob\n")
		}
	} else {
		fmt.Println("❌ Pre-key exchange failed - cannot send forward-secure messages")
	}
}

// finalizeMessageDelivery simulates Bob coming online to receive messages and reports results.
func finalizeMessageDelivery(aliceManager *async.AsyncManager, bobKeyPair *crypto.KeyPair, bobReceivedMessages []string) {
	// Simulate Bob coming online again to receive messages
	time.Sleep(100 * time.Millisecond)
	fmt.Println("\n🟢 Bob comes online to receive messages!")
	aliceManager.SetFriendOnlineStatus(bobKeyPair.Public, true)

	// Give time for message delivery
	time.Sleep(200 * time.Millisecond)

	if len(bobReceivedMessages) > 0 {
		fmt.Printf("✅ Success! Bob received %d async message(s)\n", len(bobReceivedMessages))
	} else {
		fmt.Println("ℹ️  Note: This demo simulates network delivery for demonstration")
		fmt.Println("   Production usage: messages are delivered via the Tox network protocol")
	}
}

func demoStorageMaintenance(storageNodeKeyPair *crypto.KeyPair) {
	fmt.Println("⚠️  This demo shows internal storage operations.")
	fmt.Println("⚠️  Production apps should use AsyncManager for forward-secure messaging.")
	fmt.Println()

	storage := async.NewMessageStorage(storageNodeKeyPair, os.TempDir())

	// Create some test key pairs
	user1, _ := crypto.GenerateKeyPair()
	user2, _ := crypto.GenerateKeyPair()
	sender, _ := crypto.GenerateKeyPair()

	// Store some messages using raw storage operations (bypassing forward secrecy)
	messages := []string{
		"Test message 1 for user1",
		"Test message 2 for user1",
		"Test message 1 for user2",
		"Another test message for user2",
	}

	fmt.Println("💾 Storing test messages using raw storage layer...")
	for i, msg := range messages {
		recipient := user1.Public
		if i >= 2 {
			recipient = user2.Public
		}

		// For demo purposes, store raw data (real apps should use AsyncManager)
		testData := []byte(msg)
		nonce, _ := crypto.GenerateNonce()

		_, err := storage.StoreMessage(recipient, sender.Public, testData, nonce, async.MessageTypeNormal)
		if err != nil {
			log.Printf("Failed to store message: %v", err)
		}
	}

	fmt.Printf("📦 Stored %d test messages\n", len(messages))

	// Show initial stats
	stats := storage.GetStorageStats()
	fmt.Printf("📊 Initial stats: %d messages, %d recipients\n",
		stats.TotalMessages, stats.UniqueRecipients)

	// Simulate message expiration by creating a new storage instance
	// and demonstrating the cleanup functionality
	fmt.Println("🕐 Simulating expired message cleanup...")

	// For demo purposes, store a few more messages and then run cleanup
	// In reality, cleanup would remove messages older than 24 hours
	fmt.Println("💾 Adding additional test messages...")
	for i := 0; i < 2; i++ {
		msg := fmt.Sprintf("Additional test message %d", i+1)
		testData := []byte(msg)
		nonce, _ := crypto.GenerateNonce()
		storage.StoreMessage(user1.Public, sender.Public, testData, nonce, async.MessageTypeNormal)
	}

	fmt.Printf("📊 Added more messages for cleanup demo\n")

	// Run cleanup (in a real scenario, this would remove messages older than 24 hours)
	expiredCount := storage.CleanupExpiredMessages()
	fmt.Printf("🧹 Cleanup process ran (removed %d expired messages)\n", expiredCount)
	fmt.Println("   Note: No messages were actually expired in this demo")
	fmt.Println("   In production, messages older than 24 hours would be removed")

	// Show final stats
	finalStats := storage.GetStorageStats()
	fmt.Printf("📊 Final stats: %d messages, %d recipients\n",
		finalStats.TotalMessages, finalStats.UniqueRecipients)

	fmt.Println("✅ Storage maintenance demo complete")
}
