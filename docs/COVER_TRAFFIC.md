# Cover Traffic Design for toxcore-go

**Version**: 1.0  
**Date**: April 7, 2026  
**Status**: Design specification — implementation pending  

## Abstract

This document specifies the transport-layer cover traffic system for toxcore-go, a pure-Go implementation of the Tox peer-to-peer encrypted messaging protocol. Cover traffic is injected into live P2P connections between peers to defeat timing correlation and volume analysis on the wire. The design is tiered by transport type, opt-in by default, and uses simple constant-rate padding with adaptive shaping — not a mixnet design.

## Table of Contents

1. [Context: Existing Traffic Analysis Defenses](#1-context-existing-traffic-analysis-defenses)
2. [The Gap: Transport-Layer Timing Exposure](#2-the-gap-transport-layer-timing-exposure)
3. [Threat Model](#3-threat-model)
4. [Design Principles](#4-design-principles)
5. [Tiered Cover Traffic System](#5-tiered-cover-traffic-system)
6. [I2P-Specific Rationale](#6-i2p-specific-rationale)
7. [Core Mechanism: Constant-Rate Padding with Adaptive Shaping](#7-core-mechanism-constant-rate-padding-with-adaptive-shaping)
8. [Architecture Integration](#8-architecture-integration)
9. [Implementation Components](#9-implementation-components)
10. [Protocol Compatibility](#10-protocol-compatibility)
11. [Cost Analysis](#11-cost-analysis)
12. [What This Is Not](#12-what-this-is-not)
13. [Future Work](#13-future-work)

---

## 1. Context: Existing Traffic Analysis Defenses

toxcore-go already implements three layers of traffic analysis resistance. Understanding these is essential context for why transport-layer cover traffic is the correct next addition.

### 1.1 Message Padding (`async/message_padding.go`, `messaging/message.go`)

Messages are bucketed into four standard sizes — 256 B, 1024 B, 4096 B, and 16384 B — with random fill bytes padding each message to the next bucket boundary. A four-byte length prefix allows the receiver to strip padding after decryption. This prevents an observer from determining message length from packet sizes.

**What it protects**: Message length correlation.  
**What it does not protect**: The timing of when messages are sent, or the volume of traffic over time.

### 1.2 Retrieval Cover Traffic (`async/retrieval_scheduler.go`)

The `RetrievalScheduler` generates cover traffic for async message retrieval from storage nodes. Approximately 30% of retrievals are dummies: the client fetches and discards a message slot without storing anything. Timing follows a Poisson-like schedule using `crypto/rand` for jitter: a base interval of 5 minutes ± up to 50% random variation. The scheduler's `coverTrafficRatio` and intervals are configurable.

**What it protects**: Storage-node observers cannot determine when the user is actively checking for messages versus when a message actually arrived. Retrieval patterns cannot be correlated with conversation activity.  
**What it does not protect**: The direct P2P channel between two online peers.

### 1.3 Identity Obfuscation (`async/obfs.go`)

Epoch-rotating pseudonyms hide sender and recipient identities from storage nodes. Sender pseudonyms are single-use and unlinkable across messages. Recipients rotate their storage pseudonym each epoch. This prevents storage nodes from linking stored messages to real Tox identities or to each other.

**What it protects**: Identity linkage at the async messaging layer.  
**What it does not protect**: The direct peer-to-peer communication channel.

### 1.4 Summary of Current Coverage

| Attack Surface | Current Protection |
|---|---|
| Message length | ✅ Padding buckets |
| Retrieval timing at storage nodes | ✅ RetrievalScheduler cover traffic |
| Sender/recipient identity at storage nodes | ✅ Epoch pseudonyms |
| **Timing correlation on direct P2P channel** | ❌ Not protected |
| **Volume analysis on direct P2P channel** | ❌ Not protected |
| **Idle vs. active user fingerprinting** | ❌ Not protected |

---

## 2. The Gap: Transport-Layer Timing Exposure

A network observer with access to the path between two peers (ISP, backbone, I2P router operator, Lokinet relay) can currently determine:

- **When a conversation starts**: The packet rate jumps from near-zero (DHT maintenance pings every ~60 seconds) to bursty (rapid message sends and acknowledgements).
- **When a conversation ends**: The packet rate drops back to near-zero.
- **Conversation rhythm**: Even without decryption, the timing pattern of message exchanges reveals turn-taking, response latency, and activity periods.
- **Who is talking to whom**: By correlating timing spikes across two observed endpoints simultaneously (timing correlation attack).

None of the existing three layers addresses this. Message padding conceals lengths but not timing. Retrieval cover traffic covers the storage-node channel, not the direct P2P channel. Identity obfuscation protects async identity, not live connection metadata.

Transport-layer cover traffic is the missing piece.

---

## 3. Threat Model

### 3.1 What Cover Traffic Protects Against

| Attack | Mechanism |
|---|---|
| **Timing correlation** | Constant-rate stream makes "Alice is sending now" indistinguishable from "Alice is idle" to an observer watching either endpoint. |
| **Volume analysis** | Steady baseline traffic absorbs message send bursts without a detectable change in total volume per unit time. |
| **Activity fingerprinting** | Idle and active states produce identical-looking traffic streams on the wire. |
| **Intersection attacks** | Continuous traffic prevents observers from determining that two users were online simultaneously by measuring their correlated silence-to-activity transitions. |
| **Conversation start/stop detection** | The beginning and end of a conversation are not visible in the traffic trace. |

### 3.2 What Cover Traffic Does Not Protect Against

| Limitation | Explanation |
|---|---|
| **Active tagging attacks** | An adversary who can inject delays or modify packets can tag traffic and observe the tag's arrival downstream. Cover traffic does not prevent this. |
| **Compromised endpoints** | Key compromise renders traffic analysis irrelevant. Cover traffic is transport security, not endpoint security. |
| **Long-term statistical analysis against static-rate cover** | If the cover rate is perfectly constant and never varies, a sufficiently patient adversary can learn the baseline and detect deviations caused by queuing. The Full tier's strict constant-rate shaping mitigates this for users who need it. |
| **Global passive adversary with complete network visibility** | In this extreme threat model, timing correlation across the entire network remains feasible even with cover traffic, unless the traffic is routed through a high-latency mixnet (which is Nym's role, not this system's). |
| **Lokinet and I2P internal adversaries with full tunnel visibility** | Cover traffic reduces exposure but does not eliminate timing correlation if an adversary controls multiple hops. |

### 3.3 Threat Model Fit

For Tox's primary threat model — protecting users from mass surveillance by ISPs, backbone observers, and nation-state passive monitoring programs — cover traffic provides meaningful and direct improvement. It is not designed to defeat a targeted, active, well-resourced adversary with network-wide visibility. Users in that threat category should use Nym, which provides purpose-built mixnet anonymity.

---

## 4. Design Principles

1. **Constant-rate stream masking**: The fundamental guarantee is that an observer cannot distinguish an idle peer from an active one by measuring packet rate or timing. This requires a constant-rate output, not just probabilistic cover.

2. **Replacement, not addition**: When real messages are ready to send, they replace the next scheduled cover slot rather than adding to the stream. Total output rate stays constant. This is the key property that prevents traffic analysis from detecting activity.

3. **Per-peer granularity**: Cover traffic is managed independently for each peer connection. The rate is the same whether a peer is idle or engaged in a conversation.

4. **Noise encryption for all cover packets**: Cover packets are sent through `NoiseTransport.Send()` and are encrypted identically to real traffic. A network observer cannot distinguish cover from real by examining packet bytes, only by attempting statistical analysis of the stream — which the constant rate defeats.

5. **Dedicated packet type, silently discarded**: Cover packets use a dedicated `PacketCoverTraffic` type. After Noise decryption, the receiver identifies the type and discards the packet without processing it further. This requires no changes to upper-layer protocol logic.

6. **Disabled by default**: Cover traffic is not enabled by default to avoid unexpected bandwidth and battery costs for existing users. The appropriate tier is surfaced as a recommendation based on the detected transport type.

7. **Crypto/rand timing jitter**: All timing uses `crypto/rand` for jitter, consistent with the existing `RetrievalScheduler` pattern. Predictable timing would itself be a fingerprint.

---

## 5. Tiered Cover Traffic System

Cover traffic tiers are assigned per transport type based on the threat landscape of each transport and whether the transport natively provides its own cover.

### 5.1 Tier Table

| Tier | Default Transports | Rate | Rationale |
|---|---|---|---|
| **Off** | Nym | None | Nym is a purpose-built mixnet (Loopix-based) with its own native cover traffic. Application-layer cover would be redundant and would double bandwidth through the mixnet. |
| **Light** | Direct UDP (clearnet), I2P, Lokinet | 0.2 pkt/s per peer | See §5.2 and §6. |
| **Full** | Any transport (opt-in) | 1 pkt/s per peer | Strict constant-rate stream for users who accept the bandwidth cost in exchange for the strongest available traffic-shaping guarantees. |

### 5.2 Light Tier Rationale by Transport

**Direct UDP (clearnet)**: No anonymization layer exists between peers. An ISP, network operator, or infrastructure observer can directly observe packet timing on the wire. Cover traffic is the primary and only traffic analysis defense for clearnet connections.

**I2P**: I2P's design explicitly delegates cover traffic and timing delays to the application layer. See §6 for full rationale.

**Lokinet**: Lokinet provides onion routing with low latency and a relatively small network. It faces similar timing correlation vulnerabilities as I2P for low-latency applications like Tox. The marginal cost of Light cover traffic is low relative to the protection it provides.

**Tor** (not listed): Tor's design provides stronger traffic analysis resistance than I2P or Lokinet through its cell-level uniformity and larger anonymity set. Transport-layer cover traffic on Tor would be redundant for most users, though the Full tier remains available as an option.

### 5.3 Tier Selection Logic

On initialization, the cover traffic system inspects the active transport via `SupportedNetworks()` and recommends the appropriate tier. `SupportedNetworks()` may return multiple identifiers for a single transport (for example, `IPTransport` returns `["tcp","udp","tcp4","tcp6","udp4","udp6"]`), so selection must be based on whether the returned slice contains a relevant network name, not on exact equality with a single-element slice:

- Returned networks contain `"nym"` → recommend Off
- Returned networks contain `"i2p"` or `"lokinet"` → recommend Light
- Returned networks contain `"udp"`, `"udp4"`, or `"udp6"` → recommend Light
- Returned networks contain `"tor"` → recommend Off (Tor provides strong inherent resistance)
- Any transport + user explicitly requests Full → Full

The recommendation is surfaced to the user through configuration documentation. The actual tier is set in `CoverTrafficConfig` and is never changed automatically after initialization.

---

## 6. I2P-Specific Rationale

I2P warrants detailed treatment because the decision to include it in the Light tier rests on the architecture of I2P itself, not just observed weaknesses.

### 6.1 The Application Contract

I2P provides **tunnel infrastructure**: garlic routing, layered encryption, hop selection, and tunnel management. It does NOT inject native cover traffic. This is by design. I2P's documentation and architecture explicitly assume that applications generating sensitive traffic patterns will handle their own cover traffic and timing obfuscation.

For toxcore-go, this means light cover traffic when running over I2P is not optional hardening — it is **fulfilling the expected application behavior** for correct I2P usage. An application that sends bursty, identifiable traffic over I2P without its own cover is failing to hold up its end of I2P's application contract.

### 6.2 Garlic Batching Effectiveness

I2P's garlic routing bundles multiple "cloves" (sub-messages) together for transit. The security benefit of garlic routing depends on having multiple cloves to bundle: a single-clove garlic message provides no batching and passes through 2–3 hop tunnels with negligible mixing.

For a low-traffic Tox client that only sends messages intermittently, garlic batching frequently degrades to single-clove messages. Application-layer cover traffic provides additional cloves that the I2P router *can* batch with real messages, directly improving the garlic routing effectiveness that I2P was designed to provide.

### 6.3 Demonstrated Timing Attack Vulnerability

Research has demonstrated practical timing correlation attacks against I2P for low-latency, bursty communication — exactly the traffic pattern that Tox produces. I2P's smaller network (roughly 30K routers with a smaller active user base than Tor) reduces the effective anonymity set, making statistical correlation more tractable for an adversary.

### 6.4 Marginal Cost

I2P already imposes ~300–500 ms round-trip latency. Users who chose I2P as their transport have already accepted significant performance trade-offs for privacy. Adding 0.2 pkt/s × 256 B = ~51 B/s of cover traffic per peer is negligible relative to I2P's existing tunnel overhead. The privacy benefit is not marginal; the bandwidth cost is.

### 6.5 Summary

Light cover traffic on I2P is:
- Required by I2P's application contract
- Directly beneficial for garlic batching effectiveness
- Meaningful protection against demonstrated timing attacks
- Negligible in bandwidth cost

---

## 7. Core Mechanism: Constant-Rate Padding with Adaptive Shaping

### 7.1 Idle State

When no real messages are ready to send to a peer, the cover traffic generator sends a cover packet at the next scheduled slot. The schedule uses a base rate (0.2 pkt/s for Light, 1 pkt/s for Full) plus crypto/rand jitter of up to ±10% to prevent the rate itself from being a recognizable fingerprint.

A cover packet consists of:
- Noise-encrypted header with `PacketCoverTraffic` type byte
- Random payload padded to 256 bytes using `crypto/rand`

The receiver decrypts the Noise envelope, reads the type byte, and discards the packet. No upper-layer processing occurs.

### 7.2 Active State (Message Replacement)

When a real message is ready to send before the next scheduled cover slot, the message **replaces** the cover slot. The cover slot is consumed; no additional packet is sent. The next cover slot is scheduled from the time of the real send, maintaining the constant average rate.

This "replacement" model is critical. If real messages were added to the stream (cover packets sent in addition to real messages), an observer would see a rate increase when the user becomes active — exactly what cover traffic is supposed to prevent.

```
Timeline (Light tier, 0.2 pkt/s ≈ 1 packet every 5 seconds):

Idle:   C----C----C----C----C----C
                   ^ cover packets, 5s apart

Active: C----C--M-C----M-C----C--
                ^ real message replaces cover slot
                             ^ another real message
```

In both states, the observer sees approximately one packet every 5 seconds. The content is indistinguishable (all encrypted, all 256 B after padding).

### 7.3 Jitter

Timing jitter is applied using `crypto/rand` to each inter-packet interval. For Light tier, the base interval is 5 seconds (1/0.2) with ±10% random variation, yielding actual intervals uniformly distributed in [4.5 s, 5.5 s]. This prevents the cover traffic rate from becoming a recognizable periodic signature.

The jitter range is intentionally narrow (±10%) to maintain the constant-rate guarantee. Wider jitter would create observable variance in the output stream.

### 7.4 Packet Format

Cover packets use a dedicated packet type value reserved in the Tox packet type namespace:

```
PacketCoverTraffic PacketType = 0xCE  // to be formally assigned during implementation
```

The final value must be chosen to avoid conflicts with existing packet types in the Tox protocol. The implementation PR that introduces this type should audit `transport/` and the Tox protocol specification for all currently allocated type bytes and assign an unoccupied value. The value 0xCE is a placeholder used for discussion purposes in this document.

The full cover packet on the wire is a `PacketNoiseMessage` whose payload is the AEAD-encrypted cleartext. `NoiseTransport` encrypts `Packet.Serialize()` — one type byte followed by the data — using the Noise session's ChaCha20-Poly1305 cipher, which appends a 16-byte authentication tag. The on-wire structure is therefore:

```
[ PacketNoiseMessage type (1 B) ][ encrypted: PacketCoverTraffic type (1 B) + random payload (238 B) + AEAD tag (16 B) ] = 256 B
```

The total wire size matches the smallest message padding bucket (256 B), making cover packets indistinguishable from padded real messages of minimum size to any observer that cannot decrypt the Noise envelope.

### 7.5 Comparison to Existing RetrievalScheduler

The cover traffic mechanism follows the same pattern as `async/retrieval_scheduler.go`:

| Aspect | RetrievalScheduler | CoverTrafficGenerator |
|---|---|---|
| Timing | crypto/rand jitter on base interval | crypto/rand jitter on base interval |
| Cover ratio | ~30% of retrievals are dummies | Continuous dummy stream; real messages replace slots |
| Stop signal | `stopChan chan struct{}` | `stopChan chan struct{}` |
| Goroutine per | AsyncClient instance | Per-peer connection |
| Configurable | Yes (ratio, interval) | Yes (tier, rate) |

The RetrievalScheduler proves the pattern is already understood and implemented in this codebase. The cover traffic generator is the same design applied to the transport layer.

---

## 8. Architecture Integration

### 8.1 Transport Interface Hook (`transport/types.go`)

The `Transport` interface defined in `transport/types.go` has `Send(packet *Packet, addr net.Addr) error` as its primary send method. This is the correct injection point — a `CoverTrafficTransport` wrapper implements `Transport` and wraps a `NoiseTransport` instance. Cover traffic shaping sits *above* Noise encryption so that both real and cover packets are encrypted identically before reaching the wire:

```
Application
    │
    ▼
CoverTrafficTransport (implements Transport)  ← shapes output; injects cover packets
    │                                            real and cover packets both delegated to...
    ▼
NoiseTransport (implements Transport)         ← encrypts all packets identically
    │
    ▼
UDPTransport / I2PTransport / LokinetTransport (implements NetworkTransport)
```

Because all layers use interface types with no concrete type assertions (per project conventions), wrapping requires no changes to the layers above or below. Note that `NetworkTransport` (from `transport/network_transport.go`) is a separate dial/listen abstraction used for establishing connections; it does not carry a `Send()` method. Tier detection uses `NetworkTransport.SupportedNetworks()` (see §9.3), while traffic shaping uses `Transport.Send()`.

### 8.2 IterationPipelines Integration (`iteration_pipelines.go`)

The `IterationPipelines` system already manages concurrent goroutines with ticker-based scheduling for DHT maintenance, friend connection management, and message processing. A cover traffic pipeline slots in as a new `PipelineType`:

```go
PipelineCoverTraffic PipelineType = iota // after PipelineMessages
```

The pipeline's ticker fires at a rate derived from the configured tier and manages the per-peer goroutine lifecycle (starting generators when peers connect, stopping them when peers disconnect).

### 8.3 PacketType System

Cover packets use a dedicated type constant in the packet type namespace. The existing `RegisterHandler` mechanism routes incoming packets to type-specific handlers. The cover traffic handler discards received cover packets after Noise decryption with no further processing:

```go
tox.RegisterHandler(PacketCoverTraffic, func(p *Packet, addr net.Addr) error {
    // Silently discard. Receipt confirms peer is reachable; no other action.
    return nil
})
```

### 8.4 RetrievalScheduler Pattern Reference

The `RetrievalScheduler` in `async/retrieval_scheduler.go` demonstrates all the key implementation patterns:
- `crypto/rand`-based jitter (`crypto/rand.Int` with `math/big.Int` for interval calculation)
- Probabilistic decision-making (`coverTrafficRatio` float64 threshold)
- Graceful shutdown via `stopChan chan struct{}` and `sync.Mutex`
- Configurable intervals and ratios exposed as struct fields

The `CoverTrafficGenerator` should follow these patterns directly.

---

## 9. Implementation Components

This section describes the three components that together implement the cover traffic system. Code is not provided here; this is a design specification.

### 9.1 CoverTrafficGenerator

**Purpose**: Manages cover traffic for a single peer connection.

**Behavior**:
- Runs as a goroutine per active peer connection
- Maintains a ticker at the configured rate with crypto/rand jitter
- On each tick: if no real message was sent since the last tick, constructs and sends a cover packet via the transport's `Send()` method
- If a real message was sent in the interval, resets the ticker from the send time (maintaining rate without adding extra traffic)
- Stops cleanly on `stopChan` signal or context cancellation

**Configuration**:
- `rate float64` — packets per second (0.2 for Light, 1.0 for Full)
- `jitterPercent int` — timing jitter as percentage of base interval (10% default)
- `packetSize int` — fixed at 256 B to match minimum padding bucket

**Key invariants**:
- All timing uses `crypto/rand`; no `math/rand` or `time.Now()` modulo arithmetic
- The generator goroutine holds no locks during `Send()` calls
- Graceful stop: ongoing `Send()` calls are allowed to complete; no new sends after stop signal

### 9.2 CoverTrafficTransport

**Purpose**: `Transport` wrapper that intercepts `Send()` to coordinate cover traffic timing with real message sends.

**Behavior**:
- Wraps any `Transport` implementation (typically `NoiseTransport`)
- Maintains a `sync.Map` of per-peer `CoverTrafficGenerator` instances, keyed by `net.Addr.String()`
- On `Send(packet, addr)`: records the send time for the peer's generator (so the generator can skip the next cover slot); forwards to the wrapped transport
- On peer disconnect (detected via `Close()` or explicit `RemovePeer(addr)`): stops and removes the peer's generator
- On first `Send()` to a new peer address: starts a new generator for that peer

**Interface compliance**:
- Implements `Transport` fully: `Send`, `Close`, `LocalAddr`, `RegisterHandler`, `IsConnectionOriented`
- All delegation uses interface methods; no type assertions

### 9.3 Configuration (`CoverTrafficConfig`)

```go
// CoverTrafficTier defines the cover traffic intensity level.
type CoverTrafficTier int

const (
    CoverTrafficOff   CoverTrafficTier = iota // No cover traffic (zero value; disabled by default)
    CoverTrafficLight                         // 0.2 pkt/s per peer
    CoverTrafficFull                          // 1.0 pkt/s per peer
)

// CoverTrafficConfig holds cover traffic configuration.
type CoverTrafficConfig struct {
    Tier CoverTrafficTier
    // CustomRate overrides the tier's default rate (packets per second).
    // Zero means use the tier default.
    CustomRate float64
}
```

**Integration with Options**: `CoverTrafficConfig` is a field of `Options` in the Go API (exported to C as `ToxOptions`). `NewOptions()` returns options with `CoverTrafficOff` as default.

**Auto-detection helper**: A `SuggestCoverTrafficTier(transport NetworkTransport) CoverTrafficTier` helper inspects `transport.SupportedNetworks()` (on `NetworkTransport` from `transport/network_transport.go`) and returns the recommended tier per §5.3. The recommendation is informational only; the user must explicitly set the tier.

---

## 10. Protocol Compatibility

### 10.1 Forward Compatibility

Peers that do not implement cover traffic will receive `PacketCoverTraffic` packets but have no registered handler for that packet type. In the current transport paths, packets with no handler are silently dropped — `UDPTransport.dispatchPacketToHandler` logs at debug level and returns without invoking any handler (`transport/udp.go`); `NoiseTransport.handleEncryptedPacket` similarly no-ops when no handler exists. No protocol-level negotiation is needed; cover traffic senders unilaterally decide to send cover, and receivers silently ignore what they do not understand.

When both peers implement cover traffic, the receiver's registered handler silently discards packets, which is functionally identical to the non-implementation case.

### 10.2 Noise Encryption

`CoverTrafficTransport` sits above `NoiseTransport` in the stack and delegates all `Send()` calls — both real messages and injected cover packets — to the wrapped `NoiseTransport`. All cover packets are therefore encrypted with the same Noise-IK session keys as real traffic before reaching the wire. A network observer cannot distinguish cover from real packets at the byte level.

### 10.3 No C API Impact

Cover traffic is an internal transport concern implemented entirely below the `toxcore` package's public API. The `capi` package provides C bindings to the public Go API and does not expose transport internals. No changes to `capi/` are required.

The `CoverTrafficConfig` would be exposed via `toxcore.Options` (Go) / `ToxOptions` (C export), which already has C API wrappers for configuration fields. Adding a new config field follows the existing pattern.

### 10.4 ToxAV (Audio/Video Calls)

Active audio/video calls produce continuous RTP packet streams (`av/rtp/`) at rates of 50+ packets per second. This continuous stream already functions as effective natural cover traffic for the duration of a call: conversation timing, activity, and volume are all concealed by the RTP stream's uniform rate.

During active calls, the per-peer cover traffic generator detects the high real message rate and its cover slots are continuously replaced by real RTP packets. The cover traffic system adds no overhead during calls.

After a call ends, the cover stream resumes at the configured rate. The transition from call to post-call state is not visible to an observer as a change in the packet rate, only as a reduction in total volume — which the cover stream maintains at a steady baseline.

### 10.5 DHT Traffic

DHT maintenance packets (ping, find_node, etc.) flow through the same transport layer. During DHT activity, these packets also replace cover slots for the relevant peer addresses. The DHT ping interval (~60 seconds) is much longer than the Light cover interval (~5 seconds), so DHT packets account for a small fraction of cover slot replacements and do not meaningfully affect the cover traffic rate.

---

## 11. Cost Analysis

### 11.1 Bandwidth

| Cover Tier | Per-Peer Overhead | 10 Friends Online | Monthly (30 days) |
|---|---|---|---|
| Off | 0 B/s | 0 B/s | 0 |
| Light (0.2 pkt/s × 256 B) | 51 B/s ≈ 0.4 Kbps | 512 B/s ≈ 4 Kbps | ~1.3 GB |
| Full (1 pkt/s × 256 B) | 256 B/s ≈ 2 Kbps | 2560 B/s ≈ 20 Kbps | ~6.3 GB |

For context: a typical Tox user currently generates less than 1 MB/day of DHT maintenance and messaging traffic. Even Light cover traffic at 0.2 pkt/s represents a significant baseline increase per active friend.

The calculation assumes all 10 friends are continuously online. In practice, fewer friends will be online at any given time, and the actual monthly figure will be lower.

### 11.2 CPU and Memory

| Resource | Light Tier (10 peers) | Full Tier (10 peers) |
|---|---|---|
| Goroutines | 10 (one per peer) | 10 (one per peer) |
| Stack memory | ~40 KB (4 KB/goroutine) | ~40 KB (4 KB/goroutine) |
| Timer handles | 10 | 10 |
| Encryption ops | 2/s total | 10/s total |
| CPU time | Negligible | Negligible |

Noise encryption of a 256-byte packet takes on the order of microseconds on modern hardware. At 1 pkt/s per peer and 10 peers, the CPU cost is unmeasurable against normal Tox operation.

### 11.3 Battery and Mobile Impact

Battery impact is the highest real-world cost of cover traffic on mobile devices.

**Mechanism**: Mobile cellular radios use power state machines. When data is transmitted or received, the radio enters a high-power Connected state. After a period of inactivity (typically 5–10 seconds), the radio transitions to a lower-power state or Idle. Cover traffic at 0.2 pkt/s (one packet every 5 seconds) prevents the radio from ever fully entering the low-power state.

| Scenario | Approximate Power Draw |
|---|---|
| Cellular radio in idle / no traffic | ~10–50 mW |
| Cellular radio with continuous low-rate traffic | ~500–1500 mW |
| Estimated daily battery impact (Light, 10 peers) | 10–20% additional drain |

For this reason:
- Cover traffic defaults to Off on all transports
- Mobile applications should surface the Light tier as optional, with explicit battery impact disclosure
- The Full tier is not recommended for mobile in any configuration

---

## 12. What This Is Not

### 12.1 Not a Mixnet

This design does not attempt to provide mixnet-level anonymity. A mixnet (Tor, I2P, Nym, Loopix) routes messages through multiple intermediate nodes, mixing traffic from multiple senders so that the input and output of each node cannot be correlated by a single observer. This system provides no routing intermediaries, no traffic mixing across users, and no anonymity set beyond the two communicating peers.

The goal is narrow and precise: make it impossible for a passive observer at either endpoint (or on the path between them) to distinguish an idle peer from an active one by measuring packet rate and timing. This is traffic analysis resistance, not anonymity.

### 12.2 Not Loopix

Loopix is a specific mixnet design for high-latency store-and-forward systems. It uses loop cover messages (sent to yourself through the mix cascade and returning to confirm liveness), drop cover messages (sent to random recipients and discarded), and provider-level mailbox nodes. These concepts are inapplicable here:

- Tox is peer-to-peer; there is no mix cascade and no provider layer
- Tox messaging is low-latency; users expect near-real-time delivery
- Loop cover requires a message to traverse multiple intermediate nodes and return; Tox has no such routing infrastructure in the direct messaging path
- Loopix cover ratios (2:1 cover-to-real) are tuned for a mixnet's anonymity set requirements, not for masking P2P conversation patterns

Loopix is mentioned here to be explicitly ruled out, not because it was seriously considered. It is a fine design for what it was designed for.

### 12.3 Not Silence Suppression Compensation (ToxAV)

ToxAV's RTP subsystem has a separate concern: silence suppression (VAD) during audio calls creates detectable silent periods that reveal when a participant is speaking. This is a ToxAV-specific problem that cover traffic does not address. VAD-level cover is a separate design concern for the `av/` package.

### 12.4 Not a Replacement for Existing Defenses

Cover traffic is an additional layer, not a replacement for:
- Message padding (still needed to mask content length)
- Retrieval cover traffic (still needed to mask async message storage-node access patterns)
- Identity obfuscation (still needed to mask sender/recipient identity at storage nodes)
- Nym transport (still needed for users who require full mixnet anonymity)

---

## 13. Future Work

The following related improvements are out of scope for this design but worth noting:

- **VAD cover for ToxAV**: Injecting comfort noise packets during silence periods in audio calls to prevent silence detection via traffic analysis. Design belongs in `av/audio/`.

- **DHT cover traffic**: Injecting dummy DHT queries to prevent observers from correlating DHT lookup patterns with friend additions or online status checks. This would extend `dht/` rather than `transport/`.

- **Bandwidth budgeting**: A per-peer or global bandwidth budget that automatically scales cover traffic down when the total peer count is high, preventing unexpected data overages for users with many online friends.

- **Mobile-aware adaptive rate**: Detecting mobile network conditions (via OS APIs or connectivity changes) and automatically reducing cover rate when on cellular, without user intervention.

- **Cover traffic metrics**: Counters for cover packets sent/received per peer, exposed via the toxcore stats API, for debugging and monitoring purposes.

---

## References

- `async/retrieval_scheduler.go` — RetrievalScheduler: pattern reference for jitter, probabilistic decisions, and graceful stop
- `async/message_padding.go` — Message padding: bucket sizes and random fill implementation
- `async/obfs.go` — Identity obfuscation: epoch pseudonym design
- `transport/types.go` — Transport interface: `Send()` injection point for `CoverTrafficTransport` wrapping
- `transport/network_transport.go` — NetworkTransport interface: `SupportedNetworks()` for tier detection
- `iteration_pipelines.go` — IterationPipelines: concurrent goroutine management with ticker scheduling
- I2P project documentation on application-layer traffic responsibility
- Vuvuzela (Henry et al., 2015) — constant-rate private messaging without server infrastructure assumptions
- Loopix (Piotrowska et al., 2017) — mixnet cover traffic design (cited to explicitly distinguish this design from it)
