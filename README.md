# toxcore-go

[![Build Status](https://img.shields.io/github/actions/workflow/status/opd-ai/toxcore/toxcore.yml?branch=main)](https://github.com/opd-ai/toxcore/actions) [![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE) [![Go Version](https://img.shields.io/github/go-mod-go-version/opd-ai/toxcore)](go.mod) [![Codecov](https://img.shields.io/codecov/c/github/opd-ai/toxcore)](https://codecov.io/gh/opd-ai/toxcore)

A pure Go implementation of the [Tox](https://tox.chat/) peer-to-peer encrypted messaging
protocol. toxcore-go provides DHT-based peer discovery, friend management, 1-to-1 and group
messaging, file transfers, audio/video calling (ToxAV), asynchronous offline messaging with
forward secrecy, and multi-network transport — all without cgo dependencies in the core library.

## Table of Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [Configuration](#configuration)
- [Multi-Network Transport](#multi-network-transport)
- [Noise Protocol Integration](#noise-protocol-integration)
- [Audio/Video Calls (ToxAV)](#audiovideo-calls-toxav)
- [Asynchronous Offline Messaging](#asynchronous-offline-messaging)
- [State Persistence](#state-persistence)
- [C API Bindings](#c-api-bindings)
- [Project Structure](#project-structure)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

## Features

- **DHT Routing** — Modified Kademlia DHT for serverless peer discovery with k-bucket
  routing, iterative lookups, and LAN/mDNS local discovery (`dht/`)
- **Friend Management** — Friend requests, contact list, connection status tracking,
  and sharded state storage (`friend/`)
- **1-to-1 Messaging** — Encrypted real-time messaging with delivery tracking, retry
  logic, and traffic-analysis-resistant padding (`messaging/`)
- **Group Chat** — DHT-based group chat with role-based permissions, peer-to-peer
  broadcasting, and sender key distribution (`group/`)
- **File Transfers** — Bidirectional chunked file transfers with pause, resume,
  cancellation, and progress tracking (`file/`)
- **ToxAV Audio/Video** — Peer-to-peer calling with Opus audio encoding via
  `opd-ai/magnum` and VP8 video via `opd-ai/vp8`, RTP transport, adaptive
  bitrate, and jitter buffering (`av/`, `av/audio/`, `av/video/`, `av/rtp/`)
- **Asynchronous Offline Messaging** — Store-and-forward delivery through distributed
  storage nodes with end-to-end encryption, forward secrecy via one-time pre-keys,
  and identity obfuscation via epoch-based pseudonyms (`async/`)
- **Multi-Network Transport** — IPv4/IPv6 UDP/TCP, Tor `.onion`, I2P `.b32.i2p`,
  Lokinet `.loki` (dial-only), and Nym `.nym` (dial-only) (`transport/`)
- **Noise-IK Handshakes** — Noise Protocol Framework (IK and XX patterns) for
  forward secrecy, KCI resistance, and mutual authentication via `flynn/noise`
  (`noise/`, `transport/noise_transport.go`)
- **NAT Traversal** — STUN external-address discovery, UPnP port mapping, UDP hole punching, and TCP relay fallback for symmetric NAT
  (`transport/nat.go`, `transport/hole_puncher.go`, `transport/advanced_nat.go`)
- **Cryptography** — Curve25519 key exchange, ChaCha20-Poly1305 authenticated
  encryption, Ed25519 signatures, replay protection, and secure memory wiping
  (`crypto/`)
- **C API Bindings** — libtoxcore-compatible C function exports for toxcore and
  ToxAV; requires cgo (`capi/`)
- **Go net.* Interfaces** — `net.Conn`, `net.Listener`, `net.PacketConn`, and
  `net.Addr` implementations for stream and datagram Tox communication (`toxnet/`)
- **Protocol Version Negotiation** — Automatic per-peer negotiation between legacy
  Tox protocol and Noise-IK enhanced protocol
  (`transport/negotiating_transport.go`)
- **Concurrent Iteration Pipelines** — DHT maintenance, friend connections, and
  message processing decoupled into separate goroutines
  (`iteration_pipelines.go`)

## Requirements

- **Go** 1.25.0 or later (toolchain go1.25.8)
- **Platforms**: Linux, macOS, Windows (amd64, arm64; Windows arm64 excluded from CI)
- **cgo** required only for C API bindings (`capi/` package)

## Installation

1. Add the module to your project:

```bash
go get github.com/opd-ai/toxcore
```

2. Verify the installation:

```bash
go mod verify
go build ./...
```

3. Run tests (excludes network-dependent tests):

```bash
go test -tags nonet -race ./...
```

## Usage

### Creating a Tox Instance

Create a Tox instance, register event callbacks, bootstrap into the DHT network,
and run the event loop:

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/opd-ai/toxcore"
)

func main() {
    options := toxcore.NewOptions()
    tox, err := toxcore.New(options)
    if err != nil {
        log.Fatal(err)
    }
    defer tox.Kill()

    fmt.Println("My Tox ID:", tox.SelfGetAddress())

    // Accept incoming friend requests
    tox.OnFriendRequest(func(publicKey [32]byte, message string) {
        friendID, err := tox.AddFriendByPublicKey(publicKey)
        if err != nil {
            log.Printf("Accept friend request failed: %v", err)
            return
        }
        fmt.Printf("Accepted friend %d\n", friendID)
    })

    // Echo received messages
    tox.OnFriendMessage(func(friendID uint32, message string) {
        fmt.Printf("Friend %d: %s\n", friendID, message)
        tox.SendFriendMessage(friendID, "Echo: "+message)
    })

    // Bootstrap into the DHT network
    err = tox.Bootstrap("node.tox.biribiri.org", 33445,
        "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
    if err != nil {
        log.Printf("Bootstrap warning: %v", err)
    }

    for tox.IsRunning() {
        tox.Iterate()
        time.Sleep(tox.IterationInterval())
    }
}
```

### Sending Messages

`SendFriendMessage` accepts an optional `MessageType` parameter. When the friend is
offline, messages automatically fall back to asynchronous store-and-forward delivery.

```go
// Normal text message (default)
err := tox.SendFriendMessage(friendID, "Hello!")

// Action message (like IRC /me)
err = tox.SendFriendMessage(friendID, "waves hello",
    toxcore.MessageTypeAction)
```

Messages are limited to 1372 UTF-8 bytes. For message-type-aware receiving, use
`OnFriendMessageDetailed`:

```go
tox.OnFriendMessageDetailed(func(friendID uint32, message string,
    messageType toxcore.MessageType) {
    fmt.Printf("[%v] Friend %d: %s\n", messageType, friendID, message)
})
```

### Friend Management

```go
// Send a friend request (76-character hex Tox ID)
friendID, err := tox.AddFriend(toxIDString, "Hi, let's connect!")

// Accept a friend request (in OnFriendRequest callback)
friendID, err := tox.AddFriendByPublicKey(publicKey)

// List and remove friends
friends := tox.GetFriends()
err = tox.DeleteFriend(friendID)
```

### Group Chat (Conferences)

```go
conferenceID, err := tox.ConferenceNew()
err = tox.ConferenceInvite(friendID, conferenceID)
err = tox.ConferenceSendMessage(conferenceID, "Hello group!",
    toxcore.MessageTypeNormal)
err = tox.ConferenceDelete(conferenceID)
```

Register group callbacks via the `group.Chat` interface:

```go
chat, err := tox.ValidateConferenceAccess(conferenceID)
chat.OnMessage(func(groupID, peerID uint32, message string) {
    fmt.Printf("[Group %d] Peer %d: %s\n", groupID, peerID, message)
})
```

### File Transfers

```go
// Send a file to a friend
fileNumber, err := tox.FileSend(friendID, 0, fileSize, fileID, "photo.jpg")

// Receive file data via callbacks
tox.OnFileRecv(func(friendID, fileID, kind uint32, size uint64,
    filename string) {
    tox.FileControl(friendID, fileID, toxcore.FileControlResume)
})
tox.OnFileRecvChunk(func(friendID, fileID uint32, position uint64,
    data []byte) {
    // Write data to file at position
})
```

## Configuration

### Options

`NewOptions()` returns an `Options` struct with these defaults:

| Field | Default | Description |
|-------|---------|-------------|
| `UDPEnabled` | `true` | Enable UDP transport |
| `IPv6Enabled` | `true` | Enable IPv6 support |
| `LocalDiscovery` | `true` | Enable LAN peer discovery |
| `TCPPort` | `0` (disabled) | TCP listening port |
| `StartPort` | `33445` | UDP port range start |
| `EndPort` | `33545` | UDP port range end |
| `ThreadsEnabled` | `true` | Enable concurrent iteration pipelines |
| `BootstrapTimeout` | `30s` | Timeout for initial DHT connectivity |
| `MinBootstrapNodes` | `4` | Minimum bootstrap nodes required |
| `AsyncStorageEnabled` | `true` | Participate as async message storage node |
| `SavedataType` | `SaveDataTypeNone` | Savedata format (`SaveDataTypeToxSave`, `SaveDataTypeSecretKey`) |
| `SavedataData` | `nil` | Previously saved state bytes |

### Proxy

Route TCP (and optionally UDP) traffic through HTTP or SOCKS5 proxies:

```go
options := toxcore.NewOptions()
options.Proxy = &toxcore.ProxyOptions{
    Type:            toxcore.ProxyTypeSOCKS5,
    Host:            "127.0.0.1",
    Port:            9050,
    UDPProxyEnabled: true, // SOCKS5 UDP ASSOCIATE (RFC 1928)
}
```

| Proxy Type | TCP | UDP | Notes |
|------------|-----|-----|-------|
| `ProxyTypeHTTP` | ✅ | ❌ | HTTP CONNECT only |
| `ProxyTypeSOCKS5` | ✅ | ✅ (with `UDPProxyEnabled`) | RFC 1928 compliant |

### Delivery Retry

Configure automatic message retry with exponential backoff via `DeliveryRetryConfig`:

| Field | Default | Description |
|-------|---------|-------------|
| `Enabled` | `true` | Enable automatic retry |
| `MaxRetries` | `3` | Maximum retry attempts |
| `InitialDelay` | `5s` | Delay before first retry |
| `MaxDelay` | `5m` | Maximum delay between retries |
| `BackoffFactor` | `2.0` | Exponential backoff multiplier |

## Multi-Network Transport

toxcore-go routes traffic across multiple network types through the `transport/` package.

| Network | Listen | Dial | UDP | Implementation |
|---------|--------|------|-----|----------------|
| **IPv4/IPv6** | ✅ | ✅ | ✅ | `transport/ip_transport.go` |
| **Tor .onion** | ✅ | ✅ | ❌ | TCP via `go-i2p/onramp` (Tor integration) |
| **I2P .b32.i2p** | ✅ | ✅ | ❌ | SAM bridge via `go-i2p/onramp` |
| **Lokinet .loki** | ❌ | ✅ | ❌ | Dial-only via SOCKS5 (`transport/lokinet_transport_impl.go`) |
| **Nym .nym** | ❌ | ✅ | ❌ | Dial-only via SOCKS5 |

Address conversion between `net.Addr` and `transport.NetworkAddress`:

```go
import "github.com/opd-ai/toxcore/transport"

netAddr, err := transport.ConvertNetAddrToNetworkAddress(addr)
fmt.Println(netAddr.Type.String()) // "IPv4", "Onion", "I2P", etc.
fmt.Println(netAddr.IsPrivate())   // true for RFC 1918 ranges
```

See [docs/NETWORK_ADDRESS.md](docs/NETWORK_ADDRESS.md) and
[docs/MULTINETWORK.md](docs/MULTINETWORK.md) for protocol details.

## Noise Protocol Integration

The Noise-IK pattern provides forward secrecy, KCI resistance, and mutual authentication
for peer-to-peer connections. Implemented via `flynn/noise` v1.1.0.

```go
import (
    "github.com/opd-ai/toxcore/crypto"
    "github.com/opd-ai/toxcore/transport"
)

keyPair, _ := crypto.GenerateKeyPair()
udpTransport, _ := transport.NewUDPTransport("127.0.0.1:8080")
noiseTransport, _ := transport.NewNoiseTransport(udpTransport,
    keyPair.Private[:])
defer noiseTransport.Close()

noiseTransport.AddPeer(peerAddr, peerPublicKey[:])
noiseTransport.Send(packet, peerAddr)
```

Automatic per-peer version negotiation selects between legacy Tox protocol and
Noise-IK on a per-connection basis:

```go
capabilities := transport.DefaultProtocolCapabilities()
negotiating, err := transport.NewNegotiatingTransport(udp,
    capabilities, staticKey)
```

> **Security Warning**: Setting `EnableLegacyFallback: true` permits MITM downgrade
> attacks. Enable only for interoperability with legacy c-toxcore peers.

## Audio/Video Calls (ToxAV)

ToxAV provides peer-to-peer audio and video calling. Audio uses Opus encoding
(`opd-ai/magnum`, 48 kHz mono, 64 kbps default in VoIP mode). Video uses VP8
encoding (`opd-ai/vp8`) with both I-frames (key frames) and P-frames (inter
frames) for bandwidth-efficient video. RTP transport is handled by `pion/rtp`.

```go
toxav, err := toxcore.NewToxAV(tox)
if err != nil {
    log.Fatal(err)
}
defer toxav.Kill()

// Handle incoming calls
toxav.CallbackCall(func(friendNumber uint32, audioEnabled,
    videoEnabled bool) {
    toxav.Answer(friendNumber, 64000, 500000)
})

// Initiate a call (audio bitrate, video bitrate in bps)
toxav.Call(friendNumber, 64000, 1000000)

// Send and receive audio/video frames
toxav.AudioSendFrame(friendNumber, pcm, sampleCount, channels, rate)
toxav.VideoSendFrame(friendNumber, width, height, y, u, v)

toxav.CallbackAudioReceiveFrame(func(friendNumber uint32, pcm []int16,
    sampleCount int, channels uint8, samplingRate uint32) {
    // Process received audio (PCM samples)
})

toxav.CallbackVideoReceiveFrame(func(friendNumber uint32,
    width, height uint16, y, u, v []byte,
    yStride, uStride, vStride int) {
    // Process received video (YUV420 format)
})

// Both Tox and ToxAV require iteration
for tox.IsRunning() {
    tox.Iterate()
    toxav.Iterate()
    time.Sleep(tox.IterationInterval())
}
```

Call lifecycle management requires three additional APIs not shown above:

```go
// React to call state changes (ringing, active, ended, peer rejected, etc.)
toxav.CallbackCallState(func(friendNumber uint32, state av.CallState) {
    // state is an enum value, not a bitmask
    switch state {
    case av.CallStateEnded, av.CallStateError:
        // End/cleanup the call here when the call is finished
    case av.CallStateActive:
        // Media is flowing
    case av.CallStateRinging:
        // Incoming/outgoing ringing
    }
})

// End a call, mute, or pause from your side
toxav.CallControl(friendNumber, toxcore.CallControlCancel)

// Adjust bitrates at runtime (adaptive bitrate hints from the peer)
toxav.AudioSetBitRate(friendNumber, 32000)  // 32 kbps
toxav.VideoSetBitRate(friendNumber, 500000) // 500 kbps

toxav.CallbackAudioBitRate(func(friendNumber uint32, bitRate uint32) {
    // Peer recommends switching to bitRate bps for audio
    toxav.AudioSetBitRate(friendNumber, bitRate)
})
toxav.CallbackVideoBitRate(func(friendNumber uint32, bitRate uint32) {
    // Peer recommends switching to bitRate bps for video
    toxav.VideoSetBitRate(friendNumber, bitRate)
})
```

See [examples/ToxAV_Examples_README.md](examples/ToxAV_Examples_README.md) for
complete audio/video examples.

## Asynchronous Offline Messaging

An unofficial Tox protocol extension that stores messages for offline friends on
distributed storage nodes. All messages maintain end-to-end encryption and forward
secrecy. This system is enabled by default (`AsyncStorageEnabled = true`).

```go
// Automatic: SendFriendMessage falls back to async when friend is offline
err := tox.SendFriendMessage(friendID, "Message for when you're back online.")

// Receive offline messages
tox.OnAsyncMessage(func(senderPK [32]byte, message string,
    messageType async.MessageType) {
    fmt.Printf("Offline message from %x: %s\n", senderPK[:8], message)
})
```

### Privacy Properties

- **Sender anonymity** — Random, unlinkable pseudonyms per message (`async/obfs.go`)
- **Recipient anonymity** — Time-rotating pseudonyms on 6-hour epochs (`async/epoch.go`)
- **Forward secrecy** — One-time pre-keys consumed per message, auto-refreshed
  when fewer than 20 remain (`async/prekeys.go`, `async/forward_secrecy.go`)
- **Traffic analysis resistance** — Messages padded to 256B, 1024B, 4096B, or
  16384B buckets
- **Erasure coding** — Reed-Solomon encoding for storage redundancy (`async/erasure.go`)

### Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `MaxMessageSize` | `1372` bytes | Maximum async message payload |
| `MaxStorageTime` | `24h` | Message expiration on storage nodes |
| `MaxMessagesPerRecipient` | `100` | Per-recipient anti-spam limit |
| Storage allocation | 1% of disk (1 MB–1 GB) | Auto-updates every 5 minutes |

See [docs/ASYNC.md](docs/ASYNC.md), [docs/FORWARD_SECRECY.md](docs/FORWARD_SECRECY.md),
and [docs/OBFS.md](docs/OBFS.md) for protocol specifications.

## State Persistence

Save and restore the Tox instance state (private keys, friend list, name, status):

```go
// Save state to file
savedata := tox.GetSavedata()
err := os.WriteFile("tox_state.dat", savedata, 0600)

// Restore from saved state
savedata, err := os.ReadFile("tox_state.dat")
if err == nil {
    tox, err = toxcore.NewFromSavedata(nil, savedata)
} else {
    tox, err = toxcore.New(toxcore.NewOptions())
}
```

Alternatively, pass savedata through `Options`:

```go
options := toxcore.NewOptions()
options.SavedataType = toxcore.SaveDataTypeToxSave
options.SavedataData = savedata
tox, err := toxcore.New(options)
```

The savedata contains private keys. Store it with restrictive file permissions
(`0600`) and consider application-level encryption.

Four convenience methods provide alternative persistence workflows:

- **`tox.Save() ([]byte, error)`** — equivalent to `GetSavedata()` but returns an error if serialization fails; prefer this for robust error handling.
- **`tox.Load(data []byte) error`** — restores state into an existing `Tox` instance without creating a new one (useful for hot-reload scenarios).
- **`tox.SaveSnapshot() ([]byte, error)`** — saves a point-in-time snapshot including ephemeral session state; suitable for checkpointing during a running session.
- **`tox.LoadSnapshot(data []byte) error`** — restores a snapshot saved by `SaveSnapshot`; do not mix with `Load` (which expects `GetSavedata`/`Save` format).

## C API Bindings

The `capi/` package provides libtoxcore-compatible C function exports for both
toxcore and ToxAV. Building the C shared library requires cgo:

```bash
cd capi
go build -buildmode=c-shared -o libtoxcore.so .
```

The generated `libtoxcore.h` header and `libtoxcore.so` shared library can be
linked from C/C++ programs. See [capi/doc.go](capi/doc.go) for the exported
function list and usage patterns.

## Project Structure

```
toxcore-go/
├── toxcore.go             # Main API facade (Tox struct, New, NewFromSavedata)
├── toxav.go               # ToxAV audio/video calling API
├── options.go             # Options struct and defaults
├── doc.go                 # Package-level GoDoc documentation
├── async/                 # Offline messaging, forward secrecy, identity obfuscation
├── av/                    # ToxAV orchestration, signaling, adaptation
│   ├── audio/             # Opus codec, resampling, audio effects
│   ├── rtp/               # RTP packet handling, jitter buffer
│   └── video/             # VP8 codec, frame scaling, video effects
├── bootstrap/             # DHT bootstrap server (clearnet, Tor, I2P)
├── capi/                  # C API bindings (requires cgo)
├── crypto/                # Key management, encryption, signatures, secure memory
├── dht/                   # Kademlia DHT routing, node lookup, LAN discovery
├── docs/                  # Protocol specifications and design documents
├── examples/              # Example programs for all major features
├── factory/               # Packet delivery factory (simulation vs real)
├── file/                  # File transfer manager and transfer state
├── friend/                # Friend list, requests, connection tracking
├── group/                 # Group chat, sender keys, DHT replication
├── interfaces/            # Core abstractions (IPacketDelivery, INetworkTransport)
├── limits/                # Message size constants and validation
├── messaging/             # Message state machine, priority queue, padding
├── noise/                 # Noise IK/XX handshakes, PSK resumption
├── real/                  # Production network packet delivery
├── simulation/            # In-memory packet delivery for testing
├── testnet/               # Separate module with testnet tooling
├── toxnet/                # net.Conn, net.Listener, net.PacketConn implementations
└── transport/             # UDP, TCP, Noise, Tor, I2P, Lokinet, Nym, NAT traversal
```

## Documentation

Technical specifications and design documents are in the [docs/](docs/) directory:

- [ASYNC.md](docs/ASYNC.md) — Asynchronous messaging protocol
- [FORWARD_SECRECY.md](docs/FORWARD_SECRECY.md) — Epoch-based forward secrecy
- [OBFS.md](docs/OBFS.md) — Identity obfuscation
- [MULTINETWORK.md](docs/MULTINETWORK.md) — Multi-network transport architecture
- [NETWORK_ADDRESS.md](docs/NETWORK_ADDRESS.md) — Network address handling
- [SINGLE_PROXY.md](docs/SINGLE_PROXY.md) — TSP/2.0 proxy specification
- [DHT.md](docs/DHT.md) — DHT routing table design
- [TOR_TRANSPORT.md](docs/TOR_TRANSPORT.md) — Tor transport implementation
- [I2P_TRANSPORT.md](docs/I2P_TRANSPORT.md) — I2P transport via SAMv3
- [SECURITY_AUDIT_REPORT.md](docs/SECURITY_AUDIT_REPORT.md) — Security assessment
- [TOXAV_BENCHMARKING.md](docs/TOXAV_BENCHMARKING.md) — ToxAV performance benchmarks
- [CHANGELOG.md](docs/CHANGELOG.md) — Version history

See [docs/README.md](docs/README.md) for the full index.

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Ensure code passes formatting and static analysis:
   ```bash
   gofmt -l .
   go vet ./...
   ```
4. Run tests with race detection:
   ```bash
   go test -tags nonet -race ./...
   ```
5. Commit and push: `git push origin feature/my-feature`
6. Open a Pull Request

All code must pass `gofmt`, `go vet`, and `staticcheck` (enforced in CI).
Tests run with `-race` and `-tags nonet` to exclude network-dependent tests.

## License

MIT License — see [LICENSE](LICENSE) for details.