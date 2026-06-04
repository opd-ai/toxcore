// Package nameresolver provides a decentralised naming layer for Tox identities
// using the Namecoin blockchain (via the opd-ai/nmcd library).
//
// # Overview
//
// Tox IDs are 76-character hex strings that are difficult for humans to share
// reliably. This package resolves short human-readable Namecoin names of the form
//
//	tox/d/<username>
//
// to the corresponding Tox ID stored as a JSON value on-chain:
//
//	{"toxid": "<76-char hex>", "transports": [...]}
//
// Bootstrap node records are stored under:
//
//	tox/bootstrap/<tag>
//
// and carry a ToxID alongside the network address for cryptographic verification
// of bootstrap node identity.
//
// # Usage
//
// The package exposes a [Resolver] interface with two concrete implementations:
//
//   - [DisabledResolver]: a safe zero-value stub that returns
//     [ErrNameResolutionDisabled] for every call.  Used when name resolution is
//     not opted in via [toxcore.Options].
//
//   - [NmcdResolver]: the real implementation backed by a local bbolt name
//     database populated by the nmcd Namecoin library.  Create one with
//     [NewNmcdResolver].
//
// Applications that do not require name resolution incur no overhead — the
// default [DisabledResolver] is a no-op.
//
// # Name Validation
//
// Names must satisfy the pattern [a-z0-9_-]{1,63}.  Lookups against names that
// do not match this pattern always fail with [ErrInvalidName].
//
// # Record Expiry
//
// Namecoin name registrations expire after 36 000 blocks (≈ 250 days).  The
// resolver rejects records whose ExpiresAt block height has been passed, returning
// [ErrNameExpired].  Callers that cannot obtain the current chain height should
// pass a zero value for currentHeight; in that case expiry checking is skipped
// and a warning is logged.
package nameresolver
