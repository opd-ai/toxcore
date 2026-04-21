# SECURITY AUDIT — 2026-04-21

## Project Security Profile

**Deployment model:** Pure-Go library consumed by application developers. No server process is exposed directly; the library exposes a UDP socket (and optionally TCP) for peer-to-peer communication. The main attack surface is the network — packets originate from remote Tox peers that may be adversarial.

**Trust boundaries:**
- **Untrusted:** Any data arriving over UDP/TCP from remote peers (DHT, bootstrap, file-transfer, async-messaging packets). Filenames, message content, node advertisements, and handshake material from peers are all untrusted.
- **Partially trusted:** Peers who have been mutually added as friends (still capable of malicious protocol messages).
- **Trusted:** The local filesystem and the application code embedding the library.

**Authentication model:** Curve25519 public keys serve as persistent peer identities. Noise-IK handshakes provide mutual authentication before session traffic is exchanged. DHT routing uses S-Kademlia proof-of-work node IDs.

**Data sensitivity:** The library stores long-term Curve25519 private keys (identity), friend lists (social graph), and offline messages on disk. All network traffic is authenticated+encrypted (NaCl box / ChaCha20-Poly1305 / AES-GCM / Noise-IK).

**Stated security goals:** End-to-end encryption, forward secrecy via epoch-based pre-key rotation, identity obfuscation against storage nodes, replay protection via nonce tracking, resistance to KCI attacks via Noise-IK.

---

## Security Surface Inventory

| Package | HTTP Handlers | DB Queries | Exec Calls | File I/O | Crypto | Auth |
|---------|:---:|:---:|:---:|:---:|:---:|:---:|
| `toxcore` (root) | 0 | 0 | 0 | Savedata R/W | Curve25519, box | ✅ keypair |
| `async` | 0 | 0 | 0 | WAL, prekeys, storage | AES-GCM, HKDF, HMAC | ✅ |
| `crypto` | 0 | 0 | 0 | KeyStore, NonceStore | Curve25519, ChaCha20, AES-GCM, Ed25519, Argon2id | ✅ |
| `dht` | 0 | 0 | 0 | 0 | S-Kademlia PoW, subtle | ✅ |
| `file` | 0 | 0 | 0 | Incoming file writes | none | ⚠️ path validation gap |
| `noise` | 0 | 0 | 0 | 0 | Noise-IK/XX | ✅ |
| `transport` | 0 | 0 | 0 | 0 | AES-GCM nonces | ✅ |
| `bootstrap` | 0 | 0 | 0 | 0 | hex key decode | ✅ |
| `capi` | 0 | 0 | 0 | 0 | delegates to crypto | ✅ |
| `av/*` | 0 | 0 | 0 | 0 | none | n/a |
| `examples/*` | 0 | 0 | 0 | example files | math/rand (sim only) | n/a |
| `cmd/gen-bootstrap-nodes` | 0 | 0 | 1 (gofmt) | generated bootstrap output | none | n/a |
| `factory` | 0 | 0 | 0 | 0 | none | n/a |

---

## Dependency Vulnerability Check

Dependencies checked against the GitHub Advisory Database (ghsa) as of 2026-04-21:

| Dependency | Version | Status |
|---|---|---|
| `github.com/flynn/noise` | v1.1.0 | ✅ No known CVEs |
| `github.com/go-i2p/onramp` | v0.33.92 | ✅ No known CVEs |
| `github.com/pion/rtp` | v1.8.22 | ✅ No known CVEs |
| `github.com/sirupsen/logrus` | v1.9.4 | ✅ No known CVEs |
| `golang.org/x/crypto` | v0.48.0 | ✅ No known CVEs |
| `golang.org/x/net` | v0.50.0 | ✅ No known CVEs |
| `github.com/cretz/bine` (Tor) | v0.2.0 | ✅ No known CVEs |
| `github.com/klauspost/reedsolomon` | v1.13.3 | ✅ No known CVEs |
| `github.com/stretchr/testify` | v1.11.1 | ✅ No known CVEs (test-only) |

`go vet ./...` passes with zero warnings (verified 2026-04-21). No hardcoded tokens, API keys, or connection strings were found in non-test, non-example source files.

---

## Findings

### HIGH

- [ ] **Incoming file transfer path validation permits arbitrary absolute paths** — `file/manager.go:340,354` + `file/transfer.go:184-201` — **Data flow:** A malicious peer (who has been accepted as a Tox friend) sends a `PacketFileRequest` with `filename = "/home/user/.bashrc"` or `"/etc/cron.d/evil"`. `deserializeFileRequest` (line 581) decodes the raw bytes from the network packet and returns this string as `fileName` with no sanitization. `handleFileRequest` creates a `Transfer` with `NewTransfer(…, fileName, …)` and fires the `OnFileRecv` callback. If the application calls `FileAccept`, eventually `Transfer.Start()` → `validateAndSanitizePath()` → `ValidatePath(t.FileName)`. `ValidatePath` calls `filepath.Clean` and checks for `".."` in the result, then for absolute paths it checks every path component for `".."` — but since `filepath.Clean` has already resolved `..` away, an absolute path like `/etc/cron.d/evil` passes all checks (no `..` component remains). The function returns the absolute path as "safe", and `os.Create("/etc/cron.d/evil")` is called, writing attacker-controlled bytes to an attacker-controlled path on the local filesystem. **Impact:** A trusted Tox friend can overwrite arbitrary files that the running process has permission to write (e.g., shell init files, cron jobs, SSH `authorized_keys`, config files), potentially achieving persistent code execution on the host. **Remediation:** In `ValidatePath`, reject all absolute paths unconditionally (return `ErrDirectoryTraversal` when `filepath.IsAbs(cleanedPath)`). Incoming file transfers should save to a user-configured download directory only; strip the filename to its base component with `filepath.Base(path)` before creating the transfer.

### MEDIUM

- [ ] **ForwardSecureMessage inner payload is not encrypted — missing forward secrecy** — `async/client.go:295-312` — **Data flow:** `SendAsyncMessage` pads the plaintext message and then assigns it directly: `EncryptedData: message // In production, this would be encrypted with forward secrecy`. The `ForwardSecureMessage` with this raw-plaintext `EncryptedData` is serialized (gob) and passed to `CreateObfuscatedMessage`, which wraps it in AES-GCM using a key derived from the Diffie-Hellman shared secret between the two parties' **static long-term** Curve25519 keys. There is no use of one-time pre-keys for this encryption step. The code acknowledges this incompleteness at line 295: `// For now, create a basic structure that demonstrates the obfuscation flow`. **Impact:** Forward secrecy is absent in the async path. If a peer's long-term static private key is compromised (e.g., via `GetSavedata()` exposure), an adversary who recorded past AES-GCM ciphertext can decrypt all historical async messages. The pre-key infrastructure (`async/prekeys.go`, `async/forward_secrecy.go`) exists but is not wired into `SendAsyncMessage`. **Remediation:** Integrate `ForwardSecurityManager` into `SendAsyncMessage`: fetch a one-time pre-key for the recipient, derive a session key using a 3-DH or X3DH-style exchange, encrypt `message` with that session key, and populate `EncryptedData` with the resulting ciphertext.

- [ ] **Noise handshake nonce store is in-memory only — replay window after restart** — `transport/noise_transport.go:104,796-807` — **Data flow:** The `NoiseTransport.usedNonces` map (line 104) is populated by `validateHandshakeNonce` (line 796-807) when a responder processes an incoming IK handshake. This map is never persisted to disk; it is initialized as an empty `map[[32]byte]int64` on construction (line 191). The cleanup goroutine removes nonces older than `HandshakeMaxAge` (5 minutes) every 10 minutes, but on process restart the map is empty. A `HandshakeMaxAge` of 5 minutes means an attacker who records a valid handshake packet can replay it within 5 minutes of a process restart. `crypto.NonceStore` (`crypto/replay_protection.go`) provides persistent nonce storage but is **not used** by `NoiseTransport`. **Impact:** An on-path attacker who captures a fresh handshake can replay it to the responder immediately after the process restarts (e.g., after a crash or update), potentially establishing a session using a captured ephemeral key exchange message before the peer generates a fresh one. **Remediation:** Instantiate a `crypto.NonceStore` in `NewNoiseTransport`, pass its data directory as a constructor parameter, and replace the in-memory `usedNonces` map with calls to `nonceStore.CheckAndStore(nonce, timestamp)`.

- [ ] **GetSavedata returns private key in unencrypted JSON with no at-rest protection** — `toxcore.go:415-440` + `toxcore_persistence.go:16-26` — **Data flow:** `GetSavedata()` builds a `toxSaveData` struct with `KeyPair: t.keyPair` where `KeyPair.Private [32]byte` has no `json:"-"` tag, so `json.Marshal` serializes the raw private key as a base64-encoded string. The returned `[]byte` contains the full identity private key in plaintext JSON. No file-system permissions are enforced by the library — enforcement is left to the application. The doc comment says "stored securely" but provides no mechanism. Example code in `toxav_integration/main.go:776` uses `os.WriteFile(saveDataFile, savedata, 0o600)` (correct), but the core library does not mandate this. **Impact:** Any process or user with read access to the save file (e.g., via world-readable umask, `/tmp` misuse, or log aggregation) obtains the Tox identity private key and can impersonate the user, read all saved encrypted state, and decrypt any past async messages sent to or from this identity. **Remediation:** (a) Encrypt the savedata blob using the `crypto.EncryptedKeyStore` (Argon2id + AES-GCM) before returning it, requiring a user-supplied passphrase; or (b) add a `SavedataKey` option to `Options` so the library encrypts savedata with a caller-supplied key; and (c) add a zero-on-GC finalizer to the `toxSaveData` struct and ensure the JSON byte buffer is wiped after use.

- [ ] **Async message decryption cannot identify senders without explicit allowlist — functional break** — `async/client.go:1183-1209` — **Data flow:** `RetrieveObfuscatedMessages` → `decryptRetrievedMessages` → `decryptObfuscatedMessage` → `tryDecryptFromKnownSenders`. The function at line 1206 iterates `ac.knownSenders` to trial-decrypt with each known public key. If `ac.knownSenders` is empty (the default for a newly constructed `AsyncClient`), line 1196 returns an error: `"no known senders configured"`, and **all received messages silently fail decryption**. There is no UI or API warning. The comment at line 1188 acknowledges: `"In a production system, this would iterate through a contact list"`. **Impact:** An application that does not manually populate `knownSenders` via the undocumented internal field will receive zero messages via the async path, making offline delivery non-functional. A social-engineering attack (convincing a user to clear contacts) could silence the user's inbox without any error surfacing. **Remediation:** Wire the friend list from the main `Tox` instance into `AsyncClient.knownSenders` automatically via the `AsyncManager`, document the requirement, and log a WARNING when `RetrieveObfuscatedMessages` is called with an empty sender list.

### LOW

- [ ] **Unbounded in-memory nonce map enables DoS via handshake flooding** — `transport/noise_transport.go:104,806` — **Data flow:** Each call to `validateHandshakeNonce` for a new nonce inserts an entry into `usedNonces` (line 806). Cleanup (`performNonceCleanup`) fires every 10 minutes (line 897-907) and removes entries older than 5 minutes. Between cleanup cycles an adversary can flood synthetic handshake packets with fresh random nonces, each inserting a 40-byte map entry (32-byte key + 8-byte int64). At line rate on a 1 Gbps link the map could grow unbounded during the 10-minute cleanup window. There is no rate limit on handshake processing (no `time.Ticker`, token bucket, or per-source limit). **Impact:** Memory exhaustion causing OOM kill or severe latency. **Remediation:** Cap `usedNonces` to a bounded size (e.g., 100 000 entries) with LRU or FIFO eviction, or implement per-source-IP rate limiting on incoming `PacketNoiseHandshake` packets.

- [ ] **Received file size field is not bounded — misleading application metadata** — `file/manager.go:581-606` — **Data flow:** `deserializeFileRequest` reads `fileSize` as a raw `uint64` from the network (line 593: `binary.BigEndian.Uint64(data[4:12])`) with no upper bound. A peer can advertise `fileSize = 2^64-1` (18 EB). The `Transfer` is created with this inflated size and reported to the application via the `OnFileRecv` callback. Progress UI and storage-capacity checks in the application layer will see an incorrect size. **Impact:** Applications that pre-allocate buffers or check disk space based on `fileSize` could allocate incorrectly or bypass storage checks. No DoS within the library itself, but application-layer logic may be confused. **Remediation:** Enforce a maximum advertised file size (e.g., matching the `MaxEncryptionBuffer` constant or a configurable limit) in `deserializeFileRequest`.

- [ ] **PBKDF2 legacy key retained in memory alongside Argon2id key** — `crypto/keystore.go:23,85` — **Data flow:** `NewEncryptedKeyStore` unconditionally derives both an Argon2id key AND a PBKDF2-SHA256 key (line 85: `legacyKey := pbkdf2.Key(…, PBKDF2Iterations, …)`). Both are stored as fields in `EncryptedKeyStore` for the process lifetime. The legacy key uses 100 000 PBKDF2 iterations with SHA-256, which is weaker than Argon2id against dedicated hardware. **Impact:** The weaker PBKDF2 key in memory is an additional attack surface if the process is memory-dumped. If a v1-format key file is ever decrypted using the legacy key, the key material is handled by a weaker KDF. **Remediation:** Derive the legacy PBKDF2 key only when actually needed for reading a v1 file, wipe it immediately after decryption, and do not retain it in the struct.

- [ ] **math/rand used in example code for simulation (not security-sensitive, but misleading)** — `examples/av_quality_monitor/main.go:13` — `math/rand` is imported and used for simulating network packet loss and jitter (functions `simulatePacketTransmission`, `calculateJitterVariation`). This is not security-sensitive since the randomness is purely for simulation display, but developers copying this example into production code could inadvertently use `math/rand` in security contexts. **Remediation:** Add a comment at the import noting `math/rand` is intentionally used here for non-security simulation; use `crypto/rand` for any cryptographic use.

- [ ] **Proxy password stored in plaintext struct field for reconnection** — `transport/proxy.go:30-31,77` — The `ProxyTransport.password` field stores the SOCKS5/HTTP proxy password as a plain `string` for use in reconnection (`transport/socks5_udp.go:234`). The password is never wiped. **Impact:** Memory dump or GC heap scan reveals the proxy credential. This is a low-level operational security concern, not an exploitable vulnerability in the protocol itself. **Remediation:** Store the credential in a `[]byte` and call `crypto.ZeroBytes` on it after establishing the UDP association, replacing with a re-derivation mechanism or forcing re-prompt on reconnect.

---

## False Positives Considered and Rejected

| Candidate Finding | Reason Rejected |
|---|---|
| `ValidatePath` allows `filepath.Clean("foo/../etc/passwd")` → `/etc/passwd` — could be a traversal | After `filepath.Clean`, `../etc/passwd` produces `../etc/passwd` which DOES contain `..` and IS rejected. The absolute-path bypass is the real finding (HIGH above); this specific traversal variant is caught. |
| `async/obfs.go` AES-GCM nonce reuse risk | Each message generates a fresh 12-byte nonce via `crypto/rand` (line 203). Per-epoch key rotation further limits the number of encryptions per key. No reuse vector confirmed. |
| `crypto/shared_secret.go` logs `peer_key_prefix` (first 8 bytes of public key) | The public key is not secret; logging its prefix is standard for debugging correlation without privacy impact. Not a finding. |
| `bootstrap/nodes/default_nodes.go` hardcoded bootstrap public keys | These are public DHT node identities, not secrets. They are equivalent to well-known DNS resolver IPs. Not a finding. |
| `os.Getenv` for `TOR_CONTROL_ADDR`, `I2P_SAM_ADDR`, etc. | These are non-secret network addresses, not credentials. No security impact. Not a finding. |
| `cmd/gen-bootstrap-nodes/main.go` uses `os/exec` | The tool does call `exec.Command("gofmt", "-w", outPath)` at line 94 to format its generated output. This is not a finding in context because it is a local developer code-generation utility (not production runtime code), the command name (`gofmt`) is a compile-time constant, and the sole argument (`outPath`) is a path constructed entirely by the tool itself from a fixed output directory — not from any external input. No user-controlled data flows into the exec call. |
| `toxcore_unit_test.go` contains `os/exec` usage | Test files only; not compiled into production library. Not a finding. |
| `crypto/replay_protection.go` `save()` not called during operation | The `usedNonces` map in `NonceStore` (for HMAC-based nonce tracking) is persisted on `Close()`. The real persistence gap is in `NoiseTransport.usedNonces` (separate map), which is the MEDIUM finding above. The `NonceStore` itself is correct. |
| `toxcore_persistence.go:marshalBinary` includes raw private key | The binary format is an alternative serialization for the same savedata. The security concern is the same as the JSON finding (MEDIUM above) and is not a separate issue. |
| S-Kademlia PoW difficulty — could be too low | `dht/skademlia.go` uses constant-time comparison for nonce/signature matching. PoW difficulty is a protocol-level parameter not within scope of this audit. No direct exploitability confirmed. |
