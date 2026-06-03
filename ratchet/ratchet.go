package ratchet

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"

	"github.com/opd-ai/toxcore/crypto"
)

const (
	// rootKDFInfo is the HKDF info string for root-chain derivation.
	rootKDFInfo = "toxcore-dr-root"
	// rootKDFHeaderInfo is the HKDF info string for root-chain with header-key derivation.
	rootKDFHeaderInfo = "toxcore-dr-root-header"
	// msgKDFInfo is the HKDF info string for message-key expansion.
	msgKDFInfo = "toxcore-dr-msg"
	// headerKDFInfo is the HKDF info string for header-key expansion from HKDF.
	headerKDFInfo = "toxcore-header"
	// MaxSkippedKeys is the maximum number of skipped message keys retained.
	MaxSkippedKeys = 1000
	// chainKeyInput01 derives message key from chain key.
	chainKeyInput01 = byte(0x01)
	// chainKeyInput02 derives next chain key from current chain key.
	chainKeyInput02 = byte(0x02)
	// encryptedHeaderSize is [nonce(24)] + [header(40)] + [tag(16)].
	encryptedHeaderSize = chacha20poly1305.NonceSizeX + HeaderSize + chacha20poly1305.Overhead
)

// kdfRootChain derives a new root key and chain key from the current root key
// and a fresh DH output.  Implements KDF_RK from the Signal spec.
// Both rootKey and dhOut are zeroed after use.
func kdfRootChain(rootKey, dhOut [32]byte) (newRK, chainKey [32]byte, err error) {
	defer crypto.ZeroBytes(rootKey[:])
	defer crypto.ZeroBytes(dhOut[:])

	r := hkdf.New(sha256.New, dhOut[:], rootKey[:], []byte(rootKDFInfo))
	if _, err = io.ReadFull(r, newRK[:]); err != nil {
		return newRK, chainKey, errors.New("kdfRootChain: failed to derive root key")
	}
	if _, err = io.ReadFull(r, chainKey[:]); err != nil {
		crypto.ZeroBytes(newRK[:])
		return newRK, chainKey, errors.New("kdfRootChain: failed to derive chain key")
	}
	return newRK, chainKey, nil
}

// kdfRootChainWithHeaders derives a new root key, chain key, header key, and
// next header key from the current root key and a fresh DH output.
// Extends KDF_RK to support Double Ratchet header encryption.
// Both rootKey and dhOut are zeroed after use.
func kdfRootChainWithHeaders(rootKey, dhOut [32]byte) (newRK, chainKey, hk, nhk [32]byte, err error) {
	defer crypto.ZeroBytes(rootKey[:])
	defer crypto.ZeroBytes(dhOut[:])

	r := hkdf.New(sha256.New, dhOut[:], rootKey[:], []byte(rootKDFHeaderInfo))
	if _, err = io.ReadFull(r, newRK[:]); err != nil {
		return newRK, chainKey, hk, nhk, errors.New("kdfRootChainWithHeaders: failed to derive root key")
	}
	if _, err = io.ReadFull(r, chainKey[:]); err != nil {
		crypto.ZeroBytes(newRK[:])
		return newRK, chainKey, hk, nhk, errors.New("kdfRootChainWithHeaders: failed to derive chain key")
	}
	if _, err = io.ReadFull(r, hk[:]); err != nil {
		crypto.ZeroBytes(newRK[:])
		crypto.ZeroBytes(chainKey[:])
		return newRK, chainKey, hk, nhk, errors.New("kdfRootChainWithHeaders: failed to derive header key")
	}
	if _, err = io.ReadFull(r, nhk[:]); err != nil {
		crypto.ZeroBytes(newRK[:])
		crypto.ZeroBytes(chainKey[:])
		crypto.ZeroBytes(hk[:])
		return newRK, chainKey, hk, nhk, errors.New("kdfRootChainWithHeaders: failed to derive next header key")
	}
	return newRK, chainKey, hk, nhk, nil
}

// kdfInitialHeaderKeys is the HKDF info string for bootstrap header-key derivation.
const kdfInitialHeaderKeysInfo = "toxcore-dr-init-header"

// kdfInitialHeaderKeys derives initial header keys (hks, nhks, hkr, nhkr) from
// the root key without consuming or modifying rk.  Used to bootstrap header
// encryption on a fresh session before the first DH ratchet step (L-6 remediation).
func kdfInitialHeaderKeys(rk [32]byte) (hks, nhks, hkr, nhkr [32]byte, err error) {
	// Use the root key as IKM with a distinct info string to derive initial header keys.
	// This produces deterministic keys for both peers if they share the same rk.
	r := hkdf.New(sha256.New, rk[:], nil, []byte(kdfInitialHeaderKeysInfo))

	if _, err = io.ReadFull(r, hks[:]); err != nil {
		return hks, nhks, hkr, nhkr, errors.New("kdfInitialHeaderKeys: failed to derive sending header key")
	}
	if _, err = io.ReadFull(r, nhks[:]); err != nil {
		crypto.ZeroBytes(hks[:])
		return hks, nhks, hkr, nhkr, errors.New("kdfInitialHeaderKeys: failed to derive next sending header key")
	}
	if _, err = io.ReadFull(r, hkr[:]); err != nil {
		crypto.ZeroBytes(hks[:])
		crypto.ZeroBytes(nhks[:])
		return hks, nhks, hkr, nhkr, errors.New("kdfInitialHeaderKeys: failed to derive receiving header key")
	}
	if _, err = io.ReadFull(r, nhkr[:]); err != nil {
		crypto.ZeroBytes(hks[:])
		crypto.ZeroBytes(nhks[:])
		crypto.ZeroBytes(hkr[:])
		return hks, nhks, hkr, nhkr, errors.New("kdfInitialHeaderKeys: failed to derive next receiving header key")
	}
	return hks, nhks, hkr, nhkr, nil
}

// kdfChain derives a message key and the next chain key from the current chain
// key using HMAC-SHA-256.  Implements KDF_CK from the Signal spec.
// The input chainKey is zeroed after use.
func kdfChain(chainKey [32]byte) (newChainKey, msgKey [32]byte) {
	defer crypto.ZeroBytes(chainKey[:])

	mac := hmac.New(sha256.New, chainKey[:])
	mac.Write([]byte{chainKeyInput01})
	copy(msgKey[:], mac.Sum(nil))

	mac.Reset()
	mac.Write([]byte{chainKeyInput02})
	copy(newChainKey[:], mac.Sum(nil))
	return newChainKey, msgKey
}

// encryptWithMsgKey encrypts plaintext using XChaCha20-Poly1305 with keys
// derived from msgKey via HKDF.  The associated data ad is authenticated but
// not encrypted.  msgKey is zeroed after use.
func encryptWithMsgKey(msgKey [32]byte, plaintext, ad []byte) ([]byte, error) {
	defer crypto.ZeroBytes(msgKey[:])

	encKey, nonce, err := expandMsgKey(msgKey)
	if err != nil {
		return nil, err
	}
	defer crypto.ZeroBytes(encKey[:])

	aead, err := chacha20poly1305.NewX(encKey[:])
	if err != nil {
		return nil, errors.New("encryptWithMsgKey: failed to create cipher")
	}
	return aead.Seal(nil, nonce[:], plaintext, ad), nil
}

// decryptWithMsgKey decrypts ciphertext using XChaCha20-Poly1305 with keys
// derived from msgKey via HKDF.  msgKey is zeroed after use.
func decryptWithMsgKey(msgKey [32]byte, ciphertext, ad []byte) ([]byte, error) {
	defer crypto.ZeroBytes(msgKey[:])

	encKey, nonce, err := expandMsgKey(msgKey)
	if err != nil {
		return nil, err
	}
	defer crypto.ZeroBytes(encKey[:])

	aead, err := chacha20poly1305.NewX(encKey[:])
	if err != nil {
		return nil, errors.New("decryptWithMsgKey: failed to create cipher")
	}
	plain, err := aead.Open(nil, nonce[:], ciphertext, ad)
	if err != nil {
		return nil, errors.New("decryptWithMsgKey: authentication failed")
	}
	return plain, nil
}

// expandMsgKey derives an encryption key and nonce from a message key.
func expandMsgKey(msgKey [32]byte) (encKey [32]byte, nonce [chacha20poly1305.NonceSizeX]byte, err error) {
	r := hkdf.New(sha256.New, msgKey[:], nil, []byte(msgKDFInfo))
	if _, err = io.ReadFull(r, encKey[:]); err != nil {
		return encKey, nonce, errors.New("expandMsgKey: failed to derive encryption key")
	}
	if _, err = io.ReadFull(r, nonce[:]); err != nil {
		crypto.ZeroBytes(encKey[:])
		return encKey, nonce, errors.New("expandMsgKey: failed to derive nonce")
	}
	return encKey, nonce, nil
}

// sealHeader encrypts a Header with the given header key using XChaCha20-Poly1305.
// Returns [nonce(24)][ciphertext(40+16)].
func sealHeader(hk [32]byte, h Header) ([]byte, error) {
	encKey, err := expandHeaderKey(hk)
	if err != nil {
		return nil, err
	}
	defer crypto.ZeroBytes(encKey[:])

	aead, err := chacha20poly1305.NewX(encKey[:])
	if err != nil {
		return nil, errors.New("sealHeader: failed to create cipher")
	}

	var nonce [chacha20poly1305.NonceSizeX]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, errors.New("sealHeader: failed to generate nonce")
	}

	plaintext := h.Encode()
	sealed := aead.Seal(nil, nonce[:], plaintext, nil)
	result := make([]byte, chacha20poly1305.NonceSizeX+len(sealed))
	copy(result[:chacha20poly1305.NonceSizeX], nonce[:])
	copy(result[chacha20poly1305.NonceSizeX:], sealed)
	return result, nil
}

// openHeader decrypts a sealed header with the given key(s).
// Returns the decrypted Header and a boolean indicating if the next-header-key was used
// (which signals a DH-ratchet step).
// Tries hk first, then nhk on authentication failure.
func openHeader(hk, nhk [32]byte, encHeader []byte) (Header, bool, error) {
	// Try current header key first
	h, err := openHeaderWithKey(hk, encHeader)
	if err == nil {
		return h, false, nil
	}

	// Try next header key (indicates DH-ratchet step)
	h, err = openHeaderWithKey(nhk, encHeader)
	if err == nil {
		return h, true, nil
	}

	// Both keys failed
	return Header{}, false, errors.New("openHeader: failed to decrypt with both current and next header keys")
}

// openHeaderWithKey decrypts a sealed header with a specific key.
func openHeaderWithKey(hk [32]byte, encHeader []byte) (Header, error) {
	minLen := chacha20poly1305.NonceSizeX + HeaderSize + chacha20poly1305.Overhead
	if len(encHeader) < minLen {
		return Header{}, errors.New("openHeaderWithKey: encrypted header too short")
	}

	encKey, err := expandHeaderKey(hk)
	if err != nil {
		return Header{}, err
	}
	defer crypto.ZeroBytes(encKey[:])

	aead, err := chacha20poly1305.NewX(encKey[:])
	if err != nil {
		return Header{}, errors.New("openHeaderWithKey: failed to create cipher")
	}

	nonce := encHeader[:chacha20poly1305.NonceSizeX]
	sealed := encHeader[chacha20poly1305.NonceSizeX:]
	plaintext, err := aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return Header{}, err
	}
	if len(plaintext) < HeaderSize {
		return Header{}, errors.New("openHeaderWithKey: decrypted header too short")
	}
	h, err := DecodeHeader(plaintext)
	if err != nil {
		return Header{}, err
	}
	return h, nil
}

// expandHeaderKey derives an encryption key from a header key.
func expandHeaderKey(hk [32]byte) (encKey [32]byte, err error) {
	r := hkdf.New(sha256.New, hk[:], nil, []byte(headerKDFInfo))
	if _, err = io.ReadFull(r, encKey[:]); err != nil {
		return encKey, errors.New("expandHeaderKey: failed to derive encryption key")
	}
	return encKey, nil
}
