// Package ratchet implements the Double Ratchet Algorithm as specified by
// Signal (https://signal.org/docs/specifications/doubleratchet/).
//
// The Double Ratchet combines two ratchets:
//
//   - Symmetric-key ratchet: derives a fresh message key for every single
//     message using HMAC-SHA-256, so compromise of one message key does not
//     expose adjacent keys.
//
//   - Diffie-Hellman ratchet: injects a new X25519 ephemeral DH output into
//     the root chain whenever the remote party advertises a new ratchet public
//     key in a message header, providing break-in recovery.
//
// # Key derivation
//
// Root chain KDF: HKDF-SHA-256(rootKey, dhOutput, "toxcore-dr-root")
// → (newRootKey [32]byte, chainKey [32]byte)
//
// Chain KDF: HMAC-SHA-256(chainKey, 0x01) → messageKey
//
//	HMAC-SHA-256(chainKey, 0x02) → newChainKey
//
// Message encryption: XChaCha20-Poly1305 with keys derived from
// HKDF-SHA-256(messageKey, "toxcore-dr-msg").
//
// # Backward compatibility
//
// The Double Ratchet is an optional upgrade layer.  Peers that have not
// established a ratchet session continue to use the existing NaCl-box
// transport encryption.  A Session is created explicitly via [InitInitiator]
// or [InitRecipient] after a successful key exchange (e.g., Noise-IK).
//
// # Thread safety
//
// A [Session] is safe for concurrent use by multiple goroutines.
package ratchet
