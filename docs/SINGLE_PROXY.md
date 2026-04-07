# Tox Single-Hop Proxy Extension (TSP/2.0)

## 1. Extension Summary

The Tox Single-Hop Proxy Extension (TSP/2.0) provides lightweight network metadata protection through optional proxy routing while maintaining full cryptographic identity preservation. TSP/2.0 extends the original TSP/1.0 specification with **indirect friend discovery** — an onion-compatible announce/find flow that enables users to locate each other in the DHT without revealing their IP addresses to intermediate nodes.

TSP/2.0 supports three path lengths — direct connection (0-hop), single proxy relay (1-hop, the default and preferred mode), and triple-relay (3-hop, used only when the threat model requires c-toxcore onion-level path diversity) — and two discovery operation types: announce (0x03) and lookup (0x04). Path lengths control how many relay hops protect the sender's IP, while operation types specify the DHT action being performed. The Routing Mode byte in the TSP header encodes both: values 0x00–0x02 denote path length for message relay, and values 0x03–0x04 denote discovery operations (which themselves use 1-hop or 3-hop paths as configured). The design philosophy prioritizes simplicity and performance: 1-hop routing is preferred for both message relay and friend discovery, with 3-hop routing available as a fallback for interoperability with c-toxcore's onion announce protocol or when the user explicitly requests enhanced path diversity. Strong anonymity against global adversaries is explicitly deferred to dedicated anonymity networks (Tor, I2P), and TSP does not attempt to compete with them.

TSP provides protection against casual metadata collection by message recipients and partial network observers, but does not defend against global surveillance or sophisticated traffic analysis. The extension integrates seamlessly with existing Tox cryptography — all Ed25519 signatures and friend verification remain unchanged, ensuring recipients can always verify sender authenticity.

**New in TSP/2.0**: Indirect friend discovery via the Tox DHT is always performed through TSP relay infrastructure, even when Tor or I2P transports are active for data-plane traffic. This ensures consistent friend discovery behavior across all transport configurations and avoids leaking friend-lookup metadata to the underlying anonymity network's exit/outproxy nodes. The Tox DHT is the canonical discovery layer; Tor and I2P are used only for the data-plane connections established after discovery completes.

Key constraints: TSP defaults to single-proxy routing for both messaging and discovery, supports 3-hop routing only for discovery operations when required, maintains backward compatibility with standard Tox clients, and provides graceful fallback when proxies are unavailable.

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

TSP 0-hop mode uses standard Tox UDP packets with original message format. No TSP headers are used. This mode exists only for API consistency when TSP-aware clients communicate directly.

### 1-Hop Mode (Single Proxy)

```
Phase 1: Proxy Discovery and Selection
1. Sender queries DHT for TSP proxy advertisements
2. Sender validates proxy capabilities and reputation
3. Sender establishes ephemeral session with selected proxy

Phase 2: Message Transmission
1. Sender → Proxy: TSP_MESSAGE(1-hop, encrypted_outer_payload)
   - Outer payload encrypted with proxy's public key
   - Contains target recipient and inner encrypted message

2. Proxy Processing:
   - Decrypts outer payload using proxy private key
   - Extracts target public key and inner message
   - Validates proxy instructions and rate limits

3. Proxy → Recipient: Standard Tox message format
   - Appears to originate from proxy's IP address
   - Contains original sender signatures for authenticity
   - Recipient cannot distinguish from direct messages

Phase 3: Response Handling (Optional)
4. Recipient → Proxy: Standard Tox message (if responding)
5. Proxy → Sender: TSP_MESSAGE(response_forwarding)
   - Proxy may cache responses for brief period (30 seconds)
   - Or discard responses (fire-and-forget mode)
```

### Session Management

```go
type TSPSession struct {
    SessionID    [8]byte
    ProxyPubKey  [32]byte
    CreatedAt    time.Time
    ExpiresAt    time.Time    // Max 10 minutes
    MessageCount uint32       // Rate limiting
    LastActivity time.Time
}

// Sessions are ephemeral and automatically expire
const MaxSessionDuration = 10 * time.Minute
const MaxMessagesPerSession = 100
```

## 4. Proxy Node Specification

### Proxy Discovery Mechanism

```go
// DHT announcement packet for proxy capabilities
type TSPProxyAnnouncement struct {
    ProxyPublicKey [32]byte     // Ed25519 proxy identity
    Capabilities   uint16       // Bitfield of supported features
    RateLimit      uint32       // Messages per minute capacity
    Uptime        uint32        // Seconds of continuous operation
    Version       uint8         // TSP protocol version (0x02)
    Signature     [64]byte      // Self-signed announcement
}

// Capability flags
const (
    TSP_CAP_FORWARD_MESSAGES    = 1 << 0  // Basic message forwarding
    TSP_CAP_DELIVERY_ACK        = 1 << 1  // Delivery confirmation
    TSP_CAP_RESPONSE_CACHE      = 1 << 2  // Brief response caching
    TSP_CAP_PING_RELAY          = 1 << 3  // Connectivity testing
    TSP_CAP_DISCOVERY_ANNOUNCE  = 1 << 4  // Friend discovery announce storage
    TSP_CAP_DISCOVERY_LOOKUP    = 1 << 5  // Friend discovery lookup relay
    TSP_CAP_TRIPLE_RELAY        = 1 << 6  // Supports 3-hop relay chains
)
```

Proxies announce availability via DHT every 5 minutes with packet type `0xF2`. Announcements include rate limits, uptime statistics, and capability bitfields.

### Trust and Reputation

```go
type ProxyReputation struct {
    SuccessfulForwards uint32    // Messages successfully delivered
    FailedForwards     uint32    // Failed or dropped messages  
    AvgLatency        time.Duration // Observed forwarding latency
    LastSeen          time.Time  // Most recent activity
    TrustScore        float64    // 0.0-1.0 calculated reputation
}

// Reputation calculation
func calculateTrustScore(rep ProxyReputation) float64 {
    total := rep.SuccessfulForwards + rep.FailedForwards
    if total < 10 {
        return 0.5 // Neutral for new proxies
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
    IPLimits    map[string]*rate.Limiter  // Per-IP rate limiting
    GlobalLimit *rate.Limiter             // Global proxy capacity
    
    // Limits
    MaxIPRate    int                       // 10 msg/minute per IP
    MaxGlobalRate int                      // 1000 msg/minute total
    MaxSessions  int                       // 100 concurrent sessions
}

// DoS prevention measures
const (
    MaxPacketSize        = 1415   // Bytes
    MaxSessionsPerIP     = 5      // Concurrent sessions
    MaxMessageLength     = 1024   // Inner message size limit
    SessionTimeout       = 600    // Seconds
    ProxySelectionLimit  = 3      // Max proxies to try
)
```

## 5. Client Implementation

### Mode Selection Logic

```go
type TSPConfig struct {
    DefaultMode     TSPMode      // AUTO, DIRECT, PROXY_ONLY
    ProxyThreshold  float64      // Min proxy trust score (0.7)
    FallbackTimeout time.Duration // 5 seconds
    MaxRetries      int          // 3 attempts
    DiscoveryMode   TSPDiscoveryMode // DISCOVERY_1HOP, DISCOVERY_3HOP, DISCOVERY_AUTO
}

type TSPMode int
const (
    TSP_AUTO        TSPMode = iota // Adaptive selection
    TSP_DIRECT_ONLY                // Force 0-hop mode
    TSP_PROXY_ONLY                 // Force 1-hop mode
)

// TSPDiscoveryMode controls how friend discovery operations are routed.
// Discovery always uses TSP relay infrastructure, even when Tor/I2P is active.
type TSPDiscoveryMode int
const (
    TSP_DISCOVERY_AUTO  TSPDiscoveryMode = iota // 1-hop default, 3-hop if announce node unreachable
    TSP_DISCOVERY_1HOP                          // Force single-relay discovery (fastest)
    TSP_DISCOVERY_3HOP                          // Force 3-hop onion-compatible discovery
)

func (c *TSPClient) selectMode(recipient PublicKey, config TSPConfig) TSPMode {
    switch config.DefaultMode {
    case TSP_DIRECT_ONLY:
        return TSP_DIRECT_ONLY
    case TSP_PROXY_ONLY:
        return TSP_PROXY_ONLY
    case TSP_AUTO:
        return c.adaptiveSelection(recipient, config)
    }
}

func (c *TSPClient) adaptiveSelection(recipient PublicKey, config TSPConfig) TSPMode {
    // Check proxy availability
    availableProxies := c.getAvailableProxies(config.ProxyThreshold)
    if len(availableProxies) == 0 {
        return TSP_DIRECT_ONLY
    }
    
    // Consider recipient preferences (from past interactions)
    if c.recipientPrefersProxy(recipient) {
        return TSP_PROXY_ONLY
    }
    
    // Default: use proxy for enhanced privacy
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
    CurrentMode   TSPMode       // Active routing mode
    ProxyUsed     *ProxyInfo    // Current proxy details (if any)
    LastFallback  time.Time     // When fallback last occurred
    MessagesSent  uint32        // Messages sent via TSP
    Privacy Level string        // "High", "Medium", "Low"
}

// UI indicators
func (s TSPStatus) getPrivacyIcon() string {
    switch s.CurrentMode {
    case TSP_DIRECT_ONLY:
        return "🔓" // Direct connection
    case TSP_PROXY_ONLY:
        return "🔒" // Proxied connection
    default:
        return "🔄" // Auto mode
    }
}

func (s TSPStatus) getStatusText() string {
    switch s.CurrentMode {
    case TSP_DIRECT_ONLY:
        return "Direct connection (IP visible to recipient)"
    case TSP_PROXY_ONLY:
        return fmt.Sprintf("Proxied via %s (IP hidden)", s.ProxyUsed.Name)
    default:
        return "Automatic mode selection"
    }
}
```

### Performance Impact Estimates

```go
// Performance metrics
type TSPPerformanceImpact struct {
    LatencyOverhead  time.Duration // +50-200ms typical
    BandwidthOverhead float64      // +15% (encryption + padding)
    CPUOverhead      float64       // +5% (additional crypto)
    BatteryImpact    float64       // +2% (extra network round trip)
}

const (
    TypicalProxyLatency     = 100 * time.Millisecond
    EncryptionOverhead      = 107 // bytes (headers + auth tags)
    PaddingOverhead         = 256 // bytes average
    DHTPseudonymsOverhead   = 32  // bytes per message
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
4. **Multi-hop Anonymity**: TSP does not provide strong anonymity equivalent to Tor/I2P (use those transports for data-plane traffic requiring strong anonymity)
5. **Quantum Cryptanalysis**: Uses same cryptographic primitives as base Tox protocol
6. **Announce Node Compromise**: In 1-hop discovery mode, the relay node learns both the announcer's pseudonym and IP; in 3-hop mode this is mitigated to the same degree as c-toxcore onion routing

### Attack Scenarios

**Attack 1: Timing Correlation**
- *Scenario*: Adversary monitors sender→proxy timing and proxy→recipient timing
- *Mitigation*: Random delays (50-200ms) injected by proxy before forwarding
- *Limitation*: Sophisticated timing analysis may still succeed with large datasets

**Attack 2: Malicious Proxy Logging**
- *Scenario*: Compromised proxy logs all sender→recipient mappings
- *Mitigation*: Proxy reputation system, proxy rotation, ephemeral sessions
- *Limitation*: Cannot prevent logging, only detect unreliable proxies post-facto

**Attack 3: DHT Eclipse Attack**
- *Scenario*: Adversary floods DHT with malicious proxy announcements
- *Mitigation*: Proxy reputation validation, signature verification, diversity requirements
- *Limitation*: New users with no reputation data remain vulnerable

### Privacy Guarantees

**Formal Privacy Claims:**

1. **IP Address Unlinkability**: Given TSP 1-hop message M from sender S to recipient R via proxy P, recipient R cannot determine S's IP address with probability > 1/|P| where |P| is the number of possible proxy nodes.

2. **Partial Observer Resistance**: An adversary observing only the sender→proxy communication OR only the proxy→recipient communication cannot link the sender to the recipient with probability better than random guessing.

3. **Authentication Preservation**: All TSP messages maintain the same cryptographic authenticity guarantees as standard Tox messages through Ed25519 signature preservation.

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
    "time"
    "errors"
)

// Core TSP client structure
type Client struct {
    privateKey   [32]byte
    publicKey    [32]byte
    proxyCache   map[[32]byte]*ProxyInfo
    sessions     map[[8]byte]*TSPSession
    config       TSPConfig
}

// Select optimal proxy node for message delivery
func (c *Client) selectProxyNode(recipient [32]byte) (*ProxyInfo, error) {
    // Get available proxies from DHT
    proxies := c.getDHTProxies()
    if len(proxies) == 0 {
        return nil, errors.New("no proxies available")
    }
    
    // Filter by trust score and capabilities
    candidates := make([]*ProxyInfo, 0)
    for _, proxy := range proxies {
        if proxy.TrustScore >= c.config.ProxyThreshold &&
           proxy.hasCapability(TSP_CAP_FORWARD_MESSAGES) {
            candidates = append(candidates, proxy)
        }
    }
    
    if len(candidates) == 0 {
        return nil, errors.New("no trusted proxies available")
    }
    
    // Select proxy with highest trust score and lowest latency
    bestProxy := candidates[0]
    bestScore := c.calculateProxyScore(bestProxy)
    
    for _, proxy := range candidates[1:] {
        score := c.calculateProxyScore(proxy)
        if score > bestScore {
            bestProxy = proxy
            bestScore = score
        }
    }
    
    return bestProxy, nil
}

func (c *Client) calculateProxyScore(proxy *ProxyInfo) float64 {
    trustWeight := 0.7
    latencyWeight := 0.3
    
    latencyScore := 1.0 - (float64(proxy.AvgLatency)/float64(time.Second))
    if latencyScore < 0 {
        latencyScore = 0
    }
    
    return proxy.TrustScore*trustWeight + latencyScore*latencyWeight
}

// Encapsulate message for proxy forwarding
func (c *Client) encapsulateForProxy(proxy *ProxyInfo, recipient [32]byte, message []byte) ([]byte, error) {
    // Generate session ID
    sessionID := make([]byte, 8)
    rand.Read(sessionID)
    
    // Create inner message (encrypted for recipient)
    innerMsg, err := c.createInnerMessage(recipient, message)
    if err != nil {
        return nil, err
    }
    
    // Create proxy payload
    proxyPayload := ProxyPayload{
        TargetPubKey:  recipient,
        Instructions:  TSP_FORWARD_MESSAGE,
        InnerMsgLen:   uint16(len(innerMsg)),
        InnerMessage:  innerMsg,
        Padding:      c.generatePadding(innerMsg),
    }
    
    // Encrypt payload for proxy
    encryptedPayload, err := c.encryptForProxy(proxy.PublicKey, proxyPayload)
    if err != nil {
        return nil, err
    }
    
    // Construct TSP packet
    tspPacket := TSPPacket{
        Type:         0xF1,
        Version:      0x02,
        RoutingMode:  0x01, // 1-hop
        SessionID:    sessionID,
        PayloadLen:   uint16(len(encryptedPayload)),
        Reserved:     0x0000,
        Payload:      encryptedPayload,
    }
    
    return tspPacket.marshal(), nil
}

// Verify authenticity of proxied message at recipient
func (c *Client) verifyProxiedMessage(packet []byte) (*VerifiedMessage, error) {
    // Parse TSP packet
    tsp, err := unmarshalTSPPacket(packet)
    if err != nil {
        return nil, err
    }
    
    // Decrypt payload (recipient is target)
    decrypted, err := c.decryptPayload(tsp.Payload)
    if err != nil {
        return nil, err
    }
    
    // Extract inner message
    innerMsg, err := extractInnerMessage(decrypted)
    if err != nil {
        return nil, err
    }
    
    // Verify Ed25519 signature
    msgHash := c.hashMessage(innerMsg.Data, innerMsg.Timestamp)
    if !c.verifySignature(innerMsg.SenderPubKey, msgHash, innerMsg.Signature) {
        return nil, errors.New("invalid message signature")
    }
    
    // Check if sender is trusted friend
    if !c.isTrustedFriend(innerMsg.SenderPubKey) {
        return nil, errors.New("message from unknown sender")
    }
    
    return &VerifiedMessage{
        Sender:    innerMsg.SenderPubKey,
        Data:      innerMsg.Data,
        Timestamp: innerMsg.Timestamp,
        ViaProxy:  true,
    }, nil
}

// Generate appropriately-sized padding for traffic analysis resistance
func (c *Client) generatePadding(innerMsg []byte) []byte {
    // Target sizes: 512, 1024, or 1360 bytes
    currentSize := len(innerMsg) + 35 // Account for header overhead
    
    var targetSize int
    if currentSize <= 512 {
        targetSize = 512
    } else if currentSize <= 1024 {
        targetSize = 1024
    } else {
        targetSize = 1360
    }
    
    paddingSize := targetSize - currentSize
    if paddingSize <= 0 {
        return nil
    }
    
    padding := make([]byte, paddingSize)
    rand.Read(padding)
    return padding
}

// Main message sending interface with automatic fallback
func (c *Client) SendMessage(recipient [32]byte, message []byte) error {
    mode := c.selectMode(recipient, c.config)
    
    switch mode {
    case TSP_DIRECT_ONLY:
        return c.sendDirectMessage(recipient, message)
        
    case TSP_PROXY_ONLY:
        proxy, err := c.selectProxyNode(recipient)
        if err != nil {
            // Fallback to direct if configured
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

This implementation provides a complete foundation for TSP/2.0 messaging with clear separation between 0-hop and 1-hop modes, comprehensive security measures, and practical fallback mechanisms for real-world deployment.

## 8. Indirect Friend Discovery (TSP/2.0)

### 8.1 Motivation

c-toxcore's Onion module provides anonymous friend discovery through a 3-hop onion-routed announce/find flow within the DHT. This allows users to publish their presence and locate friends without exposing their IP addresses to arbitrary DHT participants. toxcore-go's TSP/1.0 addressed only live message relay and had no equivalent for friend discovery — users' IP addresses were exposed during DHT lookups.

TSP/2.0 closes this gap by adding **indirect friend discovery** that reuses the existing TSP proxy infrastructure. The design is wire-compatible with the c-toxcore onion announce packet types (`PacketOnionAnnounceRequest`, `PacketOnionAnnounceResponse`) so that toxcore-go nodes can participate in the same announce namespace as c-toxcore nodes.

**Key design decisions:**

1. **1-hop by default**: Most discovery operations use a single TSP relay, matching the performance profile of the rest of TSP. This is sufficient for the common threat model (hiding IP from the friend and casual observers).

2. **3-hop when required**: For interoperability with c-toxcore's 3-hop onion announce protocol, or when the user explicitly requests enhanced path diversity, TSP supports building 3-node relay chains using the same layered-encryption scheme as c-toxcore.

3. **Always use Tox-native discovery**: Even when Tor or I2P transports are active for data-plane traffic, friend discovery is always performed through the Tox DHT via TSP relays. This avoids leaking friend-lookup patterns to Tor exit nodes or I2P outproxies and ensures consistent behavior across all transport configurations.

### 8.2 Discovery Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                    Friend Discovery Flow                         │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Phase 1: ANNOUNCE (Alice publishes her presence)                │
│                                                                  │
│  1-Hop Mode (default):                                           │
│    Alice → Relay R → Announce Nodes (closest to H(Alice_PK))     │
│                                                                  │
│  3-Hop Mode (c-toxcore compatible):                              │
│    Alice → R1 → R2 → R3 → Announce Nodes                        │
│                                                                  │
│  Phase 2: LOOKUP (Bob searches for Alice)                        │
│                                                                  │
│  1-Hop Mode (default):                                           │
│    Bob → Relay R → Announce Nodes → R → Bob                      │
│                                                                  │
│  3-Hop Mode (c-toxcore compatible):                              │
│    Bob → R1 → R2 → R3 → Announce Nodes → R3 → R2 → R1 → Bob    │
│                                                                  │
│  Phase 3: CONNECT (direct or proxied connection)                 │
│                                                                  │
│    Bob uses discovered DHT node info to connect to Alice         │
│    via standard TSP messaging (0-hop or 1-hop per §3)            │
│    If Tor/I2P is active, data-plane connection uses that         │
│    transport; discovery always uses Tox DHT + TSP                │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### 8.3 Announce Protocol

#### Announce Key Derivation

The announce namespace uses the same key-distance scheme as c-toxcore: the client hashes its long-term public key with a secret salt to produce an **announce location** that changes periodically:

```go
// AnnounceLocation is the DHT key under which the client publishes its presence.
// It rotates every AnnounceEpoch to limit long-term tracking.
type AnnounceLocation struct {
    LocationKey  [32]byte      // H(LongTermPK || AnnounceSalt || Epoch)
    Epoch        uint64        // Current announce epoch
    AnnounceSalt [32]byte      // Random per-instance salt (persisted across restarts)
}

const (
    AnnounceEpoch        = 5 * time.Minute  // Announce location rotation interval
    AnnounceRefresh      = 2 * time.Minute  // Re-announce interval within epoch
    AnnounceMaxAge       = 10 * time.Minute // Storage nodes discard entries older than this
    AnnounceMaxPerNode   = 32               // Max announce entries per storage node
)

// deriveAnnounceLocation produces the DHT key for the current epoch.
func deriveAnnounceLocation(publicKey [32]byte, salt [32]byte, epoch uint64) [32]byte {
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
    LocationKey   [32]byte      // H(PK||salt||epoch)
    EncryptedData []byte        // Encrypted announce data (opaque to storage node)
    StoredAt      time.Time     // When entry was stored
    PingID        [8]byte       // For verifying lookup responses
    ReturnPath    []byte        // Encrypted return path for responses
}

type AnnounceStorage struct {
    entries      map[[32]byte]*AnnounceEntry // LocationKey → entry
    maxEntries   int                          // AnnounceMaxPerNode (32)
    mu           sync.RWMutex
}

// StoreAnnounce stores or updates an announce entry.
// Rejects entries if storage is full and new entry is farther than all existing ones.
func (as *AnnounceStorage) StoreAnnounce(entry *AnnounceEntry) error {
    as.mu.Lock()
    defer as.mu.Unlock()
    
    // Purge expired entries
    as.purgeExpired()
    
    // Accept if space available or closer than the farthest existing entry
    if len(as.entries) < as.maxEntries {
        as.entries[entry.LocationKey] = entry
        return nil
    }
    
    // Replace farthest entry if new entry is closer to our node
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

When an announce storage node holds an entry matching the `SearchLocationKey`, it returns the encrypted announce data to the searcher via the return path:

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

For a searcher (Bob) to derive the correct `AnnounceLocation` for a target (Alice), Bob must know Alice's announce salt. This salt is exchanged over an already-authenticated channel:

```go
// AnnounceSaltExchange is sent to friends when both are online.
// The salt is encrypted with the existing friend-to-friend shared secret.
type AnnounceSaltExchange struct {
    Type          string    `json:"type"`           // "announce_salt_exchange"
    SenderPK      [32]byte  `json:"sender_pk"`
    AnnounceSalt  [32]byte  `json:"announce_salt"`  // Encrypted for recipient
    ValidFrom     time.Time `json:"valid_from"`     // Epoch validity start
    ValidUntil    time.Time `json:"valid_until"`    // Salt rotation (24 hours)
    Signature     [64]byte  `json:"signature"`      // Ed25519 signature
}

const (
    SaltRotationInterval = 24 * time.Hour   // Rotate announce salt daily
    SaltExchangeRetry    = 5 * time.Minute  // Retry if friend was offline
)
```

When Alice and Bob are both online, they exchange announce salts over their encrypted friend channel. Each party stores the friend's salt locally. If a friend was offline during salt rotation, the exchange is retried when the friend next comes online. Old salts are kept for one additional rotation period to handle clock skew and late lookups.

### 8.6 1-Hop Discovery Flow (Default)

The 1-hop discovery flow uses a single TSP relay for both announce and lookup operations. This is the default and preferred mode.

```
1-Hop Announce:
  Alice → Relay → AnnounceNode(closest to H(Alice_PK||salt||epoch))
    - Alice encrypts announce data for AnnounceNode
    - Outer envelope encrypted for Relay (standard TSP 1-hop)
    - Relay decrypts outer layer, forwards to AnnounceNode
    - AnnounceNode stores the entry

1-Hop Lookup:
  Bob → Relay → AnnounceNode → Relay → Bob
    - Bob sends lookup request through Relay
    - AnnounceNode returns encrypted announce data via Relay
    - Bob decrypts using knowledge of Alice's PK
    - Bob now has Alice's DHT node addresses

Privacy properties (1-hop discovery):
  - Relay knows Bob's IP but not what he is searching for
    (search key is a salted hash, not Alice's raw PK)
  - AnnounceNode knows the search key but not Bob's IP
    (sees only Relay's IP)
  - Neither party alone can link Bob to Alice
```

### 8.7 3-Hop Discovery Flow (c-toxcore Compatible)

The 3-hop discovery flow constructs a relay chain of exactly 3 TSP-capable nodes, matching c-toxcore's onion path length. Each relay decrypts one layer and forwards the remainder. This mode is used when:

1. The user explicitly sets `TSP_DISCOVERY_3HOP`
2. The client detects that announce nodes in the target's neighborhood only accept c-toxcore onion-formatted packets (backward compatibility)
3. The `TSP_DISCOVERY_AUTO` mode escalates after a 1-hop attempt fails

```
3-Hop Announce:
  Alice → R1 → R2 → R3 → AnnounceNode
    Layer 3 (outermost): Encrypted for R1, contains R2's address + layer 2
    Layer 2: Encrypted for R2, contains R3's address + layer 1
    Layer 1 (innermost): Encrypted for R3, contains AnnounceNode address + announce data

3-Hop Lookup:
  Bob → R1 → R2 → R3 → AnnounceNode → R3 → R2 → R1 → Bob
    Forward path: same layered encryption as announce
    Return path: each relay stores a symmetric return-path key
      R3 encrypts response for R2, R2 encrypts for R1, R1 encrypts for Bob

Privacy properties (3-hop discovery):
  - R1 knows Bob's IP but only R2's identity (not R3 or AnnounceNode)
  - R2 knows R1 and R3 but not Bob or AnnounceNode
  - R3 knows R2 and AnnounceNode but not Bob or R1
  - AnnounceNode knows search key and R3's IP, but not Bob
  - Same privacy level as c-toxcore onion routing
```

#### 3-Hop Relay Chain Construction

```go
// buildRelayChain selects 3 relays from the DHT for a 3-hop path.
// Relays are chosen from different /16 subnets to prevent colocation attacks.
func (c *TSPClient) buildRelayChain(target [32]byte) ([3]*RelayNode, error) {
    candidates := c.getAvailableRelays(TSP_CAP_TRIPLE_RELAY)
    if len(candidates) < 3 {
        return [3]*RelayNode{}, errors.New("insufficient relay nodes for 3-hop chain")
    }
    
    // Select 3 relays from diverse network locations
    chain := [3]*RelayNode{}
    usedSubnets := make(map[string]bool)
    
    for i := 0; i < 3; i++ {
        relay, err := selectDiverseRelay(candidates, usedSubnets)
        if err != nil {
            return chain, fmt.Errorf("relay chain construction failed at hop %d: %w", i, err)
        }
        chain[i] = relay
        usedSubnets[relay.Subnet()] = true
    }
    
    return chain, nil
}

// createOnionLayers wraps a payload in 3 encryption layers (outermost first).
// Compatible with c-toxcore's onion packet format for announce operations.
func (c *TSPClient) createOnionLayers(chain [3]*RelayNode, innerPayload []byte) ([]byte, error) {
    // Layer 1 (innermost): encrypt for R3
    layer1, err := c.encryptForRelay(chain[2], innerPayload)
    if err != nil {
        return nil, err
    }
    
    // Layer 2: encrypt for R2, containing R3's address + layer 1
    r3Addr := chain[2].AddressBytes()
    layer2Payload := make([]byte, len(r3Addr)+len(layer1))
    copy(layer2Payload, r3Addr)
    copy(layer2Payload[len(r3Addr):], layer1)
    layer2, err := c.encryptForRelay(chain[1], layer2Payload)
    if err != nil {
        return nil, err
    }
    
    // Layer 3 (outermost): encrypt for R1, containing R2's address + layer 2
    r2Addr := chain[1].AddressBytes()
    layer3Payload := make([]byte, len(r2Addr)+len(layer2))
    copy(layer3Payload, r2Addr)
    copy(layer3Payload[len(r2Addr):], layer2)
    layer3, err := c.encryptForRelay(chain[0], layer3Payload)
    if err != nil {
        return nil, err
    }
    
    return layer3, nil
}
```

### 8.8 c-toxcore Onion Compatibility

TSP/2.0's discovery protocol is designed to interoperate with c-toxcore's onion announce system:

#### Packet Type Mapping

| c-toxcore Packet Type | TSP/2.0 Equivalent | Notes |
|---|---|---|
| `OnionAnnounceRequest` | TSP Routing Mode `0x03` (Announce) | Same announce key derivation; TSP relay wrapping is stripped before reaching announce nodes |
| `OnionAnnounceResponse` | Response via TSP return path | Announce nodes see identical request format |
| `OnionDataRequest` | TSP Routing Mode `0x04` (Lookup) | Search key compatible |
| `OnionDataResponse` | Response via TSP return path | Same encrypted blob format |

#### Interoperability Modes

1. **TSP-native mode** (default): Both announcer and searcher are toxcore-go clients. Uses TSP routing modes `0x03`/`0x04` with 1-hop or 3-hop relay chains. The announce payload at the announce node is format-compatible with c-toxcore.

2. **c-toxcore compatibility mode**: When a toxcore-go client needs to reach announce nodes that only understand c-toxcore onion packets:
   - The client builds a 3-hop chain
   - The innermost relay (R3) unwraps the TSP envelope and re-emits the payload as a standard `PacketOnionAnnounceRequest` toward the announce node
   - Responses follow the reverse path

3. **Mixed network**: toxcore-go announce entries and c-toxcore announce entries coexist at the same announce nodes, indexed by the same key-distance metric.

```go
// Packet type constants for interoperability with c-toxcore onion protocol.
// These match the values already defined in transport/packet.go.
const (
    PacketOnionAnnounceRequest  = transport.PacketOnionAnnounceRequest  // 0x0E
    PacketOnionAnnounceResponse = transport.PacketOnionAnnounceResponse // 0x0F
    PacketOnionDataRequest      = transport.PacketOnionDataRequest      // 0x10
    PacketOnionDataResponse     = transport.PacketOnionDataResponse     // 0x11
)
```

### 8.9 Discovery When Tor/I2P Is Active

**Requirement**: Indirect friend discovery MUST always use the Tox DHT via TSP relay infrastructure, regardless of whether Tor, I2P, or another privacy transport is configured for data-plane traffic.

**Rationale**:

1. **Metadata isolation**: Tor exit nodes and I2P outproxies should not observe DHT announce/lookup traffic, which would reveal the user's interest in specific Tox IDs.

2. **Consistent discovery**: Using the same Tox DHT announce namespace regardless of transport ensures that a user is discoverable by all friends, whether those friends use clearnet, Tor, or I2P.

3. **Separation of concerns**: The Tox DHT is the canonical friend discovery layer. Tor/I2P provide strong data-plane anonymity for the connection established *after* discovery. TSP provides lightweight discovery-plane IP protection.

**Implementation**:

```go
// DiscoveryTransportPolicy enforces that discovery always uses Tox DHT + TSP.
type DiscoveryTransportPolicy struct {
    // DataPlaneTransport is the user's configured transport (clearnet, Tor, I2P).
    DataPlaneTransport transport.Transport
    
    // DiscoveryTransport is always the Tox UDP/TCP transport with TSP overlay.
    // This is initialized even when the data plane uses Tor/I2P.
    DiscoveryTransport *TSPTransport
}

func (p *DiscoveryTransportPolicy) Announce(location AnnounceLocation, data []byte) error {
    // Always use TSP relay, never Tor/I2P for announce
    return p.DiscoveryTransport.SendAnnounce(location, data)
}

func (p *DiscoveryTransportPolicy) Lookup(location [32]byte) (*AnnounceEntry, error) {
    // Always use TSP relay, never Tor/I2P for lookup
    return p.DiscoveryTransport.SendLookup(location)
}

func (p *DiscoveryTransportPolicy) Connect(peerAddr net.Addr) (net.Conn, error) {
    // Data-plane connection uses the user's configured transport
    return p.DataPlaneTransport.Dial(peerAddr)
}
```

**Example flow when Tor is active**:

```
1. Alice announces via Tox DHT + TSP (1-hop relay, clearnet UDP)
2. Bob discovers Alice via Tox DHT + TSP (1-hop relay, clearnet UDP)
3. Bob connects to Alice's discovered address via Tor transport
4. All subsequent messages flow over Tor
```

The clearnet UDP used for steps 1-2 is protected by TSP's relay (Bob's IP hidden from announce nodes). The Tor transport in steps 3-4 provides strong anonymity for the data-plane connection.

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

In 1-hop mode, the relay node can see both the client's IP and the announce/search key. This is the same tradeoff as 1-hop message relay (§6): the relay can log metadata but cannot read encrypted announce data. This is acceptable for the common threat model because:

1. The relay is selected by the client and can be rotated
2. The announce key is a salted hash, not the raw public key — the relay cannot determine which Tox ID is being announced/searched without the salt
3. Users requiring stronger protection can enable 3-hop mode or use Tor/I2P for the data plane

#### 3-Hop Discovery: c-toxcore Equivalent

In 3-hop mode, no single relay learns both the client's IP and the announce/search key, matching c-toxcore's onion routing privacy properties. The same known limitations apply:

1. **Timing correlation**: An adversary observing all 3 relays can correlate timing
2. **Sybil attacks**: An adversary controlling multiple relays in the chain can deanonymize
3. **Announce node enumeration**: An adversary close to a target's announce key can enumerate who is announced there

#### CVE-2018-25022 Mitigation

The IP disclosure vulnerability in c-toxcore's onion routing (CVE-2018-25022) is not applicable to TSP discovery because:

1. TSP relays do not relay NAT ping requests — only announce/lookup payloads
2. The relay's forwarding logic is strictly typed: only `0x03` (announce) and `0x04` (lookup) routing modes are processed; all other packet types are rejected
3. Announce nodes cannot send arbitrary packets back through the return path — only announce responses matching a pending lookup

### 8.11 Discovery Configuration

```go
type TSPDiscoveryConfig struct {
    Mode              TSPDiscoveryMode // AUTO, 1HOP, or 3HOP
    AnnounceInterval  time.Duration    // How often to re-announce (default: 2 min)
    LookupTimeout     time.Duration    // Max time for a lookup (default: 10 sec)
    SaltRotation      time.Duration    // How often to rotate announce salt (default: 24h)
    MaxAnnounceNodes  int              // Max nodes to announce to (default: 8)
    FallbackTo3Hop    bool             // Auto-escalate to 3-hop on 1-hop failure (default: true)
    AlwaysUseToxDHT   bool             // Force Tox DHT for discovery even with Tor/I2P (default: true, MUST be true)
}

// DefaultDiscoveryConfig returns the recommended discovery configuration.
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

TSP/2.0 is backward compatible with TSP/1.0, but compatibility is defined per packet version as well as per routing mode:

- TSP/1.0 clients ignore routing modes `0x02`–`0x04` (treated as RESERVED → `TSP_ERROR_UNSUPPORTED`)
- For interoperability with a TSP/1.0 peer or relay, a TSP/2.0 sender MUST encode legacy routing modes `0x00` and `0x01` with version byte `0x01`
- Version byte `0x02` is reserved for TSP/2.0-only semantics, including discovery-capable packets and routing modes `0x03` and `0x04`
- A TSP/2.0 receiver SHOULD accept both version `0x01` and version `0x02`; when version `0x01` is received, it MUST interpret the packet using TSP/1.0 semantics only
- A TSP/1.0 receiver is not required to ignore unknown versions and may reject version `0x02`; therefore a TSP/2.0 sender MUST NOT send version `0x02` to a peer unless TSP/2.0 support has been established
- Discovery features are only used between TSP/2.0-capable nodes
- The version byte allows receivers to detect capability level after packet parsing, while proxy capability flags are the preferred way to advertise support before sending TSP/2.0-only traffic
- Proxy announcements with `TSP_CAP_DISCOVERY_ANNOUNCE` or `TSP_CAP_DISCOVERY_LOOKUP` flags indicate TSP/2.0 support