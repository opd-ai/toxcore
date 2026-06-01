# Implementation Gaps — 2026-06-01

These are gaps between what toxcore-go's README/docs claim and what the code actually
does, derived from the same audit that produced `AUDIT.md`. Each gap cross-references the
relevant AUDIT finding where one exists.

## G-01 — Gossip peer-exchange discovery is non-functional and IP-only

- **Stated Goal**: README ("Features → DHT Routing") promises "DHT-based peer discovery"
  including gossip/peer-exchange, and the multi-network feature claims discovery across
  `.onion`, `.b32.i2p`, `.nym`, and `.loki` networks. `dht/doc.go:109-112` and
  `dht/address_detection.go:70-73` reinforce multi-network node sharing.
- **Current State**: Two parallel SendNodes parsers exist. The **main DHT path**
  (`BootstrapManager.handleSendNodesPacket` → `processReceivedNodesWithVersionDetection`
  → `parser_integration.go`'s multi-network `PacketParser`) correctly handles the
  `[senderPK(32)][numNodes(1)][entries...]` format and supports extended address types.
  The **gossip path** (`GossipBootstrap.handleSendNodes`, `dht/gossip_bootstrap.go:244-269`),
  invoked on the *same* packet via `dht/handler.go:71`, (a) reads the node count from the
  wrong byte (`Data[0]`, a public-key byte, instead of `Data[32]`) and starts parsing at
  `offset = 1` instead of `33` — see AUDIT **H-01** — so it adds essentially no peers; and
  (b) its `parseNodeEntry`/`parseIPFromType` only understand IP types `2` (IPv4) and `10`
  (IPv6) (`gossip_bootstrap.go:273-330`), so even after the offset bug is fixed it cannot
  ingest `.onion`/`.i2p`/`.nym`/`.loki` nodes that the main path can.
- **Impact**: The advertised gossip peer-exchange acceleration provides no peers, and the
  gossip cache cannot learn non-IP (Tor/I2P/Lokinet/Nym) peers. Because the caller
  swallows the error as "supplemental" (`dht/handler.go:72-78`), the failure is silent;
  operators see slower/narrower peer discovery with no diagnostic.
- **Closing the Gap**: Fix the header offsets in `handleSendNodes` per AUDIT H-01
  (`len >= 33`, count at `Data[32]`, `offset = 33`), and route gossip node parsing through
  the same multi-network `PacketParser` used by `parser_integration.go` instead of the
  IPv4/IPv6-only `parseNodeEntry`. Add a test that round-trips a builder-produced SendNodes
  packet containing both an IPv4 node and an `.onion` node through the gossip path. Validate
  with `go test -race ./dht/...`.

## G-02 — VP8 video RTP packets are not RFC 7741-compliant

- **Stated Goal**: README ("Features → ToxAV Audio/Video") promises "VP8 video via
  `opd-ai/vp8`, RTP transport"; the source comment at `av/video/rtp.go:175` explicitly
  labels the descriptor "VP8 Payload Descriptor (RFC 7741)".
- **Current State**: `buildVP8Payload` (`av/video/rtp.go:176-202`) emits a 3-byte
  descriptor that writes the PictureID directly into bytes 1-2 and omits the RFC 7741
  extension octet (the `X|I|L|T|K|RSV` byte) and the PictureID `M`-bit 7-/15-bit selector.
  The matching depacketizer and `av/rtp/session.go` `deserializeVideoRTPPacket` read the
  same non-standard layout, so the serializer/deserializer agree internally — see AUDIT
  **M-01**.
- **Impact**: toxcore-go ↔ toxcore-go video calls work, but the on-wire VP8 RTP payload
  cannot be decoded by RFC 7741-compliant peers (other libtoxcore implementations, WebRTC
  gateways, standard analyzers). The code's own "RFC 7741" comment overstates compliance.
- **Closing the Gap**: Either (a) implement the true RFC 7741 descriptor (extension octet
  + correctly-sized PictureID with the `M` bit) in `buildVP8Payload` and its reader, or
  (b) if cross-stack interop is not a goal, correct the comment to state the format is a
  toxcore-internal descriptor, not RFC 7741. Add a fixed-vector test against an RFC 7741
  sample plus a round-trip test. Validate with `go test -race ./av/...`.

## G-03 — `crypto/key_rotation.go` deep-copy iterators are inconsistent about nil handling

- **Stated Goal**: README ("Cryptography") promises robust key management with "secure
  memory wiping"; the rotation manager keeps previous keys "for message backward
  compatibility" and is expected to behave safely as keys are rotated and wiped.
- **Current State**: `GetAllActiveKeys` (`crypto/key_rotation.go:114-115`) and
  `GetPreviousKeys` (`key_rotation.go:260-261`) defensively skip nil `*KeyPair` entries,
  but `FindKeyForPublicKey` (`key_rotation.go:138-143`) dereferences `key.Public` without
  the same guard — see AUDIT **L-03**. No nil currently reaches `previousKeys`
  (only the non-nil current key is appended at `key_rotation.go:75`, and `Cleanup` nils
  entries only under the exclusive write lock before discarding the slice), so this is a
  latent inconsistency rather than an active bug.
- **Impact**: Minimal today. The inconsistency means a future change that can introduce a
  nil entry would crash `FindKeyForPublicKey` while the sibling methods stay safe — an
  uneven defensive posture in security-critical code.
- **Closing the Gap**: Add `if key == nil { continue }` to `FindKeyForPublicKey` to match
  the sibling iterators. Validate with `go test -race ./crypto/...`.
