package async

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"golang.org/x/crypto/hkdf"

	"github.com/opd-ai/toxcore/crypto"
)

// ObfuscationManager handles cryptographic obfuscation of peer identities
// for async message storage. It generates pseudonyms to hide real sender
// and recipient public keys from storage nodes while maintaining message
// deliverability and forward secrecy.
type ObfuscationManager struct {
	epochManager *EpochManager
	keyPair      *crypto.KeyPair
}

// ObfuscatedAsyncMessage represents a message with obfuscated peer identities.
// This structure hides real sender and recipient keys from storage nodes
// while containing an encrypted payload with the actual ForwardSecureMessage.
type ObfuscatedAsyncMessage struct {
	Type               string    `json:"type"`                // "obfuscated_async_message"
	MessageID          [32]byte  `json:"message_id"`          // Random message identifier
	SenderPseudonym    [32]byte  `json:"sender_pseudonym"`    // Hides real sender key
	RecipientPseudonym [32]byte  `json:"recipient_pseudonym"` // Hides real recipient key
	Epoch              uint64    `json:"epoch"`               // Time epoch for validation
	MessageNonce       [24]byte  `json:"message_nonce"`       // Nonce used for pseudonym generation and key derivation
	EncryptedPayload   []byte    `json:"encrypted_payload"`   // AES-GCM encrypted ForwardSecureMessage
	PayloadNonce       [12]byte  `json:"payload_nonce"`       // AES-GCM nonce
	PayloadTag         [16]byte  `json:"payload_tag"`         // AES-GCM authentication tag
	Timestamp          time.Time `json:"timestamp"`           // Creation time
	ExpiresAt          time.Time `json:"expires_at"`          // Expiration time
	RecipientProof     [32]byte  `json:"recipient_proof"`     // HMAC proof of recipient knowledge
}

// NewObfuscationManager creates a new obfuscation manager with the provided
// key pair and epoch manager. The key pair is used for generating pseudonyms
// and the epoch manager provides time-based pseudonym rotation.
func NewObfuscationManager(keyPair *crypto.KeyPair, epochManager *EpochManager) *ObfuscationManager {
	return &ObfuscationManager{
		epochManager: epochManager,
		keyPair:      keyPair,
	}
}

// GenerateRecipientPseudonym creates a deterministic pseudonym for message retrieval.
// The pseudonym is based on the recipient's public key and the current epoch,
// allowing the recipient to compute the same pseudonym for message retrieval
// while hiding their real identity from storage nodes.
func (om *ObfuscationManager) GenerateRecipientPseudonym(recipientPK [32]byte, epoch uint64) ([32]byte, error) {
	// Use HKDF to derive a pseudonym from the recipient's public key and epoch
	// The salt is the epoch as bytes, and the info includes a version identifier
	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, epoch)

	hkdfReader := hkdf.New(sha256.New, recipientPK[:], epochBytes, []byte("TOX_RECIPIENT_PSEUDO_V1"))

	pseudonym := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, pseudonym); err != nil {
		return [32]byte{}, err
	}

	var result [32]byte
	copy(result[:], pseudonym)
	return result, nil
}

// GenerateSenderPseudonym creates a unique pseudonym for each message.
// This pseudonym is unlinkable across messages, preventing storage nodes
// from correlating messages from the same sender. It requires knowledge
// of both the sender's private key and recipient's public key.
func (om *ObfuscationManager) GenerateSenderPseudonym(senderSK [32]byte, recipientPK [32]byte, messageNonce [24]byte) ([32]byte, error) {
	// Combine recipient public key and message nonce for unique info per message
	info := make([]byte, 0, len(recipientPK)+len(messageNonce)+len("TOX_SENDER_PSEUDO_V1"))
	info = append(info, []byte("TOX_SENDER_PSEUDO_V1")...)
	info = append(info, recipientPK[:]...)
	info = append(info, messageNonce[:]...)

	hkdfReader := hkdf.New(sha256.New, senderSK[:], messageNonce[:], info)

	pseudonym := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, pseudonym); err != nil {
		return [32]byte{}, err
	}

	var result [32]byte
	copy(result[:], pseudonym)
	return result, nil
}

// GenerateRecipientProof creates an HMAC proof that the sender knows the
// recipient's real public key. This prevents spam by requiring knowledge
// of the actual recipient identity while maintaining anonymity at the
// storage node level.
func (om *ObfuscationManager) GenerateRecipientProof(recipientPK [32]byte, messageID [32]byte, epoch uint64) ([32]byte, error) {
	// Create proof data: messageID || epoch
	proofData := make([]byte, 0, len(messageID)+8)
	proofData = append(proofData, messageID[:]...)

	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, epoch)
	proofData = append(proofData, epochBytes...)

	// Generate HMAC using recipient's public key as the key
	mac := hmac.New(sha256.New, recipientPK[:])
	mac.Write(proofData)
	hmacResult := mac.Sum(nil)

	var result [32]byte
	copy(result[:], hmacResult)
	return result, nil
}

// VerifyRecipientProof validates that a recipient proof is correct for the
// given parameters. This ensures that the sender actually knows the
// recipient's real public key.
func (om *ObfuscationManager) VerifyRecipientProof(recipientPK [32]byte, messageID [32]byte, epoch uint64, proof [32]byte) bool {
	expectedProof, err := om.GenerateRecipientProof(recipientPK, messageID, epoch)
	if err != nil {
		return false
	}

	return hmac.Equal(proof[:], expectedProof[:])
}

// DerivePayloadKey derives an AES-GCM encryption key for the message payload.
// The key is derived from a shared secret between sender and recipient,
// the message nonce, and the current epoch, ensuring forward secrecy.
func (om *ObfuscationManager) DerivePayloadKey(sharedSecret [32]byte, messageNonce [24]byte, epoch uint64) ([32]byte, error) {
	// Create epoch-specific info for key derivation
	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, epoch)

	info := make([]byte, 0, len("PAYLOAD_ENCRYPTION")+len(epochBytes))
	info = append(info, []byte("PAYLOAD_ENCRYPTION")...)
	info = append(info, epochBytes...)

	hkdfReader := hkdf.New(sha256.New, sharedSecret[:], messageNonce[:], info)

	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return [32]byte{}, err
	}

	var result [32]byte
	copy(result[:], key)
	return result, nil
}

// EncryptPayload encrypts a ForwardSecureMessage payload using AES-GCM.
// The payload key should be derived using DerivePayloadKey to ensure
// proper forward secrecy and authentication.
func (om *ObfuscationManager) EncryptPayload(payload []byte, payloadKey [32]byte) ([]byte, [12]byte, [16]byte, error) {
	// Create AES cipher
	block, err := aes.NewCipher(payloadKey[:])
	if err != nil {
		return nil, [12]byte{}, [16]byte{}, err
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, [12]byte{}, [16]byte{}, err
	}

	// Generate random nonce
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, [12]byte{}, [16]byte{}, err
	}

	// Encrypt and authenticate
	ciphertext := gcm.Seal(nil, nonce[:], payload, nil)

	// Split ciphertext and tag
	if len(ciphertext) < 16 {
		return nil, [12]byte{}, [16]byte{}, errors.New("encrypted payload too short")
	}

	encryptedData := ciphertext[:len(ciphertext)-16]
	var tag [16]byte
	copy(tag[:], ciphertext[len(ciphertext)-16:])

	return encryptedData, nonce, tag, nil
}

// DecryptPayload decrypts an AES-GCM encrypted payload.
// Returns the decrypted ForwardSecureMessage data.
func (om *ObfuscationManager) DecryptPayload(encryptedData []byte, nonce [12]byte, tag [16]byte, payloadKey [32]byte) ([]byte, error) {
	// Create AES cipher
	block, err := aes.NewCipher(payloadKey[:])
	if err != nil {
		return nil, err
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Reconstruct ciphertext with tag
	ciphertext := make([]byte, len(encryptedData)+len(tag))
	copy(ciphertext, encryptedData)
	copy(ciphertext[len(encryptedData):], tag[:])

	// Decrypt and verify
	plaintext, err := gcm.Open(nil, nonce[:], ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// generateRandomIdentifiers creates random message ID and nonce for cryptographic operations.
func (om *ObfuscationManager) generateRandomIdentifiers() ([32]byte, [24]byte, error) {
	var messageID [32]byte
	if _, err := rand.Read(messageID[:]); err != nil {
		return messageID, [24]byte{}, err
	}

	var messageNonce [24]byte
	if _, err := rand.Read(messageNonce[:]); err != nil {
		return messageID, messageNonce, err
	}

	return messageID, messageNonce, nil
}

// generateMessagePseudonyms creates both sender and recipient pseudonyms for obfuscation.
func (om *ObfuscationManager) generateMessagePseudonyms(senderSK [32]byte, recipientPK [32]byte, messageNonce [24]byte, currentEpoch uint64) ([32]byte, [32]byte, error) {
	senderPseudonym, err := om.GenerateSenderPseudonym(senderSK, recipientPK, messageNonce)
	if err != nil {
		return [32]byte{}, [32]byte{}, err
	}

	recipientPseudonym, err := om.GenerateRecipientPseudonym(recipientPK, currentEpoch)
	if err != nil {
		return [32]byte{}, [32]byte{}, err
	}

	return senderPseudonym, recipientPseudonym, nil
}

// generateSecurityElements creates recipient proof and derives payload encryption key.
func (om *ObfuscationManager) generateSecurityElements(recipientPK [32]byte, messageID [32]byte, currentEpoch uint64, sharedSecret [32]byte, messageNonce [24]byte) ([32]byte, [32]byte, error) {
	recipientProof, err := om.GenerateRecipientProof(recipientPK, messageID, currentEpoch)
	if err != nil {
		return [32]byte{}, [32]byte{}, err
	}

	payloadKey, err := om.DerivePayloadKey(sharedSecret, messageNonce, currentEpoch)
	if err != nil {
		return [32]byte{}, [32]byte{}, err
	}

	return recipientProof, payloadKey, nil
}

// encryptMessagePayload encrypts the forward secure message using the derived payload key.
func (om *ObfuscationManager) encryptMessagePayload(forwardSecureMsg []byte, payloadKey [32]byte) ([]byte, [12]byte, [16]byte, error) {
	return om.EncryptPayload(forwardSecureMsg, payloadKey)
}

// CreateObfuscatedMessage creates a new obfuscated message from a ForwardSecureMessage.
// This hides the real sender and recipient identities while maintaining the ability
// for the recipient to retrieve and decrypt the message.
func (om *ObfuscationManager) CreateObfuscatedMessage(senderSK [32]byte, recipientPK [32]byte, forwardSecureMsg []byte, sharedSecret [32]byte) (*ObfuscatedAsyncMessage, error) {
	currentEpoch := om.epochManager.GetCurrentEpoch()

	messageID, messageNonce, err := om.generateRandomIdentifiers()
	if err != nil {
		return nil, err
	}

	senderPseudonym, recipientPseudonym, err := om.generateMessagePseudonyms(senderSK, recipientPK, messageNonce, currentEpoch)
	if err != nil {
		return nil, err
	}

	recipientProof, payloadKey, err := om.generateSecurityElements(recipientPK, messageID, currentEpoch, sharedSecret, messageNonce)
	if err != nil {
		return nil, err
	}

	encryptedPayload, payloadNonce, payloadTag, err := om.encryptMessagePayload(forwardSecureMsg, payloadKey)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(24 * time.Hour)

	return &ObfuscatedAsyncMessage{
		Type:               "obfuscated_async_message",
		MessageID:          messageID,
		SenderPseudonym:    senderPseudonym,
		RecipientPseudonym: recipientPseudonym,
		Epoch:              currentEpoch,
		MessageNonce:       messageNonce,
		EncryptedPayload:   encryptedPayload,
		PayloadNonce:       payloadNonce,
		PayloadTag:         payloadTag,
		Timestamp:          time.Now(),
		ExpiresAt:          expiresAt,
		RecipientProof:     recipientProof,
	}, nil
}

// DecryptObfuscatedMessage attempts to decrypt an obfuscated message using the
// recipient's private key. This verifies that the message was intended for
// this recipient and returns the original ForwardSecureMessage.
func (om *ObfuscationManager) DecryptObfuscatedMessage(obfMsg *ObfuscatedAsyncMessage, recipientSK [32]byte, senderPK [32]byte, sharedSecret [32]byte) ([]byte, error) {
	// Verify the message is for the current recipient by checking the recipient pseudonym
	expectedPseudonym, err := om.GenerateRecipientPseudonym(om.keyPair.Public, obfMsg.Epoch)
	if err != nil {
		return nil, err
	}

	if expectedPseudonym != obfMsg.RecipientPseudonym {
		return nil, errors.New("message not intended for this recipient")
	}

	// Verify recipient proof
	if !om.VerifyRecipientProof(om.keyPair.Public, obfMsg.MessageID, obfMsg.Epoch, obfMsg.RecipientProof) {
		return nil, errors.New("invalid recipient proof")
	}

	// Derive the same payload key that was used for encryption
	// Use the stored message nonce from the obfuscated message
	payloadKey, err := om.DerivePayloadKey(sharedSecret, obfMsg.MessageNonce, obfMsg.Epoch)
	if err != nil {
		return nil, err
	}

	// Decrypt the payload
	forwardSecureMsg, err := om.DecryptPayload(obfMsg.EncryptedPayload, obfMsg.PayloadNonce, obfMsg.PayloadTag, payloadKey)
	if err != nil {
		return nil, err
	}

	return forwardSecureMsg, nil
}
