# Tox Asynchronous Messaging Extension Specification

**Version**: 1.0  
**Date**: September 2, 2025  
**Status**: Implemented  

## Abstract

This document specifies the Asynchronous Messaging extension for the Tox protocol, providing offline message delivery capabilities while maintaining Tox's core principles of decentralization, privacy, and security. This extension allows users to send messages to offline friends through a distributed network of storage nodes.

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

The standard Tox protocol requires both parties to be online simultaneously for message delivery. This limitation prevents effective communication in scenarios where users are in different time zones or have intermittent connectivity. The Asynchronous Messaging extension addresses this limitation by providing a distributed storage mechanism for offline message delivery.

### Design Goals

- **Decentralized**: No central servers required
- **End-to-End Encrypted**: Messages remain encrypted between sender and recipient
- **Spam Resistant**: Rate limiting and capacity controls prevent abuse
- **Temporary Storage**: Messages automatically expire to protect privacy
- **Backward Compatible**: Works alongside existing Tox messaging
- **Resource Efficient**: Minimal overhead on the Tox network

### Scope

This extension is an **unofficial** addition to the Tox protocol. It provides:
- Offline message storage and retrieval
- Distributed storage node discovery
- Automatic message cleanup and expiration
- Integration with existing Tox friend management

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

1. **AsyncClient**: Handles message sending and retrieval
2. **MessageStorage**: Manages stored messages on storage nodes
3. **AsyncManager**: High-level integration with Tox instances
4. **Storage Nodes**: Volunteer nodes providing temporary message storage

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

1. **Confidentiality**: Messages are encrypted end-to-end using NaCl/box
2. **Authenticity**: Messages are authenticated using sender's private key
3. **Forward Secrecy**: Not provided (messages persist until retrieved/expired)
4. **Anonymity**: Sender and recipient identities are pseudonymous via Tox public keys
5. **Integrity**: Tampering is detected through authenticated encryption

### Limitations

- **Forward Secrecy**: Messages stored on disk may be recovered if keys are compromised
- **Traffic Analysis**: Storage patterns may reveal communication metadata
- **Availability**: Messages may be lost if all storage nodes become unavailable

## Core Components

### AsyncMessage Structure

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
    MaxMessageSize           = 1372        // Maximum unencrypted message size
    MaxStorageTime          = 24 * time.Hour  // Message expiration time
    MaxMessagesPerRecipient = 100          // Anti-spam limit per recipient
    StorageNodeCapacity     = 10000        // Maximum messages per storage node
    EncryptionOverhead      = 16           // NaCl/box overhead
)
```

## Message Format

### Encryption Format

Messages use NaCl/box (Curve25519 + XSalsa20 + Poly1305) for authenticated encryption:

```
Encrypted Message = box(plaintext, nonce, recipient_pk, sender_sk)
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

### Error Handling

The implementation defines these error types:
- `ErrMessageNotFound`: Message not found in storage
- `ErrStorageFull`: Storage node at capacity
- `ErrInvalidRecipient`: Invalid recipient public key
- Standard Go errors for network and encryption failures

### Performance Considerations

- **Batch Operations**: Retrieve multiple messages efficiently
- **Concurrent Processing**: Use goroutines for network operations
- **Connection Pooling**: Reuse connections to storage nodes
- **Local Caching**: Cache storage node locations

## API Reference

### AsyncManager

```go
type AsyncManager struct {
    // Private fields
}

// NewAsyncManager creates a new async message manager
func NewAsyncManager(keyPair *crypto.KeyPair, actAsStorageNode bool) *AsyncManager

// Start begins the async messaging service
func (am *AsyncManager) Start()

// Stop shuts down the async messaging service
func (am *AsyncManager) Stop()

// SendAsyncMessage sends a message for offline delivery
func (am *AsyncManager) SendAsyncMessage(recipientPK [32]byte, message string,
    messageType MessageType) error

// SetFriendOnlineStatus updates friend online status
func (am *AsyncManager) SetFriendOnlineStatus(friendPK [32]byte, online bool)

// SetMessageHandler sets the callback for received async messages
func (am *AsyncManager) SetMessageHandler(handler func(senderPK [32]byte,
    message string, messageType MessageType))
```

### AsyncClient

```go
type AsyncClient struct {
    // Private fields
}

// NewAsyncClient creates a new async messaging client
func NewAsyncClient(keyPair *crypto.KeyPair) *AsyncClient

// SendAsyncMessage stores a message for offline delivery
func (ac *AsyncClient) SendAsyncMessage(recipientPK [32]byte, message []byte,
    messageType MessageType) error

// RetrieveAsyncMessages fetches stored messages for the client
func (ac *AsyncClient) RetrieveAsyncMessages() ([]AsyncMessage, error)

// AddStorageNode adds a known storage node
func (ac *AsyncClient) AddStorageNode(nodePK [32]byte, addr net.Addr)
```

### MessageStorage

```go
type MessageStorage struct {
    // Private fields
}

// NewMessageStorage creates a new message storage instance
func NewMessageStorage(keyPair *crypto.KeyPair) *MessageStorage

// StoreMessage stores an encrypted message for later retrieval
func (ms *MessageStorage) StoreMessage(recipientPK, senderPK [32]byte,
    encryptedMessage []byte, nonce [24]byte, messageType MessageType) ([16]byte, error)

// RetrieveMessages gets all stored messages for a recipient
func (ms *MessageStorage) RetrieveMessages(recipientPK [32]byte) ([]AsyncMessage, error)

// DeleteMessage removes a specific message from storage
func (ms *MessageStorage) DeleteMessage(messageID [16]byte) error

// CleanupExpiredMessages removes expired messages
func (ms *MessageStorage) CleanupExpiredMessages() int
```

## Examples

### Basic Usage

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

    // Create async manager (acting as storage node)
    manager := async.NewAsyncManager(keyPair, true)
    
    // Set message handler
    manager.SetMessageHandler(func(senderPK [32]byte, message string, 
        messageType async.MessageType) {
        log.Printf("Received async message: %s", message)
    })
    
    // Start service
    manager.Start()
    defer manager.Stop()
    
    // Send message to offline friend
    friendPK := [32]byte{/* friend's public key */}
    err = manager.SendAsyncMessage(friendPK, "Hello from the past!", 
        async.MessageTypeNormal)
    if err != nil {
        log.Printf("Failed to send async message: %v", err)
    }
}
```

### Storage Node Setup

```go
// Create dedicated storage node
keyPair, _ := crypto.GenerateKeyPair()
storage := async.NewMessageStorage(keyPair)

// Handle storage requests
func handleStoreRequest(recipientPK, senderPK [32]byte, 
    encryptedMessage []byte, nonce [24]byte, messageType async.MessageType) {
    
    messageID, err := storage.StoreMessage(recipientPK, senderPK, 
        encryptedMessage, nonce, messageType)
    if err != nil {
        log.Printf("Storage failed: %v", err)
        return
    }
    
    log.Printf("Stored message %x", messageID)
}

// Periodic cleanup
go func() {
    for {
        time.Sleep(time.Hour)
        cleaned := storage.CleanupExpiredMessages()
        log.Printf("Cleaned up %d expired messages", cleaned)
    }
}()
```

### Integration with Tox

```go
// Integrate with existing Tox instance
tox := /* your Tox instance */
asyncManager := async.NewAsyncManager(tox.GetKeyPair(), false)

// Auto-send async messages to offline friends
tox.OnFriendStatusChange(func(friendPK [32]byte, online bool) {
    asyncManager.SetFriendOnlineStatus(friendPK, online)
})

// Handle regular messages
tox.OnFriendMessage(func(friendPK [32]byte, message string, messageType int) {
    log.Printf("Real-time message: %s", message)
})

// Handle async messages
asyncManager.SetMessageHandler(func(senderPK [32]byte, message string, 
    messageType async.MessageType) {
    log.Printf("Async message: %s", message)
})

asyncManager.Start()
```

## Future Enhancements

### Potential Improvements

1. **Forward Secrecy**: Implement ratcheting for stored messages
2. **Push Notifications**: Notify clients when messages arrive
3. **Message Priorities**: Different expiration times based on importance
4. **Compression**: Reduce bandwidth usage for large messages
5. **Replication Strategy**: More sophisticated storage distribution

### Compatibility

This extension is designed to be:
- **Forward Compatible**: Future versions can extend the protocol
- **Backward Compatible**: Non-async clients can ignore the extension
- **Interoperable**: Works with any Tox implementation

## Conclusion

The Asynchronous Messaging extension provides a practical solution for offline communication in the Tox ecosystem while maintaining core security and privacy principles. The distributed storage approach ensures no single point of failure while automatic expiration protects user privacy.

This specification provides a complete framework for implementing offline messaging capabilities that integrate seamlessly with existing Tox deployments.

---

**Document Revision History**:
- v1.0 (2025-09-02): Initial specification based on reference implementation
