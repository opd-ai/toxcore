# Tox Single-Hop Proxy Extension (TSP/2.0)

## 1. Extension Summary

TSP/2.0 provides lightweight network metadata protection through optional proxy routing while preserving cryptographic identity. It extends TSP/1.0 with **indirect friend discovery** — an onion-compatible announce/find flow enabling DHT-based peer discovery without IP exposure.

TSP/2.0 supports three path lengths (0-hop direct, 1-hop single proxy, 3-hop triple relay) and two discovery operations (announce 0x03, lookup 0x04). The Routing Mode byte encodes both: 0x00–0x02 for relay path length, 0x03–0x04 for discovery operations. 1-hop is preferred; 3-hop is available for c-toxcore interop or explicit user request. Strong anonymity is deferred to Tor/I2P.

TSP protects against casual metadata collection but not global surveillance or sophisticated traffic analysis. All Ed25519 signatures and friend verification remain unchanged.

**New in TSP/2.0**: Friend discovery always uses TSP relay infrastructure, even when Tor/I2P transports are active for data-plane traffic, avoiding metadata leaks to exit/outproxy nodes.

## 2. Message Format Specification

### TSP Message Header

```
TSP_MESSAGE Packet Structure (UDP payload):
┌─────────────────────────────────────────────────────────────┐
│ Packet Type (1 byte): 0xF1 (TSP_PROXIED_MESSAGE)          │
├─────────────────────────────────────────────────────────────┤
│ Version (1 byte): 0x02 (TSP/2.0)                          │
├─────────────────────────────────────────────────────────────┤
│ Routing Mode (1 byte):                                     │
│   0x00 = Direct (0-hop)                                    │
│   0x01 = Single Proxy (1-hop)                             │
│   0x02 = Triple Relay (3-hop, discovery only)             │
│   0x03 = Discovery Announce (see §8)                      │
│   0x04 = Discovery Lookup (see §8)                        │
│   0x05-0xFF = RESERVED (reject with TSP_ERROR_UNSUPPORTED) │
├─────────────────────────────────────────────────────────────┤
│ Session ID (8 bytes): Random session identifier            │
├─────────────────────────────────────────────────────────────┤
│ Payload Length (2 bytes): Length of encrypted payload      │
│   Valid range: 64-1400 bytes                              │
├─────────────────────────────────────────────────────────────┤
│ Reserved Flags (2 bytes): Must be 0x0000                  │
├─────────────────────────────────────────────────────────────┤
│ Encrypted Payload (variable): Mode-specific content        │
└─────────────────────────────────────────────────────────────┘

Total header size: 15 bytes
Maximum packet size: 1415 bytes (MTU-safe for most networks)
```

### 0-Hop Mode Payload (Direct)

```
Direct Message Payload (encrypted with shared secret):
┌─────────────────────────────────────────────────────────────┐
│ Sender Public Key (32 bytes): Ed25519 sender identity      │
├─────────────────────────────────────────────────────────────┤
│ Timestamp (8 bytes): Unix timestamp in nanoseconds         │
├─────────────────────────────────────────────────────────────┤
│ Message Type (1 byte): Standard Tox message type           │
├─────────────────────────────────────────────────────────────┤
│ Message Length (2 bytes): Length of message data           │
├─────────────────────────────────────────────────────────────┤
│ Message Data (variable): Original Tox message content      │
├─────────────────────────────────────────────────────────────┤
│ Signature (64 bytes): Ed25519(sender_private, hash(payload))│
└─────────────────────────────────────────────────────────────┘

Encryption: ChaCha20-Poly1305 with curve25519(sender_private, recipient_public)
```

### 1-Hop Mode Payload Structure

```
Single Proxy Payload (encrypted with proxy's public key):
┌─────────────────────────────────────────────────────────────┐
│ Target Public Key (32 bytes): Final recipient's Ed25519    │
├─────────────────────────────────────────────────────────────┤
│ Proxy Instructions (1 byte):                               │
│   0x01 = FORWARD_MESSAGE                                   │
│   0x02 = PING_TARGET (connectivity test)                   │
│   0x03 = FORWARD_WITH_ACK (request delivery confirmation)  │
├─────────────────────────────────────────────────────────────┤
│ Inner Message Length (2 bytes): Length of encapsulated msg │
├─────────────────────────────────────────────────────────────┤
│ Inner Message (variable): Encrypted message for recipient  │
│   [Same structure as 0-hop payload, encrypted for target]  │
├─────────────────────────────────────────────────────────────┤
│ Padding (variable): Random data to reach fixed size        │
│   Total payload padded to: 512, 1024, or 1360 bytes       │
└─────────────────────────────────────────────────────────────┘

Outer encryption: ChaCha20-Poly1305 with curve25519(sender_private, proxy_public)
Inner encryption: ChaCha20-Poly1305 with curve25519(sender_private, target_public)
```

## 3. Protocol Flows

### 0-Hop Mode (Direct Connection)

```
Standard Tox Flow (for reference):
Sender → Recipient: Standard Tox message
```

TSP 0-hop mode uses standard Tox UDP packets with original message format. No TSP headers are used.

### 1-Hop Mode (Single Proxy)

```
Phase 1: Proxy Discovery and Selection
1. Sender queries DHT for TSP proxy advertisements
2. Sender validates proxy capabilities and reputation
3. Sender establishes ephemeral session with selected proxy

Phase 2: Message Transmission
1. Sender → Proxy: TSP_MESSAGE(1-hop, encrypted_outer_payload)
   - Outer payload encrypted with proxy's public key
2. Proxy Processing:
   - Decrypts outer payload, extracts target and inner message
   - Validates instructions and rate limits
3. Proxy → Recipient: Standard Tox message format
   - Appears to originate from proxy's IP address
   - Contains original sender signatures for authenticity

Phase 3: Response Handling (Optional)
4. Recipient → Proxy: Standard Tox message (if responding)
5. Proxy → Sender: TSP_MESSAGE(response_forwarding)
   - Proxy may cache responses briefly (30 seconds) or discard
```

### Session Management

```go
type TSPSession struct {
    SessionID    [8]byte
    ProxyPubKey  [32]byte
    CreatedAt    time.Time
    ExpiresAt    time.Time // Max 10 minutes
    MessageCount uint32    // Rate limiting
    LastActivity time.Time
}

const MaxSessionDuration    = 10 * time.Minute
const MaxMessagesPerSession = 100
```

## 4. Proxy Node Specification

### Proxy Discovery Mechanism

```go
type TSPProxyAnnouncement struct {
    ProxyPublicKey [32]byte  // Ed25519 proxy identity
    Capabilities   uint16    // Bitfield of supported features
    RateLimit      uint32    // Messages per minute capacity
    Uptime         uint32    // Seconds of continuous operation
    Version        uint8     // TSP protocol version (0x02)
    Signature      [64]byte  // Self-signed announcement
}

const (
    TSP_CAP_FORWARD_MESSAGES   = 1 << 0
    TSP_CAP_DELIVERY_ACK       = 1 << 1
    TSP_CAP_RESPONSE_CACHE     = 1 << 2
    TSP_CAP_PING_RELAY         = 1 << 3
    TSP_CAP_DISCOVERY_ANNOUNCE = 1 << 4
    TSP_CAP_DISCOVERY_LOOKUP   = 1 << 5
    TSP_CAP_TRIPLE_RELAY       = 1 << 6
)
```

Proxies announce availability via DHT every 5 minutes (packet type `0xF2`) with rate limits, uptime, and capability bitfields.

### Trust and Reputation

```go
type ProxyReputation struct {
    SuccessfulForwards uint32
    FailedForwards     uint32
    AvgLatency         time.Duration
    LastSeen           time.Time
    TrustScore         float64 // 0.0-1.0
}

func calculateTrustScore(rep ProxyReputation) float64 {
    total := rep.SuccessfulForwards + rep.FailedForwards
    if total < 10 {
        return 0.5
    }
    successRate := float64(rep.SuccessfulForwards) / float64(total)
    latencyPenalty := math.Min(float64(rep.AvgLatency)/time.Second, 1.0)
    uptimeFactor := math.Min(time.Since(rep.LastSeen)/time.Hour, 1.0)
    return successRate * (1.0 - latencyPenalty*0.2) * (1.0 - uptimeFactor*0.3)
}
```

### Resource Limits and DoS Prevention

```go
type ProxyRateLimiter struct {
    IPLimits     map[string]*rate.Limiter // Per-IP rate limiting
    GlobalLimit  *rate.Limiter            // Global proxy capacity
    MaxIPRate    int                      // 10 msg/minute per IP
    MaxGlobalRate int                     // 1000 msg/minute total
    MaxSessions  int                      // 100 concurrent sessions
}

const (
    MaxPacketSize       = 1415  // Bytes
    MaxSessionsPerIP    = 5
    MaxMessageLength    = 1024  // Inner message size limit
    SessionTimeout      = 600   // Seconds
    ProxySelectionLimit = 3     // Max proxies to try
)
```

## 5. Client Implementation

### Mode Selection Logic

```go
type TSPConfig struct {
    DefaultMode     TSPMode
    ProxyThreshold  float64       // Min proxy trust score (0.7)
    FallbackTimeout time.Duration // 5 seconds
    MaxRetries      int           // 3 attempts
    DiscoveryMode   TSPDiscoveryMode
}

type TSPMode int
const (
    TSP_AUTO        TSPMode = iota
    TSP_DIRECT_ONLY
    TSP_PROXY_ONLY
)

// Discovery always uses TSP relay infrastructure, even with Tor/I2P active.
type TSPDiscoveryMode int
const (
    TSP_DISCOVERY_AUTO  TSPDiscoveryMode = iota // 1-hop default, 3-hop fallback
    TSP_DISCOVERY_1HOP                          // Force single-relay
    TSP_DISCOVERY_3HOP                          // Force 3-hop onion-compatible
)

func (c *TSPClient) selectMode(recipient PublicKey, config TSPConfig) TSPMode {
    switch config.DefaultMode {
    case TSP_DIRECT_ONLY:
        return TSP_DIRECT_ONLY
    case TSP_PROXY_ONLY:
        return TSP_PROXY_ONLY
    default:
        return c.adaptiveSelection(recipient, config)
    }
}

func (c *TSPClient) adaptiveSelection(recipient PublicKey, config TSPConfig) TSPMode {
    if len(c.getAvailableProxies(config.ProxyThreshold)) == 0 {
        return TSP_DIRECT_ONLY
    }
    if c.recipientPrefersProxy(recipient) {
        return TSP_PROXY_ONLY
    }
    return TSP_PROXY_ONLY
}
```

### Fallback Behavior Flowchart

```
┌─────────────────┐
│ Send Message    │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐    No     ┌─────────────────┐
│ TSP Enabled?    │──────────▶│ Standard Tox    │
└─────────┬───────┘           │ Direct Send     │
          │ Yes               └─────────────────┘
          ▼
┌─────────────────┐
│ Mode Selection  │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐    Direct  ┌─────────────────┐
│ Selected Mode?  │───────────▶│ TSP 0-hop       │
└─────────┬───────┘            │ (Direct)        │
          │ Proxy              └─────────────────┘
          ▼
┌─────────────────┐    No      ┌─────────────────┐
│ Proxy Available?│───────────▶│ Fallback to     │
└─────────┬───────┘            │ Direct Mode     │
          │ Yes                └─────────────────┘
          ▼
┌─────────────────┐
│ TSP 1-hop Send  │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐    Timeout ┌─────────────────┐
│ Success?        │───────────▶│ Retry with      │
└─────────┬───────┘            │ Different Proxy │
          │ Success            └─────────┬───────┘
          ▼                             │
┌─────────────────┐              ┌─────▼─────┐
│ Message Sent    │              │ Max       │
└─────────────────┘              │ Retries?  │
                                 └─────┬─────┘
                                       │ Yes
                                       ▼
                                 ┌─────────────────┐
                                 │ Fallback to     │
                                 │ Direct Mode     │
                                 └─────────────────┘
```

### UI/UX Considerations

```go
type TSPStatus struct {
    CurrentMode  TSPMode
    ProxyUsed    *ProxyInfo
    LastFallback time.Time
    MessagesSent uint32
    PrivacyLevel string // "High", "Medium", "Low"
}

func (s TSPStatus) getPrivacyIcon() string {
    switch s.CurrentMode {
    case TSP_DIRECT_ONLY:  return "🔓" // Direct connection
    case TSP_PROXY_ONLY:   return "🔒" // Proxied connection
    default:               return "🔄" // Auto mode
    }
}
```

### Performance Impact Estimates

```go
const (
    TypicalProxyLatency    = 100 * time.Millisecond // +50-200ms typical
    EncryptionOverhead     = 107                     // bytes (headers + auth tags)
    PaddingOverhead        = 256                     // bytes average
    BandwidthOverheadPct   = 15                      // percent
    CPUOverheadPct         = 5                       // percent
)
```

## 6. Security Analysis

### Protected Against

1. **Recipient IP Discovery**: Recipients cannot determine sender's real IP address in 1-hop mode
2. **Casual Network Surveillance**: Observers monitoring only sender→proxy OR proxy→recipient cannot link communications
3. **Message Replay Attacks**: Session IDs and timestamps prevent replay of TSP messages
4. **Proxy Impersonation**: Ed25519 signatures prevent unauthorized proxy announcements
5. **Message Forgery**: Original Tox cryptographic signatures preserved, ensuring authenticity

### Not Protected Against

1. **Global Network Surveillance**: Adversaries monitoring both sender→proxy AND proxy→recipient can correlate traffic
2. **Compromised Proxies**: Malicious proxies can log metadata and perform timing analysis
3. **Long-term Traffic Analysis**: Patterns over multiple sessions may reveal sender-recipient relationships
4. **Multi-hop Anonymity**: TSP does not provide Tor/I2P-equivalent anonymity
5. **Quantum Cryptanalysis**: Same primitives as base Tox protocol
6. **Announce Node Compromise**: In 1-hop discovery, relay learns announcer's pseudonym and IP; mitigated in 3-hop mode

### Attack Scenarios

**Attack 1: Timing Correlation**
- *Mitigation*: Random delays (50-200ms) injected by proxy before forwarding
- *Limitation*: Sophisticated timing analysis may still succeed with large datasets

**Attack 2: Malicious Proxy Logging**
- *Mitigation*: Proxy reputation system, proxy rotation, ephemeral sessions
- *Limitation*: Cannot prevent logging, only detect unreliable proxies post-facto

**Attack 3: DHT Eclipse Attack**
- *Mitigation*: Proxy reputation validation, signature verification, diversity requirements
- *Limitation*: New users with no reputation data remain vulnerable

### Privacy Guarantees

**Formal Privacy Claims:**

1. **IP Address Unlinkability**: Recipient R cannot determine sender S's IP with probability > 1/|P| where |P| is the set of possible proxy nodes.
2. **Partial Observer Resistance**: An adversary observing only sender→proxy OR proxy→recipient cannot link the parties with probability better than random.
3. **Authentication Preservation**: All TSP messages maintain standard Tox Ed25519 signature authenticity.

**Non-Guarantees:**
- TSP provides no protection against global passive adversaries
- Proxy nodes can always see sender→recipient mappings  
- Long-term traffic analysis may compromise privacy
- Physical/legal proxy compromise reveals historical logs

## 7. Example Implementation

```go
package tsp

import (
    "crypto/rand"
    "errors"
    "time"
)

type Client struct {
    privateKey [32]byte
    publicKey  [32]byte
    proxyCache map[[32]byte]*ProxyInfo
    sessions   map[[8]byte]*TSPSession
    config     TSPConfig
}
func (c *Client) selectProxyNode(recipient [32]byte) (*ProxyInfo, error) {
    proxies := c.getDHTProxies()
    if len(proxies) == 0 {
        return nil, errors.New("no proxies available")
    }

    var best *ProxyInfo
    var bestScore float64
    for _, proxy := range proxies {
        if proxy.TrustScore >= c.config.ProxyThreshold &&
            proxy.hasCapability(TSP_CAP_FORWARD_MESSAGES) {
            score := proxy.TrustScore*0.7 + (1.0-float64(proxy.AvgLatency)/float64(time.Second))*0.3
            if best == nil || score > bestScore {
                best, bestScore = proxy, score
            }
        }
    }
    if best == nil {
        return nil, errors.New("no trusted proxies available")
    }
    return best, nil
}

func (c *Client) encapsulateForProxy(proxy *ProxyInfo, recipient [32]byte, message []byte) ([]byte, error) {
    sessionID := make([]byte, 8)
    rand.Read(sessionID)
    innerMsg, err := c.createInnerMessage(recipient, message)
    if err != nil {
        return nil, err
    }
    proxyPayload := ProxyPayload{
        TargetPubKey: recipient,
        Instructions: TSP_FORWARD_MESSAGE,
        InnerMsgLen:  uint16(len(innerMsg)),
        InnerMessage: innerMsg,
        Padding:      c.generatePadding(innerMsg),
    }
    encryptedPayload, err := c.encryptForProxy(proxy.PublicKey, proxyPayload)
    if err != nil {
        return nil, err
    }
    tspPacket := TSPPacket{
        Type: 0xF1, Version: 0x02, RoutingMode: 0x01,
        SessionID: sessionID, PayloadLen: uint16(len(encryptedPayload)),
        Reserved: 0x0000, Payload: encryptedPayload,
    }
    return tspPacket.marshal(), nil
}

func (c *Client) verifyProxiedMessage(packet []byte) (*VerifiedMessage, error) {
    tsp, err := unmarshalTSPPacket(packet)
    if err != nil {
        return nil, err
    }
    decrypted, err := c.decryptPayload(tsp.Payload)
    if err != nil {
        return nil, err
    }
    innerMsg, err := extractInnerMessage(decrypted)
    if err != nil {
        return nil, err
    }

    msgHash := c.hashMessage(innerMsg.Data, innerMsg.Timestamp)    if !c.verifySignature(innerMsg.SenderPubKey, msgHash, innerMsg.Signature) {
        return nil, errors.New("invalid message signature")
    }
    if !c.isTrustedFriend(innerMsg.SenderPubKey) {
        return nil, errors.New("message from unknown sender")
    }
    return &VerifiedMessage{
        Sender: innerMsg.SenderPubKey, Data: innerMsg.Data,
        Timestamp: innerMsg.Timestamp, ViaProxy: true,
    }, nil
}

func (c *Client) generatePadding(innerMsg []byte) []byte {
    currentSize := len(innerMsg) + 35
    var targetSize int
    switch {
    case currentSize <= 512:  targetSize = 512
    case currentSize <= 1024: targetSize = 1024
    default:                  targetSize = 1360
    }
    paddingSize := targetSize - currentSize
    if paddingSize <= 0 {
        return nil
    }
    padding := make([]byte, paddingSize)
    rand.Read(padding)
    return padding
}

func (c *Client) SendMessage(recipient [32]byte, message []byte) error {
    mode := c.selectMode(recipient, c.config)
    switch mode {
    case TSP_DIRECT_ONLY:
        return c.sendDirectMessage(recipient, message)
    case TSP_PROXY_ONLY:
        proxy, err := c.selectProxyNode(recipient)
        if err != nil {
            if c.config.AllowFallback {
                return c.sendDirectMessage(recipient, message)
            }
            return err
        }
        return c.sendViaProxy(proxy, recipient, message)
    default:
        return errors.New("invalid TSP mode")
    }
}
```

## 8. Indirect Friend Discovery (TSP/2.0)

### 8.1 Motivation

c-toxcore provides anonymous friend discovery via 3-hop onion-routed announce/find within the DHT. TSP/1.0 had no equivalent — users' IPs were exposed during DHT lookups. TSP/2.0 closes this gap with **indirect friend discovery** reusing TSP proxy infrastructure, wire-compatible with c-toxcore onion announce packets (`PacketOnionAnnounceRequest`, `PacketOnionAnnounceResponse`).

**Key design decisions:**

1. **1-hop by default**: Sufficient for hiding IP from friends and casual observers.
2. **3-hop when required**: For c-toxcore interop or explicit user request for enhanced path diversity.
3. **Always use Tox-native discovery**: Even with Tor/I2P active for data-plane, discovery uses Tox DHT via TSP relays to avoid leaking lookup patterns to exit/outproxy nodes.

### 8.2 Discovery Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                    Friend Discovery Flow                         │
├──────────────────────────────────────────────────────────────────┤
│  Phase 1: ANNOUNCE (Alice publishes her presence)                │
│  1-Hop (default):  Alice → Relay R → Announce Nodes              │
│  3-Hop (compat):   Alice → R1 → R2 → R3 → Announce Nodes        │
│                                                                  │
│  Phase 2: LOOKUP (Bob searches for Alice)                        │
│  1-Hop (default):  Bob → Relay R → Announce Nodes → R → Bob      │
│  3-Hop (compat):   Bob → R1→R2→R3 → Nodes → R3→R2→R1 → Bob      │
│                                                                  │
│  Phase 3: CONNECT                                                │
│  Bob connects to Alice via TSP messaging (0-hop or 1-hop per §3) │
│  If Tor/I2P active, data-plane uses that transport               │
└──────────────────────────────────────────────────────────────────┘
```

### 8.3 Announce Protocol

#### Announce Key Derivation

The announce namespace uses c-toxcore's key-distance scheme: the client hashes its long-term public key with a secret salt to produce an **announce location** that rotates periodically:

```go
type AnnounceLocation struct {
    LocationKey  [32]byte // H(LongTermPK || AnnounceSalt || Epoch)
    Epoch        uint64
    AnnounceSalt [32]byte // Random per-instance, persisted across restarts
}

const (
    AnnounceEpoch      = 5 * time.Minute  // Location rotation interval
    AnnounceRefresh    = 2 * time.Minute  // Re-announce interval within epoch
    AnnounceMaxAge     = 10 * time.Minute // Storage nodes discard older entries
    AnnounceMaxPerNode = 32               // Max entries per storage node
)

func deriveAnnounceLocation(publicKey, salt [32]byte, epoch uint64) [32]byte {
    h := sha256.New()
    h.Write(publicKey[:])
    h.Write(salt[:])
    epochBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(epochBytes, epoch)
    h.Write(epochBytes)
    var loc [32]byte
    copy(loc[:], h.Sum(nil))
    return loc
}
```

#### Announce Payload Structure

```
TSP Discovery Announce Payload (Routing Mode 0x03):
┌─────────────────────────────────────────────────────────────┐
│ Announce Location Key (32 bytes): H(PK||salt||epoch)        │
├─────────────────────────────────────────────────────────────┤
│ Encrypted Announce Data (variable):                         │
│   Encrypted with curve25519(announcer_private, node_public) │
│   Contains:                                                 │
│     Announcer Long-Term PK (32 bytes)                       │
│     DHT Temporary PK (32 bytes)                             │
│     Connected Node IPs (variable, up to 4 nodes)            │
│       Each: [IP type (1)][IP addr (4 or 16)][port (2)]     │
│     Announce Timestamp (8 bytes)                            │
│     Ping ID (8 bytes): random, echoed in response           │
├─────────────────────────────────────────────────────────────┤
│ Return Path (encrypted, per-relay):                         │
│   For 1-hop: [Relay PK (32)][Encrypted return addr (48)]    │
│   For 3-hop: 3 × [Relay PK (32)][Encrypted return addr]    │
├─────────────────────────────────────────────────────────────┤
│ Padding (variable): Random data to fixed size               │
└─────────────────────────────────────────────────────────────┘
```

#### Announce Storage Node Behavior

Nodes closest (by XOR distance) to the `AnnounceLocation` key store announce entries:

```go
type AnnounceEntry struct {
    LocationKey   [32]byte
    EncryptedData []byte    // Opaque to storage node
    StoredAt      time.Time
    PingID        [8]byte
    ReturnPath    []byte
}

type AnnounceStorage struct {
    entries    map[[32]byte]*AnnounceEntry
    maxEntries int // AnnounceMaxPerNode (32)
    mu         sync.RWMutex
}
// StoreAnnounce stores or updates an entry. Rejects if full and farther than all existing.
func (as *AnnounceStorage) StoreAnnounce(entry *AnnounceEntry) error {
    as.mu.Lock()
    defer as.mu.Unlock()
    as.purgeExpired()
    if len(as.entries) < as.maxEntries {
        as.entries[entry.LocationKey] = entry
        return nil
    }
    return as.replaceIfCloser(entry)
}
```

### 8.4 Lookup Protocol

#### Lookup Request Payload

```
TSP Discovery Lookup Payload (Routing Mode 0x04):
┌─────────────────────────────────────────────────────────────┐
│ Search Location Key (32 bytes): H(target_PK||salt||epoch)   │
│   Searcher must know or derive target's announce location   │
├─────────────────────────────────────────────────────────────┤
│ Searcher Temporary PK (32 bytes): ephemeral key for reply   │
├─────────────────────────────────────────────────────────────┤
│ Request Nonce (24 bytes): for encrypting the response       │
├─────────────────────────────────────────────────────────────┤
│ Return Path (encrypted, per-relay):                         │
│   Same structure as announce return path                    │
├─────────────────────────────────────────────────────────────┤
│ Padding (variable): Random data to fixed size               │
└─────────────────────────────────────────────────────────────┘
```

#### Lookup Response

When an announce storage node holds a matching entry, it returns the data via the return path:

```
TSP Discovery Lookup Response:
┌─────────────────────────────────────────────────────────────┐
│ Found (1 byte): 0x01 = found, 0x00 = not found             │
├─────────────────────────────────────────────────────────────┤
│ Encrypted Announce Data (variable): (only if found)         │
│   Same blob stored by the announcer                         │
│   Searcher decrypts using knowledge of announcer's PK       │
├─────────────────────────────────────────────────────────────┤
│ Closest Nodes (variable, up to 4): (if not found)           │
│   Nodes closer to the search key for iterative lookup       │
│   Each: [PK (32)][IP type (1)][IP addr (4 or 16)][port (2)]│
└─────────────────────────────────────────────────────────────┘
```

### 8.5 Announce Salt Exchange Between Friends

For a searcher (Bob) to derive Alice's `AnnounceLocation`, Bob must know Alice's announce salt, exchanged over the authenticated friend channel:

```go
type AnnounceSaltExchange struct {
    Type         string   `json:"type"`          // "announce_salt_exchange"
    SenderPK     [32]byte `json:"sender_pk"`
    AnnounceSalt [32]byte `json:"announce_salt"` // Encrypted for recipient
    ValidFrom    time.Time `json:"valid_from"`
    ValidUntil   time.Time `json:"valid_until"`  // Salt rotation (24 hours)
    Signature    [64]byte `json:"signature"`
}

const (
    SaltRotationInterval = 24 * time.Hour
    SaltExchangeRetry    = 5 * time.Minute
)
```

Salts are exchanged when both parties are online and stored locally. Old salts are kept for one additional rotation period for clock skew tolerance.

### 8.6 1-Hop Discovery Flow (Default)

```
1-Hop Announce:
  Alice → Relay → AnnounceNode(closest to H(Alice_PK||salt||epoch))
    - Announce data encrypted for AnnounceNode, outer envelope for Relay
    - Relay decrypts outer layer, forwards; AnnounceNode stores entry

1-Hop Lookup:
  Bob → Relay → AnnounceNode → Relay → Bob
    - Bob sends lookup through Relay; AnnounceNode returns via Relay
    - Bob decrypts using knowledge of Alice's PK

Privacy properties (1-hop discovery):
  - Relay knows Bob's IP but not search target (salted hash key)
  - AnnounceNode knows search key but not Bob's IP (sees Relay's IP)
  - Neither party alone can link Bob to Alice
```

### 8.7 3-Hop Discovery Flow (c-toxcore Compatible)

The 3-hop flow constructs a chain of 3 TSP-capable relay nodes, matching c-toxcore's onion path length. Used when the user sets `TSP_DISCOVERY_3HOP`, announce nodes require c-toxcore onion format, or `TSP_DISCOVERY_AUTO` escalates after 1-hop failure.

```
3-Hop Announce:
  Alice → R1 → R2 → R3 → AnnounceNode
    Layer 3 (outermost): Encrypted for R1, contains R2 addr + layer 2
    Layer 2: Encrypted for R2, contains R3 addr + layer 1
    Layer 1: Encrypted for R3, contains AnnounceNode addr + data

3-Hop Lookup:
  Bob → R1 → R2 → R3 → AnnounceNode → R3 → R2 → R1 → Bob
    Forward: same layered encryption; Return: symmetric per-relay keys

Privacy properties (3-hop discovery):
  - R1 knows Bob's IP but only R2's identity
  - R2 knows R1 and R3 but not Bob or AnnounceNode
  - R3 knows R2 and AnnounceNode but not Bob or R1
  - Same privacy level as c-toxcore onion routing
```

#### 3-Hop Relay Chain Construction

```go
// buildRelayChain selects 3 relays from different /16 subnets.
func (c *TSPClient) buildRelayChain(target [32]byte) ([3]*RelayNode, error) {
    candidates := c.getAvailableRelays(TSP_CAP_TRIPLE_RELAY)
    if len(candidates) < 3 {
        return [3]*RelayNode{}, errors.New("insufficient relay nodes for 3-hop chain")
    }
    chain := [3]*RelayNode{}
    usedSubnets := make(map[string]bool)
    for i := 0; i < 3; i++ {
        relay, err := selectDiverseRelay(candidates, usedSubnets)
        if err != nil {
            return chain, fmt.Errorf("relay chain failed at hop %d: %w", i, err)
        }
        chain[i] = relay
        usedSubnets[relay.Subnet()] = true
    }
    return chain, nil
}

// createOnionLayers wraps payload in 3 encryption layers (c-toxcore compatible).
func (c *TSPClient) createOnionLayers(chain [3]*RelayNode, innerPayload []byte) ([]byte, error) {
    layer1, err := c.encryptForRelay(chain[2], innerPayload)
    if err != nil {
        return nil, err
    }
    layer2Payload := append(chain[2].AddressBytes(), layer1...)
    layer2, err := c.encryptForRelay(chain[1], layer2Payload)
    if err != nil {
        return nil, err
    }
    layer3Payload := append(chain[1].AddressBytes(), layer2...)
    return c.encryptForRelay(chain[0], layer3Payload)
}
```

### 8.8 c-toxcore Onion Compatibility

TSP/2.0 discovery interoperates with c-toxcore's onion announce system.

#### Packet Type Mapping

| c-toxcore Packet Type | TSP/2.0 Equivalent | Notes |
|---|---|---|
| `OnionAnnounceRequest` | TSP Routing Mode `0x03` (Announce) | Same announce key derivation; TSP relay wrapping is stripped before reaching announce nodes |
| `OnionAnnounceResponse` | Response via TSP return path | Announce nodes see identical request format |
| `OnionDataRequest` | TSP Routing Mode `0x04` (Lookup) | Search key compatible |
| `OnionDataResponse` | Response via TSP return path | Same encrypted blob format |

#### Interoperability Modes

1. **TSP-native** (default): Both parties are toxcore-go. Uses routing modes `0x03`/`0x04` with 1-hop or 3-hop. Announce payload format-compatible with c-toxcore at the announce node.

2. **c-toxcore compatibility**: toxcore-go builds a 3-hop chain; R3 unwraps the TSP envelope and re-emits as `PacketOnionAnnounceRequest`. Responses follow the reverse path.

3. **Mixed network**: toxcore-go and c-toxcore entries coexist at announce nodes, indexed by the same key-distance metric.

```go
// Packet type constants matching transport/packet.go for c-toxcore interop.
const (
    PacketOnionAnnounceRequest  = transport.PacketOnionAnnounceRequest  // 0x0E
    PacketOnionAnnounceResponse = transport.PacketOnionAnnounceResponse // 0x0F
    PacketOnionDataRequest      = transport.PacketOnionDataRequest      // 0x10
    PacketOnionDataResponse     = transport.PacketOnionDataResponse     // 0x11
)
```

### 8.9 Discovery When Tor/I2P Is Active

**Requirement**: Friend discovery MUST always use the Tox DHT via TSP relays, regardless of data-plane transport configuration.

**Rationale**: Tor exit nodes and I2P outproxies must not observe DHT traffic. Using the Tox DHT consistently ensures universal discoverability across all transport configurations.

```go
type DiscoveryTransportPolicy struct {
    DataPlaneTransport transport.Transport // User's configured transport
    DiscoveryTransport *TSPTransport       // Always Tox UDP/TCP + TSP overlay
}

func (p *DiscoveryTransportPolicy) Announce(location AnnounceLocation, data []byte) error {
    return p.DiscoveryTransport.SendAnnounce(location, data)
}

func (p *DiscoveryTransportPolicy) Lookup(location [32]byte) (*AnnounceEntry, error) {
    return p.DiscoveryTransport.SendLookup(location)
}

func (p *DiscoveryTransportPolicy) Connect(peerAddr net.Addr) (net.Conn, error) {
    return p.DataPlaneTransport.Dial(peerAddr)
}
```

**Example flow (Tor active)**:
1. Alice announces via Tox DHT + TSP (1-hop relay, clearnet UDP)
2. Bob discovers Alice via Tox DHT + TSP (1-hop relay, clearnet UDP)
3. Bob connects to Alice via Tor; all subsequent messages flow over Tor

Steps 1-2 are protected by TSP relay (Bob's IP hidden from announce nodes). Steps 3+ use Tor for strong data-plane anonymity.

### 8.10 Discovery Security Analysis

#### Threat Model for Discovery

| Threat | 1-Hop Discovery | 3-Hop Discovery |
|---|---|---|
| **Announce node learns announcer's IP** | ❌ Protected (sees relay IP) | ❌ Protected (sees R3 IP) |
| **Single relay learns announce key + announcer IP** | ⚠️ Yes (relay sees both) | ❌ R1 sees IP only, R3 sees announce only |
| **Announce node learns searcher's IP** | ❌ Protected (sees relay IP) | ❌ Protected (sees R3 IP) |
| **Single relay learns search key + searcher IP** | ⚠️ Yes (relay sees both) | ❌ R1 sees IP only, R3 sees search only |
| **Passive observer correlates announce + lookup** | ⚠️ Possible at relay | ❌ Requires compromising all 3 relays |
| **DHT eclipse attack on announce namespace** | ⚠️ Same as c-toxcore | ⚠️ Same as c-toxcore |

#### 1-Hop Discovery: Acceptable Tradeoffs

In 1-hop mode, the relay sees both the client's IP and the announce/search key (same tradeoff as §6). This is acceptable because: the relay is client-selected and rotatable, the announce key is a salted hash (not raw PK), and users requiring stronger protection can enable 3-hop or Tor/I2P.

#### 3-Hop Discovery: c-toxcore Equivalent

In 3-hop mode, no single relay learns both client IP and announce/search key, matching c-toxcore's privacy properties. Known limitations: timing correlation across all 3 relays, Sybil attacks via relay chain compromise, and announce node enumeration near a target's key.

#### CVE-2018-25022 Mitigation

Not applicable to TSP discovery: TSP relays do not relay NAT ping requests (only announce/lookup payloads), forwarding is strictly typed to modes `0x03` and `0x04`, and announce nodes cannot send arbitrary packets through the return path.

### 8.11 Discovery Configuration

```go
type TSPDiscoveryConfig struct {
    Mode             TSPDiscoveryMode
    AnnounceInterval time.Duration // Default: 2 min
    LookupTimeout    time.Duration // Default: 10 sec
    SaltRotation     time.Duration // Default: 24h
    MaxAnnounceNodes int           // Default: 8
    FallbackTo3Hop   bool          // Auto-escalate on 1-hop failure (default: true)
    AlwaysUseToxDHT  bool          // MUST be true
}

func DefaultDiscoveryConfig() TSPDiscoveryConfig {
    return TSPDiscoveryConfig{
        Mode:             TSP_DISCOVERY_AUTO,
        AnnounceInterval: 2 * time.Minute,
        LookupTimeout:    10 * time.Second,
        SaltRotation:     24 * time.Hour,
        MaxAnnounceNodes: 8,
        FallbackTo3Hop:   true,
        AlwaysUseToxDHT:  true,
    }
}
```

### 8.12 Migration from TSP/1.0

TSP/2.0 is backward compatible with TSP/1.0:

- TSP/1.0 clients ignore routing modes `0x02`–`0x04` (`TSP_ERROR_UNSUPPORTED`)
- A TSP/2.0 sender MUST use version byte `0x01` with legacy modes `0x00`/`0x01` when communicating with TSP/1.0 peers
- Version byte `0x02` is reserved for TSP/2.0 semantics (discovery modes `0x03`/`0x04`)
- A TSP/2.0 receiver SHOULD accept both versions; version `0x01` packets use TSP/1.0 semantics only
- A TSP/2.0 sender MUST NOT send version `0x02` until TSP/2.0 support is established (TSP/1.0 receivers may reject unknown versions)
- Discovery features require TSP/2.0 on both sides; proxy capability flags advertise support before sending TSP/2.0 traffic
- Proxy announcements with `TSP_CAP_DISCOVERY_ANNOUNCE` or `TSP_CAP_DISCOVERY_LOOKUP` flags indicate TSP/2.0 support