# Implementation Gaps — 2026-05-30

This document records gaps between what toxcore-go **claims** (README.md, ROADMAP.md,
GoDoc) and what the code **actually does**. It is the end-to-end companion to
`AUDIT.md`; finding references (C-/H-/M-/L-) point at that report. The most serious
gaps below are advertised features whose primary code path is non-functional.

## ToxAV audio/video media is never transmitted over RTP via the public API

- **Stated Goal**: README — "ToxAV Audio/Video — Peer-to-peer calling … RTP transport
  is handled by `pion/rtp` … adaptive bitrate, and jitter buffering". ROADMAP marks
  Audio (Opus) and Video (VP8) as ✅.
- **Current State**: `NewToxAV` passes a `*toxAVTransportAdapter` (`toxav.go:548`) to
  the AV manager. `Call.SetupMedia` only creates an RTP session when
  `transportArg.(transport.Transport)` succeeds (`av/types.go:669`), but the adapter
  implements `av.TransportInterface` (`Send(byte, []byte, []byte)`), whose method set
  is incompatible with `transport.Transport` (`Send(*Packet, net.Addr)` plus
  `Close`/`LocalAddr`/`IsConnectionOriented`). The assertion therefore **always
  fails**, `call.rtpSession` stays `nil`, and `isAudioProcessingReady`/
  `isVideoProcessingReady` (`av/manager.go:568,727`) skip every send and receive.
- **Impact**: Frames are encoded locally but never sent or received over the network
  through the standard `NewToxAV()` path. The headline calling feature is
  non-functional end-to-end even though local processing and callbacks appear to work.
- **Closing the Gap**: Give `av.TransportInterface` a media-send capability (or pass a
  real `transport.Transport` into `SetupMedia`) and make `SetupMedia`/send paths return
  an error when no RTP session can be created. Add a two-instance integration test that
  asserts a sent frame is received. See AUDIT.md **C-01**.

## DHT-published pre-key bundles can never be verified

- **Stated Goal**: README — "Forward secrecy — One-time pre-keys consumed per message
  … Sender/Recipient anonymity"; pre-key forward secrecy is marked ✅ in ROADMAP.
- **Current State**: `signBundle` signs with the 32-byte private key used as an
  Ed25519 *seed* (`crypto.Sign`, `crypto/ed25519.go:19`), but `validateBundle` verifies
  with `bundle.OwnerPK`, the **Curve25519** public key (`async/prekey_dht.go:373`).
  The Curve25519 public key is not the Ed25519 public key for that seed, so
  verification always fails and DHT pre-key retrieval cannot complete.
- **Impact**: The DHT distribution channel for one-time pre-keys is non-functional;
  peers cannot obtain each other's pre-keys this way, undermining the advertised
  forward-secrecy bootstrap.
- **Closing the Gap**: Verify against the Ed25519 public key derived from the signing
  seed (carry a signer-PK bound to `OwnerPK`, or sign a binding of `OwnerPK`). Add a
  publish→retrieve→validate round-trip test. See AUDIT.md **C-02**.

## Forward-secure async messages fail to decrypt over the network

- **Stated Goal**: README/ROADMAP — one-time pre-keys "consumed per message" with
  forward secrecy for offline delivery.
- **Current State**: Pre-keys are generated with random 32-bit IDs
  (`async/prekeys.go:168`), but the exchange packet serializes only public keys and
  the receiver rebuilds IDs as `0..n-1` (`async/manager.go:1018,1211`). A message later
  references `PreKeyID` (`async/forward_secrecy.go:475`) that the recipient resolves by
  ID against its random-ID store (`CheckAndMarkPreKeyUsed`), which cannot match.
- **Impact**: Forward-secure async messages that traverse the pre-key exchange packet
  path cannot be decrypted, breaking the offline-messaging forward-secrecy guarantee in
  practice. (Unit tests pass because they do not cross the serialize/deserialize
  boundary.)
- **Closing the Gap**: Serialize and parse each pre-key's real ID. Add a forward-secrecy
  round-trip test through the packet path. See AUDIT.md **C-03**.

## Robustness against malformed/abusive peer traffic is incomplete

- **Stated Goal**: README emphasizes a serverless, peer-to-peer design with a strong
  security posture in which every node parses untrusted DHT/transport packets.
- **Current State**: Several parsers panic or allocate without bounds on attacker
  input: nil-address panic on a malformed `send_nodes` entry (`dht/handler.go:408`),
  `uint16` length wrap → slice panic in relay deserialization
  (`dht/relay_storage.go:202`), and a ~4 GiB allocation from a TCP length prefix
  (`transport/tcp.go:448`). Multiple maps/sessions grow without caps
  (`dht/handler.go:152`, `dht/group_storage.go`, `dht/relay_storage.go`,
  `transport/noise_transport.go`, `transport/tcp.go`).
- **Impact**: A single malicious peer can crash a node (panic) or exhaust its memory/FDs
  (DoS), which contradicts the resilience expected of a P2P node exposed to the open
  internet.
- **Closing the Gap**: Add bounds/nil checks before slicing and allocation, enforce a
  protocol maximum packet size, and cap/evict all peer-keyed maps and sessions. See
  AUDIT.md **C-04, C-05, C-06, H-09–H-13, M-21**.

## Self name/status changes do not reach the correct friend

- **Stated Goal**: A clean, libtoxcore-compatible API; `SelfSetName`/
  `SelfSetStatusMessage` are public APIs whose changes are expected to propagate to
  connected friends.
- **Current State**: The broadcast packets hard-code friend ID `0` as a "self
  placeholder" (`toxcore_self.go:173,199`), and the receiver applies the update to its
  own local friend `0` (`toxcore.go:936`) rather than resolving the sender.
- **Impact**: Remote name/status updates are mis-applied to the wrong contact and never
  to the actual sender, so peer-visible profile changes are effectively broken.
- **Closing the Gap**: Embed the sender's public key (as friend requests do) and resolve
  it to the local friend ID on receipt. See AUDIT.md **H-02**.

## Adaptive bitrate and call-quality metrics are advertised but inert

- **Stated Goal**: README — "adaptive bitrate, and jitter buffering"; quality
  monitoring callbacks (`CallbackAudioBitRate`/`CallbackVideoBitRate`).
- **Current State**: `BitrateAdapter.UpdateNetworkStats` is never called from
  `Manager.Iterate` (`av/adaptation.go:246`, `av/manager.go:1655`), and the RTP session
  never tracks `PacketsLost`/`Jitter`/`Bandwidth` (`av/rtp/session.go:419`), which the
  quality classifier consumes (`av/quality.go:307`). Jitter thresholds also
  misclassify (`av/quality.go:391`).
- **Impact**: Automatic bitrate adaptation does not run, and quality reports are
  permanently "healthy" regardless of real loss/jitter, so the advertised adaptivity
  has no effect under degraded networks.
- **Closing the Gap**: Drive per-call adapters from real RTP statistics during
  iteration and implement RFC 3550 loss/jitter tracking. See AUDIT.md **M-10, M-11,
  M-12**.

## State persistence silently drops some friend state, and bad savedata regenerates identity

- **Stated Goal**: README — "Save and restore the Tox instance state (private keys,
  friend list, name, status)"; `SaveDataTypeSecretKey` restores an identity.
- **Current State**: `cloneFriendEntry`/`GetFriends` omit `IsTyping` and
  `DisappearingMessages` (`toxcore_persistence.go:365`, `toxcore_friends.go:235`), so
  disappearing-message settings are lost across save/load. Separately,
  `SaveDataTypeSecretKey` with any length other than 32 bytes silently calls
  `GenerateKeyPair()` (`toxcore.go:441`), creating a new identity instead of failing.
- **Impact**: Round-tripped friend state is incomplete, and a malformed secret-key blob
  silently destroys the user's identity and friend relationships rather than erroring.
- **Closing the Gap**: Persist all friend fields, and return an error for an
  invalid-length `SaveDataTypeSecretKey`. See AUDIT.md **H-03, M-13**.

## Lokinet & Nym transports are dial-only (no inbound listening)

- **Stated Goal**: README explicitly lists "Lokinet `.loki` (dial-only)" and
  "Nym `.nym` (dial-only)"; ROADMAP marks both ⚠️ "Dial only (SDK immature)".
- **Current State**: Matches the documentation — `NymTransport.Listen` returns
  `ErrNymNotImplemented` and Lokinet is dial-only via SOCKS5. This is an honestly
  documented limitation, not a defect.
- **Impact**: Peers cannot receive inbound connections over Lokinet/Nym, limiting fully
  symmetric P2P operation over those mixnets.
- **Closing the Gap**: Integrate inbound service-provider support and drop the
  "dial-only" qualifier once `Listen` is implemented. No code change is required to make
  the documentation accurate today.

## C API coverage is partial (~79% of libtoxcore functions)

- **Stated Goal**: README — "C API Bindings — libtoxcore-compatible C function exports";
  ROADMAP records "63 functions (~79% coverage)".
- **Current State**: A substantial but incomplete subset of the libtoxcore C surface is
  exported in `capi/`; roughly one in five upstream functions is unbound. The README is
  accurate about this.
- **Impact**: C clients written against the full libtoxcore surface may fail to link
  against the missing entry points; drop-in compatibility is not complete.
- **Closing the Gap**: Enumerate the libtoxcore symbol set, add the remaining exports
  with matching signatures, and cover them with `go test ./capi/...`.

## No automated dependency-vulnerability scanning

- **Stated Goal**: README emphasizes a strong cryptographic and secure-transport
  posture.
- **Current State**: There is no `govulncheck` gate, and this audit could not query the
  vulnerability feed (`vuln.go.dev` is unreachable from the sandbox). Pinned versions
  include `golang.org/x/crypto v0.48.0` and `golang.org/x/net v0.50.0`.
- **Impact**: Known upstream CVEs in security-sensitive dependencies could go unnoticed
  during normal development. (No specific advisory was confirmed reachable in this run —
  recorded with explicit uncertainty.)
- **Closing the Gap**: Add `govulncheck ./...` to CI with network access and enforce an
  upgrade policy for flagged dependency ranges.
