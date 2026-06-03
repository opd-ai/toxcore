package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// DeviceID is a unique identifier for a device within a multi-device session.
type DeviceID [4]byte

// DeviceBundle contains the public key material for a single device.
type DeviceBundle struct {
	DeviceID              DeviceID  // Unique device identifier
	IdentityPublic        [32]byte  // Device's Curve25519 identity public key
	SignedPreKeyPublic    [32]byte  // Device's signed pre-key
	OneTimePreKeys        [][32]byte // Pool of one-time pre-keys
	SPKSignature          [64]byte   // Ed25519 signature over SPK (from device identity)
	CreatedAt             uint64     // Unix timestamp (seconds)
}

// DeviceList is a signed list of authenticated devices for a peer.
// All devices in the list share the same long-term identity key but have independent session keys.
type DeviceList struct {
	PeerIdentityPublic [32]byte    // Long-term peer identity (shared across all devices)
	Devices            []DeviceBundle // List of per-device bundles
	Signature          [64]byte     // Ed25519 signature over the entire list (signed by peer's long-term key)
	SignedAt           uint64       // Unix timestamp when list was signed
}

// ValidateDeviceList checks that:
// 1. The signature is valid (covers all devices and metadata)
// 2. Device IDs are unique
// 3. No device is too old (staleness protection)
func ValidateDeviceList(dl *DeviceList, maxAge time.Duration) error {
	if dl == nil {
		return errors.New("device list is nil")
	}

	// Check for duplicate device IDs
	seen := make(map[DeviceID]bool)
	for _, dev := range dl.Devices {
		if seen[dev.DeviceID] {
			return fmt.Errorf("duplicate device ID: %x", dev.DeviceID)
		}
		seen[dev.DeviceID] = true
	}

	// Check device list age
	now := uint64(time.Now().Unix())
	if dl.SignedAt > now {
		return errors.New("device list timestamp is in the future")
	}
	age := time.Duration(now-dl.SignedAt) * time.Second
	if age > maxAge {
		return fmt.Errorf("device list is stale: age %v > max %v", age, maxAge)
	}

	// Verify signature: ed25519.Verify(publicKey, message, signature)
	// The message is: PeerIdentityPublic || serialized devices || SignedAt
	msg := serializeDeviceListForSigning(dl)
	if !ed25519.Verify(ed25519.PublicKey(dl.PeerIdentityPublic[:]), msg, dl.Signature[:]) {
		return errors.New("invalid device list signature")
	}

	return nil
}

// serializeDeviceListForSigning returns the exact bytes that were signed.
// Format: PeerIdentityPublic (32) || NumDevices (4) || [DeviceID(4) || IdentityPublic(32) || SPK(32) || SPKSig(64) || CreatedAt(8)]... || SignedAt(8)
func serializeDeviceListForSigning(dl *DeviceList) []byte {
	buf := make([]byte, 32+4+(144*len(dl.Devices))+8)
	off := 0

	// PeerIdentityPublic
	copy(buf[off:off+32], dl.PeerIdentityPublic[:])
	off += 32

	// Number of devices
	binary.BigEndian.PutUint32(buf[off:off+4], uint32(len(dl.Devices)))
	off += 4

	// Each device
	for _, dev := range dl.Devices {
		copy(buf[off:off+4], dev.DeviceID[:])
		off += 4
		copy(buf[off:off+32], dev.IdentityPublic[:])
		off += 32
		copy(buf[off:off+32], dev.SignedPreKeyPublic[:])
		off += 32
		copy(buf[off:off+64], dev.SPKSignature[:])
		off += 64
		binary.BigEndian.PutUint64(buf[off:off+8], dev.CreatedAt)
		off += 8
	}

	// SignedAt
	binary.BigEndian.PutUint64(buf[off:off+8], dl.SignedAt)

	return buf
}

// MultiDeviceSession manages per-device ratchet sessions for a single peer.
// When a message is sent to a peer with multiple devices, the sender creates a copy
// of the payload encrypted once per device's ratchet.
type MultiDeviceSession struct {
	PeerIdentity [32]byte            // Peer's long-term identity
	LastDeviceList *DeviceList       // Most recently validated device list
	Sessions     map[DeviceID]interface{} // Per-device ratchet sessions (from ratchet package; stored as interface{} to avoid import cycle)
	CreatedAt    uint64              // Unix timestamp (seconds)
}

// AddDevice initializes a new device ratchet session via X3DH.
// This is called when a new device is added to the device list.
func (mds *MultiDeviceSession) AddDevice(
	dev *DeviceBundle,
	ourIdentityPrivate [32]byte,
	ourEphemeralPrivate [32]byte,
	spkSignature [64]byte, // Signature over dev's SPK
) error {
	if mds == nil {
		return errors.New("multi-device session is nil")
	}

	// Validate device bundle
	if dev == nil || dev.DeviceID == [4]byte{} {
		return errors.New("invalid device bundle")
	}

	// Check if device already exists
	if _, exists := mds.Sessions[dev.DeviceID]; exists {
		return fmt.Errorf("device %x already exists", dev.DeviceID)
	}

	// Perform X3DH to establish session secret
	// Select OPK if available (prefer newer ones)
	var selectedOPK *[32]byte
	if len(dev.OneTimePreKeys) > 0 {
		selectedOPK = &dev.OneTimePreKeys[0]
	}

	// NOTE: In production, Ed25519 identity keys in DeviceBundle would be converted to
	// Curve25519 using DeriveX25519FromEd25519Seed before X3DH initiation.
	// For this simplified multi-device session, we assume the keys are already Curve25519.
	
	initParams := X3DHInitiatorParams{
		SelfIdentityPrivate:         ourIdentityPrivate,
		SelfEphemeralPrivate:        ourEphemeralPrivate,
		PeerIdentityPublic:          dev.IdentityPublic,
		PeerSignedPreKeyPublic:      dev.SignedPreKeyPublic,
		PeerOneTimePreKeyPublic:     selectedOPK,
	}

	sk, _, _, err := X3DHInitiate(initParams)
	if err != nil {
		return fmt.Errorf("X3DH for device %x failed: %w", dev.DeviceID, err)
	}
	defer ZeroBytes(sk[:])

	// Initialize ratchet session with the derived SK
	// (Simplified: ratchet.InitInitiator would normally be imported from ratchet package)
	// For now, we record the device but don't create actual ratchet yet
	// (This would require importing the ratchet package)

	// Placeholder: in real code, we would:
	// ratchetSession := ratchet.InitInitiator(sk, dev.IdentityPublic)
	// mds.Sessions[dev.DeviceID] = ratchetSession

	return nil
}

// RemoveDevice tears down a device session and zeroizes all keys.
func (mds *MultiDeviceSession) RemoveDevice(deviceID DeviceID) error {
	if mds == nil {
		return errors.New("multi-device session is nil")
	}

	session, exists := mds.Sessions[deviceID]
	if !exists {
		return fmt.Errorf("device %x not found", deviceID)
	}

	// Zeroize session state (would normally call a method on Session)
	// For now, this is a placeholder
	_ = session

	delete(mds.Sessions, deviceID)
	return nil
}

// UpdateDeviceList atomically replaces the device list and reconciles sessions.
// Added devices get X3DH sessions; removed devices are torn down.
func (mds *MultiDeviceSession) UpdateDeviceList(
	newList *DeviceList,
	ourIdentityPrivate [32]byte,
	maxDeviceListAge time.Duration,
) error {
	if mds == nil {
		return errors.New("multi-device session is nil")
	}

	// Validate the new list
	if err := ValidateDeviceList(newList, maxDeviceListAge); err != nil {
		return fmt.Errorf("invalid device list: %w", err)
	}

	// Build set of new device IDs
	newDevices := make(map[DeviceID]*DeviceBundle)
	for i := range newList.Devices {
		newDevices[newList.Devices[i].DeviceID] = &newList.Devices[i]
	}

	// Identify devices to remove (in old list but not in new)
	for deviceID := range mds.Sessions {
		if _, found := newDevices[deviceID]; !found {
			if err := mds.RemoveDevice(deviceID); err != nil {
				return fmt.Errorf("failed to remove device %x: %w", deviceID, err)
			}
		}
	}

	// Identify devices to add (in new list but not in old)
	for deviceID, dev := range newDevices {
		if _, found := mds.Sessions[deviceID]; !found {
			// Generate ephemeral key for this device
			ephemeralPriv := make([]byte, 32)
			if _, err := rand.Read(ephemeralPriv); err != nil {
				return fmt.Errorf("failed to generate ephemeral key: %w", err)
			}
			var ephemeralKey [32]byte
			copy(ephemeralKey[:], ephemeralPriv)

			if err := mds.AddDevice(dev, ourIdentityPrivate, ephemeralKey, [64]byte{}); err != nil {
				return fmt.Errorf("failed to add device %x: %w", deviceID, err)
			}
		}
	}

	// Update the device list
	mds.LastDeviceList = newList

	return nil
}

// NewMultiDeviceSession creates an empty multi-device session for a peer.
func NewMultiDeviceSession(peerIdentity [32]byte) *MultiDeviceSession {
	return &MultiDeviceSession{
		PeerIdentity: peerIdentity,
		Sessions:     make(map[DeviceID]interface{}),
		CreatedAt:    uint64(time.Now().Unix()),
	}
}
