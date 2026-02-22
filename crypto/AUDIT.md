# Audit: github.com/opd-ai/toxcore/crypto
**Date**: 2026-02-20
**Status**: Needs Work

## Summary
Core cryptographic package with 14 source files, 89.3% test coverage, and 28+ importing source files. Strong security implementation with NaCl-based primitives, but contains one failing test in key rotation functionality (RotateKey test fails on re-encryption). Overall architecture is solid with proper mutex protection, secure memory handling, and comprehensive logging.

## Issues Found
- [x] **high** concurrency — Race condition risk in NonceStore.Close() calling save() with RLock instead of Lock (`replay_protection.go:256-262`)
- [x] **high** error-handling — Test failure in EncryptedKeyStore.RotateKey: re-encryption fails with authentication error (`keystore_test.go:339`)
- [x] **med** error-handling — Swallowed error in ZeroBytes function without any fallback handling (`secure_memory.go:45`)
- [x] **med** documentation — Missing godoc comment for calculateChecksum method violates public method documentation standard (`toxid.go:102`)
- [x] **low** error-handling — load() method silently continues on timestamp conversion errors, potentially losing data (`replay_protection.go:136-142`)
- [x] **low** concurrency — save() method range iteration over map while holding RLock, non-deterministic serialization order (`replay_protection.go:189`)

## Test Coverage
89.3% (target: 65%) - PASSING

Note: One test failure unrelated to coverage: `TestEncryptedKeyStore_RotateKey` fails with "cipher: message authentication failed" during key rotation verification.

## Dependencies
**Standard Library:** crypto/aes, crypto/cipher, crypto/ed25519, crypto/rand, crypto/sha256, crypto/subtle, encoding/binary, encoding/hex, sync, time  
**External:** golang.org/x/crypto/curve25519, golang.org/x/crypto/nacl/box, golang.org/x/crypto/nacl/secretbox, golang.org/x/crypto/pbkdf2, github.com/sirupsen/logrus

**Integration Surface:** 28 unique non-test source files across codebase import this package (async, dht, transport, friend, noise, etc.)

## Recommendations
1. **CRITICAL**: Fix NonceStore.Close() to acquire write lock (Lock instead of RLock) before calling save() to prevent data races during shutdown
2. **HIGH**: Debug and fix EncryptedKeyStore.RotateKey re-encryption logic - authentication failure suggests key derivation mismatch after rotation
3. **MEDIUM**: Add error logging/metrics in ZeroBytes for tracking secure wipe failures in production environments
4. **LOW**: Document calculateChecksum method with algorithm description and add exported wrapper if external checksum validation needed
5. **LOW**: Consider deterministic map serialization in NonceStore.save() for reproducible backups/debugging
