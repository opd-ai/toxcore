# Tox Asynchronous Messaging Extension Specification

**Version**: 1.0  
**Date**: September 2, 2025  
**Status**: Implemented in toxcore-go v1.0+  

## Abstract

Specifies the Asynchronous Messaging extension for the Tox protocol, enabling offline message delivery through distributed storage nodes while preserving decentralization, privacy, and end-to-end encryption.

## Table of Contents

1. [Introduction](#introduction)
2. [Architecture Overview](#architecture-overview)
3. [Security Model](#security-model)
4. [Core Components](#core-components)
5. [Message Format](#message-format)
6. [Storage Protocol](#storage-protocol)
7. [Client Protocol](#client-protocol)
8. [Network Discovery](#network-discovery)
9. [Security Considerations](#security-considerations)
10. [Implementation Guidelines](#implementation-guidelines)
11. [API Reference](#api-reference)
12. [Examples](#examples)

## Introduction

### Motivation

The standard Tox protocol requires both parties to be online simultaneously. This extension provides distributed storage for offline message delivery.

### Design Goals

- **Decentralized**: No central servers required
- **End-to-End Encrypted**: Messages encrypted between sender and recipient
- **Forward Secrecy**: One-time pre-exchanged keys protect against future compromise
- **Spam Resistant**: Rate limiting and capacity controls prevent abuse
- **Temporary Storage**: Messages auto-expire after 24 hours
- **Backward Compatible**: Works alongside existing Tox messaging
- **Automatic Participation**: All users become storage nodes (1% of disk space)

### Scope

This extension is an **unofficial** addition to the Tox protocol. It provides forward-secure offline message storage/retrieval using pre-exchanged one-time keys, automatic storage node participation, dynamic storage capacity based on available disk space, automatic pre-key generation and exchange between friends, and integration with existing Tox friend management.

## Architecture Overview

```
┌─────────────┐    Store Message    ┌─────────────┐
│   Sender    │ ──────────────────► │ Storage     │
│  (Online)   │                     │ Node        │
└─────────────┘                     └─────────────┘
                                           │
                                           │ Retrieve
                                           ▼
┌─────────────┐    Retrieve Messages ┌─────────────┐
│ Recipient   │ ◄─────────────────── │ Storage     │
│ (Comes      │                     │ Node        │
│  Online)    │                     │             │
└─────────────┘                     └─────────────┘
```

### Components

1. **ForwardSecurityManager**: Manages pre-key generation, exchange, and forward-secure messaging
2. **PreKeyStore**: Handles on-disk storage and management of one-time keys
3. **AsyncClient**: Handles forward-secure message sending and retrieval
4. **MessageStorage**: Manages stored messages with dynamic capacity
5. **AsyncManager**: High-level integration with Tox instances and automatic pre-key exchange
6. **Automatic Storage Nodes**: All users participate as storage nodes
7. **Storage Capacity Manager**: Calculates optimal storage limits

### Automatic Storage Node Participation

When async messaging is enabled, every toxcore-go instance **automatically participates as a storage node**. `AsyncManager.Start()` initializes storage node functionality automatically.

- **Disk Space**: Uses **1% of available disk space** (min 1 MB / ~1,536 messages, max 1 GB / ~1,536,000 messages)
- **Data Location**: In-memory by default; if a data directory is provided, persisted via WAL for crash recovery
- **Expiration**: Messages auto-expire after **24 hours**
- **Privacy**: Storage nodes cannot read contents (E2EE) or identify parties (cryptographic pseudonyms)

#### Disk Space Calculation Example

```
Available Disk: 100 GB → 1% = 1 GB (cap) → ~1,536,000 messages
Available Disk: 500 MB → 1% = 5 MB       → ~7,680 messages
```

## Security Model

### Threat Model

The async messaging system operates under the following threat model:

**Trusted**:
- Message encryption (NaCl/box)
- Friend authentication
- Local key storage

**Untrusted**:
- Storage nodes (assumed malicious)
- Network infrastructure
- Message metadata

### Security Properties

1. **Confidentiality**: Messages encrypted end-to-end using one-time pre-exchanged keys
2. **Authenticity**: Messages authenticated using sender's private key
3. **Forward Secrecy**: One-time keys prevent compromise of past messages
4. **Anonymity**: Sender/recipient identities are pseudonymous via Tox public keys
5. **Integrity**: Tampering detected through authenticated encryption
6. **Replay Protection**: Used one-time keys cannot be reused

### Forward Secrecy vs Epoch-Based Pseudonym Rotation

toxcore-go implements **two distinct complementary mechanisms**:

#### Forward Secrecy (Cryptographic Protection)

One-time pre-exchanged keys (pre-keys) protect message confidentiality. If a long-term key is compromised, past messages remain secure. Each message uses a unique key deleted after use. Implemented in `async/forward_secrecy.go` and `async/prekey.go`.

#### Epoch-Based Pseudonym Rotation (Metadata Protection)

**6-hour epochs** produce cryptographic pseudonyms derived from a user's public key and the current epoch. Storage nodes see only pseudonyms, not real Tox IDs. Different epochs produce unlinkable pseudonyms. Implemented in `async/epoch.go` and `async/obfs.go`.

**Summary**: Pre-keys protect message *content* from future key compromise; epochs/pseudonyms protect sender/recipient *identities* from storage nodes.

### Forward Secrecy Model

- **Pre-Key Generation**: Each user generates 100 one-time key pairs per peer
- **Key Exchange**: Pre-keys exchanged when both parties are online
- **Message Encryption**: Each async message consumes one unique pre-key
- **Key Exhaustion**: Async messaging disabled until refresh when keys run out
- **Automatic Refresh**: Pre-keys regenerated when peers come online together

### Limitations

- **Pre-Key Requirement**: Requires prior key exchange when both parties are online
- **Limited Messages**: 100 messages per peer until key refresh
- **Availability**: Messages may be lost if all storage nodes become unavailable

## Core Components

### ForwardSecureMessage Structure

```go
type ForwardSecureMessage struct {
    Type          string       // Message type: "forward_secure_message"
    MessageID     [32]byte     // Unique message identifier (random)
    SenderPK      [32]byte     // Sender's Tox public key  
    RecipientPK   [32]byte     // Recipient's Tox public key
    PreKeyID      uint32       // ID of the one-time key used
    EncryptedData []byte       // Message encrypted with one-time key
    Nonce         [24]byte     // Encryption nonce
    MessageType   MessageType  // Normal, action, etc.
    Timestamp     time.Time    // Creation timestamp
    ExpiresAt     time.Time    // Expiration timestamp (24 hours)
}
```

### PreKeyBundle Structure

```go
type PreKeyBundle struct {
    PeerPK           [32]byte     // Peer's public key
    Keys             []PreKey     // Array of one-time keys
    CreatedAt        time.Time    // Bundle creation timestamp
    UsedCount        int          // Number of keys already used
    MaxKeys          int          // Maximum keys (100)
    LastRefreshOffer time.Time    // Last refresh attempt
}

type PreKey struct {
    ID        uint32           // Unique key identifier
    KeyPair   *crypto.KeyPair  // One-time key pair
    Used      bool             // Whether key has been used
    UsedAt    *time.Time       // When key was used (if applicable)
}
```

### PreKeyExchangeMessage Structure

```go
type PreKeyExchangeMessage struct {
    Type       string                   // Message type: "pre_key_exchange"
    SenderPK   [32]byte                 // Sender's Tox public key
    PreKeys    []PreKeyForExchange      // Public keys being shared
    Timestamp  time.Time                // Exchange timestamp
}

type PreKeyForExchange struct {
    ID        uint32     // Key identifier
    PublicKey [32]byte   // Public portion of one-time key
}
```

### Legacy AsyncMessage Structure (for compatibility)

```go
type AsyncMessage struct {
    ID          [16]byte     // Unique message identifier (random)
    RecipientPK [32]byte     // Recipient's Tox public key
    SenderPK    [32]byte     // Sender's Tox public key
    EncryptedData []byte     // NaCl/box encrypted message content
    Timestamp   time.Time    // Storage timestamp
    Nonce       [24]byte     // Encryption nonce
    MessageType MessageType  // Normal (0) or Action (1)
}
```

### Message Types

```go
const (
    MessageTypeNormal MessageType = 0  // Regular text message
    MessageTypeAction MessageType = 1  // Action message ("/me" style)
)
```

### Constants and Limits

```go
const (
    MaxMessageSize           = 1372              // Maximum unencrypted message size
    MaxStorageTime          = 24 * time.Hour     // Message expiration time
    MaxMessagesPerRecipient = 100                // Anti-spam limit per recipient
    EncryptionOverhead      = 16                 // NaCl/box overhead
    
    // Forward secrecy constants
    PreKeysPerPeer          = 100                // One-time keys per peer
    PreKeyRefreshThreshold  = 20                 // Refresh when less than 20 keys remain
    MaxPreKeyAge           = 30 * 24 * time.Hour // Pre-keys expire after 30 days
    
    // Dynamic storage capacity limits
    MinStorageCapacity      = 1536               // Minimum capacity (1MB / ~650 bytes per message)
    MaxStorageCapacity      = 1536000            // Maximum capacity (1GB / ~650 bytes per message)
    StoragePercentage       = 1                  // 1% of available disk space
)
```

**Note**: Storage capacity is dynamic: 1% of available disk space, bounded to 1MB–1GB.

## Message Format

### Forward Secrecy Encryption Format

Forward-secure messages use one-time pre-exchanged keys with NaCl/box:

```
Forward Secure Message = box(plaintext, nonce, prekey_public, sender_private)
```

Where:
- `prekey_public`: One-time public key from pre-exchange
- `sender_private`: Sender's main private key for authentication
- Each message consumes one pre-key, providing forward secrecy

### Legacy Encryption Format (for compatibility)

Legacy messages use the recipient's main public key:

```
Legacy Message = box(plaintext, nonce, recipient_pk, sender_sk)
```

### Plaintext Format

The plaintext before encryption contains:
```
[ Message Type (1 byte) ][ Message Length (2 bytes) ][ Message Data (variable) ]
```

- **Message Type**: `0` for normal, `1` for action
- **Message Length**: Length of message data (big-endian)
- **Message Data**: UTF-8 encoded message content

### Storage Format

Stored messages are serialized as:
```
[ Message ID (16 bytes) ]
[ Recipient PK (32 bytes) ]
[ Sender PK (32 bytes) ]
[ Timestamp (8 bytes, Unix time) ]
[ Nonce (24 bytes) ]
[ Message Type (1 byte) ]
[ Encrypted Data Length (4 bytes) ]
[ Encrypted Data (variable) ]
```

## Storage Protocol

### Message Storage

When a storage node receives a store request:

1. **Validation**:
   - Check message size limits
   - Verify storage capacity
   - Check per-recipient limits

2. **Storage**:
   - Generate unique message ID
   - Store with timestamp
   - Index by recipient public key

3. **Response**:
   - Return message ID on success
   - Return error code on failure

### Message Retrieval

When a client requests messages:

1. **Authentication**: Verify client owns the recipient public key
2. **Lookup**: Find all messages for the recipient
3. **Return**: Send messages with metadata
4. **Cleanup**: Optionally delete retrieved messages

### Message Expiration

Storage nodes automatically:
- Delete messages older than `MaxStorageTime`
- Run cleanup every hour
- Prioritize recent messages when at capacity

## Client Protocol

### Sending Messages

```go
func SendAsyncMessage(recipientPK [32]byte, message []byte, 
    messageType MessageType) error
```

1. **Validation**: Check message size and format
2. **Encryption**: Encrypt message for recipient
3. **Storage Selection**: Choose storage nodes via DHT
4. **Distribution**: Store on multiple nodes for redundancy
5. **Confirmation**: Verify successful storage

### Retrieving Messages

```go
func RetrieveAsyncMessages(recipientPK [32]byte) ([]AsyncMessage, error)
```

1. **Node Discovery**: Find storage nodes for recipient
2. **Query**: Request messages from each node
3. **Decryption**: Decrypt received messages
4. **Deduplication**: Remove duplicate messages
5. **Delivery**: Pass to message handler

### Integration with Tox

The AsyncManager provides automatic integration:

```go
// Send to offline friends automatically
tox.OnFriendStatusChange(func(friendPK [32]byte, online bool) {
    asyncManager.SetFriendOnlineStatus(friendPK, online)
})

// Retrieve messages when coming online
asyncManager.SetMessageHandler(func(senderPK [32]byte, message string, 
    messageType MessageType) {
    // Process received async message
})
```

## Network Discovery

### Storage Node Discovery

Storage nodes are discovered through the existing Tox DHT:

1. **Announcement**: Nodes advertise async storage capability
2. **DHT Integration**: Use DHT for storage node lookup
3. **Capability Check**: Verify node supports async messaging
4. **Health Monitoring**: Track node availability and performance

### Node Selection

For message storage, clients:
1. Hash recipient public key to DHT space
2. Find k-closest storage nodes
3. Prefer nodes with good uptime/reliability
4. Distribute across multiple nodes for redundancy

## Security Considerations

### Privacy Protection

- **Message Content**: Always encrypted end-to-end
- **Metadata Minimization**: Only essential metadata stored
- **Automatic Deletion**: Messages expire automatically
- **No Persistent Logs**: Storage nodes don't log requests

### Abuse Prevention

- **Rate Limiting**: Maximum messages per recipient
- **Storage Limits**: Total capacity per storage node
- **Size Limits**: Maximum message size enforced
- **Expiration**: Automatic cleanup prevents accumulation

### Attack Mitigation

- **Sybil Resistance**: Use DHT for node discovery
- **Storage Flooding**: Capacity and rate limits
- **Message Injection**: Authenticated encryption prevents forgery
- **Availability Attacks**: Multiple storage nodes provide redundancy

## Implementation Guidelines

### Storage Node Implementation

```go
func (ms *MessageStorage) StoreMessage(recipientPK, senderPK [32]byte,
    encryptedMessage []byte, nonce [24]byte, messageType MessageType) ([16]byte, error) {
    // 1. Validate inputs
    // 2. Check capacity limits
    // 3. Check per-recipient limits
    // 4. Generate message ID
    // 5. Store with metadata
    // 6. Return message ID
}
```

### Client Implementation

```go
func (ac *AsyncClient) SendAsyncMessage(recipientPK [32]byte,
    message []byte, messageType MessageType) error {
    // 1. Validate message
    // 2. Encrypt for recipient
    // 3. Find storage nodes
    // 4. Store on multiple nodes
    // 5. Verify storage success
}
```

### Message Retrieval Implementation

```go
func (ac *AsyncClient) retrieveObfuscatedMessagesFromNode(nodeAddr net.Addr,
    recipientPseudonym [32]byte, epochs []uint64) ([]*ObfuscatedAsyncMessage, error) {
    // 1. Create and serialize retrieve request
    // 2. Send PacketAsyncRetrieve to storage node
    // 3. Wait for PacketAsyncRetrieveResponse with 5-second timeout
    // 4. Deserialize and return retrieved messages
}
```

Uses channel-based request/response coordination. Registers handler for `PacketAsyncRetrieveResponse` during initialization. Implements 5-second timeout, returns non-nil empty slice when no messages are available, and handles concurrent operations safely.

### Error Handling

The implementation defines these error types:
- `ErrMessageNotFound`: Message not found in storage
- `ErrStorageFull`: Storage node at capacity
- `ErrInvalidRecipient`: Invalid recipient public key
- Standard Go errors for network and encryption failures

### Performance Considerations

Use batch retrieval, concurrent goroutines for network operations, connection pooling to storage nodes, and local caching of node locations.

## API Reference

### AsyncManager

```go
type AsyncManager struct {
    // Private fields including forward security manager
}

// NewAsyncManager creates a new async message manager with forward secrecy
// All users automatically become storage nodes with capacity based on available disk space
func NewAsyncManager(keyPair *crypto.KeyPair, dataDir string) (*AsyncManager, error)

// Start begins the async messaging service
func (am *AsyncManager) Start()

// Stop shuts down the async messaging service
func (am *AsyncManager) Stop()

// SendAsyncMessage sends a forward-secure message for offline delivery
// Requires pre-exchanged keys with the recipient
func (am *AsyncManager) SendAsyncMessage(recipientPK [32]byte, message string,
    messageType MessageType) error

// SetFriendOnlineStatus updates friend online status and handles pre-key exchange
func (am *AsyncManager) SetFriendOnlineStatus(friendPK [32]byte, online bool)

// SetFriendAddress registers a friend's network address for pre-key exchange
// This must be called before pre-key exchange can occur when friend comes online
func (am *AsyncManager) SetFriendAddress(friendPK [32]byte, addr net.Addr)

// SetMessageHandler sets the callback for received async messages
func (am *AsyncManager) SetMessageHandler(handler func(senderPK [32]byte,
    message string, messageType MessageType))

// ProcessPreKeyExchange processes a received pre-key exchange message
func (am *AsyncManager) ProcessPreKeyExchange(exchange *PreKeyExchangeMessage) error

// CanSendAsyncMessage checks if we can send an async message to a peer (have pre-keys)
func (am *AsyncManager) CanSendAsyncMessage(peerPK [32]byte) bool

// GetPreKeyStats returns information about pre-keys for all peers
func (am *AsyncManager) GetPreKeyStats() map[string]int
```

### ForwardSecurityManager

```go
type ForwardSecurityManager struct {
    // Private fields
}

// NewForwardSecurityManager creates a new forward security manager
func NewForwardSecurityManager(keyPair *crypto.KeyPair, dataDir string) (*ForwardSecurityManager, error)

// SendForwardSecureMessage sends an async message using forward secrecy
func (fsm *ForwardSecurityManager) SendForwardSecureMessage(recipientPK [32]byte, 
    message []byte, messageType MessageType) (*ForwardSecureMessage, error)

// ExchangePreKeys creates a pre-key exchange message for a peer
func (fsm *ForwardSecurityManager) ExchangePreKeys(peerPK [32]byte) (*PreKeyExchangeMessage, error)

// ProcessPreKeyExchange processes received pre-keys from a peer
func (fsm *ForwardSecurityManager) ProcessPreKeyExchange(exchange *PreKeyExchangeMessage) error

// CanSendMessage checks if we can send a forward-secure message to a peer
func (fsm *ForwardSecurityManager) CanSendMessage(peerPK [32]byte) bool

// GetAvailableKeyCount returns the number of available pre-keys for a peer
func (fsm *ForwardSecurityManager) GetAvailableKeyCount(peerPK [32]byte) int
```

### PreKeyStore

```go
type PreKeyStore struct {
    // Private fields
}

// NewPreKeyStore creates a new pre-key storage manager
func NewPreKeyStore(keyPair *crypto.KeyPair, dataDir string) (*PreKeyStore, error)

// GeneratePreKeys creates a new bundle of one-time keys for a peer
func (pks *PreKeyStore) GeneratePreKeys(peerPK [32]byte) (*PreKeyBundle, error)

// GetAvailablePreKey returns an unused pre-key for a peer, if available
func (pks *PreKeyStore) GetAvailablePreKey(peerPK [32]byte) (*PreKey, error)

// NeedsRefresh checks if a peer's pre-key bundle needs refreshing
func (pks *PreKeyStore) NeedsRefresh(peerPK [32]byte) bool

// RefreshPreKeys generates new pre-keys for a peer, replacing old ones
func (pks *PreKeyStore) RefreshPreKeys(peerPK [32]byte) (*PreKeyBundle, error)

// GetRemainingKeyCount returns the number of unused keys for a peer
func (pks *PreKeyStore) GetRemainingKeyCount(peerPK [32]byte) int
```

### AsyncClient

```go
type AsyncClient struct {
    // Private fields
}

// NewAsyncClient creates a new async messaging client
func NewAsyncClient(keyPair *crypto.KeyPair, trans transport.Transport) *AsyncClient

// SendAsyncMessage stores a message for offline delivery
func (ac *AsyncClient) SendAsyncMessage(recipientPK [32]byte, message []byte,
    messageType MessageType) error

// RetrieveAsyncMessages fetches stored messages for the client
func (ac *AsyncClient) RetrieveAsyncMessages() ([]DecryptedMessage, error)

// AddStorageNode adds a known storage node
func (ac *AsyncClient) AddStorageNode(nodePK [32]byte, addr net.Addr)
```

### MessageStorage

```go
type MessageStorage struct {
    // Private fields
}

// NewMessageStorage creates a new message storage instance with dynamic capacity
func NewMessageStorage(keyPair *crypto.KeyPair, dataDir string) *MessageStorage

// StoreMessage stores an encrypted message for later retrieval
func (ms *MessageStorage) StoreMessage(recipientPK, senderPK [32]byte,
    encryptedMessage []byte, nonce [24]byte, messageType MessageType) ([16]byte, error)

// RetrieveMessages gets all stored messages for a recipient
func (ms *MessageStorage) RetrieveMessages(recipientPK [32]byte) ([]AsyncMessage, error)

// DeleteMessage removes a specific message from storage
func (ms *MessageStorage) DeleteMessage(messageID [16]byte, recipientPK [32]byte) error

// CleanupExpiredMessages removes expired messages
func (ms *MessageStorage) CleanupExpiredMessages() int

// GetMaxCapacity returns the current maximum storage capacity
func (ms *MessageStorage) GetMaxCapacity() int

// UpdateCapacity recalculates storage capacity based on current disk space
func (ms *MessageStorage) UpdateCapacity() error

// GetStorageUtilization returns current storage utilization as a percentage
func (ms *MessageStorage) GetStorageUtilization() float64
```

## Examples

### Basic Usage with Forward Secrecy

```go
package main

import (
    "log"
    "github.com/opd-ai/toxcore/async"
    "github.com/opd-ai/toxcore/crypto"
)

func main() {
    // Generate key pair
    keyPair, err := crypto.GenerateKeyPair()
    if err != nil {
        log.Fatal(err)
    }

    // Create transport for async messaging
    transport, err := transport.NewUDPTransport("0.0.0.0:0") // Auto-assign port
    if err != nil {
        log.Fatal(err)
    }

    // Create async manager with forward secrecy (all users are automatic storage nodes)
    manager, err := async.NewAsyncManager(keyPair, transport, "/home/user/.local/share/tox")
    if err != nil {
        log.Fatalf("Failed to create async manager: %v", err)
    }
    
    // Set message handler
    manager.SetMessageHandler(func(senderPK [32]byte, message string, 
        messageType async.MessageType) {
        log.Printf("Received forward-secure async message: %s", message)
    })
    
    // Start service
    manager.Start()
    defer manager.Stop()
    
    // Simulate friend coming online (automatically exchanges pre-keys)
    friendPK := [32]byte{/* friend's public key */}
    manager.SetFriendOnlineStatus(friendPK, true)
    
    // Check if we can send forward-secure messages
    if manager.CanSendAsyncMessage(friendPK) {
        // Send forward-secure message to offline friend
        err = manager.SendAsyncMessage(friendPK, "Hello from the past!", 
            async.MessageTypeNormal)
        if err != nil {
            log.Printf("Failed to send async message: %v", err)
        }
        
        // Check remaining pre-keys
        stats := manager.GetPreKeyStats()
        if remaining, ok := stats[string(friendPK[:])]; ok {
            log.Printf("Remaining pre-keys for friend: %d", remaining)
        }
    } else {
        log.Printf("Cannot send async message - no pre-keys available")
    }
    
    // Check storage capacity
    stats := manager.GetStorageStats()
    if stats != nil {
        log.Printf("Storage: %d/%d messages (%.1f%% utilized)", 
            stats.TotalMessages, stats.StorageCapacity,
            float64(stats.TotalMessages)/float64(stats.StorageCapacity)*100)
    }
}
```

### Forward Secrecy Management

```go
// Exchange pre-keys with a friend (when both are online)
exchange, err := fsm.ExchangePreKeys(friendPK)
// Send exchange to peer; process their response:
// err = fsm.ProcessPreKeyExchange(friendExchange)

// Send forward-secure messages while keys are available
if fsm.CanSendMessage(friendPK) {
    fsMsg, err := fsm.SendForwardSecureMessage(friendPK, message, async.MessageTypeNormal)
    // Send fsMsg to storage nodes...
}

// Monitor key availability
remaining := fsm.GetAvailableKeyCount(friendPK)
if remaining <= async.PreKeyRefreshThreshold {
    log.Printf("Low on pre-keys - refresh when friend comes online")
}
```

### Integration with Tox

```go
// Auto-handle pre-key exchange and async messages for offline friends
tox.OnFriendStatusChange(func(friendPK [32]byte, online bool) {
    asyncManager.SetFriendOnlineStatus(friendPK, online)
})

// Handle forward-secure async messages
asyncManager.SetMessageHandler(func(senderPK [32]byte, message string,
    messageType async.MessageType) {
    log.Printf("Forward-secure async message from %x: %s", senderPK[:8], message)
})

asyncManager.Start()
```

## Conclusion

The Asynchronous Messaging extension enables offline communication in Tox through distributed storage with forward secrecy via one-time pre-keys, automatic key management, and epoch-based identity obfuscation — all while maintaining decentralization and end-to-end encryption.

---

**Document Revision History**:
- v1.0 (2025-09-02): Initial specification based on reference implementation
- v1.1 (2025-01-01): Added forward secrecy implementation with Signal-inspired pre-key exchange
