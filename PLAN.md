# Tox Single-Hop Proxy Extension (TSP/1.0) Implementation Plan

## Overview

This plan outlines the implementation of the Tox Single-Hop Proxy Extension (TSP/1.0) with minimal new code by leveraging the existing toxcore-go infrastructure. The TSP extension provides lightweight network metadata protection through optional single-proxy routing while maintaining full cryptographic identity preservation.

## Design Philosophy

- **Minimal Code Footprint**: Reuse existing transport, crypto, and DHT infrastructure
- **Backward Compatibility**: Seamless fallback to direct Tox connections
- **Security-First**: Preserve all existing cryptographic guarantees
- **Performance-Aware**: Minimal overhead through efficient proxy selection

## Existing Infrastructure Leveraged

### 1. Transport Layer (`transport/`)
- **UDP/TCP Transport**: Reuse `UDPTransport` and `TCPTransport` implementations
- **Packet System**: Extend existing `PacketType` enum with TSP packet types
- **Noise Protocol**: Leverage existing `NoiseTransport` for proxy communications
- **Handler Registration**: Use existing `RegisterHandler` pattern for TSP packets

### 2. Cryptographic Operations (`crypto/`)
- **ChaCha20-Poly1305**: Reuse existing `Encrypt`/`Decrypt` functions for payload encryption
- **Ed25519 Signatures**: Leverage `Sign`/`Verify` for proxy announcements and message authentication
- **Curve25519 ECDH**: Use `DeriveSharedSecret` for proxy session keys
- **Secure Memory**: Apply existing `secure_memory.go` patterns for key management

### 3. DHT Infrastructure (`dht/`)
- **Packet Handling**: Extend `HandlePacket` in `handler.go` for proxy announcements
- **Bootstrap Manager**: Reuse node discovery patterns for proxy discovery
- **Routing Table**: Leverage existing structures for proxy reputation storage

### 4. Async Messaging (`async/`)
- **Obfuscation Manager**: Adapt identity obfuscation patterns for proxy routing
- **Forward Secrecy**: Preserve existing forward secrecy guarantees
- **Session Management**: Apply session patterns from `AsyncClient`

## Implementation Structure

```
tsp/                           # New TSP module (minimal new code)
├── client.go                  # TSP client with mode selection
├── proxy.go                   # Proxy node functionality  
├── session.go                 # Session management
├── reputation.go              # Proxy trust and reputation
├── packet_types.go            # TSP packet type constants
├── fallback.go                # Graceful fallback logic
└── tsp_test.go               # Comprehensive test suite

transport/                     # Extensions to existing code
├── packet.go                  # Add TSP packet types (5 lines)
└── noise_transport.go         # TSP packet handling (20 lines)

dht/                          # Extensions to existing code
├── handler.go                # TSP packet handlers (30 lines)
└── bootstrap.go              # Proxy discovery integration (15 lines)

examples/
└── tsp_demo/                 # TSP demonstration
    └── main.go               # Example usage
```

## Phase 1: Core TSP Infrastructure (Week 1)

### 1.1 Packet Type Extensions (5 lines of code)

**File**: `transport/packet.go`

```go
// Add to existing PacketType constants
const (
    // ... existing packet types ...
    
    // TSP packet types
    PacketTSPProxiedMessage PacketType = 0xF1  // TSP proxied message
    PacketTSPProxyAnnouncement PacketType = 0xF2  // Proxy capability announcement
)
```

### 1.2 TSP Packet Structures (50 lines of code)

**File**: `tsp/packet_types.go`

```go
package tsp

import "time"

// TSP packet header structure
type TSPHeader struct {
    Type        uint8    // 0xF1 for proxied messages
    Version     uint8    // 0x01 for TSP/1.0
    RoutingMode uint8    // 0x00=direct, 0x01=single-proxy
    SessionID   [8]byte  // Random session identifier
    PayloadLen  uint16   // Length of encrypted payload
    Reserved    uint16   // Must be 0x0000
}

// Proxy routing modes
const (
    TSPModeDirect     = 0x00
    TSPModeSingleHop  = 0x01
)

// Proxy capability announcement
type ProxyAnnouncement struct {
    ProxyPublicKey [32]byte  // Ed25519 proxy identity
    Capabilities   uint16    // Supported features bitfield
    RateLimit      uint32    // Messages per minute capacity
    Uptime         uint32    // Seconds of operation
    Version        uint8     // TSP protocol version
    Signature      [64]byte  // Self-signed announcement
}

// Proxy capabilities
const (
    TSPCapForwardMessages = 1 << 0  // Basic message forwarding
    TSPCapDeliveryAck    = 1 << 1   // Delivery confirmation
    TSPCapResponseCache  = 1 << 2   // Brief response caching
    TSPCapPingRelay      = 1 << 3   // Connectivity testing
)
```

### 1.3 TSP Session Management (80 lines of code)

**File**: `tsp/session.go`

```go
package tsp

import (
    "sync"
    "time"
)

// Session management for TSP connections
type SessionManager struct {
    sessions    map[[8]byte]*TSPSession
    mutex       sync.RWMutex
    maxDuration time.Duration
    maxMessages uint32
}

type TSPSession struct {
    SessionID    [8]byte
    ProxyPubKey  [32]byte
    CreatedAt    time.Time
    ExpiresAt    time.Time
    MessageCount uint32
    LastActivity time.Time
}

const (
    MaxSessionDuration = 10 * time.Minute
    MaxMessagesPerSession = 100
)

func NewSessionManager() *SessionManager {
    return &SessionManager{
        sessions:    make(map[[8]byte]*TSPSession),
        maxDuration: MaxSessionDuration,
        maxMessages: MaxMessagesPerSession,
    }
}

func (sm *SessionManager) CreateSession(proxyPK [32]byte) *TSPSession {
    // Generate random session ID (reuse crypto.GenerateNonce pattern)
    // Create and store session
    // Return session
}

func (sm *SessionManager) GetSession(sessionID [8]byte) *TSPSession {
    // Return session if valid and not expired
}

func (sm *SessionManager) CleanupExpiredSessions() {
    // Remove expired sessions (called periodically)
}
```

## Phase 2: Proxy Discovery and Reputation (Week 1)

### 2.1 DHT Integration (30 lines of code)

**File**: `dht/handler.go` (extend existing `HandlePacket`)

```go
// Add to existing HandlePacket switch statement
case transport.PacketTSPProxyAnnouncement:
    return bm.handleTSPProxyAnnouncement(packet, senderAddr)

// New handler method
func (bm *BootstrapManager) handleTSPProxyAnnouncement(packet *transport.Packet, senderAddr net.Addr) error {
    // Parse proxy announcement
    // Validate signature using existing crypto.Verify
    // Update proxy reputation
    // Store in routing table with proxy capability flag
    return nil
}
```

### 2.2 Proxy Reputation System (100 lines of code)

**File**: `tsp/reputation.go`

```go
package tsp

import (
    "math"
    "time"
    "sync"
)

type ProxyReputation struct {
    SuccessfulForwards uint32
    FailedForwards     uint32
    AvgLatency        time.Duration
    LastSeen          time.Time
    TrustScore        float64
}

type ReputationManager struct {
    proxies map[[32]byte]*ProxyReputation
    mutex   sync.RWMutex
}

func NewReputationManager() *ReputationManager {
    return &ReputationManager{
        proxies: make(map[[32]byte]*ProxyReputation),
    }
}

func (rm *ReputationManager) UpdateProxySuccess(proxyPK [32]byte, latency time.Duration) {
    // Update successful forward count and average latency
    // Recalculate trust score
}

func (rm *ReputationManager) UpdateProxyFailure(proxyPK [32]byte) {
    // Update failed forward count
    // Recalculate trust score
}

func (rm *ReputationManager) CalculateTrustScore(rep *ProxyReputation) float64 {
    // Use existing algorithm from spec
    total := rep.SuccessfulForwards + rep.FailedForwards
    if total < 10 {
        return 0.5 // Neutral for new proxies
    }
    
    successRate := float64(rep.SuccessfulForwards) / float64(total)
    latencyPenalty := math.Min(float64(rep.AvgLatency)/float64(time.Second), 1.0)
    uptimeFactor := math.Min(float64(time.Since(rep.LastSeen))/float64(time.Hour), 1.0)
    
    return successRate * (1.0 - latencyPenalty*0.2) * (1.0 - uptimeFactor*0.3)
}

func (rm *ReputationManager) GetTrustedProxies(minTrustScore float64) []ProxyInfo {
    // Return list of proxies above trust threshold
}
```

## Phase 3: TSP Client Implementation (Week 2)

### 3.1 Core TSP Client (200 lines of code)

**File**: `tsp/client.go`

```go
package tsp

import (
    "crypto/rand"
    "errors"
    "fmt"
    "net"
    "time"
    
    "github.com/opd-ai/toxcore/crypto"
    "github.com/opd-ai/toxcore/transport"
)

type TSPClient struct {
    privateKey      [32]byte
    publicKey       [32]byte
    transport       transport.Transport
    sessionManager  *SessionManager
    reputationMgr   *ReputationManager
    config          TSPConfig
}

type TSPConfig struct {
    DefaultMode     TSPMode
    ProxyThreshold  float64       // Min proxy trust score (0.7)
    FallbackTimeout time.Duration // 5 seconds
    MaxRetries      int          // 3 attempts
    AllowFallback   bool         // Allow fallback to direct
}

type TSPMode int
const (
    TSPAuto       TSPMode = iota // Adaptive selection
    TSPDirectOnly               // Force 0-hop mode
    TSPProxyOnly                // Force 1-hop mode
)

func NewTSPClient(keyPair *crypto.KeyPair, transport transport.Transport) *TSPClient {
    return &TSPClient{
        privateKey:     keyPair.Private,
        publicKey:      keyPair.Public,
        transport:      transport,
        sessionManager: NewSessionManager(),
        reputationMgr:  NewReputationManager(),
        config: TSPConfig{
            DefaultMode:     TSPAuto,
            ProxyThreshold:  0.7,
            FallbackTimeout: 5 * time.Second,
            MaxRetries:      3,
            AllowFallback:   true,
        },
    }
}

func (c *TSPClient) SendMessage(recipient [32]byte, message []byte) error {
    mode := c.selectMode(recipient)
    
    switch mode {
    case TSPDirectOnly:
        return c.sendDirectMessage(recipient, message)
    case TSPProxyOnly:
        return c.sendViaProxy(recipient, message)
    default:
        return errors.New("invalid TSP mode")
    }
}

func (c *TSPClient) selectMode(recipient [32]byte) TSPMode {
    switch c.config.DefaultMode {
    case TSPDirectOnly:
        return TSPDirectOnly
    case TSPProxyOnly:
        return TSPProxyOnly
    case TSPAuto:
        return c.adaptiveSelection(recipient)
    }
    return TSPDirectOnly
}

func (c *TSPClient) adaptiveSelection(recipient [32]byte) TSPMode {
    // Check proxy availability
    availableProxies := c.reputationMgr.GetTrustedProxies(c.config.ProxyThreshold)
    if len(availableProxies) == 0 {
        return TSPDirectOnly
    }
    
    // Default: use proxy for enhanced privacy
    return TSPProxyOnly
}

func (c *TSPClient) sendDirectMessage(recipient [32]byte, message []byte) error {
    // Create 0-hop TSP message using existing crypto.Encrypt
    // Send using existing transport layer
    return nil
}

func (c *TSPClient) sendViaProxy(recipient [32]byte, message []byte) error {
    // Select best proxy using reputation manager
    proxy := c.selectBestProxy()
    if proxy == nil {
        if c.config.AllowFallback {
            return c.sendDirectMessage(recipient, message)
        }
        return errors.New("no suitable proxy available")
    }
    
    // Create 1-hop TSP message
    return c.sendViaSpecificProxy(proxy, recipient, message)
}

func (c *TSPClient) selectBestProxy() *ProxyInfo {
    // Use reputation manager to select optimal proxy
    // Factor in trust score, latency, and capabilities
    return nil
}

func (c *TSPClient) sendViaSpecificProxy(proxy *ProxyInfo, recipient [32]byte, message []byte) error {
    // Create session if needed
    session := c.sessionManager.CreateSession(proxy.PublicKey)
    
    // Create inner message (encrypted for recipient)
    innerMsg, err := c.createInnerMessage(recipient, message)
    if err != nil {
        return err
    }
    
    // Create proxy payload (encrypted for proxy)
    proxyPayload := c.createProxyPayload(recipient, innerMsg)
    
    // Encrypt payload for proxy using existing crypto.Encrypt
    encryptedPayload, err := crypto.Encrypt(proxyPayload, nonce, proxy.PublicKey, c.privateKey)
    if err != nil {
        return err
    }
    
    // Create TSP packet
    tspPacket := c.createTSPPacket(session.SessionID, encryptedPayload)
    
    // Send using existing transport
    return c.transport.Send(tspPacket, proxy.Address)
}

func (c *TSPClient) createInnerMessage(recipient [32]byte, message []byte) ([]byte, error) {
    // Create message structure as per spec
    // Encrypt using existing crypto.Encrypt
    // Sign using existing crypto.Sign
    return nil, nil
}

func (c *TSPClient) createProxyPayload(recipient [32]byte, innerMsg []byte) []byte {
    // Create proxy instruction payload
    // Add padding to standard sizes (512, 1024, 1360 bytes)
    return nil
}

func (c *TSPClient) createTSPPacket(sessionID [8]byte, payload []byte) *transport.Packet {
    // Create TSP header + payload packet
    // Use existing transport.Packet structure
    return nil
}
```

## Phase 4: Proxy Node Implementation (Week 2)

### 4.1 Proxy Server (150 lines of code)

**File**: `tsp/proxy.go`

```go
package tsp

import (
    "net"
    "sync"
    "time"
    
    "github.com/opd-ai/toxcore/crypto"
    "github.com/opd-ai/toxcore/transport"
)

type ProxyServer struct {
    keyPair       *crypto.KeyPair
    transport     transport.Transport
    rateLimiter   *RateLimiter
    capabilities  uint16
    uptime        time.Time
    stats         ProxyStats
}

type RateLimiter struct {
    ipLimits     map[string]*TokenBucket
    globalLimit  *TokenBucket
    mutex        sync.RWMutex
}

type ProxyStats struct {
    MessagesForwarded uint32
    MessagesDropped   uint32
    ActiveSessions    uint32
}

func NewProxyServer(keyPair *crypto.KeyPair, transport transport.Transport) *ProxyServer {
    ps := &ProxyServer{
        keyPair:      keyPair,
        transport:    transport,
        rateLimiter:  NewRateLimiter(),
        capabilities: TSPCapForwardMessages | TSPCapPingRelay,
        uptime:       time.Now(),
    }
    
    // Register TSP packet handler
    transport.RegisterHandler(transport.PacketTSPProxiedMessage, ps.handleTSPMessage)
    
    return ps
}

func (ps *ProxyServer) handleTSPMessage(packet *transport.Packet, addr net.Addr) error {
    // Check rate limits
    if !ps.rateLimiter.Allow(addr) {
        ps.stats.MessagesDropped++
        return errors.New("rate limit exceeded")
    }
    
    // Parse TSP header
    header, err := ps.parseTSPHeader(packet.Data)
    if err != nil {
        return err
    }
    
    // Decrypt proxy payload using existing crypto.Decrypt
    payload, err := crypto.Decrypt(header.Payload, nonce, senderPK, ps.keyPair.Private)
    if err != nil {
        return err
    }
    
    // Extract target and inner message
    target, innerMsg, err := ps.parseProxyPayload(payload)
    if err != nil {
        return err
    }
    
    // Forward to target as standard Tox message
    return ps.forwardToTarget(target, innerMsg)
}

func (ps *ProxyServer) forwardToTarget(target [32]byte, message []byte) error {
    // Create standard Tox packet
    // Forward using existing transport
    // Update statistics
    ps.stats.MessagesForwarded++
    return nil
}

func (ps *ProxyServer) announceCapabilities() error {
    // Create proxy announcement
    announcement := ProxyAnnouncement{
        ProxyPublicKey: ps.keyPair.Public,
        Capabilities:   ps.capabilities,
        RateLimit:      1000, // Messages per minute
        Uptime:        uint32(time.Since(ps.uptime).Seconds()),
        Version:       0x01,
    }
    
    // Sign announcement using existing crypto.Sign
    hash := crypto.HashMessage(announcement)
    signature, err := crypto.Sign(hash, ps.keyPair.Private)
    if err != nil {
        return err
    }
    announcement.Signature = signature
    
    // Broadcast via DHT
    return ps.broadcastAnnouncement(announcement)
}
```

## Phase 5: Integration and Fallback (Week 3)

### 5.1 Graceful Fallback Logic (50 lines of code)

**File**: `tsp/fallback.go`

```go
package tsp

import (
    "context"
    "time"
)

type FallbackManager struct {
    client         *TSPClient
    fallbackDelay  time.Duration
    maxRetries     int
}

func NewFallbackManager(client *TSPClient) *FallbackManager {
    return &FallbackManager{
        client:        client,
        fallbackDelay: 5 * time.Second,
        maxRetries:    3,
    }
}

func (fm *FallbackManager) SendWithFallback(recipient [32]byte, message []byte) error {
    // Try proxy mode first
    ctx, cancel := context.WithTimeout(context.Background(), fm.fallbackDelay)
    defer cancel()
    
    err := fm.tryProxyWithTimeout(ctx, recipient, message)
    if err == nil {
        return nil
    }
    
    // Fallback to direct mode
    return fm.client.sendDirectMessage(recipient, message)
}

func (fm *FallbackManager) tryProxyWithTimeout(ctx context.Context, recipient [32]byte, message []byte) error {
    // Attempt proxy delivery with timeout
    // Return error if timeout or proxy unavailable
    return nil
}
```

### 5.2 Transport Layer Integration (20 lines of code)

**File**: `transport/noise_transport.go` (extend existing)

```go
// Add to existing RegisterHandler calls in NewNoiseTransport
func (nt *NoiseTransport) setupTSPHandlers() {
    // Register TSP packet handlers
    nt.RegisterHandler(transport.PacketTSPProxiedMessage, nt.handleTSPMessage)
    nt.RegisterHandler(transport.PacketTSPProxyAnnouncement, nt.handleProxyAnnouncement)
}

func (nt *NoiseTransport) handleTSPMessage(packet *transport.Packet, addr net.Addr) error {
    // Forward to TSP client for processing
    return nil
}

func (nt *NoiseTransport) handleProxyAnnouncement(packet *transport.Packet, addr net.Addr) error {
    // Forward to DHT for proxy discovery
    return nil
}
```

## Phase 6: Testing and Examples (Week 3)

### 6.1 Comprehensive Test Suite (300 lines of code)

**File**: `tsp/tsp_test.go`

```go
package tsp

import (
    "testing"
    "time"
    
    "github.com/opd-ai/toxcore/crypto"
    "github.com/opd-ai/toxcore/transport"
)

func TestTSPClientCreation(t *testing.T) {
    // Test client initialization
}

func TestDirectMessageSending(t *testing.T) {
    // Test 0-hop mode
}

func TestProxyMessageSending(t *testing.T) {
    // Test 1-hop mode
}

func TestFallbackBehavior(t *testing.T) {
    // Test proxy failure -> direct fallback
}

func TestProxyReputation(t *testing.T) {
    // Test reputation scoring
}

func TestSessionManagement(t *testing.T) {
    // Test session creation and cleanup
}

func TestRateLimiting(t *testing.T) {
    // Test proxy rate limiting
}

func TestMessageEncryption(t *testing.T) {
    // Test double encryption (proxy + recipient)
}

func TestMessageAuthenticity(t *testing.T) {
    // Test Ed25519 signature preservation
}

func TestPacketParsing(t *testing.T) {
    // Test TSP packet format parsing
}
```

### 6.2 Example Implementation (100 lines of code)

**File**: `examples/tsp_demo/main.go`

```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "github.com/opd-ai/toxcore/crypto"
    "github.com/opd-ai/toxcore/transport"
    "github.com/opd-ai/toxcore/tsp"
)

func main() {
    fmt.Println("TSP Demo - Tox Single-Hop Proxy Extension")
    
    // Create key pairs for Alice, Bob, and Proxy
    aliceKeys, _ := crypto.GenerateKeyPair()
    bobKeys, _ := crypto.GenerateKeyPair()
    proxyKeys, _ := crypto.GenerateKeyPair()
    
    // Create transport layer
    transport, _ := transport.NewUDPTransport("127.0.0.1:0")
    
    // Create TSP client for Alice
    aliceClient := tsp.NewTSPClient(aliceKeys, transport)
    
    // Create proxy server
    proxyServer := tsp.NewProxyServer(proxyKeys, transport)
    
    // Configure Alice to use proxy mode
    config := tsp.TSPConfig{
        DefaultMode:    tsp.TSPProxyOnly,
        ProxyThreshold: 0.7,
        AllowFallback:  true,
    }
    aliceClient.SetConfig(config)
    
    // Send message via proxy
    message := []byte("Hello Bob, this message is routed via proxy!")
    err := aliceClient.SendMessage(bobKeys.Public, message)
    if err != nil {
        log.Printf("Failed to send via proxy: %v", err)
    } else {
        fmt.Println("✅ Message sent via proxy successfully!")
    }
    
    // Demonstrate fallback behavior
    fmt.Println("\nTesting fallback to direct mode...")
    // ... fallback demonstration
    
    fmt.Println("\nTSP Demo completed successfully!")
}
```

## Code Size Summary

| Component | New Lines | Reused Infrastructure |
|-----------|-----------|----------------------|
| Packet Types | 50 | transport.Packet, PacketType enum |
| Session Management | 80 | crypto patterns, time handling |
| Reputation System | 100 | DHT routing table, crypto.Verify |
| TSP Client | 200 | crypto.Encrypt/Decrypt, transport layer |
| Proxy Server | 150 | transport handlers, crypto.Sign |
| Fallback Logic | 50 | context, error handling |
| Transport Integration | 20 | existing transport.RegisterHandler |
| Test Suite | 300 | testing framework |
| Example Demo | 100 | existing examples pattern |
| **Total New Code** | **1,050 lines** | **90% infrastructure reuse** |

## Security Considerations

### Preserved Guarantees
- **Ed25519 Authentication**: All message signatures preserved
- **Forward Secrecy**: Existing async messaging forward secrecy maintained
- **Encryption**: Double encryption (proxy + recipient) using existing crypto
- **Secure Memory**: Apply existing secure memory wiping patterns

### TSP-Specific Security
- **Proxy Reputation**: Prevent eclipse attacks through reputation scoring
- **Session Limits**: 10-minute sessions with 100-message limits
- **Rate Limiting**: Per-IP and global rate limiting on proxy nodes
- **Signature Verification**: All proxy announcements cryptographically signed

## Performance Impact

- **Latency**: +50-200ms (single proxy hop)
- **Bandwidth**: +15% (headers + padding)
- **Memory**: Minimal (session cache, reputation table)
- **CPU**: +5% (additional encryption layer)

## Integration Points

### 1. Tox Core Integration
```go
// In toxcore.go - extend existing SendFriendMessage
func (t *Tox) SendFriendMessage(friendID uint32, message string) error {
    if t.tspClient != nil && t.config.EnableTSP {
        return t.tspClient.SendMessage(friendPK, []byte(message))
    }
    // Existing direct message logic
}
```

### 2. DHT Integration
- Proxy announcements via existing DHT packet handling
- Reputation storage in existing routing table structures
- Bootstrap manager integration for proxy discovery

### 3. Transport Integration
- TSP packet types in existing PacketType enum
- Handler registration using existing RegisterHandler pattern
- Noise protocol integration for secure proxy communication

## Deployment Strategy

### Phase 1: Core Implementation (Week 1)
- Implement packet types and session management
- Basic proxy discovery via DHT
- Reputation system foundation

### Phase 2: Client/Server (Week 2)  
- Complete TSP client with mode selection
- Proxy server implementation
- Message encryption and forwarding

### Phase 3: Integration (Week 3)
- Transport layer integration
- Fallback mechanisms
- Comprehensive testing

### Phase 4: Production Ready (Week 4)
- Performance optimization
- Security audit
- Documentation and examples

## Success Metrics

- **Code Reuse**: >90% existing infrastructure leveraged
- **Performance**: <200ms additional latency
- **Compatibility**: 100% backward compatibility with standard Tox
- **Security**: All existing cryptographic properties preserved
- **Reliability**: Graceful fallback in 100% of proxy failure cases

This implementation plan delivers TSP/1.0 functionality with minimal new code by maximally leveraging the existing, well-tested toxcore-go infrastructure while maintaining security, performance, and compatibility guarantees.
