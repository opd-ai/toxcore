// Package messaging provides the core message handling system for the Tox protocol.
//
// # Overview
//
// The messaging package implements secure message sending, delivery tracking, retry logic,
// and state management for peer-to-peer communication. It integrates with the Tox transport
// layer and cryptographic system to provide end-to-end encrypted messaging with forward
// secrecy properties.
//
// # Architecture
//
// The package is built around three core components:
//
//   - [MessageManager]: Central coordinator for message lifecycle, queue management,
//     and delivery tracking. Thread-safe for concurrent use.
//   - [Message]: Individual message representation with state machine for delivery tracking.
//   - [MessageTransport] and [KeyProvider] interfaces: Dependency injection points for
//     transport and encryption integration.
//
// # Security Properties
//
// The messaging package provides several security features:
//
//   - Traffic Analysis Resistance: Messages are automatically padded to standard sizes
//     (256B, 1024B, 4096B) using [padMessage] to prevent length-based traffic analysis.
//   - Deterministic Time Injection: The [TimeProvider] interface allows test injection
//     and prevents timing side-channel attacks in production.
//   - Encrypted Storage: Encrypted message content is base64-encoded to prevent
//     data corruption from null bytes or invalid UTF-8 sequences.
//   - Graceful Shutdown: The [MessageManager.Close] method ensures pending goroutines
//     complete cleanly without message loss.
//
// # Usage
//
// Basic usage pattern with the Tox core integration:
//
//	// Create message manager
//	mm := messaging.NewMessageManager()
//	defer mm.Close()
//
//	// Configure transport and encryption
//	mm.SetTransport(toxInstance)
//	mm.SetKeyProvider(toxInstance)
//
//	// Send a message
//	msg, err := mm.SendMessage(friendID, "Hello!", messaging.MessageTypeNormal)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Track delivery state changes
//	msg.OnDeliveryStateChange(func(m *messaging.Message, state messaging.MessageState) {
//	    log.Printf("Message %d state: %v", m.ID, state)
//	})
//
//	// Process pending messages in iteration loop
//	for running {
//	    mm.ProcessPendingMessages()
//	    time.Sleep(100 * time.Millisecond)
//	}
//
// # Message States
//
// Messages progress through a state machine:
//
//	Pending -> Sending -> Sent -> Delivered -> Read
//	             |
//	             v (on failure)
//	          Failed
//
// Failed messages are automatically retried up to [MessageManager.maxRetries] times
// with exponential backoff controlled by [MessageManager.retryInterval].
//
// # Integration with Tox Core
//
// The messaging package integrates with toxcore through two interfaces:
//
//   - [MessageTransport]: Implemented by Tox to send message packets over the network.
//   - [KeyProvider]: Implemented by Tox to provide friend public keys and self private key
//     for end-to-end encryption.
//
// The Tox instance implements both interfaces, enabling transparent encryption:
//
//	// In toxcore.go initialization:
//	t.messageManager.SetTransport(t)
//	t.messageManager.SetKeyProvider(t)
//
// # Testing
//
// For deterministic testing, inject a mock [TimeProvider]:
//
//	type MockTimeProvider struct {
//	    currentTime time.Time
//	}
//
//	func (m *MockTimeProvider) Now() time.Time { return m.currentTime }
//	func (m *MockTimeProvider) Since(t time.Time) time.Duration {
//	    return m.currentTime.Sub(t)
//	}
//
//	mm := messaging.NewMessageManager()
//	mm.SetTimeProvider(&MockTimeProvider{currentTime: fixedTime})
//
// # Concurrency
//
// [MessageManager] is safe for concurrent use from multiple goroutines.
// All methods use internal locking. [Message] instances are also thread-safe
// for state updates via [Message.SetState].
//
// # Persistence
//
// The messaging package supports optional persistence through the [MessageStore]
// interface. When configured, messages can be saved to and loaded from persistent
// storage, enabling recovery after restarts.
//
// To enable persistence:
//
//  1. Implement [MessageStore] for your storage backend (file, database, etc.)
//  2. Configure the store during initialization
//  3. Load existing messages on startup
//  4. Save messages periodically or before shutdown
//
// Example with file-based persistence:
//
//	type FileMessageStore struct {
//	    path string
//	}
//
//	func (s *FileMessageStore) Save(data []byte) error {
//	    return os.WriteFile(s.path, data, 0600)
//	}
//
//	func (s *FileMessageStore) Load() ([]byte, error) {
//	    data, err := os.ReadFile(s.path)
//	    if os.IsNotExist(err) {
//	        return nil, nil // First run, no data yet
//	    }
//	    return data, err
//	}
//
//	// During initialization:
//	mm := messaging.NewMessageManager()
//	mm.SetStore(&FileMessageStore{path: "messages.json"})
//	if err := mm.LoadMessages(); err != nil {
//	    log.Printf("Warning: could not load messages: %v", err)
//	}
//
//	// Before shutdown:
//	if err := mm.SaveMessages(); err != nil {
//	    log.Printf("Error saving messages: %v", err)
//	}
//	mm.Close()
//
// If no store is configured, messages are kept only in memory and lost on restart.
// This is acceptable for applications that don't need message history persistence.
package messaging
