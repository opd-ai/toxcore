# Implementation Gaps — 2026-05-25

This document lists places where the project's `README.md` (and adjacent published docs) describes a capability that is not fully present in the implementation. "Gap" here means a documented behavior that is either absent, partial, behind an unannounced caveat, or implemented in a way that quietly degrades on failure. Findings here cross-reference the corresponding entries in `AUDIT.md`; "Closing the Gap" describes the work needed to align code with documentation.

---

## Gap 1 — Async offline message delivery races a 500 ms timer instead of a synchronization primitive

- **Stated Goal**: README "Asynchronous Offline Messaging — store-and-forward delivery through distributed storage nodes with end-to-end encryption, forward secrecy via one-time pre-keys". `async/doc.go` further says messages are reliably delivered once the recipient's pre-key bundle is published.
- **Current State**: `async/manager.go:842` (`sendQueuedMessages`) drains queued messages and then calls `time.Sleep(500 * time.Millisecond)` to "wait briefly for pre-key exchange to complete" before sending. There is no signal that the pre-key exchange actually completed; on a slow network or busy host the queued messages will be sent into a still-incomplete state, return errors, and be discarded (only `log.Printf`'d).
- **Impact**: Users who send messages to an offline friend at the moment the friend comes back online may silently lose those messages on slow links — directly contradicting the "asynchronous offline messaging" promise. See `AUDIT.md` finding **H-001**.
- **Closing the Gap**: Replace the timer with a per-peer `chan struct{}` that the pre-key-exchange completion path closes; `sendQueuedMessages` should `select { case <-doneCh: ... case <-time.After(maxTimeout): }` and on failure re-queue rather than drop. Add an integration test that injects a 1-second pre-key publish delay and asserts queued messages are still delivered.

---

## Gap 2 — Noise-IK handshake responder test failure suggests the production responder may not accept all spec-conforming initiators

- **Stated Goal**: README "Noise-IK Handshakes — Noise Protocol Framework (IK and XX patterns) for forward secrecy, KCI resistance, and mutual authentication via `flynn/noise`" (`noise/`, `transport/noise_transport.go`). The negotiating transport (`transport/negotiating_transport.go`) is supposed to upgrade peers to Noise-IK when both sides support it.
- **Current State**: `go test -tags nonet -race ./transport/... -run TestHandshakePacketHandling` fails with `chacha20poly1305: message authentication failed` inside `noiseTransport.handleHandshakePacket`, and then panics because the test code does not guard the subsequent slice access. See `AUDIT.md` findings **H-002** and **M-003**.
- **Impact**: At minimum, CI for the `transport` package is red. At worst, a real initiator constructing a slightly different (but spec-conformant) handshake message will fall back to the legacy Tox encryption path, silently weakening the forward-secrecy and KCI-resistance properties the README promises.
- **Closing the Gap**: Trace the responder path through `handleHandshakePacket` → `noise.HandshakeState.ReadMessage`; reconcile prologue, static-key, and pattern selection with the test's initiator construction. Replace the panicking `t.Errorf`+index access in the test with `t.Fatalf`. Add a round-trip test that drives both initiator and responder through the production transport API (not the lower-level `noise` library) to detect future drift.

---

## Gap 3 — Traffic-analysis-resistant padding loses its property on a `crypto/rand` failure

- **Stated Goal**: README "automatic message padding (256B, 1024B, 4096B) to resist traffic analysis"; `docs/COVER_TRAFFIC.md:344` "All timing uses `crypto/rand`; no `math/rand` or `time.Now()` modulo arithmetic".
- **Current State**: `async/message_padding.go:64` calls `rand.Read(paddedMessage[originalLen+LengthPrefixSize:])` and ignores the error. On any `crypto/rand` failure, the padding bytes remain zero (from `make`), which an observer can statistically distinguish from a genuinely large message. See `AUDIT.md` finding **H-003**.
- **Impact**: A passive adversary with rare access to a host in an entropy-starved state could fingerprint short messages padded to a large bucket — exactly the threat the padding was designed to defeat.
- **Closing the Gap**: Check the error and return it (callers — `async/forward_secrecy.go`, `async/obfs.go` — already propagate errors from this function). Add a test that uses a fake reader returning an error to assert `PadMessageToStandardSize` fails closed rather than silently producing zero-padded output.

---

## Gap 4 — Encrypted keystore rotation can permanently destroy data on partial-rename failure

- **Stated Goal**: `crypto/doc.go` and the README's "Encrypted keystore" feature imply durable, atomic rotation of the master encryption key.
- **Current State**: `crypto/keystore.go:430-447` `reencryptWithNewKey` performs `os.Rename(tmpPath, finalPath)` for each file in turn; on the *k*-th failure it attempts to "roll back" earlier files by renaming the final-path file back to a `.reencrypt.tmp`. But that file already contains the *new-key* ciphertext — the original old-key file was overwritten by the rename. The in-memory key is then restored to `oldKey`, leaving the files unreadable. See `AUDIT.md` finding **M-001**.
- **Impact**: A transient filesystem error (Windows file lock, EXDEV across mounts, ENOSPC) part-way through a key rotation can permanently corrupt the keystore. The README does not warn users that key rotation is a non-recoverable operation.
- **Closing the Gap**: Stage all writes to side-files (`name + ".reencrypt.tmp"`), preserve the originals as `name + ".preencrypt.tmp"` *before* the final rename, then commit all renames; on failure, rename `.preencrypt.tmp` back. After successful salt-file rename, unlink the `.preencrypt.tmp` files. Add a fault-injection test that fails the third rename and asserts the original files remain readable with the old key.

---

## Gap 5 — Lokinet and Nym transports are "dial-only"; the README mentions this only in passing

- **Stated Goal**: README lists "Lokinet `.loki` (dial-only), and Nym `.nym` (dial-only)" as supported multi-network transports. The wording is brief.
- **Current State**: `docs/PRIVACY_NETWORK_QUICKSTART.md:167,196,438-439` and `transport/nym_transport_impl.go:16` confirm that Nym integration "requires the Nym SDK websocket client which is not yet implemented", and Lokinet listening is not implemented. The transports succeed at outbound dialling but cannot accept incoming connections.
- **Impact**: A user enabling Lokinet/Nym expecting symmetric inbound/outbound reachability will find their node is unreachable to other peers over those transports. Friend requests cannot be received over `.loki` / `.nym`.
- **Closing the Gap**: Either (a) implement listen-side support for both transports, or (b) keep dial-only and make the limitation more prominent in the README (a dedicated "Limitations" subsection in the Multi-Network Transport section). Until (a) lands, the public `*Transport.Listen` methods should return a typed `ErrListenNotSupported` rather than a generic error so callers can detect the gap programmatically.

---

## Gap 6 — Group chat DHT response collection is not implemented

- **Stated Goal**: README "Group Chat — DHT-based group chat with role-based permissions, peer-to-peer broadcasting, and sender key distribution".
- **Current State**: `dht/group_storage.go:17-19` exports `ErrGroupDHTNotImplemented = errors.New("group DHT response collection not yet implemented")` and the corresponding response-aggregation path returns this error.
- **Impact**: Group operations that depend on DHT-side fan-in (e.g. retrieving a freshly-published sender-key bundle from a peer who is not in your direct routing table) will fail with the sentinel error. The README does not currently warn users about this.
- **Closing the Gap**: Implement the response-collection path in `dht/group_storage.go` (mirror the pattern used in `async/prekey_dht.go` for prekey collection — a singleflight-coalesced `transport.Send` followed by a per-request response channel keyed by query nonce). Until implemented, mention in the README's Group Chat bullet that DHT-only peer discovery within groups is not yet supported.

---

## Gap 7 — C API contains stub functions that silently no-op

- **Stated Goal**: README "C API Bindings — libtoxcore-compatible C function exports for toxcore and ToxAV; requires cgo (`capi/`)". Implies that linked C clients can use any documented libtoxcore function.
- **Current State**: `capi/toxav_c.go:377, 382` (`tox_new`/`toxav_new` flow) and several `capi/toxcore_c.go` conference-related functions (lines 949, 1522, 1536, 1555-1557) use `_ = parameter` to suppress unused-variable warnings — they accept their arguments but perform no operation and return without populating the `*_err` out-parameter or returning a documented error code.
- **Impact**: A C client calling these will observe success (no error) but the requested operation did not happen. Hardest-to-debug class of integration bug.
- **Closing the Gap**: For each stub function, define and assign a `TOX_ERR_*_NOT_IMPLEMENTED` enum value via the existing `setError(err, …)` helper, so C callers receive an unambiguous "not implemented" indication. Mirror this fix in `AUDIT.md` finding **L-018**. Add a CI test (using a small C smoke-test program under `capi/test/`) that asserts each documented function either succeeds or sets a non-zero error code, never both silent + no-op.

---

## Gap 8 — `staticcheck.conf` is present but staticcheck is not enforced

- **Stated Goal**: Implicit — the project ships a `staticcheck.conf` in the repository root, signalling that staticcheck is part of the intended toolchain.
- **Current State**: `.github/workflows/toxcore.yml` runs `go vet ./...` and tests only; no `staticcheck` invocation. A contributor running `staticcheck ./...` locally may see findings that never block PRs.
- **Impact**: Documentation/tooling drift; quality bar is implicit rather than enforced.
- **Closing the Gap**: Either add a `staticcheck ./...` step to CI (preferred) or delete `staticcheck.conf` to avoid implying a stricter standard than is enforced.

---

## Gap 9 — `BACKLOG_ANALYSIS.md` and `ROADMAP.md` versus README — current vs. planned state is ambiguous

- **Stated Goal**: README lists features as if implemented. `BACKLOG_ANALYSIS.md` and `ROADMAP.md` exist alongside but their scope (current vs. planned vs. historical) is not labeled.
- **Current State**: A new contributor reading the README treats every bullet as available today; reading the ROADMAP makes them uncertain which items are aspirational. Several README features have known caveats (Gaps 5-7 above) that are not flagged at the bullet itself.
- **Impact**: Onboarding friction; users may architect around features that are partially implemented.
- **Closing the Gap**: Add a one-line scope preamble to `BACKLOG_ANALYSIS.md` and `ROADMAP.md` ("Planned future work, not yet implemented"). In the README, add a small ✅/⚠️ glyph next to each feature bullet where a known limitation exists, with a link to the corresponding section of `docs/`. Cross-link from this `GAPS.md` from the README's documentation index.

---

## Gap 10 — UDP datagram support is missing from privacy-network examples

- **Stated Goal**: README's Multi-Network Transport section lists IPv4/IPv6 UDP/TCP, Tor, I2P, Lokinet, Nym uniformly.
- **Current State**: `examples/privacy_networks/README.md:164` notes "❌ UDP datagrams not yet implemented" for the privacy-network combined example.
- **Impact**: Users exploring the multi-network feature via the example will be unable to test UDP-level interop across privacy networks.
- **Closing the Gap**: Either implement UDP-over-privacy-network in the example (using the existing `transport/socks5_udp.go` for SOCKS5 UDP-ASSOCIATE) or update the README's claim to distinguish per-transport TCP vs. UDP support.

---

## Gap 11 — Several `*net.UDPAddr` / `*net.TCPAddr` literals across `transport/` and `dht/` violate the README's own networking guideline

- **Stated Goal**: README's "Networking Best Practices" (also reproduced in `interfaces/doc.go:108` and `transport/doc.go:10`): "Never use concrete network types … Never use a type switch or type assertion to convert from an interface type to a concrete type."
- **Current State**: `transport/address.go:357-367`, `transport/network_detector.go:222-235`, `transport/reuseport_unix.go:53-123`, `transport/socks5_udp.go:328-602`, `transport/stun_client.go:322-365`, `transport/nat.go:441`, `dht/local_discovery.go:188,213,420`, `dht/mdns_discovery.go:167,179,509`, `dht/gossip_bootstrap.go:298`, `av/types.go:153`, `transport/hole_puncher.go:357`, and several `examples/*` use concrete `*net.UDPAddr` / `*net.TCPAddr` types or switch on them.
- **Impact**: Code-style drift; new contributors copy the pattern; mock transports must construct `*net.UDPAddr` values they would prefer to leave abstract.
- **Closing the Gap**: Most of these sites are unavoidable because the underlying syscalls (`syscall.Sendto`, `golang.org/x/net/ipv4.JoinGroup`) and external libs (Tor/I2P/SOCKS5) require concrete types. Amend the README guideline to read "use `net.Addr` in public APIs; concrete types are acceptable in internal call-sites only when an external dependency requires them," and add an inline comment at each unavoidable site referencing the exception. Audit `transport/hole_puncher.go:357 SimultaneousPunch` separately — it is a public method whose `*net.UDPAddr` parameter is gratuitous and should change to `net.Addr`. Cross-reference `AUDIT.md` findings **L-006** and **L-021**.

---

## Gap 12 — Pre-key bundle generation does not validate that `crypto/rand` succeeded before forming a key ID

- **Stated Goal**: README "forward secrecy via one-time pre-keys"; `async/doc.go` documents that pre-keys are uniquely identified per peer.
- **Current State**: `async/prekeys.go:92` correctly checks the error from `rand.Read(idBytes)`. This finding was investigated and rejected as a false positive (the error *is* checked), but the adjacent ignored-error pattern in `async/retrieval_scheduler.go:173` (see `AUDIT.md` **M-002**) and `async/message_padding.go:64` (see **H-003**) indicate inconsistent error-handling discipline across the `async` package.
- **Impact**: Per-site impact varies (covered in `AUDIT.md`). The systemic gap is that no automated lint forbids the `rand.Read(...)` / `rand.Int(...)` ignored-error pattern.
- **Closing the Gap**: Add a `staticcheck` rule (`SA4006` / `SA1000`-style) or a small `go vet` analyzer that fails CI when `crypto/rand` return values are discarded. Combine with Gap 8.

---

## Summary

The toxcore-go project broadly delivers what the README promises: the core DHT, friend, messaging, file-transfer, audio/video, async-messaging, and multi-transport layers exist and compile. The gaps above are concentrated in three areas: (1) **silent error swallowing on security-critical paths** (Gaps 3, 4, 12), (2) **timing-based synchronization that the docs imply is event-based** (Gap 1), and (3) **partially-implemented features that the README presents without caveat** (Gaps 5, 6, 7, 10). Closing Gaps 1-4 and 6 would materially strengthen the project's primary claims; the remaining gaps are documentation alignment that can be addressed by README edits and tooling additions.
