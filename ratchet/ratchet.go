package ratchet

import (
	"crypto/hmac"
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
	// msgKDFInfo is the HKDF info string for message-key expansion.
	msgKDFInfo = "toxcore-dr-msg"
	// MaxSkippedKeys is the maximum number of skipped message keys retained.
	MaxSkippedKeys = 1000
	// chainKeyInput01 derives message key from chain key.
	chainKeyInput01 = byte(0x01)
	// chainKeyInput02 derives next chain key from current chain key.
	chainKeyInput02 = byte(0x02)
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
