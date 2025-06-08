# Tox Handshake Migration to Noise-IK Implementation

## Technical Specification for New Handshake Implementation

### Current Tox Handshake Analysis

The current Tox implementation uses a custom cryptographic handshake based on:

1. **NaCl Crypto Box (crypto_box)**: Uses Curve25519 ECDH + XSalsa20 + Poly1305
2. **Static Key Exchange**: Public keys are exchanged directly during friend requests
3. **No Forward Secrecy**: Uses long-term keys for all communications
4. **Simple Packet Format**: `[sender_pk(32)][nonce(24)][encrypted_data]`

**Identified vulnerabilities:**
- **Key Compromise Impersonation (KCI)**: If a long-term private key is compromised, an attacker can impersonate any peer to the key holder
- **No Forward Secrecy**: Past communications can be decrypted if long-term keys are compromised
- **Replay Attacks**: Limited protection against message replay
- **Weak Identity Binding**: No strong binding between network identity and cryptographic identity

### Noise-IK Pattern Overview

The Noise-IK (Interactive, Known-responder) pattern provides:

1. **Forward Secrecy**: Ephemeral keys ensure past sessions remain secure
2. **Mutual Authentication**: Both parties authenticate each other
3. **KCI Resistance**: Responder's static key being compromised doesn't allow impersonation to the responder
4. **0-RTT Capability**: Initiator can send encrypted data in the first message

**Noise-IK Pattern:**
```
<- s
...
-> e, es, s, ss
<- e, ee, se
```

Where:
- `s` = static key
- `e` = ephemeral key  
- `es` = ephemeral-to-static DH
- `ss` = static-to-static DH
- `ee` = ephemeral-to-ephemeral DH
- `se` = static-to-ephemeral DH

### Key Differences Analysis

| Aspect | Current Tox | Noise-IK |
|--------|-------------|----------|
| Key Types | Static only | Static + Ephemeral |
| Forward Secrecy | No | Yes |
| KCI Resistance | Vulnerable | Resistant |
| Handshake Messages | 1 (friend request) | 2 messages |
| Cryptographic Primitives | NaCl crypto_box | Configurable (X25519, ChaCha20-Poly1305) |
| Session Keys | Derived from static keys | Derived from multiple DH operations |
| Identity Authentication | Public key exchange | Cryptographic proof |

## Implementation Roadmap with Milestones

### Phase 1: Foundation Setup (Weeks 1-2)

**Milestone 1.1: Noise Library Integration**
- Add flynn/noise dependency to go.mod
- Create noise configuration wrapper
- Implement basic Noise handshake state management

**Milestone 1.2: Protocol Negotiation Framework**
- Design version negotiation mechanism
- Create protocol capability advertisement
- Implement fallback detection logic

### Phase 2: Core Handshake Implementation (Weeks 3-5)

**Milestone 2.1: Noise-IK Handshake**
- Implement initiator handshake logic
- Implement responder handshake logic
- Create session key derivation

**Milestone 2.2: Key Management Transition**
- Design static key migration strategy
- Implement ephemeral key generation
- Create secure key storage mechanism

### Phase 3: Transport Integration (Weeks 6-8)

**Milestone 3.1: Packet Format Update**
- Design new packet structure
- Implement packet serialization/deserialization
- Update transport layer handlers

**Milestone 3.2: Session Management**
- Implement session lifecycle management
- Create session persistence mechanism
- Design session rekeying protocol

### Phase 4: Backward Compatibility (Weeks 9-11)

**Milestone 4.1: Dual Protocol Support**
- Implement protocol detection
- Create compatibility layer
- Design graceful degradation

**Milestone 4.2: Migration Mechanism**
- Create peer capability discovery
- Implement automatic protocol upgrade
- Design rollback procedures

### Phase 5: Testing and Validation (Weeks 12-14)

**Milestone 5.1: Security Testing**
- Implement security test suite
- Create KCI resistance tests
- Validate forward secrecy properties

**Milestone 5.2: Performance Testing**
- Benchmark handshake performance
- Compare with existing implementation
- Optimize critical paths

## Code Components Requiring Modification

### 1. Crypto Package (`/crypto/`)

**New Files:**
- `noise_handshake.go` - Noise protocol implementation
- `session_keys.go` - Session key management
- `ephemeral_keys.go` - Ephemeral key generation

**Modified Files:**
- `encrypt.go` - Add session-based encryption
- `decrypt.go` - Add session-based decryption
- `keypair.go` - Extended key management

### 2. Transport Package (`/transport/`)

**New Files:**
- `noise_packet.go` - Noise-specific packet types
- `session_manager.go` - Session lifecycle management

**Modified Files:**
- `packet.go` - New packet types for Noise handshake
- `udp.go` - Updated packet handling
- `tcp.go` - Updated packet handling

### 3. Friend Package (`/friend/`)

**New Files:**
- `noise_request.go` - Noise-based friend requests
- `handshake_manager.go` - Handshake state management

**Modified Files:**
- `request.go` - Backward compatibility wrapper
- `friend.go` - Session state integration

### 4. Main Package (`/toxcore.go`)

**Modified Sections:**
- Handshake initialization
- Packet handler registration
- Session management integration
- Protocol negotiation logic

## Protocol Negotiation for Compatibility

### Version Advertisement

Each node advertises supported protocol versions in DHT announcements:

```go
type ProtocolCapabilities struct {
    SupportedVersions []uint8
    PreferredVersion  uint8
    NoiseSupport      bool
    LegacySupport     bool
}
```

### Handshake Negotiation

1. **Discovery Phase**: Query peer capabilities via DHT
2. **Version Selection**: Choose highest mutually supported version
3. **Handshake Initiation**: Use selected protocol version
4. **Fallback Handling**: Graceful degradation to legacy protocol

### Compatibility Matrix

| Initiator | Responder | Result |
|-----------|-----------|---------|
| Noise-IK | Noise-IK | Noise-IK handshake |
| Noise-IK | Legacy | Legacy handshake |
| Legacy | Noise-IK | Legacy handshake |
| Legacy | Legacy | Legacy handshake |

## Key Management Transition

### Static Key Continuity

Existing Ed25519/Curve25519 keys remain valid:
- ToxID derivation unchanged
- Friend relationships preserved
- Identity continuity maintained

### Session Key Architecture

```go
type NoiseSession struct {
    StaticKeys    *crypto.KeyPair
    EphemeralKeys *crypto.KeyPair
    SharedSecrets [][]byte
    SendCipher    cipher.AEAD
    RecvCipher    cipher.AEAD
    Timestamp     time.Time
    RekeyNeeded   bool
}
```

### Key Rotation Strategy

1. **Ephemeral Key Rotation**: Every 24 hours or 1GB data transfer
2. **Session Rekeying**: Periodic handshake renewal
3. **Emergency Rekeying**: On suspected compromise

## Secure Fallback Mechanisms

### Handshake Failure Handling

1. **Timeout Detection**: 30-second handshake timeout
2. **Retry Logic**: Exponential backoff with jitter
3. **Protocol Downgrade**: Automatic fallback to legacy
4. **Error Reporting**: Detailed failure diagnostics

### Security Guarantees During Transition

- No cleartext credential transmission
- Authenticated fallback decisions
- Secure channel establishment before data exchange
- Protection against downgrade attacks

## Testing Framework for Security Validation

### Security Test Categories

1. **KCI Resistance Tests**
2. **Forward Secrecy Validation**
3. **Replay Attack Prevention**
4. **Protocol Downgrade Protection**
5. **Session Management Security**

### Test Implementation Strategy

Create comprehensive test suites covering:
- Cryptographic property verification
- Protocol state machine validation
- Attack simulation and resistance
- Performance and scalability testing

## Migration Strategy for Existing Network Nodes

### Phased Rollout Plan

**Phase 1: Bootstrap Nodes (Month 1)**
- Upgrade high-connectivity bootstrap nodes
- Enable dual protocol support
- Monitor network compatibility

**Phase 2: Core Infrastructure (Month 2)**
- Upgrade relay nodes and bridges
- Enable Noise-IK by default
- Maintain legacy support

**Phase 3: Client Rollout (Months 3-6)**
- Release client updates with Noise-IK
- Gradual protocol preference shift
- Monitor adoption metrics

**Phase 4: Legacy Deprecation (Month 12)**
- Begin legacy protocol deprecation
- Provide migration incentives
- Plan end-of-life timeline

### Network Compatibility Monitoring

Implement monitoring for:
- Protocol version distribution
- Handshake success rates
- Performance metrics
- Security incident tracking

## Performance Comparison

### Expected Performance Characteristics

| Metric | Current Tox | Noise-IK | Impact |
|--------|-------------|----------|--------|
| Handshake RTT | 1 RTT | 1.5 RTT | +50% |
| Handshake CPU | Low | Medium | +200% |
| Memory Usage | Low | Medium | +150% |
| Bandwidth | Low | Medium | +100 bytes |
| Security | Weak | Strong | Significant |

### Optimization Strategies

1. **Handshake Caching**: Cache ephemeral keys temporarily
2. **Batch Processing**: Group multiple handshakes
3. **Hardware Acceleration**: Use crypto acceleration when available
4. **Connection Reuse**: Maximize session lifetime

## Cryptographic Agility Considerations

### Algorithm Selection

Support multiple cryptographic suites:
- **Default**: X25519 + ChaCha20-Poly1305 + SHA256
- **Alternative**: P-256 + AES-GCM + SHA256
- **Future**: Post-quantum algorithms when standardized

### Algorithm Negotiation

Include cipher suite negotiation in protocol version exchange:

```go
type CipherSuite struct {
    DH       string // "X25519", "P256"
    Cipher   string // "ChaCha20-Poly1305", "AES-GCM"
    Hash     string // "SHA256", "SHA512"
}
```

### Migration Path for New Algorithms

Design framework to support:
- Gradual algorithm introduction
- Backwards compatibility preservation  
- Secure algorithm deprecation
- Emergency algorithm replacement

## Implementation Considerations

### Error Handling

Implement comprehensive error handling for:
- Handshake failures and timeouts
- Key generation errors
- Protocol negotiation failures
- Session management errors

### Logging and Monitoring

Add detailed logging for:
- Handshake attempts and outcomes
- Protocol version negotiations
- Security events and anomalies
- Performance metrics

### Configuration Management

Provide configuration options for:
- Protocol preferences
- Handshake timeouts
- Rekeying intervals
- Fallback behavior

This migration plan provides a comprehensive roadmap for transitioning Tox from its current custom handshake to the more secure Noise-IK pattern while maintaining backward compatibility and network stability.
