# Security Gaps — 2026-04-21

**Repository:** `opd-ai/toxcore`
**Companion to:** [`AUDIT.md`](AUDIT.md)
**Scope:** Discrepancies between the project's stated security goals and the actual security controls present in the source. Each gap identifies what the project claims, what currently exists, the risk if the gap is exploited, and specific controls needed to close it.

---

## GAP-1 — Forward Secrecy Not Wired Into Async Message Send Path

- **Stated Goal**: README §"Asynchronous Offline Messaging" and `async/doc.go` state: "Forward secrecy via one-time pre-keys", "pre-key bundle rotation", and "epoch-based forward secrecy". The `ForwardSecureMessage` type has an `EncryptedData` field explicitly named to imply the payload is encrypted with a forward-secret session key.
- **Current State**: `async/client.go:295-312` sets `EncryptedData: message` where `message` is the **padded plaintext**. The comment on line 312 reads: `// In production, this would be encrypted with forward secrecy`. The surrounding infrastructure (`async/prekeys.go`, `async/forward_secrecy.go`, `ForwardSecurityManager`) exists and is tested, but is never invoked from `SendAsyncMessage`. Encryption of the inner payload occurs only at the outer `ObfuscatedAsyncMessage` layer using a static Diffie-Hellman shared secret — this provides confidentiality but **not forward secrecy**.
- **Risk**: Compromise of either party's long-term Curve25519 private key (obtainable from an unencrypted savedata file) retroactively decrypts all past async messages. An adversary who records encrypted traffic today can decrypt it after obtaining keys in the future, defeating a primary security promise of the library.
- **Closing the Gap**: In `SendAsyncMessage`, call `ForwardSecurityManager.SendMessage(recipientPK, message)` instead of constructing a `ForwardSecureMessage` manually. This uses a one-time pre-key from the recipient's pre-key bundle, performs an X3DH-style key agreement, and populates `EncryptedData` with ciphertext under a per-message ephemeral key that is deleted after use.

---

## GAP-2 — Noise Handshake Replay Protection Does Not Survive Process Restart

- **Stated Goal**: `transport/doc.go:52` states: "Handshake negotiation with replay protection via nonce tracking". `crypto/doc.go:74` documents `NonceStore` as a persistence mechanism for replay protection across restarts.
- **Current State**: `transport/noise_transport.go:104` stores the nonce deduplication set as `usedNonces map[[32]byte]int64` — a plain in-memory Go map that is initialized empty on every `NewNoiseTransport` call (line 191). `crypto.NonceStore`, which provides disk persistence and cross-restart protection, is never instantiated or called by `NoiseTransport`. After a process restart, the replay window is reset to empty; a 5-minute-old handshake packet captured before the restart is indistinguishable from a fresh one.
- **Risk**: An active on-path attacker who captures a Noise-IK initiation packet can replay it to the responder within `HandshakeMaxAge` (5 minutes) of any process restart. The attacker forces the responder to accept a session based on the captured ephemeral key, enabling a man-in-the-middle or session-confusion attack within that window.
- **Closing the Gap**: Pass a data directory into `NewNoiseTransport` and instantiate a `crypto.NonceStore` there. Replace all insertions into `usedNonces` with calls to `nonceStore.CheckAndStore(nonce, timestamp)` and remove the in-memory map. However, do **not** assume this alone makes replay protection survive crashes: the `NonceStore.CheckAndStore` method does **not** persist to disk immediately — `save()` is only called synchronously during `Close()`. Nonces accepted between the last successful save and a crash will be lost. To reliably close the gap, `NoiseTransport` must either (a) call `nonceStore` periodically via a background goroutine that triggers `save()` at a short interval (e.g., every 30 seconds), or (b) explicitly document that crash recovery only preserves nonces up to the last flush and size the `HandshakeMaxAge` window accordingly. `NonceStore` still provides concurrent-access safety, TTL cleanup, and atomic disk writes on save — it is the correct foundation for this design.

---

## GAP-3 — Savedata Contains Unencrypted Private Key with No Enforcement of Secure Storage

- **Stated Goal**: `doc.go:149` example shows `os.WriteFile("tox.save", data, 0600)` but provides no commentary on why 0600 is required. `GetSavedata()` doc comment (toxcore.go:409) says the data "should be stored securely as it contains cryptographic keys" — advisory only.
- **Current State**: `GetSavedata()` returns a JSON blob where `KeyPair.Private [32]byte` is serialized by `json.Marshal` in base64 encoding with no encryption. The library enforces no file permissions, no passphrase protection, and no warning beyond a doc comment. The binary `marshalBinary()` path also embeds the raw private key (toxcore_persistence.go:63-64). No finalizer wipes the returned `[]byte` when it is GC'd.
- **Risk**: Any process with read access to the save file — a different user, a world-readable umask, a bug in the application, or a log-aggregation service — obtains the Tox identity private key in cleartext. With the private key an adversary can: impersonate the user to all Tox contacts, read all historic async messages encrypted to this identity (once the forward secrecy gap is also present), and add themselves as a trusted friend silently.
- **Closing the Gap**: Provide an `EncryptSavedata(passphrase []byte) []byte` API that wraps the raw savedata with Argon2id + AES-GCM (matching the already-implemented `crypto.EncryptedKeyStore`). Deprecate the unencrypted `GetSavedata` for identity-bearing uses. Add a `SecureSavedata` option to `Options` that requires a passphrase on `New()` and `Load()`. As a minimum, zero the returned byte slice using a finalizer or document an explicit `SecureWipe(data)` call in every code path.

---

## GAP-4 — Incoming File Transfer Filename Not Restricted to Base Component

- **Stated Goal**: `file/doc.go:93` and `file/transfer.go:29` document `ErrDirectoryTraversal` and `ValidatePath` as the mechanism to "prevent directory traversal attacks".
- **Current State**: `file/transfer.go:184-201` implements `ValidatePath` which: (1) calls `filepath.Clean`; (2) checks `strings.Contains(cleanedPath, "..")` to reject relative traversal; (3) for absolute paths, iterates components and rejects any `..` part. However, an absolute path with no `..` components — such as `/etc/cron.d/evil`, `/home/user/.ssh/authorized_keys`, or `/var/spool/cron/root` — passes all checks. The network-received filename is used as-is, and `os.Create(fileName)` will write to that absolute path if the process has permission.
- **Risk**: A Tox friend (who must be manually added, so they are semi-trusted) sends a `PacketFileRequest` with a crafted absolute filename. If the receiving application accepts the transfer, the incoming file bytes are written to the attacker-specified path. This can overwrite shell init files, cron jobs, SSH authorized_keys, or application config files, leading to persistent code execution under the victim's account.
- **Closing the Gap**: Change `ValidatePath` to **reject all absolute paths** (`return "", ErrDirectoryTraversal` when `filepath.IsAbs(cleanedPath)`). In `handleFileRequest`, additionally strip the incoming filename to its base component only: `fileName = filepath.Base(fileName)`. Provide a `SetDownloadDirectory(dir string)` method on `Manager` that constrains all incoming file saves to that directory, and make this a required configuration step before `OnFileRecv` is callable.

---

## GAP-5 — Async Message Sender Identification Requires Manual Allowlist — Silent Delivery Failure

- **Stated Goal**: README §"Asynchronous Offline Messaging" states that the system delivers offline messages to the correct recipient and that the sender is authenticated. `async/client.go` comment at line 1188 says "In a production system, this would iterate through a contact list".
- **Current State**: `tryDecryptFromKnownSenders` (line 1183) tries decryption only against entries in `ac.knownSenders` — a map that is never populated automatically from the Tox friend list. If `knownSenders` is empty (the default), the function returns an error immediately and all received async messages are silently discarded. There is no error propagated to the caller, no log warning, and no documentation on how to populate the list.
- **Risk**: Applications that do not manually call the undocumented internal API to populate `knownSenders` will silently drop all incoming async messages. This is a reliability failure that could be mistaken for a security property (no messages received = attacker suppressing delivery), and a sophisticated attacker who can manipulate the contact list or the application state could silence a victim's inbox without any observable error.
- **Closing the Gap**: At `AsyncManager` initialization, automatically populate `knownSenders` from the `Tox` friend list by injecting a reference or callback. Provide a public `AsyncClient.AddKnownSender(publicKey [32]byte)` method. Log a `WARN` level message the first time `RetrieveObfuscatedMessages` is called with an empty `knownSenders` map. Document the requirement prominently in `async/doc.go`.

---

## GAP-6 — No Rate Limiting on Noise Handshake Processing — Memory Exhaustion Risk

- **Stated Goal**: The transport layer is designed to be robust against network-level adversaries (peer-to-peer, no centralized server, all peers potentially adversarial).
- **Current State**: `transport/noise_transport.go:796-807` inserts a nonce entry per handshake into `usedNonces` with no cap on map size. Cleanup fires every `NonceCleanupInterval` (10 minutes) and removes entries older than `HandshakeMaxAge` (5 minutes). Between cleanup runs the map can grow without bound. There is no per-source-IP rate limit, no token bucket, and no circuit breaker on `PacketNoiseHandshake` processing.
- **Risk**: An adversary who can send UDP packets to the node floods synthetic handshakes with fresh random nonces, each consuming ~40–100 bytes of map overhead. At 1 Gbps the map can exhaust available RAM within seconds; even at moderate rates (10 000 pkts/s) a 10-minute window fills ~40 MB, compounding with Go GC pressure.
- **Closing the Gap**: (a) Cap `usedNonces` at a configurable maximum (e.g., 100 000 entries) and evict the oldest entries when the cap is reached. (b) Add a per-source-address rate limiter (e.g., 10 handshakes/second per IP using `golang.org/x/time/rate`) before inserting into `usedNonces`. (c) Consider reducing `NonceCleanupInterval` to 1 minute to bound memory growth.

---

## GAP-7 — Pre-Key Bundle Encryption Keyed by Identity Key; WAL Entries Are Plaintext

- **Stated Goal**: `async/doc.go:193` lists "AES-GCM: Authenticated encryption for message payloads". The pre-key infrastructure (`async/prekeys.go`) is documented as providing forward secrecy. Key material on disk should be protected.
- **Current State**: `async/prekeys.go:319-342` marshals the pre-key bundle as JSON and then encrypts it with `encryptData(data, pks.keyPair.Private[:])` before writing to a `.json.enc` file — so bundle files themselves are encrypted at rest. However, two weaker points remain: (1) **Encryption key = identity private key**: the same long-term Curve25519 private key that protects the savedata also encrypts every pre-key bundle file. A single key compromise breaks savedata confidentiality and all pre-key bundle confidentiality simultaneously, collapsing the forward secrecy boundary. (2) **Legacy plaintext fallback**: `loadBundleFromDisk` (line 356-368) reads files without a `.enc` extension as raw plaintext JSON (backward compatibility path), meaning any legacy bundle files in the data directory are unprotected. (3) **WAL entries are plaintext**: `async/wal.go` writes `LogStoreMessage` and `LogDeleteMessage` entries as unencrypted binary data (line 296-318). The WAL contains the `data []byte` payload of stored messages — which may include message content — with no encryption layer.
- **Risk**: (1) Compromising the identity private key (e.g., via the unencrypted savedata) simultaneously decrypts all pre-key bundle files, undermining forward secrecy. (2) Legacy plaintext bundle files expose pre-key private keys directly. (3) An adversary with read access to the WAL file (same user, backup exfiltration, symlink attack) can read message payloads that have not yet been committed or checkpointed.
- **Closing the Gap**: (1) Derive the bundle encryption key from the identity key and a per-bundle salt using HKDF, so that exposure of a bundle key does not directly expose the identity key (separation of key material). (2) In `loadBundleFromDisk`, log a warning and trigger conversion for any unencrypted legacy file on load, rather than silently reading plaintext. (3) Encrypt WAL entries using a session-derived key (e.g., from `crypto.EncryptedKeyStore`) before calling `writeEntry`, so that the WAL file at rest does not expose message content.
