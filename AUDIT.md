# SECURITY AUDIT — 2026-06-03

## Project Security Profile

`toxcore-go` (`github.com/opd-ai/toxcore`, Go 1.25) is a **pure-Go library** implementing
the Tox peer-to-peer encrypted messaging protocol: DHT peer discovery, friend management,
1-to-1 and group messaging, file transfer, ToxAV audio/video, asynchronous offline messaging,
and multi-network transport (UDP/TCP, Tor, I2P, Nym/Lokinet dial-only). It also ships a cgo
C-API binding (`capi/`) and `net.*` adapters (`toxnet/`).

- **Deployment model:** Embedded library / P2P node. There is **no HTTP server, no SQL
  database, no HTML templating, and no web front end**. Within the **library runtime**, the
  only outbound HTTP client is the UPnP/SSDP NAT-traversal client
  (`transport/upnp_client.go`). The build-time code generator (`cmd/gen-bootstrap-nodes`)
  also performs outbound HTTP requests (`fetchNodeList`), but it is not part of the library
  runtime. The only `os/exec` call is also in that same code generator.
- **Trust boundaries:** All network peers are untrusted. The asynchronous-messaging design
  explicitly treats **storage/relay nodes as untrusted** — end-to-end encryption, forward
  secrecy (one-time pre-keys), and epoch pseudonyms exist precisely because relays may be
  malicious (`SECURITY.md` → "Security-Relevant Design Decisions"; `README.md` async section).
  Untrusted input enters via UDP/TCP datagrams, the async retrieve/store packet handlers, the
  file-transfer control/data packets, and the UPnP gateway's SSDP/XML responses.
- **Authentication model:** Curve25519 key exchange, Ed25519 signatures, ChaCha20-Poly1305 /
  AES-256-GCM AEAD, Noise-IK/XX handshakes (`flynn/noise`), pre-key bundle signing, and
  out-of-band safety numbers. There is no password/session/JWT/RBAC surface.
- **Data sensitivity:** Long-term identity private keys, ratchet/session keys, one-time
  pre-keys, and message plaintext. Key material is wiped via `crypto.ZeroBytes` /
  `crypto.SecureWipe` and optionally `mlock`-ed (`crypto/secure_memory.go`).
- **Stated security goals (`SECURITY.md`):** cryptographic correctness, protocol-level
  forward secrecy, memory-safety of key material, authentication-bypass resistance
  (friend-request / pre-key spoofing), and DoS resistance on core protocol state. The project
  self-describes as **experimental, pre-third-party-audit**.

Overall the codebase is **unusually well hardened** for its maturity: prior-audit finding
labels (`C-0x`, `H-0x`, `M-0x`, `L-0x`) are embedded in comments next to remediations, network
allocation sizes are bounded, constant-time comparisons are used for all secret material, and
`crypto/rand` is used universally. This audit found **no CRITICAL or HIGH** issues with a
demonstrable exploit path; the residual findings are DoS / defense-in-depth at MEDIUM and LOW.

## Security Surface Inventory

| Package | HTTP Handlers | DB Queries | Exec Calls | File I/O | Crypto | Auth |
|---------|--------------|------------|------------|----------|--------|------|
| `crypto` | 0 | 0 | 0 | keystore/replay files (trusted dir) | AEAD, Ed25519, X25519, HKDF, subtle | ✅ |
| `async` | 0 (Tox packet handlers) | 0 | 0 | pre-key/WAL files (`%x`-hex names) | AES-256-GCM+HKDF, gob/json codec | ✅ |
| `noise` | 0 | 0 | 0 | 0 | Noise-IK/XX via `flynn/noise` | ✅ |
| `transport` | UPnP SOAP **client** only | 0 | 0 | onion/i2p key dirs | version commitment HMAC, TLS n/a | ✅ |
| `ratchet` | 0 | 0 | 0 | 0 | Double Ratchet, AEAD | ✅ |
| `dht` | 0 (UDP handlers) | 0 | 0 | 0 | node-id crypto | ✅ |
| `file` | 0 (Tox packet handlers) | 0 | 0 | `os.Open`/`OpenFile` (path-guarded) | 0 | ✅ |
| `toxnet` | 0 (`net.PacketConn`) | 0 | 0 | 0 | per-peer packet AEAD | ✅ |
| `cmd/gen-bootstrap-nodes` | 0 | 0 | `gofmt` (build-time) | codegen output | 0 | n/a |
| `av`, `av/*` | 0 | 0 | 0 | 0 | SRTP/RTP | ✅ |

Scan results (non-test, non-example code):
- `os/exec` / `syscall.Exec`: **1** site — `cmd/gen-bootstrap-nodes/main.go:109`, a build-time
  code generator invoking the constant binary `gofmt`. Not reachable from runtime input.
- `database/sql`, `text/template`, `html/template`: **none**.
- `math/rand` (non-test): **0** in library code (one example, `examples/av_quality_monitor`,
  explicitly commented "simulation only, not security-sensitive").
- `crypto/md5`, `crypto/sha1`: **none**.
- `InsecureSkipVerify`: **none**.
- `//go:embed`: **none**.
- Hardcoded secrets (password/token/apikey/private_key literals): **none** found.
- `os.Getenv`: configuration only (proxy/service addresses such as SAM/Tor/Nym/Lokinet
  endpoints and `XDG_DATA_HOME`; tuning parameters `TOX_NETWORK_TIMEOUT`,
  `TOX_RETRY_ATTEMPTS`, `TOX_ENABLE_BROADCAST` in `factory/`; `GOPACKAGE` in
  `cmd/gen-bootstrap-nodes`); no secrets are read from the environment.
- Random generation: **`crypto/rand` everywhere** (keypairs, nonces, salts, message IDs,
  nospam, jitter) — verified in `crypto/`, `async/`, `noise/`, `transport/`.
- Secret comparisons: `subtle.ConstantTimeCompare` and `hmac.Equal` used for all keys,
  pseudonyms, and MACs (`crypto/constant_time.go`, `async/obfs.go:131,394`,
  `transport/version_commitment.go:151`, `async/client.go:1389`).

## Dependency Vulnerability Check

`govulncheck ./...` **could not complete** in this sandbox: outbound DNS to `vuln.go.dev` is
blocked (`dial tcp: lookup vuln.go.dev: no such host`). No offline vulnerability DB is bundled,
so an automated CVE cross-check was not possible here. Manual observations from `go.mod`:

- Crypto/transport dependencies are current: `golang.org/x/crypto v0.48.0`,
  `golang.org/x/net v0.50.0`, `golang.org/x/sys v0.41.0`, `flynn/noise v1.1.0`.
- No JWT, ORM, SQL-driver, YAML-as-config, or HTML-template dependency is present, removing
  entire CVE classes from scope.
- `gopkg.in/yaml.v3 v3.0.1` is present only as an indirect dependency (via `testify`); it is
  not used to parse untrusted runtime input in library code.

**Action (not a code change):** wire `govulncheck ./...` into CI where network egress to
`vuln.go.dev` is permitted, so dependency CVEs are caught on every build. See `GAPS.md`.

## Findings

### CRITICAL
- None identified with a demonstrable data flow.

### HIGH
- None identified with a demonstrable data flow. (The previously reported `toxnet`
  `RequireEncryption` plaintext-bypass is **remediated** in the current tree — see
  "Previously Reported — Re-verified as Fixed".)

### MEDIUM
- [x] **M-1 — Unbounded `gob` decode of retrieve responses from untrusted storage nodes**
  — `async/client.go:1267` (`handleRetrieveResponse`) → `async/client.go:1313`
  (`deserializeRetrieveResponse`) → `gob.Decode(&messages)` at `async/client.go:1320`.
  **Evidence (data flow):** an inbound `PacketAsyncRetrieveResponse` arrives from a storage
  node; `handleRetrieveResponse` passes `packet.Data` directly into
  `deserializeRetrieveResponse`, which constructs a `gob.NewDecoder` and decodes into
  `[]*ObfuscatedAsyncMessage` **with no prior length check and no cap on the decoded element
  count**. `encoding/gob` is documented as unsafe against adversarial input: a crafted payload
  can declare a large slice/element count and induce disproportionate allocation before the
  decode fails. Storage nodes are untrusted **by the project's own design** (forward-secrecy
  and obfuscation exist because relays may be malicious), and the only caller-side gate is
  `findResponseChannel` matching the sender address — satisfiable by the queried node itself or
  via UDP source-address spoofing. **Impact:** memory-pressure / allocation DoS on a client
  retrieving offline messages from a malicious or compromised relay. **Mitigating factor:** the
  UDP read buffer is fixed at 2048 bytes (`transport/udp.go:208`), bounding this vector on UDP;
  the same handler is reachable over TCP-relay transport, where frames are bounded at 1 MiB
  rather than 2 KiB, so the amplification headroom is larger there.
  **Remediation:** before decoding, reject oversized inputs with the existing helper
  `limits.ValidateProcessingBuffer(packet.Data)` (or a tighter async-specific bound), and cap
  the decoded slice length (e.g. reject once `len(messages)` exceeds a documented maximum such
  as the per-node retrieval batch size). Prefer the project's existing length-prefixed binary
  framing (as used in `async/storage.go` / `transport/*.go`) over `gob` for any network-facing
  decode path. **Validation:** add a unit test feeding `deserializeRetrieveResponse` a
  hand-crafted `gob` payload that declares an oversized count and assert it returns an error
  without large allocation; `go test ./async/...` and `go vet ./...` must stay green.

### LOW
- [x] **L-1 — UPnP control URL is not re-validated against the private-IP allowlist**
  — `transport/upnp_client.go:295-305` (`controlURL` construction) vs. the M-06 validation at
  `:137-180` (`validateUPnPLocationURL` / `isPrivateIP`).
  **Evidence (data flow):** the SSDP `LOCATION` (gateway) URL is correctly validated to use
  `http`/`https` and resolve to a private/LAN IP (M-06 fix). The client then fetches the
  gateway's device-description XML and extracts `<controlURL>` from that **network-controlled**
  response; `baseURL.Parse(controlPath)` is used to form `uc.controlURL`, which is **not**
  re-checked against `isPrivateIP`. Because `url.Parse` honours an absolute URL in
  `controlPath`, a malicious or spoofed gateway on the LAN can point the subsequent SOAP `POST`
  (`sendSOAPRequestWithResponse`, `:399`) at an arbitrary host. **Impact:** LAN-adjacent SSRF /
  request redirection from the NAT-traversal client; bounded by requiring an attacker already
  positioned on the local network (or able to forge the SSDP response). Redirects are already
  disabled (M-06, `:199`), which limits but does not eliminate this. **Remediation:** after
  resolving `controlURL`, run the same `isPrivateIP` / scheme check used for the LOCATION URL
  and reject control URLs whose host is not a private/loopback address; reuse
  `validateUPnPLocationURL` on the final `controlURL.String()`. **Validation:** unit-test
  `buildControlURL` (and its callers `tryExtractControlURL` / `parseDeviceDescription`) with
  an XML body whose `<controlURL>` is an absolute public-IP URL and assert it is rejected.

- [x] **L-2 — No automated dependency CVE gate in the build** — repository-wide (`go.mod`,
  CI). **Evidence:** `govulncheck` is not invoked anywhere reachable in this environment and
  could not be run (network blocked). **Impact:** a future CVE in `golang.org/x/crypto`,
  `golang.org/x/net`, `flynn/noise`, or a transitive dependency would not be flagged
  automatically. **Remediation:** add a `govulncheck ./...` step to the existing GitHub Actions
  workflow (`.github/workflows/`). **Validation:** the CI job fails on any known advisory.

## False Positives Considered and Rejected

| Candidate Finding | Reason Rejected |
|-------------------|-----------------|
| Command injection via `exec.Command` (`cmd/gen-bootstrap-nodes/main.go:109`) | Build-time code generator only; the binary name is the compile-time constant `"gofmt"`/`"gofmt.exe"` and arguments are a fixed flag plus a generator-controlled output path. Not reachable from any runtime/network input. |
| Insecure RNG via `math/rand` (`examples/av_quality_monitor/main.go:13`) | Example program, not library code; explicitly commented "simulation only, not security-sensitive." No security decision depends on it. |
| Path traversal in pre-key / file persistence (`async/prekeys.go:433,489`, `crypto/keystore.go`) | Filenames are `fmt.Sprintf("%x...", PeerPK)` (hex of a 32-byte key) or fixed constants joined to a trusted, process-owned data directory — never raw peer strings. |
| Path traversal in incoming file transfer (`file/transfer.go:255`) | Peer-supplied names are reduced with `filepath.Base` in `deserializeFileRequest` (`file/manager.go:606-637`), `ValidatePath` rejects absolute paths and `..` components (`file/transfer.go:205`), and pre-existing symlinks are refused before create (M-08, `file/transfer.go:251`). |
| Unbounded network allocation (`transport/relay.go:419`, `tcp.go:475`, `relay_mux.go:369`) | Each length is bounded before `make`: relay 64 KiB (`relay.go:414`), TCP 1 MiB (`tcp.go:469`), mux frame ≤ `MaxFrameSize` with over-limit frames drained (`relay_mux.go:320-328`). |
| AES-GCM "legacy" path uses `sha256(keyMaterial)` as the key (`async/secure_storage.go:97`) | Backward-compatibility decrypt fallback only; it still uses authenticated AES-256-GCM. New writes use HKDF-derived keys with a random per-message salt (`:18-59`). No downgrade is attacker-forceable on encrypt. |
| `gob` decode of `ForwardSecureMessage` (`async/client.go:1383,1469`) | Reached only **after** AEAD decryption of the payload (`decryptedPayload`); an attacker without the recipient key cannot supply controlled bytes to this decoder, so there is no untrusted-input data flow (unlike M-1, which decodes pre-decryption). |
| Weak hash usage | No `crypto/md5` or `crypto/sha1` anywhere; SHA-256 is used only for HKDF/legacy-KDF and integrity, not for password storage (there are no passwords). |
| `InsecureSkipVerify` / weak TLS | No `crypto/tls` client/server configuration in library code; transport confidentiality is provided by Noise/AEAD, not TLS. |

## Previously Reported — Re-verified as Fixed

- **`toxnet` `RequireEncryption` plaintext bypass (prior H-1).** `createPacketWithAddr` now
  returns `(packet, ok bool)` and, in strict mode, returns `false` on decryption failure
  (`toxnet/packet_conn.go:177-189`); `processIncomingPacket` only enqueues when `ok` is true
  (`toxnet/packet_conn.go:144-147`). Forged/plaintext datagrams are dropped in strict mode as
  documented. The prior `GAPS.md` "Gap 1" no longer reflects the code.
- **UPnP LOCATION SSRF (M-06).** Gateway LOCATION URLs are validated for scheme and private-IP
  host, and HTTP redirects are disabled (`transport/upnp_client.go:126-207`). (Residual
  derived-`controlURL` gap captured as L-1 above.)
- **File-transfer symlink write (M-08).** Pre-existing symlink destinations are refused before
  `os.OpenFile` (`file/transfer.go:251-255`).

## Metrics Note

`go-stats-generator analyze .` summarized 45,054 LoC across 27 packages (1,340 functions,
3,107 methods). The longest non-test functions are modest (≤ 77 LoC; e.g.
`checkForRiskyConfigurations` 77, `RatchetDecrypt` 48), so no security-relevant logic is buried
in oversized, hard-to-review functions. `go vet ./...` reported no issues.
