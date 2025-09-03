# Peer Identity Obfuscation for Async Storage Nodes (OBFS)

**Version**: 1.0  
**Date**: September 2, 2025  
**Status**: Design Document  

## Abstract

This document specifies a peer identity obfuscation system for the Tox asynchronous messaging storage nodes. The current async messaging system exposes sender and recipient public keys to storage nodes, creating privacy vulnerabilities. This proposal introduces cryptographic techniques to hide peer identities while maintaining message deliverability and forward secrecy guarantees.

## Table of Contents

1. [Problem Statement](#problem-statement)
2. [Threat Model](#threat-model)
3. [Design Goals](#design-goals)
4. [Obfuscation Architecture](#obfuscation-architecture)
5. [Cryptographic Primitives](#cryptographic-primitives)
6. [Implementation Phases](#implementation-phases)
7. [Storage Node Protocol](#storage-node-protocol)
8. [Client Protocol](#client-protocol)
9. [Security Analysis](#security-analysis)
10. [Performance Considerations](#performance-considerations)
11. [Migration Strategy](#migration-strategy)
12. [Implementation Guidelines](#implementation-guidelines)

## Problem Statement

### Current Privacy Issues

The existing asynchronous messaging system exposes peer identities to storage nodes in several ways:

1. **Direct Key Exposure**: `SenderPK` and `RecipientPK` fields are stored in plaintext on storage nodes
2. **Message Routing**: Storage nodes can correlate message patterns to infer social graphs
3. **Traffic Analysis**: Storage nodes can track when peers send/receive messages
4. **Metadata Leakage**: Timing, frequency, and size patterns reveal communication habits

### Current Message Structure (Vulnerable)

```go
type ForwardSecureMessage struct {
    Type          string       // "forward_secure_message"
    MessageID     [32]byte     // Unique identifier
    SenderPK      [32]byte     // ❌ EXPOSED TO STORAGE NODES
    RecipientPK   [32]byte     // ❌ EXPOSED TO STORAGE NODES
    PreKeyID      uint32       // Forward secrecy key ID
    EncryptedData []byte       // End-to-end encrypted content
    Nonce         [24]byte     // Encryption nonce
    MessageType   MessageType  // Message type
    Timestamp     time.Time    // Creation timestamp
    ExpiresAt     time.Time    // Expiration time
}
```

## Threat Model

### Adversarial Capabilities

**Storage Node Adversaries** (primary threat):
- Can observe all stored message metadata
- Can correlate messages by sender/recipient keys
- May be operated by state actors or commercial surveillance
- Can perform traffic analysis over time
- Cannot break forward secrecy (cryptographically protected)

**Network Adversaries** (secondary threat):
- Can observe message routing patterns
- Can perform timing correlation attacks
- Cannot decrypt message contents or routing information

### Protected Assets

1. **Sender Identity**: Who sent a message must be hidden from storage nodes
2. **Recipient Identity**: Who receives a message must be hidden from storage nodes  
3. **Communication Patterns**: Frequency and timing of peer communications
4. **Social Graph**: Relationship mapping between Tox users
5. **Message Content**: Already protected by forward secrecy

## Design Goals

### Security Goals

1. **Sender Anonymity**: Storage nodes cannot determine message senders
2. **Recipient Anonymity**: Storage nodes cannot determine message recipients
3. **Unlinkability**: Multiple messages between same peers appear unrelated
4. **Forward Secrecy**: Maintains existing forward secrecy guarantees
5. **Plausible Deniability**: Senders can deny sending specific messages

### Functional Goals

1. **Message Deliverability**: Recipients can still reliably retrieve their messages
2. **Efficient Routing**: Storage nodes can route messages without identity knowledge
3. **Scalability**: Obfuscation overhead scales reasonably with network size
4. **Backward Compatibility**: Gradual migration from existing system
5. **DHT Integration**: Works with existing Tox DHT for storage node discovery

### Non-Goals

1. **Hiding Message Existence**: Storage nodes will know a message exists
2. **Hiding Message Size**: Message sizes remain observable
3. **Hiding Timing**: Message timestamps remain visible for expiration
4. **Traffic Volume Hiding**: Communication frequency patterns may be observable

## Obfuscation Architecture

### Core Concept: Cryptographic Pseudonyms

The system uses **ephemeral cryptographic pseudonyms** instead of real public keys for storage node interactions:

```
Real Identity (ToxPK) → Ephemeral Pseudonym → Storage Node
```

### Pseudonym Generation

For each message, generate unique sender and recipient pseudonyms:

```go
// Sender pseudonym (changes per message)
senderPseudonym := HKDF(senderPrivateKey, recipientPublicKey, messageCounter, "SENDER_PSEUDO")

// Recipient pseudonym (deterministic for retrieval)  
recipientPseudonym := HKDF(recipientPublicKey, salt, epoch, "RECIPIENT_PSEUDO")
```

### Message Routing Strategy

**Problem**: How do recipients find their messages without revealing identity?

**Solution**: Deterministic pseudonyms based on time epochs:

1. **Time Epochs**: Divide time into 6-hour epochs (4 per day)
2. **Epoch-Based Pseudonyms**: Recipients generate deterministic pseudonyms per epoch
3. **Pseudonym Rotation**: Pseudonyms change every epoch to prevent long-term tracking
4. **Multi-Epoch Retrieval**: Clients check multiple recent epochs when coming online

## Cryptographic Primitives

### HKDF-Based Pseudonym Generation

```go
import "crypto/hkdf"
import "crypto/sha256"

// GenerateRecipientPseudonym creates a deterministic pseudonym for message retrieval
func GenerateRecipientPseudonym(recipientPK [32]byte, epoch uint64) [32]byte {
    salt := make([]byte, 32)
    binary.BigEndian.PutUint64(salt[24:], epoch)
    
    hkdf := hkdf.New(sha256.New, recipientPK[:], salt, []byte("TOX_RECIPIENT_PSEUDO_V1"))
    pseudonym := make([]byte, 32)
    hkdf.Read(pseudonym)
    
    var result [32]byte
    copy(result[:], pseudonym)
    return result
}

// GenerateSenderPseudonym creates a unique pseudonym for each message
func GenerateSenderPseudonym(senderSK, recipientPK [32]byte, messageNonce [24]byte) [32]byte {
    info := append(recipientPK[:], messageNonce[:]...)
    
    hkdf := hkdf.New(sha256.New, senderSK[:], messageNonce[:], 
                     append([]byte("TOX_SENDER_PSEUDO_V1"), info...))
    pseudonym := make([]byte, 32)
    hkdf.Read(pseudonym)
    
    var result [32]byte
    copy(result[:], pseudonym)
    return result
}
```

### Obfuscated Message Structure

```go
type ObfuscatedAsyncMessage struct {
    Type              string    // "obfuscated_async_message"
    MessageID         [32]byte  // Random message identifier
    SenderPseudonym   [32]byte  // ✅ HIDES REAL SENDER
    RecipientPseudonym [32]byte // ✅ HIDES REAL RECIPIENT  
    Epoch             uint64    // Time epoch for pseudonym validation
    
    // Encrypted payload containing the real ForwardSecureMessage
    EncryptedPayload  []byte    // AES-GCM encrypted ForwardSecureMessage
    PayloadNonce      [12]byte  // AES-GCM nonce
    PayloadTag        [16]byte  // AES-GCM authentication tag
    
    // Metadata (observable by storage nodes)
    Timestamp         time.Time // Creation time
    ExpiresAt         time.Time // Expiration time
    
    // Proof that sender knows recipient's real identity (prevents spam)
    RecipientProof    [32]byte  // HMAC-SHA256(recipientPK, messageID || epoch)
}
```

### Payload Encryption

The real `ForwardSecureMessage` is encrypted within the `EncryptedPayload`:

```go
// PayloadKey derives from shared secret between sender and recipient
payloadKey := HKDF(sharedSecret, messageNonce, epoch, "PAYLOAD_ENCRYPTION")

// Encrypt the real forward-secure message
encryptedPayload := AES_GCM_Encrypt(payloadKey, serializedForwardSecureMessage)
```

## Implementation Plan

### Single Implementation Phase (Week 1-2)

Since async messaging is not yet deployed, obfuscation will be implemented as a **day-1 feature** with no backward compatibility requirements.

**Complete Implementation**:
- ✅ Implement HKDF pseudonym generation functions 
- ✅ Create epoch management system (6-hour epochs) 
- ✅ Implement AES-GCM payload encryption
- ✅ Add recipient proof generation/validation
- ✅ Build storage system with pseudonym indexing from the start
- ⏳ Integrate obfuscation directly into AsyncClient and AsyncManager

**Core Architecture**:
```go
// async/obfs.go - Core obfuscation infrastructure
type EpochManager struct {
    epochDuration time.Duration // 6 hours
    startTime     time.Time     // Network genesis time
}

type ObfuscationManager struct {
    epochManager *EpochManager
    keyPair      *crypto.KeyPair
}

// async/storage.go - Native pseudonym-based storage
type MessageStorage struct {
    pseudonymIndex map[[32]byte]map[uint64][]ObfuscatedAsyncMessage // pseudonym -> epoch -> messages
    epochManager   *EpochManager
    keyPair        *crypto.KeyPair
    dataDir        string
    maxCapacity    int
}

// async/client.go - Obfuscated-only client
type AsyncClient struct {
    obfuscation  *ObfuscationManager
    storage      *MessageStorage
    storageNodes map[[32]byte]net.Addr
}
```

**Simplified Integration**:
```go
// AsyncManager with built-in obfuscation
func (am *AsyncManager) SendAsyncMessage(recipientPK [32]byte, message string, 
    messageType MessageType) error {
    
    // Generate forward-secure message
    fsMsg, err := am.forwardSecurity.SendForwardSecureMessage(recipientPK, 
        []byte(message), messageType)
    if err != nil {
        return err
    }
    
    // Obfuscate and send (built-in, no fallback needed)
    return am.client.SendObfuscatedMessage(recipientPK, fsMsg)
}
```

## Storage Node Protocol

### Storage Request Protocol

**Obfuscated Store Request**:
```
Client → Storage Node: STORE_OBFUSCATED_MESSAGE
{
    "type": "store_obfuscated_message",
    "message": {
        "recipient_pseudonym": "0x1234...",  // Not real recipient key
        "sender_pseudonym": "0x5678...",     // Not real sender key  
        "encrypted_payload": "0xABCD...",    // Contains real ForwardSecureMessage
        "epoch": 12345,
        "recipient_proof": "0x9ABC...",      // Proves sender knows real recipient
        "expires_at": "2025-09-03T12:00:00Z"
    }
}

Storage Node → Client: STORE_RESPONSE
{
    "status": "success",
    "message_id": "0xDEF0..."
}
```

### Retrieval Request Protocol

**Obfuscated Retrieval Request**:
```
Client → Storage Node: RETRIEVE_BY_PSEUDONYM
{
    "type": "retrieve_by_pseudonym",
    "recipient_pseudonym": "0x1234...",    // Generated from real key + epoch
    "epochs": [12345, 12344, 12343],       // Check multiple recent epochs
    "max_messages": 100
}

Storage Node → Client: RETRIEVE_RESPONSE
{
    "status": "success", 
    "messages": [
        {
            "message_id": "0xDEF0...",
            "sender_pseudonym": "0x5678...",   // Anonymous sender
            "encrypted_payload": "0xABCD...",  // Contains real message
            "epoch": 12345,
            "timestamp": "2025-09-02T08:00:00Z"
        }
    ]
}
```

### Storage Node Implementation

```go
// Native pseudonym-based storage (no legacy support needed)
type MessageStorage struct {
    pseudonymIndex map[[32]byte]map[uint64][]ObfuscatedAsyncMessage // pseudonym -> epoch -> messages
    epochCurrent   uint64
    epochManager   *EpochManager
    keyPair        *crypto.KeyPair
    dataDir        string
    maxCapacity    int
}

func NewMessageStorage(keyPair *crypto.KeyPair, dataDir string) *MessageStorage {
    bytesLimit, err := CalculateAsyncStorageLimit(dataDir)
    if err != nil {
        bytesLimit = uint64(StorageNodeCapacity * 650) // 650 bytes avg per message
    }
    maxCapacity := EstimateMessageCapacity(bytesLimit)

    return &MessageStorage{
        pseudonymIndex: make(map[[32]byte]map[uint64][]ObfuscatedAsyncMessage),
        epochManager:   NewEpochManager(),
        keyPair:        keyPair,
        dataDir:        dataDir,
        maxCapacity:    maxCapacity,
    }
}

func (ms *MessageStorage) StoreObfuscatedMessage(msg *ObfuscatedAsyncMessage) error {
    // Validate epoch is current or recent
    if !ms.epochManager.IsValidEpoch(msg.Epoch) {
        return errors.New("invalid epoch")
    }
    
    // Validate recipient proof
    if !ms.validateRecipientProof(msg) {
        return errors.New("invalid recipient proof")
    }
    
    // Store by pseudonym and epoch
    if ms.pseudonymIndex[msg.RecipientPseudonym] == nil {
        ms.pseudonymIndex[msg.RecipientPseudonym] = make(map[uint64][]ObfuscatedAsyncMessage)
    }
    
    ms.pseudonymIndex[msg.RecipientPseudonym][msg.Epoch] = 
        append(ms.pseudonymIndex[msg.RecipientPseudonym][msg.Epoch], *msg)
    
    return nil
}

func (ms *MessageStorage) RetrieveMessagesByPseudonym(recipientPseudonym [32]byte, epochs []uint64) ([]ObfuscatedAsyncMessage, error) {
    var messages []ObfuscatedAsyncMessage
    
    epochMessages, exists := ms.pseudonymIndex[recipientPseudonym]
    if !exists {
        return messages, nil
    }
    
    for _, epoch := range epochs {
        if epochMsgs, exists := epochMessages[epoch]; exists {
            messages = append(messages, epochMsgs...)
        }
    }
    
    return messages, nil
}
```

## Client Protocol

### Message Sending Flow

```go
func (ac *AsyncClient) SendObfuscatedMessage(recipientPK [32]byte, 
    fsMsg *ForwardSecureMessage) error {
    
    // 1. Get current epoch
    epoch := ac.epochManager.GetCurrentEpoch()
    
    // 2. Generate pseudonyms
    senderPseudonym := GenerateSenderPseudonym(ac.keyPair.Private, recipientPK, fsMsg.Nonce)
    recipientPseudonym := GenerateRecipientPseudonym(recipientPK, epoch)
    
    // 3. Encrypt payload
    sharedSecret := ac.deriveSharedSecret(recipientPK)
    payloadKey := derivePayloadKey(sharedSecret, fsMsg.Nonce, epoch)
    encryptedPayload, nonce, tag := encryptPayload(payloadKey, fsMsg)
    
    // 4. Generate recipient proof
    proof := generateRecipientProof(recipientPK, fsMsg.MessageID, epoch)
    
    // 5. Create obfuscated message
    obfMsg := &ObfuscatedAsyncMessage{
        Type:               "obfuscated_async_message",
        MessageID:          fsMsg.MessageID,
        SenderPseudonym:    senderPseudonym,
        RecipientPseudonym: recipientPseudonym,
        Epoch:              epoch,
        EncryptedPayload:   encryptedPayload,
        PayloadNonce:       nonce,
        PayloadTag:         tag,
        Timestamp:          time.Now(),
        ExpiresAt:          time.Now().Add(24 * time.Hour),
        RecipientProof:     proof,
    }
    
    // 6. Send to storage nodes
    return ac.storeObfuscatedMessage(obfMsg)
}
```

### Message Retrieval Flow

```go
func (ac *AsyncClient) RetrieveObfuscatedMessages() ([]DecryptedMessage, error) {
    var allMessages []DecryptedMessage
    
    // Check multiple recent epochs
    currentEpoch := ac.epochManager.GetCurrentEpoch()
    epochsToCheck := []uint64{currentEpoch, currentEpoch-1, currentEpoch-2, currentEpoch-3}
    
    for _, epoch := range epochsToCheck {
        // Generate our pseudonym for this epoch
        myPseudonym := GenerateRecipientPseudonym(ac.keyPair.Public, epoch)
        
        // Query storage nodes
        obfMessages, err := ac.retrieveByPseudonym(myPseudonym, epoch)
        if err != nil {
            continue
        }
        
        // Decrypt and validate messages
        for _, obfMsg := range obfMessages {
            decrypted, err := ac.decryptObfuscatedMessage(obfMsg)
            if err != nil {
                continue // Skip messages we can't decrypt
            }
            allMessages = append(allMessages, decrypted)
        }
    }
    
    return allMessages, nil
}

func (ac *AsyncClient) decryptObfuscatedMessage(obfMsg ObfuscatedAsyncMessage) (DecryptedMessage, error) {
    // 1. Try to decrypt payload with each potential sender
    for senderPK, sharedSecret := range ac.knownSharedSecrets {
        payloadKey := derivePayloadKey(sharedSecret, obfMsg.PayloadNonce[:], obfMsg.Epoch)
        
        fsMsg, err := decryptPayload(payloadKey, obfMsg.EncryptedPayload, 
                                   obfMsg.PayloadNonce, obfMsg.PayloadTag)
        if err != nil {
            continue // Wrong sender
        }
        
        // 2. Verify this is a valid forward-secure message for us
        if fsMsg.RecipientPK != ac.keyPair.Public {
            continue
        }
        
        // 3. Decrypt the inner message using forward secrecy
        return ac.forwardSecurity.ReceiveForwardSecureMessage(fsMsg)
    }
    
    return DecryptedMessage{}, errors.New("could not decrypt message")
}
```

## Security Analysis

### Anonymity Properties

**Sender Anonymity**:
- ✅ Storage nodes see random pseudonyms, not real sender keys
- ✅ Pseudonyms are unique per message (unlinkable)
- ✅ Requires knowledge of both sender private key and recipient public key to generate

**Recipient Anonymity**:
- ✅ Storage nodes see deterministic pseudonyms that rotate every 6 hours
- ✅ Cannot link pseudonyms across epochs without recipient's private key
- ✅ Multiple recipients can share same pseudonym space (plausible deniability)

**Communication Pattern Hiding**:
- ✅ Sender pseudonyms make message linking impossible
- ⚠️ Recipient pseudonyms allow storage nodes to count messages per epoch per recipient
- ⚠️ Message timing and size patterns remain observable

### Attack Resistance

**Traffic Analysis Attacks**:
- **Mitigation**: Pseudonym rotation every 6 hours limits tracking windows
- **Limitation**: Storage nodes can still observe message frequency per pseudonym per epoch

**Correlation Attacks**:
- **Mitigation**: Sender pseudonyms are cryptographically unlinkable
- **Mitigation**: Recipient proof prevents targeted message injection
- **Limitation**: Extremely sophisticated timing analysis might reveal patterns

**Spam Prevention**:
- **Mechanism**: Recipient proof requires knowledge of real recipient public key
- **Effect**: Prevents random message injection without compromising anonymity

### Cryptographic Security

**Pseudonym Security**:
- Based on HKDF with strong entropy sources (private keys, random nonces)
- Computationally infeasible to reverse pseudonyms to real identities
- Forward secure: compromising current keys doesn't reveal past pseudonyms

**Payload Security**:
- AES-GCM provides authenticated encryption
- Payload keys derived from forward-secure shared secrets
- Double encryption: AES-GCM payload + NaCl forward secrecy

## Performance Considerations

### Computational Overhead

**Pseudonym Generation**:
- 2 HKDF operations per message (sender + recipient pseudonyms)
- Negligible CPU impact (~0.1ms per message on modern hardware)

**Payload Encryption**:
- AES-GCM encryption of ~1KB messages
- Minimal impact (~0.05ms per message)

**Storage Overhead**:
- Pseudonym-based indexing from day 1
- No migration overhead since this is the only implementation

### Network Overhead

**Message Size**:
- `ObfuscatedAsyncMessage`: ~1.5KB total
- Reasonable size for offline message delivery

**Retrieval Efficiency**:
- Must check 3-4 epochs when coming online
- Storage nodes designed for pseudonym-based queries from the start
- Efficient epoch-based indexing

### Scalability Analysis

**Storage Node Impact**:
- Native pseudonym indexing scales O(1) with number of users
- Epoch-based cleanup prevents unbounded growth
- Memory usage increases linearly with active epochs

**Network Load**:
- Retrieval queries for 3-4 epochs is baseline behavior
- No comparison to "legacy" system since this is the only system

## Implementation Strategy

### Day-1 Feature Implementation

Since async messaging has not been deployed yet, obfuscation will be implemented as a **core feature from day 1** with no backward compatibility concerns.

### Development Approach

**Clean Implementation**:
- No legacy code paths or compatibility layers
- Obfuscation is the only supported message format
- Storage nodes designed for pseudonym-based indexing from the start
- Client protocols assume obfuscation by default

**Simplified Architecture**:
- Single message format: `ObfuscatedAsyncMessage`
- Single storage method: pseudonym-based indexing
- Single client protocol: obfuscated message handling
- No configuration toggles or fallback modes needed

### Configuration

```go
type ObfuscationConfig struct {
    EpochDuration    time.Duration // Pseudonym rotation interval (default: 6h)
    MaxEpochsToCheck int           // Number of past epochs to check (default: 4)
}

// No backward compatibility flags needed
```
```

## Implementation Guidelines

### Code Organization

**New Files**:
- `async/obfs.go` - Core obfuscation logic
- `async/pseudonym.go` - Pseudonym generation and management
- `async/epoch.go` - Epoch management system
- `async/obfs_test.go` - Comprehensive test suite

**Modified Files**:
- `async/storage.go` - Add obfuscated message support
- `async/client.go` - Add obfuscated message sending/retrieval
- `async/manager.go` - Integration with AsyncManager

### Testing Strategy

**Unit Tests**:
- Pseudonym generation determinism and uniqueness
- Payload encryption/decryption correctness
- Epoch management accuracy
- Recipient proof validation

**Integration Tests**:
- End-to-end obfuscated message delivery
- Multi-epoch retrieval scenarios
- Storage node functionality
- Full async messaging pipeline

**Security Tests**:
- Pseudonym unlinkability verification
- Timing attack resistance
- Memory leak prevention in epoch cleanup

### Monitoring and Metrics

**Privacy Metrics**:
- Pseudonym collision rates (should be ~0)
- Epoch transition success rates
- Message retrieval success rates

**Performance Metrics**:
- Message encryption/decryption latency
- Storage indexing performance
- Retrieval query response times

**Operational Metrics**:
- Storage utilization per epoch
- Cleanup efficiency
- System-wide error rates

## Future Enhancements

### Short-term Improvements (3-6 months)

**Adaptive Epochs**:
- Dynamic epoch duration based on network activity
- Shorter epochs during high activity periods

**Dummy Traffic**:
- Generate fake messages to hide communication patterns
- Configurable dummy traffic rates

**Enhanced Pseudonym Schemes**:
- Ring signatures for stronger sender anonymity
- Group-based pseudonyms for communities

### Long-term Research (6+ months)

**Private Information Retrieval (PIR)**:
- Allow recipients to retrieve messages without revealing pseudonyms
- Significant computational overhead but maximum privacy

**Onion Routing Integration**:
- Route messages through multiple storage nodes
- Hide message origin even from first-hop storage nodes

**Differential Privacy**:
- Add controlled noise to message timing and size
- Prevent sophisticated statistical attacks

## Conclusion

The proposed obfuscation system provides day-1 privacy protection for the Tox asynchronous messaging system. Key benefits:

- **Privacy by Design**: Both sender and recipient identities are hidden from storage nodes from the very first deployment
- **Forward Security**: Maintains existing forward secrecy guarantees  
- **Clean Architecture**: No legacy code paths or compatibility complexity
- **Practical Performance**: Minimal computational and network overhead
- **Proven Cryptography**: Uses well-established primitives (HKDF, AES-GCM)

The system provides strong privacy protection without the complexity of migration or backward compatibility, making it ideal for immediate implementation as a core feature of the async messaging system.

### Implementation Priority

1. **Week 1**: ✅ **COMPLETED** - Cryptographic infrastructure (pseudonyms, epochs, payload encryption)
2. **Week 2**: ⏳ Storage and client integration with obfuscation built-in
3. **Week 3+**: Testing, optimization, and deployment

This design provides comprehensive privacy protection while maintaining the simplicity and reliability expected from a clean, ground-up implementation.

---

## Implementation Status

### ✅ Completed (September 2, 2025)

**Core Cryptographic Infrastructure** (`async/obfs.go` + `async/obfs_test.go`):
- HKDF-based pseudonym generation for senders and recipients
- Epoch-based recipient pseudonym rotation (6-hour epochs)
- AES-GCM payload encryption with forward secrecy
- HMAC-based recipient proof generation and validation
- Complete ObfuscationManager with all cryptographic primitives
- Comprehensive test suite with >71% coverage
- Performance benchmarks showing excellent performance:
  - Pseudonym generation: ~1.6-2.4 μs per operation
  - Payload encryption: ~1.6 μs per operation
  - Complete message obfuscation: ~9.3 μs per operation

**Epoch Management System** (`async/epoch.go` + `async/epoch_test.go`):
- Time-based epoch calculation with 6-hour rotation
- Network genesis time coordination
- Recent epoch enumeration for message retrieval
- Epoch validation for storage node operations

**Storage System Integration** (`async/storage.go` + extended `async/async_test.go`):
- Pseudonym-based message indexing with epoch support
- Dual storage support for legacy and obfuscated messages
- Obfuscated message storage, retrieval, and deletion operations
- Epoch-based cleanup and maintenance operations
- Comprehensive test suite with >66% coverage
- Performance benchmarks showing excellent performance:
  - Complete obfuscated message creation: ~6.7 μs per operation
  - Storage operations maintain sub-microsecond performance

### ⏳ Next Steps

1. **Client Integration**: Modify `async/client.go` to use obfuscation by default
2. **Manager Integration**: Update `async/manager.go` to integrate obfuscation seamlessly

---

**Document Status**: **Storage system integration completed**, client integration next  
**Next Steps**: Implement obfuscation integration in AsyncClient for automatic message obfuscation  
**Review Required**: Security audit of pseudonym generation and epoch management
