# Security Audit Report: toxcore-go

## Overview

This document provides security assessment and known vulnerability mitigations for the toxcore-go implementation of the Tox Messenger protocol.

## 1. Cryptographic Security

### 1.1 Noise Protocol Implementation

toxcore-go implements the Noise Protocol Framework using the `flynn/noise` library with the following configuration:

- **Pattern**: IK (Initiator with Knowledge) for friend connections, XX for initial contact
- **Cipher Suite**: ChaCha20-Poly1305 (authenticated encryption)
- **DH Function**: Curve25519
- **Hash Function**: SHA-256

### 1.2 Flynn/Noise Nonce Exhaustion Vulnerability

**Status**: MITIGATED as of v1.3.0

**Description**: The flynn/noise library uses a 64-bit counter for ChaCha20-Poly1305 nonces. While the 64-bit space (2^64 messages) is astronomically large, the Noise Protocol specification recommends re-keying before nonce exhaustion to maintain forward secrecy guarantees.

**Risk Assessment**: THEORETICAL - Exploiting this would require sending over 18 quintillion messages on a single session, which is impractical. However, defense-in-depth principles mandate mitigation.

**Mitigation Implementation** (transport/noise_transport.go):

1. **Message Counter Tracking**: Each `NoiseSession` tracks separate counters for sent and received messages:
   ```go
   type NoiseSession struct {
       // ...
       sendMessageCount uint64 // Encrypted message counter
       recvMessageCount uint64 // Decrypted message counter
       rekeyThreshold   uint64 // Configurable threshold
   }
   ```

2. **Configurable Rekey Threshold**: Default threshold is 2^32 (4 billion messages), providing massive safety margin:
   ```go
   const DefaultRekeyThreshold uint64 = 1 << 32 // 4,294,967,296 messages
   ```

3. **Proactive Warning**: Sessions log warnings at 90% of threshold (RekeyWarningThreshold).

4. **Automatic Enforcement**: `Encrypt()` and `Decrypt()` return `ErrRekeyRequired` when threshold is reached, forcing session re-establishment.

**Usage**:
```go
// Check if session needs rekeying
if session.NeedsRekey() {
    // Initiate new handshake with peer
}

// Check for warning condition
if session.NeedsRekeyWarning() {
    // Schedule proactive re-handshake
}

// Custom threshold (e.g., for high-frequency connections)
session.SetRekeyThreshold(1 << 28) // 268 million messages
```

**Validation**: Long-running sessions automatically enforce re-keying before counter overflow.

## 2. Key Management

### 2.1 Secure Memory Handling

All sensitive key material uses secure memory practices from `crypto/secure_memory.go`:

- Keys are zeroed after use via `crypto.ZeroBytes()`
- Private keys are not logged or serialized unnecessarily
- Ephemeral keys are generated fresh for each handshake

### 2.2 Forward Secrecy

The implementation provides forward secrecy through:

1. **Ephemeral Key Exchange**: Each Noise handshake generates fresh ephemeral keys
2. **Session Independence**: Compromise of one session does not affect others
3. **Pre-key Rotation**: Async messaging uses epoch-based pre-key rotation (`async/forward_secrecy.go`)

## 3. Replay Protection

### 3.1 Handshake Replay Protection

Implemented in `transport/noise_transport.go`:

- **Nonce Tracking**: Used handshake nonces are stored with timestamps
- **Time Bounds**: Handshakes must be within `HandshakeMaxAge` (5 minutes)
- **Future Drift Limit**: `HandshakeMaxFutureDrift` (1 minute) prevents clock manipulation
- **Automatic Cleanup**: Expired nonces removed every `NonceCleanupInterval`

### 3.2 Message Replay Protection

ChaCha20-Poly1305's incrementing counter provides implicit replay protection within sessions.

## 4. Network Security

### 4.1 NAT Traversal

- Relay fallback for symmetric NAT (transport/relay.go, transport/advanced_nat.go)
- TCP relay protocol support for connection-oriented fallback

### 4.2 Privacy Networks

- Tor transport via onramp library
- I2P transport with persistent destination management
- Nym mixnet support for traffic analysis resistance

## 5. Recommendations

### 5.1 For Developers

1. Always check for `ErrRekeyRequired` errors and handle by re-establishing sessions
2. Use `NeedsRekeyWarning()` for proactive session refresh in long-running connections
3. Monitor session message counts for anomalous activity

### 5.2 For Operators

1. Configure appropriate rekey thresholds based on expected message volume
2. Implement monitoring for rekey events
3. Ensure time synchronization for replay protection

## 6. Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.3.0 | 2025 | Added nonce exhaustion mitigation, message counter tracking |
| 1.2.0 | 2025 | Async messaging security improvements |
| 1.0.0 | 2024 | Initial security implementation |

## 7. Contact

Security issues should be reported via GitHub Issues with the "security" label, or directly to the maintainers for critical vulnerabilities.
