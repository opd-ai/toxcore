# Implementation Gaps — 2026-06-01

Gaps between what toxcore-go's README/GoDoc claims and what the code actually does. Each gap
cross-references the relevant `AUDIT.md` finding where one exists. This report supersedes the
previous `GAPS.md`; two prior gaps it listed (gossip header-offset bug, VP8 "RFC 7741"
descriptor) have since been fixed in the code and are noted as resolved.

## G-01 — File transfer is non-functional between two toxcore-go instances

- **Stated Goal**: README ("Features → File Transfers" and the "File Transfers" usage section)
  documents `tox.FileSend(...)`, `OnFileRecv`, `OnFileRecvChunk`, and `FileControl` as a
  supported, bidirectional feature.
- **Current State**: The public `Tox.FileSend` serializes the request with a 32-byte file-hash
  field (`createFileTransferPacketData`, `toxcore_file.go:141-172`) and sends it as
  `PacketFileRequest`, but the only handler for that packet type is
  `file.Manager.handleFileRequest` → `deserializeFileRequest` (`file/manager.go:585`), which
  expects a layout **without** the hash. The receiver reads the file-name length from two bytes
  of the hash, almost always rejecting the request (`ErrFileNameTooLong`) or mis-parsing it.
  See AUDIT **C-01 (CRITICAL)**. The only "round-trip" test asserts callback wiring, not actual
  packet parsing (`toxcore_integration_test.go:1736-1755`), so the defect is invisible to CI.
- **Impact**: The headline file-transfer feature does not work between two peers running this
  library. The failure is silent (the request is dropped or mis-parsed; no error surfaces to
  the sender's application beyond a successful local `FileSend` return).
- **Closing the Gap**: Unify the request wire format across `Tox.FileSend` and
  `file.Manager` (route the public API through `serializeFileRequest`, or extend
  `serialize`/`deserialize` to carry the hash symmetrically). Add a genuine cross-instance test:
  Peer A `FileSend` → assert Peer B's `OnFileRecv` fires with correct filename/size, then stream
  and checksum-verify the bytes. Validate with `go test -tags nonet -race ./file .`.

## G-02 — Gossip peer-exchange cannot ingest non-IP (Tor/I2P/Nym/Lokinet) peers

- **Stated Goal**: README advertises "DHT-based peer discovery" together with multi-network
  transport across `.onion`, `.b32.i2p`, `.nym`, and `.loki`; `dht/` documents node sharing
  across network types.
- **Current State**: The main DHT path parses extended (overlay) address types correctly, but
  the **gossip** path's `parseIPFromType` (`dht/gossip_bootstrap.go:311-330`) only understands
  IP type `2` (IPv4) and `10` (IPv6) and returns `unsupported IP type` for anything else. The
  header-offset bug that previously made the gossip parser ingest *zero* peers has been **fixed**
  (`handleSendNodes` now guards `len<33`, reads the count at `Data[32]`, and starts at
  `offset=33`, `dht/gossip_bootstrap.go:246-269`), so the gossip cache now works for IPv4/IPv6 —
  but it still silently drops Tor/I2P/Nym/Lokinet nodes the main path can learn. See AUDIT
  **L-01** for the related `sendNodes` count discrepancy.
- **Impact**: Gossip-accelerated discovery is narrower than advertised on overlay networks;
  peers reachable only via `.onion`/`.b32.i2p`/`.nym`/`.loki` are not propagated through the
  gossip cache. Because the caller treats gossip results as supplemental, the shortfall is silent.
- **Closing the Gap**: Route gossip node parsing through the same multi-network packet parser
  used by the main DHT path instead of the IPv4/IPv6-only `parseNodeEntry`/`parseIPFromType`.
  Add a test that round-trips a SendNodes packet containing both an IPv4 node and an `.onion`
  node through the gossip path. Validate with `go test -tags nonet -race ./dht`.

## G-03 — Overlay-network addresses do not round-trip (duplicated port)

- **Stated Goal**: README "Multi-Network Transport" promises dialing across Tor/I2P/Nym/Lokinet
  and address conversion between `net.Addr` and `transport.NetworkAddress`.
- **Current State**: `parsePrivacyNetworkAddress` stores the full `host:port` string in
  `NetworkAddress.Data` while also populating `Port` (`transport/address_parser.go:378-382`).
  Converting back via `toCustomAddr` re-appends the port (`transport/address.go:115-118`),
  producing `host:port:port`. See AUDIT **M-01 (MEDIUM)**.
- **Impact**: Overlay addresses that pass through parse→`ToNetAddr` (e.g. serialization,
  re-dialing) are malformed, breaking the documented multi-network dialing for the affected
  address types.
- **Closing the Gap**: Store only the host in `Data` so `Port` is authoritative; add
  parser→`ToNetAddr` round-trip tests per overlay type. Validate with
  `go test -tags nonet -race ./transport`.

## G-04 — Noise rollback/replay-protection layer is documented in code but inactive

- **Stated Goal**: README "Noise Protocol Integration" promises forward secrecy, KCI
  resistance, mutual authentication, and warns that legacy fallback "permits MITM downgrade
  attacks" — implying the non-legacy path defends against downgrade/rollback.
- **Current State**: The version-commitment mechanism intended to detect rollback is wired up
  but never exercised: `sendVersionCommitment` is unused and application data is sent without
  checking `versionCommitted` (`transport/noise_transport.go:681,695`); the inbound-handshake
  "replay" guard validates a locally-created nonce rather than an authenticated peer nonce
  (`noise_transport.go:607`). See AUDIT **M-04** and **L-02**. Core Noise-IK handshakes and
  interop still function — only the extra downgrade/replay-detection layer is inert.
- **Impact**: The advertised additional protection against version rollback / handshake replay
  is not actually enforced. This does not compromise the Noise session itself, but the codebase
  implies a guarantee it does not deliver.
- **Closing the Gap**: Either activate the commitment exchange (send after handshake, gate app
  data until the peer's commitment verifies) and bind replay detection to an authenticated peer
  nonce/timestamp, or remove the dormant code and the comments that imply enforcement. Validate
  with `go test -tags nonet -race ./transport`.

## G-05 — `toxnet` encryption cannot be enforced (best-effort mixed mode only)

- **Stated Goal**: README "Go net.* Interfaces" presents `toxnet` `net.Conn`/`net.PacketConn`
  implementations for "Tox communication", and the package exposes encryption (peer keys,
  `encryptPacket`/`decryptPacket`), implying confidential/authenticated datagrams.
- **Current State**: `ToxPacketConn.decryptPacket` returns the raw bytes as plaintext whenever
  the peer key is unknown or decryption fails (`toxnet/packet_conn.go:580-598`) — an
  intentionally documented "mixed encrypted/unencrypted" design. There is no mode that *requires*
  encryption, so an application cannot rely on confidentiality or authenticity. See AUDIT
  **M-05**. Separately, the stream read buffer is unbounded (AUDIT **M-03**).
- **Impact**: Callers expecting encrypted datagrams may silently receive forged or
  cleartext packets from unknown sources; there is no API to opt into strict encryption.
- **Closing the Gap**: Add an opt-in "encryption required" mode that drops packets from unknown
  peers and on decrypt failure, document the default best-effort behavior, and cap the stream
  read buffer. Validate with `go test -tags nonet -race ./toxnet`.

## G-06 — C API exported functions can crash the host instead of returning error codes

- **Stated Goal**: README "C API Bindings" presents `capi/` as a libtoxcore-compatible surface
  with `TOX_AV_ERR_*` status codes, implying robust error reporting to C callers.
- **Current State**: `getToxAVID` dereferences caller-supplied raw C pointers before validating
  them against the live-handle registry (`capi/toxav_c.go:368`), and `toxav_new` dereferences an
  unchecked `C.malloc` result (`capi/toxav_c.go:461-462`). Both turn recoverable error
  conditions (stale handle, OOM) into process crashes / panics across the cgo boundary. See
  AUDIT **H-01** and **M-09**.
- **Impact**: A C/C++ host passing an invalid handle, or running under memory pressure, crashes
  rather than receiving a `TOX_AV_ERR_*` code — contrary to the expectations a libtoxcore-style
  API sets.
- **Closing the Gap**: Validate handles against `liveToxAVHandles` before dereference; check the
  `malloc` result and return `TOX_AV_ERR_NEW_MALLOC` on failure (deleting the registry entry).
  Validate with `go test -tags nonet -race ./capi`.

## G-07 — Messaging "automatic retry" can silently strand messages queued before transport setup

- **Stated Goal**: README documents delivery retry with exponential backoff and async fallback
  ("When the friend is offline, messages automatically fall back to asynchronous
  store-and-forward delivery").
- **Current State**: A message processed while `MessageManager.transport == nil` is left in
  `MessageStateSending` and never returned to `Pending`, so the reprocessing/retry loop (which
  only handles `Pending`) skips it forever — despite an in-code comment claiming the message
  "stays Pending" (`messaging/message.go:1166-1178`). See AUDIT **M-07**. Incoming file/message
  chunk `position` handling is also order-sensitive (AUDIT **M-06**).
- **Impact**: In the (admittedly narrow) window where messages are enqueued before a transport
  is configured, those messages are silently lost rather than retried.
- **Closing the Gap**: Guard the `Sending` transition on a non-nil transport, or reset to
  `Pending` when the transport is nil. Validate with `go test -tags nonet -race ./messaging`.

---

### Resolved since the previous GAPS.md
- **Gossip SendNodes header offset** (was H-01): the byte-offset/count bug is fixed; the
  remaining limitation is multi-network ingestion only (now tracked as G-02).
- **VP8 "RFC 7741" descriptor** (was G-02/M-01): `buildVP8Payload` now emits the RFC 7741
  extension octet and 15-bit `M`-bit PictureID (`av/video/rtp.go:175-205`).
