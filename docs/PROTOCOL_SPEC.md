# Protocol Specification: toxcore-go (Tox + opd-ai Extensions)

**Version:** 1.4.0-qtox-preview (library); Protocol versions `ProtocolLegacy` = 0, `ProtocolNoiseIK` = 1; opd-ai extension set v0.1
**Last Updated:** 2026-04-04 (derived from `docs/CHANGELOG.md`)
**Status:** Draft / Preview

> This document is authoritative technical documentation for the wire protocol implemented by
> [`opd-ai/toxcore`](https://github.com/opd-ai/toxcore), a pure-Go implementation of the
> [Tox](https://tox.chat/) peer-to-peer encrypted messaging protocol. It is intended for
> developers implementing interoperable clients, debugging the network, or extending the protocol.
>
> Accuracy notes: every message type, constant, and size in this document is cross-referenced to
> source. Where a value is inferred rather than explicitly declared, it is labelled
> *(Inferred from implementation)*. Where a detail is not present in the codebase, it is labelled
> *"Not specified in codebase."*

## Table of Contents

1. [Overview](#1-overview)
   - [1.1 Purpose](#11-purpose)
   - [1.2 Architecture](#12-architecture)
   - [1.3 Design Principles](#13-design-principles)
   - [1.4 Version & Compatibility Matrix](#14-version--compatibility-matrix)
2. [Message Format](#2-message-format)
   - [2.1 Message Types](#21-message-types)
   - [2.2 Field Specifications](#22-field-specifications)
   - [2.3 Serialization](#23-serialization)
3. [Protocol Flow](#3-protocol-flow)
   - [3.1 Connection Lifecycle](#31-connection-lifecycle)
   - [3.2 Version Negotiation & Noise Handshake](#32-version-negotiation--noise-handshake)
   - [3.3 DHT Bootstrap & Peer Discovery](#33-dht-bootstrap--peer-discovery)
   - [3.4 Friend Request & Messaging Flow](#34-friend-request--messaging-flow)
   - [3.5 Asynchronous (Offline) Messaging](#35-asynchronous-offline-messaging)
   - [3.6 State Machines](#36-state-machines)
   - [3.7 Keepalive & Graceful Shutdown](#37-keepalive--graceful-shutdown)
4. [Error Handling](#4-error-handling)
5. [Implementation Requirements](#5-implementation-requirements)
6. [Examples](#6-examples)
7. [Appendices](#7-appendices)

---

## 1. Overview

### 1.1 Purpose

toxcore-go implements the Tox protocol: a serverless, end-to-end encrypted, peer-to-peer
messaging system. It provides DHT-based peer discovery, friend management, 1-to-1 and group
messaging, file transfers, audio/video calling (ToxAV), and store-and-forward asynchronous
messaging with forward secrecy — without any centralized infrastructure and without cgo in the
core library (`doc.go:1-7`, `README.md`).

The protocol is designed for environments where users cannot rely on always-on servers and where
metadata privacy matters. Peers locate each other through a modified Kademlia DHT, establish
encrypted sessions directly (UDP-preferred, TCP-relay fallback), and exchange authenticated,
encrypted application messages. When a peer is offline, messages can be stored at distributed
storage nodes and retrieved later, protected by one-time pre-keys and epoch-based pseudonyms.

This implementation extends the classic c-toxcore wire protocol with an **opd-ai extension set**:
explicit protocol-version negotiation, Noise Protocol Framework (Noise-IK) handshakes, version
commitment (anti-rollback), DHT-discoverable relays, and cover traffic. Extension packet types use
the reserved range `0xF8`–`0xFF` (248–255) so that legacy c-toxcore clients silently ignore them
(`transport/packet.go:120-160`).

### 1.2 Architecture

The library is organized as a facade (`toxcore.Tox`) over independent subsystems
(`doc.go:178-196`):

```
                         +-----------------------------------+
   Application  <----->  |            toxcore.Tox            |   (toxcore*.go)
   (callbacks/API)       |  facade: friends, messaging,      |
                         |  files, groups, self, lifecycle   |
                         +------+----------+---------+--------+
                                |          |         |
            +-------------------+          |         +-------------------+
            v                              v                             v
     +-------------+              +-----------------+            +----------------+
     |     dht     |              |    messaging    |            |     async      |
     | Kademlia +  |              |  realtime msgs, |            | store-&-forward|
     | S/Kademlia  |              |  receipts,retry |            | forward secrecy|
     +------+------+              +--------+--------+            +-------+--------+
            |                              |                             |
            +--------------+---------------+-------------+---------------+
                           v                             v
                  +-----------------------------------------------+
                  |                  transport                    |
                  |  Packet (de)serialization, handler routing,   |
                  |  NegotiatingTransport -> NoiseTransport ->    |
                  |  UDP / TCP / Tor / I2P / Lokinet / Nym        |
                  +-----------------------+-----------------------+
                                          v
                                    +-----------+
                                    |  crypto   |  NaCl box, Curve25519,
                                    |           |  Ed25519, Noise (flynn/noise)
                                    +-----------+
```

Outbound packets pass through layered transports: `NegotiatingTransport` selects a protocol version
per-peer, then dispatches either through `NoiseTransport` (encrypted, `ProtocolNoiseIK`) or the
underlying legacy transport (`ProtocolLegacy`) (`transport/negotiating_transport.go:183-219`). The
base transport is one of UDP, TCP, or a privacy-network overlay.

### 1.3 Design Principles

- **Serverless / P2P first** — DHT-based discovery; no required central server (`dht/`).
- **Secure by default** — `RequireSignedNegotiation = true` and `EnableLegacyFallback = false` by
  default to resist downgrade/MITM attacks
  (`transport/negotiating_transport.go:52-60`).
- **Backward compatible** — extension packet types live in the reserved `0xF8`–`0xFF` range so
  legacy clients ignore them (`transport/packet.go:120-160`).
- **Forward secrecy** — one-time pre-keys for async messages; Noise-IK ephemeral keys and
  time/volume-based rekeying for live sessions (`async/forward_secrecy.go`,
  `transport/noise_transport.go:42-78`).
- **Metadata minimization** — epoch-based pseudonyms, message padding to fixed buckets, and cover
  traffic (`async/epoch.go`, `async/message_padding.go`, `transport/cover_traffic.go`).
- **Anti-rollback** — version commitment binds the agreed version to the handshake hash via HMAC
  (`transport/version_commitment.go`).
- **Pure Go, no cgo** in the core; deterministic testing via injectable `TimeProvider`
  (`doc.go:158-167`).

### 1.4 Version & Compatibility Matrix

| Concept | Value | Source |
|---------|-------|--------|
| Library / API version | `1.4.0-qtox-preview` | `docs/CHANGELOG.md` |
| Go toolchain | `go 1.25.0` (toolchain `go1.25.8`) | `go.mod:3-5` |
| Wire protocol version: Legacy | `ProtocolLegacy = 0` | `transport/version_negotiation.go` |
| Wire protocol version: Noise-IK | `ProtocolNoiseIK = 1` | `transport/version_negotiation.go` |
| Extension packet set | opd-ai v0.1 (packet types 248–255) | `transport/packet.go:120-160` |
| Legacy c-toxcore | Compatible for packet types 1–40; ignores 248–255 | `transport/packet.go:122-124` |

`ProtocolVersion` is a `uint8` (`transport/version_negotiation.go`). Peers advertise their
supported versions and the highest mutually-supported version is selected
(`SelectBestVersion`, `transport/version_negotiation.go`).

---

## 2. Message Format

### 2.1 Message Types

All packets share the base envelope `[packet type (1 byte)][data (variable)]`
(`transport/packet.go:187-190`). `PacketType` is a single byte (`transport/packet.go:26-27`).

Core types are defined with `iota + 1` (so `PacketPingRequest = 1`), with extension types assigned
explicit values 248–255 (`transport/packet.go:31-160`).

| Type ID | Name | Direction | Description |
|---------|------|-----------|-------------|
| 1 | `PacketPingRequest` | C→S / peer→peer | DHT liveness ping request |
| 2 | `PacketPingResponse` | response | DHT ping response |
| 3 | `PacketGetNodes` | request | Request DHT nodes near a target |
| 4 | `PacketSendNodes` | response | Reply with DHT nodes |
| 5 | `PacketFriendRequest` | initiator→peer | Initiate a friend request |
| 6 | `PacketLANDiscovery` | broadcast | Local network peer discovery |
| 7 | `PacketFriendMessage` | peer→peer | Encrypted message to a friend |
| 8 | `PacketFriendMessageAck` | response | Acknowledge a friend message |
| 9 | `PacketFriendNameUpdate` | peer→peer | Notify name change |
| 10 | `PacketFriendStatusMessageUpdate` | peer→peer | Notify status-message change |
| 11 | `PacketOnionSend` | peer→peer | Onion routing send |
| 12 | `PacketOnionReceive` | peer→peer | Onion routing receive |
| 13 | `PacketOnionReply` | peer→peer | Onion routing reply |
| 14 | `PacketOnionAnnounceRequest` | request | Onion announce request |
| 15 | `PacketOnionAnnounceResponse` | response | Onion announce response |
| 16 | `PacketOnionDataRequest` | request | Onion data request |
| 17 | `PacketOnionDataResponse` | response | Onion data response |
| 18 | `PacketFileRequest` | initiator→peer | Begin a file transfer |
| 19 | `PacketFileControl` | peer→peer | Control a transfer (pause/resume/cancel) |
| 20 | `PacketFileData` | sender→peer | File data chunk |
| 21 | `PacketFileDataAck` | response | Acknowledge a file data chunk |
| 22 | `PacketGroupInvite` | peer→peer | Invite to a group chat |
| 23 | `PacketGroupInviteResponse` | response | Respond to a group invite |
| 24 | `PacketGroupBroadcast` | peer→group | Broadcast to all members |
| 25 | `PacketGroupAnnounce` | DHT | Announce group presence |
| 26 | `PacketGroupQuery` | request | Query group info from DHT |
| 27 | `PacketGroupQueryResponse` | response | Reply to group query |
| 28 | `PacketOnet` | reserved | Reserved for overlay extensions |
| 29 | `PacketDHTRequest` | request | Generic DHT request |
| 30 | `PacketAsyncStore` | client→storage | Store an offline message |
| 31 | `PacketAsyncStoreResponse` | response | Confirm async storage |
| 32 | `PacketAsyncRetrieve` | client→storage | Retrieve stored messages |
| 33 | `PacketAsyncRetrieveResponse` | response | Return retrieved messages |
| 34 | `PacketAsyncPreKeyExchange` | peer↔peer | Exchange forward-secrecy pre-keys |
| 35 | `PacketAVCallRequest` | initiator→peer | Initiate audio/video call |
| 36 | `PacketAVCallResponse` | response | Respond to call request |
| 37 | `PacketAVCallControl` | peer→peer | Call control (mute/hold/end) |
| 38 | `PacketAVAudioFrame` | peer→peer | Encoded audio frame |
| 39 | `PacketAVVideoFrame` | peer→peer | Encoded video frame |
| 40 | `PacketAVBitrateControl` | peer→peer | Adjust media bitrate |
| 248 | `PacketCoverTraffic` | peer→peer | Encrypted dummy payload; recipient MUST discard |
| 249 | `PacketVersionNegotiation` | peer↔peer | Negotiate protocol version |
| 250 | `PacketNoiseHandshake` | peer↔peer | Noise-IK handshake message |
| 251 | `PacketNoiseMessage` | peer↔peer | Noise-encrypted payload |
| 252 | `PacketVersionCommitment` | peer↔peer | Anti-rollback version commitment |
| 253 | `PacketRelayAnnounce` | DHT | Announce relay availability |
| 254 | `PacketRelayQuery` | request | Query DHT for relays |
| 255 | `PacketRelayQueryResponse` | response | Reply with relay info |

> Source: `transport/packet.go:31-160`. Type IDs 1–40 are derived from the `iota + 1` sequence;
> 248–255 are explicit literals.

### 2.2 Field Specifications

#### 2.2.1 Packet (base envelope)

**Purpose:** Universal wire envelope for all packet types.

**Definition** (`transport/packet.go:166-169`):

```go
type Packet struct {
    PacketType PacketType
    Data       []byte
}
```

**Fields:**
- `PacketType` (`PacketType`/`byte`): Message type discriminator [required] [valid range: 1–255; 0 is unused].
- `Data` (`[]byte`): Type-specific payload [required; must be non-nil on serialize — see §4].

**Wire format:** `[PacketType (1 byte)][Data (N bytes)]`. Total length = `1 + len(Data)`
(`transport/packet.go:187-190`). Parsing requires `len(data) >= 1` (`transport/packet.go:210`).

#### 2.2.2 NodePacket (DHT encrypted envelope, Type context: 1–4, 29)

**Purpose:** Carries an encrypted DHT payload tagged with the sender's public key and nonce.

**Definition** (`transport/packet.go:237-241`):

```go
type NodePacket struct {
    PublicKey [32]byte
    Nonce     [24]byte
    Payload   []byte
}
```

**Fields:**
- `PublicKey` (`[32]byte`): Sender Curve25519 public key [required] [exactly 32 bytes].
- `Nonce` (`[24]byte`): NaCl box nonce [required] [exactly 24 bytes].
- `Payload` (`[]byte`): NaCl-box-encrypted DHT data [required].

**Wire format:** `[PublicKey (32)][Nonce (24)][Payload (N)]`; minimum length 56 bytes
(`transport/packet.go:251-256`, parse check at `:273`).

#### 2.2.3 ToxID (identity / address)

**Purpose:** Stable user-facing address shared out-of-band to add a contact.

**Definition** (`crypto/toxid.go:12-16`):

```go
type ToxID struct {
    PublicKey [32]byte // KeySize
    Nospam    [4]byte  // ToxIDNospamSize
    Checksum  [2]byte  // ToxIDChecksumSize
}
```

**Fields:**
- `PublicKey` (`[32]byte`): Long-term Curve25519 public key [required].
- `Nospam` (`[4]byte`): Random anti-spam token; changing it invalidates outstanding friend
  requests [required] [default: random via `GenerateNospam`, `crypto/toxid.go:85-92`].
- `Checksum` (`[2]byte`): XOR of the 36 preceding bytes; **typo detection only, not
  integrity** [required, derived] (`crypto/toxid.go:116-146`).

**Sizes:** total 38 bytes (`ToxIDSize`); hex-encoded form is 76 characters (`ToxIDHexLength`)
(`crypto/constants.go:39-49`).

#### 2.2.4 Friend request payload (Type 5)

**Wire format** *(Inferred from implementation, `toxcore_friends.go:531-556`)*:
`[sender_public_key (32 bytes)][message (UTF-8, ≤ 1016 bytes)]`. The handler rejects packets
smaller than the 32-byte key prefix ("friend request packet too small", `toxcore.go:889`).

**Fields:**
- `sender_public_key` (`[32]byte`): Requester's public key [required].
- `message` (`string`): Greeting text [required] [max length 1016 bytes,
  `toxcore_friends.go:532`].

#### 2.2.5 Friend / FriendStatus (application state)

**Definition** (`toxcore.go:970-999`):

```go
type Friend struct {
    PublicKey            [32]byte
    Status               FriendStatus
    ConnectionStatus     ConnectionStatus
    Name                 string
    StatusMessage        string
    LastSeen             time.Time
    UserData             interface{}
    IsTyping             bool
    DisappearingMessages messaging.DisappearingMessageConfig
}

type FriendStatus uint8
const (
    FriendStatusNone   FriendStatus = iota // 0
    FriendStatusAway                        // 1
    FriendStatusBusy                        // 2
    FriendStatusOnline                      // 3
)
```

#### 2.2.6 Version negotiation packets (Type 249)

**Definitions** (`transport/version_negotiation.go`):

```go
type VersionNegotiationPacket struct {
    SupportedVersions []ProtocolVersion
    PreferredVersion  ProtocolVersion
}

type SignedVersionNegotiationPacket struct {
    VersionNegotiationPacket
    SenderPublicKey [32]byte
    Signature       crypto.Signature // Ed25519, 64 bytes
}
```

**Wire format:**
- Unsigned: `[preferred_version (1)][num_versions (1)][versions (num_versions × 1)]`
  (min 2 bytes).
- Signed: `[public_key (32)][signature (64)][preferred_version (1)][num_versions (1)][versions...]`
  (min 98 bytes).

**Fields:**
- `SupportedVersions` (`[]ProtocolVersion`): list of supported versions [required] [≤ 255 entries].
- `PreferredVersion` (`ProtocolVersion`): preferred version [required].
- `SenderPublicKey` (`[32]byte`): Ed25519 public key (signed variant) [required when signed].
- `Signature` (`crypto.Signature`/`[64]byte`): Ed25519 signature over the version data
  [required when signed].

#### 2.2.7 Version commitment (Type 252)

**Definition** (`transport/version_commitment.go:20-28`):

```go
type VersionCommitment struct {
    Version   ProtocolVersion
    Timestamp int64    // Unix seconds
    HMAC      [32]byte // HMAC-SHA256(handshake_hash, version || timestamp)
}
```

**Wire format:** `[version (1)][timestamp (8, big-endian)][hmac (32)]` = **41 bytes**
(`transport/version_commitment.go:81-99`).

**Validation:** version must match the agreed version; timestamp age ≤ 5 min
(`CommitmentMaxAge`) and future drift ≤ 1 min (`CommitmentMaxFutureDrift`); HMAC verified with a
constant-time compare (`transport/version_commitment.go:123-156`).

#### 2.2.8 Versioned handshake (Type 250)

**Definitions** (`transport/versioned_handshake.go:42-63`):

```go
type VersionedHandshakeRequest struct {
    ProtocolVersion   ProtocolVersion
    SupportedVersions []ProtocolVersion
    NoiseMessage      []byte
    LegacyData        []byte
}

type VersionedHandshakeResponse struct {
    AgreedVersion ProtocolVersion
    NoiseMessage  []byte
    LegacyData    []byte
}
```

**Wire format:**
- Request: `[version (1)][num_supported (1)][supported...][noise_len (2, BE)][noise_data][legacy_data]`
  — ≤ 255 supported versions, ≤ 65535-byte Noise message.
- Response: `[agreed_version (1)][noise_len (2, BE)][noise_data][legacy_data]`.

#### 2.2.9 Asynchronous message structures (Types 30–34)

`ForwardSecureMessage` (`async/forward_secrecy.go:38`):

```go
type ForwardSecureMessage struct {
    Type          string    `json:"type"`
    MessageID     [32]byte  `json:"message_id"`
    SenderPK      [32]byte  `json:"sender_pk"`
    RecipientPK   [32]byte  `json:"recipient_pk"`
    PreKeyID      uint32    `json:"pre_key_id"`
    EncryptedData []byte    `json:"encrypted_data"`
    Nonce         [24]byte  `json:"nonce"`
    MessageType   MessageType
    Timestamp     time.Time
    ExpiresAt     time.Time
}
```

`ObfuscatedAsyncMessage` (`async/obfs.go:33`) adds epoch-based pseudonyms and AEAD payload framing:

```go
type ObfuscatedAsyncMessage struct {
    Type               string
    MessageID          [32]byte
    SenderPseudonym    [32]byte
    RecipientPseudonym [32]byte
    Epoch              uint64
    MessageNonce       [24]byte
    SenderEphemeralPK  [32]byte
    EncryptedPayload   []byte
    PayloadNonce       [12]byte
    PayloadTag         [16]byte
    Timestamp          time.Time
    ExpiresAt          time.Time
    RecipientProof     [32]byte
}
```

`AsyncMessage` (storage record, `async/storage.go:66`):

```go
type AsyncMessage struct {
    ID              [16]byte
    RecipientPK     [32]byte
    SenderPK        [32]byte
    EncryptedData   []byte
    Message         []byte
    Timestamp       time.Time
    Nonce           [24]byte
    MessageType     MessageType
    LamportClock    uint64
    SenderClockHint uint64
}
```

### 2.3 Serialization

There are **two serialization domains** in this codebase:

1. **Binary wire framing** (transport layer). Big-endian / fixed-offset byte packing.
   - Base packet: `[type (1)][data]` (`transport/packet.go:187-190`).
   - DHT node packet: `[pubkey (32)][nonce (24)][payload]` (`transport/packet.go:251-256`).
   - TCP stream framing prepends a **4-byte big-endian length prefix** before each packet
     (`transport/tcp.go:289-296,404`).
   - Multi-byte integers in extension packets are **big-endian** (e.g. commitment timestamp,
     handshake `noise_len`) (`transport/version_commitment.go:81-99`,
     `transport/versioned_handshake.go:65-111`).

2. **Structured encoding** (async / persistence layer).
   - Async message structures carry JSON tags and are JSON-encoded
     (`async/forward_secrecy.go:38-48`, `async/obfs.go:33-47`).
   - The async network client uses **gob** encoding for transmission
     (`async/client.go:32-44`, `encodeGob`).
   - Storage durability uses a Write-Ahead Log with CRC32-checksummed entries
     (`async/wal.go:52-62`).

**Byte order:** Big-endian for all explicitly packed multi-byte integers in the transport layer.
**Encryption envelope:** application payloads are encrypted with NaCl box (Curve25519 +
XSalsa20-Poly1305) — see §5 — before being placed in `Packet.Data`; the 24-byte nonce is carried
in the clear (e.g. in `NodePacket.Nonce` or alongside ciphertext) since it need not be secret.

---

## 3. Protocol Flow

### 3.1 Connection Lifecycle

toxcore is connectionless at the application layer over UDP and connection-oriented over TCP. A
"connection" to a friend is a logical state (`ConnectionStatus`) backed by DHT reachability and,
for `ProtocolNoiseIK`, an established Noise session.

```
Initiator (A)                                   Responder (B)
   |                                                  |
   |---- PacketVersionNegotiation (signed) -------->  |   (249) advertise supported versions
   |<--- PacketVersionNegotiation (signed) ---------  |       select highest mutual version
   |                                                  |
   |---- PacketNoiseHandshake (msg 0, IK) --------->  |   (250) Noise-IK initiator message
   |<--- PacketNoiseHandshake (msg 1, IK) ----------  |       responder message; ciphers derived
   |                                                  |
   |---- PacketVersionCommitment ----------------->   |   (252) HMAC(handshake_hash, ver||ts)
   |<--- PacketVersionCommitment ------------------   |       anti-rollback confirmation
   |                                                  |
   |==== PacketNoiseMessage (encrypted app data) ===> |   (251) e.g. wrapped PacketFriendMessage
   |<=== PacketNoiseMessage (encrypted app data) ==== |
```

When `ProtocolLegacy` (0) is selected, the Noise/commitment steps are skipped and packets are sent
through the underlying transport using classic Tox NaCl-box encryption
(`transport/negotiating_transport.go:205-219`).

### 3.2 Version Negotiation & Noise Handshake

**Version negotiation** (`transport/negotiating_transport.go`, `transport/version_negotiation.go`):

1. On first send to an unknown peer, `NegotiatingTransport` checks its per-peer version cache; on a
   miss it begins negotiation (`negotiateWithPeer`).
2. The initiator sends a (by default **signed**) `VersionNegotiationPacket` listing
   `SupportedVersions` and `PreferredVersion`.
3. The peer selects the highest mutually-supported version via `SelectBestVersion` and replies.
4. The result is cached with a TTL: **5 minutes** for `ProtocolNoiseIK` (`PeerVersionTTL`),
   **1 minute** for `ProtocolLegacy` (`PeerVersionLegacyTTL`, encourages re-negotiation/upgrade).
5. Concurrent negotiations to the same peer are de-duplicated via `singleflight`.
6. Default negotiation timeout is **5 seconds** (`DefaultProtocolCapabilities`).

`DefaultProtocolCapabilities` (`transport/negotiating_transport.go:52-60`):
`SupportedVersions = {Legacy, NoiseIK}`, `PreferredVersion = NoiseIK`,
`EnableLegacyFallback = false`, `RequireSignedNegotiation = true`.

**Noise handshake** (`transport/noise_transport.go`):

- Pattern: **Noise-IK** (initiator knows responder static key). Provided by `flynn/noise` via the
  `toxnoise.IKHandshake` wrapper.
- The initiator creates the handshake (`NewIKHandshake(priv, peerPub, Initiator)`), writes message
  0, and stores an incomplete session (role=Initiator).
- The responder creates `NewIKHandshake(priv, nil, Responder)`, processes message 0, writes message
  1, and both sides derive `sendCipher`/`recvCipher` via `GetCipherStates`.
- Subsequent application packets are encrypted with **ChaCha20-Poly1305** cipher states; each
  cipher uses a 64-bit message counter.
- Handshake nonces are tracked to detect **replay**; a handshake is rejected if older than
  `HandshakeMaxAge` (5 min) or more than `HandshakeMaxFutureDrift` (1 min) in the future.

**Rekeying / nonce-exhaustion protection** (`transport/noise_transport.go:42-78`):

| Parameter | Value | Constant |
|-----------|-------|----------|
| Forced rekey threshold (messages) | `^uint64(0) / 2` (½ of uint64 max) | `DefaultRekeyThreshold` |
| Rekey warning threshold | 90% of forced threshold | `RekeyWarningThreshold` |
| Time-based rekey interval | 30 min | `DefaultRekeyInterval` |
| Rekey-on-idle timeout | 10 min | `DefaultRekeyIdleTimeout` |

### 3.3 DHT Bootstrap & Peer Discovery

The DHT is a modified Kademlia with optional S/Kademlia proof-of-work for Sybil resistance
(`dht/skademlia.go`). Node IDs are 32-byte public keys; distance is XOR (`dht/node.go:125-130`).

**Bootstrap** (`toxcore_network.go:341-362`):

1. Validate the supplied bootstrap public key.
2. Resolve the host:port to a `net.Addr`.
3. Add the node to the routing table.
4. Run the bootstrap process with up to **3 retries and exponential backoff**, bounded by
   `Options.BootstrapTimeout` (default 30 s).

```go
err = tox.Bootstrap("node.tox.biribiri.org", 33445,
    "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
```

**Routing & maintenance constants:**

| Parameter | Value | Source |
|-----------|-------|--------|
| Base k-bucket size | 8 | `dht/dynamic_bucket.go:12` |
| Max k-bucket size | 64 | `dht/dynamic_bucket.go:15` |
| Lookup `k` (closest nodes) | 8 | `dht/iterative_lookup.go:18` |
| Parallelism `alpha` | 3 | `dht/iterative_lookup.go:15` |
| Ping interval | 1 min | `dht/maintenance.go:30` |
| Lookup refresh interval | 5 min | `dht/maintenance.go:31` |
| Node unresponsive timeout | 10 min | `dht/maintenance.go:32` |
| Bad-node prune timeout | 1 hour | `dht/maintenance.go:33` |
| Single lookup step timeout | 5 s | `dht/iterative_lookup.go:21` |
| Per-node response timeout | 3 s | `dht/iterative_lookup.go:27` |
| Lookup cache TTL | 30 s | `dht/routing.go:15` |
| PoW difficulty (default) | 16 leading zero bits | `dht/skademlia.go:33` |

### 3.4 Friend Request & Messaging Flow

**Adding a friend** (`toxcore_friends.go:23-66`):

1. Parse the 76-hex-char Tox address; reject if already a friend.
2. Allocate a unique friend ID under `friendsAddMu` (prevents TOCTOU duplicate IDs).
3. Create a `Friend` with `FriendStatusNone` / `ConnectionNone`.
4. Send a `PacketFriendRequest` (type 5) to the closest DHT node toward the target; if the network
   send fails, queue it for retry.
5. Register the contact with the async manager so offline messages can be decrypted later.

**Sending a message** (`toxcore_messaging.go:26-150`):

1. Validate the message is non-empty and ≤ **1372 bytes** (`MaxPlaintextMessage`).
2. Resolve the friend; determine `MessageType` (`MessageTypeNormal` = 0 default,
   `MessageTypeAction` = 1).
3. **If the friend is online** (`ConnectionStatus != ConnectionNone`): route to the realtime
   `messaging.MessageManager` (`sendRealTimeMessage`) which emits a `PacketFriendMessage` (type 7),
   tracks delivery, and awaits `PacketFriendMessageAck` (type 8).
4. **If offline**: route to `async.AsyncManager` (`sendAsyncMessage`) for store-and-forward.

### 3.5 Asynchronous (Offline) Messaging

When a recipient is offline, messages are encrypted to a one-time **pre-key** and stored at
distributed storage nodes for later retrieval (`async/`).

**Pre-key management** (`async/forward_secrecy.go`, `async/prekeys.go`):

| Parameter | Value | Constant |
|-----------|-------|----------|
| Pre-keys generated per peer | 200 | `PreKeysPerPeer` |
| Low watermark (triggers refresh) | 30 | `PreKeyLowWatermark` |
| Minimum required to send | 20 | `PreKeyMinimum` |
| Rate limit (keys / window / peer) | 10 | `PreKeyRateLimit` |
| Proactive refresh interval | 7 days | `PreKeyProactiveRefreshInterval` |

**Pseudonym epochs** (`async/epoch.go`): sender/recipient pseudonyms rotate every
**6 hours** (`EpochDuration`), anchored to a network genesis time of **2025-01-01 00:00:00 UTC**
(`DefaultNetworkGenesisTime`).

**Storage node limits** (`async/storage.go`):

| Parameter | Value |
|-----------|-------|
| Min storage capacity | 1,536 messages (≈1 MB) |
| Max storage capacity | 1,536,000 messages (≈1 GB) |
| Max retention time | 24 hours (`MaxStorageTime`) |
| Max messages per recipient | 100 (`MaxMessagesPerRecipient`) |

**Message padding** (`async/message_padding.go`): plaintext is padded to one of the fixed buckets
256 / 1024 / 4096 / 16384 bytes for traffic-analysis resistance.

Flow: sender → `PacketAsyncStore` (30) → storage node replies `PacketAsyncStoreResponse` (31);
recipient → `PacketAsyncRetrieve` (32) → `PacketAsyncRetrieveResponse` (33). Pre-keys are exchanged
via `PacketAsyncPreKeyExchange` (34).

### 3.6 State Machines

**Message delivery** (`messaging/message.go:48-64`):

```
Pending(0) -> Sending(1) -> Sent(2) -> Delivered(3) -> Read(4)
                  |
                  +-------> Failed(5)   (on send error / retries exhausted)
```

**Friend connection** (`ConnectionStatus`, `toxcore.go:85-96`):

```
ConnectionNone(0) --(DHT reachable, UDP path)--> ConnectionUDP(2)
ConnectionNone(0) --(relay path)-------------->  ConnectionTCP(1)
ConnectionUDP/TCP --(timeout / peer offline)-->  ConnectionNone(0)
```

**Noise session** (`transport/noise_transport.go`): `created (complete=false)` →
`established (complete=true, ciphers ready)` → `idle/expired` (cleaned up after
`SessionIdleTimeout`) or `rekey-required` (counter/time thresholds reached).

### 3.7 Keepalive & Graceful Shutdown

- **DHT keepalive:** nodes are pinged every **1 minute**; unresponsive after **10 minutes**;
  pruned after **1 hour** (`dht/maintenance.go:30-33`).
- **Noise session keepalive/cleanup:** stale sessions are swept every
  `SessionCleanupInterval` = **10 s**; idle sessions expire after `SessionIdleTimeout` = **5 min**;
  used handshake nonces are cleaned every `NonceCleanupInterval` = **10 min**
  (`transport/noise_transport.go:48-54`).
- **Event loop:** the caller drives `tox.Iterate()` every `tox.IterationInterval()`; each iteration
  performs DHT maintenance, friend-connection updates, message processing, and friend-request
  retries (`toxcore_lifecycle.go:18-43`).
- **Graceful shutdown:** `tox.Kill()` stops the instance; `IsRunning()` reflects the atomic running
  flag. Transports run cleanup goroutines tracked by a `WaitGroup` and stopped via dedicated
  channels (`transport/noise_transport.go`). The 1.4.0 changelog notes "improved goroutine
  lifecycle management with graceful shutdown" (`docs/CHANGELOG.md`).

---

## 4. Error Handling

The codebase uses Go idioms: sentinel `error` values (`var Err… = errors.New(...)`), wrapped errors
(`fmt.Errorf("…: %w", err)`), and contextual `errors.New` at validation sites. There is **no single
numeric error-code enum**; the table below enumerates the principal sentinel/validation errors by
subsystem.

### 4.1 Error Catalogue

| Code / Sentinel | Subsystem | Meaning | Recovery action |
|-----------------|-----------|---------|-----------------|
| `"packet data is nil"` | transport | Serialize called with nil `Data` | Caller must supply non-nil payload (`transport/packet.go:184`) |
| `"packet too short"` | transport | Parsed packet < 1 byte | Drop packet (`transport/packet.go:216`) |
| `"node packet too short"` | transport | DHT node packet < 56 bytes | Drop packet (`transport/packet.go:280`) |
| `ErrNoiseSessionNotFound` | transport/noise | No session for peer | Re-initiate handshake |
| `ErrHandshakeReplay` | transport/noise | Reused handshake nonce | Drop; treat as attack |
| `ErrHandshakeTooOld` / `ErrHandshakeFromFuture` | transport/noise | Timestamp outside window | Drop; check clocks |
| `ErrRekeyRequired` | transport/noise | Counter/time threshold reached | Perform rekey/new handshake |
| `ErrNoiseHandshakeFailed` | transport/noise | Handshake could not complete | Retry; fall back if policy allows |
| `ErrVersionMismatch` | transport | No mutual protocol version | Abort or fall back to legacy (if enabled) |
| `ErrHandshakeTimeout` | transport | Versioned handshake timed out | Retry (default 10 s budget) |
| `ErrCommitmentVersionMismatch` / `ErrInvalidCommitmentMAC` | transport | Rollback/forgery detected | Abort session |
| `ErrCommitmentTooOld` / `ErrCommitmentFromFuture` | transport | Commitment timestamp invalid | Abort; check clocks |
| `"already a friend"` | core | Duplicate friend add | No-op for caller (`toxcore_friends.go:33`) |
| `"friend not found"` | core | Unknown friend ID | Validate ID (`toxcore_messaging.go:82`) |
| `"message cannot be empty"` | core | Empty message body | Caller fix input (`toxcore_messaging.go:29`) |
| `"message too long: maximum 1372 bytes"` | core | Exceeds plaintext limit | Split/shorten (`toxcore_messaging.go:35`) |
| `"friend request message too long"` | core | > 1016 bytes | Shorten (`toxcore_friends.go:533`) |
| `"friend is not connected and async messaging is unavailable"` | core | Offline + no async manager | Enable async / retry later |
| `ErrMessageTooLong`, `ErrMessageEmpty`, `ErrNoEncryption`, `ErrMessageNotFound`, `ErrStoreNotConfigured`, `ErrLoadFailed` | messaging | Message validation / store errors | Per-case (`messaging/message.go:19-36`) |
| `ErrMessageTooLarge`, `ErrInvalidPaddedMessage`, `ErrKeyRotationNotConfigured` | async | Size / padding / config errors | Per-case (`async/message_padding.go`, `async/key_rotation_client.go`) |
| `ErrDirectoryTraversal`, `ErrChunkTooLarge`, `ErrFileNameTooLong`, `ErrTransferStalled`, `ErrFileSizeTooLarge` | file | File transfer safety/limits | Reject transfer (`file/transfer.go`) |
| `ErrNoActiveCall`, `ErrFriendOffline`, `ErrFriendNotFound` | toxav | Call state errors | Per-case (`toxav.go:17-24`) |
| `"all %d default bootstrap nodes failed…"` | core | Bootstrap failed | Retry with other nodes (`toxcore_defaults.go:32`) |

### 4.2 Retry & Backoff

- **Friend message delivery** uses `DeliveryRetryConfig` (`toxcore.go:129-154`): `Enabled=true`,
  `MaxRetries=3`, `InitialDelay=5s`, `MaxDelay=5m`, `BackoffFactor=2.0` (exponential backoff).
- **Friend requests** that fail to send are queued and retried on each `Iterate()` cycle
  (`toxcore_friends.go`).
- **Bootstrap** retries up to 3 times with exponential backoff (`toxcore_network.go:341-362`).
- **Failed messages** transition to `MessageStateFailed` once retries are exhausted.

---

## 5. Implementation Requirements

### 5.1 Transport

- **Protocols:** UDP (preferred), TCP (relay/fallback), plus privacy-network overlays Tor
  (`.onion`), I2P (`.b32.i2p`), Lokinet (`.loki`, dial-only), Nym (`.nym`, dial-only)
  (`transport/`, `README.md`).
- **Default UDP port range:** `StartPort = 33445` … `EndPort = 33545` (`toxcore.go` defaults).
- **Default TCP port:** `0` (disabled unless configured).
- **Bootstrap reference:** `node.tox.biribiri.org:33445` example (`doc.go:32`).
- **TLS:** Not used. Confidentiality/authentication is provided at the application layer by NaCl
  box and (for `ProtocolNoiseIK`) the Noise Protocol Framework — TLS is unsupported/unnecessary.

### 5.2 Cryptography

| Primitive | Use | Source |
|-----------|-----|--------|
| Curve25519 (X25519) | ECDH shared secret (32 bytes) | `crypto/shared_secret.go:34` |
| NaCl `box` (Curve25519 + XSalsa20-Poly1305) | Authenticated public-key encryption | `crypto/encrypt.go:71`, `crypto/decrypt.go:24` |
| NaCl `secretbox` (XSalsa20-Poly1305) | Symmetric authenticated encryption | `crypto/encrypt.go:116` |
| Ed25519 | Signatures (e.g. signed version negotiation) | `crypto/ed25519.go:33` |
| Noise-IK (`flynn/noise`, ChaCha20-Poly1305) | Forward-secret session encryption | `transport/noise_transport.go` |
| HMAC-SHA256 | Version commitment MAC | `transport/version_commitment.go` |

**Key/size constants** (`crypto/constants.go`): public/private key 32 B, nonce 24 B, shared secret
32 B, box overhead 16 B (Poly1305 tag), Ed25519 public 32 B / private 64 B / signature 64 B, ToxID
38 B (32 + 4 + 2).

### 5.3 Timeouts

| Timeout | Value | Source |
|---------|-------|--------|
| Bootstrap (overall) | 30 s (default) | `Options.BootstrapTimeout` |
| Version negotiation | 5 s | `DefaultProtocolCapabilities` |
| Versioned handshake | 10 s | `transport/versioned_handshake.go:302` |
| Noise handshake | 30 s | `transport/noise_transport.go:50` |
| Noise session idle | 5 min | `transport/noise_transport.go:52` |
| UDP read deadline | 100 ms (loop tick) | `transport/udp.go:260` |
| TCP write deadline | 5 s | `transport/tcp.go:289` |
| DHT lookup step / response | 5 s / 3 s | `dht/iterative_lookup.go:21,27` |
| Message retry initial / max delay | 5 s / 5 min | `DeliveryRetryConfig` |

### 5.4 Limits & Buffer Sizes

| Limit | Value | Source |
|-------|-------|--------|
| Max plaintext message | 1372 B | `limits/constants.go:13` |
| Max encrypted message | 1388 B (1372 + 16) | `limits/constants.go:17` |
| Max storage (padded) message | 16384 B | `limits/constants.go:21` |
| Max processing buffer | 1 MB | `limits/constants.go:25` |
| Friend-request message | 1016 B | `toxcore_friends.go:532` |
| UDP read buffer | 2048 B | `transport/udp.go:204` |
| TCP length prefix | 4 B (uint32, BE) | `transport/tcp.go:404` |
| In-memory handshake nonce map | 100,000 entries | `transport/noise_transport.go:59` |
| WAL file size / checkpoint | 64 MB / 5 min / 1000 entries | `async/wal.go:82-84` |

### 5.5 Concurrency Model

- The `Tox` facade is safe for concurrent use; public methods use mutexes and callbacks are invoked
  with proper locking (`doc.go:169-176`).
- **UDP:** a single `processPackets` goroutine reads with a 100 ms deadline and dispatches each
  handler asynchronously in its own goroutine (`transport/udp.go:38-43,198-217`).
- **TCP:** one goroutine per accepted connection (`handleConnection`), with a client connection
  pool (`transport/tcp.go`).
- **Optional worker pool** (`transport/worker_pool.go`): default 100 workers, queue size 10,000,
  min 10 workers / 100 queue, drop-on-full enabled.
- Noise/transport maintenance runs background cleanup goroutines tracked by a `WaitGroup` and
  stopped via channels.

---

## 6. Examples

### 6.1 Establishing a Noise-IK session and sending a message

**Description:** Two peers that both support `ProtocolNoiseIK` negotiate, handshake, commit the
version, then exchange an encrypted friend message.

**Message sequence:**

```
1. A → B: PacketVersionNegotiation (249), signed
   VersionNegotiationPacket{
     SupportedVersions: [0, 1],   // Legacy, NoiseIK
     PreferredVersion:  1,        // NoiseIK
   } + SenderPublicKey(32) + Ed25519 Signature(64)

2. B → A: PacketVersionNegotiation (249), signed
   { SupportedVersions: [0,1], PreferredVersion: 1 }
   // SelectBestVersion -> 1 (NoiseIK)

3. A → B: PacketNoiseHandshake (250)   // Noise-IK message 0
4. B → A: PacketNoiseHandshake (250)   // Noise-IK message 1 -> ciphers derived

5. A → B: PacketVersionCommitment (252)
   [version=1][timestamp=<unix>][HMAC-SHA256(handshake_hash, 0x01 || ts)]   // 41 bytes
6. B → A: PacketVersionCommitment (252)  // verified, anti-rollback confirmed

7. A → B: PacketNoiseMessage (251)
   // ChaCha20-Poly1305 ciphertext wrapping a PacketFriendMessage (7) "Hello!"
8. B → A: PacketNoiseMessage (251)
   // wraps PacketFriendMessageAck (8)
```

### 6.2 Bootstrapping into the DHT

**Description:** A fresh client joins the network and finds peers.

**Message sequence:**

```
1. Client.Bootstrap("node.tox.biribiri.org", 33445, "<32-byte pubkey hex>")
   Client → Bootstrap node: PacketGetNodes (3)
   NodePacket{ PublicKey: <client pk, 32>, Nonce: <24>, Payload: <encrypted target id> }

2. Bootstrap node → Client: PacketSendNodes (4)
   // up to k=8 closest known nodes to the requested target

3. Client → each returned node: PacketPingRequest (1)
4. Node → Client: PacketPingResponse (2)
   // responsive nodes inserted into k-buckets (base size 8, max 64)

// Iterative lookup continues with alpha=3 parallel queries until convergence.
```

### 6.3 Offline (asynchronous) message delivery

**Description:** Alice messages Bob while Bob is offline; Bob retrieves it later.

**Message sequence:**

```
1. Alice → Bob (earlier, both online): PacketAsyncPreKeyExchange (34)
   PreKeyExchangeMessage{ SenderPK, PreKeys: [...200...], SignedPreKey, Timestamp }

--- Bob goes offline ---

2. Alice encrypts to one of Bob's one-time pre-keys and pads to a fixed bucket (e.g. 1024 B):
   Alice → Storage node: PacketAsyncStore (30)
   ObfuscatedAsyncMessage{
     SenderPseudonym, RecipientPseudonym,   // epoch-based (6h rotation)
     Epoch, SenderEphemeralPK,
     EncryptedPayload, PayloadNonce(12), PayloadTag(16),
     RecipientProof(32), ExpiresAt           // <= 24h retention
   }
3. Storage node → Alice: PacketAsyncStoreResponse (31)  // stored (<=100 msgs/recipient)

--- Bob comes online ---

4. Bob → Storage node: PacketAsyncRetrieve (32)   // queried by recipient pseudonym for current epoch
5. Storage node → Bob: PacketAsyncRetrieveResponse (33)   // returns stored ObfuscatedAsyncMessage(s)
   // Bob decrypts with the matching one-time pre-key; the pre-key is then consumed (forward secrecy)
```

### 6.4 Edge cases

- **Nil packet data:** `Packet.Serialize()` returns `"packet data is nil"` rather than emitting a
  malformed frame; senders must always provide a non-nil `Data` (`transport/packet.go:184`).
- **Undersized DHT packet:** any `NodePacket` shorter than 56 bytes is rejected as
  `"node packet too short"` (`transport/packet.go:280`).
- **Downgrade attempt:** if a peer's version commitment does not match the agreed version, or its
  HMAC fails, the session is aborted (`ErrCommitmentVersionMismatch` / `ErrInvalidCommitmentMAC`).
- **Pre-key exhaustion:** if Bob has fewer than `PreKeyMinimum` (20) pre-keys, Alice cannot send a
  forward-secret async message and receives a "no pre-keys available" condition; a refresh is
  triggered at the low watermark (30).
- **Legacy peer:** a peer that supports only `ProtocolLegacy` causes `SelectBestVersion` to choose
  0; with `EnableLegacyFallback = false` (default) the session is restricted accordingly, and the
  legacy entry is cached with a short 1-minute TTL to retry the upgrade.

---

## 7. Appendices

### 7.1 Glossary

| Term | Definition |
|------|------------|
| **Tox** | Serverless, end-to-end encrypted P2P messaging protocol. |
| **ToxID** | 38-byte address: 32-byte public key + 4-byte nospam + 2-byte checksum. |
| **Nospam** | Random 4-byte anti-spam token embedded in a ToxID. |
| **DHT** | Distributed hash table (modified Kademlia) for serverless peer discovery. |
| **k-bucket** | Kademlia routing-table bucket of known nodes (base 8, max 64). |
| **S/Kademlia** | Sybil-resistant Kademlia variant using proof-of-work on node IDs. |
| **Noise-IK** | Noise Protocol Framework handshake pattern with responder static key known to initiator. |
| **Pre-key** | One-time key consumed per async message to provide forward secrecy. |
| **Epoch** | 6-hour window controlling pseudonym rotation for async messages. |
| **Rekey** | Replacing session keys after a message-count or time threshold to limit exposure. |
| **Cover traffic** | Encrypted dummy packets (type 248) that recipients silently discard. |
| **Version commitment** | HMAC binding the agreed protocol version to the handshake hash (anti-rollback). |

### 7.2 References

- Tox project: <https://tox.chat/>
- Noise Protocol Framework: <https://noiseprotocol.org/> (via `flynn/noise`)
- NaCl / `golang.org/x/crypto`: `nacl/box`, `nacl/secretbox`, `curve25519`, `ed25519`
- Kademlia: Maymounkov & Mazières, "Kademlia: A Peer-to-peer Information System Based on the XOR
  Metric"
- S/Kademlia: Baumgart & Mies, "S/Kademlia: A Practicable Approach Towards Secure Key-Based Routing"
- HMAC: RFC 2104; SHA-256: FIPS 180-4; Curve25519: RFC 7748; Ed25519: RFC 8032
- Repository documentation: `docs/DHT.md`, `docs/ASYNC.md`, `docs/FORWARD_SECRECY.md`,
  `docs/MULTINETWORK.md`, `docs/OBFS.md`, `docs/COVER_TRAFFIC.md`, `docs/SECURITY_AUDIT_REPORT.md`

### 7.3 Security Considerations

- **Downgrade/MITM resistance:** signed version negotiation and version commitment are enabled by
  default; `EnableLegacyFallback` defaults to `false` (`transport/negotiating_transport.go:52-60`).
  Enabling legacy fallback re-introduces MITM/downgrade exposure (see README security note).
- **Replay protection:** handshake nonces are tracked (bounded to 100,000 entries) with time-window
  checks (`HandshakeMaxAge` 5 min, `HandshakeMaxFutureDrift` 1 min).
- **Forward secrecy:** Noise-IK ephemeral keys plus volume/time-based rekeying for live sessions;
  consumable one-time pre-keys for async messages.
- **Metadata protection:** epoch pseudonyms, fixed-size padding buckets, and cover traffic reduce
  traffic-analysis surface.
- **ToxID checksum is not integrity protection:** it is a 16-bit XOR for typo detection only
  (`crypto/toxid.go:116-146`). Authenticate identities cryptographically, not via the checksum.
- **Sensitive-buffer hygiene:** the codebase wipes secrets via `crypto.ZeroBytes` /
  `crypto.SecureWipe` rather than manual loops (`crypto/secure_memory.go`).

### 7.4 Changelog (selected)

From `docs/CHANGELOG.md`:

- **1.4.0-qtox-preview (2026-04-04):** qTox C-API compatibility suite; configurable
  `MaxMessagesPerRecipient`; WAL persistence on by default for storage nodes; TCP relay NAT
  traversal on by default; group peer auto-discovery; DHT-based async storage discovery; delivery
  receipts. Confirmed `flynn/noise` v1.1.0 patched against CVE-2021-4239; added security warning for
  `EnableLegacyFallback`.

> For the full, authoritative version history, see `docs/CHANGELOG.md`.

---

*Generated from source analysis of `opd-ai/toxcore`. All constants, sizes, and type definitions are
cited to their defining files. Where the implementation and this document disagree, the source code
is authoritative — please file an issue so this specification can be corrected.*
