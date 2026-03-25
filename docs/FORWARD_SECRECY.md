# Forward Secrecy and Privacy in Async Messaging

**Version**: 1.0  
**Date**: March 2026  
**Status**: Reference Documentation  

## Overview

The toxcore-go asynchronous messaging system provides two distinct but complementary security mechanisms:

1. **Forward Secrecy via One-Time Pre-Keys**: Cryptographic protection ensuring past messages cannot be decrypted even if long-term keys are compromised
2. **Epoch-Based Pseudonym Rotation**: Metadata privacy through rotating cryptographic pseudonyms that hide real identities from storage nodes

These mechanisms serve different security goals and should not be confused with each other.

## Forward Secrecy: One-Time Pre-Key Consumption

### Purpose

Forward secrecy protects **message confidentiality** against future key compromise. If an attacker obtains a user's long-term private key, they cannot decrypt messages that were encrypted using already-consumed pre-keys.

### Mechanism

```
┌─────────────┐                    ┌─────────────┐
│   Alice     │                    │    Bob      │
│  (Sender)   │                    │ (Recipient) │
└──────┬──────┘                    └──────┬──────┘
       │                                  │
       │  1. Bob generates 100 pre-keys   │
       │     (ephemeral key pairs)        │
       │                                  │
       │  2. Bob sends pre-key bundle     │
       │◄─────────────────────────────────│
       │     to Alice when both online    │
       │                                  │
       │  3. Alice encrypts message       │
       │     using Bob's pre-key #1       │
       │     (one-time use)               │
       │                                  │
       │  4. Alice sends encrypted msg    │
       │─────────────────────────────────►│
       │                                  │
       │  5. Bob decrypts with pre-key #1 │
       │     and DELETES pre-key #1       │
       │                                  │
```

### Key Properties

| Property | Description |
|----------|-------------|
| **One-Time Use** | Each pre-key is used exactly once, then permanently deleted |
| **Limited Supply** | 100 pre-keys per peer; async messaging unavailable when exhausted |
| **Automatic Refresh** | Pre-keys regenerated when consumed below threshold (async: 10 remaining via PreKeyLowWatermark, check: 20 remaining via PreKeyRefreshThreshold) |
| **Cryptographic FS** | Compromising long-term keys does NOT reveal past messages |

### Implementation

Pre-keys are managed in `async/forward_secrecy.go`:

```go
// Pre-key consumption is automatic and handled internally
fsm.consumePreKey(recipientPK, peerPreKeys) // Removes and returns first pre-key
```

### Limitations

- **Requires Prior Exchange**: Both parties must be online at least once to exchange pre-keys
- **Message Count Limit**: Only 100 messages per peer until refresh (when both online)
- **Refresh Dependency**: If users never coincide online, pre-keys exhaust permanently

## Epoch-Based Pseudonym Rotation: Metadata Privacy

### Purpose

Epoch-based pseudonyms protect **metadata privacy** by hiding the real identities of senders and recipients from storage nodes. This prevents storage nodes from:

- Building social graphs based on communication patterns
- Tracking which users communicate with each other
- Correlating messages over time

### Mechanism

```
┌─────────────────────────────────────────────────────────────────┐
│                    EPOCH 0 (Hours 0-6)                          │
│                                                                 │
│  Bob's Real PK: abc123...                                       │
│  Bob's Pseudonym: HKDF(abc123..., epoch=0) = xyz789...         │
│                                                                 │
│  Storage Node sees: "Message for recipient xyz789..."           │
│  Storage Node CANNOT link xyz789 to Bob's real identity         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ 6 hours pass
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    EPOCH 1 (Hours 6-12)                         │
│                                                                 │
│  Bob's Real PK: abc123...                                       │
│  Bob's Pseudonym: HKDF(abc123..., epoch=1) = def456...         │
│                                                                 │
│  Storage Node sees: "Message for recipient def456..."           │
│  Storage Node CANNOT link def456 to xyz789 or Bob              │
└─────────────────────────────────────────────────────────────────┘
```

### Key Properties

| Property | Description |
|----------|-------------|
| **6-Hour Rotation** | Pseudonyms change every 6 hours (4 per day) |
| **Deterministic Derivation** | Recipients can compute their own pseudonyms for retrieval |
| **Unlinkability** | Different epochs produce unrelated pseudonyms |
| **No Key Compromise Protection** | Does NOT protect message content if keys compromised |

### Implementation

Epochs are managed in `async/epoch.go` and `async/obfs.go`:

```go
// Epoch duration is fixed at 6 hours
const EpochDuration = 6 * time.Hour

// Pseudonym generation uses HKDF with epoch as salt
pseudonym := HKDF(recipientPK, epoch, "TOX_RECIPIENT_PSEUDO_V1")
```

### What Epochs Do NOT Provide

- **Cryptographic Forward Secrecy**: Epoch rotation does not protect past messages from key compromise
- **Message Confidentiality**: Epochs only hide identities, not message content
- **Protection Against Recipients**: The recipient always knows who sent them messages

## Comparison Table

| Feature | Pre-Keys (Forward Secrecy) | Epochs (Metadata Privacy) |
|---------|---------------------------|---------------------------|
| **Protects** | Message content from future key compromise | Sender/recipient identities from storage nodes |
| **Rotation Trigger** | Per-message (consumed after use) | Time-based (every 6 hours) |
| **Requires Prior Setup** | Yes (both parties must be online once) | No (deterministic from public key) |
| **Exhaustible** | Yes (100 keys, then wait for refresh) | No (always computable) |
| **Key Compromise Impact** | Past messages remain safe | Storage node can now link pseudonyms to user |
| **Implementation** | `async/forward_secrecy.go` | `async/epoch.go`, `async/obfs.go` |

## Combined Security Model

When used together, the async messaging system provides:

1. **End-to-End Encryption**: Message content encrypted with pre-keys
2. **Forward Secrecy**: Past messages protected if long-term keys leak
3. **Sender Anonymity**: Storage nodes see random, per-message pseudonyms
4. **Recipient Anonymity**: Storage nodes see rotating, epoch-based pseudonyms
5. **Unlinkability**: Multiple messages appear unrelated to observers

```
┌─────────────────────────────────────────────────────────────────┐
│                    Security Layer Stack                          │
├─────────────────────────────────────────────────────────────────┤
│  [Application Layer] - Plaintext message                        │
│           │                                                     │
│           ▼                                                     │
│  [Pre-Key Encryption] - Forward-secure encryption               │
│           │              (protects confidentiality)             │
│           ▼                                                     │
│  [Identity Obfuscation] - Pseudonymous sender/recipient         │
│           │                (protects metadata)                  │
│           ▼                                                     │
│  [Transport Layer] - Delivered to storage nodes                 │
└─────────────────────────────────────────────────────────────────┘
```

## Correct Terminology

When describing the async messaging security model, use precise language:

✅ **Correct**:
- "Forward secrecy via one-time pre-key consumption"
- "Epoch-based pseudonym rotation for metadata privacy/unlinkability"
- "Pre-keys protect message confidentiality; epochs protect identity metadata"

❌ **Incorrect**:
- "Forward secrecy via epoch-based rotation" (epochs don't provide FS)
- "Epoch-based forward secrecy" (conflates distinct mechanisms)
- "Pre-keys for anonymity" (pre-keys don't hide identities)

## Related Documentation

- [ASYNC.md](ASYNC.md) - Full async messaging specification
- [OBFS.md](OBFS.md) - Peer identity obfuscation details
- `async/forward_secrecy.go` - Pre-key implementation
- `async/epoch.go` - Epoch manager implementation
- `async/obfs.go` - Obfuscation manager implementation
