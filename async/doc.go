// Package async implements forward-secure asynchronous messaging with identity
// obfuscation for the Tox protocol. This unofficial extension provides offline
// messaging capabilities while maintaining Tox's decentralized nature and
// security properties.
//
// # Overview
//
// The async package provides comprehensive offline messaging with advanced
// privacy protections:
//
//   - Forward secrecy via one-time pre-keys (Signal Protocol pattern)
//   - Identity obfuscation using HKDF-derived pseudonyms
//   - Epoch-based key rotation (6-hour epochs)
//   - Message padding for traffic analysis resistance
//   - Cover traffic for activity pattern protection
//   - Distributed storage across peer network
//
// # Core Components
//
// The package is organized around several key components:
//
//   - AsyncManager: Main orchestration layer integrating with Tox
//   - AsyncClient: Client-side operations for sending and retrieving messages
//   - MessageStorage: Dual-mode storage (legacy + obfuscated) with capacity limits
//   - ForwardSecurityManager: Pre-key lifecycle and forward secrecy guarantees
//   - ObfuscationManager: Cryptographic pseudonym generation and rotation
//   - PreKeyStore: Encrypted on-disk pre-key bundles with secure wiping
//   - EpochManager: Deterministic time-based epoch calculation
//   - RetrievalScheduler: Randomized retrieval with cover traffic
//
// # AsyncManager
//
// AsyncManager is the primary integration point with the main Tox system:
//
//	manager, err := async.NewAsyncManager(keyPair, transport, "/path/to/data")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Set handler for incoming async messages
//	manager.SetMessageHandler(func(senderPK [32]byte, message string, msgType async.MessageType) {
//	    fmt.Printf("Async message from %x: %s\n", senderPK[:8], message)
//	})
//
//	// Start the async messaging service
//	manager.Start()
//	defer manager.Stop()
//
//	// Send message to offline friend
//	err = manager.SendAsyncMessage(friendPublicKey, "Hello!", async.MessageTypeNormal)
//
// # AsyncClient
//
// AsyncClient handles direct communication with storage nodes:
//
//	client := async.NewAsyncClient(keyPair, transport)
//
//	// Add known storage nodes
//	client.AddStorageNode(nodePublicKey, nodeAddress)
//
//	// Register known senders for message decryption
//	client.AddKnownSender(friendPublicKey)
//
//	// Configure retrieval behavior
//	client.SetRetrieveTimeout(5 * time.Second)
//	client.SetCollectionTimeout(10 * time.Second)
//	client.SetParallelQueries(true)
//
// # Forward Secrecy
//
// Forward secrecy is achieved through one-time pre-keys that are consumed
// after each message. The ForwardSecurityManager handles key lifecycle:
//
//	// Pre-keys are automatically managed via AsyncManager
//	// Manual pre-key exchange can be triggered:
//	manager.TriggerPreKeyExchange(friendPublicKey)
//
//	// Check available pre-keys for a peer
//	count := manager.GetAvailablePreKeyCount(friendPublicKey)
//
// Pre-key thresholds:
//   - PreKeyLowWatermark (10): Triggers automatic refresh
//   - PreKeyMinimum (5): Minimum required to send messages
//   - PreKeysPerPeer (100): Initial pre-keys generated per peer
//
// # Identity Obfuscation
//
// The ObfuscationManager generates cryptographic pseudonyms to hide real
// identities from storage nodes:
//
//	epochManager := async.NewEpochManager()
//	obfuscation := async.NewObfuscationManager(keyPair, epochManager)
//
//	// Recipient pseudonyms are deterministic for retrieval
//	recipientPseudo, _ := obfuscation.GenerateRecipientPseudonym(recipientPK, epoch)
//
//	// Sender pseudonyms are unique per message (unlinkable)
//	senderPseudo, _ := obfuscation.GenerateSenderPseudonym()
//
// # Epoch Management
//
// Epochs provide time-based pseudonym rotation for enhanced privacy:
//
//	epochManager := async.NewEpochManager()
//
//	// Get current epoch (6-hour periods since network genesis)
//	currentEpoch := epochManager.GetCurrentEpoch()
//
//	// Get epoch for a specific time
//	pastEpoch := epochManager.GetEpochAt(someTime)
//
//	// Validate epoch freshness
//	if epochManager.IsValidEpoch(messageEpoch) {
//	    // Process message
//	}
//
// Network genesis time is January 1, 2025 00:00:00 UTC for consistent
// epoch calculation across all nodes.
//
// # Message Storage
//
// MessageStorage provides dual-mode storage with capacity management:
//
//	storage := async.NewMessageStorage(keyPair, "/path/to/data")
//
//	// Store an obfuscated message
//	err := storage.StoreObfuscatedMessage(obfuscatedMsg)
//
//	// Retrieve messages by recipient pseudonym
//	messages, err := storage.GetObfuscatedMessages(recipientPseudonym)
//
//	// Update storage capacity based on available disk space
//	storage.UpdateCapacity()
//
// Storage limits:
//   - MinStorageCapacity: ~1,536 messages (~1MB)
//   - MaxStorageCapacity: ~1,536,000 messages (~1GB)
//   - MaxStorageTime: 24 hours before expiration
//   - MaxMessagesPerRecipient: 100 per recipient
//
// # Retrieval Scheduler
//
// RetrievalScheduler provides randomized retrieval with cover traffic to
// prevent activity pattern analysis:
//
//	scheduler := async.NewRetrievalScheduler(client)
//
//	// Configure retrieval behavior
//	scheduler.SetBaseInterval(5 * time.Minute)
//	scheduler.SetJitterPercent(50)          // Add up to 50% random delay
//	scheduler.SetCoverTrafficEnabled(true)  // Enable dummy retrievals
//	scheduler.SetCoverTrafficRatio(0.3)     // 30% cover traffic
//
//	scheduler.Start()
//	defer scheduler.Stop()
//
// # Message Types
//
// Two message types are supported:
//
//	const (
//	    MessageTypeNormal  // Regular text message
//	    MessageTypeAction  // Action message (like "/me" in IRC)
//	)
//
// # Message Padding
//
// Messages are automatically padded to standard sizes to prevent traffic
// analysis based on message length:
//
//   - Small messages: 256 bytes
//   - Medium messages: 1024 bytes
//   - Large messages: 4096 bytes
//
// # Security Properties
//
// The async package provides the following security guarantees:
//
//   - Forward Secrecy: One-time pre-keys ensure past messages cannot be
//     decrypted even if long-term keys are compromised
//   - Identity Obfuscation: Storage nodes cannot identify senders or
//     recipients through pseudonym analysis
//   - Unlinkable Messages: Each message uses a unique sender pseudonym
//   - Activity Privacy: Cover traffic and randomized timing prevent
//     tracking of user activity patterns
//   - HMAC Recipient Proof: Prevents spam while preserving anonymity
//   - Secure Key Storage: Pre-keys are encrypted on disk with secure wiping
//
// # Cryptographic Primitives
//
// The package uses industry-standard cryptographic primitives:
//
//   - AES-GCM: Authenticated encryption for message payloads
//   - HKDF (SHA-256): Key derivation for pseudonyms and session keys
//   - Ed25519: Digital signatures for pre-key authentication
//   - Curve25519: Key exchange for shared secrets
//   - crypto/rand: Cryptographically secure random number generation
//
// # Thread Safety
//
// All exported types are safe for concurrent access:
//
//   - AsyncManager uses sync.RWMutex for state protection
//   - AsyncClient uses sync.RWMutex and sync.Mutex for different operations
//   - MessageStorage uses sync.RWMutex for storage operations
//   - ForwardSecurityManager uses sync.RWMutex for pre-key access
//   - PreKeyStore uses sync.RWMutex for bundle management
//   - RetrievalScheduler uses sync.Mutex for scheduling state
//
// The package passes Go's race detector validation.
//
// # Integration
//
// The async package integrates with core toxcore-go infrastructure:
//
//   - Transport layer via transport.Transport interface
//   - Packet types: PacketAsyncPreKeyExchange, PacketAsyncRetrieveResponse
//   - Crypto package for key management via crypto.KeyPair
//   - Platform-specific storage detection via build tags
//
// # Error Handling
//
// Common errors returned by the package:
//
//	var (
//	    ErrMessageNotFound  // Message not found in storage
//	    ErrStorageFull      // Storage node at capacity
//	    ErrInvalidRecipient // Invalid recipient public key
//	    ErrRecipientOnline  // Use regular messaging instead
//	)
//
// All errors include context via fmt.Errorf wrapping for debugging.
//
// # Platform Support
//
// Storage capacity detection is platform-specific:
//
//   - Unix (storage_limits_unix.go): Uses syscall.Statfs
//   - Windows (storage_limits_windows.go): Uses GetDiskFreeSpaceExW
//
// # Performance
//
// The package includes several performance optimizations:
//
//   - Parallel storage node queries (configurable)
//   - In-memory pre-key bundle caching
//   - Adaptive storage capacity based on available disk space
//   - Benchmark tests for critical paths (see *_benchmark_test.go)
package async
