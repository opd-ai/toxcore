# toxcore-go — Documentation / Implementation Gaps

This file lists places where `README.md`, `doc.go`, `ROADMAP.md`, or `BACKLOG_ANALYSIS.md` describe the library as offering some capability that the **current source code does not, in fact, deliver at runtime**. Each gap is independently verifiable — the *Closing the Gap* sections are observations only; no source changes were made by the audit.

---

## G-1 — "Asynchronous offline messaging with forward secrecy" is broken at runtime

- **Stated Goal**
  - `README.md` (Features → *"Asynchronous Messaging"* and dedicated section, lines ~30–60 and ~300–360): describes offline message retrieval via `RetrieveAsyncMessages`, claims privacy via obfuscated pseudonyms.
  - `doc.go` (package overview): lists "asynchronous offline messaging" as a first-class supported feature.
  - `BACKLOG_ANALYSIS.md` summary near the top: declares the project passes `go test -race -tags nonet ./...`.
- **Current State**
  - `async/client.go::RetrieveObfuscatedMessages` (the implementation behind `RetrieveAsyncMessages`) deadlocks on first call because it holds `ac.mutex` write-locked while a transitively-called helper (`collectCandidateNodes`, line 994) re-acquires the same mutex with `RLock` (see AUDIT.md → C-ASYNC-1 for the full data flow and stack trace from the race test).
  - `go test -race -tags nonet ./async` therefore fails (`TestObfuscatedMessageRetrieval` times out), contradicting the BACKLOG claim of clean tests.
- **Impact**
  - Any process that calls the documented offline-retrieval entry point hangs that goroutine forever.
  - The async storage-node selection path is unreachable as documented, which in turn means the *forward secrecy*, *epoch rotation*, and *pseudonym-based privacy* features advertised in the README cannot be exercised in a real client.
- **Closing the Gap (observational)**
  - Either honour the existing `// Must be called while holding ac.mutex` contract on `collectCandidateNodes` (delete its self-acquired `RLock`), or refactor `RetrieveObfuscatedMessages` to snapshot the storage-node map under a short lock and run the rest of the algorithm without holding the mutex. The README should also stop claiming clean `-race` results until CI for the `async` package is restored.

---

## G-2 — "Noise PSK 0-RTT session resumption" is non-functional

- **Stated Goal**
  - `noise/psk_resumption.go` package comment and exported `PSKHandshakeConfig` (lines ~1–80, ~340–395) describe an IKpsk2 0-RTT profile for fast reconnection.
  - The README's "Security architecture" section advertises Noise-IK including PSK options.
  - `BACKLOG_ANALYSIS.md` lists the PSK feature as production-ready (no open items against it).
- **Current State**
  - `noise/psk_resumption.go::WriteMessage`, on the responder side, calls `psk.state.ReadMessage(...)` (line 493) and the underlying `flynn/noise` AEAD step fails: `chacha20poly1305: message authentication failed`.
  - `TestPSKHandshakeInitiatorResponder` (in `noise/psk_resumption_test.go`) reproduces this on every run.
- **Impact**
  - Any caller that opts into PSK 0-RTT resumption will fail every handshake. Production code paths that fall back to plain IK will succeed (so this is not a complete outage of the noise package), but the advertised resumption feature is a no-op at best and a connection-killer at worst.
- **Closing the Gap (observational)**
  - The most likely root cause is `createPSKNoiseConfig` (line 417) passing an Ed25519-shaped key as Noise's `StaticKeypair.Private`; Noise needs the X25519 DH keypair, so initiator's `PeerStatic` cannot match the responder's derived public. This should be verified by adding an in-process integration test (initiator + responder in the same goroutine pair) before re-advertising the feature.

---

## G-3 — Architectural rule "never use concrete `net.*` types" is violated in the very subsystems that document it

- **Stated Goal**
  - Custom instructions / project rule (also restated near the top of `doc.go` and in `README.md` *"Networking Best Practices"*): *"Never use `net.UDPAddr`, `net.IPAddr`, or `net.TCPAddr`. Use `net.Addr` only instead. … Never use a type switch or type assertion to convert from an interface type to a concrete type."*
- **Current State**
  - The DHT and transport subpackages — i.e. the canonical examples cited as "see `transport/network_transport.go`" — themselves construct concrete `net.UDPAddr` values:
    - `dht/local_discovery.go`, `dht/mdns_discovery.go` (multicast)
    - `transport/socks5_udp.go` lines 328, 339, 359, 571, 581, 595 (SOCKS5 UDP-associate parser returns `&net.UDPAddr{…}`)
    - `transport/stun_client.go` lines 322, 337, 358 (STUN response parser)
- **Impact**
  - Functional behaviour is correct today because the values are immediately returned through `net.Addr` interfaces.
  - However, mock-substitution of these subsystems (one of the cited reasons for the rule) is currently blocked by the internal concrete construction.
  - The documentation and the code disagree, which costs contributor trust and slows reviews.
- **Closing the Gap (observational)**
  - Either soften the rule to "no concrete `net.*` types **at API boundaries**", documenting that internal construction is permitted; or extend the `transport/address.go` helpers and route every site through a single factory (`transport.NewUDPAddress`) that returns `net.Addr`.

---

## G-4 — README/BACKLOG claim "all tests pass with race detector under `-tags nonet`" is currently false

- **Stated Goal**
  - `BACKLOG_ANALYSIS.md` summary: declares `go test -race -tags nonet ./...` passes cleanly.
  - `README.md` Testing section directs users to run `go test -tags nonet -race ./...` as the supported developer workflow.
- **Current State**
  - That command fails today on two packages:
    - `./async` (`TestObfuscatedMessageRetrieval` deadlock — see G-1).
    - `./noise` (`TestPSKHandshakeInitiatorResponder` AEAD MAC failure — see G-2).
  - All other packages (transport, dht, av/*, group, friend, file, crypto, toxnet, bootstrap, etc.) pass.
- **Impact**
  - First-time contributors following the README will believe their environment is broken.
  - CI relying on the claim will produce false positives or, if `nonet` is dropped, additional unrelated failures.
- **Closing the Gap (observational)**
  - Either update the README/BACKLOG to reflect the known failures and link to AUDIT.md (C-ASYNC-1, H-NOISE-1), or fix the two underlying bugs so the documented invariant is restored.
