// Package crypto implements cryptographic primitives for the Tox protocol.
//
// This package provides the cryptographic foundation for toxcore-go, implementing
// NaCl-based authenticated encryption, Ed25519 signatures, secure key management,
// and memory-safe cryptographic operations. It follows the Tox protocol specification
// while providing additional security features like key rotation and replay protection.
//
// # Core Types
//
// The package defines several core types for cryptographic operations:
//
//   - [KeyPair]: NaCl crypto_box key pair (Curve25519) for encryption/decryption
//   - [Nonce]: 24-byte random nonce for encryption operations
//   - [Signature]: Ed25519 signature with public key for verification
//   - [ToxID]: Complete Tox identity (public key + nospam + checksum)
//
// # Encryption and Decryption
//
// The package supports both authenticated public-key encryption (NaCl box) and
// symmetric encryption (NaCl secretbox):
//
//	// Public-key encryption
//	nonce, _ := crypto.GenerateNonce()
//	ciphertext, _ := crypto.EncryptWithPeer(plaintext, nonce, peerPublicKey, myPrivateKey)
//
//	// Public-key decryption
//	plaintext, _ := crypto.DecryptWithPeer(ciphertext, nonce, peerPublicKey, myPrivateKey)
//
//	// Symmetric encryption with shared secret
//	sharedKey := crypto.SharedSecret(peerPublicKey, myPrivateKey)
//	ciphertext, _ := crypto.SymmetricEncrypt(plaintext, nonce, sharedKey)
//
// # Digital Signatures
//
// Ed25519 signatures are used for message authentication and identity verification:
//
//	// Sign a message
//	signature, _ := crypto.Sign(message, privateKey)
//
//	// Verify a signature
//	valid := crypto.Verify(message, signature, publicKey)
//
// # Key Generation
//
// Generate new cryptographic key pairs using secure random entropy:
//
//	// Generate NaCl box key pair
//	keyPair, err := crypto.GenerateKeyPair()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer crypto.WipeKeyPair(keyPair) // Secure cleanup
//
//	// Create key pair from existing secret key
//	keyPair, err := crypto.FromSecretKey(secretKeyBytes)
//
// # Key Management
//
// The package provides several key management facilities:
//
// KeyRotationManager handles periodic rotation of identity keys for forward secrecy:
//
//	krm := crypto.NewKeyRotationManager(initialKeyPair)
//	if krm.ShouldRotate() {
//	    newKeys, _ := krm.RotateKey()
//	}
//
// EncryptedKeyStore provides encrypted at-rest storage for sensitive keys:
//
//	store, _ := crypto.NewEncryptedKeyStore("/path/to/data", []byte("passphrase"))
//	store.StoreKey("identity", keyPair.Private[:])
//	key, _ := store.LoadKey("identity")
//
// NonceStore provides replay attack protection through persistent nonce tracking:
//
//	ns, _ := crypto.NewNonceStore("/path/to/data")
//	if ns.CheckAndStore(nonce, timestamp) {
//	    // Nonce is fresh, proceed with message
//	} else {
//	    // Replay attack detected
//	}
//
// # Secure Memory Handling
//
// All sensitive data should be securely wiped after use to prevent memory disclosure:
//
//	defer crypto.SecureWipe(sensitiveData)
//	defer crypto.WipeKeyPair(keyPair)
//
// The [SecureWipe] function uses constant-time XOR operations that cannot be
// optimized away by the compiler, ensuring memory is actually zeroed.
//
// # Deterministic Testing
//
// For reproducible testing, time-dependent components support injectable time providers:
//
//	mockTime := &crypto.MockTimeProvider{CurrentTime: time.Unix(1000, 0)}
//	krm := crypto.NewKeyRotationManagerWithTimeProvider(keyPair, mockTime)
//	ns, _ := crypto.NewNonceStoreWithTimeProvider(dataDir, mockTime)
//
// # Security Considerations
//
// The package implements several security best practices:
//
//   - Constant-time operations via crypto/subtle to prevent timing attacks
//   - Proper Curve25519 key clamping per RFC 7748
//   - PBKDF2 with 100,000 iterations for key derivation (NIST recommendation)
//   - AES-256-GCM for at-rest encryption with unique nonces
//   - Automatic secure wiping of intermediate cryptographic material
//   - Input validation to prevent buffer overflows and DoS attacks
//
// # Thread Safety
//
// All exported types in this package are safe for concurrent use:
//
//   - KeyRotationManager uses sync.RWMutex for concurrent access
//   - NonceStore uses sync.RWMutex with background cleanup goroutine
//   - EncryptedKeyStore operations are atomic file operations
//   - Pure functions (encryption/decryption/signing) are inherently thread-safe
//
// # C API Bindings
//
// Most exported functions include //export directives for C interoperability:
//
//	ToxGenerateKeyPair()     - Generate new key pair
//	ToxGenerateNonce()       - Generate random nonce
//	ToxEncrypt()             - Encrypt with public key
//	ToxDecrypt()             - Decrypt with public key
//	ToxSecureWipe()          - Securely erase memory
//	ToxSign()                - Create Ed25519 signature
//	ToxVerify()              - Verify Ed25519 signature
//
// Build with -buildmode=c-shared to generate the C library.
//
// # Integration with Tox Protocol
//
// This package integrates with other toxcore-go packages:
//
//   - async/: Pre-key encryption, forward secrecy, identity obfuscation
//   - dht/: Node identity verification, bootstrap handshakes
//   - transport/: Noise protocol integration, packet encryption
//   - friend/: Friend request signatures and verification
//   - capi/: C bindings for cross-language interoperability
package crypto
