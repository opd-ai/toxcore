# Noise-IK Migration Design Document

## Executive Summary

This document outlines the migration plan from Tox's current custom handshake to the Noise Protocol Framework's IK pattern using the flynn/noise library. This migration addresses potential Key Compromise Impersonation (KCI) vulnerabilities in the current implementation.

## Current State Analysis

### Existing Cryptographic Infrastructure

Based on codebase analysis, the current Tox implementation uses:

1. **Ed25519** for signatures (identity verification)
2. **Curve25519** for key exchange (ECDH)
3. **Custom encryption/decryption** functions
4. **Basic keypair management**

### Current Handshake Characteristics

The existing handshake appears to be:
- **Ad hoc design**: Custom cryptographic protocol without formal security analysis
- **KCI vulnerability**: Potential susceptibility to Key Compromise Impersonation attacks
- **Limited forward secrecy**: May not provide proper forward secrecy guarantees
- **No cryptographic agility**: Difficult to upgrade cryptographic primitives

### Transport Layer Integration

Current transport components:
- UDP-based communication (`transport/udp.go`)
- TCP fallback capability (`transport/tcp.go`)
- NAT traversal support (`transport/nat.go`)
- Packet handling (`transport/packet.go`)

## Target: Noise-IK Pattern

### Why Noise-IK?

The **IK (Initiator with Knowledge)** pattern is suitable for Tox because:

1. **Known Recipients**: Initiator knows recipient's static public key (ToxID)
2. **Mutual Authentication**: Both parties authenticate each other
3. **Forward Secrecy**: Provides proper forward secrecy properties
4. **KCI Resistance**: Resistant to Key Compromise Impersonation attacks
5. **Formal Security**: Formally verified security properties

### Noise-IK Message Pattern

```
IK:
  <- s
  ...
  -> e, es, s, ss
  <- e, ee, se
```

Where:
- `s` = static key
- `e` = ephemeral key  
- `es`, `ss`, `ee`, `se` = DH operations

## Implementation Strategy

### Phase 1: Library Integration and Basic Setup

**Deliverables:**
1. Add flynn/noise dependency
2. Create noise handshake wrapper interface
3. Basic IK pattern implementation
4. Unit tests for handshake components

**Timeline:** 2-3 days

### Phase 2: Protocol Integration

**Deliverables:**
1. Integrate noise handshake with transport layer
2. Update packet format for noise messages
3. Modify connection establishment flow
4. Integration tests

**Timeline:** 3-4 days

### Phase 3: Backward Compatibility and Migration

**Deliverables:**
1. Protocol version negotiation
2. Fallback to legacy handshake
3. Gradual migration strategy
4. Network compatibility testing

**Timeline:** 3-4 days

### Phase 4: Performance and Security Validation

**Deliverables:**
1. Performance benchmarks
2. Security property validation
3. Interoperability testing
4. Documentation updates

**Timeline:** 2-3 days

## Technical Design

### Library Choice: flynn/noise

**Rationale:**
- Most mature Noise Protocol implementation for Go
- Active maintenance (updated August 2025)
- Used in production systems
- Comprehensive IK pattern support

**Trade-offs:**
- Below 1000 star threshold (549 stars)
- Additional dependency
- Learning curve for team

### Key Components to Implement

#### 1. Noise Handshake Manager

```go
type NoiseHandshake struct {
    pattern     *noise.HandshakePattern
    suite       noise.CipherSuite
    staticKey   *noise.DHKey
    peerKey     *noise.DHKey
    state       *noise.HandshakeState
}

func NewNoiseHandshake(staticPrivKey []byte, peerPubKey []byte) (*NoiseHandshake, error)
func (nh *NoiseHandshake) InitiateHandshake() ([]byte, error)
func (nh *NoiseHandshake) ProcessMessage(message []byte) ([]byte, bool, error)
func (nh *NoiseHandshake) GetCipherStates() (*noise.CipherState, *noise.CipherState, error)
```

#### 2. Transport Integration

```go
type NoiseTransport struct {
    conn        net.PacketConn
    sendCipher  *noise.CipherState
    recvCipher  *noise.CipherState
    handshake   *NoiseHandshake
}

func (nt *NoiseTransport) PerformHandshake(peerAddr net.Addr) error
func (nt *NoiseTransport) SendEncrypted(data []byte, addr net.Addr) error
func (nt *NoiseTransport) ReceiveDecrypted() ([]byte, net.Addr, error)
```

#### 3. Protocol Negotiation

```go
type ProtocolVersion uint8

const (
    ProtocolLegacy ProtocolVersion = 0
    ProtocolNoiseIK ProtocolVersion = 1
)

func NegotiateProtocol(conn net.PacketConn, addr net.Addr) (ProtocolVersion, error)
```

### Migration Strategy

#### Version Negotiation Flow

1. **Connection Attempt**: Client sends version negotiation packet
2. **Server Response**: Server responds with supported versions
3. **Protocol Selection**: Highest mutually supported version selected
4. **Handshake Execution**: Selected protocol handshake performed

#### Backward Compatibility

- Maintain legacy handshake code during transition period
- Automatic fallback to legacy if peer doesn't support Noise-IK
- Gradual network migration over 6-12 months
- Configuration option to disable legacy support

### Security Properties

#### Achieved by Noise-IK

1. **Mutual Authentication**: Both parties verify identity
2. **Forward Secrecy**: Compromise of long-term keys doesn't affect past sessions
3. **KCI Resistance**: Resistant to Key Compromise Impersonation
4. **Replay Protection**: Nonces prevent replay attacks
5. **Confidentiality**: All application data encrypted

#### Validation Methods

1. **Unit Tests**: Test handshake state transitions
2. **Property Tests**: Verify security properties hold
3. **Interoperability Tests**: Test with reference implementations
4. **Performance Tests**: Ensure no significant performance regression

## Risk Assessment

### High Risk Items

1. **Network Fragmentation**: Different protocol versions on network
2. **Performance Impact**: Additional handshake overhead
3. **Implementation Bugs**: Cryptographic implementation errors

### Mitigation Strategies

1. **Gradual Rollout**: Phased deployment with monitoring
2. **Comprehensive Testing**: Security-focused test suite
3. **Fallback Mechanisms**: Automatic fallback to working protocols
4. **Performance Monitoring**: Continuous performance measurement

## Success Criteria

### Functional Requirements

- [ ] Noise-IK handshake successfully establishes secure channel
- [ ] Backward compatibility with legacy clients maintained
- [ ] No functional regressions in existing features
- [ ] Performance within 20% of current implementation

### Security Requirements

- [ ] Formal verification of KCI resistance
- [ ] Forward secrecy properties validated
- [ ] Replay attack protection verified
- [ ] No cryptographic vulnerabilities introduced

### Quality Requirements

- [ ] >90% test coverage for new components
- [ ] Comprehensive documentation
- [ ] Code review by security-aware developers
- [ ] Performance benchmarks established

## Next Steps

1. **Create Migration Issue**: Track implementation progress
2. **Add flynn/noise Dependency**: Update go.mod
3. **Implement Basic Handshake**: Start with minimal IK implementation
4. **Write Tests**: Comprehensive test suite for handshake
5. **Integrate with Transport**: Connect handshake to existing transport layer

---

**Document Version**: 1.0  
**Author**: GitHub Copilot  
**Date**: September 2, 2025  
**Status**: Design Phase - Ready for Implementation
