// Package noise provides Noise Protocol Framework implementations for secure
// cryptographic handshakes in the Tox protocol.
//
// This package implements two Noise handshake patterns using the formally verified
// flynn/noise library with ChaCha20-Poly1305 encryption, SHA256 hashing, and
// Curve25519 key exchange.
//
// # Pattern Selection Guide
//
// The package supports two handshake patterns with different security properties:
//
//	Pattern │ When to Use                                │ Security Properties
//	────────┼────────────────────────────────────────────┼────────────────────────────────────────
//	IK      │ Initiator knows responder's public key    │ Mutual auth, forward secrecy, KCI resist
//	XX      │ Neither party knows the other's key       │ Mutual auth, forward secrecy
//
// # IK Pattern (Initiator with Knowledge)
//
// Use IK when the initiator already knows the responder's static public key.
// This is the default pattern for Tox since friend public keys are known before
// connection attempts.
//
// Security properties:
//   - Mutual authentication: Both parties verify each other's identity
//   - Forward secrecy: Compromise of long-term keys doesn't expose past sessions
//   - Key Compromise Impersonation (KCI) resistance: Compromised key cannot be
//     used to impersonate others to the key owner
//   - Identity hiding: Initiator's identity protected from passive observers
//
// Message flow (2 round trips):
//
//	Initiator                              Responder
//	─────────                              ─────────
//	-> e, es, s, ss  (ephemeral, static)
//	                                       <- e, ee, se  (ephemeral)
//	[session established]
//
// Example usage:
//
//	// Initiator (knows peer's public key)
//	ik, err := noise.NewIKHandshake(myPrivKey, peerPubKey, noise.Initiator)
//	if err != nil {
//	    return err
//	}
//	msg, _, err := ik.WriteMessage(nil, nil)  // Create initial message
//	// Send msg to peer...
//	// Receive response...
//	payload, complete, err := ik.ReadMessage(response)
//	if complete {
//	    send, recv, _ := ik.GetCipherStates()
//	    // Use send/recv for encrypted communication
//	}
//
//	// Responder (doesn't need peer's key initially)
//	ik, err := noise.NewIKHandshake(myPrivKey, nil, noise.Responder)
//	payload, _, err := ik.WriteMessage(nil, receivedMsg)  // Process and respond
//	// Get peer's key after handshake
//	peerKey, _ := ik.GetRemoteStaticKey()
//
// # XX Pattern (Interactive Exchange)
//
// Use XX when neither party knows the other's static public key beforehand.
// This is useful for initial contact scenarios or public services where
// static keys haven't been pre-shared.
//
// Security properties:
//   - Mutual authentication: Both parties exchange and verify static keys
//   - Forward secrecy: Ephemeral keys protect past sessions
//   - No prior key knowledge required
//   - 3 message round trip (slower than IK)
//
// Message flow (3 round trips):
//
//	Initiator                              Responder
//	─────────                              ─────────
//	-> e           (ephemeral only)
//	                                       <- e, ee, s, es
//	-> s, se       (static exchange)
//	[session established]
//
// Example usage:
//
//	// Initiator
//	xx, err := noise.NewXXHandshake(myPrivKey, noise.Initiator)
//	if err != nil {
//	    return err
//	}
//	msg1, _, err := xx.WriteMessage(nil, nil)
//	if err != nil {
//	    return err
//	}
//	// Send msg1, receive response1
//	_, _, err = xx.ReadMessage(response1)
//	if err != nil {
//	    return err
//	}
//	msg2, complete, err := xx.WriteMessage(nil, nil)
//	if err != nil {
//	    return err
//	}
//	// Send msg2, handshake complete
//
//	// Responder
//	xx, err := noise.NewXXHandshake(myPrivKey, noise.Responder)
//	if err != nil {
//	    return err
//	}
//	_, _, err = xx.ReadMessage(msg1)
//	if err != nil {
//	    return err
//	}
//	response1, _, err := xx.WriteMessage(nil, nil)
//	if err != nil {
//	    return err
//	}
//	// Send response1, receive msg2
//	_, complete, err = xx.ReadMessage(msg2)
//	if err != nil {
//	    return err
//	}
//
// # Security Considerations
//
// Replay Protection: Each IKHandshake includes a unique 32-byte nonce accessible
// via GetNonce(). Applications should track used nonces to prevent replay attacks.
// The transport layer (transport/noise_transport.go) implements this tracking.
//
// Timestamp Validation: IKHandshake includes a Unix timestamp via GetTimestamp().
// Applications should validate handshake freshness. Recommended limits:
//   - Maximum age: 5 minutes (HandshakeMaxAge)
//   - Maximum future drift: 1 minute (HandshakeMaxFutureDrift)
//
// Key Verification: After successful handshake, verify the peer's identity using
// GetRemoteStaticKey(). Compare against known trusted keys or implement a trust-on-
// first-use (TOFU) model.
//
// Secure Memory: Private key material is automatically wiped from memory using
// crypto.ZeroBytes() after key derivation to minimize exposure window.
//
// # Cipher Suite
//
// All handshakes use:
//   - DH: Curve25519 (X25519 key exchange)
//   - Cipher: ChaCha20-Poly1305 (AEAD encryption)
//   - Hash: SHA256 (key derivation and authentication)
//
// This suite provides 128-bit security level and is resistant to timing attacks.
//
// # Thread Safety
//
// IKHandshake and XXHandshake instances are thread-safe. All public methods
// are protected by internal mutexes. However, a single handshake instance
// should typically only be used from one goroutine because the handshake
// protocol requires sequential message processing. The thread safety ensures
// that concurrent getter calls (IsComplete, GetNonce, etc.) do not race with
// ongoing handshake operations.
//
// The resulting CipherStates from GetCipherStates() are NOT thread-safe;
// concurrent encrypt/decrypt operations require external synchronization.
//
// # Error Handling
//
// Common errors returned by handshake operations:
//   - ErrHandshakeNotComplete: Operation requires completed handshake
//   - ErrInvalidMessage: Received message is invalid for current state
//   - ErrHandshakeComplete: Handshake already finished, cannot process more messages
//
// # Integration with Transport Layer
//
// The transport package wraps noise handshakes for Tox communication:
//
//	import "github.com/opd-ai/toxcore/transport"
//
//	// NoiseTransport handles handshake lifecycle and encrypted packet I/O
//	nt := transport.NewNoiseTransport(underlying, keyPair)
//
// See transport/noise_transport.go for production usage patterns including
// session management, idle timeout handling, and automatic replay protection.
package noise
