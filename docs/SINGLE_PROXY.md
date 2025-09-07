# Tox Single-Hop Proxy Extension (TSP/1.0)

## 1. Extension Summary

The Tox Single-Hop Proxy Extension (TSP/1.0) provides lightweight network metadata protection through optional single-proxy routing while maintaining full cryptographic identity preservation. This extension deliberately limits complexity to exactly two routing modes: direct connection (0-hop) and single proxy relay (1-hop), explicitly rejecting multi-hop requests to avoid competing with dedicated anonymity networks like Tor and I2P.

The design philosophy prioritizes simplicity and performance over strong anonymity. TSP provides protection against casual metadata collection by message recipients and partial network observers, but does not defend against global surveillance or sophisticated traffic analysis. The extension integrates seamlessly with existing Tox cryptography - all Ed25519 signatures and friend verification remain unchanged, ensuring recipients can always verify sender authenticity.

Key constraints: TSP never routes through multiple proxies, maintains backward compatibility with standard Tox clients, and provides graceful fallback when proxies are unavailable. The extension adds minimal overhead (single proxy lookup, one additional network hop) while solving the common use case of hiding sender IP addresses from message recipients.

## 2. Message Format Specification

### TSP Message Header

```
TSP_MESSAGE Packet Structure (UDP payload):
┌─────────────────────────────────────────────────────────────┐
│ Packet Type (1 byte): 0xF1 (TSP_PROXIED_MESSAGE)          │
├─────────────────────────────────────────────────────────────┤
│ Version (1 byte): 0x01 (TSP/1.0)                          │
├─────────────────────────────────────────────────────────────┤
│ Routing Mode (1 byte):                                     │
│   0x00 = Direct (0-hop)                                    │
│   0x01 = Single Proxy (1-hop)                             │
│   0x02-0xFF = RESERVED (reject with TSP_ERROR_UNSUPPORTED) │
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
    Version       uint8         // TSP protocol version (0x01)
    Signature     [64]byte      // Self-signed announcement
}

// Capability flags
const (
    TSP_CAP_FORWARD_MESSAGES = 1 << 0  // Basic message forwarding
    TSP_CAP_DELIVERY_ACK    = 1 << 1   // Delivery confirmation
    TSP_CAP_RESPONSE_CACHE  = 1 << 2   // Brief response caching
    TSP_CAP_PING_RELAY     = 1 << 3   // Connectivity testing
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
}

type TSPMode int
const (
    TSP_AUTO        TSPMode = iota // Adaptive selection
    TSP_DIRECT_ONLY                // Force 0-hop mode
    TSP_PROXY_ONLY                 // Force 1-hop mode
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
4. **Multi-hop Anonymity**: TSP explicitly does not provide strong anonymity (use Tor/I2P instead)
5. **Quantum Cryptanalysis**: Uses same cryptographic primitives as base Tox protocol

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
        Version:      0x01,
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

This implementation provides a complete foundation for TSP/1.0 with clear separation between 0-hop and 1-hop modes, comprehensive security measures, and practical fallback mechanisms for real-world deployment.