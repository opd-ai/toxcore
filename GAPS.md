# Implementation Gaps — 2026-05-27

This document records places where the project's stated goals (from `README.md`,
`doc.go`, and package-level GoDoc) diverge from what the code actually does. Each gap
links back to the corresponding `AUDIT.md` finding where the underlying bug is recorded.

## Replay protection silently rejects messages from clock-skewed peers
- **Stated Goal**: README and `crypto/replay_protection.go:1-15` package documentation
  describe a nonce store that detects replays by remembering recently-used nonces.
- **Current State**: `NonceStore.CheckAndStore` (`crypto/replay_protection.go:96-149`)
  rejects **any** nonce whose timestamp falls outside a ±5-minute window *before*
  consulting the store. Three of the package's own tests
  (`TestNonceStoreExpiration`, `TestNonceStoreCleanupLoop`,
  `TestNonceStoreWithTimeProvider`) fail under `go test -race ./crypto/...` because
  they expect the "old" nonce to be stored. Production callers cannot distinguish a
  true replay from a skew-rejected legitimate message.
- **Impact**: A peer whose wall clock has drifted more than 5 minutes (very common on
  mobile devices that suspend / resume across timezone boundaries, or on freshly-booted
  IoT devices that have not yet completed NTP sync) will see **every** message it
  emits silently dropped, with only a warning log on the receiver. The Tox
  README markets "DHT-based serverless" operation — but serverless deployments are
  exactly where clock drift is hardest to control.
- **Closing the Gap**: See `AUDIT.md` F-CRYPTO-001. Either (a) surface skew rejection as
  a distinct error sentinel so callers can prompt re-sync, or (b) store the nonce with
  its declared timestamp and let the existing 6-minute expiry bound memory growth.
  In either case, fix the failing tests so they reflect the chosen contract.

## Asynchronous-messaging pseudonym uniqueness contract is unverified
- **Stated Goal**: README ("identity obfuscation via epoch-based pseudonyms"),
  `async/doc.go`, and `async/async_test.go:576-578` all describe each offline message
  as carrying a unique per-message ephemeral key whose derived
  `RecipientPseudonym` cannot be linked to other messages for the same recipient.
- **Current State**: `TestRetrieveMessagesByPseudonym` retrieves three messages when
  querying by the *first* message's pseudonym, indicating that either (i) all three
  messages share the same pseudonym (privacy regression — a storage node trivially
  links them), or (ii) `pseudonymIndex` is keyed in a way that aliases distinct
  pseudonyms together (privacy regression — same outcome).
- **Impact**: The forward-secrecy story in the README rests on storage nodes being
  unable to fingerprint recipients across messages. This gap undermines the headline
  privacy claim for offline messaging.
- **Closing the Gap**: See `AUDIT.md` F-ASYNC-001. Determine which behaviour is the
  intended one by inspecting `CreateObfuscatedMessage` and the
  `pseudonymIndex` write path; then either fix the obfuscation derivation or rewrite
  `async/doc.go` and the test to reflect the actual (weaker) guarantee.

## Offline messages queued for offline friends are not delivered when the friend reconnects
- **Stated Goal**: README ("store-and-forward delivery through distributed storage
  nodes"), `async/manager.go:824` comment ("unblocks any sendQueuedMessages call
  waiting for friendPK's pre-keys").
- **Current State**: `TestQueuedMessagesSentAfterPreKeyExchange` fails because
  `sendQueuedMessages` re-queues all messages when no `preKeyReadyCh` was previously
  registered for the friend (`async/manager.go:865-924`). There is no further trigger
  that retries delivery, so the messages remain stuck in `pendingMessages` until the
  program restarts.
- **Impact**: Every offline message sent before the friend's pre-key channel is
  registered is effectively lost. This is the primary documented use case of the
  `async/` package.
- **Closing the Gap**: See `AUDIT.md` F-ASYNC-002. Either lazily register the pre-key
  channel inside `sendQueuedMessages` when it is missing, or defer the send loop until
  `signalPreKeyReady` runs.

## File transfers can leak file descriptors on write errors
- **Stated Goal**: README ("Bidirectional chunked file transfers with pause, resume,
  cancellation"). Implicit promise: a transfer that fails is cleaned up automatically.
- **Current State**: `Transfer.writeDataToFile` (`file/transfer.go:530-545`) sets the
  transfer to `TransferStateError` and fires the completion callback, but never closes
  `t.FileHandle`. The handle stays open for the remainder of the process.
- **Impact**: A hostile or buggy peer that triggers repeated chunk-write errors can
  exhaust the receiver's file-descriptor limit, denying service to other transfers,
  network connections, and storage layers.
- **Closing the Gap**: See `AUDIT.md` F-FILE-001. Mirror the cleanup already present in
  `Cancel` (line 461) and `complete` (line 656) on the write-error branch.

## `BUG:` annotations in production crypto code
- **Stated Goal**: README emphasises security as a primary concern; `doc.go` and the
  package documentation pages describe the crypto layer as the trust anchor.
- **Current State**: Six `BUG:` annotations remain in `crypto/logging.go`,
  `crypto/shared_secret.go`, `toxav.go`, and `toxcore_defaults.go`. Each is flagged as
  `critical` by `go-stats-generator`'s documentation scanner. The annotations describe
  logging in hot paths and a known potential information leak via debug logging.
- **Impact**: The lingering tags create audit noise and signal unfinished work in
  exactly the place users look to assess project maturity.
- **Closing the Gap**: See `AUDIT.md` F-DOC-001. Either remove the hot-path logging
  (preferred — it is `debug`-level and contributes nothing in production) or restate
  the comments as `// NOTE:` once the team has decided the logging stays.
