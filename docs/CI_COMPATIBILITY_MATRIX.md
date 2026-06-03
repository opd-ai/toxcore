# CI/CD Compatibility and Fuzz Testing Matrix

**Date**: 2026-06-03  
**Purpose**: Define expanded CI testing for protocol compatibility and fuzzing

## Overview

This document describes the protocol compatibility test matrix and fuzz testing enhancements for toxcore-go's CI/CD pipeline. These additions ensure that security features (Noise-IK, ratcheting, etc.) remain compatible across different protocol versions and versions of the implementation.

---

## Protocol Compatibility Test Matrix

The test matrix validates that toxcore-go correctly negotiates and communicates with peers using different protocol versions and features.

### Test Scenarios

#### Legacy Protocol (ProtocolLegacy = 0)

**Peer Configuration**: Classic Tox with NaCl-box encryption only

- [ ] **Scenario 1.1**: Legacy-only peer ↔ Legacy-only peer
  - **Test**: Exchange friend requests, send messages, receive delivery receipts
  - **Expected**: All communication succeeds, uses NaCl-box encryption
  - **Verification**: Packet capture shows NaCl-box format, no Noise packets
  - **Duration**: ~30 seconds

- [ ] **Scenario 1.2**: Legacy+Noise peer ↔ Legacy-only peer
  - **Test**: Noise-capable peer connects to legacy-only peer
  - **Expected**: Automatic downgrade to legacy protocol, communication succeeds
  - **Verification**: No Noise packets, uses legacy encryption
  - **Duration**: ~30 seconds

#### Noise Protocol (ProtocolNoiseIK = 1)

**Peer Configuration**: Tox with Noise-IK encryption

- [ ] **Scenario 2.1**: Noise-only peer ↔ Noise-only peer
  - **Test**: Both peers support only Noise-IK
  - **Expected**: Negotiate Noise-IK, exchange messages
  - **Verification**: Handshake uses Noise pattern, forward secrecy active
  - **Duration**: ~30 seconds

- [ ] **Scenario 2.2**: Legacy+Noise peer ↔ Noise-only peer
  - **Test**: Legacy-capable peer connects to Noise-only peer
  - **Expected**: Noise-IK negotiation succeeds
  - **Verification**: Noise handshake completes, messages encrypted with Noise
  - **Duration**: ~30 seconds

- [ ] **Scenario 2.3**: Noise-only peer ↔ Legacy-only peer
  - **Test**: Noise-only peer connects to legacy-only peer
  - **Expected**: Automatic fallback to legacy
  - **Verification**: Uses legacy encryption after downgrade
  - **Duration**: ~30 seconds

#### Noise+Ratchet Protocol (ProtocolNoiseIK with ratcheting)

**Peer Configuration**: Tox with Noise-IK and symmetric ratcheting

- [ ] **Scenario 3.1**: Noise+Ratchet peer ↔ Noise+Ratchet peer
  - **Test**: Both peers support full ratcheting
  - **Expected**: Noise-IK + ratchet negotiation, per-message forward secrecy
  - **Verification**: Ratchet state updates after each message
  - **Duration**: ~30 seconds

- [ ] **Scenario 3.2**: Noise+Ratchet peer ↔ Noise-only peer
  - **Test**: Ratchet-capable peer connects to Noise-only peer
  - **Expected**: Downgrade to Noise-IK without ratchet
  - **Verification**: No per-message ratcheting, session ratcheting still active
  - **Duration**: ~30 seconds

- [ ] **Scenario 3.3**: Noise+Ratchet peer ↔ Legacy-only peer
  - **Test**: Ratchet-capable peer connects to legacy-only peer
  - **Expected**: Full fallback to legacy (no Noise, no ratcheting)
  - **Verification**: Uses NaCl-box encryption only
  - **Duration**: ~30 seconds

#### Version Commitment & Signed Negotiation

**Feature**: Signature validation on protocol version negotiation

- [ ] **Scenario 4.1**: Signed negotiation with trusted peer
  - **Test**: Version commitment validated with Ed25519 signature
  - **Expected**: Signature verification succeeds, negotiation completes
  - **Verification**: No MITM detection triggered
  - **Duration**: ~15 seconds

- [ ] **Scenario 4.2**: Signed negotiation with forged signature
  - **Test**: Attacker replaces version commitment signature
  - **Expected**: Signature verification fails, connection rejected
  - **Verification**: Security error logged, no fallback to unsigned
  - **Duration**: ~15 seconds

- [ ] **Scenario 4.3**: Unsigned negotiation with mandatory-signature peer
  - **Test**: Peer without signature support connects to signature-mandatory peer
  - **Expected**: Negotiation fails or downgrades gracefully (policy-dependent)
  - **Verification**: Explicit error about signature mismatch
  - **Duration**: ~15 seconds

#### Replay Protection

**Feature**: Sequence numbers and nonce validation

- [ ] **Scenario 5.1**: Replay of old message fails
  - **Test**: Attacker replays previously sent message
  - **Expected**: Message rejected (sequence number already seen)
  - **Verification**: No duplicate processing
  - **Duration**: ~15 seconds

- [ ] **Scenario 5.2**: Out-of-order message handling
  - **Test**: Messages arrive out of sequence
  - **Expected**: Skipped key tracking, message still decrypts
  - **Verification**: No message loss due to reordering
  - **Duration**: ~15 seconds

#### Asymmetric Capability Negotiation

**Feature**: Peers with different capability sets negotiate correctly

- [ ] **Scenario 6.1**: Old client (no ratchet) ↔ New client (with ratchet)
  - **Test**: Mixed version deployment compatibility
  - **Expected**: Works with best common protocol (Noise-IK)
  - **Verification**: Old client unaffected, new client uses Noise-IK
  - **Duration**: ~30 seconds

- [ ] **Scenario 6.2**: Future protocol version compatibility
  - **Test**: Graceful handling of unknown protocol versions
  - **Expected**: Falls back to known highest version
  - **Verification**: Communication succeeds
  - **Duration**: ~15 seconds

---

## Fuzz Testing Corpus

### Fuzz Test Targets

#### 1. Packet Parsing (transport layer)

**Function**: `transport.ParsePacket`

**Fuzz Inputs**:
- Random byte sequences (0-4KB)
- Malformed headers
- Invalid packet types
- Truncated packets
- Very large packets (DoS testing)

**Corpus Size Target**: 500+ generated test cases

**Command**:
```bash
go test -fuzz=FuzzParsePacket -fuzztime=5m ./transport
```

**Success Criteria**:
- No panics
- No memory leaks
- Parsing rejects all invalid inputs
- Valid packets parse correctly

#### 2. Noise Handshake Parsing

**Function**: `noise.ProcessHandshakePacket`

**Fuzz Inputs**:
- Random pre-shared keys
- Malformed ephemeral keys
- Invalid DH outputs
- Truncated handshake packets

**Corpus Size Target**: 1000+ generated test cases

**Command**:
```bash
go test -fuzz=FuzzNoiseHandshake -fuzztime=10m ./noise
```

**Success Criteria**:
- No panics during parsing
- Handshake fails gracefully on invalid input
- No key material leaked on error

#### 3. Ratchet State Update

**Function**: `ratchet.UpdateState`

**Fuzz Inputs**:
- Random message numbers
- Random key material
- Out-of-order updates
- Duplicate message numbers

**Corpus Size Target**: 500+ generated test cases

**Command**:
```bash
go test -fuzz=FuzzRatchetUpdate -fuzztime=5m ./ratchet
```

**Success Criteria**:
- No panics
- Proper replay protection
- Correct skipped key tracking

#### 4. Cryptographic Operations

**Function**: `crypto.Decrypt`, `crypto.Verify`

**Fuzz Inputs**:
- Random ciphertexts
- Random signatures
- Truncated messages
- Zero-length inputs

**Corpus Size Target**: 2000+ generated test cases

**Command**:
```bash
go test -fuzz=FuzzCryptoDecrypt -fuzztime=15m ./crypto
```

**Success Criteria**:
- All operations are deterministic
- No panics on invalid input
- Proper error handling
- No timing side-channels revealed

#### 5. Protocol Message Parsing

**Function**: `messaging.ParseMessage`, `group.ParseGroupMessage`

**Fuzz Inputs**:
- Random message types
- Truncated fields
- Invalid message IDs
- Oversized payloads

**Corpus Size Target**: 1000+ generated test cases

**Command**:
```bash
go test -fuzz=FuzzMessageParsing -fuzztime=10m ./messaging
```

**Success Criteria**:
- All inputs parse without panic
- Invalid messages rejected cleanly
- Memory bounds respected

### Corpus Management

**Location**: `.github/fuzz-corpus/`

**Seed Collection**:
1. **Reproducer corpus**: Real handshake/message captures from integration tests
2. **Edge cases**: Manually curated edge-case inputs
3. **Historical bugs**: Inputs that exposed previous vulnerabilities

**Example Seed Entry**:
```
# Seed: valid Noise handshake message
--- FuzzNoiseHandshake/valid_handshake_001 ---
0x00 0x1f 0x19 0x80 [ephemeral_key_32_bytes] [auth_tag_16_bytes]
```

---

## CI/CD Integration

### New Workflow: `.github/workflows/protocol-compat-matrix.yml`

```yaml
name: Protocol Compatibility Matrix

on:
  pull_request:
    types: [opened, synchronize, reopened]
  push:
    branches: [main, develop]
  schedule:
    - cron: '0 2 * * *'  # Run daily at 2 AM UTC

jobs:
  compatibility-matrix:
    runs-on: ubuntu-latest
    timeout-minutes: 45
    strategy:
      matrix:
        scenario:
          - "legacy-legacy"
          - "legacy-noise"
          - "noise-noise"
          - "noise-ratchet"
          - "signed-negotiation"
          - "replay-protection"
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      
      - name: Run compatibility test - ${{ matrix.scenario }}
        run: |
          go test -tags integration,compat_matrix \
            -run "TestCompat${{ matrix.scenario }}" \
            -timeout 60s \
            -v ./tests/compatibility
      
      - name: Report results
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: compat-results-${{ matrix.scenario }}
          path: test-results.json
```

### New Workflow: `.github/workflows/fuzz-testing.yml`

```yaml
name: Continuous Fuzz Testing

on:
  push:
    branches: [main, develop]
  schedule:
    - cron: '0 0 * * 0'  # Weekly (Sunday 00:00 UTC)

jobs:
  fuzz:
    runs-on: ubuntu-latest
    timeout-minutes: 120
    strategy:
      matrix:
        fuzz-target:
          - "FuzzParsePacket"
          - "FuzzNoiseHandshake"
          - "FuzzRatchetUpdate"
          - "FuzzCryptoDecrypt"
          - "FuzzMessageParsing"
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      
      - name: Run fuzzing - ${{ matrix.fuzz-target }}
        run: |
          go test -fuzz=${{ matrix.fuzz-target }} \
            -fuzztime=2h \
            -fuzzminimizationduration=30s \
            -v ./...
      
      - name: Archive crash inputs
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: fuzz-crashes-${{ matrix.fuzz-target }}
          path: |
            testdata/fuzz
          retention-days: 30
      
      - name: Expand corpus
        run: |
          # Merge new interesting inputs into corpus
          find testdata/fuzz -type f -newer .git | \
            xargs -I {} cp {} .github/fuzz-corpus/
```

---

## Test Framework

### Compatibility Test Structure

```go
// tests/compatibility/legacy_test.go

func TestCompatLegacyLegacy(t *testing.T) {
    // Create two legacy-only peers
    peer1 := NewTestPeer(t, WithProtocol("legacy"))
    peer2 := NewTestPeer(t, WithProtocol("legacy"))
    defer peer1.Close()
    defer peer2.Close()
    
    // Exchange friend requests
    err := peer1.AddFriend(peer2.PublicKey())
    require.NoError(t, err)
    
    err = peer2.AcceptFriend(peer1.PublicKey())
    require.NoError(t, err)
    
    // Send message
    msg := "Hello"
    err = peer1.SendMessage(peer2.PublicKey(), msg)
    require.NoError(t, err)
    
    // Verify received
    received, err := peer2.ReceiveMessage(10 * time.Second)
    require.NoError(t, err)
    assert.Equal(t, msg, received)
    
    // Verify encryption (packet capture)
    packets := peer1.LastSentPackets()
    assert.True(t, isNaClEncrypted(packets[0]))
}
```

---

## Success Metrics

### Compatibility Testing

- **Pass Rate**: 100% (all 9+ scenarios pass)
- **Mean Duration**: < 5 minutes total
- **Regression Detection**: Any protocol change fails matrix until explicitly approved

### Fuzz Testing

- **Crash Freedom**: Zero crashes in 2 hours of fuzzing per target
- **Corpus Growth**: Minimum 50% new interesting inputs per week
- **Coverage**: > 90% code coverage in fuzzed functions

---

## Maintenance & Schedule

### Weekly
- [ ] Review new fuzz crashes (if any)
- [ ] Merge interesting corpus seeds
- [ ] Monitor test execution time trends

### Monthly
- [ ] Review protocol compatibility matrix results
- [ ] Update fuzzing duration targets if needed
- [ ] Archive closed crashes, document root causes

### Quarterly
- [ ] Expand compatibility matrix with new scenarios
- [ ] Optimize fuzz seeds for better coverage
- [ ] Review and update CI execution strategies

---

## See Also

- [PROTOCOL_SPEC.md](./PROTOCOL_SPEC.md) - Protocol details
- [SECURITY_PATCH_PLAYBOOK.md](./SECURITY_PATCH_PLAYBOOK.md) - Release procedures
- [Go Fuzzing Documentation](https://go.dev/security/fuzz/)
- [GitHub Actions Workflow Syntax](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions)
